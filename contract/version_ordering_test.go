package contract

import (
	"errors"
	"testing"
	"time"
)

func TestDetectVersionConflict(t *testing.T) {
	if DetectVersionConflict(0, 5) != nil {
		t.Fatalf("unversioned write should not conflict")
	}
	if DetectVersionConflict(5, 5) != nil {
		t.Fatalf("matching versions should not conflict")
	}
	err := DetectVersionConflict(3, 5)
	var v *VersionMismatchError
	if !errors.As(err, &v) {
		t.Fatalf("want *VersionMismatchError, got %T", err)
	}
	if v.ExpectedVersion != 3 || v.ActualVersion != 5 || v.Kind != "version_mismatch" {
		t.Fatalf("unexpected mismatch: %+v", v)
	}
}

func TestNewConflictRecord(t *testing.T) {
	now := time.Now().UTC()
	r := NewConflictRecord("site-a", "animal", "animal-0001", 3, 5, "key-1", now)
	if r.Site != "site-a" || r.Doctype != "animal" || r.LosingExpectedV != 3 || r.WinningActualV != 5 || r.State != "open" {
		t.Fatalf("unexpected conflict record: %+v", r)
	}
	if r.Name == "" {
		t.Fatalf("conflict record missing name")
	}
}

func TestOrderByCausationParentBeforeChild(t *testing.T) {
	parent := CommandEnvelope{ID: "p1"}
	child := CommandEnvelope{ID: "c1", CausationID: "p1"}
	// Shuffle: child submitted before parent.
	ordered, err := OrderByCausation([]CommandEnvelope{child, parent})
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if ordered[0].ID != "p1" || ordered[1].ID != "c1" {
		t.Fatalf("parent must precede child: %v", ordered)
	}
}

func TestOrderByCausationIndependentPreservesOrder(t *testing.T) {
	a := CommandEnvelope{ID: "a"}
	b := CommandEnvelope{ID: "b"}
	c := CommandEnvelope{ID: "c"}
	ordered, err := OrderByCausation([]CommandEnvelope{a, b, c})
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if ordered[0].ID != "a" || ordered[1].ID != "b" || ordered[2].ID != "c" {
		t.Fatalf("independent order changed: %v", ordered)
	}
}

func TestOrderByCausationCycleDetected(t *testing.T) {
	a := CommandEnvelope{ID: "a", CausationID: "b"}
	b := CommandEnvelope{ID: "b", CausationID: "a"}
	_, err := OrderByCausation([]CommandEnvelope{a, b})
	if !errors.Is(err, ErrCausationCycle) {
		t.Fatalf("want ErrCausationCycle, got %v", err)
	}
}

func TestOrderByCausationMissingParent(t *testing.T) {
	// A command whose parent is not in the batch is treated as root.
	child := CommandEnvelope{ID: "c1", CausationID: "not-in-batch"}
	other := CommandEnvelope{ID: "o1"}
	ordered, err := OrderByCausation([]CommandEnvelope{child, other})
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if len(ordered) != 2 {
		t.Fatalf("len = %d", len(ordered))
	}
}

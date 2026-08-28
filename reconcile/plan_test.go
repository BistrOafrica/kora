package reconcile

import (
	"testing"

	"github.com/asenawritescode/kora/contract"
)

func ref(ns, name string, version int) contract.ResourceRef {
	return contract.ResourceRef{Namespace: ns, Name: name, Version: version}
}

func TestCompareInSync(t *testing.T) {
	plan := Compare(
		DesiredState{Generation: 4, Resources: []contract.ResourceRef{ref("tenant", "animal", 1), ref("tenant", "farm", 1)}},
		ObservedState{Generation: 4, Resources: []contract.ResourceRef{ref("tenant", "farm", 1), ref("tenant", "animal", 1)}},
	)
	if plan.Status != PlanInSync {
		t.Fatalf("status = %v, want in_sync", plan.Status)
	}
	if len(plan.Missing) != 0 || len(plan.Stale) != 0 {
		t.Fatalf("expected aligned state, got %+v", plan)
	}
}

func TestComparePendingGeneration(t *testing.T) {
	plan := Compare(
		DesiredState{Generation: 5, Resources: []contract.ResourceRef{ref("tenant", "animal", 1)}},
		ObservedState{Generation: 4, Resources: []contract.ResourceRef{ref("tenant", "animal", 1)}},
	)
	if plan.Status != PlanPending {
		t.Fatalf("status = %v, want pending", plan.Status)
	}
	if len(plan.Missing) != 0 || len(plan.Stale) != 0 {
		t.Fatalf("generation mismatch should not imply resource drift: %+v", plan)
	}
}

func TestCompareDetectsDrift(t *testing.T) {
	plan := Compare(
		DesiredState{Generation: 7, Resources: []contract.ResourceRef{ref("tenant", "animal", 1)}},
		ObservedState{Generation: 7, Resources: []contract.ResourceRef{ref("tenant", "farm", 1)}},
	)
	if plan.Status != PlanDrifted {
		t.Fatalf("status = %v, want drifted", plan.Status)
	}
	if len(plan.Missing) != 1 || plan.Missing[0].Name != "animal" {
		t.Fatalf("expected missing desired resource: %+v", plan.Missing)
	}
	if len(plan.Stale) != 1 || plan.Stale[0].Name != "farm" {
		t.Fatalf("expected stale observed resource: %+v", plan.Stale)
	}
}

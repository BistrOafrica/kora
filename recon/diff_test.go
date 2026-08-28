package recon

import (
	"testing"
	"time"

	"github.com/asenawritescode/kora/contract"
)

func ref(ns, name string, v int) contract.ResourceRef {
	return contract.ResourceRef{Namespace: ns, Name: name, Version: v}
}

func TestDiffClassifiesAllFourDriftKinds(t *testing.T) {
	now := time.Now().UTC()
	desired := DesiredState{
		TenantID:   "tenant-a",
		Generation: 3,
		Resources:  []contract.ResourceRef{ref("ns", "animal", 1), ref("ns", "invoice", 1)},
	}
	observed := []contract.ComponentObservation{
		{Ref: ref("ns", "animal", 1), Status: contract.ObservationFailed, Generation: 3},   // StatusMismatch
		{Ref: ref("ns", "invoice", 1), Status: contract.ObservationHealthy, Generation: 2}, // GenerationLag
		{Ref: ref("ns", "stray", 1), Status: contract.ObservationHealthy, Generation: 3},   // Extra
	}
	// "animal" and "invoice" present; "missing" resource absent from desired but not observed → missing only if desired has an unobserved resource.
	desired.Resources = append(desired.Resources, ref("ns", "missing", 1)) // Missing

	drifts := Diff(desired, observed, now)
	kinds := map[DriftKind]bool{}
	for _, d := range drifts {
		kinds[d.Kind] = true
	}
	for _, want := range []DriftKind{DriftMissing, DriftExtra, DriftStatusMismatch, DriftGenerationLag} {
		if !kinds[want] {
			t.Fatalf("missing drift kind %q in %+v", want, drifts)
		}
	}
}

func TestDiffDeterministic(t *testing.T) {
	now := time.Now().UTC()
	desired := DesiredState{TenantID: "t", Generation: 2, Resources: []contract.ResourceRef{ref("n", "b", 1), ref("n", "a", 1)}}
	observed := []contract.ComponentObservation{{Ref: ref("n", "a", 1), Status: contract.ObservationFailed, Generation: 2}}
	a := Diff(desired, observed, now)
	b := Diff(desired, observed, now)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Resource != b[i].Resource || a[i].Kind != b[i].Kind {
			t.Fatalf("non-deterministic order at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestDiffGenerationLagWhenObservedBehindDesired(t *testing.T) {
	now := time.Now().UTC()
	desired := DesiredState{TenantID: "t", Generation: 5, Resources: []contract.ResourceRef{ref("n", "a", 1)}}
	observed := []contract.ComponentObservation{{Ref: ref("n", "a", 1), Status: contract.ObservationHealthy, Generation: 4}}
	drifts := Diff(desired, observed, now)
	if len(drifts) != 1 || drifts[0].Kind != DriftGenerationLag {
		t.Fatalf("want single GenerationLag, got %+v", drifts)
	}
}

func TestDiffNoDriftWhenMatchedHealthy(t *testing.T) {
	now := time.Now().UTC()
	desired := DesiredState{TenantID: "t", Generation: 2, Resources: []contract.ResourceRef{ref("n", "a", 1)}}
	observed := []contract.ComponentObservation{{Ref: ref("n", "a", 1), Status: contract.ObservationHealthy, Generation: 2}}
	if drifts := Diff(desired, observed, now); len(drifts) != 0 {
		t.Fatalf("expected no drift, got %+v", drifts)
	}
}

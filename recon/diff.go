// Package recon implements reconciliation primitives (RECON-001/002): a typed
// comparison between desired generations and observed state that produces
// deterministic drift records. Diff is pure — no DB, no I/O — so it can be
// unit-tested and re-derived identically after restart (invariant 1).
package recon

import (
	"sort"
	"time"

	"github.com/asenawritescode/kora/contract"
)

// DriftKind classifies a divergence between desired and observed state.
type DriftKind string

const (
	DriftMissing        DriftKind = "missing"
	DriftExtra          DriftKind = "extra"
	DriftStatusMismatch DriftKind = "status_mismatch"
	DriftGenerationLag  DriftKind = "generation_lag"
)

// Drift is one detected divergence. Desired is zero-valued for Extra; Observed
// is zero-valued for Missing.
type Drift struct {
	TenantID   string
	Resource   contract.ResourceRef
	Kind       DriftKind
	Desired    contract.ComponentObservation
	Observed   contract.ComponentObservation
	DetectedAt time.Time
}

// DesiredState is the active desired generation for a tenant: the set of
// resources that should exist at a given generation.
type DesiredState struct {
	TenantID   string
	Generation int
	Resources  []contract.ResourceRef
}

// Diff compares desired against observed and returns all drifts in a
// deterministic order (namespace, name, kind, version). It is pure and
// deterministic: identical inputs produce identical ordered output.
func Diff(desired DesiredState, observed []contract.ComponentObservation, now time.Time) []Drift {
	observedByRef := make(map[contract.ResourceRef]contract.ComponentObservation, len(observed))
	for _, o := range observed {
		observedByRef[o.Ref] = o
	}
	desiredSet := make(map[contract.ResourceRef]bool, len(desired.Resources))
	for _, r := range desired.Resources {
		desiredSet[r] = true
	}

	drifts := make([]Drift, 0, len(desired.Resources)+len(observed))

	for _, r := range desired.Resources {
		o, ok := observedByRef[r]
		switch {
		case !ok:
			drifts = append(drifts, Drift{
				TenantID: desired.TenantID, Resource: r, Kind: DriftMissing, DetectedAt: now,
			})
		case o.Generation < desired.Generation:
			drifts = append(drifts, Drift{
				TenantID: desired.TenantID, Resource: r, Kind: DriftGenerationLag,
				Desired: desiredObs(r, desired.Generation), Observed: o, DetectedAt: now,
			})
		case o.Generation == desired.Generation && o.Status != contract.ObservationHealthy:
			drifts = append(drifts, Drift{
				TenantID: desired.TenantID, Resource: r, Kind: DriftStatusMismatch,
				Desired: desiredObs(r, desired.Generation), Observed: o, DetectedAt: now,
			})
		}
	}

	for _, o := range observed {
		if !desiredSet[o.Ref] {
			drifts = append(drifts, Drift{
				TenantID: desired.TenantID, Resource: o.Ref, Kind: DriftExtra, Observed: o, DetectedAt: now,
			})
		}
	}

	sort.Slice(drifts, func(i, j int) bool {
		a, b := drifts[i], drifts[j]
		if a.Resource.Namespace != b.Resource.Namespace {
			return a.Resource.Namespace < b.Resource.Namespace
		}
		if a.Resource.Name != b.Resource.Name {
			return a.Resource.Name < b.Resource.Name
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Resource.Version < b.Resource.Version
	})
	return drifts
}

func desiredObs(r contract.ResourceRef, gen int) contract.ComponentObservation {
	return contract.ComponentObservation{Ref: r, Status: contract.ObservationHealthy, Generation: gen}
}

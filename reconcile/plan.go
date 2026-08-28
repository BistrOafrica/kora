package reconcile

import (
	"sort"

	"github.com/asenawritescode/kora/contract"
)

// DesiredState is the runtime's intended resource set at a specific generation.
type DesiredState struct {
	Generation int
	Resources  []contract.ResourceRef
}

// ObservedState is the runtime-reported resource set at a specific generation.
type ObservedState struct {
	Generation int
	Resources  []contract.ResourceRef
}

// PlanStatus describes whether desired and observed state are aligned.
type PlanStatus string

const (
	PlanInSync  PlanStatus = "in_sync"
	PlanPending PlanStatus = "pending"
	PlanDrifted PlanStatus = "drifted"
)

// Plan compares desired vs observed state and reports missing or stale
// resources deterministically.
type Plan struct {
	DesiredGeneration  int
	ObservedGeneration int
	Status             PlanStatus
	Missing            []contract.ResourceRef
	Stale              []contract.ResourceRef
}

// Compare builds a reconciliation plan from desired and observed state.
func Compare(desired DesiredState, observed ObservedState) Plan {
	plan := Plan{
		DesiredGeneration:  desired.Generation,
		ObservedGeneration: observed.Generation,
		Status:             PlanInSync,
	}

	desiredSet := make(map[contract.ResourceRef]struct{}, len(desired.Resources))
	observedSet := make(map[contract.ResourceRef]struct{}, len(observed.Resources))

	for _, ref := range desired.Resources {
		desiredSet[ref] = struct{}{}
	}
	for _, ref := range observed.Resources {
		observedSet[ref] = struct{}{}
	}

	for _, ref := range desired.Resources {
		if _, ok := observedSet[ref]; !ok {
			plan.Missing = append(plan.Missing, ref)
		}
	}
	for _, ref := range observed.Resources {
		if _, ok := desiredSet[ref]; !ok {
			plan.Stale = append(plan.Stale, ref)
		}
	}

	sort.Slice(plan.Missing, func(i, j int) bool {
		return resourceLess(plan.Missing[i], plan.Missing[j])
	})
	sort.Slice(plan.Stale, func(i, j int) bool {
		return resourceLess(plan.Stale[i], plan.Stale[j])
	})

	switch {
	case len(plan.Missing) == 0 && len(plan.Stale) == 0 && desired.Generation == observed.Generation:
		plan.Status = PlanInSync
	case len(plan.Missing) > 0 || len(plan.Stale) > 0:
		plan.Status = PlanDrifted
	default:
		plan.Status = PlanPending
	}

	return plan
}

func resourceLess(a, b contract.ResourceRef) bool {
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Version < b.Version
}

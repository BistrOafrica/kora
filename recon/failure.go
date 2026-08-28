package recon

import (
	"context"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/graph"
)

// PropagateFailure returns the resources that must transition to Waiting when
// the given resources fail: exactly their transitive dependents, computed from
// the dependency graph (RECON-004). Non-dependents are unaffected, satisfying
// the acceptance scenario "failed components put only dependents in waiting".
//
// It is pure — no mutation, no I/O — so the same input yields the same ordered
// waiting set after a restart (invariant 1).
func PropagateFailure(ctx context.Context, failed []contract.ResourceRef, g graph.DependencyGraph) ([]contract.ResourceRef, error) {
	analyzer := graph.NewImpactAnalyzer(g)
	waiting := map[contract.ResourceRef]bool{}
	order := make([]contract.ResourceRef, 0)

	for _, f := range failed {
		res, err := analyzer.Impact(ctx, graph.ImpactQuery{Target: f, MaxDepth: 20})
		if err != nil {
			return nil, err
		}
		for _, d := range res.Dependents {
			if !waiting[d.Resource] {
				waiting[d.Resource] = true
				order = append(order, d.Resource)
			}
		}
	}
	return order, nil
}

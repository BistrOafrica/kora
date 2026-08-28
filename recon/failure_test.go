package recon

import (
	"context"
	"testing"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/graph"
)

func TestPropagateFailureOnlyDependentsWaiting(t *testing.T) {
	g := graph.NewMemoryGraph()
	ctx := context.Background()
	// a fails; b depends on a, c depends on b; d is unrelated.
	_ = g.AddEdge(ctx, graph.Edge{From: ref("n", "b", 1), To: ref("n", "a", 1), Kind: graph.EdgeCapability})
	_ = g.AddEdge(ctx, graph.Edge{From: ref("n", "c", 1), To: ref("n", "b", 1), Kind: graph.EdgeCapability})
	_ = g.AddEdge(ctx, graph.Edge{From: ref("n", "d", 1), To: ref("n", "other", 1), Kind: graph.EdgeCapability})

	waiting, err := PropagateFailure(ctx, []contract.ResourceRef{ref("n", "a", 1)}, g)
	if err != nil {
		t.Fatalf("propagate: %v", err)
	}
	set := map[string]bool{}
	for _, r := range waiting {
		set[r.Name] = true
	}
	if !set["b"] || !set["c"] {
		t.Fatalf("dependents b,c should be waiting: %v", waiting)
	}
	if set["d"] {
		t.Fatalf("unrelated d must not be waiting: %v", waiting)
	}
	if set["a"] {
		t.Fatalf("failed resource itself should not be listed as waiting: %v", waiting)
	}
}

func TestPropagateFailureMultipleRootsDedup(t *testing.T) {
	g := graph.NewMemoryGraph()
	ctx := context.Background()
	// c depends on both a and b.
	_ = g.AddEdge(ctx, graph.Edge{From: ref("n", "c", 1), To: ref("n", "a", 1), Kind: graph.EdgeCapability})
	_ = g.AddEdge(ctx, graph.Edge{From: ref("n", "c", 1), To: ref("n", "b", 1), Kind: graph.EdgeCapability})

	waiting, err := PropagateFailure(ctx, []contract.ResourceRef{ref("n", "a", 1), ref("n", "b", 1)}, g)
	if err != nil {
		t.Fatalf("propagate: %v", err)
	}
	if len(waiting) != 1 || waiting[0].Name != "c" {
		t.Fatalf("c should appear exactly once: %v", waiting)
	}
}

func TestPropagateFailureNoDependents(t *testing.T) {
	g := graph.NewMemoryGraph()
	ctx := context.Background()
	waiting, err := PropagateFailure(ctx, []contract.ResourceRef{ref("n", "orphan", 1)}, g)
	if err != nil {
		t.Fatalf("propagate: %v", err)
	}
	if len(waiting) != 0 {
		t.Fatalf("expected empty waiting set: %v", waiting)
	}
}

package graph

import (
	"context"
	"errors"
	"testing"
)

func TestImpactFindsTransitiveDependents(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	// d is the provider; c depends on d, b depends on c, a depends on b.
	_ = g.AddEdge(ctx, Edge{From: ref("n", "c", 1), To: ref("n", "d", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "b", 1), To: ref("n", "c", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "a", 1), To: ref("n", "b", 1), Kind: EdgeLink})

	res, err := NewImpactAnalyzer(g).Impact(ctx, ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 5})
	if err != nil {
		t.Fatalf("impact: %v", err)
	}
	if len(res.Dependents) != 3 {
		t.Fatalf("dependents = %d, want 3: %+v", len(res.Dependents), res.Dependents)
	}
	depths := map[string]int{}
	for _, d := range res.Dependents {
		depths[d.Resource.Name] = d.Depth
	}
	if depths["c"] != 1 || depths["b"] != 2 || depths["a"] != 3 {
		t.Fatalf("depths = %v, want c=1 b=2 a=3", depths)
	}
	// Path for the deepest dependent should carry three edges.
	for _, d := range res.Dependents {
		if d.Resource.Name == "a" && len(d.Path) != 3 {
			t.Fatalf("deepest dependent path length = %d, want 3", len(d.Path))
		}
	}
}

func TestImpactRespectsMaxDepthAndSetsTruncated(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, Edge{From: ref("n", "c", 1), To: ref("n", "d", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "b", 1), To: ref("n", "c", 1), Kind: EdgeLink})

	res, err := NewImpactAnalyzer(g).Impact(ctx, ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 1})
	if err != nil {
		t.Fatalf("impact: %v", err)
	}
	if len(res.Dependents) != 1 || res.Dependents[0].Resource.Name != "c" {
		t.Fatalf("want only c at depth 1, got %+v", res.Dependents)
	}
	if !res.Truncated {
		t.Fatalf("expected Truncated=true at depth bound")
	}
}

func TestImpactKindFilterExcludesUnrelatedEdges(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, Edge{From: ref("n", "c", 1), To: ref("n", "d", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "b", 1), To: ref("n", "c", 1), Kind: EdgeCapability})

	res, err := NewImpactAnalyzer(g).Impact(ctx, ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 5, Kinds: []EdgeKind{EdgeLink}})
	if err != nil {
		t.Fatalf("impact: %v", err)
	}
	if len(res.Dependents) != 1 || res.Dependents[0].Resource.Name != "c" {
		t.Fatalf("kind filter leaked unrelated dependents: %+v", res.Dependents)
	}
}

func TestImpactInvalidDepth(t *testing.T) {
	g := NewMemoryGraph()
	_, err := NewImpactAnalyzer(g).Impact(context.Background(), ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 0})
	if !errors.Is(err, ErrInvalidDepth) {
		t.Fatalf("want ErrInvalidDepth, got %v", err)
	}
	_, err = NewImpactAnalyzer(g).Impact(context.Background(), ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 21})
	if !errors.Is(err, ErrInvalidDepth) {
		t.Fatalf("want ErrInvalidDepth for >20, got %v", err)
	}
}

func TestImpactEmptyWhenNoDependents(t *testing.T) {
	g := NewMemoryGraph()
	res, err := NewImpactAnalyzer(g).Impact(context.Background(), ImpactQuery{Target: ref("n", "orphan", 1), MaxDepth: 5})
	if err != nil {
		t.Fatalf("impact: %v", err)
	}
	if len(res.Dependents) != 0 || res.Truncated {
		t.Fatalf("expected empty non-truncated result: %+v", res)
	}
}

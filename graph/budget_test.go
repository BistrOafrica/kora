package graph

import (
	"context"
	"errors"
	"testing"
)

func TestQueryBudgetExceededTypedError(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, Edge{From: ref("n", "c", 1), To: ref("n", "d", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "b", 1), To: ref("n", "c", 1), Kind: EdgeLink})

	b := NewBoundedAnalyzer(NewImpactAnalyzer(g))
	_, err := b.ImpactPage(ctx, ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 5}, 1, "")
	var budget *ErrQueryBudgetExceeded
	if !errors.As(err, &budget) {
		t.Fatalf("want ErrQueryBudgetExceeded, got %T: %v", err, err)
	}
	if budget.Limit != 1 || budget.Observed != 2 {
		t.Fatalf("unexpected budget error: %+v", budget)
	}
}

func TestImpactPaginationWalksFullGraph(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, Edge{From: ref("n", "c", 1), To: ref("n", "d", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "b", 1), To: ref("n", "c", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "a", 1), To: ref("n", "b", 1), Kind: EdgeLink})

	b := NewBoundedAnalyzer(NewImpactAnalyzer(g))
	page, err := b.ImpactPage(ctx, ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 5}, 10, "")
	if err != nil {
		t.Fatalf("impact page: %v", err)
	}
	if len(page.Dependents) != 3 {
		t.Fatalf("page = %d dependents, want 3", len(page.Dependents))
	}
	if page.Stats.NodesVisited != 3 {
		t.Fatalf("stats = %+v", page.Stats)
	}
}

func TestImpactPageRespectsCursor(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, Edge{From: ref("n", "c", 1), To: ref("n", "d", 1), Kind: EdgeLink})
	_ = g.AddEdge(ctx, Edge{From: ref("n", "b", 1), To: ref("n", "c", 1), Kind: EdgeLink})

	b := NewBoundedAnalyzer(NewImpactAnalyzer(g))
	page, _ := b.ImpactPage(ctx, ImpactQuery{Target: ref("n", "d", 1), MaxDepth: 5}, 10, "1")
	if len(page.Dependents) != 1 {
		t.Fatalf("cursor offset 1 should return 1 dependent, got %d", len(page.Dependents))
	}
}

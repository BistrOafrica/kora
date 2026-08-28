package graph

import (
	"context"
	"errors"
	"testing"

	"github.com/asenawritescode/kora/contract"
)

func ref(ns, name string, v int) contract.ResourceRef {
	return contract.ResourceRef{Namespace: ns, Name: name, Version: v}
}

func edge(from, to contract.ResourceRef, kind EdgeKind) Edge {
	return Edge{From: from, To: to, Kind: kind}
}

func TestCycleDetectionRejectsSelfAndMutualDependency(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()

	// Self-dependency.
	if err := g.AddEdge(ctx, edge(ref("n", "a", 1), ref("n", "a", 1), EdgeLink)); err == nil {
		t.Fatalf("self-dependency accepted")
	} else {
		var cyc *ErrCycleDetected
		if !errors.As(err, &cyc) {
			t.Fatalf("want *ErrCycleDetected, got %T: %v", err, err)
		}
	}

	// Mutual dependency: a -> b, then b -> a must be rejected.
	if err := g.AddEdge(ctx, edge(ref("n", "a", 1), ref("n", "b", 1), EdgeLink)); err != nil {
		t.Fatalf("a->b: %v", err)
	}
	err := g.AddEdge(ctx, edge(ref("n", "b", 1), ref("n", "a", 1), EdgeLink))
	if err == nil {
		t.Fatalf("mutual dependency accepted")
	}
	var cyc *ErrCycleDetected
	if !errors.As(err, &cyc) {
		t.Fatalf("want *ErrCycleDetected, got %T: %v", err, err)
	}
	if len(cyc.Path) != 3 {
		t.Fatalf("cycle path length = %d, want 3 (b→a→b): %v", len(cyc.Path), cyc.Path)
	}
	if cyc.Path[0] != cyc.Path[len(cyc.Path)-1] {
		t.Fatalf("cycle path not closed: %v", cyc.Path)
	}
}

func TestCycleDetectionLeavesGraphUnchanged(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, edge(ref("n", "a", 1), ref("n", "b", 1), EdgeLink))
	if err := g.AddEdge(ctx, edge(ref("n", "b", 1), ref("n", "a", 1), EdgeLink)); err == nil {
		t.Fatalf("expected rejection")
	}
	// Rejected edge must not have been added.
	if es, _ := g.EdgesFrom(ref("n", "b", 1)); len(es) != 0 {
		t.Fatalf("rejected edge was persisted: %v", es)
	}
}

func TestTopologicalOrderPlacesProvidersFirst(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	// animal depends on (is served by) sms and email; sms depends on nothing.
	_ = g.AddEdge(ctx, edge(ref("n", "animal", 1), ref("n", "sms", 1), EdgeCapability))
	_ = g.AddEdge(ctx, edge(ref("n", "animal", 1), ref("n", "email", 1), EdgeCapability))

	order, err := g.TopologicalOrder("n")
	if err != nil {
		t.Fatalf("topological order: %v", err)
	}
	idx := map[contract.ResourceRef]int{}
	for i, r := range order {
		idx[r] = i
	}
	if idx[ref("n", "sms", 1)] >= idx[ref("n", "animal", 1)] {
		t.Fatalf("provider sms must precede dependent animal: %v", order)
	}
	if idx[ref("n", "email", 1)] >= idx[ref("n", "animal", 1)] {
		t.Fatalf("provider email must precede dependent animal: %v", order)
	}
}

func TestReverseLookupFindsAllDependents(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, edge(ref("n", "animal", 1), ref("n", "sms", 1), EdgeCapability))
	_ = g.AddEdge(ctx, edge(ref("n", "invoice", 1), ref("n", "sms", 1), EdgeCapability))

	deps, err := g.EdgesTo(ref("n", "sms", 1))
	if err != nil {
		t.Fatalf("edgesTo: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("dependent count = %d, want 2", len(deps))
	}
}

func TestTenantIsolationInEdgeQueries(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	_ = g.AddEdge(ctx, edge(ref("tenant-a", "animal", 1), ref("tenant-a", "sms", 1), EdgeCapability))
	_ = g.AddEdge(ctx, edge(ref("tenant-b", "animal", 1), ref("tenant-b", "sms", 1), EdgeCapability))

	// Reverse lookup in tenant-a must not surface tenant-b dependents.
	deps, _ := g.EdgesTo(ref("tenant-a", "sms", 1))
	for _, d := range deps {
		if d.From.Namespace != "tenant-a" {
			t.Fatalf("cross-tenant dependent leaked: %v", d)
		}
	}

	order, err := g.TopologicalOrder("tenant-a")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	for _, r := range order {
		if r.Namespace != "tenant-a" {
			t.Fatalf("cross-tenant resource in order: %v", r)
		}
	}
}

func TestCrossNamespaceEdgeRejected(t *testing.T) {
	g := NewMemoryGraph()
	ctx := context.Background()
	err := g.AddEdge(ctx, edge(ref("a", "x", 1), ref("b", "y", 1), EdgeLink))
	if !errors.Is(err, contract.ErrResourceNamespaceRequired) {
		t.Fatalf("want ErrResourceNamespaceRequired, got %v", err)
	}
}

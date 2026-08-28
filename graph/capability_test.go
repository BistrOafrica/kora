package graph

import (
	"context"
	"testing"

	"github.com/asenawritescode/kora/contract"
)

func TestRemovingCapabilityDegradesOnlyDependents(t *testing.T) {
	// Resource dependency graph: dashboard depends on animal.
	deps := NewMemoryGraph()
	ctx := context.Background()
	_ = deps.AddEdge(ctx, Edge{From: ref("n", "dashboard", 1), To: ref("n", "animal", 1), Kind: EdgeLink})

	g := NewCapabilityGraph(deps)
	sms := CapabilityID{Namespace: "sms", Name: "send", Version: 1}
	_ = g.DeclareConsumer(ref("n", "animal", 1), sms)
	_ = g.DeclareConsumer(ref("n", "invoice", 1), sms)

	res, err := g.DegradeSet(sms)
	if err != nil {
		t.Fatalf("degrade: %v", err)
	}
	set := map[string]bool{}
	for _, d := range res.Dependents {
		set[d.Resource.Name] = true
	}
	if !set["animal"] || !set["invoice"] || !set["dashboard"] {
		t.Fatalf("degrade set missing dependents: %v", set)
	}
	if set["report"] {
		t.Fatalf("non-consumer report must not degrade")
	}
}

func TestConsumersOfReturnsCompleteSet(t *testing.T) {
	g := NewCapabilityGraph(nil)
	sms := CapabilityID{Namespace: "sms", Name: "send", Version: 1}
	_ = g.DeclareConsumer(ref("n", "animal", 1), sms)
	_ = g.DeclareConsumer(ref("n", "invoice", 1), sms)
	_ = g.DeclareConsumer(ref("n", "alert", 1), CapabilityID{Namespace: "email", Name: "send", Version: 1})

	consumers, err := g.ConsumersOf(sms)
	if err != nil {
		t.Fatalf("consumersOf: %v", err)
	}
	if len(consumers) != 2 {
		t.Fatalf("consumers = %d, want 2", len(consumers))
	}
}

func TestDegradeSetEmptyWhenNoConsumers(t *testing.T) {
	g := NewCapabilityGraph(nil)
	res, err := g.DegradeSet(CapabilityID{Namespace: "sms", Name: "send", Version: 1})
	if err != nil {
		t.Fatalf("degrade: %v", err)
	}
	if len(res.Dependents) != 0 {
		t.Fatalf("expected empty degrade set: %+v", res.Dependents)
	}
}

func TestCapabilityIDString(t *testing.T) {
	id := CapabilityID{Namespace: "sms", Name: "send", Version: 1}
	if id.String() != "sms.send@1" {
		t.Fatalf("String = %q", id.String())
	}
}

// Guard: contract.ResourceRef is still the edge node type; capability graph
// must not require a specific provider implementation.
func TestCapabilityGraphIsProviderNeutral(t *testing.T) {
	g := NewCapabilityGraph(nil)
	_ = g.DeclareConsumer(contract.ResourceRef{Namespace: "n", Name: "x", Version: 1}, CapabilityID{Namespace: "storage", Name: "blob", Version: 2})
	if _, err := g.ConsumersOf(CapabilityID{Namespace: "storage", Name: "blob", Version: 2}); err != nil {
		t.Fatalf("provider-neutral graph failed: %v", err)
	}
}

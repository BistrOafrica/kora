package graph

import (
	"context"
	"sort"
	"strconv"

	"github.com/asenawritescode/kora/contract"
)

// CapabilityID is the stable identity of a capability (GRAPH-004). Namespace is
// the capability category ("sms", "storage", "events"); it is not a tenant.
type CapabilityID struct {
	Namespace string
	Name      string
	Version   int
}

func (c CapabilityID) String() string {
	return c.Namespace + "." + c.Name + "@" + strconv.Itoa(c.Version)
}

// CapabilityGraph records which resources consume which capabilities and
// computes the degrade set when a capability is removed or fails.
type CapabilityGraph interface {
	DeclareConsumer(consumer contract.ResourceRef, cap CapabilityID) error
	ConsumersOf(cap CapabilityID) ([]contract.ResourceRef, error)
	DegradeSet(cap CapabilityID) (ImpactResult, error)
}

// capGraph implements CapabilityGraph over a resource DependencyGraph for
// transitive impact.
type capGraph struct {
	consumers map[CapabilityID]map[contract.ResourceRef]bool
	deps      DependencyGraph
}

// NewCapabilityGraph returns a capability graph. deps supplies the resource
// dependency graph used to compute transitive degradation.
func NewCapabilityGraph(deps DependencyGraph) CapabilityGraph {
	if deps == nil {
		deps = NewMemoryGraph()
	}
	return &capGraph{
		consumers: make(map[CapabilityID]map[contract.ResourceRef]bool),
		deps:      deps,
	}
}

// DeclareConsumer records that consumer depends on cap. Idempotent.
func (g *capGraph) DeclareConsumer(consumer contract.ResourceRef, cap CapabilityID) error {
	if g.consumers[cap] == nil {
		g.consumers[cap] = make(map[contract.ResourceRef]bool)
	}
	g.consumers[cap][consumer] = true
	return nil
}

// ConsumersOf returns the direct consumers of cap, sorted deterministically.
func (g *capGraph) ConsumersOf(cap CapabilityID) ([]contract.ResourceRef, error) {
	set := g.consumers[cap]
	out := make([]contract.ResourceRef, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return refLess(out[i], out[j]) })
	return out, nil
}

// DegradeSet returns every resource degraded when cap is removed: the direct
// consumers plus their transitive dependents, deduplicated and deterministic.
// Non-consumers are unaffected.
func (g *capGraph) DegradeSet(cap CapabilityID) (ImpactResult, error) {
	analyzer := NewImpactAnalyzer(g.deps)
	result := ImpactResult{}
	seen := map[contract.ResourceRef]bool{}

	consumers := g.consumers[cap]
	ordered := make([]contract.ResourceRef, 0, len(consumers))
	for r := range consumers {
		ordered = append(ordered, r)
	}
	sort.Slice(ordered, func(i, j int) bool { return refLess(ordered[i], ordered[j]) })

	for _, consumer := range ordered {
		if !seen[consumer] {
			seen[consumer] = true
			result.Dependents = append(result.Dependents, DependentPath{Resource: consumer, Depth: 1})
		}
		impact, err := analyzer.Impact(context.Background(), ImpactQuery{Target: consumer, MaxDepth: 20})
		if err != nil {
			return ImpactResult{}, err
		}
		for _, d := range impact.Dependents {
			if !seen[d.Resource] {
				seen[d.Resource] = true
				result.Dependents = append(result.Dependents, DependentPath{Resource: d.Resource, Depth: d.Depth + 1})
			}
		}
	}
	return result, nil
}

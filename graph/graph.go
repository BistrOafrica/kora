package graph

import (
	"context"
	"sort"
	"sync"

	"github.com/asenawritescode/kora/contract"
)

// MemoryGraph is an in-memory, concurrency-safe DependencyGraph (GRAPH-002).
// It rejects self- and mutual-dependency cycles with a path-bearing typed
// error and provides deterministic topological ordering (dependencies before
// dependents). It is the reference implementation for the SQL-backed edge
// store, mirroring Memory (GRAPH-001).
type MemoryGraph struct {
	mu   sync.RWMutex
	from map[contract.ResourceRef][]Edge // outgoing edges by From
	to   map[contract.ResourceRef][]Edge // incoming edges by To
}

// NewMemoryGraph returns an empty in-memory dependency graph.
func NewMemoryGraph() *MemoryGraph {
	return &MemoryGraph{
		from: make(map[contract.ResourceRef][]Edge),
		to:   make(map[contract.ResourceRef][]Edge),
	}
}

// AddEdge registers e after proving it does not close a cycle. Adding an edge
// whose To can already reach From would create From → … → To → … → From.
func (g *MemoryGraph) AddEdge(ctx context.Context, e Edge) error {
	if e.From.Namespace != e.To.Namespace {
		// Cross-namespace edges are a modeling error: dependencies are
		// explainable only within one scope. Fail closed.
		return contract.ErrResourceNamespaceRequired
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Self-dependency is always a cycle.
	if e.From == e.To {
		return &ErrCycleDetected{Path: []contract.ResourceRef{e.From, e.To}}
	}

	// Does To already reach From? If so, this edge closes a cycle.
	if path, ok := g.reachesLocked(e.To, e.From); ok {
		cycle := append([]contract.ResourceRef{e.From}, path...)
		return &ErrCycleDetected{Path: cycle}
	}

	g.from[e.From] = append(g.from[e.From], e)
	g.to[e.To] = append(g.to[e.To], e)
	return nil
}

// reachesLocked reports whether start can reach target through existing edges,
// returning the path (start → … → target) when reachable. Caller holds mu.
func (g *MemoryGraph) reachesLocked(start, target contract.ResourceRef) ([]contract.ResourceRef, bool) {
	type frame struct {
		ref  contract.ResourceRef
		path []contract.ResourceRef
	}
	stack := []frame{{ref: start, path: []contract.ResourceRef{start}}}
	visited := map[contract.ResourceRef]bool{}

	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[f.ref] {
			continue
		}
		visited[f.ref] = true
		for _, e := range g.from[f.ref] {
			if e.To == target {
				return append(append([]contract.ResourceRef{}, f.path...), e.To), true
			}
			np := append(append([]contract.ResourceRef{}, f.path...), e.To)
			stack = append(stack, frame{ref: e.To, path: np})
		}
	}
	return nil, false
}

// EdgesFrom returns all outgoing edges for ref (things ref depends on).
func (g *MemoryGraph) EdgesFrom(ref contract.ResourceRef) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return cloneEdges(g.from[ref]), nil
}

// EdgesTo returns all incoming edges for ref (things that depend on ref).
func (g *MemoryGraph) EdgesTo(ref contract.ResourceRef) ([]Edge, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return cloneEdges(g.to[ref]), nil
}

// TopologicalOrder returns the resources in namespace in dependency order:
// dependencies (To) precede dependents (From). Deterministic for a fixed
// graph; ties break by resource name then version.
func (g *MemoryGraph) TopologicalOrder(namespace string) ([]contract.ResourceRef, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	inDegree := map[contract.ResourceRef]int{}
	nodes := map[contract.ResourceRef]bool{}
	adj := map[contract.ResourceRef][]contract.ResourceRef{}

	for from, edges := range g.from {
		if from.Namespace != namespace {
			continue
		}
		nodes[from] = true
		for _, e := range edges {
			// e.From depends on e.To, so e.To must precede e.From.
			adj[e.To] = append(adj[e.To], e.From)
			inDegree[e.From]++
			nodes[e.To] = true
		}
	}

	// Nodes with zero in-degree have no dependencies; they are providers.
	var queue []contract.ResourceRef
	for n := range nodes {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}

	var order []contract.ResourceRef
	for len(queue) > 0 {
		sort.Slice(queue, func(i, j int) bool { return refLess(queue[i], queue[j]) })
		n := queue[0]
		queue = queue[1:]
		order = append(order, n)
		for _, m := range adj[n] {
			inDegree[m]--
			if inDegree[m] == 0 {
				queue = append(queue, m)
			}
		}
	}

	if len(order) != len(nodes) {
		return nil, &ErrCycleDetected{Path: order} // cycle prevents a full order
	}
	return order, nil
}

func refLess(a, b contract.ResourceRef) bool {
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.Version != b.Version {
		return a.Version < b.Version
	}
	return a.Namespace < b.Namespace
}

func cloneEdges(in []Edge) []Edge {
	out := make([]Edge, len(in))
	copy(out, in)
	return out
}

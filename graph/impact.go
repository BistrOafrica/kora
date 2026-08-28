package graph

import (
	"context"
	"errors"
	"sort"

	"github.com/asenawritescode/kora/contract"
)

// ImpactQuery asks "what transitively depends on Target, out to MaxDepth, via
// these edge kinds". It is the programmatic form of the destructive-change
// dependency report (GRAPH-003).
type ImpactQuery struct {
	Target   contract.ResourceRef
	MaxDepth int        // required, 1..20
	Kinds    []EdgeKind // empty means all kinds
}

// ImpactResult reports transitive dependents with explanatory paths.
type ImpactResult struct {
	Dependents []DependentPath
	Truncated  bool // traversal hit MaxDepth with more nodes remaining
}

// DependentPath is one transitive dependent and the edge chain from the target
// to it. Path[i].From depends on Path[i].To; Path[0].To is the target's direct
// dependency edge target, so Path reads from the queried resource outward.
type DependentPath struct {
	Resource contract.ResourceRef
	Path     []Edge
	Depth    int
}

// ErrInvalidDepth is a typed error for an out-of-range MaxDepth.
var ErrInvalidDepth = errors.New("graph: impact MaxDepth must be 1..20")

// ImpactAnalyzer computes transitive dependency impact.
type ImpactAnalyzer interface {
	Impact(ctx context.Context, q ImpactQuery) (ImpactResult, error)
}

// impactAnalyzer is the in-memory implementation over a DependencyGraph. It
// performs an iterative BFS in Go (no recursive CTE), so results are identical
// across dialects — matching the libSQL fallback path required by the spec.
type impactAnalyzer struct {
	graph DependencyGraph
}

// NewImpactAnalyzer returns an analyzer over g.
func NewImpactAnalyzer(g DependencyGraph) ImpactAnalyzer {
	return &impactAnalyzer{graph: g}
}

func (a *impactAnalyzer) Impact(ctx context.Context, q ImpactQuery) (ImpactResult, error) {
	if q.MaxDepth < 1 || q.MaxDepth > 20 {
		return ImpactResult{}, ErrInvalidDepth
	}

	kindSet := make(map[EdgeKind]bool, len(q.Kinds))
	for _, k := range q.Kinds {
		kindSet[k] = true
	}
	allowed := func(e Edge) bool {
		return len(kindSet) == 0 || kindSet[e.Kind]
	}

	type frame struct {
		ref   contract.ResourceRef
		path  []Edge
		depth int
	}

	result := ImpactResult{}
	visited := map[contract.ResourceRef]bool{q.Target: true}
	queue := []frame{{ref: q.Target, depth: 0}}

	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]

		if f.depth >= q.MaxDepth {
			result.Truncated = true
			continue
		}

		incoming, _ := a.graph.EdgesTo(f.ref)
		incoming = filterEdges(incoming, allowed)
		sort.Slice(incoming, func(i, j int) bool { return refLess(incoming[i].From, incoming[j].From) })

		for _, e := range incoming {
			dep := e.From
			if visited[dep] {
				continue
			}
			visited[dep] = true
			path := append(append([]Edge{}, f.path...), e)
			result.Dependents = append(result.Dependents, DependentPath{
				Resource: dep,
				Path:     path,
				Depth:    f.depth + 1,
			})
			queue = append(queue, frame{ref: dep, path: path, depth: f.depth + 1})
		}
	}

	// Deterministic order by (depth, namespace, name).
	sort.Slice(result.Dependents, func(i, j int) bool {
		a, b := result.Dependents[i], result.Dependents[j]
		if a.Depth != b.Depth {
			return a.Depth < b.Depth
		}
		return refLess(a.Resource, b.Resource)
	})
	return result, nil
}

func filterEdges(in []Edge, keep func(Edge) bool) []Edge {
	out := make([]Edge, 0, len(in))
	for _, e := range in {
		if keep(e) {
			out = append(out, e)
		}
	}
	return out
}

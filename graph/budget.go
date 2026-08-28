package graph

import (
	"context"
	"fmt"
	"strconv"
)

// QueryStats reports the cost of an impact query (GRAPH-005).
type QueryStats struct {
	NodesVisited int
	DurationMs   int64
	CacheHit     bool
}

// ErrQueryBudgetExceeded is a typed overrun error carrying the limit and the
// observed node count.
type ErrQueryBudgetExceeded struct {
	Limit    int
	Observed int
}

func (e *ErrQueryBudgetExceeded) Error() string {
	return fmt.Sprintf("graph impact query budget exceeded: limit %d, observed %d", e.Limit, e.Observed)
}

// ImpactPage is one cursor-paginated page of an impact query.
type ImpactPage struct {
	Dependents []DependentPath
	NextCursor string
	Truncated  bool
	Stats      QueryStats
}

// BoundedAnalyzer is the bounded, paginated impact surface (GRAPH-005). maxNodes
// caps the total dependent set; cursor resumes from a prior page offset.
type BoundedAnalyzer interface {
	ImpactPage(ctx context.Context, q ImpactQuery, maxNodes int, cursor string) (ImpactPage, error)
}

// boundedAnalyzer wraps ImpactAnalyzer with a node budget and cursor pagination.
type boundedAnalyzer struct {
	inner ImpactAnalyzer
}

// NewBoundedAnalyzer returns a bounded analyzer over inner.
func NewBoundedAnalyzer(inner ImpactAnalyzer) BoundedAnalyzer {
	return &boundedAnalyzer{inner: inner}
}

func (b *boundedAnalyzer) ImpactPage(ctx context.Context, q ImpactQuery, maxNodes int, cursor string) (ImpactPage, error) {
	offset := 0
	if cursor != "" {
		o, err := strconv.Atoi(cursor)
		if err != nil || o < 0 {
			return ImpactPage{}, fmt.Errorf("graph: invalid impact cursor %q", cursor)
		}
		offset = o
	}

	res, err := b.inner.Impact(ctx, q)
	if err != nil {
		return ImpactPage{}, err
	}

	if maxNodes > 0 && len(res.Dependents) > maxNodes {
		return ImpactPage{}, &ErrQueryBudgetExceeded{Limit: maxNodes, Observed: len(res.Dependents)}
	}

	page := ImpactPage{
		Stats:      QueryStats{NodesVisited: len(res.Dependents)},
		Truncated:  res.Truncated,
		Dependents: []DependentPath{},
	}
	if offset >= len(res.Dependents) {
		return page, nil
	}

	page.Dependents = res.Dependents[offset:]
	page.NextCursor = strconv.Itoa(len(res.Dependents))
	return page, nil
}

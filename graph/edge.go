package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/asenawritescode/kora/contract"
)

// EdgeKind classifies the relationship an edge represents. The value is stable
// and used for telemetry labels and impact classification.
type EdgeKind string

const (
	EdgeLink        EdgeKind = "link"
	EdgeTable       EdgeKind = "table"
	EdgeComputed    EdgeKind = "computed"
	EdgeLinkedField EdgeKind = "linked_field"
	EdgeWorkflow    EdgeKind = "workflow"
	EdgeCapability  EdgeKind = "capability"
)

// Edge is one explainable dependency: From depends on To (declared via Via).
type Edge struct {
	From contract.ResourceRef `json:"from"`
	To   contract.ResourceRef `json:"to"`
	Kind EdgeKind             `json:"kind"`
	Via  string               `json:"via,omitempty"`
}

// ErrCycleDetected is a typed error returned when adding an edge would close a
// dependency cycle. Path carries the full cycle (From → … → From) so callers
// can explain the rejection without re-deriving it.
type ErrCycleDetected struct {
	Path []contract.ResourceRef
}

func (e *ErrCycleDetected) Error() string {
	names := make([]string, len(e.Path))
	for i, r := range e.Path {
		names[i] = r.String()
	}
	return fmt.Sprintf("dependency cycle detected: %s", strings.Join(names, " → "))
}

// DependencyGraph is the queryable edge store for explainable dependencies
// (GRAPH-002). Implementations enforce cycle-free registration.
type DependencyGraph interface {
	AddEdge(ctx context.Context, e Edge) error
	EdgesFrom(ref contract.ResourceRef) ([]Edge, error)
	EdgesTo(ref contract.ResourceRef) ([]Edge, error)
	TopologicalOrder(namespace string) ([]contract.ResourceRef, error)
}

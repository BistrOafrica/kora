// Package fieldtype implements the semantic field-type registry (GRAPH-007).
//
// Components contribute semantic field types (e.g. "accounting.money",
// "livestock.body_condition_score") that map to a base storage type plus
// optional validation and a renderer hint. Storage stays base-typed, so SQL
// remains dialect-simple; the engine kernel stays domain-neutral (no
// domain-specific type names appear in engine code).
package fieldtype

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Descriptor is one registered field type.
type Descriptor struct {
	Name     string          // e.g. "accounting.money" or "core.decimal"
	Base     string          // base storage type name, e.g. "core.decimal"
	Renderer string          // UI hint, optional; renderers fall back on base
	Validate func(any) error // optional per-value validation (nil = none)
}

// Typed registry sentinels. Callers match via errors.Is, never string
// matching.
var (
	ErrDuplicateFieldType = errors.New("fieldtype: duplicate field type")
	ErrUnknownBaseType    = errors.New("fieldtype: unknown base type")
	ErrFieldTypeNotFound  = errors.New("fieldtype: field type not found")
)

// coreBaseTypes are the built-in storage types, registered under the "core."
// prefix. Semantic types map onto these; no component may redefine them.
var coreBaseTypes = []Descriptor{
	{Name: "core.text"},
	{Name: "core.decimal"},
	{Name: "core.int"},
	{Name: "core.float"},
	{Name: "core.bool"},
	{Name: "core.date"},
}

// Registry is a concurrency-safe field-type registry. It is generation-scoped:
// a full Registry is swapped atomically on generation activation so rollback
// restores the prior type set.
type Registry struct {
	mu    sync.RWMutex
	types map[string]Descriptor
}

// NewRegistry returns a registry pre-populated with the core base types.
func NewRegistry() *Registry {
	r := &Registry{types: make(map[string]Descriptor)}
	for _, d := range coreBaseTypes {
		r.types[d.Name] = d
	}
	return r
}

// Register adds a semantic type. Name must be namespaced (contain a dot) and
// unique; Base must already be registered (a core base type or a previously
// registered semantic type). Failures are typed and never silent.
func (r *Registry) Register(d Descriptor) error {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		return fmt.Errorf("fieldtype: name is required")
	}
	if !strings.Contains(d.Name, ".") {
		return fmt.Errorf("fieldtype: name %q must be namespaced (e.g. accounting.money)", d.Name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.types[d.Name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateFieldType, d.Name)
	}
	if d.Base != "" {
		if _, ok := r.types[d.Base]; !ok {
			return fmt.Errorf("%w: %s", ErrUnknownBaseType, d.Base)
		}
	}
	r.types[d.Name] = d
	return nil
}

// Resolve returns a field type by name and whether it exists.
func (r *Registry) Resolve(name string) (Descriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.types[name]
	return d, ok
}

// List returns all registered types sorted by name.
func (r *Registry) List() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Descriptor, 0, len(r.types))
	for _, d := range r.types {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

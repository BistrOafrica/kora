// Package component defines the typed component manifest (PLUGIN-004):
// what a component provides, what it requires, and which events it emits and
// handles. The normalized typed model is authoritative; YAML and JSON are
// lossless interchange only, and validation fails at load time rather than at
// runtime.
package component

import (
	"fmt"
	"regexp"
	"strings"
)

// Runtime identifies the execution substrate for a component.
type Runtime string

const (
	RuntimeGoja   Runtime = "goja"
	RuntimeWasm   Runtime = "wasm"
	RuntimeNative Runtime = "native"
)

// BindingStrategy describes how a component should be bound to capability
// providers when a runtime resolves its requirements.
type BindingStrategy string

const (
	BindingStrategyDynamic   BindingStrategy = "dynamic"
	BindingStrategyPerOp     BindingStrategy = "per-operation"
	BindingStrategyPerReq    BindingStrategy = "per-request"
	BindingStrategyPerTenant BindingStrategy = "per-tenant"
	BindingStrategyFixed     BindingStrategy = "fixed"
)

// Lifecycle declares the expected runtime lifecycle posture for the component.
type Lifecycle string

const (
	LifecycleActive       Lifecycle = "active"
	LifecycleDegraded     Lifecycle = "degraded"
	LifecycleWaiting      Lifecycle = "waiting"
	LifecycleStopping     Lifecycle = "stopping"
	LifecycleInactive     Lifecycle = "inactive"
	LifecycleExperimental Lifecycle = "experimental"
)

// Manifest is the typed component definition.
type Manifest struct {
	Name            string          `yaml:"name" json:"name"`
	Version         string          `yaml:"version" json:"version"`
	Runtime         Runtime         `yaml:"runtime" json:"runtime"`
	BindingStrategy BindingStrategy `yaml:"binding_strategy,omitempty" json:"binding_strategy,omitempty"`
	Lifecycle       Lifecycle       `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty"`
	Provides        []string        `yaml:"provides" json:"provides,omitempty"`
	Requires        []string        `yaml:"requires" json:"requires,omitempty"`
	Events          EventsDecl      `yaml:"events" json:"events,omitempty"`
}

// EventsDecl declares the event types a component emits and handles.
type EventsDecl struct {
	Emits   []EventDecl `yaml:"emits" json:"emits,omitempty"`
	Handles []EventDecl `yaml:"handles" json:"handles,omitempty"`
}

// EventDecl references one event envelope type and its schema version.
type EventDecl struct {
	Type    string `yaml:"type" json:"type"`
	Version int    `yaml:"version" json:"version"`
}

// ErrManifestInvalid is a typed validation failure carrying the key path, a
// human detail, and an optional "did you mean" suggestion.
type ErrManifestInvalid struct {
	Path       string
	Detail     string
	Suggestion string
}

func (e *ErrManifestInvalid) Error() string {
	msg := fmt.Sprintf("manifest %s: %s", e.Path, e.Detail)
	if e.Suggestion != "" {
		msg += fmt.Sprintf(" (did you mean %q?)", e.Suggestion)
	}
	return msg
}

// EventTypeRegistry reports whether an event envelope type is registered.
// Callers pass the canonical event registry; unknown types fail validation.
type EventTypeRegistry func(eventType string) bool

var (
	nameRe   = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)
	semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
)

// Validate checks name, version, runtime, capability duplicates, and event
// types. It returns the first failure as a typed *ErrManifestInvalid.
func (m Manifest) Validate(events EventTypeRegistry) error {
	if strings.TrimSpace(m.Name) == "" {
		return &ErrManifestInvalid{Path: "name", Detail: "is required"}
	}
	if !nameRe.MatchString(m.Name) {
		return &ErrManifestInvalid{Path: "name", Detail: fmt.Sprintf("must be DNS-like (got %q)", m.Name)}
	}
	if strings.TrimSpace(m.Version) == "" {
		return &ErrManifestInvalid{Path: "version", Detail: "is required"}
	}
	if !semverRe.MatchString(m.Version) {
		return &ErrManifestInvalid{Path: "version", Detail: fmt.Sprintf("must be semver (got %q)", m.Version)}
	}
	switch m.Runtime {
	case RuntimeGoja, RuntimeWasm, RuntimeNative:
	case "":
		return &ErrManifestInvalid{Path: "runtime", Detail: "is required"}
	default:
		return &ErrManifestInvalid{Path: "runtime", Detail: fmt.Sprintf("unknown runtime %q", m.Runtime), Suggestion: "goja"}
	}
	switch m.BindingStrategy {
	case "", BindingStrategyDynamic, BindingStrategyPerOp, BindingStrategyPerReq, BindingStrategyPerTenant, BindingStrategyFixed:
	default:
		return &ErrManifestInvalid{Path: "binding_strategy", Detail: fmt.Sprintf("unknown binding strategy %q", m.BindingStrategy), Suggestion: string(BindingStrategyPerOp)}
	}
	switch m.Lifecycle {
	case "", LifecycleActive, LifecycleDegraded, LifecycleWaiting, LifecycleStopping, LifecycleInactive, LifecycleExperimental:
	default:
		return &ErrManifestInvalid{Path: "lifecycle", Detail: fmt.Sprintf("unknown lifecycle %q", m.Lifecycle), Suggestion: string(LifecycleActive)}
	}

	if err := checkDuplicates("provides", "requires", m.Provides, m.Requires); err != nil {
		return err
	}

	if events != nil {
		for _, e := range append(append([]EventDecl{}, m.Events.Emits...), m.Events.Handles...) {
			if !events(e.Type) {
				return &ErrManifestInvalid{Path: "events", Detail: fmt.Sprintf("event type %q not registered", e.Type)}
			}
		}
	}
	return nil
}

func checkDuplicates(providePath, requirePath string, provides, requires []string) error {
	seen := map[string]string{}
	for _, c := range provides {
		if _, ok := seen[c]; ok {
			return &ErrManifestInvalid{Path: providePath, Detail: fmt.Sprintf("capability %q duplicated", c)}
		}
		seen[c] = providePath
	}
	for _, c := range requires {
		if prior, ok := seen[c]; ok {
			return &ErrManifestInvalid{Path: requirePath, Detail: fmt.Sprintf("capability %q already declared in %s", c, prior)}
		}
		seen[c] = requirePath
	}
	return nil
}

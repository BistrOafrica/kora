package plugin

import (
	"encoding/json"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/effect"
)

// StubProvider is an in-process fake of a WASM-like runtime (PLUGIN-008). It
// enforces the seam's serialization-only rule: Execute returns JSON-encoded
// bytes and never passes host pointers or closures across the boundary. It
// proves the RuntimeProvider contract is implementable by a non-Goja backend.
type StubProvider struct {
	id       string
	compiled map[string]ComponentUnit
	required contract.CapabilityName
}

// NewStubProvider returns a stub runtime with the given ID. required is the
// capability Execute insists on (deny-by-default at the seam).
func NewStubProvider(id string, required contract.CapabilityName) *StubProvider {
	return &StubProvider{id: id, compiled: map[string]ComponentUnit{}, required: required}
}

func (s *StubProvider) ID() string { return s.id }

func (s *StubProvider) Compile(unit ComponentUnit) error {
	if unit.Runtime != "" && unit.Runtime != s.id {
		return ErrRuntimeUnsupported
	}
	if err := VerifyChecksum(unit); err != nil {
		return err
	}
	s.compiled[unit.Name] = unit
	return nil
}

func (s *StubProvider) Execute(ctx contract.OperationContext, call InvokeCall, handles []contract.CapabilityHandle) (InvokeResult, error) {
	if s.required != "" && !hasCapability(handles, s.required) {
		return InvokeResult{}, ErrCapabilityNotGranted
	}
	// Serialize at the seam: the result is JSON bytes, never a host pointer.
	out := map[string]any{
		"function": call.Function,
		"runtime":  s.id,
		"args":     json.RawMessage(call.Args),
	}
	value, err := json.Marshal(out)
	if err != nil {
		return InvokeResult{}, err
	}
	return InvokeResult{
		Value:    value,
		Evidence: effect.CleanupEvidence{Owner: "stub:" + s.id, ScopeID: contract.NewID()},
	}, nil
}

func (s *StubProvider) Shutdown(ctx contract.OperationContext) error {
	s.compiled = map[string]ComponentUnit{}
	return nil
}

package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/effect"
)

// This file defines the runtime-neutral provider seam (PLUGIN-008). Any
// execution backend — Goja today, WASM or native later — implements
// RuntimeProvider. Values crossing the seam are serialized; no Go closures,
// host pointers, or shared maps may cross it.

// ComponentUnit is the artifact + manifest a runtime compiles.
type ComponentUnit struct {
	Name     string
	Runtime  string
	Artifact []byte
	Checksum string
}

// InvokeCall is one serialized invocation request.
type InvokeCall struct {
	Function string
	Args     []byte
}

// InvokeResult is a serialized result plus cleanup evidence.
type InvokeResult struct {
	Value    []byte
	Evidence effect.CleanupEvidence
}

// RuntimeProvider is the seam any execution backend implements.
type RuntimeProvider interface {
	ID() string
	Compile(unit ComponentUnit) error
	Execute(ctx contract.OperationContext, call InvokeCall, handles []contract.CapabilityHandle) (InvokeResult, error)
	Shutdown(ctx contract.OperationContext) error
}

// Typed seam errors. Callers match via errors.Is/As, never string matching.
var (
	ErrRuntimeUnsupported       = errors.New("plugin: runtime not supported")
	ErrArtifactChecksumMismatch = errors.New("plugin: artifact checksum mismatch")
	ErrCapabilityNotGranted     = errors.New("plugin: capability not granted")
)

// VerifyChecksum checks unit.Artifact against unit.Checksum (SHA-256 hex).
func VerifyChecksum(unit ComponentUnit) error {
	sum := sha256.Sum256(unit.Artifact)
	if hex.EncodeToString(sum[:]) != unit.Checksum {
		return fmt.Errorf("%w: %s", ErrArtifactChecksumMismatch, unit.Name)
	}
	return nil
}

// hasCapability reports whether a handle for name is present in handles.
func hasCapability(handles []contract.CapabilityHandle, name contract.CapabilityName) bool {
	for _, h := range handles {
		if h.Name() == name {
			return true
		}
	}
	return false
}

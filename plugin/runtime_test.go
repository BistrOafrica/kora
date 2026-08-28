package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/asenawritescode/kora/contract"
)

func checksum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestStubCompileExecuteShutdown(t *testing.T) {
	p := NewStubProvider("wasm-stub", "sms.send")
	artifact := []byte("wasm-bytes")
	unit := ComponentUnit{Name: "sms.twilio", Runtime: "wasm-stub", Artifact: artifact, Checksum: checksum(artifact)}
	if err := p.Compile(unit); err != nil {
		t.Fatalf("compile: %v", err)
	}

	ctx := contract.OperationContext{Site: "s", Actor: "a"}
	handle := noopHandle{name: "sms.send"}
	res, err := p.Execute(ctx, InvokeCall{Function: "send", Args: []byte(`{"to":"x"}`)}, []contract.CapabilityHandle{handle})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(res.Value, &out); err != nil {
		t.Fatalf("result not JSON-serialized at the seam: %v", err)
	}
	if out["function"] != "send" {
		t.Fatalf("unexpected result: %v", out)
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestStubChecksumMismatch(t *testing.T) {
	p := NewStubProvider("wasm-stub", "")
	unit := ComponentUnit{Name: "x", Runtime: "wasm-stub", Artifact: []byte("a"), Checksum: "deadbeef"}
	err := p.Compile(unit)
	if !errors.Is(err, ErrArtifactChecksumMismatch) {
		t.Fatalf("want ErrArtifactChecksumMismatch, got %v", err)
	}
}

func TestStubDeniesUngrantedCapabilities(t *testing.T) {
	p := NewStubProvider("wasm-stub", "sms.send")
	ctx := contract.OperationContext{Site: "s", Actor: "a"}
	_, err := p.Execute(ctx, InvokeCall{Function: "send"}, nil) // no handles
	if !errors.Is(err, ErrCapabilityNotGranted) {
		t.Fatalf("want ErrCapabilityNotGranted, got %v", err)
	}
}

func TestStubRuntimeUnsupported(t *testing.T) {
	p := NewStubProvider("wasm-stub", "")
	unit := ComponentUnit{Name: "x", Runtime: "native", Artifact: []byte("a"), Checksum: checksum([]byte("a"))}
	if err := p.Compile(unit); !errors.Is(err, ErrRuntimeUnsupported) {
		t.Fatalf("want ErrRuntimeUnsupported, got %v", err)
	}
}

func TestConformanceSuiteApplicableToStub(t *testing.T) {
	// The same RuntimeProvider contract must be implementable by a non-Goja
	// backend. Assert the stub satisfies the interface (compile-time).
	var _ RuntimeProvider = NewStubProvider("wasm-stub", "")
}

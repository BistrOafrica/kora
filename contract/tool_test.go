package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestToolDescriptorJSONShape freezes the canonical tool descriptor from RFC
// §10.4.2 so adapter parity tests have a stable reference.
func TestToolDescriptorJSONShape(t *testing.T) {
	d := ToolDescriptor{
		ID:                   "customer_create",
		Source:               "registry",
		Name:                 "customer_create",
		Description:          "Create a new Customer",
		InputSchema:          map[string]any{"type": "object"},
		SafetyLevel:          ToolSafetyHigh,
		RequiresConfirmation: true,
		RequiresRecentAuth:   true,
		ChannelAllowlist:     []string{"web", "whatsapp"},
		Operation:            "create",
		Doctype:              "Customer",
	}

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{
		"id", "name", "description", "input_schema", "safety_level",
		"requires_confirmation", "requires_recent_auth", "channel_allowlist",
		"operation", "doctype",
	} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("tool descriptor missing key %q: %s", key, b)
		}
	}

	var out ToolDescriptor
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.RequiresConfirmation != true || out.Operation != "create" || out.Doctype != "Customer" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestUsageEventJSONShape freezes the immutable usage record shape.
func TestUsageEventJSONShape(t *testing.T) {
	u := UsageEvent{
		ID:         NewID(),
		Site:       "acme.example.com",
		Model:      "gpt-4o",
		Provider:   "openai",
		RunID:      "run-1",
		Attempt:    1,
		Status:     "completed",
		Tokens:     map[UsageClass]int64{UsageClassInput: 100, UsageClassOutput: 50},
		OccurredAt: time.Now().UTC(),
	}

	b, err := json.Marshal(u)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{"id", "site", "model", "provider", "attempt", "status", "tokens", "occurred_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("usage event missing key %q: %s", key, b)
		}
	}
}

// TestApprovalJSONShape freezes the durable approval record shape.
func TestApprovalJSONShape(t *testing.T) {
	a := Approval{
		ID:                NewID(),
		Site:              "acme.example.com",
		OperationID:       NewOperationID(),
		Actor:             ActorContext{PrincipalID: "usr-1", PrincipalType: PrincipalHuman},
		ToolName:          "customer_create",
		State:             ApprovalPending,
		TargetFingerprint: "Customer:SINV-001",
		ArgumentHash:      "abc123",
		RequestedAt:       time.Now().UTC(),
	}

	b, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{"id", "site", "operation_id", "actor", "tool_name", "state", "target_fingerprint", "argument_hash"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("approval missing key %q: %s", key, b)
		}
	}

	var out Approval
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.State != ApprovalPending {
		t.Errorf("state = %v, want %v", out.State, ApprovalPending)
	}
}

// TestCursorJSONShape freezes the opaque cursor shape used by runs and sync.
func TestCursorJSONShape(t *testing.T) {
	c := Cursor{Token: "tok-1", Version: 42, At: time.Now().UTC()}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"token", "version", "at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("cursor missing key %q: %s", key, b)
		}
	}
}

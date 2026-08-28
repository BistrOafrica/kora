package contract

import (
	"errors"
	"testing"
	"time"
)

func grant(component string, cap CapabilityName, scope GrantScope) CapabilityGrant {
	return CapabilityGrant{Component: component, Capability: cap, Scope: scope}
}

func TestStaticIssuerDenyByDefault(t *testing.T) {
	issuer := NewStaticIssuer([]CapabilityGrant{grant("sms", "http.outbound", GrantScopeOperation)})
	ctx := OperationContext{Site: "site-a", Actor: "u1", TraceID: "t1", Deadline: time.Now().Add(time.Minute)}

	if _, err := issuer.Issue(ctx, "sms", []CapabilityName{"http.outbound"}); err != nil {
		t.Fatalf("declared capability denied: %v", err)
	}
	_, err := issuer.Issue(ctx, "sms", []CapabilityName{"secrets.read"})
	var denied *ErrCapabilityDenied
	if !errors.As(err, &denied) {
		t.Fatalf("want *ErrCapabilityDenied, got %T: %v", err, err)
	}
	if denied.Component != "sms" || denied.Capability != "secrets.read" {
		t.Fatalf("denial does not identify component/capability: %+v", denied)
	}
}

func TestStaticIssuerMixedRequestFailsClosed(t *testing.T) {
	issuer := NewStaticIssuer([]CapabilityGrant{grant("sms", "http.outbound", GrantScopeOperation)})
	ctx := OperationContext{Site: "s", Actor: "a", TraceID: "t"}

	// One declared + one undeclared: the whole request must fail, no partial.
	hs, err := issuer.Issue(ctx, "sms", []CapabilityName{"http.outbound", "secrets.read"})
	if err == nil {
		t.Fatalf("expected denial for mixed request")
	}
	if hs != nil {
		t.Fatalf("mixed request must not return partial handles: %v", hs)
	}
}

func TestHandleRevokeIdempotent(t *testing.T) {
	issuer := NewStaticIssuer([]CapabilityGrant{grant("sms", "http.outbound", GrantScopeCall)})
	hs, err := issuer.Issue(OperationContext{Site: "s", Actor: "a", TraceID: "t"}, "sms", []CapabilityName{"http.outbound"})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	h := hs[0]
	if h.Name() != "http.outbound" || h.Scope() != GrantScopeCall {
		t.Fatalf("unexpected handle: name=%q scope=%q", h.Name(), h.Scope())
	}
	h.Revoke()
	first := h.Evidence()
	if first.ReleasedAt.IsZero() {
		t.Fatalf("release not recorded on first revoke")
	}
	h.Revoke()
	second := h.Evidence()
	if !first.ReleasedAt.Equal(second.ReleasedAt) {
		t.Fatalf("double revoke changed release time: %v vs %v", first.ReleasedAt, second.ReleasedAt)
	}
}

func TestStaticIssuerSourceAgnosticParity(t *testing.T) {
	issuer := NewStaticIssuer([]CapabilityGrant{grant("sms", "http.outbound", GrantScopeOperation)})

	// The same component + capability request must produce the same grant
	// decision regardless of actor/site (authorization parity across sources).
	for name, ctx := range map[string]OperationContext{
		"http": {Site: "s", Actor: "u", TraceID: "t"},
		"sdk":  {Site: "s", Actor: "svc", TraceID: "t"},
		"mcp":  {Site: "s", Actor: "agent", TraceID: "t"},
		"ai":   {Site: "s", Actor: "ai-assistant", TraceID: "t"},
	} {
		hs, err := issuer.Issue(ctx, "sms", []CapabilityName{"http.outbound"})
		if err != nil || len(hs) != 1 {
			t.Fatalf("%s: parity violated: err=%v handles=%d", name, err, len(hs))
		}
	}
}

func TestStaticIssuerUndeclaredComponentDenied(t *testing.T) {
	issuer := NewStaticIssuer(nil)
	_, err := issuer.Issue(OperationContext{Site: "s", Actor: "a"}, "unknown", []CapabilityName{"docs.crud"})
	var denied *ErrCapabilityDenied
	if !errors.As(err, &denied) {
		t.Fatalf("want *ErrCapabilityDenied, got %T: %v", err, err)
	}
}

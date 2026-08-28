package plugin

import (
	"errors"
	"testing"

	"github.com/asenawritescode/kora/component"
	"github.com/asenawritescode/kora/contract"
)

func TestRemoveSMSProviderDegradesOnlySMSDependents(t *testing.T) {
	ctx := contract.OperationContext{Site: "s", Actor: "a"}
	manifests := []component.Manifest{
		{Name: "sms.twilio", Runtime: component.RuntimeGoja, Requires: []string{"sms.send"}},
		{Name: "storage.local", Runtime: component.RuntimeNative, Requires: []string{"storage.read"}},
	}
	// SMS factory is absent; storage factory is present.
	factories := []ProviderFactory{NewStaticFactory("storage.read", nil)}

	report, err := NewBinder(StrategyDegraded).Bind(ctx, manifests, factories)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if len(report.Bound) != 1 || report.Bound[0].Component != "storage.local" {
		t.Fatalf("storage should be bound: %+v", report.Bound)
	}
	if len(report.Degraded) != 1 || report.Degraded[0].Component != "sms.twilio" || report.Degraded[0].Capability != "sms.send" {
		t.Fatalf("sms should be degraded: %+v", report.Degraded)
	}
}

func TestStrictBindingFailsStartup(t *testing.T) {
	ctx := contract.OperationContext{Site: "s", Actor: "a"}
	manifests := []component.Manifest{{Name: "sms.twilio", Runtime: component.RuntimeGoja, Requires: []string{"sms.send"}}}

	_, err := NewBinder(StrategyStrict).Bind(ctx, manifests, nil)
	var unsatisfied *ErrUnsatisfiedRequirement
	if !errors.As(err, &unsatisfied) {
		t.Fatalf("want *ErrUnsatisfiedRequirement, got %T: %v", err, err)
	}
	if unsatisfied.Component != "sms.twilio" || unsatisfied.Capability != "sms.send" {
		t.Fatalf("unexpected unsatisfied: %+v", unsatisfied)
	}
}

func TestDegradedHandleDenied(t *testing.T) {
	// A degraded component's missing capability has no handle; a call through it
	// is represented by the absence of a binding (the caller must deny).
	ctx := contract.OperationContext{Site: "s", Actor: "a"}
	manifests := []component.Manifest{{Name: "sms.twilio", Runtime: component.RuntimeGoja, Requires: []string{"sms.send"}}}

	report, err := NewBinder(StrategyDegraded).Bind(ctx, manifests, nil)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if len(report.Bound) != 0 {
		t.Fatalf("degraded component must have no bound handle")
	}
	if len(report.Degraded) != 1 {
		t.Fatalf("expected one degradation")
	}
}

func TestProviderUnhealthyDegrades(t *testing.T) {
	ctx := contract.OperationContext{Site: "s", Actor: "a"}
	manifests := []component.Manifest{{Name: "sms.twilio", Runtime: component.RuntimeGoja, Requires: []string{"sms.send"}}}
	factories := []ProviderFactory{NewStaticFactory("sms.send", errors.New("down"))}

	report, err := NewBinder(StrategyDegraded).Bind(ctx, manifests, factories)
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if len(report.Degraded) != 1 || report.Degraded[0].Reason != "provider-unhealthy" {
		t.Fatalf("expected provider-unhealthy degradation: %+v", report.Degraded)
	}
}

func TestRebindOnProviderReturn(t *testing.T) {
	ctx := contract.OperationContext{Site: "s", Actor: "a"}
	manifests := []component.Manifest{{Name: "sms.twilio", Runtime: component.RuntimeGoja, Requires: []string{"sms.send"}}}

	first, _ := NewBinder(StrategyDegraded).Bind(ctx, manifests, nil)
	if len(first.Degraded) != 1 {
		t.Fatalf("expected degraded initially")
	}

	second, _ := NewBinder(StrategyDegraded).Bind(ctx, manifests, []ProviderFactory{NewStaticFactory("sms.send", nil)})
	if len(second.Degraded) != 0 || len(second.Bound) != 1 {
		t.Fatalf("expected bound after provider return")
	}
}

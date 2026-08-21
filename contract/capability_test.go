package contract

import (
	"os"
	"strings"
	"testing"
)

// TestBaselineCapabilitiesAreRegistered verifies every baseline capability can be
// registered and that a supported capability never carries a blocking risk.
func TestBaselineCapabilitiesAreRegistered(t *testing.T) {
	r := NewRuntime()
	for _, c := range BaselineCapabilities() {
		r.Register(c)
	}

	if len(r.Names()) != len(BaselineCapabilities()) {
		t.Errorf("registered %d capabilities, want %d", len(r.Names()), len(BaselineCapabilities()))
	}

	// A supported capability with unresolved risks must be reported as blocked.
	if blocked := r.Blocked(); len(blocked) != 0 {
		t.Errorf("baseline has supported capabilities with blocking risks: %+v", blocked)
	}
}

// TestCapabilityStatusReadsPlannedForUnknown verifies fail-closed defaults.
func TestCapabilityStatusReadsPlannedForUnknown(t *testing.T) {
	r := NewRuntime()
	if got := r.Status("not.registered"); got != CapabilityPlanned {
		t.Errorf("Status(unknown) = %v, want planned", got)
	}
}

// TestCapabilitySetStatusRejectsUnknown verifies status can't be faked on a
// capability that was never registered.
func TestCapabilitySetStatusRejectsUnknown(t *testing.T) {
	r := NewRuntime()
	if r.SetStatus("not.registered", CapabilitySupported) {
		t.Errorf("SetStatus should reject unknown capability")
	}
}

// TestCapabilityBlockedReportsSupportedWithRisk verifies the blocking-risk gate.
func TestCapabilityBlockedReportsSupportedWithRisk(t *testing.T) {
	r := NewRuntime()
	r.Register(Capability{
		Name:   "dangerous",
		Status: CapabilitySupported,
		Risks:  []BlockingRisk{RiskTenantIsolation},
	})
	blocked := r.Blocked()
	if len(blocked) != 1 || blocked[0].Name != "dangerous" {
		t.Errorf("Blocked() = %+v, want the dangerous capability", blocked)
	}
}

// TestCapabilityRegisterPanicsOnDuplicate guards against silent conflicts.
func TestCapabilityRegisterPanicsOnDuplicate(t *testing.T) {
	r := NewRuntime()
	r.Register(Capability{Name: "x", Status: CapabilityPlanned})
	defer func() {
		if recover() == nil {
			t.Errorf("expected panic on duplicate registration")
		}
	}()
	r.Register(Capability{Name: "x", Status: CapabilitySupported})
}

// TestCapabilityEvidenceDocsStayInSync acts as a data-integrity gate for the
// current phase evidence: the public Phase 0 evidence page must not drift from
// the canonical capability registry.
func TestCapabilityEvidenceDocsStayInSync(t *testing.T) {
	data, err := os.ReadFile("../docs/phase-0-contract-extraction.md")
	if err != nil {
		t.Fatalf("read evidence doc: %v", err)
	}
	doc := string(data)

	for _, cap := range BaselineCapabilities() {
		if !strings.Contains(doc, "`"+cap.Name+"`") {
			t.Fatalf("evidence doc missing capability %q", cap.Name)
		}
		if !strings.Contains(doc, "`"+string(cap.Status)+"`") {
			t.Fatalf("evidence doc missing status %q for capability %q", cap.Status, cap.Name)
		}
	}
}

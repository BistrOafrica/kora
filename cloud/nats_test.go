package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNATSDeploymentJSONShape(t *testing.T) {
	spec := NATSDeploymentSpec{
		ID:                "nats-1",
		Name:              "production-nats-af-south",
		Region:            "af-south",
		Servers:           []string{"tls://nats.example.internal:4222"},
		AccountMode:       "per_tenant",
		TLSRequired:       true,
		JetStreamRequired: true,
		BackupPolicyRef:   "backup-standard-v1",
		RPOSeconds:        300,
		RTOSeconds:        900,
		CredentialRef: CredentialReference{
			ID:        "cred-1",
			Provider:  "nats",
			SecretRef: "secret://nats/production-af-south/cloud-agent",
		},
		StreamPolicyRefs: []string{"streams/core-v1"},
		OperationID:      "op-1",
	}

	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	for _, key := range []string{"id", "name", "region", "servers", "account_mode", "tls_required", "jetstream_required", "backup_policy_ref", "rpo_seconds", "rto_seconds", "credential_ref", "operation_id"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("NATSDeploymentSpec missing key %q: %s", key, b)
		}
	}

	deployment := NATSDeployment{
		ID:               spec.ID,
		Name:             spec.Name,
		Region:           spec.Region,
		Servers:          spec.Servers,
		AccountMode:      spec.AccountMode,
		State:            NATSDeploymentUnregistered,
		CredentialRef:    spec.CredentialRef,
		RegisteredAt:     time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		OperationID:      spec.OperationID,
		StreamPolicyRefs: []string{"streams/core-v1"},
	}

	b, err = json.Marshal(deployment)
	if err != nil {
		t.Fatalf("marshal deployment: %v", err)
	}
	for _, key := range []string{"state", "registered_at", "credential_ref", "operation_id"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("NATSDeployment missing key %q: %s", key, b)
		}
	}

	manifest := BackupManifest{
		DeploymentID: deployment.ID,
		PolicyRef:    spec.BackupPolicyRef,
		RPOSeconds:   spec.RPOSeconds,
		RTOSeconds:   spec.RTOSeconds,
		CreatedAt:    deployment.RegisteredAt,
	}
	b, err = json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	for _, key := range []string{"deployment_id", "policy_ref", "rpo_seconds", "rto_seconds", "created_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("BackupManifest missing key %q: %s", key, b)
		}
	}
}

func TestNATSDeploymentTransitionRules(t *testing.T) {
	tests := []struct {
		from NATSDeploymentState
		to   NATSDeploymentState
		want bool
	}{
		{NATSDeploymentUnregistered, NATSDeploymentReady, true},
		{NATSDeploymentUnregistered, NATSDeploymentDraining, false},
		{NATSDeploymentReady, NATSDeploymentDegraded, true},
		{NATSDeploymentDegraded, NATSDeploymentReady, true},
		{NATSDeploymentDraining, NATSDeploymentReady, true},
		{NATSDeploymentIncompatible, NATSDeploymentReady, true},
	}

	for _, tt := range tests {
		if got := NATSDeploymentCanTransition(tt.from, tt.to); got != tt.want {
			t.Fatalf("NATSDeploymentCanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestNATSValidationReadyAndFailed(t *testing.T) {
	health := NATSHealth{
		DeploymentID:  "nats-1",
		State:         NATSDeploymentReady,
		ServerVersion: "2.10.18",
		Checks:        []string{"compatibility", "permission", "backup", "restore"},
		ValidatedAt:   time.Date(2026, 8, 13, 12, 10, 0, 0, time.UTC),
	}
	if !NATSValidationReady(health) {
		t.Fatal("expected health to be ready")
	}
	if NATSValidationFailed(health.State) {
		t.Fatal("ready health should not be failed")
	}

	health.Checks = []string{"compatibility", "permission", "backup"}
	if NATSValidationReady(health) {
		t.Fatal("missing restore check should fail readiness")
	}
	if !NATSValidationFailed(NATSDeploymentIncompatible) {
		t.Fatal("incompatible deployment should be failed")
	}
	if !NATSValidationFailed(NATSDeploymentUnreachable) {
		t.Fatal("unreachable deployment should be failed")
	}
}

func TestNATSDeploymentStateClassifiers(t *testing.T) {
	if !NATSDeploymentIsDraining(NATSDeploymentDraining) {
		t.Fatal("draining deployment should be classified as draining")
	}
	if NATSDeploymentIsDraining(NATSDeploymentReady) {
		t.Fatal("ready deployment should not be draining")
	}
	if !NATSDeploymentNeedsRecovery(NATSDeploymentUnreachable) {
		t.Fatal("unreachable deployment should need recovery")
	}
	if !NATSDeploymentNeedsRecovery(NATSDeploymentDegraded) {
		t.Fatal("degraded deployment should need recovery")
	}
	if NATSDeploymentNeedsRecovery(NATSDeploymentIncompatible) {
		t.Fatal("incompatible deployment should not be treated as recoverable")
	}
	if !NATSDeploymentNeedsOperatorAction(NATSDeploymentIncompatible) {
		t.Fatal("incompatible deployment should need operator action")
	}
	if NATSDeploymentNeedsOperatorAction(NATSDeploymentUnreachable) {
		t.Fatal("unreachable deployment should not be operator-only")
	}
}

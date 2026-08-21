package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTenantAccountAndBootstrapJSONShape(t *testing.T) {
	account := TenantAccount{
		ID:           "tenant-1",
		DeploymentID: "nats-1",
		Name:         "acme-west",
		State:        "ready",
		Quota: TenantQuota{
			MaxStreams:         6,
			MaxConsumers:       18,
			MaxKeyValueBuckets: 4,
			MaxObjectStores:    2,
			MaxMessages:        100000,
			MaxStorageBytes:    1 << 30,
			MaxWorkers:         12,
		},
		Resources: TenantResourceSet{
			AccountName:        "ACME_WEST",
			Streams:            []string{"KORA_EVENTS", "KORA_COMMANDS"},
			KeyValueBuckets:    []string{"KORA_CONFIG"},
			ObjectStores:       []string{"KORA_ARTIFACTS"},
			EngineCredentials:  []string{"secret://engine"},
			WorkerCredentials:  []string{"secret://worker"},
			GatewayCredentials: []string{"secret://gateway"},
		},
		CreatedAt:        time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		OperationID:      "op-2",
		LastBootstrapAt:  time.Date(2026, 8, 13, 12, 5, 0, 0, time.UTC),
		LastValidationAt: time.Date(2026, 8, 13, 12, 4, 0, 0, time.UTC),
	}

	b, err := json.Marshal(account)
	if err != nil {
		t.Fatalf("marshal account: %v", err)
	}
	for _, key := range []string{"id", "deployment_id", "quota", "resources", "last_bootstrap_at", "last_validation_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("TenantAccount missing key %q: %s", key, b)
		}
	}

	plan := TenantBootstrapPlan{
		TenantID:       account.ID,
		DeploymentID:   account.DeploymentID,
		Resources:      account.Resources,
		Quota:          account.Quota,
		RequestedAt:    account.CreatedAt,
		OperationID:    account.OperationID,
		Resumable:      true,
		ValidationOnly: false,
	}

	b, err = json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	for _, key := range []string{"tenant_id", "deployment_id", "resources", "quota", "requested_at", "resumable", "validation_only"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("TenantBootstrapPlan missing key %q: %s", key, b)
		}
	}
}

func TestTenantBootstrapTransitionRules(t *testing.T) {
	tests := []struct {
		from TenantBootstrapState
		to   TenantBootstrapState
		want bool
	}{
		{TenantBootstrapRequested, TenantBootstrapValidating, true},
		{TenantBootstrapValidating, TenantBootstrapProvisioning, true},
		{TenantBootstrapProvisioning, TenantBootstrapReady, true},
		{TenantBootstrapReady, TenantBootstrapRequested, false},
		{TenantBootstrapFailed, TenantBootstrapRequested, true},
	}

	for _, tt := range tests {
		if got := TenantBootstrapCanTransition(tt.from, tt.to); got != tt.want {
			t.Fatalf("TenantBootstrapCanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

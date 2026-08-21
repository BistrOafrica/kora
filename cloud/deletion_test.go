package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDeletionEvidenceJSONShape(t *testing.T) {
	wf := DeletionWorkflow{
		ID:                  "del-1",
		OrganizationID:      "org-1",
		Site:                "acme.example.com",
		Region:              "af-south",
		State:               DeletionRequested,
		OperationID:         "op-1",
		ManifestRef:         "manifest://delete/1",
		CredentialRefs:      []string{"secret://nats", "secret://db"},
		BackupVerified:      true,
		ObjectStoreVerified: true,
		RetentionPolicyRef:  "retention-standard-v1",
		RequestedAt:         time.Date(2026, 8, 13, 15, 10, 0, 0, time.UTC),
	}
	b, err := json.Marshal(wf)
	if err != nil {
		t.Fatalf("marshal workflow: %v", err)
	}
	for _, key := range []string{"organization_id", "site", "region", "state", "operation_id", "manifest_ref", "credential_refs", "backup_verified", "object_store_verified", "retention_policy_ref", "requested_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("DeletionWorkflow missing key %q: %s", key, b)
		}
	}

	boundary := IsolationBoundary{
		SQL: true, NATS: true, KeyValue: true, ObjectStore: true, Cache: true, Logs: true, Traces: true, Metrics: true, Backups: true, Credentials: true,
	}
	evidence := RPOEvidence{
		DeploymentID: "dep-1",
		Region:       "af-south",
		RPOSeconds:   300,
		RTOSeconds:   900,
		LastVerified: wf.RequestedAt.Add(time.Minute),
		VerifiedBy:   "auditor-1",
		BackupRef:    "backup://dep-1/2026-08-13",
		RestoreRef:   "restore://dep-1/2026-08-13",
	}
	for _, v := range []struct {
		val  any
		keys []string
	}{
		{boundary, []string{"sql", "nats", "key_value", "object_store", "cache", "logs", "traces", "metrics", "backups", "credentials"}},
		{evidence, []string{"deployment_id", "region", "rpo_seconds", "rto_seconds", "last_verified", "backup_ref", "restore_ref"}},
	} {
		b, err := json.Marshal(v.val)
		if err != nil {
			t.Fatalf("marshal %T: %v", v.val, err)
		}
		for _, key := range v.keys {
			if !strings.Contains(string(b), `"`+key+`"`) {
				t.Fatalf("%T missing key %q: %s", v.val, key, b)
			}
		}
	}
}

func TestDeletionIsolationAndRPOHelpers(t *testing.T) {
	if !DeletionCanTransition(DeletionRequested, DeletionRevoking) {
		t.Fatal("requested should revoke")
	}
	if DeletionCanTransition(DeletionCompleted, DeletionRequested) {
		t.Fatal("completed should not rewind")
	}
	if !IsolationBoundaryComplete(IsolationBoundary{
		SQL: true, NATS: true, KeyValue: true, ObjectStore: true, Cache: true, Logs: true, Traces: true, Metrics: true, Backups: true, Credentials: true,
	}) {
		t.Fatal("complete isolation boundary should pass")
	}
	if IsolationBoundaryComplete(IsolationBoundary{SQL: true, NATS: true}) {
		t.Fatal("partial isolation boundary should fail")
	}
	if !RPOEvidenceValid(RPOEvidence{
		DeploymentID: "dep-1",
		Region:       "af-south",
		RPOSeconds:   300,
		RTOSeconds:   900,
		LastVerified: time.Now().UTC(),
		BackupRef:    "backup://dep-1",
		RestoreRef:   "restore://dep-1",
	}) {
		t.Fatal("valid RPO evidence should pass")
	}
	if RPOEvidenceValid(RPOEvidence{DeploymentID: "dep-1", Region: "af-south", RPOSeconds: 0, RTOSeconds: 900, LastVerified: time.Now().UTC(), BackupRef: "backup://dep-1", RestoreRef: "restore://dep-1"}) {
		t.Fatal("zero RPO should fail")
	}
	if !DeletionEvidenceComplete(DeletionWorkflow{
		ID:                  "del-1",
		OperationID:         "op-1",
		State:               DeletionCompleted,
		ManifestRef:         "manifest://delete/1",
		BackupVerified:      true,
		ObjectStoreVerified: true,
	}) {
		t.Fatal("completed deletion evidence should pass")
	}
}

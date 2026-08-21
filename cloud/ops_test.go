package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestOpsJSONShape(t *testing.T) {
	policy := BackupPolicy{
		ID:             "bp-1",
		DeploymentID:   "dep-1",
		Region:         "af-south",
		State:          BackupPolicyRequested,
		RPOSeconds:     300,
		RTOSeconds:     900,
		RetentionDays:  30,
		RequiredChecks: []string{"stream", "kv", "object_store"},
		CreatedAt:      time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	for _, key := range []string{"id", "deployment_id", "region", "state", "rpo_seconds", "rto_seconds", "retention_days", "required_checks"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("BackupPolicy missing key %q: %s", key, b)
		}
	}

	record := BackupRecord{
		ID:           "br-1",
		PolicyID:     policy.ID,
		DeploymentID: policy.DeploymentID,
		Region:       policy.Region,
		ArtifactRef:  "s3://backups/dep-1/2026-08-13",
		VerifiedAt:   policy.CreatedAt.Add(time.Minute),
		CreatedAt:    policy.CreatedAt,
		Restorable:   true,
	}
	snapshot := ObservabilitySnapshot{
		ID:             "obs-1",
		DeploymentID:   policy.DeploymentID,
		Region:         policy.Region,
		State:          "healthy",
		Healthy:        true,
		RequestsPerMin: 120,
		ErrorsPerMin:   1,
		LatencyP95Ms:   180,
		CollectedAt:    policy.CreatedAt.Add(2 * time.Minute),
		SLORef:         "slo-standard-v1",
	}
	proj := BillingUsageProjection{
		ID:             "bill-1",
		OrganizationID: "org-1",
		DeploymentID:   policy.DeploymentID,
		Region:         policy.Region,
		Period:         "2026-08",
		Currency:       "USD",
		UsageUnits:     1500,
		EstimatedCost:  42.50,
		Reconciled:     false,
		CreatedAt:      policy.CreatedAt,
	}
	placement := RegionalPlacement{
		ID:              "rp-1",
		DeploymentID:    policy.DeploymentID,
		PreferredRegion: "af-south",
		AllowedRegions:  []string{"af-south", "eu-west"},
		FailoverRegions: []string{"eu-west"},
		LockInRegion:    true,
		CreatedAt:       policy.CreatedAt,
	}

	checkJSONKeys := func(v any, keys []string) {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %T: %v", v, err)
		}
		for _, key := range keys {
			if !strings.Contains(string(b), `"`+key+`"`) {
				t.Fatalf("%T missing key %q: %s", v, key, b)
			}
		}
	}
	checkJSONKeys(record, []string{"artifact_ref", "verified_at", "restorable"})
	checkJSONKeys(snapshot, []string{"collected_at", "slo_ref"})
	checkJSONKeys(proj, []string{"estimated_cost", "reconciled"})
	checkJSONKeys(placement, []string{"preferred_region", "allowed_regions", "failover_regions"})
}

func TestOpsValidationHelpers(t *testing.T) {
	policy := BackupPolicy{ID: "bp-1", DeploymentID: "dep-1", Region: "af-south", State: BackupPolicyActive, RPOSeconds: 300, RTOSeconds: 900}
	record := &BackupRecord{PolicyID: "bp-1", DeploymentID: "dep-1", Region: "af-south", Restorable: true}
	if !BackupPolicyHealthy(policy, record) {
		t.Fatal("healthy backup policy should pass")
	}
	if BackupPolicyHealthy(policy, nil) {
		t.Fatal("missing backup record should fail")
	}
	if !BackupPolicyCanTransition(BackupPolicyRequested, BackupPolicyActive) {
		t.Fatal("requested backup policy should activate")
	}
	if BackupPolicyCanTransition(BackupPolicyRetired, BackupPolicyActive) {
		t.Fatal("retired backup policy should not reactivate")
	}

	snapshot := ObservabilitySnapshot{State: "healthy", Healthy: true, RequestsPerMin: 10, ErrorsPerMin: 0, LatencyP95Ms: 200}
	if !ObservabilityHealthy(snapshot) {
		t.Fatal("healthy snapshot should pass")
	}
	snapshot.ErrorsPerMin = -1
	if ObservabilityHealthy(snapshot) {
		t.Fatal("negative error rate should fail")
	}

	proj := BillingUsageProjection{OrganizationID: "org-1", DeploymentID: "dep-1", Region: "af-south", Period: "2026-08", Currency: "USD", CreatedAt: time.Now().UTC()}
	if !BillingProjectionIsValid(proj) {
		t.Fatal("billing projection should be valid")
	}
	proj.Currency = ""
	if BillingProjectionIsValid(proj) {
		t.Fatal("missing currency should fail billing projection")
	}

	placement := RegionalPlacement{PreferredRegion: "af-south", AllowedRegions: []string{"af-south", "eu-west"}}
	if !RegionalPlacementMatches(placement, "eu-west") {
		t.Fatal("allowed region should match")
	}
	if RegionalPlacementMatches(placement, "us-east") {
		t.Fatal("disallowed region should not match")
	}
}

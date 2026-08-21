package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPackageRegistryJSONShape(t *testing.T) {
	entry := PackageRegistryEntry{
		ID: "pkg-1",
		Artifact: PackageArtifact{
			ID:         "artifact-1",
			Name:       "kora-erp",
			Version:    "2026.08.1",
			Digest:     "sha256:abc123",
			SizeBytes:  1024,
			UploadedAt: time.Date(2026, 8, 13, 12, 40, 0, 0, time.UTC),
		},
		State:         PackageLifecycleUploaded,
		Compatibility: []string{"2026.08", "2026.09"},
		DeploymentIDs: []string{"dep-1"},
		OperationID:   "op-1",
	}

	b, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	for _, key := range []string{"id", "artifact", "state", "compatibility", "deployment_ids", "operation_id"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("PackageRegistryEntry missing key %q: %s", key, b)
		}
	}

	rollout := DeploymentRollout{
		ID:           "rollout-1",
		DeploymentID: "dep-1",
		PackageID:    entry.ID,
		FromVersion:  "2026.07.1",
		ToVersion:    entry.Artifact.Version,
		State:        DeploymentRolloutRequested,
		StartedAt:    time.Date(2026, 8, 13, 12, 41, 0, 0, time.UTC),
	}
	b, err = json.Marshal(rollout)
	if err != nil {
		t.Fatalf("marshal rollout: %v", err)
	}
	for _, key := range []string{"deployment_id", "package_id", "from_version", "to_version", "state", "started_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("DeploymentRollout missing key %q: %s", key, b)
		}
	}
}

func TestPackageAndRolloutTransitions(t *testing.T) {
	if !PackageLifecycleCanTransition(PackageLifecycleUploaded, PackageLifecycleVerified) {
		t.Fatal("uploaded package should verify")
	}
	if PackageLifecycleCanTransition(PackageLifecycleVerified, PackageLifecycleUploaded) {
		t.Fatal("verified package should not move backwards")
	}
	if !PackageLifecycleCanTransition(PackageLifecycleBlocked, PackageLifecycleUploaded) {
		t.Fatal("blocked package should be re-uploadable")
	}
	if !DeploymentRolloutCanTransition(DeploymentRolloutRequested, DeploymentRolloutRunning) {
		t.Fatal("requested rollout should start running")
	}
	if !DeploymentRolloutCanTransition(DeploymentRolloutRunning, DeploymentRolloutCompleted) {
		t.Fatal("running rollout should complete")
	}
	if DeploymentRolloutCanTransition(DeploymentRolloutCompleted, DeploymentRolloutRunning) {
		t.Fatal("completed rollout should not resume")
	}
}

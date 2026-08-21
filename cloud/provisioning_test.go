package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProvisioningJSONShape(t *testing.T) {
	job := ProvisioningJob{
		ID:                "job-1",
		Site:              "acme.example.com",
		Resource:          "deployment-1",
		State:             ProvisioningRequested,
		OperationID:       "op-1",
		IdempotencyKey:    "idem-1",
		InputFingerprint:  "fingerprint-1",
		ProviderRequestID: "provider-req-1",
		OutputID:          "output-1",
		Attempt:           2,
		LeaseOwner:        "worker-1",
		LeaseUntil:        time.Date(2026, 8, 13, 15, 0, 0, 0, time.UTC),
		RetryAt:           time.Date(2026, 8, 13, 15, 5, 0, 0, time.UTC),
		CreatedAt:         time.Date(2026, 8, 13, 14, 55, 0, 0, time.UTC),
	}
	b, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	for _, key := range []string{"id", "site", "resource", "state", "operation_id", "idempotency_key", "input_fingerprint", "provider_request_id", "output_id", "attempt", "lease_owner", "lease_until", "retry_at", "created_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("ProvisioningJob missing key %q: %s", key, b)
		}
	}

	cp := ProvisioningCheckpoint{
		JobID:      job.ID,
		Stage:      "provisioning_nats",
		Completed:  true,
		RecordedAt: job.CreatedAt,
	}
	b, err = json.Marshal(cp)
	if err != nil {
		t.Fatalf("marshal checkpoint: %v", err)
	}
	for _, key := range []string{"job_id", "stage", "completed", "recorded_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("ProvisioningCheckpoint missing key %q: %s", key, b)
		}
	}
}

func TestProvisioningResumableAndDuplicateDetection(t *testing.T) {
	previous := ProvisioningJob{
		ID:               "job-1",
		Site:             "acme.example.com",
		Resource:         "deployment-1",
		State:            ProvisioningProvisioning,
		OperationID:      "op-1",
		IdempotencyKey:   "idem-1",
		InputFingerprint: "fingerprint-1",
	}
	next := previous
	if !ProvisioningIsResumable(previous) {
		t.Fatal("provisioning job should be resumable")
	}
	if !ProvisioningIsDuplicate(previous, next) {
		t.Fatal("matching provisioning job should be treated as duplicate restart")
	}
	next.IdempotencyKey = "idem-2"
	if ProvisioningIsDuplicate(previous, next) {
		t.Fatal("different idempotency key should not be duplicate")
	}
	if !ProvisioningCanTransition(ProvisioningRequested, ProvisioningValidating) {
		t.Fatal("requested should transition to validating")
	}
	if ProvisioningCanTransition(ProvisioningActive, ProvisioningRequested) {
		t.Fatal("active should not rewind")
	}

	cp := ProvisioningCheckpoint{JobID: "job-1", Stage: "provisioning_nats", Completed: true, RecordedAt: time.Now().UTC()}
	if !ProvisioningCheckpointCanResume(cp) {
		t.Fatal("completed checkpoint should resume")
	}
	if ProvisioningCheckpointCanResume(ProvisioningCheckpoint{JobID: "job-1", Stage: "", Completed: true}) {
		t.Fatal("checkpoint without stage should not resume")
	}
}

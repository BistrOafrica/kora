package site

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOnboardingStoreFilePersistence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KORA_CONFIG_DIR", dir)
	store := NewOnboardingStore(nil)

	job := OnboardingJob{
		ID:               "job-1",
		Site:             "acme.example.com",
		Resource:         "acme.example.com",
		State:            OnboardingRequested,
		OperationID:      "op-1",
		IdempotencyKey:   "idem-1",
		InputFingerprint: "fingerprint-1",
		Attempt:          1,
		CreatedAt:        time.Now().UTC(),
	}
	cp := OnboardingCheckpoint{JobID: job.ID, Stage: "requested", Completed: true, RecordedAt: time.Now().UTC()}
	if err := store.UpsertJob(job); err != nil {
		t.Fatalf("upsert job: %v", err)
	}
	if err := store.UpsertCheckpoint(cp); err != nil {
		t.Fatalf("upsert checkpoint: %v", err)
	}

	loaded, err := store.LoadJobByID(job.ID)
	if err != nil {
		t.Fatalf("load job by id: %v", err)
	}
	if loaded.ID != job.ID || loaded.Site != job.Site {
		t.Fatalf("loaded job mismatch: %+v", loaded)
	}

	byKey, err := store.LoadJob(job.Site, job.Resource, job.OperationID, job.IdempotencyKey, job.InputFingerprint)
	if err != nil {
		t.Fatalf("load job by key: %v", err)
	}
	if byKey.ID != job.ID {
		t.Fatalf("load job by key mismatch: %+v", byKey)
	}

	jobs, err := store.ListJobs()
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if _, err := os.Stat(filepath.Join(dir, "_kora_provisioning_jobs.json")); err != nil {
		t.Fatalf("expected backing file: %v", err)
	}
}

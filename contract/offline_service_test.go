package contract

import (
	"encoding/json"
	"testing"
	"time"
)

type memoryOfflineStore struct {
	operations []OfflineOperation
	conflicts  []OfflineConflict
	tombstones []OfflineTombstone
	cursors    []SyncCursor
}

func (m *memoryOfflineStore) SaveOperation(op OfflineOperation) error {
	m.operations = append(m.operations, op)
	return nil
}

func (m *memoryOfflineStore) SaveConflict(conflict OfflineConflict) error {
	m.conflicts = append(m.conflicts, conflict)
	return nil
}

func (m *memoryOfflineStore) SaveTombstone(t OfflineTombstone) error {
	m.tombstones = append(m.tombstones, t)
	return nil
}

func (m *memoryOfflineStore) AdvanceCursor(cursor SyncCursor) error {
	m.cursors = append(m.cursors, cursor)
	return nil
}

func TestOfflineSyncServiceApplyLocal(t *testing.T) {
	store := &memoryOfflineStore{}
	svc := OfflineSyncService{Store: store, Clock: func() time.Time {
		return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	}}

	op := OfflineOperation{
		ID:            NewID(),
		DeviceID:      "device-1",
		BranchID:      "branch-west",
		Site:          "acme.example.com",
		EntityType:    "Sales Invoice",
		OperationType: "create",
		BaseVersion:   0,
		OccurredAt:    time.Date(2026, 8, 13, 11, 0, 0, 0, time.UTC),
		SchemaVersion: "2026.08",
		Status:        OfflineOperationQueued,
		Payload:       json.RawMessage(`{"name":"SINV-0001"}`),
	}

	if err := svc.ApplyLocal(op); err != nil {
		t.Fatalf("ApplyLocal: %v", err)
	}
	if len(store.operations) != 1 || store.operations[0].ID != op.ID {
		t.Fatalf("operation was not stored: %+v", store.operations)
	}
}

func TestOfflineSyncServiceIntakeAcceptedAndRejected(t *testing.T) {
	store := &memoryOfflineStore{}
	svc := OfflineSyncService{Store: store, Clock: func() time.Time {
		return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	}}

	accepted := OfflineOperation{
		ID:            NewID(),
		DeviceID:      "device-1",
		BranchID:      "branch-west",
		Site:          "acme.example.com",
		EntityType:    "Sales Invoice",
		OperationType: "create",
		BaseVersion:   0,
		OccurredAt:    time.Date(2026, 8, 13, 11, 0, 0, 0, time.UTC),
		SchemaVersion: "2026.08",
		Status:        OfflineOperationQueued,
	}

	rejected := accepted
	rejected.ID = NewID()
	rejected.SchemaVersion = "2025.12"

	receipts, err := svc.Intake(OfflineSyncBatch{
		SchemaVersion: "2026.08",
		Gate:          OfflineSchemaGateAccepted,
		Operations:    []OfflineOperation{accepted},
		Cursor:        SyncCursor{Token: "c-1"},
		NextCursor:    SyncCursor{Token: "c-2"},
	})
	if err != nil {
		t.Fatalf("Intake accepted: %v", err)
	}
	if len(receipts) != 1 || receipts[0].Status != OfflineOperationAcked {
		t.Fatalf("accepted receipt mismatch: %+v", receipts)
	}
	if len(store.operations) != 1 || len(store.cursors) != 1 {
		t.Fatalf("accepted operation should be stored and cursor advanced: %+v %+v", store.operations, store.cursors)
	}

	receipts, err = svc.Intake(OfflineSyncBatch{
		SchemaVersion: "2025.12",
		Gate:          OfflineSchemaGateRejected,
		Operations:    []OfflineOperation{rejected},
		Cursor:        SyncCursor{Token: "c-2"},
		NextCursor:    SyncCursor{Token: "c-3"},
	})
	if err != nil {
		t.Fatalf("Intake rejected: %v", err)
	}
	if len(receipts) != 1 || receipts[0].Status != OfflineOperationConflict || receipts[0].Conflict == nil {
		t.Fatalf("rejected receipt mismatch: %+v", receipts)
	}
	if len(store.conflicts) != 1 {
		t.Fatalf("rejected operation should create a conflict: %+v", store.conflicts)
	}
	if receipts[0].Conflict.ResolutionMode != "retry" {
		t.Fatalf("rejected conflict resolution mode = %q, want retry", receipts[0].Conflict.ResolutionMode)
	}
	if !RetainConflict(*receipts[0].Conflict) {
		t.Fatal("rejected conflict should remain retryable or resolvable")
	}
}

func TestOfflineSyncServiceAcknowledge(t *testing.T) {
	store := &memoryOfflineStore{}
	svc := OfflineSyncService{Store: store}
	conflict := &OfflineConflict{
		ID:             NewID(),
		OperationID:    NewID(),
		DeviceID:       "device-1",
		BranchID:       "branch-west",
		Site:           "acme.example.com",
		EntityType:     "Sales Invoice",
		ReasonCode:     CodeConflict,
		ResolutionMode: "manual_review",
		ServerVersion:  42,
		RecordedAt:     time.Now().UTC(),
	}

	if err := svc.Acknowledge(
		OfflineOperation{ID: conflict.OperationID},
		SyncCursor{Token: "c-4"},
		conflict,
	); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}
	if len(store.conflicts) != 1 || len(store.cursors) != 1 {
		t.Fatalf("acknowledge should persist conflict and cursor: %+v %+v", store.conflicts, store.cursors)
	}
}

func TestOfflinePullSinceIsRepeatable(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	ops := []OfflineOperation{
		{ID: "op-1", OccurredAt: now.Add(-2 * time.Minute)},
		{ID: "op-2", OccurredAt: now.Add(-time.Minute)},
		{ID: "op-3", OccurredAt: now},
	}

	page1 := PullSince(ops, SyncCursor{Token: "op-0", At: now.Add(-3 * time.Minute), BranchID: "branch-west", DeviceID: "device-1"})
	page2 := PullSince(ops, SyncCursor{Token: "op-0", At: now.Add(-3 * time.Minute), BranchID: "branch-west", DeviceID: "device-1"})

	if len(page1.Operations) != 3 || len(page2.Operations) != 3 {
		t.Fatalf("expected repeatable full page: %+v %+v", page1.Operations, page2.Operations)
	}
	if page1.NextCursor.Token != "op-3" || page1.NextCursor.Version != 3 {
		t.Fatalf("next cursor mismatch: %+v", page1.NextCursor)
	}

	next := PullSince(ops, page1.NextCursor)
	if len(next.Operations) != 0 {
		t.Fatalf("advancing the cursor should exhaust the page: %+v", next.Operations)
	}
}

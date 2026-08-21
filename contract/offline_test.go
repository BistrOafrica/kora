package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestOfflineOperationJSONShape(t *testing.T) {
	op := OfflineOperation{
		ID:            NewID(),
		DeviceID:      "device-1",
		BranchID:      "branch-west",
		Site:          "acme.example.com",
		EntityType:    "Sales Invoice",
		EntityID:      "SINV-0001",
		OperationType: "update",
		BaseVersion:   41,
		OccurredAt:    time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC),
		SchemaVersion: "2026.08",
		Status:        OfflineOperationQueued,
		Payload:       json.RawMessage(`{"status":"Draft"}`),
		Metadata:      map[string]string{"source": "mobile"},
	}

	b, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{"id", "device_id", "branch_id", "site", "entity_type", "entity_id", "operation_type", "base_version", "occurred_at", "schema_version", "status", "payload", "metadata"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("offline operation missing key %q: %s", key, b)
		}
	}

	var out OfflineOperation
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.DeviceID != op.DeviceID || out.BranchID != op.BranchID || out.SchemaVersion != op.SchemaVersion {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}

func TestOfflineConflictJSONShape(t *testing.T) {
	conflict := OfflineConflict{
		ID:             NewID(),
		OperationID:    NewID(),
		DeviceID:       "device-1",
		BranchID:       "branch-west",
		Site:           "acme.example.com",
		EntityType:     "Sales Invoice",
		EntityID:       "SINV-0001",
		ReasonCode:     CodeConflict,
		ResolutionMode: "retry",
		ServerVersion:  42,
		RecordedAt:     time.Date(2026, 8, 12, 9, 5, 0, 0, time.UTC),
		Snapshot:       json.RawMessage(`{"status":"Submitted"}`),
	}

	b, err := json.Marshal(conflict)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{"id", "operation_id", "device_id", "branch_id", "site", "entity_type", "entity_id", "reason_code", "resolution_mode", "server_version", "recorded_at", "snapshot"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("offline conflict missing key %q: %s", key, b)
		}
	}

	if !ConflictResolutionRetryable(conflict.ResolutionMode) {
		t.Fatal("retryable conflict should be retryable")
	}
	if ConflictResolutionRetryable("discard") {
		t.Fatal("discard should not be retryable")
	}
}

func TestOfflineSyncBatchJSONShape(t *testing.T) {
	batch := OfflineSyncBatch{
		SchemaVersion: "2026.08",
		Gate:          OfflineSchemaGateAccepted,
		Operations: []OfflineOperation{{
			ID:            NewID(),
			DeviceID:      "device-1",
			BranchID:      "branch-west",
			Site:          "acme.example.com",
			EntityType:    "Sales Invoice",
			OperationType: "create",
			BaseVersion:   0,
			OccurredAt:    time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC),
			SchemaVersion: "2026.08",
			Status:        OfflineOperationQueued,
		}},
		Cursor: SyncCursor{
			Token:    "branch-west:1042",
			BranchID: "branch-west",
			DeviceID: "device-1",
			Version:  1042,
			At:       time.Date(2026, 8, 12, 9, 5, 0, 0, time.UTC),
		},
		NextCursor: SyncCursor{
			Token:    "branch-west:1043",
			BranchID: "branch-west",
			DeviceID: "device-1",
			Version:  1043,
			At:       time.Date(2026, 8, 12, 9, 6, 0, 0, time.UTC),
		},
	}

	b, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, key := range []string{"schema_version", "gate", "operations", "cursor", "next_cursor"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("offline sync batch missing key %q: %s", key, b)
		}
	}
}

func TestOfflineSchemaAndTransitionRules(t *testing.T) {
	if !AcceptsOfflineSchema(OfflineSchemaGateAccepted) {
		t.Fatal("accepted schema gate should pass")
	}
	if AcceptsOfflineSchema(OfflineSchemaGateRejected) {
		t.Fatal("rejected schema gate should fail")
	}

	tests := []struct {
		from OfflineOperationStatus
		to   OfflineOperationStatus
		want bool
	}{
		{OfflineOperationQueued, OfflineOperationSent, true},
		{OfflineOperationQueued, OfflineOperationConflict, false},
		{OfflineOperationSent, OfflineOperationAcked, true},
		{OfflineOperationSent, OfflineOperationDiscarded, false},
		{OfflineOperationConflict, OfflineOperationQueued, true},
		{OfflineOperationRejected, OfflineOperationQueued, false},
	}

	for _, tt := range tests {
		if got := OperationCanTransition(tt.from, tt.to); got != tt.want {
			t.Fatalf("OperationCanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestOfflineTombstoneRetention(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tombstone := OfflineTombstone{
		ID:            NewID(),
		DeviceID:      "device-1",
		BranchID:      "branch-west",
		Site:          "acme.example.com",
		EntityType:    "Sales Invoice",
		EntityID:      "SINV-0001",
		DeletedAt:     now.Add(-time.Hour),
		RetainUntil:   now.Add(time.Hour),
		SchemaVersion: "2026.08",
	}

	if !TombstoneRetained(tombstone, now) {
		t.Fatal("tombstone should be retained during the offline window")
	}

	tombstone.RetainUntil = now.Add(-time.Minute)
	if TombstoneRetained(tombstone, now) {
		t.Fatal("expired tombstone should not be retained")
	}
}

func TestOfflineStaleWriteDuplicateAckAndSchemaVersionRules(t *testing.T) {
	if !StaleWrite(4, 5) {
		t.Fatal("older base version should be stale")
	}
	if StaleWrite(5, 5) {
		t.Fatal("equal versions should not be stale")
	}
	if StaleWrite(6, 5) {
		t.Fatal("newer base version should not be stale")
	}

	previous := SyncCursor{Token: "branch-west:1042", Version: 1042}
	if !DuplicateAck(previous, SyncCursor{Token: "branch-west:1042", Version: 1042}) {
		t.Fatal("matching cursor should be treated as duplicate ack")
	}
	if DuplicateAck(previous, SyncCursor{Token: "branch-west:1043", Version: 1043}) {
		t.Fatal("different cursor should not be duplicate ack")
	}

	accepted := []string{"2026.06", "2026.07", "2026.08"}
	if SkippedSchemaVersion("2026.07", "2026.08", accepted) {
		t.Fatal("present intermediate version should not be skipped")
	}
	if !SkippedSchemaVersion("2026.05", "2026.08", accepted) {
		t.Fatal("missing intermediate version should be skipped")
	}
	if !SkippedSchemaVersion("", "2026.08", accepted) {
		t.Fatal("empty client version should be treated as skipped")
	}
}

func TestOfflineRetentionAndGCPolicy(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tombstones := []OfflineTombstone{
		{
			ID:            "keep",
			EntityID:      "INV-1",
			RetainUntil:   now.Add(time.Hour),
			SchemaVersion: "2026.08",
		},
		{
			ID:            "drop",
			EntityID:      "INV-2",
			RetainUntil:   now.Add(-time.Minute),
			SchemaVersion: "2026.08",
		},
	}

	kept := PruneExpiredTombstones(tombstones, now)
	if len(kept) != 1 {
		t.Fatalf("PruneExpiredTombstones() len = %d, want 1", len(kept))
	}
	if kept[0].ID != "keep" {
		t.Fatalf("PruneExpiredTombstones() kept %q, want keep", kept[0].ID)
	}

	retryable := OfflineConflict{ResolutionMode: "retry"}
	mergeable := OfflineConflict{ResolutionMode: "merge"}
	manual := OfflineConflict{ResolutionMode: "manual_review"}
	discarded := OfflineConflict{ResolutionMode: "discard"}

	for _, conflict := range []OfflineConflict{retryable, mergeable, manual} {
		if !RetainConflict(conflict) {
			t.Fatalf("RetainConflict(%q) = false, want true", conflict.ResolutionMode)
		}
	}
	if RetainConflict(discarded) {
		t.Fatal("discarded conflict should not be retained")
	}
}

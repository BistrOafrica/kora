package contract

import (
	"encoding/json"
	"time"
)

// OfflineOperationStatus is the local and server-side lifecycle of an offline
// operation. The RFC requires rejected operations to remain visible as conflict
// records rather than disappear.
type OfflineOperationStatus string

const (
	OfflineOperationQueued    OfflineOperationStatus = "queued"
	OfflineOperationSent      OfflineOperationStatus = "sent"
	OfflineOperationAcked     OfflineOperationStatus = "acked"
	OfflineOperationConflict  OfflineOperationStatus = "conflict"
	OfflineOperationRejected  OfflineOperationStatus = "rejected"
	OfflineOperationDiscarded OfflineOperationStatus = "discarded"
)

// OfflineSchemaGate records whether a client schema bundle is accepted.
type OfflineSchemaGate string

const (
	OfflineSchemaGateAccepted OfflineSchemaGate = "accepted"
	OfflineSchemaGateRejected OfflineSchemaGate = "rejected"
	OfflineSchemaGateStale    OfflineSchemaGate = "stale"
)

// OfflineOperation records one device/branch operation for later sync.
type OfflineOperation struct {
	ID            string                 `json:"id"`
	DeviceID      string                 `json:"device_id"`
	BranchID      string                 `json:"branch_id"`
	Site          string                 `json:"site"`
	EntityType    string                 `json:"entity_type"`
	EntityID      string                 `json:"entity_id,omitempty"`
	OperationType string                 `json:"operation_type"`
	BaseVersion   int                    `json:"base_version"`
	OccurredAt    time.Time              `json:"occurred_at"`
	SchemaVersion string                 `json:"schema_version"`
	Status        OfflineOperationStatus `json:"status"`
	Payload       json.RawMessage        `json:"payload,omitempty"`
	Metadata      map[string]string      `json:"metadata,omitempty"`
}

// OfflineConflict captures a rejected or stale operation and the server-side
// reason that must stay auditable.
type OfflineConflict struct {
	ID             string          `json:"id"`
	OperationID    string          `json:"operation_id"`
	DeviceID       string          `json:"device_id"`
	BranchID       string          `json:"branch_id"`
	Site           string          `json:"site"`
	EntityType     string          `json:"entity_type"`
	EntityID       string          `json:"entity_id,omitempty"`
	ReasonCode     Code            `json:"reason_code"`
	ResolutionMode string          `json:"resolution_mode"`
	ServerVersion  int             `json:"server_version"`
	RecordedAt     time.Time       `json:"recorded_at"`
	Snapshot       json.RawMessage `json:"snapshot,omitempty"`
}

// OfflineTombstone records a retained delete marker. Tombstones remain visible
// for the supported offline window so deletes can be replayed, retried, and
// rejected deterministically.
type OfflineTombstone struct {
	ID            string    `json:"id"`
	DeviceID      string    `json:"device_id"`
	BranchID      string    `json:"branch_id"`
	Site          string    `json:"site"`
	EntityType    string    `json:"entity_type"`
	EntityID      string    `json:"entity_id"`
	DeletedAt     time.Time `json:"deleted_at"`
	RetainUntil   time.Time `json:"retain_until"`
	SchemaVersion string    `json:"schema_version"`
}

// SyncCursor is the opaque cursor shared by pull and acknowledgement flows.
type SyncCursor struct {
	Token    string    `json:"token"`
	BranchID string    `json:"branch_id"`
	DeviceID string    `json:"device_id"`
	Version  int       `json:"version"`
	At       time.Time `json:"at"`
}

// OfflineSyncBatch groups operations under one schema gate result.
type OfflineSyncBatch struct {
	SchemaVersion string             `json:"schema_version"`
	Gate          OfflineSchemaGate  `json:"gate"`
	Operations    []OfflineOperation `json:"operations"`
	Conflicts     []OfflineConflict  `json:"conflicts,omitempty"`
	Cursor        SyncCursor         `json:"cursor"`
	NextCursor    SyncCursor         `json:"next_cursor,omitempty"`
}

// AcceptsOfflineSchema reports whether the server should accept a bundle.
func AcceptsOfflineSchema(gate OfflineSchemaGate) bool {
	return gate == OfflineSchemaGateAccepted
}

// ConflictResolutionRetryable reports whether a rejected operation should stay
// available for another sync attempt or manual resolution.
func ConflictResolutionRetryable(mode string) bool {
	switch mode {
	case "retry", "merge", "manual_review":
		return true
	default:
		return false
	}
}

// TombstoneRetained reports whether the tombstone still falls within the
// supported offline retention window.
func TombstoneRetained(t OfflineTombstone, now time.Time) bool {
	if t.RetainUntil.IsZero() {
		return false
	}
	return !now.After(t.RetainUntil)
}

// PruneExpiredTombstones removes tombstones that have aged past their
// retention window so GC can keep the retained delete set bounded.
func PruneExpiredTombstones(tombstones []OfflineTombstone, now time.Time) []OfflineTombstone {
	kept := make([]OfflineTombstone, 0, len(tombstones))
	for _, t := range tombstones {
		if TombstoneRetained(t, now) {
			kept = append(kept, t)
		}
	}
	return kept
}

// RetainConflict reports whether a rejected operation should remain visible to
// operators or the retry loop.
func RetainConflict(conflict OfflineConflict) bool {
	return ConflictResolutionRetryable(conflict.ResolutionMode)
}

// StaleWrite reports whether the incoming offline write is based on an older
// version than the server currently accepts.
func StaleWrite(baseVersion, serverVersion int) bool {
	return baseVersion > 0 && baseVersion < serverVersion
}

// DuplicateAck reports whether a cursor acknowledgement is a repeat of a
// previously acknowledged version.
func DuplicateAck(previous, next SyncCursor) bool {
	return previous.Token != "" && previous.Token == next.Token && previous.Version == next.Version
}

// SkippedSchemaVersion reports whether the client jumped over a required
// intermediate schema version.
func SkippedSchemaVersion(client, current string, accepted []string) bool {
	if client == "" || current == "" {
		return true
	}
	if client == current {
		return false
	}
	for _, version := range accepted {
		if version == client {
			return false
		}
	}
	return true
}

// OperationCanTransition reports whether an offline operation can move to the
// next local lifecycle state without losing audit visibility.
func OperationCanTransition(from, to OfflineOperationStatus) bool {
	switch from {
	case OfflineOperationQueued:
		return to == OfflineOperationSent || to == OfflineOperationRejected || to == OfflineOperationDiscarded
	case OfflineOperationSent:
		return to == OfflineOperationAcked || to == OfflineOperationConflict || to == OfflineOperationRejected
	case OfflineOperationAcked:
		return false
	case OfflineOperationConflict:
		return to == OfflineOperationQueued || to == OfflineOperationDiscarded
	case OfflineOperationRejected:
		return to == OfflineOperationDiscarded
	case OfflineOperationDiscarded:
		return false
	default:
		return false
	}
}

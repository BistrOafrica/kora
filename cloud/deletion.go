package cloud

import "time"

// DeletionState tracks a confirmed, auditable deletion workflow.
type DeletionState string

const (
	DeletionRequested DeletionState = "requested"
	DeletionRevoking  DeletionState = "revoking"
	DeletionDraining  DeletionState = "draining"
	DeletionDeleting  DeletionState = "deleting"
	DeletionVerifying DeletionState = "verifying"
	DeletionCompleted DeletionState = "completed"
	DeletionFailed    DeletionState = "failed"
)

// DeletionWorkflow records the durable state of an auditable deletion request.
type DeletionWorkflow struct {
	ID                  string        `json:"id"`
	OrganizationID      string        `json:"organization_id"`
	Site                string        `json:"site"`
	Region              string        `json:"region"`
	State               DeletionState `json:"state"`
	OperationID         string        `json:"operation_id"`
	ManifestRef         string        `json:"manifest_ref"`
	CredentialRefs      []string      `json:"credential_refs,omitempty"`
	BackupVerified      bool          `json:"backup_verified"`
	ObjectStoreVerified bool          `json:"object_store_verified"`
	RetentionPolicyRef  string        `json:"retention_policy_ref,omitempty"`
	RequestedAt         time.Time     `json:"requested_at"`
	CompletedAt         time.Time     `json:"completed_at,omitempty"`
	LastError           string        `json:"last_error,omitempty"`
}

// IsolationBoundary enumerates the systems that must be proven isolated.
type IsolationBoundary struct {
	SQL         bool `json:"sql"`
	NATS        bool `json:"nats"`
	KeyValue    bool `json:"key_value"`
	ObjectStore bool `json:"object_store"`
	Cache       bool `json:"cache"`
	Logs        bool `json:"logs"`
	Traces      bool `json:"traces"`
	Metrics     bool `json:"metrics"`
	Backups     bool `json:"backups"`
	Credentials bool `json:"credentials"`
}

// RPOEvidence captures the accepted data-loss window for a deployment.
type RPOEvidence struct {
	DeploymentID string    `json:"deployment_id"`
	Region       string    `json:"region"`
	RPOSeconds   int       `json:"rpo_seconds"`
	RTOSeconds   int       `json:"rto_seconds"`
	LastVerified time.Time `json:"last_verified"`
	VerifiedBy   string    `json:"verified_by,omitempty"`
	BackupRef    string    `json:"backup_ref,omitempty"`
	RestoreRef   string    `json:"restore_ref,omitempty"`
}

// DeletionCanTransition validates deletion workflow movement.
func DeletionCanTransition(from, to DeletionState) bool {
	switch from {
	case DeletionRequested:
		return to == DeletionRevoking || to == DeletionFailed
	case DeletionRevoking:
		return to == DeletionDraining || to == DeletionFailed
	case DeletionDraining:
		return to == DeletionDeleting || to == DeletionFailed
	case DeletionDeleting:
		return to == DeletionVerifying || to == DeletionFailed
	case DeletionVerifying:
		return to == DeletionCompleted || to == DeletionFailed
	case DeletionCompleted:
		return false
	case DeletionFailed:
		return to == DeletionRequested
	default:
		return false
	}
}

// IsolationBoundaryComplete reports whether all declared storage, routing, and
// audit boundaries are covered.
func IsolationBoundaryComplete(boundary IsolationBoundary) bool {
	return boundary.SQL && boundary.NATS && boundary.KeyValue && boundary.ObjectStore && boundary.Cache && boundary.Logs && boundary.Traces && boundary.Metrics && boundary.Backups && boundary.Credentials
}

// RPOEvidenceValid reports whether the deployment has verified backup/restore
// evidence within a bounded recovery window.
func RPOEvidenceValid(e RPOEvidence) bool {
	return e.DeploymentID != "" && e.Region != "" && e.RPOSeconds > 0 && e.RTOSeconds > 0 && !e.LastVerified.IsZero() && e.BackupRef != "" && e.RestoreRef != ""
}

// DeletionEvidenceComplete reports whether a deletion workflow has the
// evidence required by the RFC.
func DeletionEvidenceComplete(w DeletionWorkflow) bool {
	return w.State == DeletionCompleted && w.BackupVerified && w.ObjectStoreVerified && w.ManifestRef != "" && w.OperationID != ""
}

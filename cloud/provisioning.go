package cloud

import "time"

// ProvisioningState tracks durable control-plane provisioning progress.
type ProvisioningState string

const (
	ProvisioningRequested         ProvisioningState = "requested"
	ProvisioningValidating        ProvisioningState = "validating"
	ProvisioningAllocating        ProvisioningState = "allocating"
	ProvisioningProvisioning      ProvisioningState = "provisioning"
	ProvisioningValidatingRuntime ProvisioningState = "validating_runtime"
	ProvisioningActive            ProvisioningState = "active"
	ProvisioningFailed            ProvisioningState = "failed"
)

// ProvisioningJob records a resumable, idempotent control-plane operation.
type ProvisioningJob struct {
	ID                string            `json:"id"`
	Site              string            `json:"site"`
	Resource          string            `json:"resource"`
	State             ProvisioningState `json:"state"`
	OperationID       string            `json:"operation_id"`
	IdempotencyKey    string            `json:"idempotency_key"`
	InputFingerprint  string            `json:"input_fingerprint"`
	ProviderRequestID string            `json:"provider_request_id,omitempty"`
	OutputID          string            `json:"output_id,omitempty"`
	Attempt           int               `json:"attempt"`
	LeaseOwner        string            `json:"lease_owner,omitempty"`
	LeaseUntil        time.Time         `json:"lease_until,omitempty"`
	RetryAt           time.Time         `json:"retry_at,omitempty"`
	LastError         string            `json:"last_error,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at,omitempty"`
}

// ProvisioningCheckpoint captures the last successfully completed stage so a
// restart can resume without re-running completed work.
type ProvisioningCheckpoint struct {
	JobID      string    `json:"job_id"`
	Stage      string    `json:"stage"`
	Completed  bool      `json:"completed"`
	RecordedAt time.Time `json:"recorded_at"`
}

// ProvisioningCanTransition validates durable provisioning movement.
func ProvisioningCanTransition(from, to ProvisioningState) bool {
	switch from {
	case ProvisioningRequested:
		return to == ProvisioningValidating || to == ProvisioningFailed
	case ProvisioningValidating:
		return to == ProvisioningAllocating || to == ProvisioningFailed
	case ProvisioningAllocating:
		return to == ProvisioningProvisioning || to == ProvisioningFailed
	case ProvisioningProvisioning:
		return to == ProvisioningValidatingRuntime || to == ProvisioningFailed
	case ProvisioningValidatingRuntime:
		return to == ProvisioningActive || to == ProvisioningFailed
	case ProvisioningActive:
		return false
	case ProvisioningFailed:
		return to == ProvisioningRequested
	default:
		return false
	}
}

// ProvisioningIsResumable reports whether a job may safely continue after a
// restart without losing idempotency or creating a duplicate resource.
func ProvisioningIsResumable(job ProvisioningJob) bool {
	return job.OperationID != "" && job.IdempotencyKey != "" && job.InputFingerprint != "" && job.State != ProvisioningActive
}

// ProvisioningIsDuplicate reports whether a restarted reconcile is the same
// durable job rather than a new provisioning request.
func ProvisioningIsDuplicate(previous, next ProvisioningJob) bool {
	return previous.Site == next.Site &&
		previous.Resource == next.Resource &&
		previous.OperationID == next.OperationID &&
		previous.IdempotencyKey == next.IdempotencyKey &&
		previous.InputFingerprint == next.InputFingerprint
}

// ProvisioningCheckpointCanResume reports whether the checkpoint allows a
// restarted controller to skip already completed stages.
func ProvisioningCheckpointCanResume(cp ProvisioningCheckpoint) bool {
	return cp.JobID != "" && cp.Stage != "" && cp.Completed
}

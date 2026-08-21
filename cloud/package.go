package cloud

import "time"

// PackageLifecycleState tracks package registry status for deployment rollout.
type PackageLifecycleState string

const (
	PackageLifecycleUploaded PackageLifecycleState = "uploaded"
	PackageLifecycleVerified PackageLifecycleState = "verified"
	PackageLifecycleActive   PackageLifecycleState = "active"
	PackageLifecycleRetired  PackageLifecycleState = "retired"
	PackageLifecycleBlocked  PackageLifecycleState = "blocked"
)

// PackageArtifact records an immutable package binary or manifest reference.
type PackageArtifact struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Digest       string    `json:"digest"`
	SignatureRef string    `json:"signature_ref,omitempty"`
	SizeBytes    int64     `json:"size_bytes"`
	UploadedAt   time.Time `json:"uploaded_at"`
}

// PackageRegistryEntry stores verification and lifecycle metadata for one
// package version.
type PackageRegistryEntry struct {
	ID            string                `json:"id"`
	Artifact      PackageArtifact       `json:"artifact"`
	State         PackageLifecycleState `json:"state"`
	Compatibility []string              `json:"compatibility,omitempty"`
	DeploymentIDs []string              `json:"deployment_ids,omitempty"`
	VerifiedAt    time.Time             `json:"verified_at,omitempty"`
	ActivatedAt   time.Time             `json:"activated_at,omitempty"`
	RetiredAt     time.Time             `json:"retired_at,omitempty"`
	LastError     string                `json:"last_error,omitempty"`
	OperationID   string                `json:"operation_id,omitempty"`
}

// DeploymentRolloutState tracks deployment rollout lifecycle.
type DeploymentRolloutState string

const (
	DeploymentRolloutRequested DeploymentRolloutState = "requested"
	DeploymentRolloutRunning   DeploymentRolloutState = "running"
	DeploymentRolloutCompleted DeploymentRolloutState = "completed"
	DeploymentRolloutFailed    DeploymentRolloutState = "failed"
	DeploymentRolloutPaused    DeploymentRolloutState = "paused"
)

// DeploymentRollout captures the durable state of a package rollout.
type DeploymentRollout struct {
	ID           string                 `json:"id"`
	DeploymentID string                 `json:"deployment_id"`
	PackageID    string                 `json:"package_id"`
	FromVersion  string                 `json:"from_version,omitempty"`
	ToVersion    string                 `json:"to_version"`
	State        DeploymentRolloutState `json:"state"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  time.Time              `json:"completed_at,omitempty"`
	OperationID  string                 `json:"operation_id,omitempty"`
	LastError    string                 `json:"last_error,omitempty"`
}

// PackageLifecycleCanTransition validates package registry movement.
func PackageLifecycleCanTransition(from, to PackageLifecycleState) bool {
	switch from {
	case PackageLifecycleUploaded:
		return to == PackageLifecycleVerified || to == PackageLifecycleBlocked
	case PackageLifecycleVerified:
		return to == PackageLifecycleActive || to == PackageLifecycleRetired || to == PackageLifecycleBlocked
	case PackageLifecycleActive:
		return to == PackageLifecycleRetired || to == PackageLifecycleBlocked
	case PackageLifecycleRetired:
		return false
	case PackageLifecycleBlocked:
		return to == PackageLifecycleUploaded
	default:
		return false
	}
}

// DeploymentRolloutCanTransition validates rollout movement.
func DeploymentRolloutCanTransition(from, to DeploymentRolloutState) bool {
	switch from {
	case DeploymentRolloutRequested:
		return to == DeploymentRolloutRunning || to == DeploymentRolloutFailed || to == DeploymentRolloutPaused
	case DeploymentRolloutRunning:
		return to == DeploymentRolloutCompleted || to == DeploymentRolloutFailed || to == DeploymentRolloutPaused
	case DeploymentRolloutPaused:
		return to == DeploymentRolloutRunning || to == DeploymentRolloutFailed
	case DeploymentRolloutCompleted:
		return false
	case DeploymentRolloutFailed:
		return to == DeploymentRolloutRequested
	default:
		return false
	}
}

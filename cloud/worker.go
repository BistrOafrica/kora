package cloud

import "time"

// WorkerPlacementState tracks the lifecycle of a managed worker pool.
type WorkerPlacementState string

const (
	WorkerPlacementRequested WorkerPlacementState = "requested"
	WorkerPlacementPlacing   WorkerPlacementState = "placing"
	WorkerPlacementActive    WorkerPlacementState = "active"
	WorkerPlacementScaling   WorkerPlacementState = "scaling"
	WorkerPlacementDraining  WorkerPlacementState = "draining"
	WorkerPlacementFailed    WorkerPlacementState = "failed"
)

// WorkerPlacementPolicy defines the desired runtime capacity and isolation
// constraints for a deployment.
type WorkerPlacementPolicy struct {
	ID                string   `json:"id"`
	DeploymentID      string   `json:"deployment_id"`
	Region            string   `json:"region"`
	MinWorkers        int      `json:"min_workers"`
	MaxWorkers        int      `json:"max_workers"`
	TargetQueueDepth  int      `json:"target_queue_depth"`
	TargetLagSeconds  int      `json:"target_lag_seconds"`
	AllowedPools      []string `json:"allowed_pools,omitempty"`
	DrainingWindowSec int      `json:"draining_window_seconds,omitempty"`
}

// WorkerPlacement records the current managed worker pool status.
type WorkerPlacement struct {
	ID           string               `json:"id"`
	DeploymentID string               `json:"deployment_id"`
	PolicyID     string               `json:"policy_id"`
	PoolName     string               `json:"pool_name"`
	Region       string               `json:"region"`
	State        WorkerPlacementState `json:"state"`
	Desired      int                  `json:"desired"`
	Ready        int                  `json:"ready"`
	Unavailable  int                  `json:"unavailable"`
	ObservedAt   time.Time            `json:"observed_at"`
	OperationID  string               `json:"operation_id,omitempty"`
	LastError    string               `json:"last_error,omitempty"`
	LastScaledAt time.Time            `json:"last_scaled_at,omitempty"`
}

// WorkerScalingEvent records a bounded autoscaling adjustment.
type WorkerScalingEvent struct {
	ID           string    `json:"id"`
	PlacementID  string    `json:"placement_id"`
	DeploymentID string    `json:"deployment_id"`
	From         int       `json:"from"`
	To           int       `json:"to"`
	Reason       string    `json:"reason"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// WorkerPlacementCanTransition validates lifecycle movement.
func WorkerPlacementCanTransition(from, to WorkerPlacementState) bool {
	switch from {
	case WorkerPlacementRequested:
		return to == WorkerPlacementPlacing || to == WorkerPlacementFailed
	case WorkerPlacementPlacing:
		return to == WorkerPlacementActive || to == WorkerPlacementFailed
	case WorkerPlacementActive:
		return to == WorkerPlacementScaling || to == WorkerPlacementDraining || to == WorkerPlacementFailed
	case WorkerPlacementScaling:
		return to == WorkerPlacementActive || to == WorkerPlacementDraining || to == WorkerPlacementFailed
	case WorkerPlacementDraining:
		return to == WorkerPlacementActive || to == WorkerPlacementFailed
	case WorkerPlacementFailed:
		return to == WorkerPlacementRequested
	default:
		return false
	}
}

// WorkerPlacementNeedsScale reports whether the current ready count violates
// the desired bounds.
func WorkerPlacementNeedsScale(p WorkerPlacement, policy WorkerPlacementPolicy) bool {
	if p.Ready < policy.MinWorkers {
		return true
	}
	if p.Ready > policy.MaxWorkers && policy.MaxWorkers > 0 {
		return true
	}
	if p.Desired != p.Ready {
		return true
	}
	return false
}

// WorkerPlacementSupportsAutoscaling reports whether the policy has the data
// needed for bounded scaling decisions.
func WorkerPlacementSupportsAutoscaling(policy WorkerPlacementPolicy) bool {
	return policy.MinWorkers > 0 && policy.MaxWorkers >= policy.MinWorkers && policy.TargetQueueDepth > 0
}

package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWorkerPlacementJSONShape(t *testing.T) {
	policy := WorkerPlacementPolicy{
		ID:               "policy-1",
		DeploymentID:     "nats-1",
		Region:           "af-south",
		MinWorkers:       2,
		MaxWorkers:       8,
		TargetQueueDepth: 25,
		TargetLagSeconds: 60,
		AllowedPools:     []string{"default", "priority"},
	}

	b, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	for _, key := range []string{"id", "deployment_id", "region", "min_workers", "max_workers", "target_queue_depth", "target_lag_seconds"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("WorkerPlacementPolicy missing key %q: %s", key, b)
		}
	}

	placement := WorkerPlacement{
		ID:           "placement-1",
		DeploymentID: policy.DeploymentID,
		PolicyID:     policy.ID,
		PoolName:     "default",
		Region:       policy.Region,
		State:        WorkerPlacementRequested,
		Desired:      2,
		Ready:        1,
		Unavailable:  1,
		ObservedAt:   time.Date(2026, 8, 13, 12, 30, 0, 0, time.UTC),
		OperationID:  "op-1",
	}

	b, err = json.Marshal(placement)
	if err != nil {
		t.Fatalf("marshal placement: %v", err)
	}
	for _, key := range []string{"policy_id", "pool_name", "state", "desired", "ready", "unavailable", "observed_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("WorkerPlacement missing key %q: %s", key, b)
		}
	}

	event := WorkerScalingEvent{
		ID:           "scale-1",
		PlacementID:  placement.ID,
		DeploymentID: placement.DeploymentID,
		From:         2,
		To:           4,
		Reason:       "queue_depth",
		RecordedAt:   placement.ObservedAt,
	}
	b, err = json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	for _, key := range []string{"placement_id", "deployment_id", "from", "to", "reason", "recorded_at"} {
		if !strings.Contains(string(b), `"`+key+`"`) {
			t.Fatalf("WorkerScalingEvent missing key %q: %s", key, b)
		}
	}
}

func TestWorkerPlacementTransitionsAndAutoscaling(t *testing.T) {
	if !WorkerPlacementCanTransition(WorkerPlacementRequested, WorkerPlacementPlacing) {
		t.Fatal("requested should transition to placing")
	}
	if WorkerPlacementCanTransition(WorkerPlacementRequested, WorkerPlacementActive) {
		t.Fatal("requested should not skip to active")
	}
	if !WorkerPlacementSupportsAutoscaling(WorkerPlacementPolicy{MinWorkers: 2, MaxWorkers: 8, TargetQueueDepth: 25}) {
		t.Fatal("valid policy should support autoscaling")
	}
	if WorkerPlacementSupportsAutoscaling(WorkerPlacementPolicy{MinWorkers: 0, MaxWorkers: 8, TargetQueueDepth: 25}) {
		t.Fatal("zero min workers should not support autoscaling")
	}

	policy := WorkerPlacementPolicy{MinWorkers: 2, MaxWorkers: 4, TargetQueueDepth: 25}
	if !WorkerPlacementNeedsScale(WorkerPlacement{Desired: 2, Ready: 1}, policy) {
		t.Fatal("ready below min should need scale")
	}
	if !WorkerPlacementNeedsScale(WorkerPlacement{Desired: 5, Ready: 5}, policy) {
		t.Fatal("ready above max should need scale")
	}
	if WorkerPlacementNeedsScale(WorkerPlacement{Desired: 3, Ready: 3}, policy) {
		t.Fatal("balanced placement should not need scale")
	}
}

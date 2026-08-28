package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asenawritescode/kora/workflow"
)

// RunActor bridges durable AI run state with the workflow lease/timer runtime.
type RunActor struct {
	Runtime *workflow.Runtime
	DB      *sql.DB
	Owner   string
}

func NewRunActor(db *sql.DB, owner string, handler workflow.StepHandler) *RunActor {
	return &RunActor{
		Runtime: workflow.NewRuntime(db, handler),
		DB:      db,
		Owner:   owner,
	}
}

func (a *RunActor) Ensure(ctx context.Context) error {
	if a.Runtime == nil {
		return fmt.Errorf("run actor runtime not configured")
	}
	return a.Runtime.Ensure(ctx)
}

// Resume advances a run snapshot using the shared workflow fencing/timer contracts.
func (a *RunActor) Resume(ctx context.Context, site, runID string) (workflow.InstanceState, error) {
	if a.Runtime == nil {
		return workflow.InstanceState{}, fmt.Errorf("run actor runtime not configured")
	}
	return a.Runtime.Resume(ctx, site, "ai_run", runID, a.Owner)
}

// Retry schedules the run for another attempt using the shared timer scheduler.
func (a *RunActor) Retry(ctx context.Context, state workflow.InstanceState, after time.Duration) error {
	if a.Runtime == nil {
		return fmt.Errorf("run actor runtime not configured")
	}
	return a.Runtime.Retry(ctx, state, after)
}

// DeadLetter marks a run as terminal and keeps the durable audit payload.
func (a *RunActor) DeadLetter(ctx context.Context, state workflow.InstanceState, cause string) error {
	if a.Runtime == nil {
		return fmt.Errorf("run actor runtime not configured")
	}
	return a.Runtime.DeadLetter(ctx, state, cause)
}

func RunInstanceStateFromRecord(rec RunRecord) workflow.InstanceState {
	payload := json.RawMessage(`{}`)
	if rec.Summary != "" {
		payload = json.RawMessage(fmt.Sprintf(`{"summary":%q}`, rec.Summary))
	}
	return workflow.InstanceState{
		Site:       rec.Site,
		Workflow:   "ai_run",
		InstanceID: rec.ID,
		TimerID:    rec.CurrentStepID,
		Status:     rec.Status,
		Payload:    payload,
		UpdatedAt:  rec.UpdatedAt,
	}
}

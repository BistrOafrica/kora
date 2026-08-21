package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asenawritescode/kora/doctype"
)

// StepHandler executes one workflow step against a document snapshot.
type StepHandler func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error)

// Runtime wires lease/fencing, timers, and step execution together.
type Runtime struct {
	LeaseStore     *LeaseStore
	TimerScheduler *TimerScheduler
	StepHandler    StepHandler
	LeaseTTL       time.Duration
}

// Ensure prepares the workflow tables needed by the runtime.
func (r *Runtime) Ensure(ctx context.Context) error {
	if r.LeaseStore != nil {
		if err := r.LeaseStore.EnsureTables(ctx); err != nil {
			return err
		}
	}
	if r.TimerScheduler != nil {
		if err := r.TimerScheduler.EnsureTables(ctx); err != nil {
			return err
		}
	}
	if err := EnsureAuditTables(ctx, r.db()); err != nil {
		return err
	}
	return nil
}

// Resume acquires the actor lease, executes the step handler, and persists the
// resulting state. It returns the updated state for the caller to continue from.
func (r *Runtime) Resume(ctx context.Context, site, workflowName, instanceID, owner string) (InstanceState, error) {
	if r.LeaseStore == nil || r.StepHandler == nil {
		return InstanceState{}, fmt.Errorf("workflow runtime not configured")
	}
	ttl := r.LeaseTTL
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	state, err := r.LeaseStore.Acquire(ctx, site, workflowName, instanceID, owner, ttl)
	if err != nil {
		return InstanceState{}, err
	}
	var doc doctype.Document
	if len(state.Payload) > 0 {
		_ = json.Unmarshal(state.Payload, &doc)
	}
	next, err := r.StepHandler(ctx, state, &doc)
	if err != nil {
		_ = r.LeaseStore.Release(context.Background(), site, workflowName, instanceID, owner)
		return InstanceState{}, err
	}
	if next.Payload == nil {
		next.Payload = state.Payload
	}
	next.Site = site
	next.Workflow = workflowName
	next.InstanceID = instanceID
	next.LeaseOwner = owner
	next.LeaseUntil = time.Now().UTC().Add(ttl)
	next.UpdatedAt = time.Now().UTC()
	if err := r.LeaseStore.Upsert(ctx, next); err != nil {
		return InstanceState{}, err
	}
	_ = RecordTransition(ctx, r.db(), TransitionAudit{
		Site:       site,
		Workflow:   workflowName,
		InstanceID: instanceID,
		StepID:     next.TimerID,
		Kind:       "resume",
		FromState:  state.Status,
		ToState:    next.Status,
	})
	if r.TimerScheduler != nil && !next.NextWakeAt.IsZero() {
		timerID := next.TimerID
		if timerID == "" {
			timerID = fmt.Sprintf("%s:%s:%s", site, workflowName, instanceID)
		}
		next.TimerID = timerID
		_ = r.TimerScheduler.Schedule(ctx, site, workflowName, instanceID, timerID, next.NextWakeAt, next.Payload)
	}
	return next, nil
}

// Pause records a paused state and clears the lease.
func (r *Runtime) Pause(ctx context.Context, state InstanceState, owner string) error {
	if r.LeaseStore == nil {
		return fmt.Errorf("lease store not configured")
	}
	state.Status = "paused"
	state.LeaseOwner = ""
	state.LeaseUntil = time.Time{}
	state.UpdatedAt = time.Now().UTC()
	if err := r.LeaseStore.Upsert(ctx, state); err != nil {
		return err
	}
	_ = RecordTransition(ctx, r.db(), TransitionAudit{
		Site:       state.Site,
		Workflow:   state.Workflow,
		InstanceID: state.InstanceID,
		StepID:     state.TimerID,
		Kind:       "pause",
		FromState:  "",
		ToState:    state.Status,
	})
	if r.TimerScheduler != nil {
		_ = r.TimerScheduler.Delete(ctx, state.Site, state.Workflow, state.InstanceID)
	}
	return r.LeaseStore.Release(ctx, state.Site, state.Workflow, state.InstanceID, owner)
}

// Retry marks an instance for retry and pushes a new wakeup.
func (r *Runtime) Retry(ctx context.Context, state InstanceState, after time.Duration) error {
	if r.TimerScheduler == nil || r.LeaseStore == nil {
		return fmt.Errorf("timer scheduler not configured")
	}
	state.Status = "retrying"
	state.NextWakeAt = time.Now().UTC().Add(after)
	state.UpdatedAt = time.Now().UTC()
	if err := r.LeaseStore.Upsert(ctx, state); err != nil {
		return err
	}
	_ = RecordTransition(ctx, r.db(), TransitionAudit{
		Site:       state.Site,
		Workflow:   state.Workflow,
		InstanceID: state.InstanceID,
		StepID:     state.TimerID,
		Kind:       "retry",
		FromState:  "",
		ToState:    state.Status,
	})
	timerID := state.TimerID
	if timerID == "" {
		timerID = fmt.Sprintf("%s:%s:%s", state.Site, state.Workflow, state.InstanceID)
	}
	state.TimerID = timerID
	return r.TimerScheduler.Schedule(ctx, state.Site, state.Workflow, state.InstanceID, timerID, state.NextWakeAt, state.Payload)
}

// DeadLetter persists a terminal failure state for operator recovery.
func (r *Runtime) DeadLetter(ctx context.Context, state InstanceState, cause string) error {
	if r.LeaseStore == nil {
		return fmt.Errorf("lease store not configured")
	}
	state.Status = "dead_letter"
	if state.Payload == nil {
		state.Payload = json.RawMessage(`{}`)
	}
	state.UpdatedAt = time.Now().UTC()
	meta := map[string]any{"cause": cause, "payload": json.RawMessage(state.Payload)}
	b, _ := json.Marshal(meta)
	state.Payload = b
	if err := r.LeaseStore.Upsert(ctx, state); err != nil {
		return err
	}
	_ = RecordTransition(ctx, r.db(), TransitionAudit{
		Site:       state.Site,
		Workflow:   state.Workflow,
		InstanceID: state.InstanceID,
		StepID:     state.TimerID,
		Kind:       "dead_letter",
		FromState:  "",
		ToState:    state.Status,
		Cause:      cause,
	})
	if r.TimerScheduler != nil {
		_ = r.TimerScheduler.Delete(ctx, state.Site, state.Workflow, state.InstanceID)
	}
	return nil
}

// ProcessDue resumes any due workflow instances using the supplied owner identity.
// It is a small recovery loop for timer-driven workflows and returns the states
// that were successfully resumed.
func (r *Runtime) ProcessDue(ctx context.Context, now time.Time, limit int, owner string) ([]InstanceState, error) {
	if r.TimerScheduler == nil || r.LeaseStore == nil || r.StepHandler == nil {
		return nil, fmt.Errorf("workflow runtime not configured")
	}
	due, err := r.TimerScheduler.Due(ctx, now, limit)
	if err != nil {
		return nil, err
	}
	out := make([]InstanceState, 0, len(due))
	seen := make(map[string]struct{}, len(due))
	for _, st := range due {
		timerKey := st.TimerID
		if timerKey == "" {
			timerKey = st.WakeAt.UTC().Format(time.RFC3339Nano)
		}
		key := st.Site + "\x00" + st.Workflow + "\x00" + st.InstanceID + "\x00" + timerKey
		if _, ok := seen[key]; ok {
			if st.TimerID != "" {
				_ = r.TimerScheduler.DeleteID(ctx, st.Site, st.Workflow, st.InstanceID, st.TimerID)
			} else if !st.WakeAt.IsZero() {
				_ = r.TimerScheduler.DeleteAt(ctx, st.Site, st.Workflow, st.InstanceID, st.WakeAt)
			} else {
				_ = r.TimerScheduler.Delete(ctx, st.Site, st.Workflow, st.InstanceID)
			}
			continue
		}
		seen[key] = struct{}{}
		current, err := r.LeaseStore.Load(ctx, st.Site, st.Workflow, st.InstanceID)
		if err != nil {
			_ = r.TimerScheduler.DeleteAt(ctx, st.Site, st.Workflow, st.InstanceID, st.WakeAt)
			continue
		}
		if !current.NextWakeAt.IsZero() && !current.NextWakeAt.Equal(st.WakeAt) {
			if st.TimerID != "" {
				_ = r.TimerScheduler.DeleteID(ctx, st.Site, st.Workflow, st.InstanceID, st.TimerID)
			} else {
				_ = r.TimerScheduler.DeleteAt(ctx, st.Site, st.Workflow, st.InstanceID, st.WakeAt)
			}
			continue
		}
		if current.LeaseOwner != "" && current.LeaseOwner != owner {
			if st.TimerID != "" {
				_ = r.TimerScheduler.DeleteID(ctx, st.Site, st.Workflow, st.InstanceID, st.TimerID)
			} else {
				_ = r.TimerScheduler.DeleteAt(ctx, st.Site, st.Workflow, st.InstanceID, st.WakeAt)
			}
			continue
		}
		next, err := r.Resume(ctx, st.Site, st.Workflow, st.InstanceID, owner)
		if err != nil {
			return out, err
		}
		if st.TimerID != "" {
			_ = r.TimerScheduler.DeleteID(ctx, st.Site, st.Workflow, st.InstanceID, st.TimerID)
		} else if !st.WakeAt.IsZero() {
			_ = r.TimerScheduler.DeleteAt(ctx, st.Site, st.Workflow, st.InstanceID, st.WakeAt)
		} else {
			_ = r.TimerScheduler.Delete(ctx, st.Site, st.Workflow, st.InstanceID)
		}
		out = append(out, next)
	}
	return out, nil
}

func NewRuntime(db *sql.DB, handler StepHandler) *Runtime {
	return &Runtime{
		LeaseStore:     &LeaseStore{DB: db},
		TimerScheduler: &TimerScheduler{DB: db},
		StepHandler:    handler,
		LeaseTTL:       30 * time.Second,
	}
}

func (r *Runtime) db() *sql.DB {
	if r.LeaseStore != nil && r.LeaseStore.DB != nil {
		return r.LeaseStore.DB
	}
	if r.TimerScheduler != nil {
		return r.TimerScheduler.DB
	}
	return nil
}

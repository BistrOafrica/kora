package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/asenawritescode/kora/doctype"
)

func TestRuntimeResumeAndRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		state.Status = "waiting"
		state.NextWakeAt = time.Now().Add(time.Minute)
		state.Payload = json.RawMessage(`{"step":"waiting"}`)
		return state, nil
	})

	now := time.Now().UTC()
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT site, workflow, instance_id").WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
		AddRow("site-a", "approval", "inst-1", 1, "owner-1", now.Add(time.Minute), now.Add(2*time.Minute), "active", `{"doc":"x"}`, now))
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_audit").
		WithArgs(sqlmock.AnyArg(), "site-a", "approval", "inst-1", "", "resume", "active", "waiting", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	state, err := r.Resume(context.Background(), "site-a", "approval", "inst-1", "owner-1")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if state.Status != "waiting" {
		t.Fatalf("unexpected state: %+v", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRetryReschedulesTimer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		return state, nil
	})
	state := InstanceState{
		Site:       "site-a",
		Workflow:   "approval",
		InstanceID: "inst-1",
		TimerID:    "timer-1",
		Status:     "waiting",
		Payload:    json.RawMessage(`{"doc":"x"}`),
	}
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_audit").
		WithArgs(sqlmock.AnyArg(), "site-a", "approval", "inst-1", "timer-1", "retry", "waiting", "retrying", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	if err := r.Retry(context.Background(), state, 2*time.Minute); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeDeadLetterPersistsCauseAndClearsTimers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		return state, nil
	})
	state := InstanceState{
		Site:       "site-a",
		Workflow:   "approval",
		InstanceID: "inst-1",
		TimerID:    "timer-1",
		Status:     "retrying",
		Payload:    json.RawMessage(`{"doc":"x"}`),
	}
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_audit").
		WithArgs(sqlmock.AnyArg(), "site-a", "approval", "inst-1", "timer-1", "dead_letter", "retrying", "dead_letter", "fatal error", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	if err := r.DeadLetter(context.Background(), state, "fatal error"); err != nil {
		t.Fatalf("DeadLetter: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimePauseRecordsPreviousStatusInAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		return state, nil
	})
	state := InstanceState{
		Site:       "site-a",
		Workflow:   "approval",
		InstanceID: "inst-1",
		TimerID:    "timer-1",
		Status:     "waiting",
		Payload:    json.RawMessage(`{"doc":"x"}`),
	}
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_audit").
		WithArgs(sqlmock.AnyArg(), "site-a", "approval", "inst-1", "timer-1", "pause", "waiting", "paused", "", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))

	if err := r.Pause(context.Background(), state, "owner-1"); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeResumeReleasesLeaseOnHandlerError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		return InstanceState{}, fmt.Errorf("boom")
	})
	now := time.Now().UTC()
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT site, workflow, instance_id").WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
		AddRow("site-a", "approval", "inst-1", 1, "owner-1", now.Add(time.Minute), now.Add(2*time.Minute), "active", `{"doc":"x"}`, now))
	mock.ExpectExec("UPDATE _kora_workflow_actor").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if _, err := r.Resume(context.Background(), "site-a", "approval", "inst-1", "owner-1"); err == nil {
		t.Fatal("expected handler failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeResumeRejectsStaleOwnerUnderRedelivery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		t.Fatal("handler should not run for a stale owner")
		return state, nil
	})

	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(0, 0))

	if _, err := r.Resume(context.Background(), "site-a", "approval", "inst-1", "worker-2"); err == nil {
		t.Fatal("expected stale owner error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeProcessDue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		state.Status = "resumed"
		state.Payload = json.RawMessage(`{"step":"resumed"}`)
		return state, nil
	})

	now := time.Now().UTC()
	wakeAt := now.Add(-time.Minute)
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, timer_id, wake_at, payload\\s+FROM _kora_workflow_timer").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "timer_id", "wake_at", "payload"}).
			AddRow("site-a", "approval", "inst-1", "timer-1", wakeAt, `{"doc":"x"}`))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 1, "", now.Add(-time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 2, "", now.Add(-time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	states, err := r.ProcessDue(context.Background(), now, 10, "owner-2")
	if err != nil {
		t.Fatalf("ProcessDue: %v", err)
	}
	if len(states) != 1 || states[0].Status != "resumed" {
		t.Fatalf("unexpected states: %+v", states)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeProcessDueSkipsStaleTimer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		t.Fatalf("handler should not run for stale timer")
		return state, nil
	})

	now := time.Now().UTC()
	staleWake := now.Add(-time.Minute)
	currentWake := now.Add(time.Minute)
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, timer_id, wake_at, payload\\s+FROM _kora_workflow_timer").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "timer_id", "wake_at", "payload"}).
			AddRow("site-a", "approval", "inst-1", "timer-1", staleWake, `{"doc":"x"}`))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 1, "owner-2", now.Add(time.Minute), currentWake, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	states, err := r.ProcessDue(context.Background(), now, 10, "owner-2")
	if err != nil {
		t.Fatalf("ProcessDue: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected stale timer to be skipped: %+v", states)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeProcessDueRejectsStaleOwnerAfterRedelivery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		t.Fatalf("handler should not run for stale owner redelivery")
		return state, nil
	})

	now := time.Now().UTC()
	wakeAt := now.Add(-time.Minute)
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, timer_id, wake_at, payload\\s+FROM _kora_workflow_timer").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "timer_id", "wake_at", "payload"}).
			AddRow("site-a", "approval", "inst-1", "timer-1", wakeAt, `{"doc":"x"}`))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 1, "owner-2", now.Add(time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	states, err := r.ProcessDue(context.Background(), now, 10, "owner-1")
	if err != nil {
		t.Fatalf("ProcessDue: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected stale owner timer to be skipped: %+v", states)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeProcessDueDeduplicatesTimerDeliveries(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	var calls int
	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		calls++
		state.Status = "resumed"
		state.Payload = json.RawMessage(`{"step":"resumed"}`)
		return state, nil
	})

	now := time.Now().UTC()
	wakeAt := now.Add(-time.Minute)
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, timer_id, wake_at, payload\\s+FROM _kora_workflow_timer").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "timer_id", "wake_at", "payload"}).
			AddRow("site-a", "approval", "inst-1", "timer-1", wakeAt, `{"doc":"x"}`).
			AddRow("site-a", "approval", "inst-1", "timer-1", wakeAt, `{"doc":"x"}`))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 1, "", now.Add(-time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 2, "", now.Add(-time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	states, err := r.ProcessDue(context.Background(), now, 10, "owner-1")
	if err != nil {
		t.Fatalf("ProcessDue: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one handler call, got %d", calls)
	}
	if len(states) != 1 || states[0].Status != "resumed" {
		t.Fatalf("unexpected states: %+v", states)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeResumePropagatesCancellationToHandler(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	r := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		<-ctx.Done()
		return InstanceState{}, ctx.Err()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	now := time.Now().UTC()
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT site, workflow, instance_id").WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
		AddRow("site-a", "approval", "inst-1", 1, "owner-1", now.Add(time.Minute), now.Add(2*time.Minute), "active", `{"doc":"x"}`, now))
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))

	if _, err := r.Resume(ctx, "site-a", "approval", "inst-1", "owner-1"); err == nil {
		t.Fatal("expected cancellation error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeProcessDueRecoversAfterRestart(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	wakeAt := now.Add(-time.Minute)

	// First pass: handler fails, but the timer remains for a later retry.
	failing := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		return InstanceState{}, fmt.Errorf("transient crash")
	})
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, timer_id, wake_at, payload\\s+FROM _kora_workflow_timer").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "timer_id", "wake_at", "payload"}).
			AddRow("site-a", "approval", "inst-1", "timer-1", wakeAt, `{"doc":"x"}`))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 1, "", now.Add(-time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))

	states, err := failing.ProcessDue(context.Background(), now, 10, "owner-1")
	if err == nil {
		t.Fatal("expected transient failure")
	}
	if len(states) != 0 {
		t.Fatalf("expected no recovered states on failure, got %+v", states)
	}

	// Restarted worker: the same due timer is still present and should now succeed once.
	recovering := NewRuntime(db, func(ctx context.Context, state InstanceState, doc *doctype.Document) (InstanceState, error) {
		state.Status = "resumed"
		state.Payload = json.RawMessage(`{"step":"resumed"}`)
		return state, nil
	})
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, timer_id, wake_at, payload\\s+FROM _kora_workflow_timer").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "timer_id", "wake_at", "payload"}).
			AddRow("site-a", "approval", "inst-1", "timer-1", wakeAt, `{"doc":"x"}`))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 1, "", now.Add(-time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at\\s+FROM _kora_workflow_actor WHERE site = \\? AND workflow = \\? AND instance_id = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
			AddRow("site-a", "approval", "inst-1", 2, "", now.Add(-time.Minute), wakeAt, "active", `{"doc":"x"}`, now))
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	states, err = recovering.ProcessDue(context.Background(), now, 10, "owner-1")
	if err != nil {
		t.Fatalf("ProcessDue after restart: %v", err)
	}
	if len(states) != 1 || states[0].Status != "resumed" {
		t.Fatalf("unexpected states after restart: %+v", states)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

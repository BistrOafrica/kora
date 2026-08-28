package ai

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/asenawritescode/kora/doctype"
	"github.com/asenawritescode/kora/workflow"
)

func TestRunActorResumeUsesSharedWorkflowRuntime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	actor := NewRunActor(db, "worker-1", func(ctx context.Context, state workflow.InstanceState, doc *doctype.Document) (workflow.InstanceState, error) {
		state.Status = "waiting"
		state.NextWakeAt = time.Now().Add(time.Minute)
		state.Payload = json.RawMessage(`{"phase":"waiting"}`)
		return state, nil
	})
	now := time.Now().UTC()
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT site, workflow, instance_id").WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
		AddRow("site-a", "ai_run", "run-1", 1, "worker-1", now.Add(time.Minute), now.Add(2*time.Minute), "active", `{"summary":"hello"}`, now))
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	state, err := actor.Resume(context.Background(), "site-a", "run-1")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if state.Status != "waiting" || state.Workflow != "ai_run" {
		t.Fatalf("unexpected state: %+v", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunActorDeadLetterUsesSharedWorkflowRuntime(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	actor := NewRunActor(db, "worker-1", func(ctx context.Context, state workflow.InstanceState, doc *doctype.Document) (workflow.InstanceState, error) {
		return state, nil
	})
	state := workflow.InstanceState{
		Site:       "site-a",
		Workflow:   "ai_run",
		InstanceID: "run-1",
		Status:     "retrying",
		Payload:    json.RawMessage(`{"summary":"hello"}`),
	}
	mock.ExpectExec("INSERT INTO _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("DELETE FROM _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))

	if err := actor.DeadLetter(context.Background(), state, "fatal"); err != nil {
		t.Fatalf("DeadLetter: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

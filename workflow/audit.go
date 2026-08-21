package workflow

import (
	"context"
	"database/sql"
	"time"

	"github.com/oklog/ulid/v2"
)

type TransitionAudit struct {
	ID         string
	Site       string
	Workflow   string
	InstanceID string
	StepID     string
	Kind       string
	FromState  string
	ToState    string
	Cause      string
	CreatedAt  time.Time
}

func EnsureAuditTables(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS _kora_workflow_audit (
  id TEXT PRIMARY KEY,
  site TEXT NOT NULL DEFAULT '',
  workflow TEXT NOT NULL DEFAULT '',
  instance_id TEXT NOT NULL DEFAULT '',
  step_id TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT '',
  from_state TEXT NOT NULL DEFAULT '',
  to_state TEXT NOT NULL DEFAULT '',
  cause TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT (datetime('now'))
)`)
	return err
}

func RecordTransition(ctx context.Context, db *sql.DB, ev TransitionAudit) error {
	if db == nil {
		return nil
	}
	if ev.ID == "" {
		ev.ID = ulid.Make().String()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	_, err := db.ExecContext(ctx, `
INSERT INTO _kora_workflow_audit (
	id, site, workflow, instance_id, step_id, kind, from_state, to_state, cause, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, ev.Site, ev.Workflow, ev.InstanceID, ev.StepID, ev.Kind, ev.FromState, ev.ToState, ev.Cause, ev.CreatedAt)
	return err
}

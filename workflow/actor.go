package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// InstanceState is the persisted workflow actor state used for lease/fencing
// and timer recovery.
type InstanceState struct {
	Site       string          `json:"site"`
	Workflow   string          `json:"workflow"`
	InstanceID string          `json:"instance_id"`
	Version    int64           `json:"version"`
	LeaseOwner string          `json:"lease_owner"`
	LeaseUntil time.Time       `json:"lease_until"`
	TimerID    string          `json:"timer_id,omitempty"`
	NextWakeAt time.Time       `json:"next_wake_at,omitempty"`
	WakeAt     time.Time       `json:"wake_at,omitempty"`
	Status     string          `json:"status"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// LeaseStore persists and fences actor ownership.
type LeaseStore struct {
	DB *sql.DB
}

func (s *LeaseStore) EnsureTables(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS _kora_workflow_actor (
  site TEXT NOT NULL,
  workflow TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 0,
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_until DATETIME NULL,
  next_wake_at DATETIME NULL,
  status TEXT NOT NULL DEFAULT 'active',
  payload TEXT NOT NULL DEFAULT '{}',
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (site, workflow, instance_id)
)`)
	return err
}

func (s *LeaseStore) Load(ctx context.Context, site, workflow, instanceID string) (InstanceState, error) {
	var out InstanceState
	var payload string
	err := s.DB.QueryRowContext(ctx, `
SELECT site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at
FROM _kora_workflow_actor WHERE site = ? AND workflow = ? AND instance_id = ?`,
		site, workflow, instanceID,
	).Scan(&out.Site, &out.Workflow, &out.InstanceID, &out.Version, &out.LeaseOwner, &out.LeaseUntil, &out.NextWakeAt, &out.Status, &payload, &out.UpdatedAt)
	if err != nil {
		return InstanceState{}, err
	}
	out.Payload = json.RawMessage(payload)
	return out, nil
}

func (s *LeaseStore) Upsert(ctx context.Context, st InstanceState) error {
	if st.UpdatedAt.IsZero() {
		st.UpdatedAt = time.Now().UTC()
	}
	if len(st.Payload) == 0 {
		st.Payload = json.RawMessage(`{}`)
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO _kora_workflow_actor (site, workflow, instance_id, version, lease_owner, lease_until, next_wake_at, status, payload, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(site, workflow, instance_id) DO UPDATE SET
  version=excluded.version,
  lease_owner=excluded.lease_owner,
  lease_until=excluded.lease_until,
  next_wake_at=excluded.next_wake_at,
  status=excluded.status,
  payload=excluded.payload,
  updated_at=excluded.updated_at`,
		st.Site, st.Workflow, st.InstanceID, st.Version, st.LeaseOwner, nullTime(st.LeaseUntil), nullTime(st.NextWakeAt), st.Status, string(st.Payload), st.UpdatedAt)
	return err
}

func (s *LeaseStore) Acquire(ctx context.Context, site, workflow, instanceID, owner string, ttl time.Duration) (InstanceState, error) {
	now := time.Now().UTC()
	leaseUntil := now.Add(ttl)
	res, err := s.DB.ExecContext(ctx, `
UPDATE _kora_workflow_actor
SET lease_owner = ?, lease_until = ?, version = version + 1, updated_at = ?
WHERE site = ? AND workflow = ? AND instance_id = ?
  AND (lease_until IS NULL OR lease_until < ? OR lease_owner = ?)
`, owner, leaseUntil, now, site, workflow, instanceID, now, owner)
	if err != nil {
		return InstanceState{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return InstanceState{}, fmt.Errorf("workflow actor: stale owner or missing instance")
	}
	return s.Load(ctx, site, workflow, instanceID)
}

func (s *LeaseStore) Release(ctx context.Context, site, workflow, instanceID, owner string) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE _kora_workflow_actor
SET lease_owner = '', lease_until = NULL, updated_at = ?
WHERE site = ? AND workflow = ? AND instance_id = ? AND lease_owner = ?`,
		time.Now().UTC(), site, workflow, instanceID, owner)
	return err
}

func (s *LeaseStore) Renew(ctx context.Context, site, workflow, instanceID, owner string, ttl time.Duration) (InstanceState, error) {
	now := time.Now().UTC()
	leaseUntil := now.Add(ttl)
	res, err := s.DB.ExecContext(ctx, `
UPDATE _kora_workflow_actor
SET lease_until = ?, updated_at = ?
WHERE site = ? AND workflow = ? AND instance_id = ? AND lease_owner = ? AND lease_until >= ?`,
		leaseUntil, now, site, workflow, instanceID, owner, now)
	if err != nil {
		return InstanceState{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return InstanceState{}, fmt.Errorf("workflow actor: stale owner or expired lease")
	}
	return s.Load(ctx, site, workflow, instanceID)
}

// TimerScheduler tracks due workflow wakeups with SQL persistence.
type TimerScheduler struct {
	DB *sql.DB
}

func (s *TimerScheduler) EnsureTables(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS _kora_workflow_timer (
  site TEXT NOT NULL,
  workflow TEXT NOT NULL,
  instance_id TEXT NOT NULL,
  timer_id TEXT NOT NULL DEFAULT '',
  wake_at DATETIME NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY (site, workflow, instance_id, timer_id)
)`)
	return err
}

func (s *TimerScheduler) Schedule(ctx context.Context, site, workflow, instanceID, timerID string, wakeAt time.Time, payload json.RawMessage) error {
	if timerID == "" {
		return fmt.Errorf("timer id is required")
	}
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO _kora_workflow_timer (site, workflow, instance_id, timer_id, wake_at, payload)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(site, workflow, instance_id, timer_id) DO UPDATE SET wake_at=excluded.wake_at, payload=excluded.payload`,
		site, workflow, instanceID, timerID, wakeAt.UTC(), string(payload))
	return err
}

func (s *TimerScheduler) Delete(ctx context.Context, site, workflow, instanceID string) error {
	_, err := s.DB.ExecContext(ctx, `
DELETE FROM _kora_workflow_timer
WHERE site = ? AND workflow = ? AND instance_id = ?`,
		site, workflow, instanceID)
	return err
}

func (s *TimerScheduler) DeleteID(ctx context.Context, site, workflow, instanceID, timerID string) error {
	_, err := s.DB.ExecContext(ctx, `
DELETE FROM _kora_workflow_timer
WHERE site = ? AND workflow = ? AND instance_id = ? AND timer_id = ?`,
		site, workflow, instanceID, timerID)
	return err
}

func (s *TimerScheduler) DeleteAt(ctx context.Context, site, workflow, instanceID string, wakeAt time.Time) error {
	_, err := s.DB.ExecContext(ctx, `
DELETE FROM _kora_workflow_timer
WHERE site = ? AND workflow = ? AND instance_id = ? AND wake_at = ?`,
		site, workflow, instanceID, wakeAt.UTC())
	return err
}

func (s *TimerScheduler) Due(ctx context.Context, now time.Time, limit int) ([]InstanceState, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT site, workflow, instance_id, timer_id, wake_at, payload
FROM _kora_workflow_timer
WHERE wake_at <= ?
ORDER BY wake_at ASC
LIMIT ?`, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InstanceState
	for rows.Next() {
		var st InstanceState
		var payload string
		if err := rows.Scan(&st.Site, &st.Workflow, &st.InstanceID, &st.TimerID, &st.WakeAt, &payload); err != nil {
			return nil, err
		}
		st.Payload = json.RawMessage(payload)
		out = append(out, st)
	}
	return out, rows.Err()
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

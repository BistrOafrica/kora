package cloud

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProvisioningStore persists provisioning jobs and checkpoints in the platform database.
type ProvisioningStore struct {
	DB   *sql.DB
	mu   sync.Mutex
	path string
}

func NewProvisioningStore(db *sql.DB) *ProvisioningStore {
	path := os.Getenv("KORA_CONFIG_DIR")
	if path == "" {
		path = "."
	}
	return &ProvisioningStore{DB: db, path: filepath.Join(path, "_kora_provisioning_jobs.json")}
}

func (s *ProvisioningStore) Bootstrap() error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.Exec(`
CREATE TABLE IF NOT EXISTS _kora_provisioning_job (
  id VARCHAR(140) NOT NULL,
  site VARCHAR(140) NOT NULL DEFAULT '',
  resource VARCHAR(140) NOT NULL DEFAULT '',
  state VARCHAR(40) NOT NULL DEFAULT '',
  operation_id VARCHAR(140) NOT NULL DEFAULT '',
  idempotency_key VARCHAR(255) NOT NULL DEFAULT '',
  input_fingerprint VARCHAR(255) NOT NULL DEFAULT '',
  provider_request_id VARCHAR(255) NOT NULL DEFAULT '',
  output_id VARCHAR(255) NOT NULL DEFAULT '',
  attempt INT NOT NULL DEFAULT 0,
  lease_owner VARCHAR(140) NOT NULL DEFAULT '',
  lease_until DATETIME,
  retry_at DATETIME,
  last_error TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME,
  PRIMARY KEY (id),
  UNIQUE KEY uq_kora_provisioning_dedupe (site, resource, operation_id, idempotency_key, input_fingerprint)
)
`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`
CREATE TABLE IF NOT EXISTS _kora_provisioning_checkpoint (
  job_id VARCHAR(140) NOT NULL,
  stage VARCHAR(140) NOT NULL,
  completed TINYINT(1) NOT NULL DEFAULT 0,
  recorded_at DATETIME NOT NULL,
  PRIMARY KEY (job_id, stage)
)
`)
	return err
}

func (s *ProvisioningStore) UpsertJob(job ProvisioningJob) error {
	if s.DB == nil {
		return s.upsertFile(job)
	}
	_, err := s.DB.Exec(`
INSERT INTO _kora_provisioning_job (
  id, site, resource, state, operation_id, idempotency_key, input_fingerprint, provider_request_id, output_id,
  attempt, lease_owner, lease_until, retry_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  site=VALUES(site),
  resource=VALUES(resource),
  state=VALUES(state),
  operation_id=VALUES(operation_id),
  idempotency_key=VALUES(idempotency_key),
  input_fingerprint=VALUES(input_fingerprint),
  provider_request_id=VALUES(provider_request_id),
  output_id=VALUES(output_id),
  attempt=VALUES(attempt),
  lease_owner=VALUES(lease_owner),
  lease_until=VALUES(lease_until),
  retry_at=VALUES(retry_at),
  last_error=VALUES(last_error),
  updated_at=VALUES(updated_at)
`,
		job.ID, job.Site, job.Resource, string(job.State), job.OperationID, job.IdempotencyKey, job.InputFingerprint, job.ProviderRequestID, job.OutputID,
		job.Attempt, job.LeaseOwner, nullableTime(job.LeaseUntil), nullableTime(job.RetryAt), job.LastError, job.CreatedAt, nullableTime(job.UpdatedAt),
	)
	return err
}

func (s *ProvisioningStore) UpsertCheckpoint(cp ProvisioningCheckpoint) error {
	if s.DB == nil {
		return s.upsertCheckpointFile(cp)
	}
	_, err := s.DB.Exec(`
INSERT INTO _kora_provisioning_checkpoint (job_id, stage, completed, recorded_at)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE completed=VALUES(completed), recorded_at=VALUES(recorded_at)
`, cp.JobID, cp.Stage, boolToInt(cp.Completed), cp.RecordedAt)
	return err
}

func (s *ProvisioningStore) LoadJob(site, resource, operationID, idempotencyKey, fingerprint string) (*ProvisioningJob, error) {
	if s.DB == nil {
		return s.loadJobFile(site, resource, operationID, idempotencyKey, fingerprint)
	}
	row := s.DB.QueryRow(`
SELECT id, site, resource, state, operation_id, idempotency_key, input_fingerprint, provider_request_id, output_id,
       attempt, lease_owner, lease_until, retry_at, last_error, created_at, updated_at
FROM _kora_provisioning_job
WHERE site = ? AND resource = ? AND operation_id = ? AND idempotency_key = ? AND input_fingerprint = ?
ORDER BY created_at DESC LIMIT 1
`, site, resource, operationID, idempotencyKey, fingerprint)
	var job ProvisioningJob
	var leaseUntil, retryAt, updatedAt sql.NullTime
	var leaseOwner, providerReq, outputID, lastError sql.NullString
	if err := row.Scan(&job.ID, &job.Site, &job.Resource, &job.State, &job.OperationID, &job.IdempotencyKey, &job.InputFingerprint, &providerReq, &outputID, &job.Attempt, &leaseOwner, &leaseUntil, &retryAt, &lastError, &job.CreatedAt, &updatedAt); err != nil {
		return nil, err
	}
	job.ProviderRequestID = providerReq.String
	job.OutputID = outputID.String
	job.LeaseOwner = leaseOwner.String
	job.LastError = lastError.String
	if leaseUntil.Valid {
		job.LeaseUntil = leaseUntil.Time
	}
	if retryAt.Valid {
		job.RetryAt = retryAt.Time
	}
	if updatedAt.Valid {
		job.UpdatedAt = updatedAt.Time
	}
	return &job, nil
}

func (s *ProvisioningStore) LoadJobByID(id string) (*ProvisioningJob, error) {
	if s.DB == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		state, err := s.readFileState()
		if err != nil {
			return nil, err
		}
		for i := len(state.Jobs) - 1; i >= 0; i-- {
			job := state.Jobs[i]
			if job.ID == id {
				return &job, nil
			}
		}
		return nil, sql.ErrNoRows
	}
	row := s.DB.QueryRow(`
SELECT id, site, resource, state, operation_id, idempotency_key, input_fingerprint, provider_request_id, output_id,
       attempt, lease_owner, lease_until, retry_at, last_error, created_at, updated_at
FROM _kora_provisioning_job
WHERE id = ?
`, id)
	var job ProvisioningJob
	var leaseUntil, retryAt, updatedAt sql.NullTime
	var leaseOwner, providerReq, outputID, lastError sql.NullString
	if err := row.Scan(&job.ID, &job.Site, &job.Resource, &job.State, &job.OperationID, &job.IdempotencyKey, &job.InputFingerprint, &providerReq, &outputID, &job.Attempt, &leaseOwner, &leaseUntil, &retryAt, &lastError, &job.CreatedAt, &updatedAt); err != nil {
		return nil, err
	}
	job.ProviderRequestID = providerReq.String
	job.OutputID = outputID.String
	job.LeaseOwner = leaseOwner.String
	job.LastError = lastError.String
	if leaseUntil.Valid {
		job.LeaseUntil = leaseUntil.Time
	}
	if retryAt.Valid {
		job.RetryAt = retryAt.Time
	}
	if updatedAt.Valid {
		job.UpdatedAt = updatedAt.Time
	}
	return &job, nil
}

func (s *ProvisioningStore) ListJobs() ([]ProvisioningJob, error) {
	if s.DB == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		state, err := s.readFileState()
		if err != nil {
			return nil, err
		}
		jobs := make([]ProvisioningJob, len(state.Jobs))
		copy(jobs, state.Jobs)
		return jobs, nil
	}
	rows, err := s.DB.Query(`
SELECT id, site, resource, state, operation_id, idempotency_key, input_fingerprint, provider_request_id, output_id,
       attempt, lease_owner, lease_until, retry_at, last_error, created_at, updated_at
FROM _kora_provisioning_job
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []ProvisioningJob
	for rows.Next() {
		var job ProvisioningJob
		var leaseUntil, retryAt, updatedAt sql.NullTime
		var leaseOwner, providerReq, outputID, lastError sql.NullString
		if err := rows.Scan(&job.ID, &job.Site, &job.Resource, &job.State, &job.OperationID, &job.IdempotencyKey, &job.InputFingerprint, &providerReq, &outputID, &job.Attempt, &leaseOwner, &leaseUntil, &retryAt, &lastError, &job.CreatedAt, &updatedAt); err != nil {
			return nil, err
		}
		job.ProviderRequestID = providerReq.String
		job.OutputID = outputID.String
		job.LeaseOwner = leaseOwner.String
		job.LastError = lastError.String
		if leaseUntil.Valid {
			job.LeaseUntil = leaseUntil.Time
		}
		if retryAt.Valid {
			job.RetryAt = retryAt.Time
		}
		if updatedAt.Valid {
			job.UpdatedAt = updatedAt.Time
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC()
}

func (s *ProvisioningStore) EncodeCheckpoint(cp ProvisioningCheckpoint) string {
	b, _ := json.Marshal(cp)
	return string(b)
}

type provisioningFileState struct {
	Jobs        []ProvisioningJob        `json:"jobs"`
	Checkpoints []ProvisioningCheckpoint `json:"checkpoints"`
}

func (s *ProvisioningStore) readFileState() (provisioningFileState, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return provisioningFileState{}, nil
		}
		return provisioningFileState{}, err
	}
	var state provisioningFileState
	if err := json.Unmarshal(b, &state); err != nil {
		return provisioningFileState{}, err
	}
	return state, nil
}

func (s *ProvisioningStore) writeFileState(state provisioningFileState) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func (s *ProvisioningStore) upsertFile(job ProvisioningJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readFileState()
	if err != nil {
		return err
	}
	replaced := false
	for i := range state.Jobs {
		if state.Jobs[i].Site == job.Site && state.Jobs[i].Resource == job.Resource && state.Jobs[i].OperationID == job.OperationID && state.Jobs[i].IdempotencyKey == job.IdempotencyKey && state.Jobs[i].InputFingerprint == job.InputFingerprint {
			state.Jobs[i] = job
			replaced = true
			break
		}
	}
	if !replaced {
		state.Jobs = append(state.Jobs, job)
	}
	return s.writeFileState(state)
}

func (s *ProvisioningStore) upsertCheckpointFile(cp ProvisioningCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readFileState()
	if err != nil {
		return err
	}
	replaced := false
	for i := range state.Checkpoints {
		if state.Checkpoints[i].JobID == cp.JobID && state.Checkpoints[i].Stage == cp.Stage {
			state.Checkpoints[i] = cp
			replaced = true
			break
		}
	}
	if !replaced {
		state.Checkpoints = append(state.Checkpoints, cp)
	}
	return s.writeFileState(state)
}

func (s *ProvisioningStore) loadJobFile(site, resource, operationID, idempotencyKey, fingerprint string) (*ProvisioningJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readFileState()
	if err != nil {
		return nil, err
	}
	for i := len(state.Jobs) - 1; i >= 0; i-- {
		job := state.Jobs[i]
		if job.Site == site && job.Resource == resource && job.OperationID == operationID && job.IdempotencyKey == idempotencyKey && job.InputFingerprint == fingerprint {
			return &job, nil
		}
	}
	return nil, sql.ErrNoRows
}

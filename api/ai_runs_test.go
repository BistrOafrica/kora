package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"github.com/asenawritescode/kora/db"
	"github.com/asenawritescode/kora/doctype"
	"github.com/asenawritescode/kora/orm"
)

func TestHandleAIGrantApprovalTransitionsPendingRow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer mockDB.Close()

	reg := doctype.NewRegistry()
	handler := NewHandler(reg, &orm.TxManager{DB: mockDB, Registry: reg, Dialect: db.Resolve("mysql")})

	requestedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	pendingRows := sqlmock.NewRows([]string{
		"id", "site", "operation_id", "actor_principal_id", "actor_principal_type",
		"tool_name", "state", "target_fingerprint", "argument_hash", "record_version",
		"requested_at", "expires_at", "granted_at", "granted_by", "auth_session_id",
	}).AddRow(
		"approval-1", "test.local", "op-1", "user-1", "human",
		"customer_update", "pending_approval", "fingerprint", "fingerprint", 1,
		requestedAt, nil, nil, "", "sess-1",
	)
	mock.ExpectQuery("SELECT id, site, operation_id, actor_principal_id, actor_principal_type, tool_name, state, target_fingerprint, argument_hash, record_version, requested_at, expires_at, granted_at, granted_by, auth_session_id FROM _kora_ai_approval WHERE id = \\?").
		WithArgs("approval-1").
		WillReturnRows(pendingRows)
	mock.ExpectExec("INSERT INTO _kora_ai_approval").
		WithArgs(
			"approval-1", "test.local", "op-1", "user-1", "human",
			"customer_update", "granted", "fingerprint", "fingerprint", 1,
			sqlmock.AnyArg(), nil, sqlmock.AnyArg(), "alice", "sess-1",
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	grantedRows := sqlmock.NewRows([]string{
		"id", "site", "operation_id", "actor_principal_id", "actor_principal_type",
		"tool_name", "state", "target_fingerprint", "argument_hash", "record_version",
		"requested_at", "expires_at", "granted_at", "granted_by", "auth_session_id",
	}).AddRow(
		"approval-1", "test.local", "op-1", "user-1", "human",
		"customer_update", "granted", "fingerprint", "fingerprint", 1,
		requestedAt, nil, time.Date(2026, 8, 13, 11, 0, 0, 0, time.UTC), "alice", "sess-1",
	)
	mock.ExpectQuery("SELECT id, site, operation_id, actor_principal_id, actor_principal_type, tool_name, state, target_fingerprint, argument_hash, record_version, requested_at, expires_at, granted_at, granted_by, auth_session_id FROM _kora_ai_approval WHERE id = \\?").
		WithArgs("approval-1").
		WillReturnRows(grantedRows)
	mock.ExpectQuery("SELECT id, site, conversation_id, channel, status, model, provider, current_step_id, summary, input_message, output_message, error_message, cancel_reason, resume_token, created_at, updated_at, completed_at, cancelled_at, retention_expires_at FROM _kora_ai_run WHERE id = \\?").
		WithArgs("op-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "site", "conversation_id", "channel", "status", "model", "provider", "current_step_id", "summary", "input_message", "output_message", "error_message", "cancel_reason", "resume_token", "created_at", "updated_at", "completed_at", "cancelled_at", "retention_expires_at",
		}).AddRow("op-1", "test.local", "conv-1", "chat", "pending_approval", "gpt-4o", "openai_api_key", "step-1", "summary", "hello", "", "approval required", "", "resume-1", requestedAt, requestedAt, requestedAt, requestedAt, requestedAt))
	mock.ExpectExec("INSERT INTO _kora_ai_run").
		WithArgs(
			"op-1", "test.local", "conv-1", "chat", "planning", "gpt-4o", "openai_api_key", "step-1", "summary", "hello", "", "", "", "resume-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, site, conversation_id, channel, status, model, provider, current_step_id, summary, input_message, output_message, error_message, cancel_reason, resume_token, created_at, updated_at, completed_at, cancelled_at, retention_expires_at FROM _kora_ai_run WHERE id = \\?").
		WithArgs("op-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "site", "conversation_id", "channel", "status", "model", "provider", "current_step_id", "summary", "input_message", "output_message", "error_message", "cancel_reason", "resume_token", "created_at", "updated_at", "completed_at", "cancelled_at", "retention_expires_at",
		}).AddRow("op-1", "test.local", "conv-1", "chat", "planning", "gpt-4o", "openai_api_key", "step-1", "summary", "hello", "", "", "", "resume-1", requestedAt, requestedAt, requestedAt, requestedAt, requestedAt))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ai/approvals/approval-1/grant", nil)
	c.Params = gin.Params{{Key: "id", Value: "approval-1"}}
	c.Set("site_db", mockDB)
	c.Set("site_registry", reg)
	c.Set("user", "alice")

	handler.HandleAIGrantApproval(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"run_status":"planning"`) {
		t.Fatalf("expected run status in response, body=%s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet mock expectations: %v", err)
	}
}

func TestHandleAIResumeDoesNotLeakRunSummary(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer mockDB.Close()

	reg := doctype.NewRegistry()
	handler := NewHandler(reg, &orm.TxManager{DB: mockDB, Registry: reg, Dialect: db.Resolve("mysql")})

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT id, site, conversation_id, channel, status, model, provider, current_step_id, summary, input_message, output_message, error_message, cancel_reason, resume_token, created_at, updated_at, completed_at, cancelled_at, retention_expires_at FROM _kora_ai_run WHERE id = \\?").
		WithArgs("run-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "site", "conversation_id", "channel", "status", "model", "provider", "current_step_id", "summary", "input_message", "output_message", "error_message", "cancel_reason", "resume_token", "created_at", "updated_at", "completed_at", "cancelled_at", "retention_expires_at",
		}).AddRow("run-1", "test.local", "conv-1", "chat", "planning", "gpt-4o", "openai_api_key", "step-1", "sensitive summary", "hello", "assistant answer", "", "", "resume-1", now, now, now, now, time.Time{}))
	mock.ExpectExec("INSERT INTO _kora_ai_run").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, site, conversation_id, channel, status, model, provider, current_step_id, summary, input_message, output_message, error_message, cancel_reason, resume_token, created_at, updated_at, completed_at, cancelled_at, retention_expires_at FROM _kora_ai_run WHERE id = \\?").
		WithArgs("run-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "site", "conversation_id", "channel", "status", "model", "provider", "current_step_id", "summary", "input_message", "output_message", "error_message", "cancel_reason", "resume_token", "created_at", "updated_at", "completed_at", "cancelled_at", "retention_expires_at",
		}).AddRow("run-1", "test.local", "conv-1", "chat", "planning", "gpt-4o", "openai_api_key", "step-1", "sensitive summary", "hello", "assistant answer", "", "", "resume-2", now, now, now, now, time.Time{}))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ai/runs/run-1/resume", bytes.NewBufferString(`{"resume_token":"resume-1"}`))
	c.Params = gin.Params{{Key: "id", Value: "run-1"}}
	c.Set("site_db", mockDB)
	c.Set("site_registry", reg)

	handler.HandleAIResume(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "sensitive summary") || strings.Contains(w.Body.String(), "assistant answer") {
		t.Fatalf("response leaked run internals: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"resume_token":"resume-2"`) {
		t.Fatalf("expected resume token in response, body=%s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet mock expectations: %v", err)
	}
}

func TestHandleAICancelMarksRunCancelled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer mockDB.Close()

	reg := doctype.NewRegistry()
	handler := NewHandler(reg, &orm.TxManager{DB: mockDB, Registry: reg, Dialect: db.Resolve("mysql")})

	now := time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT id, site, conversation_id, channel, status, model, provider, current_step_id, summary, input_message, output_message, error_message, cancel_reason, resume_token, created_at, updated_at, completed_at, cancelled_at, retention_expires_at FROM _kora_ai_run WHERE id = \\?").
		WithArgs("run-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "site", "conversation_id", "channel", "status", "model", "provider", "current_step_id", "summary", "input_message", "output_message", "error_message", "cancel_reason", "resume_token", "created_at", "updated_at", "completed_at", "cancelled_at", "retention_expires_at",
		}).AddRow("run-1", "test.local", "conv-1", "chat", "planning", "gpt-4o", "openai_api_key", "step-1", "summary", "hello", "", "", "", "resume-1", now, now, time.Time{}, time.Time{}, time.Time{}))
	mock.ExpectExec("INSERT INTO _kora_ai_run").
		WithArgs(
			"run-1", "test.local", "conv-1", "chat", "cancelled", "gpt-4o", "openai_api_key", "step-1", "summary", "hello", "", "", "stop here", "resume-1",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id, site, channel, subject_key, title, summary, status, last_run_id, last_message_at, retention_expires_at, created_at, updated_at FROM _kora_ai_conversation WHERE id = \\?").
		WithArgs("conv-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "site", "channel", "subject_key", "title", "summary", "status", "last_run_id", "last_message_at", "retention_expires_at", "created_at", "updated_at",
		}).AddRow("conv-1", "test.local", "chat", "subject", "title", "summary", "active", "run-0", time.Time{}, time.Time{}, now, now))
	mock.ExpectExec("INSERT INTO _kora_ai_conversation").
		WithArgs(
			"conv-1", "test.local", "chat", "subject", "title", "summary", "cancelled", "run-1",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ai/runs/run-1/cancel", bytes.NewBufferString(`{"reason":"stop here"}`))
	c.Params = gin.Params{{Key: "id", Value: "run-1"}}
	c.Set("site_db", mockDB)
	c.Set("site_registry", reg)

	handler.HandleAICancel(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("expected cancelled response, body=%s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet mock expectations: %v", err)
	}
}

func TestHandleAIRetentionCleanupRemovesExpiredGraph(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer mockDB.Close()

	reg := doctype.NewRegistry()
	handler := NewHandler(reg, &orm.TxManager{DB: mockDB, Registry: reg, Dialect: db.Resolve("mysql")})

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM _kora_ai_message").
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("DELETE FROM _kora_ai_step").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM _kora_ai_run").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("DELETE FROM _kora_ai_conversation").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/ai/retention/cleanup", nil)
	c.Set("site_db", mockDB)
	c.Set("site_registry", reg)

	handler.HandleAIRetentionCleanup(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"removed":8`) {
		t.Fatalf("expected removal count, body=%s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet mock expectations: %v", err)
	}
}

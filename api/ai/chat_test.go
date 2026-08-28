package ai

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/asenawritescode/kora/contract"
)

func TestEstimatePromptTokens(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": "hello world"},
		{"role": "user", "content": "please help me"},
	}
	functions := []map[string]any{
		{"type": "function", "function": map[string]any{"name": "test_tool", "description": "x"}},
	}

	got := estimatePromptTokens(messages, functions)
	if got <= 0 {
		t.Fatalf("estimatePromptTokens returned %d, want > 0", got)
	}
}

func TestReserveAndReleaseBudget(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(requested_tokens\\), 0\\)").
		WithArgs("site-a", "gpt-4o").
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(10))
	mock.ExpectExec("INSERT INTO _kora_ai_budget_reservation").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	res, err := ReserveBudget(context.Background(), db, "site-a", "gpt-4o", 25, 100, "chat request")
	if err != nil {
		t.Fatalf("ReserveBudget: %v", err)
	}
	if res.ID == "" {
		t.Fatal("expected reservation id")
	}
	mock.ExpectExec("UPDATE _kora_ai_budget_reservation").
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := ReleaseBudget(context.Background(), db, res); err != nil {
		t.Fatalf("ReleaseBudget: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFinalizeBudget(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE _kora_ai_budget_reservation").
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := FinalizeBudget(context.Background(), db, BudgetReservation{ID: "res-1"}, 77); err != nil {
		t.Fatalf("FinalizeBudget: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRecordUsageCarriesRunID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("INSERT INTO _kora_ai_usage").
		WithArgs(
			"usage-1",
			"site-a",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"gpt-4o",
			"openai_api_key",
			"run-123",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			1,
			"completed",
			sqlmock.AnyArg(),
			int64(42),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = RecordUsage(context.Background(), db, contract.UsageEvent{
		ID:        "usage-1",
		Site:      "site-a",
		Model:     "gpt-4o",
		Provider:  "openai_api_key",
		RunID:     "run-123",
		Attempt:   1,
		Status:    "completed",
		LatencyMs: 42,
	})
	if err != nil {
		t.Fatalf("RecordUsage: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReserveBudgetRejectsOverage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(requested_tokens\\), 0\\)").
		WithArgs("site-a", "gpt-4o").
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(95))
	mock.ExpectRollback()

	if _, err := ReserveBudget(context.Background(), db, "site-a", "gpt-4o", 10, 100, "chat request"); err == nil {
		t.Fatal("expected budget exceeded error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

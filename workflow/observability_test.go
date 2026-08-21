package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCollectMetricsReportsQueueDepthAndLeaseExpiry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	nextExpiry := now.Add(5 * time.Minute)
	mock.ExpectQuery("SELECT COUNT\\(1\\)[\\s\\S]*FROM _kora_workflow_timer[\\s\\S]*wake_at <= \\?").
		WithArgs(now).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(1)"}).AddRow(3))
	mock.ExpectQuery("SELECT COUNT\\(1\\)[\\s\\S]*FROM _kora_workflow_actor[\\s\\S]*lease_owner <> ''[\\s\\S]*lease_until IS NOT NULL[\\s\\S]*lease_until >= \\?").
		WithArgs(now).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(1)"}).AddRow(2))
	mock.ExpectQuery("SELECT lease_until[\\s\\S]*FROM _kora_workflow_actor[\\s\\S]*ORDER BY lease_until ASC[\\s\\S]*LIMIT 1").
		WillReturnRows(sqlmock.NewRows([]string{"lease_until"}).AddRow(nextExpiry))

	metrics, err := CollectMetrics(context.Background(), db, now)
	if err != nil {
		t.Fatalf("CollectMetrics: %v", err)
	}
	if metrics.DueTimers != 3 || metrics.LeasedActors != 2 || !metrics.NextLeaseExpiry.Equal(nextExpiry) {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

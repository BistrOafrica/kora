package outbox

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPublisherMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT\s+COALESCE\(SUM\(CASE WHEN status = 'pending' THEN 1 ELSE 0 END\), 0\),`).
		WillReturnRows(sqlmock.NewRows([]string{"pending", "failed", "published"}).AddRow(3, 1, 9))

	p := &Publisher{DB: db}
	m, err := p.Metrics(context.Background())
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	if m.Pending != 3 || m.Failed != 1 || m.Published != 9 {
		t.Fatalf("metrics = %+v, want pending=3 failed=1 published=9", m)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}


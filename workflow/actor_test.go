package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLeaseStoreAcquireAndRelease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &LeaseStore{DB: db}
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT site, workflow, instance_id").WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
		AddRow("site-a", "approval", "inst-1", 2, "worker-1", now.Add(time.Minute), now.Add(2*time.Minute), "active", `{}`, now))

	st, err := store.Acquire(context.Background(), "site-a", "approval", "inst-1", "worker-1", time.Minute)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if st.InstanceID != "inst-1" || st.LeaseOwner != "worker-1" {
		t.Fatalf("unexpected state: %+v", st)
	}
	mock.ExpectExec("UPDATE _kora_workflow_actor").
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := store.Release(context.Background(), "site-a", "approval", "inst-1", "worker-1"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTimerSchedulerScheduleAndDue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	sched := &TimerScheduler{DB: db}
	mock.ExpectExec("INSERT INTO _kora_workflow_timer").WillReturnResult(sqlmock.NewResult(1, 1))
	if err := sched.Schedule(context.Background(), "site-a", "approval", "inst-1", "timer-1", time.Now().Add(time.Hour), []byte(`{"step":"wait"}`)); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	wakeAt := time.Now().UTC().Add(time.Minute)
	mock.ExpectQuery("SELECT site, workflow, instance_id, timer_id, wake_at, payload").WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "timer_id", "wake_at", "payload"}).
		AddRow("site-a", "approval", "inst-1", "timer-1", wakeAt, `{"step":"wait"}`))
	due, err := sched.Due(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 1 || due[0].InstanceID != "inst-1" {
		t.Fatalf("unexpected due result: %+v", due)
	}
	if due[0].TimerID != "timer-1" {
		t.Fatalf("expected timer id to round-trip: %+v", due[0])
	}
	if due[0].WakeAt.IsZero() {
		t.Fatalf("expected wake_at to be populated: %+v", due[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseStoreRejectsStaleOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &LeaseStore{DB: db}
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(0, 0))
	if _, err := store.Acquire(context.Background(), "site-a", "approval", "inst-1", "worker-2", time.Minute); err == nil {
		t.Fatal("expected stale owner error")
	}
}

func TestLeaseStoreRenewAndRejectExpiredLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	store := &LeaseStore{DB: db}
	now := time.Now().UTC()
	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT site, workflow, instance_id").WillReturnRows(sqlmock.NewRows([]string{"site", "workflow", "instance_id", "version", "lease_owner", "lease_until", "next_wake_at", "status", "payload", "updated_at"}).
		AddRow("site-a", "approval", "inst-1", 2, "worker-1", now.Add(2*time.Minute), now.Add(time.Minute), "active", `{}`, now))
	st, err := store.Renew(context.Background(), "site-a", "approval", "inst-1", "worker-1", 2*time.Minute)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if st.LeaseOwner != "worker-1" {
		t.Fatalf("unexpected state: %+v", st)
	}

	mock.ExpectExec("UPDATE _kora_workflow_actor").WillReturnResult(sqlmock.NewResult(0, 0))
	if _, err := store.Renew(context.Background(), "site-a", "approval", "inst-1", "worker-1", time.Minute); err == nil {
		t.Fatal("expected expired lease error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

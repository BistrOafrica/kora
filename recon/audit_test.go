package recon

import (
	"context"
	"testing"
	"time"

	"github.com/asenawritescode/kora/contract"
)

func TestLedgerAppendAndQueryTenantIsolation(t *testing.T) {
	l := NewAuditLedger()
	ctx := context.Background()
	now := time.Now().UTC()

	_ = l.Append(ctx, ReconAuditEntry{TenantID: "a", Kind: AuditLeaseAcquired, Actor: "owner-1", Fence: 1, CreatedAt: now})
	_ = l.Append(ctx, ReconAuditEntry{TenantID: "b", Kind: AuditDriftDetected, Actor: "owner-2", Fence: 1, CreatedAt: now})
	_ = l.Append(ctx, ReconAuditEntry{TenantID: "a", Kind: AuditPropagation, Subject: contract.ResourceRef{Namespace: "n", Name: "x", Version: 1}, Actor: "owner-1", Fence: 1, CreatedAt: now})

	rows, next, err := l.Query(ctx, "a", "", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("tenant-a rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.TenantID != "a" {
			t.Fatalf("cross-tenant row leaked: %+v", r)
		}
	}
	if next != "" {
		t.Fatalf("expected empty next cursor, got %q", next)
	}
}

func TestLedgerPagination(t *testing.T) {
	l := NewAuditLedger()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_ = l.Append(ctx, ReconAuditEntry{TenantID: "a", Kind: AuditDriftDetected, Fence: int64(i)})
	}
	page1, cursor, err := l.Query(ctx, "a", "", 2)
	if err != nil || len(page1) != 2 || cursor == "" {
		t.Fatalf("page1 = %d cursor=%q err=%v", len(page1), cursor, err)
	}
	page2, cursor2, err := l.Query(ctx, "a", cursor, 2)
	if err != nil || len(page2) != 2 {
		t.Fatalf("page2 = %d err=%v", len(page2), err)
	}
	if page1[1].ID == page2[0].ID {
		t.Fatalf("pagination overlap")
	}
	if cursor2 == "" {
		t.Fatalf("expected cursor2 non-empty")
	}
}

func TestLedgerAppendOnlyByInterface(t *testing.T) {
	// The ReconAuditWriter interface exposes only Append and Query; there is no
	// Update/Delete. This is a compile-time property, asserted here by type.
	var _ ReconAuditWriter = NewAuditLedger()
}

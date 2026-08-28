package recon

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/asenawritescode/kora/contract"
)

// ReconAuditKind classifies a reconciliation decision (RECON-005).
type ReconAuditKind string

const (
	AuditLeaseAcquired  ReconAuditKind = "lease_acquired"
	AuditLeaseTakenOver ReconAuditKind = "lease_taken_over"
	AuditDriftDetected  ReconAuditKind = "drift_detected"
	AuditDriftResolved  ReconAuditKind = "drift_resolved"
	AuditPropagation    ReconAuditKind = "propagation"
	AuditRecovery       ReconAuditKind = "recovery"
)

// ReconAuditEntry is one append-only ledger row. DetailJSON is a typed,
// redacted, size-capped serialization — never a raw payload.
type ReconAuditEntry struct {
	ID         string
	TenantID   string
	Kind       ReconAuditKind
	Subject    contract.ResourceRef
	Actor      string
	Fence      int64
	DetailJSON string
	CreatedAt  time.Time
}

// ReconAuditWriter is the append-only reconciliation ledger. The API exposes
// only Append and Query: there is deliberately no update or delete method, so
// the append-only property is enforced by the interface itself.
type ReconAuditWriter interface {
	Append(ctx context.Context, e ReconAuditEntry) error
	Query(ctx context.Context, tenantID string, cursor string, limit int) ([]ReconAuditEntry, string, error)
}

// AuditLedger is the in-memory reference ledger. Entries are keyed by sortable
// ULID IDs; Query is tenant-isolated and cursor-paginated. The SQL-backed store
// (over _kora_recon_audit) must satisfy the same semantics.
type AuditLedger struct {
	mu      sync.RWMutex
	entries []ReconAuditEntry
}

// NewAuditLedger returns an empty ledger.
func NewAuditLedger() *AuditLedger {
	return &AuditLedger{}
}

// Append records an entry. ID defaults to a fresh ULID; CreatedAt defaults to
// now.
func (l *AuditLedger) Append(ctx context.Context, e ReconAuditEntry) error {
	if e.ID == "" {
		e.ID = contract.NewID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
	return nil
}

// Query returns entries for tenantID after cursor (exclusive, by ULID ID), up
// to limit, plus the next cursor (empty when no more). Tenant isolation is
// enforced here, not by the caller.
func (l *AuditLedger) Query(ctx context.Context, tenantID, cursor string, limit int) ([]ReconAuditEntry, string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Collect the tenant's entries in ID order.
	tenant := make([]ReconAuditEntry, 0)
	for _, e := range l.entries {
		if e.TenantID == tenantID && e.ID > cursor {
			tenant = append(tenant, e)
		}
	}
	sort.Slice(tenant, func(i, j int) bool { return tenant[i].ID < tenant[j].ID })

	if limit <= 0 || limit > len(tenant) {
		limit = len(tenant)
	}
	page := tenant[:limit]
	next := ""
	if limit < len(tenant) {
		next = tenant[limit-1].ID
	}
	return page, next, nil
}

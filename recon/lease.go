package recon

import (
	"fmt"
	"sync"
	"time"
)

// ReconcileLease is the fencing token for one tenant's reconciliation. Fence is
// monotonic: a takeover increments it so a stale owner's later writes fail
// closed.
type ReconcileLease struct {
	TenantID  string
	OwnerID   string
	Fence     int64
	ExpiresAt time.Time
}

// ErrLeaseHeld reports that another owner currently holds the lease.
type ErrLeaseHeld struct {
	Holder string
}

func (e *ErrLeaseHeld) Error() string {
	return fmt.Sprintf("reconcile lease held by %q", e.Holder)
}

// ErrStaleFence is returned when an operation carries an outdated fence token.
type ErrStaleFence struct {
	Have, Current int64
}

func (e *ErrStaleFence) Error() string {
	return fmt.Sprintf("reconcile stale fence: have %d, current %d", e.Have, e.Current)
}

// LeaseStore is the in-memory reference lease store (RECON-003). It guarantees
// exactly one active holder per tenant: Acquire on a live lease returns
// ErrLeaseHeld; on an expired lease it takes over and bumps the fence. The
// SQL-backed store (over _kora_recon_lease) must satisfy the same semantics.
type LeaseStore struct {
	mu     sync.Mutex
	leases map[string]ReconcileLease
}

// NewLeaseStore returns an empty lease store.
func NewLeaseStore() *LeaseStore {
	return &LeaseStore{leases: make(map[string]ReconcileLease)}
}

// Acquire takes the lease for tenantID if it is free or expired, otherwise
// returns ErrLeaseHeld. On takeover of an expired lease, the fence increments.
func (s *LeaseStore) Acquire(tenantID, ownerID string, ttl time.Duration, now time.Time) (ReconcileLease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.leases[tenantID]
	if ok && now.Before(existing.ExpiresAt) {
		return ReconcileLease{}, &ErrLeaseHeld{Holder: existing.OwnerID}
	}
	fence := int64(1)
	if ok {
		fence = existing.Fence + 1
	}
	lease := ReconcileLease{
		TenantID:  tenantID,
		OwnerID:   ownerID,
		Fence:     fence,
		ExpiresAt: now.Add(ttl),
	}
	s.leases[tenantID] = lease
	return lease, nil
}

// Validate reports whether lease is still the current, unexpired holder.
func (s *LeaseStore) Validate(lease ReconcileLease, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.leases[lease.TenantID]
	if !ok || current.Fence != lease.Fence {
		return &ErrStaleFence{Have: lease.Fence, Current: current.Fence}
	}
	if !now.Before(lease.ExpiresAt) {
		return fmt.Errorf("reconcile lease expired for %q", lease.TenantID)
	}
	return nil
}

// Release drops the lease if fence still matches; otherwise ErrStaleFence.
func (s *LeaseStore) Release(lease ReconcileLease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.leases[lease.TenantID]
	if !ok || current.Fence != lease.Fence {
		return &ErrStaleFence{Have: lease.Fence, Current: current.Fence}
	}
	delete(s.leases, lease.TenantID)
	return nil
}

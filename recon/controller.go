package recon

import (
	"context"
	"time"
)

// ReconcileBatchResult summarizes one bounded reconcile batch.
type ReconcileBatchResult struct {
	Attempted   int
	Converged   int
	Failed      int
	Deferred    int
	NextBackoff time.Duration
}

// Controller is the convergence controller (RECON-003). It fences each tenant
// with a lease and runs bounded reconcile batches. Corrective mutations are
// enqueued as CommandEnvelopes through the kernel/outbox by the caller — the
// controller holds no direct write path.
type Controller struct {
	leases *LeaseStore
	ttl    time.Duration
	now    func() time.Time
}

// NewController returns a controller with the given lease TTL.
func NewController(leases *LeaseStore, ttl time.Duration) *Controller {
	return &Controller{leases: leases, ttl: ttl, now: time.Now}
}

// Acquire fences tenantID. It fails with ErrLeaseHeld if another controller is
// active.
func (c *Controller) Acquire(ctx context.Context, tenantID, ownerID string) (ReconcileLease, error) {
	return c.leases.Acquire(tenantID, ownerID, c.ttl, c.now().UTC())
}

// ReconcileBatch runs up to max work items under a validated lease. work
// returns nil for converged, err for failed; it may return a sentinel to defer.
// A stale or expired lease aborts the batch.
func (c *Controller) ReconcileBatch(ctx context.Context, lease ReconcileLease, max int, work func(i int) error) (ReconcileBatchResult, error) {
	var res ReconcileBatchResult
	if max <= 0 {
		return res, nil
	}
	for i := 0; i < max; i++ {
		if err := c.leases.Validate(lease, c.now().UTC()); err != nil {
			return res, err
		}
		res.Attempted++
		if err := work(i); err != nil {
			if err == ErrDeferred {
				res.Deferred++
			} else {
				res.Failed++
			}
			continue
		}
		res.Converged++
	}
	res.NextBackoff = backoff(res.Failed, res.Deferred)
	return res, nil
}

// ErrDeferred marks a work item the controller chose to defer (not a failure).
var ErrDeferred = errDeferred{}

type errDeferred struct{}

func (errDeferred) Error() string { return "reconcile: deferred" }

// backoff computes the next delay: exponential in failures, capped, with a
// small fixed add for deferrals.
func backoff(failed, deferred int) time.Duration {
	if failed == 0 && deferred == 0 {
		return 0
	}
	d := time.Second
	for i := 1; i < failed; i++ {
		d *= 2
		if d >= time.Minute {
			return time.Minute
		}
	}
	if deferred > 0 {
		d += time.Duration(deferred) * 100 * time.Millisecond
	}
	return d
}

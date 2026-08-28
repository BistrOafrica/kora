// Package effect implements the effect-scope contract (PLUGIN-003, invariant
// 7): every runtime effect — a timer, subscription, temp state, blob handle, or
// outbox listener — has an owner, a scope, a disposer, and observable cleanup
// evidence. Scopes dispose effects LIFO, continue past a failing disposer, and
// report a CleanupEvidence on close.
package effect

import (
	"fmt"
	"sync"
	"time"

	"github.com/asenawritescode/kora/contract"
)

// EffectKind classifies an effect for telemetry and evidence.
type EffectKind string

const (
	EffectTimer          EffectKind = "timer"
	EffectSubscription   EffectKind = "subscription"
	EffectTempState      EffectKind = "temp_state"
	EffectBlobHandle     EffectKind = "blob_handle"
	EffectOutboxListener EffectKind = "outbox_listener"
)

// CloseReason records why a scope closed.
type CloseReason string

const (
	CloseNormal        CloseReason = "normal"
	CloseTimeout       CloseReason = "timeout"
	CloseShutdown      CloseReason = "shutdown"
	CloseCrashRecovery CloseReason = "crash_recovery"
)

// Effect is one tracked effect. Disposer must be non-nil; Track rejects a nil
// disposer because an undisposable effect would be an untracked leak.
type Effect struct {
	Kind      EffectKind
	Owner     string
	Disposer  func() error
	CreatedAt time.Time
}

// CleanupEvidence is the observable result of a scope close.
type CleanupEvidence struct {
	ScopeID  string
	Owner    string
	Effects  int
	Disposed int
	Failed   int
	Duration time.Duration
	Reason   CloseReason
}

// ErrDisposalFailure wraps a single disposer error, retaining the scope, kind,
// and underlying cause so operators can attribute the leak.
type ErrDisposalFailure struct {
	ScopeID string
	Kind    EffectKind
	Cause   error
}

func (e *ErrDisposalFailure) Error() string {
	return fmt.Sprintf("effect disposal failed: scope=%s kind=%s: %v", e.ScopeID, e.Kind, e.Cause)
}

func (e *ErrDisposalFailure) Unwrap() error { return e.Cause }

// EffectScope tracks effects opened on behalf of one owner and disposes them
// deterministically on Close.
type EffectScope struct {
	mu       sync.Mutex
	owner    string
	scopeID  string
	ctx      contract.OperationContext
	effects  []Effect
	closed   bool
	failures []ErrDisposalFailure
}

// OpenScope returns a new scope for owner. ScopeID is a ULID so evidence rows
// sort chronologically and never collide.
func OpenScope(ctx contract.OperationContext, owner string) *EffectScope {
	return &EffectScope{
		owner:   owner,
		scopeID: contract.NewID(),
		ctx:     ctx,
	}
}

// ScopeID returns the scope's identity.
func (s *EffectScope) ScopeID() string { return s.scopeID }

// Owner returns the scope's owner.
func (s *EffectScope) Owner() string { return s.owner }

// Track records an effect for later disposal. A nil disposer is rejected:
// every tracked effect must be releasable.
func (s *EffectScope) Track(e Effect) error {
	if e.Disposer == nil {
		return fmt.Errorf("effect: %s has nil disposer", e.Kind)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("effect: scope %s already closed", s.scopeID)
	}
	e.Owner = s.owner
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	s.effects = append(s.effects, e)
	return nil
}

// Failures returns the disposer failures observed during the last Close.
func (s *EffectScope) Failures() []ErrDisposalFailure {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ErrDisposalFailure, len(s.failures))
	copy(out, s.failures)
	return out
}

// Close disposes all tracked effects LIFO, continuing past failures, and
// returns the CleanupEvidence. It is idempotent: a second Close returns
// evidence with zero effects.
func (s *EffectScope) Close(reason CloseReason) CleanupEvidence {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	evidence := CleanupEvidence{
		ScopeID: s.scopeID,
		Owner:   s.owner,
		Effects: len(s.effects),
		Reason:  reason,
	}

	if s.closed {
		return evidence
	}
	s.closed = true

	for i := len(s.effects) - 1; i >= 0; i-- {
		e := s.effects[i]
		if err := e.Disposer(); err != nil {
			evidence.Failed++
			s.failures = append(s.failures, ErrDisposalFailure{ScopeID: s.scopeID, Kind: e.Kind, Cause: err})
			continue
		}
		evidence.Disposed++
	}
	evidence.Duration = time.Since(start)
	s.effects = nil
	return evidence
}

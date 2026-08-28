package recon

import (
	"errors"
	"testing"
	"time"
)

func TestOnlyOneControllerHoldsLeasePerTenant(t *testing.T) {
	s := NewLeaseStore()
	now := time.Now().UTC()

	_, err := s.Acquire("tenant-a", "owner-1", time.Minute, now)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	_, err = s.Acquire("tenant-a", "owner-2", time.Minute, now)
	var held *ErrLeaseHeld
	if !errors.As(err, &held) {
		t.Fatalf("want ErrLeaseHeld, got %T: %v", err, err)
	}
	if held.Holder != "owner-1" {
		t.Fatalf("holder = %q, want owner-1", held.Holder)
	}
}

func TestStaleFenceRejectedAfterExpiryAndTakeover(t *testing.T) {
	s := NewLeaseStore()
	now := time.Now().UTC()

	lease1, _ := s.Acquire("tenant-a", "owner-1", time.Minute, now)
	// Owner-1's lease expires.
	later := now.Add(2 * time.Minute)
	lease2, err := s.Acquire("tenant-a", "owner-2", time.Minute, later)
	if err != nil {
		t.Fatalf("takeover after expiry: %v", err)
	}
	if lease2.Fence != lease1.Fence+1 {
		t.Fatalf("takeover fence = %d, want %d", lease2.Fence, lease1.Fence+1)
	}

	// Stale owner-1 can no longer validate or release.
	if err := s.Validate(lease1, later); err == nil {
		t.Fatalf("stale fence should fail validation")
	}
	var stale *ErrStaleFence
	if !errors.As(s.Release(lease1), &stale) {
		t.Fatalf("stale release should be ErrStaleFence")
	}
	// Current owner-2 still valid.
	if err := s.Validate(lease2, later); err != nil {
		t.Fatalf("current owner invalid: %v", err)
	}
}

func TestReleaseClearsLease(t *testing.T) {
	s := NewLeaseStore()
	now := time.Now().UTC()
	lease, _ := s.Acquire("tenant-a", "owner-1", time.Minute, now)
	if err := s.Release(lease); err != nil {
		t.Fatalf("release: %v", err)
	}
	// After release, another owner can acquire immediately.
	if _, err := s.Acquire("tenant-a", "owner-2", time.Minute, now); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
}

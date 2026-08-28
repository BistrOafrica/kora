package contract

import (
	"errors"
	"testing"
)

func TestStaleGenerationRejectedTypedError(t *testing.T) {
	client := GenerationInfo{Site: "s", ConfigVersion: 1, ProjectionHash: "h"}
	server := GenerationInfo{Site: "s", ConfigVersion: 2, ProjectionHash: "h"}

	err := CheckProjectionGate(client, server, nil, nil)
	var stale *StaleProjectionError
	if !errors.As(err, &stale) {
		t.Fatalf("want *StaleProjectionError, got %T: %v", err, err)
	}
	if stale.Kind != "stale_generation" || stale.ClientGeneration != 1 || stale.ServerGeneration != 2 {
		t.Fatalf("unexpected stale error: %+v", stale)
	}
}

func TestProjectionHashMismatchRejected(t *testing.T) {
	client := GenerationInfo{Site: "s", ConfigVersion: 2, ProjectionHash: "old"}
	server := GenerationInfo{Site: "s", ConfigVersion: 2, ProjectionHash: "new"}

	err := CheckProjectionGate(client, server, nil, nil)
	var stale *StaleProjectionError
	if !errors.As(err, &stale) {
		t.Fatalf("want *StaleProjectionError, got %T: %v", err, err)
	}
}

func TestCapabilityMissingRejected(t *testing.T) {
	client := GenerationInfo{Site: "s", ConfigVersion: 2, ProjectionHash: "h"}
	server := GenerationInfo{Site: "s", ConfigVersion: 2, ProjectionHash: "h"}

	err := CheckProjectionGate(client, server, []string{"offline.sync", "barcode.scan"}, []string{"offline.sync"})
	var stale *StaleProjectionError
	if !errors.As(err, &stale) {
		t.Fatalf("want *StaleProjectionError, got %T: %v", err, err)
	}
	if stale.Kind != "capability_missing" || stale.MissingCapability != "barcode.scan" {
		t.Fatalf("unexpected capability error: %+v", stale)
	}
}

func TestProjectionGateAcceptsMatching(t *testing.T) {
	client := GenerationInfo{Site: "s", ConfigVersion: 3, ProjectionHash: "h"}
	server := GenerationInfo{Site: "s", ConfigVersion: 3, ProjectionHash: "h"}
	if err := CheckProjectionGate(client, server, []string{"offline.sync"}, []string{"offline.sync", "barcode.scan"}); err != nil {
		t.Fatalf("matching gate rejected: %v", err)
	}
}

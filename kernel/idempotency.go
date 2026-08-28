package kernel

import (
	"context"
	"database/sql"
	"errors"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/db"
)

// Receipt status values for _kora_idempotency_receipt.
const (
	receiptCompleted = "completed"
)

// lookupReceipt returns the committed result of a prior operation with the
// same (site, idempotency key). A payload-hash mismatch is a key reuse: the
// caller intended a different operation under a key that is already spent.
func (k *Kernel) lookupReceipt(ctx context.Context, siteDB *sql.DB, op Operation, opID string) (contract.CommandResult, bool, *contract.Error) {
	var (
		storedPayloadHash string
		resultHash        string
		status            string
	)
	err := siteDB.QueryRowContext(ctx,
		db.Rebind(k.dialect(), `SELECT payload_hash, result_hash, status FROM _kora_idempotency_receipt
		 WHERE site = ? AND idempotency_key = ?`),
		op.Context.Site, op.Context.IdempotencyKey,
	).Scan(&storedPayloadHash, &resultHash, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return contract.CommandResult{}, false, nil
	}
	if err != nil {
		return contract.CommandResult{}, false, contract.NewError(contract.CodeDependencyUnavailable, "idempotency lookup failed")
	}
	if storedPayloadHash != "" && storedPayloadHash != payloadHash(op.Payload) {
		return contract.CommandResult{}, true, ErrKeyReused
	}
	// A completed receipt replays as an empty completed result carrying only
	// the recorded result hash; full result replay lands with SPEC-005's
	// canonical envelope (receipt stores result_hash today).
	return contract.CommandResult{
		OperationID:   opID,
		CorrelationID: op.Context.CorrelationID,
		Status:        contract.StatusCompleted,
		Replayed:      true,
	}, true, nil
}

// claimReceipt inserts the receipt inside the active transaction. A primary-
// key collision means a concurrent operation committed the same key first:
// identical payloads are treated as replay; different payloads are reuse.
func (k *Kernel) claimReceipt(dbTx *sql.Tx, op Operation, def CommandDefinition, opID, pHash string) *contract.Error {
	_, err := dbTx.Exec(
		db.Rebind(k.dialect(), `INSERT INTO _kora_idempotency_receipt
			(site, idempotency_key, operation_id, command_name, payload_hash, result_hash, status, actor_user)
		 VALUES (?, ?, ?, ?, ?, '', ?, ?)`),
		op.Context.Site,
		op.Context.IdempotencyKey,
		opID,
		def.Name,
		pHash,
		receiptCompleted,
		op.Context.User,
	)
	if err == nil {
		return nil
	}
	// Unique violation → concurrent duplicate. Distinguish by payload hash.
	var existing string
	scanErr := dbTx.QueryRow(
		db.Rebind(k.dialect(), `SELECT payload_hash FROM _kora_idempotency_receipt
		 WHERE site = ? AND idempotency_key = ?`),
		op.Context.Site, op.Context.IdempotencyKey,
	).Scan(&existing)
	if scanErr == nil && existing != pHash {
		return ErrKeyReused
	}
	return contract.NewError(contract.CodeIdempotencyKeyReused, "idempotency key concurrently used")
}

// finalizeReceipt records the committed result hash so later replays can be
// verified. Failure to finalize degrades to at-least-once execution safety:
// the business commit stands, and the next attempt re-executes under the same
// key and hits the receipt's unique constraint instead of double-applying.
func (k *Kernel) finalizeReceipt(ctx context.Context, siteDB *sql.DB, op Operation, opID, rHash string) {
	_, _ = siteDB.ExecContext(ctx,
		db.Rebind(k.dialect(), `UPDATE _kora_idempotency_receipt SET result_hash = ? WHERE site = ? AND idempotency_key = ?`),
		rHash, op.Context.Site, op.Context.IdempotencyKey,
	)
}

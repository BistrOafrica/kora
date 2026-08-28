package contract

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewID returns a ULID string for envelopes, outbox rows, and receipts.
func NewID() string {
	return ulid.Make().String()
}

// NewIDAt returns a ULID string for the given monotonic time.
func NewIDAt(t time.Time) string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

// NewEventID returns a ULID suitable for an EventEnvelope.ID.
func NewEventID() string { return NewID() }

// NewCommandID returns a ULID suitable for a CommandEnvelope.ID.
func NewCommandID() string { return NewID() }

// NewOperationID returns a ULID suitable for CommandResult.OperationID and
// TaskReceipt.OperationID.
func NewOperationID() string { return NewID() }

// NewCorrelationID returns a ULID suitable for a correlation trace.
func NewCorrelationID() string { return NewID() }

// CurrentVersion is the single frozen contract schema version emitted by this
// package. Envelope implementations set their Version from this value.
const CurrentVersion = 1

// EncodeData marshals a value into the json.RawMessage field of an envelope.
func EncodeData(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// MustEncodeData marshals a value into json.RawMessage, panicking on error. Use
// only for values known to be JSON-encodable.
func MustEncodeData(v any) json.RawMessage {
	b, err := EncodeData(v)
	if err != nil {
		panic(fmt.Sprintf("contract: encode data: %v", err))
	}
	return b
}

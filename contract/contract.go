// Package contract defines the provider-neutral, versioned messaging contracts
// for the Kora Engine execution fabric.
//
// These types are the frozen wire-level contracts from KORA-ENGINE-RFC.md §7
// (provider contracts), §7.1 (command and query protocol), §7.3 (identity and
// delegation), and §8 (durable delivery). They are deliberately independent of
// any concrete provider (LocalProvider or NATSProvider) and of the analytics,
// orm, and webhook packages so that every adapter (HTTP, chat, channel, MCP,
// SDK, UI, NATS) can project from a single canonical source.
//
// Once a contract version is frozen, its JSON shape must not change in a
// breaking way. Additive changes bump the version field on the envelope and are
// accompanied by a contract version test in contract_test.go.
package contract

import (
	"context"
	"encoding/json"
	"time"
)

// Status is the operation result state for a command or query.
//
// The RFC defines exactly these five states. They are stable machine-readable
// values used uniformly across all externally callable operations.
type Status string

const (
	StatusCompleted Status = "completed"
	StatusAccepted  Status = "accepted"
	StatusRejected  Status = "rejected"
	StatusConflict  Status = "conflict"
	StatusFailed    Status = "failed"
	StatusPending   Status = "pending"
)

// Code is a stable, machine-readable error code. These values must not be
// renamed or repurposed once frozen, because callers depend on them.
type Code string

const (
	CodePermissionDenied      Code = "PERMISSION_DENIED"
	CodeValidationFailed      Code = "VALIDATION_FAILED"
	CodeNotFound              Code = "NOT_FOUND"
	CodeConflict              Code = "CONFLICT"
	CodeDeadlineExceeded      Code = "DEADLINE_EXCEEDED"
	CodeDependencyUnavailable Code = "DEPENDENCY_UNAVAILABLE"
	CodeIdempotencyKeyReused  Code = "IDEMPOTENCY_KEY_REUSED"
	CodeUnauthenticated       Code = "UNAUTHENTICATED"
	CodeInternal              Code = "INTERNAL_ERROR"
)

// PrincipalType classifies the kind of authenticated principal carrying out an
// operation. ai-assistant, mcp-agent, and similar strings are not implicit
// users, roles, or owners; they are always resolved to a typed principal.
type PrincipalType string

const (
	PrincipalHuman   PrincipalType = "human"
	PrincipalService PrincipalType = "service"
	PrincipalAgent   PrincipalType = "agent"
)

// Projection is the consumer-approved data projection that an event payload may
// carry (RFC §8.3). Full documents and old-document snapshots are restricted to
// explicitly authorized audit or rebuild consumers.
type Projection string

const (
	ProjectionMetadata      Projection = "metadata"
	ProjectionChangedFields Projection = "changed_fields"
	ProjectionSummary       Projection = "summary"
	ProjectionFullDocument  Projection = "full_document"
)

// NormalizeCode returns the Code for an unstructured error string. It is a
// best-effort mapping used when an adapter cannot produce a typed error; the
// caller should prefer wrapping with a typed Error via NewError.
func NormalizeCode(s string) Code {
	switch s {
	case "PERMISSION_DENIED":
		return CodePermissionDenied
	case "VALIDATION_FAILED":
		return CodeValidationFailed
	case "NOT_FOUND":
		return CodeNotFound
	case "CONFLICT":
		return CodeConflict
	case "DEADLINE_EXCEEDED":
		return CodeDeadlineExceeded
	case "DEPENDENCY_UNAVAILABLE":
		return CodeDependencyUnavailable
	case "IDEMPOTENCY_KEY_REUSED":
		return CodeIdempotencyKeyReused
	case "UNAUTHENTICATED":
		return CodeUnauthenticated
	default:
		return CodeInternal
	}
}

// Error is the structured, versioned error contract. Error codes are stable
// machine-readable values per RFC §7.1.
type Error struct {
	Type    Code            `json:"type"`
	Message string          `json:"message"`
	Field   string          `json:"field,omitempty"`
	Details json.RawMessage `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// NewError builds a structured error with the given code and message.
func NewError(code Code, message string) *Error {
	return &Error{Type: code, Message: message}
}

// ActorContext is the versioned identity/delegation context attached to every
// command (RFC §7.3). It records who is executing and, where applicable, the
// human on whose behalf a service or agent acts.
type ActorContext struct {
	PrincipalID     string        `json:"principal_id"`
	PrincipalType   PrincipalType `json:"principal_type"` // human|service|agent
	SubjectUserID   string        `json:"subject_user_id,omitempty"`
	OrganizationID  string        `json:"organization_id,omitempty"`
	Site            string        `json:"site"`
	Roles           []string      `json:"roles,omitempty"`
	Scopes          []string      `json:"scopes,omitempty"`
	Channel         string        `json:"channel,omitempty"`
	DeviceID        string        `json:"device_id,omitempty"`
	AuthenticatedAt time.Time     `json:"authenticated_at"`
	AuthSessionID   string        `json:"auth_session_id,omitempty"`
	DelegationID    string        `json:"delegation_id,omitempty"`
}

// Authenticated reports whether the actor carries an identity. Missing identity
// fails closed in production paths.
func (a ActorContext) Authenticated() bool {
	return a.PrincipalID != "" && a.PrincipalType != ""
}

// CommandEnvelope is the versioned transport for a command (RFC §7). It is
// serialized as the command is dispatched through any provider.
type CommandEnvelope struct {
	Type           string          `json:"type"`
	Version        int             `json:"version"`
	ID             string          `json:"id"`
	Site           string          `json:"site"`
	Actor          ActorContext    `json:"actor"`
	CorrelationID  string          `json:"correlation_id,omitempty"`
	CausationID    string          `json:"causation_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Deadline       time.Time       `json:"deadline"`
	Data           json.RawMessage `json:"data,omitempty"`
}

// Expired reports whether the command deadline has passed. A missing or expired
// deadline returns CodeDeadlineExceeded before business execution.
func (c CommandEnvelope) Expired(now time.Time) bool {
	return !c.Deadline.IsZero() && !now.Before(c.Deadline)
}

// EventEnvelope is the versioned transport for a domain event (RFC §7/§8.3).
type EventEnvelope struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Version       int             `json:"version"`
	Source        string          `json:"source"`
	Site          string          `json:"site"`
	AggregateType string          `json:"aggregate_type,omitempty"`
	AggregateID   string          `json:"aggregate_id,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	CausationID   string          `json:"causation_id,omitempty"`
	Data          json.RawMessage `json:"data"`
}

// CommandResult is the versioned result of a synchronous command (RFC §7.1).
type CommandResult struct {
	OperationID   string          `json:"operation_id"`
	CorrelationID string          `json:"correlation_id"`
	Status        Status          `json:"status"`
	Data          json.RawMessage `json:"data,omitempty"`
	Error         *Error          `json:"error,omitempty"`
	Replayed      bool            `json:"replayed,omitempty"`
}

// TaskReceipt acknowledges a submitted (asynchronous) command. Receiving a
// receipt does not imply completion (RFC §7).
type TaskReceipt struct {
	OperationID   string    `json:"operation_id"`
	CorrelationID string    `json:"correlation_id"`
	Status        Status    `json:"status"`
	AcceptedAt    time.Time `json:"accepted_at"`
}

// Delivery carries a message to a Consumer handler (RFC §7).
type Delivery struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Site    string          `json:"site"`
	Data    json.RawMessage `json:"data"`
	Attempt int             `json:"attempt"`
}

// Handler processes a single Delivery. Returning a nil error acknowledges the
// message; returning a non-nil error causes redelivery per policy.
type Handler func(ctx context.Context, d Delivery) error

// EventPublisher publishes events. Local WAL rotation/replay is not part of this
// contract; it lives behind LocalProvider.
type EventPublisher interface {
	Publish(ctx context.Context, event EventEnvelope) error
}

// CommandBus dispatches commands synchronously (Request) or asynchronously
// (Submit).
type CommandBus interface {
	Request(ctx context.Context, command CommandEnvelope) (CommandResult, error)
	Submit(ctx context.Context, command CommandEnvelope) (TaskReceipt, error)
}

// Consumer subscribes to a durable message stream.
type Consumer interface {
	Run(ctx context.Context, handler Handler) error
	Drain(ctx context.Context) error
}

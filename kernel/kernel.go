// Package kernel implements the single operation kernel (architecture
// specification invariants 2–3, KERNEL-001..007): every mutation from every
// source — HTTP, SDK, MCP, AI, CLI, UI, integrations, workers — enters one
// pipeline that resolves tenant and actor, authorizes, validates, checks
// idempotency and concurrency, executes one SQL transaction containing the
// business mutation plus the idempotency receipt, the audit row, and the
// outbox events, and returns a typed OperationResult.
//
// The kernel never talks to a provider directly; durable delivery happens
// exclusively through the transactional outbox and its publishers.
package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/db"
	"github.com/asenawritescode/kora/doctype"
	"github.com/asenawritescode/kora/orm"
	"github.com/asenawritescode/kora/outbox"
)

// Source identifies the adapter surface that produced an operation. It is
// recorded on the audit ledger and drives authorization-parity tests: two
// sources executing the same command must produce identical outcomes.
type Source string

const (
	SourceHTTP        Source = "http"
	SourceSDK         Source = "sdk"
	SourceMCP         Source = "mcp"
	SourceAI          Source = "ai"
	SourceCLI         Source = "cli"
	SourceUI          Source = "ui"
	SourceWorker      Source = "worker"
	SourceIntegration Source = "integration"
)

// Command names are canonical across all sources. Adapters resolve these
// definitions from the registry; they never implement their own semantics.
const (
	CommandRecordCreate = "record.create"
	CommandRecordUpdate = "record.update"
)

// RecordCreatePayload is the typed payload for record.create. Field values
// are schema-generic by design (the engine is config-driven) but are always
// validated against the doctype definition before execution; invalid shapes
// produce typed ValidationError lists, never silent acceptance.
type RecordCreatePayload struct {
	Doctype string          `json:"doctype"`
	Data    json.RawMessage `json:"data"`
}

// RecordUpdatePayload is the typed payload for record.update.
type RecordUpdatePayload struct {
	Doctype string          `json:"doctype"`
	Name    string          `json:"name"`
	Data    json.RawMessage `json:"data"`
}

// CommandDefinition is the single registered description of a command:
// its payload shape, authorization operation, idempotency class, and timeout.
type CommandDefinition struct {
	Name            string
	Version         int
	AuthorizesWith  string // permission-matrix operation ("create", "write")
	IdempotentByKey bool   // replay-safe when an idempotency key is present
}

// commandRegistry holds the canonical command definitions of the first
// vertical slice. Additional commands register here as epics land.
var commandRegistry = map[string]CommandDefinition{
	CommandRecordCreate: {Name: CommandRecordCreate, Version: 1, AuthorizesWith: "create", IdempotentByKey: true},
	CommandRecordUpdate: {Name: CommandRecordUpdate, Version: 1, AuthorizesWith: "write", IdempotentByKey: true},
}

// LookupCommand returns the canonical definition for a command name.
func LookupCommand(name string) (CommandDefinition, bool) {
	d, ok := commandRegistry[name]
	return d, ok
}

// OperationContext carries everything an adapter must resolve before invoking
// the kernel. It is an explicit value, never smuggled through context.Context.
type OperationContext struct {
	Site           string                // tenant identifier (site name)
	Actor          contract.ActorContext // authenticated principal
	User           string                // owner-attribution user ("" → "system")
	Roles          []string              // roles used for permission-matrix lookup
	Source         Source                // adapter surface
	CorrelationID  string
	CausationID    string
	ExpectedVersion string
	IdempotencyKey string
}

// Operation is a fully specified invocation handed to Kernel.Execute.
type Operation struct {
	Context  OperationContext
	Command  string          // canonical command name
	Payload  json.RawMessage // JSON-encoded typed payload
	Deadline time.Time       // zero means no deadline; enforced before execution
}

// ResultData is the typed success payload returned by record commands.
type ResultData struct {
	Doctype   string `json:"doctype"`
	Name      string `json:"name"`
	Created   bool   `json:"created"`
	AuditID   string `json:"audit_id"`
	Operation string `json:"operation_id"`
}

// Kernel is the process-wide operation entry point. It is safe for concurrent
// use; per-site state (DB handle, registry) is supplied per call via SiteDB.
type Kernel struct {
	Dialect db.Dialect
	Outbox  outbox.Writer

	// Commands holds config-defined command resources (KERNEL-008). Nil means
	// only built-in commands are available. Definitions are generation-scoped:
	// swap the registry atomically on activation.
	Commands *CommandRegistry
}

// New returns a Kernel. Outbox may be nil only in tests that do not assert
// event delivery; production wiring always supplies outbox.NewSQLWriter().
func New(dialect db.Dialect, w outbox.Writer) *Kernel {
	return &Kernel{Dialect: dialect, Outbox: w}
}

// Execute runs the full pipeline for op against the given site database and
// registry. Exactly one of the following occurs:
//
//   - completed: business row + receipt + audit + outbox rows committed atomically;
//   - replayed:  a prior committed result for the same site+idempotency key is returned;
//   - rejected:  a typed error; nothing is persisted (transaction rolled back).
func (k *Kernel) Execute(ctx context.Context, siteDB *sql.DB, reg *doctype.Registry, op Operation) (contract.CommandResult, *contract.Error) {
	opID := contract.NewOperationID()

	if err := k.validateContext(op); err != nil {
		return k.rejected(opID, op, err)
	}
	if !op.Deadline.IsZero() && !time.Now().Before(op.Deadline) {
		return k.rejected(opID, op, contract.NewError(contract.CodeDeadlineExceeded, "operation deadline exceeded"))
	}
	def, ok := LookupCommand(op.Command)
	var dyn *CommandResource
	if !ok {
		if c, found := k.Commands.Lookup(op.Command); found {
			dyn = c
			def = CommandDefinition{
				Name:            c.FullName(),
				Version:         c.Version,
				AuthorizesWith:  c.PermOperation(),
				IdempotentByKey: true,
			}
		} else {
			return k.rejected(opID, op, contract.NewError(contract.CodeValidationFailed, "unknown command "+op.Command))
		}
	}

	// Authorization parity gate: identical evaluation regardless of source.
	authzErr := authorizeOp(reg, op, def, dyn)
	if authzErr != nil {
		k.writeDenialAudit(siteDB, op, def, opID, authzErr)
		return k.rejected(opID, op, authzErr)
	}

	// Idempotency fast path: return a committed result without re-executing.
	if op.Context.IdempotencyKey != "" {
		res, found, err := k.lookupReceipt(ctx, siteDB, op, opID)
		if err != nil {
			return k.rejected(opID, op, err)
		}
		if found {
			return res, nil
		}
	}

	txMgr := &orm.TxManager{
		DB:          siteDB,
		Registry:    reg,
		Dialect:     k.Dialect,
		Outbox:      k.Outbox,
		SiteName:    op.Context.Site,
		CurrentUser: op.Context.User,
	}

	if dyn != nil {
		data, execErr := k.execDefinedCommand(ctx, siteDB, txMgr, reg, op, def, opID, dyn)
		if execErr != nil {
			return k.rejected(opID, op, execErr)
		}
		result := contract.CommandResult{
			OperationID:   opID,
			CorrelationID: op.Context.CorrelationID,
			Status:        contract.StatusCompleted,
			Data:          data,
		}
		if op.Context.IdempotencyKey != "" && def.IdempotentByKey {
			k.finalizeReceipt(ctx, siteDB, op, opID, resultHash(result))
		}
		return result, nil
	}

	data, execErr := k.executeInTx(ctx, siteDB, txMgr, reg, op, def, opID)
	if execErr != nil {
		return k.rejected(opID, op, execErr)
	}

	result := contract.CommandResult{
		OperationID:   opID,
		CorrelationID: op.Context.CorrelationID,
		Status:        contract.StatusCompleted,
	}
	if data != nil {
		raw, _ := json.Marshal(data)
		result.Data = raw
	}

	// Post-commit: finalize the receipt so replays return this exact result.
	if op.Context.IdempotencyKey != "" && def.IdempotentByKey {
		k.finalizeReceipt(ctx, siteDB, op, opID, resultHash(result))
	}

	return result, nil
}

// dialect returns the configured SQL dialect for placeholder rewriting.
// A nil dialect falls back to MySQL-style '?' placeholders.
func (k *Kernel) dialect() db.Dialect {
	if k.Dialect == nil {
		return &db.MySQLDialect{}
	}
	return k.Dialect
}

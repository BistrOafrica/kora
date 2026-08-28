package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/asenawritescode/kora/contract"
	"github.com/asenawritescode/kora/doctype"
	"github.com/asenawritescode/kora/orm"
)

// Typed sentinel errors. Adapters map these to wire errors via contract.Code;
// they must never be detected by string matching.
var (
	ErrUnauthenticated = contract.NewError(contract.CodeUnauthenticated, "operation context has no authenticated actor")
	ErrNoSite          = contract.NewError(contract.CodeValidationFailed, "operation context missing tenant site")
	ErrPermission      = contract.NewError(contract.CodePermissionDenied, "not permitted")
	ErrStaleVersion    = contract.NewError(contract.CodeConflict, "document was modified by another operation")
	ErrKeyReused       = contract.NewError(contract.CodeIdempotencyKeyReused, "idempotency key already used with a different payload")
)

// validateContext enforces fail-closed identity: no site or no authenticated
// principal means no execution. A mismatch between the executing site and the
// actor's home site is a cross-tenant attempt and is rejected.
func (k *Kernel) validateContext(op Operation) *contract.Error {
	if op.Context.Site == "" {
		return ErrNoSite
	}
	if !op.Context.Actor.Authenticated() {
		return ErrUnauthenticated
	}
	if op.Context.Actor.Site != "" && op.Context.Actor.Site != op.Context.Site {
		return ErrPermission
	}
	return nil
}

// authorize evaluates the site registry's permission matrix identically for
// every source. AI/MCP actors carry resolved human subjects; the gate never
// special-cases them (authorization parity, KERNEL-004 / SEC-003).
func authorize(reg *doctype.Registry, op Operation, def CommandDefinition) *contract.Error {
	if reg == nil {
		return ErrPermission
	}
	doctypeName := payloadDoctype(op.Payload)
	if doctypeName == "" {
		return contract.NewError(contract.CodeValidationFailed, "payload missing doctype")
	}
	roles := op.Context.Roles
	if len(roles) == 0 {
		roles = []string{doctype.AdminRole}
	}
	if allowed, _ := reg.CanUser(roles, doctypeName, def.AuthorizesWith); !allowed {
		return ErrPermission
	}
	return nil
}

// payloadDoctype extracts the target doctype from either typed payload.
func payloadDoctype(raw json.RawMessage) string {
	var probe struct {
		Doctype string `json:"doctype"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	return probe.Doctype
}

// executeInTx runs mutation + receipt claim + audit write inside ONE SQL
// transaction (the mutation itself writes its outbox rows via orm InTx
// variants). Any failure rolls back everything; a commit implies all of it.
func (k *Kernel) executeInTx(ctx context.Context, siteDB *sql.DB, txMgr *orm.TxManager, reg *doctype.Registry, op Operation, def CommandDefinition, opID string) (*ResultData, *contract.Error) {

	dbTx, err := siteDB.Begin()
	if err != nil {
		return nil, contract.NewError(contract.CodeDependencyUnavailable, "begin transaction: "+err.Error())
	}
	defer dbTx.Rollback()

	doctypeName := payloadDoctype(op.Payload)
	dt := reg.Get(doctypeName)
	if dt == nil {
		return nil, contract.NewError(contract.CodeNotFound, fmt.Sprintf("doctype %q not found", doctypeName))
	}

	user := op.Context.User
	if user == "" {
		user = "system"
	}

	var result *ResultData

	switch op.Command {
	case CommandRecordCreate:
		var p RecordCreatePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return nil, contract.NewError(contract.CodeValidationFailed, "invalid record.create payload")
		}
		doc := doctype.NewDocument(dt.Name)
		if err := applyData(dt, doc, p.Data); err != nil {
			return nil, contract.NewError(contract.CodeValidationFailed, err.Error())
		}
		if verrs := doctype.ValidateDocument(dt, doc, reg, nil); verrs.HasErrors() {
			return nil, validationContractError(verrs)
		}
		if op.Context.IdempotencyKey != "" && def.IdempotentByKey {
			if cerr := k.claimReceipt(dbTx, op, def, opID, payloadHash(op.Payload)); cerr != nil {
				return nil, cerr
			}
		}
		afterHash := CanonicalDocHash(doc.Fields)
		if err := txMgr.InsertInTx(dbTx, dt, doc, user, user); err != nil {
			return nil, wrapORMError(err)
		}
		doc.IsNew = false
		row := k.buildAuditRow(op, def, opID, dt.Name, doc.Name, contract.StatusCompleted, "")
		row.AfterHash = afterHash
		auditID, aerr := k.writeAudit(dbTx, row)
		if aerr != nil {
			return nil, contract.NewError(contract.CodeInternal, "audit write failed")
		}
		result = &ResultData{Doctype: dt.Name, Name: doc.Name, Created: true, AuditID: auditID, Operation: opID}

	case CommandRecordUpdate:
		var p RecordUpdatePayload
		if err := json.Unmarshal(op.Payload, &p); err != nil {
			return nil, contract.NewError(contract.CodeValidationFailed, "invalid record.update payload")
		}
		if p.Name == "" {
			return nil, contract.NewError(contract.CodeValidationFailed, "record.update missing name")
		}
		oldDoc, err := txMgr.GetDoc(dt, p.Name, "")
		if err != nil {
			if errors.Is(err, orm.ErrNotFound) {
				return nil, contract.NewError(contract.CodeNotFound, "document not found")
			}
			return nil, contract.NewError(contract.CodeDependencyUnavailable, "load document failed")
		}
		// Optimistic concurrency: read the row's current version token
		// (`modified`) inside this transaction and compare against the
		// caller's expected_version in canonical form.
		if ev := expectedVersionFrom(op); ev != "" {
			var priorModified sql.NullTime
			if err := dbTx.QueryRow(
				fmt.Sprintf("SELECT modified FROM %s WHERE name = ?", dt.TableName()), p.Name,
			).Scan(&priorModified); err != nil {
				return nil, contract.NewError(contract.CodeNotFound, "document not found")
			}
			if CanonicalVersion(priorModified.Time) != ev {
				k.writeFailureAudit(siteDB, op, def, opID, ErrStaleVersion.Type)
				return nil, ErrStaleVersion
			}
		}
		doc := cloneDoc(oldDoc)
		if err := applyData(dt, doc, p.Data); err != nil {
			return nil, contract.NewError(contract.CodeValidationFailed, err.Error())
		}
		doc.IsNew = false
		if verrs := doctype.ValidateDocument(dt, doc, reg, oldDoc); verrs.HasErrors() {
			return nil, validationContractError(verrs)
		}
		if op.Context.IdempotencyKey != "" && def.IdempotentByKey {
			if cerr := k.claimReceipt(dbTx, op, def, opID, payloadHash(op.Payload)); cerr != nil {
				return nil, cerr
			}
		}
		beforeHash := CanonicalDocHash(oldDoc.Fields)
		afterHash := CanonicalDocHash(doc.Fields)
		if err := txMgr.SaveInTx(dbTx, dt, doc, user, "", oldDoc); err != nil {
			return nil, wrapORMError(err)
		}
		row := k.buildAuditRow(op, def, opID, dt.Name, doc.Name, contract.StatusCompleted, "")
		row.BeforeHash = beforeHash
		row.AfterHash = afterHash
		auditID, aerr := k.writeAudit(dbTx, row)
		if aerr != nil {
			return nil, contract.NewError(contract.CodeInternal, "audit write failed")
		}
		result = &ResultData{Doctype: dt.Name, Name: doc.Name, Created: false, AuditID: auditID, Operation: opID}

	default:
		return nil, contract.NewError(contract.CodeValidationFailed, "unknown command "+op.Command)
	}

	if err := dbTx.Commit(); err != nil {
		return nil, contract.NewError(contract.CodeInternal, "commit failed")
	}
	return result, nil
}

// cloneDoc copies a loaded document so mutation attempts never alias the
// previously persisted state.
func cloneDoc(d *doctype.Document) *doctype.Document {
	c := doctype.NewDocument(d.DocType)
	for k, v := range d.Fields {
		c.Fields[k] = v
	}
	c.Name = d.Name
	c.DocStatus = d.DocStatus
	c.IsNew = false
	return c
}

// applyData merges JSON field data into a document, rejecting unknown keys —
// unsupported configuration is never silently discarded.
func applyData(dt *doctype.DocType, doc *doctype.Document, raw json.RawMessage) error {
	var fields map[string]any
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("data is not a JSON object")
	}
	for key, val := range fields {
		f := dt.GetField(key)
		if f == nil {
			return fmt.Errorf("unknown field %q on doctype %q", key, dt.Name)
		}
		doc.Set(key, val)
	}
	return nil
}

func validationContractError(verrs doctype.ValidationErrors) *contract.Error {
	raw, _ := json.Marshal(verrs)
	e := contract.NewError(contract.CodeValidationFailed, "validation failed")
	e.Details = raw
	return e
}

// wrapORMError converts ORM sentinel errors into typed contract errors.
func wrapORMError(err error) *contract.Error {
	switch {
	case errors.Is(err, orm.ErrDuplicate):
		return contract.NewError(contract.CodeConflict, sanitize(err.Error()))
	case errors.Is(err, orm.ErrNotFound):
		return contract.NewError(contract.CodeNotFound, "document not found or access denied")
	default:
		return contract.NewError(contract.CodeValidationFailed, sanitize(err.Error()))
	}
}

// sanitize strips SQL driver internals from user-facing messages while keeping
// the meaningful suffix.
func sanitize(msg string) string {
	if i := strings.LastIndex(msg, ": "); i > 0 && strings.Count(msg, ": ") > 1 {
		return msg[i+2:]
	}
	return msg
}

func (k *Kernel) rejected(opID string, op Operation, e *contract.Error) (contract.CommandResult, *contract.Error) {
	return contract.CommandResult{
		OperationID:   opID,
		CorrelationID: op.Context.CorrelationID,
		Status:        statusFor(e.Type),
		Error:         e,
	}, e
}

func statusFor(c contract.Code) contract.Status {
	switch c {
	case contract.CodeConflict:
		return contract.StatusConflict
	case contract.CodePermissionDenied, contract.CodeUnauthenticated,
		contract.CodeValidationFailed, contract.CodeNotFound, contract.CodeIdempotencyKeyReused:
		return contract.StatusRejected
	default:
		return contract.StatusFailed
	}
}

// expectedVersionFrom reads the optimistic-concurrency token from the
// envelope-level convention: Context.CausationID prefixed "expected:".
// A dedicated envelope field lands with SPEC-003's canonical envelope work;
// absence of the token preserves backward compatibility (no guard).
func expectedVersionFrom(op Operation) string {
	if strings.TrimSpace(op.Context.ExpectedVersion) != "" {
		return strings.TrimSpace(op.Context.ExpectedVersion)
	}
	if strings.HasPrefix(op.Context.CausationID, "expected:") {
		return strings.TrimPrefix(op.Context.CausationID, "expected:")
	}
	return ""
}

// CanonicalVersion renders a document version token (the `modified`
// timestamp) in a stable string form so callers can echo it back as
// expected_version regardless of driver time parsing.
func CanonicalVersion(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// CanonicalDocHash renders a document's field map as sorted-key JSON and
// returns the sha256 hex digest — the audit before/after state token
// (KERNEL-009). Hash input is deterministic: sorted keys, JSON-encoded values.
func CanonicalDocHash(fields map[string]any) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(fields[k])
		sb.Write(kb)
		sb.WriteByte(':')
		sb.Write(vb)
	}
	sb.WriteByte('}')
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

func payloadHash(raw json.RawMessage) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func resultHash(r contract.CommandResult) string {
	raw, _ := json.Marshal(struct {
		Status contract.Status `json:"status"`
		Data   json.RawMessage `json:"data,omitempty"`
		Error  *contract.Error `json:"error,omitempty"`
	}{r.Status, r.Data, r.Error})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

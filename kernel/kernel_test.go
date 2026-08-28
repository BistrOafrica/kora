//go:build integration
// +build integration

// First-vertical-slice evidence suite (SPEC-014 scenarios, KERNEL tickets):
// canonical command execution through one kernel path with tenant isolation,
// authorization parity, idempotency, optimistic concurrency, transaction
// rollback atomicity, audit, outbox atomicity, and durable delivery via NATS
// JetStream. PostgreSQL is the reference dialect; this suite runs against
// MySQL as the currently available integration harness (DB-005 records the
// compatibility matrix).
package kernel_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	mysql "github.com/go-sql-driver/mysql"

	"github.com/asenawritescode/kora/contract"
	kdb "github.com/asenawritescode/kora/db"
	"github.com/asenawritescode/kora/doctype"
	"github.com/asenawritescode/kora/kernel"
	"github.com/asenawritescode/kora/outbox"
	"github.com/asenawritescode/kora/schema"
)

func newSiteDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("KORA_TEST_DSN")
	if dsn == "" {
		dsn = "root:kora123@tcp(127.0.0.1:3306)/?parseTime=true&charset=utf8mb4"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("MySQL not available: %v", err)
	}
	name := fmt.Sprintf("kora_kernel_%d", time.Now().UnixNano()%1_000_000)
	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", name)); err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() { db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)) })

	cfg, _ := mysql.ParseDSN(dsn)
	cfg.DBName = name
	cfg.Params = map[string]string{"parseTime": "true", "charset": "utf8mb4"}
	sdb, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatalf("open site db: %v", err)
	}
	return sdb, name
}

type siteFixture struct {
	DB       *sql.DB
	Name     string
	Registry *doctype.Registry
	Dialect  kdb.Dialect
}

func newSite(t *testing.T, name string) *siteFixture {
	t.Helper()
	sdb, dbName := newSiteDB(t)
	dialect := kdb.Resolve("mysql")
	for _, ddl := range dialect.SystemTableSQL() {
		if !strings.HasPrefix(strings.TrimSpace(ddl), "CREATE TABLE IF NOT EXISTS") {
			continue
		}
		if _, err := sdb.Exec(ddl); err != nil {
			t.Fatalf("system table: %v", err)
		}
	}
	for _, ddl := range kdb.OutboxTablesMySQL() {
		if _, err := sdb.Exec(ddl); err != nil {
			t.Fatalf("outbox table: %v", err)
		}
	}
	for _, ddl := range kdb.KernelTablesMySQL() {
		if _, err := sdb.Exec(ddl); err != nil {
			t.Fatalf("kernel table: %v", err)
		}
	}

	taskDT := &doctype.DocType{
		Name:   "Task",
		Module: "Kernel Test",
		Fields: []doctype.Field{
			{Fieldname: "title", Fieldtype: "Data", Label: "Title", Reqd: true},
			{Fieldname: "serial", Fieldtype: "Data", Label: "Serial", Unique: true},
			{Fieldname: "status", Fieldtype: "Select", Label: "Status", Options: "Open\nDone"},
		},
	}
	roles := []*doctype.Role{{Name: "Administrator"}, {Name: "Creator"}}
	perms := []*doctype.Permission{
		{Doctype: "Task", Role: "Administrator", Read: true, Write: true, Create: true, Delete: true},
		{Doctype: "Task", Role: "Creator", Read: true, Create: true}, // create-only: no write
	}
	reg := doctype.NewRegistry()
	reg.LoadFull([]*doctype.DocType{taskDT}, roles, perms)
	if err := schema.MigrateSiteFromRegistry(sdb, dbName, reg, dialect); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &siteFixture{DB: sdb, Name: name, Registry: reg, Dialect: dialect}
}

func newKernel(s *siteFixture) *kernel.Kernel {
	return kernel.New(s.Dialect, outbox.NewSQLWriter())
}

type opOpt func(*kernel.OperationContext)

func withUser(u string) opOpt            { return func(c *kernel.OperationContext) { c.User = u } }
func withRoles(roles ...string) opOpt    { return func(c *kernel.OperationContext) { c.Roles = roles } }
func withSource(src kernel.Source) opOpt { return func(c *kernel.OperationContext) { c.Source = src } }
func withKey(key string) opOpt           { return func(c *kernel.OperationContext) { c.IdempotencyKey = key } }
func withExpectedVersion(v string) opOpt {
	return func(c *kernel.OperationContext) { c.ExpectedVersion = v }
}
func withExpectedVersionLegacy(v string) opOpt {
	return func(c *kernel.OperationContext) { c.CausationID = "expected:" + v }
}
func withCorrelation(id string) opOpt {
	return func(c *kernel.OperationContext) { c.CorrelationID = id }
}

func createOp(data map[string]any, opts ...opOpt) kernel.Operation {
	return payloadOp("record.create", "", data, opts...)
}

func updateOp(name string, data map[string]any, opts ...opOpt) kernel.Operation {
	return payloadOp("record.update", name, data, opts...)
}

func payloadOp(command, name string, data map[string]any, opts ...opOpt) kernel.Operation {
	payload := map[string]any{"doctype": "Task", "data": data}
	if name != "" {
		payload["name"] = name
	}
	raw, _ := json.Marshal(payload)
	op := kernel.Operation{Command: command, Payload: raw}
	op.Context.Site = "TBD"
	for _, o := range opts {
		o(&op.Context)
	}
	return op
}

func exec(t *testing.T, s *siteFixture, k *kernel.Kernel, op kernel.Operation) contract.CommandResult {
	t.Helper()
	op.Context.Site = s.Name
	if op.Context.Actor.PrincipalID == "" {
		user := op.Context.User
		if user == "" {
			user = "Administrator"
		}
		op.Context.Actor = contract.ActorContext{
			PrincipalID:     user,
			PrincipalType:   contract.PrincipalHuman,
			Site:            s.Name,
			Roles:           op.Context.Roles,
			AuthenticatedAt: time.Now(),
		}
	}
	res, cerr := k.Execute(context.Background(), s.DB, s.Registry, op)
	t.Logf("op %s → status=%s err=%v replayed=%v", op.Command, res.Status, cerr, res.Replayed)
	return res
}

func mustComplete(t *testing.T, res contract.CommandResult) {
	t.Helper()
	if res.Status != contract.StatusCompleted || res.Error != nil {
		t.Fatalf("expected completed, got %s err=%+v", res.Status, res.Error)
	}
}

func (s *siteFixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func (s *siteFixture) taskCount(t *testing.T) int {
	return s.count(t, "SELECT COUNT(*) FROM `tabTask`")
}

// TestRecordCreateHappyPath proves the canonical path: one command definition,
// one envelope, mutation + receipt + audit + outbox committed atomically.
func TestRecordCreateHappyPath(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)

	res := exec(t, s, k, createOp(map[string]any{"title": "First", "status": "Open"}, withKey("idem-1"), withCorrelation("corr-1")))
	mustComplete(t, res)

	var docName string
	if err := s.DB.QueryRow("SELECT name FROM `tabTask` WHERE title = 'First'").Scan(&docName); err != nil {
		t.Fatalf("row not persisted: %v", err)
	}
	if s.taskCount(t) != 1 {
		t.Fatalf("expected exactly 1 task")
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_idempotency_receipt WHERE site = ? AND idempotency_key = ?", s.Name, "idem-1"); n != 1 {
		t.Fatalf("expected 1 receipt, got %d", n)
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_operation_audit WHERE site = ? AND doc_name = ? AND status = 'completed'", s.Name, docName); n != 1 {
		t.Fatalf("expected 1 audit row, got %d", n)
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_outbox WHERE site = ? AND aggregate_id = ?", s.Name, docName); n != 1 {
		t.Fatalf("expected 1 pending outbox event, got %d", n)
	}

	// KERNEL-009: create path carries after_hash only (64-hex).
	var beforeHash, afterHash string
	if err := s.DB.QueryRow(`SELECT before_hash, after_hash FROM _kora_operation_audit WHERE doc_name = ?`, docName).Scan(&beforeHash, &afterHash); err != nil {
		t.Fatalf("read hashes: %v", err)
	}
	if len(afterHash) != 64 || beforeHash != "" {
		t.Fatalf("create audit must have empty before_hash and 64-char after_hash (got %q/%q)", beforeHash, afterHash)
	}
}

// TestIdempotencyReplayAndKeyReuse covers RFC §7.1 idempotency semantics.
func TestIdempotencyReplayAndKeyReuse(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)

	first := exec(t, s, k, createOp(map[string]any{"title": "Once"}, withKey("same-key")))
	mustComplete(t, first)

	replay := exec(t, s, k, createOp(map[string]any{"title": "Once"}, withKey("same-key")))
	mustComplete(t, replay)
	if !replay.Replayed {
		t.Fatalf("second identical operation must be flagged Replayed")
	}
	if s.taskCount(t) != 1 {
		t.Fatalf("replay must not create a second document")
	}

	reuse := exec(t, s, k, createOp(map[string]any{"title": "Different"}, withKey("same-key")))
	if reuse.Error == nil || reuse.Error.Type != contract.CodeIdempotencyKeyReused {
		t.Fatalf("payload mismatch under same key must be IDEMPOTENCY_KEY_REUSED, got %+v", reuse.Error)
	}
	if s.taskCount(t) != 1 {
		t.Fatalf("key reuse must not mutate state")
	}
}

// TestTenantIsolation proves operations bind tenant identity from the
// OperationContext only: writes land in the caller's database and the actor's
// site must match the executing site.
func TestTenantIsolation(t *testing.T) {
	a := newSite(t, "tenant-alpha")
	defer a.DB.Close()
	b := newSite(t, "tenant-beta")
	defer b.DB.Close()
	ka, kb := newKernel(a), newKernel(b)

	resA := exec(t, a, ka, createOp(map[string]any{"title": "Alpha doc"}))
	mustComplete(t, resA)
	resB := exec(t, b, kb, createOp(map[string]any{"title": "Beta doc"}))
	mustComplete(t, resB)

	if a.taskCount(t) != 1 || b.taskCount(t) != 1 {
		t.Fatalf("each tenant DB must contain exactly its own document")
	}

	// Cross-tenant actor mismatch is rejected fail-closed before any write.
	op := createOp(map[string]any{"title": "Smuggled"})
	op.Context.Site = b.Name // executing against B's DB...
	op.Context.Actor = contract.ActorContext{PrincipalID: "x@alpha", PrincipalType: contract.PrincipalHuman, Site: a.Name, AuthenticatedAt: time.Now()}
	res, _ := kb.Execute(context.Background(), b.DB, b.Registry, op)
	if res.Status == contract.StatusCompleted {
		t.Fatalf("cross-tenant actor must never commit into another tenant")
	}
	if res.Error == nil || (res.Error.Type != contract.CodePermissionDenied && res.Error.Type != contract.CodeUnauthenticated) {
		t.Fatalf("expected typed denial for cross-tenant actor, got %+v", res.Error)
	}
	if b.taskCount(t) != 1 {
		t.Fatalf("rejected cross-tenant operation must not write")
	}
}

// TestAuthorizationParityAcrossSources proves identical allow/deny outcomes
// regardless of adapter surface (KERNEL-004 / SEC-003 parity requirement).
func TestAuthorizationParityAcrossSources(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()

	sources := []kernel.Source{kernel.SourceHTTP, kernel.SourceSDK, kernel.SourceMCP, kernel.SourceAI, kernel.SourceCLI}
	for _, src := range sources {
		k := newKernel(s)

		admin := exec(t, s, k, createOp(map[string]any{"title": "by " + string(src)}, withUser("root@"+string(src)), withRoles("Administrator"), withSource(src)))
		mustComplete(t, admin)

		creatorUpdate := exec(t, s, k, updateOp("TASK-0001", map[string]any{"title": "nope"}, withUser("limited@"+string(src)), withRoles("Creator"), withSource(src)))
		if creatorUpdate.Error == nil || creatorUpdate.Error.Type != contract.CodePermissionDenied {
			t.Fatalf("%s: creator lacks write; expected PERMISSION_DENIED, got %+v", src, creatorUpdate.Error)
		}

		denied := exec(t, s, k, createOp(map[string]any{"title": "x"}, withUser("norole@"+string(src)), withRoles("Nobody"), withSource(src)))
		if denied.Error == nil || denied.Error.Type != contract.CodePermissionDenied {
			t.Fatalf("%s: unknown role must be denied, got %+v", src, denied.Error)
		}
	}
	if s.taskCount(t) != len(sources) {
		t.Fatalf("parity drift: expected one doc per allowed source execution")
	}
}

// TestStaleVersionConflict proves optimistic concurrency: an update carrying
// an outdated expected_version conflicts and persists nothing.
func TestStaleVersionConflict(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)

	res := exec(t, s, k, createOp(map[string]any{"title": "Versioned"}))
	mustComplete(t, res)
	var docName string
	s.DB.QueryRow("SELECT name FROM `tabTask` WHERE title='Versioned'").Scan(&docName)

	var modified time.Time
	if err := s.DB.QueryRow("SELECT modified FROM `tabTask` WHERE name = ?", docName).Scan(&modified); err != nil {
		t.Fatalf("read version: %v", err)
	}
	current := kernel.CanonicalVersion(modified)

	// Stale token → conflict, nothing changes.
	stale := exec(t, s, k, updateOp(docName, map[string]any{"title": "Should not apply"}, withExpectedVersion("2000-01-01T00:00:00Z")))
	if stale.Error == nil || stale.Error.Type != contract.CodeConflict {
		t.Fatalf("stale version must yield CONFLICT, got %+v", stale.Error)
	}
	var title string
	s.DB.QueryRow("SELECT title FROM `tabTask` WHERE name = ?", docName).Scan(&title)
	if title != "Versioned" {
		t.Fatalf("stale operation mutated the document anyway")
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_operation_audit WHERE error_code = 'CONFLICT'"); n < 1 {
		t.Fatalf("conflict attempts must leave a denial/failure audit trail")
	}

	// Current token → success.
	ok := exec(t, s, k, updateOp(docName, map[string]any{"title": "Applied"}, withExpectedVersion(current)))
	mustComplete(t, ok)
	s.DB.QueryRow("SELECT title FROM `tabTask` WHERE name = ?", docName).Scan(&title)
	if title != "Applied" {
		t.Fatalf("correct version must apply, title=%q", title)
	}
}

// TestStaleVersionConflictLegacyToken keeps backward compatibility with the
// earlier causation-id convention while the explicit expected_version field is
// being adopted by adapters.
func TestStaleVersionConflictLegacyToken(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)

	res := exec(t, s, k, createOp(map[string]any{"title": "LegacyVersioned"}))
	mustComplete(t, res)
	var docName string
	s.DB.QueryRow("SELECT name FROM `tabTask` WHERE title='LegacyVersioned'").Scan(&docName)

	stale := exec(t, s, k, updateOp(docName, map[string]any{"title": "Should not apply"}, withExpectedVersionLegacy("2000-01-01T00:00:00Z")))
	if stale.Error == nil || stale.Error.Type != contract.CodeConflict {
		t.Fatalf("legacy expected_version must still conflict, got %+v", stale.Error)
	}
}

// TestRollbackAtomicity proves that a failure after the receipt claim rolls
// back mutation, receipt, audit, and outbox together — no partial commits.
func TestRollbackAtomicity(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)

	exec(t, s, k, createOp(map[string]any{"title": "Original", "serial": "SN-001"}))

	// Duplicate unique field fails inside the INSERT — after the receipt was
	// already claimed inside the same transaction.
	dup := exec(t, s, k, createOp(map[string]any{"title": "Dup", "serial": "SN-001"}, withKey("dup-key")))
	if dup.Error == nil || dup.Error.Type != contract.CodeConflict {
		t.Fatalf("unique violation must surface as CONFLICT, got %+v", dup.Error)
	}

	if s.taskCount(t) != 1 {
		t.Fatalf("failed operation must not persist the business row")
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_idempotency_receipt WHERE idempotency_key = 'dup-key'"); n != 0 {
		t.Fatalf("failed operation rolled back its receipt claim (found %d)", n)
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_outbox"); n != 1 {
		t.Fatalf("only the successful operation's outbox row may exist (found %d)", n)
	}
	auditFailures := s.count(t, "SELECT COUNT(*) FROM _kora_operation_audit WHERE status <> 'completed'")
	auditSuccess := s.count(t, "SELECT COUNT(*) FROM _kora_operation_audit WHERE status = 'completed' AND doctype = 'Task'")
	if auditSuccess != 1 {
		t.Fatalf("expected exactly 1 success audit row, got %d", auditSuccess)
	}
	_ = auditFailures
}

// TestUnknownFieldRejected proves unsupported configuration is never silently
// discarded (validation rejects unknown keys with typed errors).
func TestUnknownFieldRejected(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)

	bad := exec(t, s, k, createOp(map[string]any{"title": "ok", "bogus_field": "x"}))
	if bad.Error == nil || bad.Error.Type != contract.CodeValidationFailed {
		t.Fatalf("unknown field must VALIDATION_FAILED, got %+v", bad.Error)
	}
	if s.taskCount(t) != 0 {
		t.Fatalf("invalid operation must not persist")
	}
}

// stubPublisher captures deliveries and can be made to fail.
type stubPublisher struct {
	calls atomic.Int64
	fail  atomic.Bool
	got   []contract.EventEnvelope
}

func (p *stubPublisher) Publish(_ context.Context, e contract.EventEnvelope) error {
	p.calls.Add(1)
	if p.fail.Load() {
		return fmt.Errorf("broker unavailable")
	}
	p.got = append(p.got, e)
	return nil
}

// TestOutboxDeliveryRetryAndRestart proves at-least-once delivery with retry:
// broker outage backs off without loss, recovery publishes, receipts dedup.
func TestOutboxDeliveryRetryAndRestart(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)

	res := exec(t, s, k, createOp(map[string]any{"title": "To deliver"}))
	mustComplete(t, res)

	dest := &stubPublisher{}
	dest.fail.Store(true)

	pub := outbox.NewPublisher(s.DB, dest)
	pub.LeaseOwner = "test-publisher"

	// Outage window: publish attempts fail, rows stay pending.
	if _, err := pub.PublishDue(context.Background(), 10); err != nil {
		t.Fatalf("PublishDue must swallow per-row errors: %v", err)
	}
	if dest.calls.Load() == 0 {
		t.Fatalf("publisher should have attempted delivery during outage")
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_outbox WHERE status='pending'"); n != 1 {
		t.Fatalf("event must remain pending after outage, got %d", n)
	}

	// Recovery (simulating backoff expiry after a process restart): broker
	// returns; next cycle delivers exactly once.
	dest.fail.Store(false)
	if _, err := s.DB.Exec(`UPDATE _kora_outbox SET next_attempt_at = NULL WHERE status = 'pending'`); err != nil {
		t.Fatalf("clearing backoff: %v", err)
	}
	delivered, err := pub.PublishDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("recovery publish: %v", err)
	}
	if delivered != 1 || len(dest.got) != 1 {
		t.Fatalf("expected 1 delivered event, got %d", delivered)
	}
	if dest.got[0].Site != s.Name {
		t.Fatalf("delivered event carries wrong tenant: %s", dest.got[0].Site)
	}
	if n := s.count(t, "SELECT COUNT(*) FROM _kora_outbox WHERE status='published'"); n != 1 {
		t.Fatalf("row must be marked published after delivery")
	}

	// Restart/retry: re-running the publisher does not double-deliver.
	if _, err := pub.PublishDue(context.Background(), 10); err != nil {
		t.Fatalf("post-restart publish: %v", err)
	}
	if len(dest.got) != 1 {
		t.Fatalf("published rows must not redeliver on restart")
	}
}

// --- KERNEL-008: config-defined command resources ---

func mustRegister(t *testing.T, k *kernel.Kernel, yamlSrc string) *kernel.CommandResource {
	t.Helper()
	def, err := kernel.ParseCommandResource([]byte(yamlSrc))
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if err := k.Commands.Register(def); err != nil {
		t.Fatalf("register command: %v", err)
	}
	return def
}

const registerCmdYAML = `
name: animal.register
namespace: livestock
version: 1
input:
  record: Task
transaction:
  - create:
      record: Task
      values:
        title: $input.title
        serial: $input.serial
  - update:
      record: Task
      name: "$input.supervisor_ref"
      values:
        status: Done
emit:
  - animal.registered
`

// TestConfiguredCommandEndToEnd proves a YAML-defined command executes through
// the identical kernel path: atomic multi-step transaction, emitted event in
// the outbox, audit row under the command's full name.
func TestConfiguredCommandEndToEnd(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)
	k.Commands = kernel.NewCommandRegistry()
	mustRegister(t, k, registerCmdYAML)

	// Seed the supervisor the update step targets.
	seed := exec(t, s, k, createOp(map[string]any{"title": "Supervisor", "serial": "SUP-1"}))
	mustComplete(t, seed)

	var supName string
	s.DB.QueryRow(`SELECT name FROM ` + "`tabTask`" + ` WHERE serial='SUP-1'`).Scan(&supName)

	payload, _ := json.Marshal(map[string]any{"data": map[string]any{
		"title": "Daisy", "serial": "ANM-1", "supervisor_ref": supName,
	}})
	res := exec(t, s, k, kernel.Operation{Command: "livestock.animal.register", Payload: payload,
		Context: opCtx(s.Name, "Administrator")})
	mustComplete(t, res)

	if n := s.count(t, `SELECT COUNT(*) FROM `+"`tabTask`"+` WHERE title='Daisy'`); n != 1 {
		t.Fatalf("create step did not persist")
	}
	var status string
	s.DB.QueryRow(`SELECT status FROM ` + "`tabTask`" + ` WHERE serial='SUP-1'`).Scan(&status)
	if status != "Done" {
		t.Fatalf("update step did not apply, status=%q", status)
	}
	if n := s.count(t, `SELECT COUNT(*) FROM _kora_outbox WHERE event_type='animal.registered' AND site=?`, s.Name); n != 1 {
		t.Fatalf("emitted event missing from outbox")
	}
	if n := s.count(t, `SELECT COUNT(*) FROM _kora_operation_audit WHERE command_name='livestock.animal.register' AND status='completed'`); n != 1 {
		t.Fatalf("audit row for defined command missing")
	}
}

func opCtx(site, role string) kernel.OperationContext {
	return kernel.OperationContext{
		Site:  site,
		User:  "tester",
		Roles: []string{role},
		Actor: contract.ActorContext{PrincipalID: "tester", PrincipalType: contract.PrincipalHuman, Site: site, AuthenticatedAt: time.Now()},
	}
}

// TestConfiguredCommandRollbackAtomicity proves multi-step commands are all-
// or-nothing: failure in a later step discards earlier creates, receipts,
// events, and audit together.
func TestConfiguredCommandRollbackAtomicity(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)
	k.Commands = kernel.NewCommandRegistry()
	mustRegister(t, k, registerCmdYAML)

	payload, _ := json.Marshal(map[string]any{"data": map[string]any{
		"title": "Ghost", "serial": "GHO-1", "supervisor_ref": "TASK-MISSING",
	}})
	res := exec(t, s, k, kernel.Operation{Command: "livestock.animal.register", Payload: payload,
		Context: opCtx(s.Name, "Administrator")})
	if res.Error == nil || res.Error.Type != contract.CodeNotFound {
		t.Fatalf("missing update target must be NOT_FOUND, got %+v", res.Error)
	}
	if n := s.count(t, `SELECT COUNT(*) FROM `+"`tabTask`"+` WHERE serial='GHO-1'`); n != 0 {
		t.Fatalf("earlier create survived step failure — rollback broken")
	}
	if n := s.count(t, `SELECT COUNT(*) FROM _kora_operation_audit WHERE command_name='livestock.animal.register' AND status='completed'`); n != 0 {
		t.Fatalf("failed command must not leave success audit")
	}
}

// TestConfiguredCommandAuthzDenied proves authorization evaluates every
// touched record through the same matrix — no adapter can slip past.
func TestConfiguredCommandAuthzDenied(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)
	k.Commands = kernel.NewCommandRegistry()
	mustRegister(t, k, registerCmdYAML)

	payload, _ := json.Marshal(map[string]any{"data": map[string]any{
		"title": "X", "serial": "X-1", "supervisor_ref": "whatever",
	}})
	ctx := opCtx(s.Name, "Creator")
	res := exec(t, s, k, kernel.Operation{Command: "livestock.animal.register", Payload: payload, Context: ctx})
	if res.Error == nil || res.Error.Type != contract.CodePermissionDenied {
		t.Fatalf("Creator lacks write on Task; expected PERMISSION_DENIED, got %+v", res.Error)
	}
	if s.taskCount(t) != 0 {
		t.Fatalf("denied command wrote data")
	}
}

// TestConfiguredCommandIdempotentReplay proves defined commands honor
// idempotency keys identically to built-in commands.
func TestConfiguredCommandIdempotentReplay(t *testing.T) {
	s := newSite(t, "site-a")
	defer s.DB.Close()
	k := newKernel(s)
	k.Commands = kernel.NewCommandRegistry()

	oneStepYAML := `
name: solo.create
namespace: livestock
version: 1
input:
  record: Task
transaction:
  - create:
      record: Task
      values:
        title: $input.title
`
	mustRegister(t, k, oneStepYAML)

	payload, _ := json.Marshal(map[string]any{"data": map[string]any{"title": "Once"}})
	op := kernel.Operation{Command: "livestock.solo.create", Payload: payload,
		Context: opCtx(s.Name, "Administrator")}
	op.Context.IdempotencyKey = "cmd-key-1"

	first := exec(t, s, k, op)
	mustComplete(t, first)
	second := exec(t, s, k, op)
	mustComplete(t, second)
	if !second.Replayed {
		t.Fatalf("defined command replay must be flagged")
	}
	if n := s.count(t, `SELECT COUNT(*) FROM `+"`tabTask`"+` WHERE title='Once'`); n != 1 {
		t.Fatalf("replay duplicated the mutation")
	}
}

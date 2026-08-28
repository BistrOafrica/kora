# RFC: Kora Engine — NATS-Native, Offline-Capable Application Runtime

**Status:** Proposed

**Audience:** Kora core maintainers, Cloud engineers, UI engineers, extension authors

**Scope:** Kora OSS runtime, Kora Cloud, NATS/JetStream integration, actor-based workflows, offline synchronization, and the next frontend architecture.

**Implementation status:** Proposed; architecture direction is approved for implementation, but a capability is not production-supported until its phase gate and acceptance tests pass.

**Status vocabulary:** `planned` means specified but not implemented; `experimental` means implemented behind an explicit feature flag and not covered by a compatibility promise; `supported` means the contract, recovery, security, and observability tests pass in the stated deployment mode; `retired` means no new callers may depend on it.

**Handoff rule:** An implementation agent must start with §19–§21, implement contracts in dependency order, update capability status, and attach test evidence before marking a phase complete. This RFC is the source of architectural intent; code and generated schemas are the source of exact wire compatibility once a contract is frozen.

> **Implementation risk status:** The risks in §18.1 are part of this specification. A capability must remain `planned` or `experimental` while a blocking risk is unresolved. Security risks that affect tenant isolation, authorization, identity, credential scope, or durable business effects are release blockers; they may not be accepted as operational warnings only.

> **Highlighted missing work for the current implementation run:** the AI runtime still needs a durable internal task tracker that is updated after every round, a dynamic follow-up queue that adds newly discovered edge cases on each pass, and a resumption path that can continue unfinished work without requiring a fresh user prompt. This is the work that should be completed next before the AI plane can be considered restart-safe and self-driving.

## 1. Executive decision

Kora will become a relational business application runtime with a NATS-based execution fabric.

The boundary is:

```text
SQL                    authoritative business state
NATS Core              low-latency service request/reply and realtime signals
NATS JetStream         durable commands, events, tasks, and replay
NATS KV                runtime/config/lease state and watches
NATS Object Store      large immutable artifacts and manifests
Actors                 active, long-running processes only
Kora UI                versioned, server-driven page composition
Kora Cloud             managed infrastructure and fleet control plane
```

NATS is not the replacement for the document database. Kora documents, accounting entries, stock ledgers, permissions, and other transactional records remain in SQL. NATS owns execution, propagation, coordination, replay, and synchronization.

The OSS distribution must work without NATS. It will provide a local provider using the existing in-process/WAL implementation. Self-hosted NATS JetStream is the first production distributed provider and is operated by the customer or deployment operator. Kora Cloud must work with customer/operator-hosted NATS; a future managed NATS offering is an optional infrastructure product, not an engine requirement.

## 2. Why this change

The current runtime already has useful seams:

- [`analytics.EventBus`](analytics/event.go) receives document change events.
- [`analytics.channelBus`](analytics/bus.go) provides an in-process channel with WAL durability.
- [`analytics.MultiBus`](analytics/multibus.go) fans events out to webhooks.
- [`orm.TxManager`](orm/query.go) emits events after successful transactions.
- Async hooks currently use a bounded process-local channel in [`orm/hooks.go`](orm/hooks.go).
- The UI has dynamic page routes at `ui/src/routes/workspace/pages/$viewName.tsx`, but the page model is not yet a general composable application surface.

These are good foundations, but the current design has four limitations:

1. Async work can be dropped when a process-local queue is full.
2. The event interface contains local WAL operations that do not belong in a distributed provider contract.
3. Analytics, webhooks, hooks, and future integrations cannot independently scale or replay from a shared durable log.
4. Pages and views are partly configuration-driven but still require route/component code for new page types.

This RFC defines the target architecture and the migration path.

## 3. Product outcomes

The architecture is successful only if it produces visible product improvements:

- A user operation is accepted and not lost when a worker restarts.
- After-save work does not block the request path.
- Analytics, webhooks, notifications, search, and integrations can be independently scaled.
- Long-running approvals and external operations survive restarts.
- Branches and devices can continue approved operations offline and synchronize later.
- A new vertical module can be built from doctypes, commands, events, workflows, projections, and page manifests without modifying the Kora core.
- The same application package runs in embedded OSS mode and managed Kora Cloud mode.
- A page can be assembled from registered components and data queries without shipping a new frontend route.
- A non-technical administrator can assemble a useful page from safe defaults, a guided component palette, and schema-aware bindings without writing code.
- The visual builder, draft preview, and published page use the same manifest, component registry, layout rules, design tokens, and responsive behavior; the builder never invents a separate approximation of the final screen.
- Live pages show authoritative updates, operation progress, reconnect/degraded state, and conflict/reconciliation state without requiring a manual refresh.

## 4. Non-goals

This RFC does not:

- replace SQL with NATS;
- make every document an actor;
- promise exactly-once business effects;
- expose raw NATS credentials to browsers;
- make Kora Cloud required for OSS deployments;
- define every ERP module;
- define a generic microservice framework for arbitrary applications.

Kora will provide durable at-least-once delivery plus idempotent business processing. “Exactly once” may be presented only for a specific operation after an idempotency key and database uniqueness rule make the effect safe.

## 5. Target topology

### 5.1 Embedded OSS

```text
Kora process
  ├── HTTP/API + MCP
  ├── UI asset server or standalone UI
  ├── SQL connection
  ├── local command handlers
  ├── local workers
  └── LocalProvider (channel + WAL)
```

This is the default developer and small-installation mode.

### 5.2 Distributed OSS

```text
Kora API × N ──────────────┐
Kora workers × N ──────────┼── NATS JetStream ─── SQL
Kora UI/realtime gateway ──┘          │
                              KV / Object Store
```

Operators can run NATS themselves and configure `KORA_EVENT_PROVIDER=nats`.

### 5.3 Self-hosted NATS production mode

This is the primary distributed deployment model. The customer or operator owns the NATS cluster, JetStream storage, backups, TLS, upgrades, accounts, and operational SLOs. Kora supplies an idempotent bootstrap/validation command and provider configuration; it does not assume access to a Kora-managed NATS control plane.

```text
Customer/operator infrastructure
  ├── NATS cluster + JetStream
  ├── NATS accounts, users, TLS, stream/KV/Object Store policy
  ├── SQL database
  ├── Kora API/workers
  └── backup, monitoring, restore and upgrade procedures
```

The self-hosted operator must provide:

- NATS server URL(s), TLS CA/client credentials, account/user credentials, and JetStream storage;
- stream, consumer, KV, and Object Store capacity and retention policy;
- NATS account permissions that isolate sites/environments where multiple sites share a cluster;
- backups and restore verification for JetStream streams, KV buckets, and Object Store data;
- clock synchronization, network policy, failure monitoring, and upgrade/drain procedures;
- declared RPO/RTO and maximum tolerated event/task lag.

Kora must validate connectivity, JetStream availability, required stream/KV/Object Store configuration, subject permissions, server compatibility, and read/write/restore health before marking the NATS provider `supported`. Kora must never silently fall back from NATS to LocalProvider after production startup; fallback is an explicit operator-selected mode because it changes durability and delivery semantics.

### 5.4 Kora Cloud

```text
                       Kora Cloud control plane
          billing · orgs · provisioning · deploy · secrets
                         · placement · metering
                                  │
              ┌───────────────────┴───────────────────┐
              │                                       │
       Kora data plane                         Operator NATS
       API/workers/UI gateway                  JetStream/KV
              │                                       │
              └──────────── tenant SQL + object store ┘
```

The Cloud control plane manages customer/account state, provisioning, routing, billing, channels, delegated identity, usage projections, and runtime lifecycle. The default data plane connects to a customer- or operator-hosted NATS JetStream deployment. Kora application packages, doctypes, workflows, scripts, page manifests, and extension contracts remain portable. If Kora later offers managed NATS, it is another `NATSDeploymentProvider` implementation behind the same contract.

### 5.5 Edge/offline topology

```text
Branch Kora + local SQL + local NATS leaf
       │ local operations continue during outage
       └──────── intermittent leaf connection ────────┐
                                                       ▼
                         Cloud Kora + central NATS + central SQL
```

NATS leaf nodes are appropriate for this topology because they route local traffic locally and route permitted subjects to a remote cluster when connected. They are not a substitute for the synchronization protocol or conflict policy.

## 6. Component groups

The repository and eventual services should be organized into these logical groups. A group may be one Go package in OSS or several deployable processes in Cloud.

### 6.1 Kernel group

Owns stable contracts and synchronous business execution:

- document model and ORM;
- SQL dialects and migrations;
- permissions and authorization;
- hooks and validation;
- command validation;
- transaction/outbox writing;
- application package loading;
- schema and contract version checks.

The kernel must not import a concrete NATS client.

### 6.2 Runtime fabric group

Owns provider-neutral execution interfaces and provider implementations:

- `LocalProvider`;
- `NATSProvider`;
- command bus;
- event publisher;
- task queue;
- consumer lifecycle;
- idempotency and delivery metadata;
- timers and retry policy.

### 6.3 Worker group

Each worker has a durable consumer identity and can be run embedded or separately:

- analytics projection worker;
- webhook delivery worker;
- async script worker;
- notification worker;
- search/index worker;
- integration worker;
- file/artifact worker;
- workflow actor host;
- AI agent-run actor/worker;
- AI provider adapter and routing worker;
- AI usage, budget, and cost reconciliation worker;
- sync worker.

### 6.4 Application package group

Contains business-level artifacts:

- doctypes;
- permissions;
- workflows;
- scripts;
- commands;
- events;
- projections;
- page manifests;
- component bindings;
- fixtures and migrations.

This is the unit that customers and extension authors should build.

### 6.5 Delivery group

Contains HTTP, MCP, SDK, websocket/SSE, and standalone frontend adapters. These adapters call the same command/query contracts; they do not contain business rules that are absent from the kernel.

### 6.6 Cloud group

Contains organization, billing, deployment, provisioning, fleet, usage, and managed infrastructure services. It must not become the only place where application semantics exist.

The Cloud group also owns the integration boundary to self-hosted NATS: deployment registration, credential references, stream/KV/Object Store bootstrap, health checks, capacity metadata, backup/restore evidence, and operator-facing diagnostics. It does not own NATS message semantics or duplicate engine authorization.

## 7. Provider contracts

The current `analytics.EventBus` should be split. Local WAL rotation is an implementation detail, not a required capability for every backend.

```go
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

type CommandEnvelope struct {
    ID            string          `json:"id"`
    Type          string          `json:"type"`
    Version       int             `json:"version"`
    Site          string          `json:"site"`
    Actor         ActorContext    `json:"actor"`
    CorrelationID string          `json:"correlation_id,omitempty"`
    CausationID   string          `json:"causation_id,omitempty"`
    IdempotencyKey string         `json:"idempotency_key,omitempty"`
    Deadline      time.Time       `json:"deadline"`
    Data          json.RawMessage `json:"data"`
}

type EventPublisher interface {
    Publish(ctx context.Context, event EventEnvelope) error
}

type CommandBus interface {
    Request(ctx context.Context, command CommandEnvelope) (CommandResult, error)
    Submit(ctx context.Context, command CommandEnvelope) (TaskReceipt, error)
}

type Consumer interface {
    Run(ctx context.Context, handler Handler) error
    Drain(ctx context.Context) error
}

type Handler func(context.Context, Delivery) error
```

Required semantics:

- `Request` is synchronous request/reply and has a deadline.
- `Submit` returns an operation ID and does not imply completion.
- A consumer acknowledges only after the handler commits its effect.
- Handler retries are safe because every business effect has an idempotency key.
- A poison message moves to a dead-letter subject after policy-defined attempts.
- Every envelope is JSON or Protobuf-compatible and includes a schema version.
- `Deadline` is mandatory for bounded requests and is propagated without being extended by downstream services. A missing or expired deadline returns `DEADLINE_EXCEEDED` before business execution.
- `ActorContext` is defined in §7.3. `CommandResult`, `TaskReceipt`, `Delivery`, and structured errors are versioned contracts, not implementation-private structs.

The old `DrainWAL`, `RotateWAL`, and `CommitWALRotation` methods move behind `LocalProvider` and are not exposed on `EventPublisher`.

### 7.1 Command and query protocol

All externally callable operations use a versioned command or query contract. HTTP, MCP, SDK, UI, and internal NATS callers use the same logical contract.

```json
{
  "type": "document.submit",
  "version": 1,
  "id": "01J...",
  "site": "acme.example.com",
  "actor": {"user_id": "usr-1", "roles": ["Accounts User"], "device_id": "pos-7"},
  "correlation_id": "req-123",
  "idempotency_key": "client-op-456",
  "deadline": "2026-08-12T10:00:05Z",
  "data": {"doctype": "Sales Invoice", "name": "SINV-0001"}
}
```

The response is always one of these states: `completed`, `accepted`, `rejected`, `conflict`, or `failed`. Every response contains `operation_id`, `correlation_id`, `status`, and a structured error when applicable. Error codes are stable machine-readable values such as `PERMISSION_DENIED`, `VALIDATION_FAILED`, `NOT_FOUND`, `CONFLICT`, `DEADLINE_EXCEEDED`, and `DEPENDENCY_UNAVAILABLE`.

`Request` has a maximum deadline and never waits for unrelated projections. `Submit` persists the command before returning `accepted`. The backend, not the frontend, decides whether an operation is authorized and whether it may run offline.

Command transport is a deployment concern, not an authorization boundary. In-process command handling is the default: the API process validates and re-authorizes the command, executes the kernel transaction, and writes the outbox directly. NATS request/reply is reserved for cross-process dispatch, Cloud control-plane operations, sync gateway intake, and MCP or agent calls requiring service discovery. NATS commands carry the same actor, site, correlation, deadline, and idempotency context, and the kernel re-authorizes them. NATS identity alone never grants business authorization.

### 7.2 Ordering and delivery contract

Kora guarantees ordering only within an aggregate key:

```text
ordering_key = site + aggregate_type + aggregate_id
```

There is no global ordering guarantee across tenants, documents, subjects, or consumers. Producers set the ordering key on commands/events. Consumers that require order use a single actor/consumer lane per key and reject an event whose `expected_version` is not the next version.

Delivery is at-least-once. Duplicate delivery is expected and must be safe through event IDs, idempotency receipts, aggregate versions, and domain uniqueness constraints.

### 7.3 Identity, delegation, and operation identity

Every request has an authenticated principal. The principal is either a human actor or an explicitly provisioned service principal; missing identity fails closed in production paths. A service principal is scoped to organization, environment, site, allowed commands/queries, channels, data classifications, expiry, and purpose.

```go
type ActorContext struct {
    PrincipalID       string   `json:"principal_id"`
    PrincipalType     string   `json:"principal_type"` // human|service|agent
    SubjectUserID     string   `json:"subject_user_id,omitempty"`
    OrganizationID    string   `json:"organization_id,omitempty"`
    Site              string   `json:"site"`
    Roles             []string `json:"roles,omitempty"`
    Scopes            []string `json:"scopes,omitempty"`
    Channel           string   `json:"channel,omitempty"`
    DeviceID          string   `json:"device_id,omitempty"`
    AuthenticatedAt   time.Time `json:"authenticated_at"`
    AuthSessionID     string   `json:"auth_session_id,omitempty"`
    DelegationID      string   `json:"delegation_id,omitempty"`
}
```

`PrincipalID` identifies who is executing. `SubjectUserID` identifies the human on whose behalf a service or agent acts. Authorization uses both identities and the delegation policy. `ai-assistant`, `mcp-agent`, and similar strings are not implicit users, roles, or owners. A draft or audit record stores the initiating principal, subject user, agent identity, and delegation separately.

`operation_id` identifies one logical business operation; `correlation_id` identifies the request/run trace; `causation_id` identifies the message that caused the current message; `idempotency_key` identifies the caller's retryable intent. A retry with the same key and equivalent contract/data returns the original result. Reuse with different contract/data returns `IDEMPOTENCY_KEY_REUSED`.

## 8. Data model and durable delivery

### 8.1 Transactional outbox

Document writes must not publish directly to NATS after committing SQL. The crash window between SQL commit and publish would lose the event.

Every business transaction that emits an event writes:

```text
business tables
_kora_outbox
```

in the same SQL transaction.

Minimum outbox schema:

```text
id                  ULID primary key
site                tenant/site identifier
event_type          stable event name
event_version       integer
aggregate_type      doctype or domain aggregate
aggregate_id        document/operation identifier
payload             JSON/blob
status              pending|publishing|published|failed
attempts            integer
next_attempt_at     timestamp
created_at          timestamp
published_at        timestamp nullable
last_error          text nullable
```

The publisher claims rows with a lease, publishes to JetStream, and marks them published. If it crashes after publishing but before marking the row, the event is published again; the event ID makes the duplicate safe.

Outbox state transitions are:

```text
pending → publishing → published
pending → publishing → pending       transient failure / lease expiry
pending → publishing → failed        retry budget exhausted
failed  → pending                    operator replay
```

Each publisher claim includes `lease_owner`, `lease_until`, and `attempts`. A worker may claim only rows where `status=pending` and `next_attempt_at <= now`, or rows whose publishing lease expired. Claims use the database dialect's row-lock/skip-locked equivalent where available; the fallback is an atomic conditional update. Publishing uses exponential backoff with jitter and a bounded maximum delay. No row is deleted automatically.

The publisher sets the JetStream message ID to the outbox event ID. A duplicate publish is therefore detectable by JetStream deduplication where configured and always safe at the consumer through idempotency receipts. Outbox age, failed rows, and oldest pending event are operational metrics.

### 8.2 Idempotency

Every externally visible operation must accept or generate an idempotency key. The key is scoped to `organization + environment + site + principal + operation type`, unless a contract explicitly defines a wider scope. The kernel stores the request fingerprint, status, response envelope, operation ID, first-seen time, and expiry:

```text
_kora_idempotency_receipt
  scope
  idempotency_key
  request_fingerprint
  contract_type
  contract_version
  status                 pending|completed|failed
  response_envelope
  operation_id
  first_seen_at
  expires_at
  completed_at
```

Concurrent requests with the same key serialize on one receipt. A retry while `pending` returns the existing operation state or waits only within its deadline; it does not execute a second effect. A retry after expiry is a new operation and must use a new key. Consumers also persist a unique `(consumer_name, event_id)` receipt for event delivery. For financial, stock, payment, message-send, and external-provider operations, use domain-specific immutable ledgers in addition to generic receipts.

### 8.3 Event envelope example

```json
{
  "id": "01J...",
  "type": "kora.document.sales_invoice.submitted",
  "version": 1,
  "source": "kora.kernel",
  "site": "acme.example.com",
  "aggregate_type": "Sales Invoice",
  "aggregate_id": "SINV-0001",
  "occurred_at": "2026-08-12T10:00:00Z",
  "correlation_id": "req-123",
  "data": {
    "projection": "changed_fields",
    "changed_fields": ["status"],
    "document": null,
    "old_document": null
  }
}
```

Event payloads are minimized by default. The kernel classifies fields and emits a consumer-approved projection: `metadata`, `changed_fields`, `summary`, or `full_document`. Full documents and old-document snapshots are restricted to explicitly authorized audit or rebuild consumers and are never included in generic realtime, webhook, search, or analytics events by default. Projection policy, field classification, and recipient authorization are recorded with the event schema. Sensitive fields must be omitted or redacted before publication, not only in logs.

## 9. NATS design

### 9.1 Use the NATS features deliberately

| Capability | Kora use | Not used for |
|---|---|---|
| Core request/reply | service commands, queries, health, capability discovery | durable business work |
| Core pub/sub | realtime hints, ephemeral presence, low-value notifications | accounting or critical events |
| JetStream stream | durable domain events and task messages | arbitrary SQL replacement |
| Durable consumer | one independent position per worker/projection | shared global cursor |
| Work-queue consumer | competing workers for one task type | fan-out domain events |
| KV bucket | config snapshots, leases, actor placement, feature flags, cursors | canonical invoices or stock balances |
| KV watch/history | hot reload, cache invalidation, runtime observation | audit ledger |
| Object Store | manifests, exports, large artifacts, offline bundles | small command payloads |
| Accounts/imports/exports | Cloud tenant and service isolation | application authorization by itself |
| Leaf nodes | branch/edge connectivity | conflict resolution |

JetStream streams provide persistence and replay; consumers maintain independent delivery positions and redeliver unacknowledged messages. KV supports atomic create/update and watches, but direct reads may be served by followers; use the stream leader where read-your-write semantics are required. These constraints are part of the implementation design, not assumptions. See the official [JetStream](https://docs.nats.io/concepts/jetstream), [KV](https://docs.nats.io/nats-concepts/jetstream/key-value-store), and [security](https://docs.nats.io/nats-concepts/security) documentation.

### 9.2 Streams

Initial streams:

```text
KORA_EVENTS    kora.events.>
KORA_COMMANDS  kora.commands.>
KORA_TASKS     kora.tasks.>
KORA_SYNC      kora.sync.>
KORA_AUDIT     kora.audit.>
KORA_AI        kora.commands.agent.>, kora.tasks.agent.>, kora.events.agent.>, kora.events.ai.>
```

Deployments may split streams by tenant, region, retention class, or workload once measured. Do not create one stream per document or per actor.

Domain event consumers are fan-out:

```text
KORA_EVENTS
  ├── analytics-v1
  ├── webhook-v1
  ├── audit-v1
  ├── search-v1
  └── realtime-v1
```

Task consumers are work queues:

```text
KORA_TASKS
  ├── async-hooks-v1   (competing workers)
  ├── email-v1         (competing workers)
  └── integration-v1   (competing workers)
```

### 9.3 Subject naming

Subjects are contracts, not arbitrary implementation strings:

```text
kora.events.document.<doctype-slug>.<operation>
kora.events.workflow.<workflow-slug>.<transition>
kora.commands.document.<action>
kora.commands.workflow.<action>
kora.tasks.<task-type>
kora.realtime.<opaque-site>.<channel>
kora.sync.<opaque-site>.<branch>.<direction>
kora.service.<service>.<operation>
```

Subject tokens are generated from stable opaque internal IDs, not customer names, hostnames, or mutable labels. Every site, channel, branch, direction, doctype slug, operation, and task token is validated against a strict ASCII allowlist and length limit; wildcard tokens (`*`, `>`), separators, encoded separators, empty tokens, and control characters are rejected. Client input must never be interpolated directly into a subject. The gateway authorizes each realtime subscription and sync operation against the authenticated actor, site, branch, channel, and deployment before constructing a subject. The envelope site remains authoritative for business authorization, while NATS accounts and permissions provide infrastructure isolation.

### 9.4 KV buckets

Initial buckets:

```text
KORA_CONFIG       active application/config package pointer
KORA_FLAGS        feature flags and rollout percentages
KORA_LEASES       worker/actor ownership leases
KORA_PLACEMENT    site/branch/worker placement
KORA_SYNC         last acknowledged sync cursor and device state
KORA_CAPABILITIES service versions and supported contracts
KORA_AI_RUNS      agent run checkpoints and current state
KORA_AI_LEASES    agent run ownership and fencing tokens
KORA_AI_SESSIONS  channel delivery cursors and conversation pointers
KORA_AI_POLICIES  immutable policy snapshot pointers
KORA_AI_BUDGETS   scoped budget reservations and counters
```

KV values are snapshots, not an event log. Changes that must be audited or replayed are also emitted as events and/or persisted in SQL. Configuration and policy pointers are content-addressed (`sha256` or equivalent), signed by an approved package/policy key, bound to organization/environment/site, and verified before activation. A KV value alone can never select a policy, credential, package, or authorization configuration.

AI KV values are bounded and reconstructable. They must have a maximum size, TTL, schema version, owner/fencing metadata where applicable, and per-bucket quotas/history limits. High-contention counters use scoped shards or an equivalent bounded reservation strategy. KV is never the sole source of permissions, approvals, business state, usage, cost, or audit history.

### 9.5 Object Store

Use Object Store for:

- application package bundles;
- frontend manifests and assets;
- offline data bundles;
- import/export files;
- generated PDFs and attachments;
- large integration payloads.

Messages should contain an object reference, checksum, content type, and schema version when payloads are too large. The browser accesses these through authenticated Kora HTTP endpoints, not raw NATS credentials.

### 9.6 NATS services

Expose service-style request/reply endpoints for operations that require a response:

```text
kora.service.document.get
kora.service.document.list
kora.service.document.mutate
kora.service.workflow.command
kora.service.sync.push
kora.service.sync.pull
kora.service.runtime.capabilities
kora.service.ai.provider.generate
kora.service.ai.provider.embed
kora.service.ai.provider.moderate
```

NATS service metadata must publish service name, semantic version, endpoints, and instance ID. NATS service names and versions follow the NATS services protocol; see [Building Services](https://docs.nats.io/using-nats/developer/services) and [Request-Reply](https://docs.nats.io/nats-concepts/core-nats/reqreply).

### 9.7 Self-hosted NATS bootstrap and lifecycle

Kora provides a repeatable bootstrap/validation operation for operator-hosted NATS:

```text
kora nats validate
kora nats bootstrap --site <site> --environment <environment>
kora nats status
kora nats backup-manifest
kora nats drain
```

Bootstrap is idempotent and creates or validates only the Kora resources declared by the deployment contract. It must not overwrite operator-owned streams, accounts, retention, or permissions without an explicit migration flag. The command reports `created`, `already_present`, `incompatible`, and `permission_denied` outcomes per resource.

The deployment contract records:

```text
deployment_id
operator_id
nats_server_version
cluster_id
account_id
credential_ref
tls_policy
stream_config_hash
kv_config_hash
object_store_config_hash
backup_policy
rpo
rto
last_validated_at
```

A NATS upgrade is a controlled lifecycle: validate compatibility, pause new provisioning, drain workers, preserve consumer durable names, upgrade or fail over the cluster, validate streams/KV/Object Store and permissions, resume workers, and emit a deployment health event. NATS loss must produce an explicit `DEPENDENCY_UNAVAILABLE` state; it must not be hidden as a successful local operation.

## 10. Actor model

Kora uses actors for active processes, not for every document.

### 10.1 Actor candidates

- workflow instance;
- payment/external operation;
- webhook delivery sequence;
- branch/device sync session;
- tenant runtime coordinator;
- long-running AI or import job.

### 10.2 Actor contract

```go
type ActorID struct {
    Site string
    Kind string
    Key  string
}

type ActorMessage struct {
    ID       string
    Actor    ActorID
    Type     string
    Version  int
    Data     json.RawMessage
}

type Actor interface {
    Handle(ctx context.Context, msg ActorMessage) ([]CommandEnvelope, []EventEnvelope, error)
}
```

An actor host must guarantee one active owner per actor key at a time using a KV lease or equivalent fencing token. Actor state is persisted in SQL or an explicit snapshot table. KV may hold placement and lease metadata, but business state must not exist only in memory or KV.

Actor state is not event-sourced by default. The required baseline is a SQL state row plus an append-only actor message/history table for recovery and audit. A handler receives the persisted state version and must produce a compare-and-swap update. The update succeeds only when the expected version matches.

Actor ownership uses a lease record containing `actor_id`, `owner_id`, `fencing_token`, `lease_until`, and `updated_at`. Every SQL mutation made by an actor includes the fencing token; stale owners are rejected. A worker renews leases before half the lease duration. If renewal fails, it stops accepting messages for that actor.

Actor recovery is deterministic: acquire lease, load the latest state, load unacknowledged messages in sequence order, process one message, commit state/effects/outbox in one SQL transaction, then acknowledge the NATS message. Actor mailboxes have bounded depth; overflow creates a durable backpressure event rather than silently dropping work. Durable timers are JetStream tasks containing `actor_id`, `due_at`, and `timer_id`; timer delivery is idempotent.

Durable timers require an explicit scheduler because JetStream has no native delayed delivery. Kora stores timers in SQL with at least `timer_id`, `actor_id`, `due_at`, `status`, `attempts`, `claimed_until`, and `published_at`. Timer-wheel workers claim due rows with a lease or compare-and-swap update, publish a JetStream task, and record publication. Claims and publication are retryable; delivery is deduplicated by `timer_id`. Completion occurs only after the actor commits its state, effects, and outbox transaction. Failed delivery retries and eventually enters dead letter. The implementation must define clock-skew tolerance, claim duration, retry limits, and cleanup policy.

### 10.3 Workflow flow

```text
Document command
  → SQL transaction + outbox
  → document.submitted event
  → workflow actor message
  → actor loads state/version
  → actor emits approval.requested
  → notification task
  → human command
  → actor validates expected version
  → SQL transition + outbox
```

Timers are durable tasks. A timer must be represented by an operation ID and due time, not a process-local `time.After` that disappears on restart. Scheduling, claiming, publication, redelivery, and completion are observable by `timer_id`.

### 10.4 AI and agent runtime

AI is a first-class Kora runtime capability. Chat, WhatsApp and other channels, MCP, SDKs, scheduled jobs, and internal services enter one agent runtime. Model work is routed through NATS; SQL is authoritative for durable history, audit, approvals, usage, and costs; NATS KV contains only bounded, reconstructable hot state.

#### 10.4.1 Routing and durable state

```text
HTTP / UI / WhatsApp / SMS / MCP / SDK
  → channel adapter: identity, consent, site, rate limit, dedupe
  → agent gateway: admission, policy, budget, capability snapshot
  → kora.commands.agent.run.v1 (JetStream)
  → fenced agent worker / actor host
      ├── KV: hot run state, lease, active step, checkpoint pointer
      ├── SQL: run, message, step, approval, usage, cost, audit, idempotency
      ├── kora.service.ai.provider.*: provider adapter request/reply
      └── kora.commands.*: kernel-authorized tools and queries
  → delivery worker → WhatsApp/UI/MCP response
```

The run is always durable, even when a bounded model call uses request/reply. Streaming chunks are ephemeral; final messages, tool results, usage, cost, and run transitions are durable. Initial subjects are `kora.commands.agent.run.v1`, `kora.commands.agent.cancel.v1`, `kora.tasks.agent.step.v1`, `kora.tasks.agent.timer.v1`, `kora.events.agent.run.v1`, `kora.events.agent.approval.v1`, `kora.events.ai.usage.v1`, `kora.events.ai.cost.v1`, and `kora.service.ai.provider.{generate,embed,moderate}.v1`. Consumers partition by `site + run_id`; one fenced worker advances a run at a time.

Minimum SQL entities are `_kora_ai_conversation`, `_kora_ai_run`, `_kora_ai_message`, `_kora_ai_step`, `_kora_ai_approval`, `_kora_ai_usage`, and `_kora_ai_cost`. KV buckets are `KORA_AI_RUNS`, `KORA_AI_LEASES`, `KORA_AI_SESSIONS`, `KORA_AI_POLICIES`, and `KORA_AI_BUDGETS`. KV entries are versioned, TTL-managed, compare-and-swap protected, and rebuildable from SQL/events. A stale worker is rejected by SQL fencing; no business decision depends on an uncommitted KV write.

```text
accepted → planning → waiting_provider → waiting_tool → planning
planning → pending_approval → planning
planning → completed | failed | cancelled | expired | budget_exhausted
```

`pending_approval` is a protocol state, never a prompt instruction. A `requires_confirmation` tool cannot execute until a durable, expiring, actor-bound approval authorizes the same run, step, site, record versions, tool, complete validated arguments, effective actor/delegation, provider policy, and action fingerprint. The approval presentation must render the complete evaluated action, including target records, fields, and values. Any argument, target, record-version, actor, policy, or authorization-context divergence invalidates the approval and returns to `pending_approval`. Confirmation and authorization are re-evaluated immediately before execution.

#### 10.4.2 Tool catalog and agentic interface

The versioned command/query registry is the source of truth for tool names, JSON Schema arguments, contract versions, permissions, data scope, safety level, confirmation/re-auth requirements, channel allowlist, timeout, and result size. It is published to `KORA_CAPABILITIES` with an immutable hash and included in the run policy snapshot.

```text
tool catalog → chat function schema → MCP tool definition
             → SDK types → UI action declaration → command/query descriptor
```

All adapters use one kernel path: catalog lookup, schema validation, actor/site/record authorization, policy and budget checks, approval, idempotency receipt, then `CommandEnvelope` or typed query execution. MCP is not a stub or alternate REST implementation. Named queries (`document.list`, `sales.summary`, `workflow.funnel`) are the default; free-form filters are bounded JSON-Schema ASTs and never SQL. Results contain `data`, `next_cursor`, `has_more`, `status`, and stable errors. Large results use opaque cursors or Object Store references. External messages and retrieved content are untrusted data, not instructions; provenance is retained. Channel adapters must authenticate the provider webhook/session, bind the verified identity to the server-resolved site, and re-verify that binding for each run; client-supplied user, site, or role claims never grant authority.

#### 10.4.3 Provider routing and failover

```go
type AIProvider interface {
    Generate(context.Context, GenerateRequest) (GenerateResponse, error)
    Embed(context.Context, EmbedRequest) (EmbedResponse, error)
    Moderate(context.Context, ModerateRequest) (ModerateResponse, error)
}
```

Provider profiles declare capabilities, models, context/output limits, regions, residency, retention, safety features, streaming, rate limits, and a versioned price sheet. Routing uses capability, tenant policy, residency, latency, reliability, quality tier, and budget. Fallback is allowed only between policy-compatible profiles and is recorded as a distinct attempt. Prompt templates, system instructions, tool schemas, model parameters, and routing policy are immutable versioned artifacts; hashes and the actual response model are stored on every run. Adapter errors normalize to stable codes such as `RATE_LIMITED`, `CONTEXT_TOO_LARGE`, `CONTENT_BLOCKED`, `PROVIDER_UNAVAILABLE`, and `USAGE_UNAVAILABLE`.

#### 10.4.4 Usage, cost, safety, and evaluation

Every provider attempt emits an immutable usage event, including partial or zero usage: provider/account, model, operation, request/response IDs, input/output/cached/reasoning tokens, latency, retries, status, region, and pricing-sheet version. Attribution dimensions include organization, site, application package, agent/version, conversation, run, step, channel, actor, feature, environment, provider, model, region, currency, and pricing version.

Cost is calculated from normalized usage and a versioned price sheet. Estimated cost supports admission control; invoices or provider usage exports reconcile it into correction rows. Historical cost is never overwritten. The ledger supports token, cache, reasoning, embedding, moderation, image/audio, request, and commitment charges. Billing exports are FOCUS-compatible, with provider-specific fields in namespaced extensions. Expiring compare-and-swap reservations prevent concurrent runs from overspending tenant, agent, user, or channel budgets. Reservations are admission bounds, not execution guarantees. Every run also enforces hard maximum tokens, tool calls, rounds, request/response size, provider retries, and wall-clock duration; limits are checked between provider and tool steps and terminate the run with `budget_exhausted` when reached.

Each agent/tool declares risk tier, data classification, allowed channels/regions/providers, retention, approval policy, and evaluation suite. High-impact workflows require deployment approval, stronger audit, redaction, and human review. Telemetry follows OpenTelemetry GenAI semantic conventions for provider, model, operation, conversation/run, tool, retrieval, token usage, latency, and errors. Prompt/output bodies are opt-in and redacted. Quality and safety evaluations link to agent/model/prompt versions and run IDs. This operationalizes NIST AI RMF governance, mapping, measurement, and management across the lifecycle.

AI acceptance requires one catalog and kernel authorization path for every channel; durable approvals for confirmation tools; restart-safe runs, leases, cursors, timers, and correlation IDs; auditable provider failover; pre-invocation budget enforcement; usage/cost attribution at provider, model, organization, site, application, agent, conversation, run, step, and channel levels; version/hash capture; explicit content retention; typed cursor-based queries; and at most one authorized business effect after duplicate delivery, retries, approval replay, or stale ownership.

#### 10.4.5 Current implementation reconciliation

This RFC distinguishes the current implementation from the target architecture. The following findings are known gaps, not claims that the target behavior already exists.

| Review finding | Normative RFC decision | Required implementation gate |
|---|---|---|
| Chat tool execution bypasses permissions | Authorization is performed inside the shared tool executor, per tool call, using the actor, site, operation, doctype, record, field, and channel context. No adapter may rely on the ORM or prompt for authorization. | A read-only actor attempting create/update/delete, script, view, or doctype tools receives `PERMISSION_DENIED`; the same test must pass for HTTP, chat, channel, MCP, SDK, and NATS. |
| Confirmation is prompt-only; recent auth is dead metadata | `RequiresConfirmation` and `RequiresRecentAuth` are executable policy. A confirmation-required command enters `pending_approval`; recent-auth policy verifies a server-side session/authentication event, not a client assertion. | Delete, mutation, script, view, workflow, and schema tools cannot commit without durable approval/recent-auth evidence. Prompt text is advisory only. |
| Page context, history, and tool results can inject instructions | Client context is untrusted data. Store it in typed fields, delimit it from policy, label provenance, strip control/system messages, and pass document/tool data as structured tool results. Summaries must inherit data classification and redaction policy. | Injection fixtures in pathnames, document names, history, and tool results never change tool permissions, policy, or system instructions. |
| Provider timeout configuration is not applied | Every provider call receives a context deadline derived from request/run/step policy and `HTTPTimeoutSec`; the client must also have bounded transport timeouts. | A hung provider releases the request/worker within the configured deadline and produces `DEADLINE_EXCEEDED` or `PROVIDER_UNAVAILABLE`. |
| Usage is accumulated only in memory | Persist one immutable usage record per provider attempt and publish a usage event through the outbox. Store partial/zero usage, retries, model, provider, token classes, and attribution dimensions. | Usage survives restart and is queryable by site, organization, user, model, provider, run, step, and channel; billing reconciliation creates corrections rather than overwrites. |
| No chat-specific rate/cost controls | Admit a run only after atomic reservations for tenant, site, actor, agent, channel, and run budgets. Enforce request size, history size, max rounds, max retries, tool count, concurrency, and daily/monthly spend limits. | A rejected reservation makes no provider call; concurrent budget tests cannot overspend. |
| Three tool-schema generators drift | `BuildToolCatalog` becomes the sole registry projection. Chat/OpenAI, channel, MCP, SDK, and UI adapters render it; `FieldToJSONSchema` is reused by the projection. | Catalog parity tests compare names, arguments, versions, safety, permissions, and allowlists across adapters, including delete and system tools. |
| MCP is currently validation-only/stubbed | The RFC explicitly defines two phases: validation-only MCP is not a production capability; executable MCP requires authenticated actor/site context and the shared executor. Stdio must use an explicit site credential/session configuration and must not default to a synthetic privileged user. | MCP execution tests prove real CRUD/query behavior, authorization, confirmation, audit, idempotency, and safe credential scoping before MCP is advertised as supported. |
| Chat has no server-side conversation/session | Stateless chat remains a compatibility mode with bounded history. Agentic work uses `_kora_ai_conversation` and `_kora_ai_run`, with opaque cursors, server-side compaction checkpoints, resume, cancel, and channel delivery cursors. | Closing a browser or losing WhatsApp connectivity does not lose a run; retention/deletion removes conversation content and derived summaries according to policy. |
| `_find` truncates results; `_list` ignores filters | Agent queries use named, typed query contracts with explicit limits, filters, sorting, cursors, and duplicate-detection semantics. Legacy helpers are compatibility tools and must document their cap/behavior. | Query-contract tests cover filters, pagination, duplicate detection, authorization-scoped results, and oversized responses. |
| Stall detection only compares consecutive rounds | Track a bounded rolling fingerprint of tool-call plans, arguments, results, and state versions. Detect cycles of length 1..N, repeated failed transitions, and no-progress state changes; terminate with typed `AGENT_STALLED`. | A→B→A→B, repeated equivalent calls, and changing-but-non-progressing calls terminate before budget exhaustion. |
| `isToolError` uses string prefixes | Tool results use typed envelopes with `status`, `error.code`, `retryable`, and `data`; textual prefixes are never control flow. | A document/result beginning with “Error” remains data; only structured error codes affect circuit breaking. |
| Long synchronous loop has no streaming/resume contract | Short requests may return synchronously; multi-step or deadline-risking runs return `accepted` with `operation_id` and stream typed progress via SSE/WebSocket/realtime gateway. Streaming is delivery only; SQL/NATS state is authoritative. | Client disconnect, reconnect, cancel, timeout, and resume behavior are tested. |
| Model override can mismatch provider; key precedence is implicit | Model selection is a validated `(provider, model, credential/profile)` tuple. Site policy may allow one or many profiles, but routing and billing identity are explicit; “first key wins” is forbidden. | Invalid pairs fail before network I/O; every attempt records provider, credential/account, model, region, pricing version, and routing decision. |
| Synthetic `mcp-agent` / `ai-assistant` identity is ambiguous | Remove implicit privileged identities from production paths. Every run has an authenticated actor or an explicitly configured service principal with scopes, purpose, site, expiry, and audit owner. Draft ownership records the initiating actor and agent identity separately. | `if_owner` and role checks behave consistently for human, service, and agent actors; missing identity fails closed. |
| Anthropic compatibility assumptions are undocumented | Adapters implement provider-native request/response normalization behind `AIProvider`; OpenAI-compatible endpoints are separate profiles and cannot be assumed for native providers. | Contract tests cover system instructions, tool schemas, finish reasons, streaming, usage, safety blocks, and retries per provider profile. |
| Compaction may retain sensitive data | Compaction is a versioned, policy-controlled operation. Redact or reference sensitive tool output, retain classification/provenance, and permit tenant-configured retention/deletion. | PII/secret fixtures do not survive compaction when policy forbids it; summaries remain attributable to their source run and policy version. |
| Core AI loop lacks regression tests | The AI plane requires deterministic contract tests, provider fakes, fuzzing for malformed provider payloads, security tests, property tests for idempotency/budgeting, and restart tests with a real NATS provider. | Stall/cycle detection, circuit breaking, compaction, timeout, tool-call parsing, authorization, approval, usage, and failover are covered before production status. |

The implementation status must be published separately from this target contract. A capability is `planned` until its gate passes, `experimental` while behind an explicit feature flag, and `supported` only after the relevant contract and recovery tests pass. Documentation, CLI help, and advertised MCP/Cloud capabilities must be generated from this status rather than from aspirational package descriptions.

## 11. Core data flow

### 11.1 Synchronous document mutation

```text
HTTP/MCP/SDK
  → auth + site resolution
  → command validation
  → SQL transaction
      ├── before hooks / validation
      ├── document write
      ├── audit row
      └── outbox row(s)
  → commit
  → response with operation/event IDs
```

The request must not wait for analytics, webhooks, email, search, or integrations unless the command explicitly requests synchronous completion.

### 11.2 Outbox publication

```text
Outbox publisher
  → claim row
  → JetStream publish with event ID
  → mark published
  → retry on uncertain outcome
```

### 11.3 Projection

```text
JetStream event
  → analytics/search/audit/realtime consumer
  → idempotency receipt
  → projection transaction
  → ack
```

### 11.4 Webhook

```text
document event
  → webhook consumer
  → match extension subscription
  → delivery attempt
  → _kora_webhook_delivery row
  → ack or retry/dead-letter
```

The current database delivery table remains the audit and customer-visible delivery history. JetStream is the work transport, not the only record of delivery.

### 11.5 Async scripts

Replace pointer-containing `orm.AsyncHookRequest` with a serialized DTO:

```json
{
  "site": "acme.example.com",
  "doctype": "Sales Invoice",
  "event": "after_save",
  "script_name": "sync_to_erp",
  "document": {},
  "old_document": {},
  "user": "user@example.com",
  "user_role": "Accounts User"
}
```

The worker reloads the registry and script definition. It never serializes runtime pointers.

## 12. Offline and synchronization design

Offline is a Kora feature, not a Cloud-only feature.

### 12.1 Local write

```text
User/device
  → branch Kora API
  → local authorization
  → local SQL transaction
  → local operation log/outbox
  → local UI update
```

### 12.2 Synchronization

```text
local outbox
  → sync worker
  → NATS leaf or HTTPS sync gateway
  → central intake
  → idempotency check
  → conflict policy
  → central SQL transaction
  → acknowledgement/correction event
```

### 12.3 Conflict rules

Conflict policy is domain-specific:

- stock: append immutable movements; never last-write-wins a balance;
- payments: append immutable attempts/settlements;
- sales: branch-owned records with globally unique IDs;
- customer profile: merge or send to review;
- workflow: reject stale version and produce a conflict task;
- configuration: central authority with versioned bundle rollout.

Every offline operation has `device_id`, `branch_id`, `operation_id`, `base_version`, and `occurred_at`. `occurred_at` is display/audit metadata only; server receipt time, server versions, and domain ledgers determine ordering and business state. A stolen device receives scoped, expiring, revocable offline credentials and only the data/actions needed by its role.

### 12.4 Synchronization protocol

The sync API is cursor-based and idempotent:

```json
{
  "device_id": "pos-7",
  "branch_id": "branch-west",
  "push": [
    {
      "operation_id": "op-1",
      "entity": "Sales",
      "entity_id": "sale-1",
      "action": "create",
      "base_version": 0,
      "payload": {},
      "occurred_at": "2026-08-12T10:00:00Z"
    }
  ],
  "cursor": "branch-west:1042"
}
```

```json
{
  "accepted": [{"operation_id": "op-1", "server_version": 1}],
  "rejected": [],
  "events": [],
  "next_cursor": "branch-west:1043"
}
```

Rejected operations are never deleted. They become local `conflict` records with the server version, error code, resolution mode (`retry`, `merge`, `manual_review`, or `discard`), and the server record snapshot where permitted. `discard` means that the local business effect is abandoned after explicit authorization; it does not delete the original operation or conflict record. Discard emits an immutable, auditable resolution event and remains subject to retention policy. Pull is repeatable from any cursor. Cursors are opaque and scoped to device/branch/application package.

Schema/configuration bundles are versioned. A device may continue using its last compatible bundle while offline, but the server may reject operations whose schema version is no longer accepted. Tombstones are retained for at least the maximum supported offline window.

The synchronization implementation must define behavior for creates with `base_version = 0`, stale updates, deletes against retained tombstones, branch-owned ID collisions, partial push responses, duplicate acknowledgements, and skipped schema versions. Rejected operations remain retryable or resolvable through conflict records and are never silently removed. Tombstone garbage collection is allowed only after the maximum supported offline window and configured device-retention grace period.

## 13. Open-source Kora state

Kora OSS must provide:

- embedded local provider;
- optional NATS provider;
- local SQL and customer-managed SQL;
- local worker process mode;
- CLI commands to create streams, consumers, KV buckets, and credentials;
- package export/import;
- standalone UI build;
- offline sync runtime;
- extension SDKs;
- observability endpoints and health checks;
- the shared agent executor and provider-neutral AI contracts;
- local AI run persistence, budget enforcement, usage metering, and audit;
- an explicit capability report identifying AI/MCP features as `planned`, `experimental`, or `supported`.

Recommended OSS deployment modes:

```text
kora serve                         embedded mode
kora serve --workers 4             local worker mode
kora serve --event-provider=nats   NATS-backed mode
kora worker analytics              dedicated worker mode
kora package export/import         portable application mode
```

OSS must include development NATS configuration and contract tests, but NATS should not be mandatory for `go test`, local CRUD, or a basic single-container deployment.

## 14. Kora Cloud state

Kora Cloud has two planes.

### 14.1 Control plane

Owns:

- organizations, users, plans, billing, and usage;
- AI provider accounts, pricing sheets, usage reconciliation, and cost corrections;
- tenant/database provisioning;
- application package registry;
- deployment/release orchestration;
- domain/TLS/certificate management;
- secret and credential issuance;
- NATS account/stream/consumer provisioning;
- worker placement and autoscaling;
- backup/restore and disaster recovery;
- regional placement and Cloud observability.

### 14.2 Data plane

Runs customer application workloads:

- Kora API services;
- command/query services;
- workflow actor hosts;
- projection and integration workers;
- realtime gateway;
- sync gateway;
- managed SQL;
- operator-hosted JetStream/KV/Object Store by default;
- optional Kora-managed NATS only as a future infrastructure offering.

The default Cloud data plane connects to a registered customer/operator NATS deployment. The NATS deployment may be shared by multiple Cloud sites only when accounts, subject permissions, KV buckets, Object Store namespaces, and backup/restore boundaries are separately validated. Cloud does not assume that a NATS URL implies ownership or administrative access.

Cloud tenants receive isolated NATS accounts or an equivalent operator-approved account boundary with explicit import/export permissions. NATS accounts provide infrastructure isolation; Kora authorization remains the business-level permission system. See [NATS security](https://docs.nats.io/nats-concepts/security).

Cloud may use shared worker pools for small tenants and dedicated pools for enterprise tenants. A shared pool must not use one credential spanning tenants: each delivery executes under a tenant-scoped credential/context, with per-tenant concurrency limits, fair scheduling, and tenant-level lag/usage accounting. Placement is a Cloud concern; application code sees only the provider contract.

Cloud resource model:

```text
Organization
  └── Environment
       ├── Site
       ├── ApplicationPackage
       ├── Deployment
       ├── Database
       ├── WorkerPool
       ├── NATSDeployment
       ├── NATSAccount
       ├── StreamSet
       ├── KVSet
       ├── ObjectStoreNamespace
       ├── CredentialReference
       ├── BackupPolicy
       ├── Backup
       └── UsageRecord
```

Each resource has an explicit lifecycle (`creating`, `ready`, `updating`, `failed`, `deleting`, `deleted`) and an operation ID for asynchronous transitions. `NATSDeployment` additionally has `unregistered`, `unreachable`, `incompatible`, `degraded`, and `draining` states. The control plane exposes versioned APIs for provisioning, package activation, deployment rollout, NATS registration/bootstrap, backup/restore evidence, credentials, quotas, and suspension. The data plane never makes billing or organization decisions.

#### 14.2.1 Tenant onboarding flow

The steady-state onboarding flow is:

```text
signup → organization → environment → site → NATS deployment registration → database + NATS account + streams/KV/consumers → application package upload → package verification and activation → deployment creation → credentials → DNS/TLS → warm → active
```

Each resource is created through the existing control-plane provisioning-job model. Resource creation starts in `creating`; successful reconciliation moves it to `ready`; retryable errors remain attached to the provisioning job and retry; terminal errors move the resource to `failed`; replacement and rollout use `updating`; teardown uses `deleting` and then `deleted`. Provisioning jobs are resumable and idempotent. A deployment becomes `active` only after package verification, data-plane readiness, NATS validation, credentials, DNS/TLS, and health checks succeed. The control plane records the operation ID and dependency status for every step. An embedded package can be exported, verified, activated as an immutable version, and deployed against self-hosted NATS without application-code changes; only provider configuration, credentials, and topology change.

NATS isolation is explicit:

```text
tenant account:
  publish:   kora.commands.<opaque-tenant-id>.>
  subscribe: kora.events.<opaque-tenant-id>.>
worker user:
  publish:   only its result/event subjects for one tenant
  subscribe: only its assigned task/event subjects for one tenant
branch user:
  publish:   kora.sync.<opaque-tenant-id>.<opaque-branch-id>.push
  subscribe: kora.sync.<opaque-tenant-id>.<opaque-branch-id>.pull
```

The exact subject policy is generated from the tenant/resource identity and is deny-by-default. Worker, branch, gateway, and operator credentials are separate, short-lived where possible, rotated, revocable, and audited. Leaf-node credentials are treated as revocable assets; revocation must cover already-connected sessions and reconnect attempts. Kora authorization remains authoritative for business actions; NATS authorization isolates infrastructure traffic. The outbox publisher resolves the account and credential from each row’s site and must reject an unresolved or mismatched site/account binding.

### 14.3 Kora Cloud repository boundary

`kora-cloud` is the control-plane and infrastructure-adapter repository. It imports Kora as a dependency and must not fork engine authorization, document mutation, workflow, tool-catalog, or agent-run logic.

```text
kora (engine)
  ├── SQL business state and migrations
  ├── kernel authorization and workflows
  ├── command/query contracts and shared tool executor
  ├── agent runs, approvals, provider adapters, and AI usage events
  └── LocalProvider/NATSProvider

kora-cloud (control plane)
  ├── organizations, plans, subscriptions, invoices, payments
  ├── site/environment/deployment desired state
  ├── NATSDeployment registration and bootstrap/validation
  ├── runtime placement and lifecycle adapters
  ├── channel ingress/delivery, consent, and delegated identity
  ├── usage ingestion, cost reconciliation, quotas, and FOCUS exports
  ├── health, alerts, backups, restore evidence, and operations
  └── provider credentials, secrets, and regional policy
```

The Cloud repository may contain channel-specific formatting and delivery code, but a WhatsApp, SMS, or email channel must submit an authenticated `CommandEnvelope{type: agent.run}` to the engine. It must not maintain a second agent loop, tool-schema generator, permission evaluator, or authoritative conversation state. Cloud dashboards project engine events; they do not replace them.

The Cloud/NATS integration is an infrastructure adapter:

```go
type NATSDeploymentProvider interface {
    Register(ctx context.Context, spec NATSDeploymentSpec) (NATSDeployment, error)
    Validate(ctx context.Context, deployment NATSDeployment) (NATSHealth, error)
    Bootstrap(ctx context.Context, deployment NATSDeployment, resources ResourceSet) error
    Drain(ctx context.Context, deployment NATSDeployment) error
    BackupManifest(ctx context.Context, deployment NATSDeployment) (BackupManifest, error)
}
```

The first implementation is `SelfHostedNATSProvider`, which stores only credential references and deployment metadata in Cloud. A later `ManagedNATSProvider` may own cluster creation, but must produce the same validation, resource, permission, backup, and lifecycle evidence. Neither implementation grants business authorization.

Recommended Cloud packages are:

```text
internal/controlplane       records, migrations, desired state
internal/provisioning       durable resource reconciler and recovery
internal/runtime            shared/Dokploy/container runtime adapters
internal/nats               registration, bootstrap, validation, permissions
internal/tenantgateway      engine command, capability, and event client
internal/channels           WhatsApp/SMS/email ingress, consent, delivery
internal/authbroker         Cloud-to-engine delegation and identity mapping
internal/usage              immutable event ingest and usage projections
internal/billing            plans, quotas, invoices, reconciliation
internal/secrets             secret references, rotation, revocation
internal/observability       health, metrics, traces, alerts, SLO evidence
```

The current DB-backed provisioning worker is a valid early Cloud implementation. Its durable job records must evolve toward the same command, idempotency, lease, fencing, retry, dead-letter, and recovery semantics as the engine runtime. Redis may remain a compatibility cache, but it is not the source of truth for NATS, agent, workflow, or lease coordination.

#### 14.3.1 Current `kora-cloud` mapping

The existing Cloud repository maps to the target as follows:

| Existing Cloud area | Target responsibility |
|---|---|
| `internal/controlplane` and Cloud DocTypes | Cloud organization, site, plan, subscription, payment, provisioning, usage, and health projections. |
| `internal/provisioning` | Durable resource reconciler; retain DB-backed execution first, then add NATS command/actor dispatch and fencing. |
| `internal/runtime` | Runtime-provider adapters for shared/container/Dokploy deployment lifecycle; no business semantics. |
| `internal/messaging` WhatsApp ingress/delivery | Channel adapter only; submit engine runs and consume typed progress/completion events. |
| `internal/messaging` agent loop/tools | Move to the Kora engine agent runtime; Cloud must not keep a second executor or catalog. |
| `internal/messaging` usage records | Projection of immutable engine usage/cost events; retain Cloud aggregates for billing and dashboards. |
| `internal/cache` Redis | Compatibility/cache layer only; NATS KV becomes authoritative for distributed coordination. |
| `internal/tenantclient` | Versioned engine gateway for commands, queries, capability discovery, health, and event delivery. |
| `internal/config` | Cloud control-plane configuration, NATS deployment references, secret references, quotas, and feature status. |

The existing Cloud architecture may continue to use one engine per site and operator-hosted MariaDB in the first deployment tier. That deployment shape is compatible with this RFC if the engine connects to the registered self-hosted NATS deployment and all asynchronous behavior follows the provider-neutral contracts.

### 14.4 Kora Cloud implementation specification

This section is normative for `kora-cloud`. Cloud is a control plane and infrastructure adapter. It stores desired state, reconciles external resources, exposes operator/customer APIs, and projects engine events into billing and operations views. It does not execute tenant business commands directly.

#### 14.4.1 Resource ownership

Cloud owns control-plane resources: organizations, environments, sites, plans, subscriptions, invoices, payments, deployments, NATS deployments, account bindings, database/storage bindings, credential references, provisioning jobs, backup policies/records, usage projections, cost reconciliations, health snapshots, and domains. The tenant engine owns users, documents, workflows, agent runs, approvals, tool audit, and raw AI usage/cost events.

Every Cloud resource has `id`, organization/environment/site scope, desired state, observed state, version, operation ID, last error, retry time, and timestamps. Updates use optimistic concurrency; a reconciler cannot overwrite a newer desired state from an old provider response.

#### 14.4.2 Self-hosted NATS registration

The first Cloud release assumes customer/operator-hosted NATS JetStream. Cloud stores metadata and secret references, not private keys or credential values:

```json
{
  "name": "production-nats-af-south",
  "region": "af-south",
  "servers": ["tls://nats.example.internal:4222"],
  "credential_ref": "secret://nats/production-af-south/cloud-agent",
  "account_mode": "per_tenant",
  "tls_required": true,
  "jetstream_required": true,
  "backup_policy_ref": "backup-standard-v1",
  "rpo_seconds": 300,
  "rto_seconds": 900
}
```

Registration is `register → resolve secret → TLS/connectivity check → server/version check → JetStream check → account/permission check → stream/KV/Object Store check → backup evidence check → ready|incompatible`.

The default integration receives pre-created tenant accounts and scoped credentials from the operator. A delegated bootstrap account may create resources only under an explicit, audited policy. Normal runtime credentials must never be NATS system-admin credentials. Credential rotation preserves deployment/resource IDs and invalidates the old credential after a successful health check.

#### 14.4.3 Tenant resource provisioning

For each site/environment, Cloud provisions or validates a tenant account boundary, `KORA_EVENTS`, `KORA_COMMANDS`, `KORA_TASKS`, `KORA_AI`, `KORA_SYNC`, and `KORA_AUDIT` streams, configuration/capability/lease/AI KV buckets, an Object Store namespace, engine and worker credentials, and channel gateway credentials. Names and subjects use stable internal IDs rather than mutable hostnames or labels.

Every resource step records an idempotency key, input fingerprint, provider request ID, output ID, attempt, lease, and cleanup action. Partial failure leaves the job resumable and never marks the site active.

#### 14.4.4 Provisioning state machine

```text
requested → validating → awaiting_capacity → provisioning_control_resources
  → provisioning_nats → provisioning_database → deploying_engine
  → configuring_domains → validating_runtime → active

active → updating | suspending | deleting
any active step → retry_wait | failed
suspending → suspended; deleting → deleted
```

Workers use leases/fencing so only one reconciler mutates a deployment. Retryable failures use bounded backoff; terminal failures require an operator action or a new operation. `active` requires committed control records, database/bootstrap success, NATS validation, healthy engine, authenticated read-only command success, scoped credentials, healthy DNS/TLS, package compatibility, and backup/restore evidence.

#### 14.4.5 Cloud-to-engine gateway

```go
type TenantGateway interface {
    Request(context.Context, string, CommandEnvelope) (CommandResult, error)
    Submit(context.Context, string, CommandEnvelope) (TaskReceipt, error)
    GetOperation(context.Context, string, string) (OperationStatus, error)
    Capabilities(context.Context, string) (CapabilitySnapshot, error)
    Health(context.Context, string) (EngineHealth, error)
}
```

The gateway validates registered site/deployment routing but does not rewrite actor, delegation, site, operation, correlation, idempotency, or deadline fields. The engine re-authorizes every command. Cloud identity, tenant subject user, channel identity, agent identity, and delegation ID remain separate.

#### 14.4.6 Channels and agents

```text
provider webhook → signature/replay check → consent/rate limit
  → site/conversation resolution → delegated ActorContext
  → agent.run CommandEnvelope → typed progress/completion event
  → Cloud delivery policy/formatting → provider send with idempotency key
```

Cloud stores channel delivery attempts, consent, provider message IDs, and delivery state. The engine stores authoritative agent state, tools, approvals, business effects, and AI usage. Cloud channels must not maintain a second agent loop, tool catalog, permission evaluator, or authoritative conversation state. `accepted` runs are delivered asynchronously; `pending_approval` requires explicit user confirmation before Cloud submits an approval command.

#### 14.4.7 Usage and billing

Cloud consumes `KORA_EVENTS`/`KORA_AI` with a durable `cloud-usage-v1` consumer, deduplicates by source event ID, stores raw usage projections, reconciles provider invoices, and produces billing projections/corrections. Estimates are allowed for admission/UI feedback; final charges require immutable usage events, versioned pricing, provider attribution, and reconciliation. Replaying events must reproduce the same projection.

The engine reserves per-run/site/agent budgets before provider calls. Cloud enforces commercial plan/account limits and suspension. A quota rejection is typed and causes no tenant command or provider call.

#### 14.4.8 Suspension, deletion, and recovery

Suspension stops new work, revokes/deactivates credentials, drains or cancels workers, blocks provider calls and external delivery, and pauses or expires pending approvals according to policy. Resumption revalidates NATS, database, credentials, package compatibility, and engine health.

Deletion is a confirmed, auditable workflow: create manifest, revoke credentials, stop routing, drain workers, preserve required financial/audit records, delete resources under retention policy, verify backup/Object Store cleanup, and emit a completion receipt. Failed deletion remains visible and retryable.

#### 14.4.9 Cloud implementation gates

Cloud is `supported` only when operator-hosted NATS registration/rotation/drain/recovery works; provisioning is idempotent, resumable, and fenced; `active` cannot precede all dependency gates; Cloud-to-engine context is preserved and re-authorized; channels use engine runs; usage ingestion is durable/replayable/reconciled; suspension blocks effects; and deletion, backup/restore, NATS permissions, credential rotation, and cross-tenant isolation tests pass.

## 15. Frontend architecture: standalone and composable

### 15.1 Decision

The frontend should become a standalone, versioned, hybrid application. It may be:

- served by Kora for convenience;
- deployed independently to a CDN or customer infrastructure;
- embedded as a host shell with independently deployed feature bundles.

The frontend communicates through versioned HTTP APIs and authenticated realtime endpoints. It does not connect directly to NATS.

The architecture is deliberately hybrid:

```text
Code-owned platform:
  shell, router, component implementations, security, forms,
  data client, offline engine, accessibility, error/loading states

Manifest-owned application surface:
  page layout, sections, blocks, data bindings, filters,
  actions, workflow panels, navigation, visibility rules
```

This is not a pure server-rendered UI and not a microfrontend system. The browser receives safe, versioned composition metadata and renders it using a locally installed component runtime. This preserves frontend quality and performance while allowing application packages and tenants to compose pages without adding route code.

The primary precedents are Shopify JSON templates/sections/blocks, Salesforce Lightning App Builder metadata, Backstage frontend plugins, TanStack Router code splitting/data loading, and JSON Forms-style schema-driven rendering. These are references for specific mechanisms, not dependencies Kora must adopt wholesale.

### 15.2 Current problem

The current router has hard-coded route/component imports for dashboard, CRUD, admin, and page routes. Dynamic views exist, but a new page still tends to require a new React component or route-level decision.

The new target is a page manifest runtime:

```text
route → page manifest → data resources + actions + component tree → renderer
```

The current `View` and `Page` concepts become compatibility projections over this single model. A view is a reusable component tree or data preset; a page is a routable manifest that composes views, resources, and actions. New page categories must not require separate renderer architectures.

#### 15.2.1 Runtime layers

```text
Kora UI runtime
  ├── App shell: auth, router, navigation, theme, permissions, offline state
  ├── Manifest runtime: fetch, verify, validate, resolve, load, dispatch
  ├── Component registry: layout, data, forms, workflow, commerce, operations
  ├── Data/command client: API, cache, optimistic updates, offline queue
  └── Extension runtime: packages, components, pages, declarations
```

The shell and runtime are shipped as a standalone `kora-ui` application. Kora may serve its assets, but UI deployment must not require rebuilding the Go server. Runtime configuration contains API/realtime URLs, tenant identity, frontend runtime version, and manifest endpoint; it must not contain secrets.

The initial design is not microfrontends. Independently mounted applications would duplicate routing, dependencies, caches, design tokens, and accessibility behavior. The initial extension unit is an installable page/component package consumed by one stable runtime. Microfrontends may be added later for extensions requiring independent release or security isolation.

#### 15.2.2 Builder and runtime parity

Kora adopts a renderer-first visual builder. The builder is an authoring mode of the Kora UI runtime, not a separate design tool that approximates the application. It edits the canonical page/view manifest and renders that manifest through the same component registry, layout engine, design tokens, data-binding evaluator, permission boundaries, responsive breakpoints, and resource-state components used by the published page.

The builder must provide:

- a guided start flow with page templates, sensible defaults, route/type/layout selection, and a visible setup checklist;
- a searchable component palette showing only components supported by the current package, runtime, permissions, and parent slot;
- a semantic component tree and named regions/slots so users can see hierarchy, order, and nesting directly;
- drag-and-drop insertion and reordering constrained by the manifest schema, with keyboard alternatives and explicit drop targets;
- a property inspector generated from the selected component's schema, including schema-aware field, resource, filter, action, and visibility choices;
- binding suggestions derived from the selected DocType/resource schema, with inline validation and a clear explanation when a binding is invalid;
- undo/redo, duplicate, move, hide/show, reset-to-default, draft autosave, validation summary, and publish/rollback entry points;
- exact desktop, tablet, and mobile previews using the same responsive rules as the runtime, plus a live data preview when authorized.

The editor may draw selection outlines, grid guides, drop targets, and diagnostic overlays around the rendered page. Those overlays are editor chrome and must not change layout measurement, typography, component behavior, data loading, or action semantics. “Design” and “Live” modes are views of the same manifest; they are not two renderers.

The builder must not default to arbitrary absolute positioning or a freeform coordinate canvas. Layout is represented as a deterministic tree of regions, stacks, grids, splits, tabs, and components with explicit order, span, alignment, and responsive overrides. Coordinates are allowed only for a registered component whose contract explicitly defines bounded positioning. Reordering the same manifest must produce the same result across clients and sessions.

### 15.3 Page manifest

```yaml
apiVersion: ui.kora.dev/v1
kind: Page
metadata:
  name: sales-dashboard
  version: 2.1.0
  package: erp.sales
spec:
  route: /workspace/sales
  required_permissions: ["Sales Order:read"]
  resources:
    - id: sales_summary
      query: sales.summary
      params: {period: "$url.period|30d"}
    - id: recent_orders
      query: document.list
      params: {doctype: "Sales Order", limit: 20}
  actions:
    - id: create_order
      command: document.create
      input: {doctype: "Sales Order"}
      invalidate: [recent_orders, sales_summary]
  layout:
    type: grid
    columns: 12
    children:
      - component: StatCard
        props: {title: "Revenue"}
        data: sales_summary.revenue
      - component: TimeSeriesChart
        data: sales_summary.timeline
      - component: DataTable
        data: recent_orders
        actions: [create_order]
```

The manifest is declarative. The renderer only permits registered components and query/action types. Arbitrary executable JavaScript must not be accepted from a tenant page manifest.

#### 15.3.1 Binding language

Bindings use a restricted path language; arbitrary JavaScript is prohibited. Valid roots are:

```text
resource.<id>.data.<path>
record.<field-path>
url.<query-or-path-param>
session.user.<field>
literal(<json-value>)
```

Allowed operators are limited to `coalesce`, `format`, `equals`, `not`, `and`, `or`, `if`, and numeric/date formatting. Expressions are represented as JSON AST nodes, not executable strings. The renderer rejects unknown roots, functions, or paths.

Rendered tenant data is untrusted. Markdown and rich text are sanitized by default, raw HTML and scriptable URLs are rejected, links are restricted to approved schemes/origins, and the standalone UI uses a restrictive Content Security Policy. Manifest bindings cannot select HTML injection paths, execute code, or bypass component escaping.

Resources declare dependencies explicitly. A dependent resource starts only after its dependency resolves. Every resource has loading, error, empty, stale, and refreshing states. Pagination uses opaque cursors; filters and sorting use the backend query schema rather than frontend-generated SQL.

#### 15.3.2 Forms and record editing

`RecordPage` and `Form` use the doctype schema as the data contract and a UI schema as the layout contract:

```text
doctype schema → fields/types/validation/permissions
UI schema      → sections/order/widgets/visibility
runtime        → dirty state/drafts/actions/workflow/offline policy
```

The backend remains authoritative for computed fields, child-table validation, hooks, workflow transitions, field permissions, and optimistic-concurrency checks. A mutation includes `base_version`; a stale version returns `conflict` and never silently overwrites the server record. File uploads use pre-authorized Kora endpoints and are represented in the command payload by an upload ID.

#### 15.3.3 Manifest contract

Every manifest must validate against JSON Schema and include:

```yaml
apiVersion: ui.kora.dev/v1
kind: Page
metadata:
  name: sales-dashboard
  version: 2.1.0
  package: erp.sales
  hash: sha256:...
  status: draft|preview|active|retired
spec:
  route: /workspace/sales
  runtime: ">=2.0.0 <3.0.0"
  permissions: []
  capabilities: []
  resources: []
  actions: []
  layout: {}
```

The schema constrains component names/versions, props, data paths, query/action names and parameters, nesting depth, component count, permission expressions, and offline eligibility. The server validates at publish time and the client validates at load time. Unsupported manifests produce a structured page error; they never execute arbitrary code.

Resources are read declarations. Actions are command declarations. A manifest contains no SQL, arbitrary URLs, credentials, or business authorization logic. Backend authorization and validation remain authoritative.

#### 15.3.4 Visual builder contract

The builder persists semantic manifest data, never a screenshot, DOM snapshot, browser-specific coordinates, or editor-only orientation metadata. A save operation must produce a normalized manifest that can be loaded by the standalone runtime without the builder bundle. The normalization step must make component IDs, child order, regions, spans, defaults, responsive overrides, bindings, and action references deterministic so equivalent edits produce stable output and meaningful diffs.

The builder-to-runtime flow is:

```text
choose template or start blank
  → select page goal and data source
  → compose valid regions/components in the semantic tree
  → configure schema-backed bindings/actions in the inspector
  → validate draft and show actionable errors
  → render exact draft through the production renderer
  → preview with real permissions and representative data states
  → publish an immutable version
```

Preview must exercise loading, empty, error, permission-denied, stale/refreshing, offline, and long-running-operation states for every component that declares them. A builder preview that renders placeholder cards or a different orientation than the active page does not satisfy this contract. The publish preflight must report missing data sources, unsupported components, invalid bindings, unreachable actions, accessibility failures, incompatible responsive rules, and unresolved permissions before activation.

The builder is allowed to expose advanced manifest details through a source/JSON view for expert users, but the visual editor remains the primary path. Source edits are schema-validated, normalized, undoable, and immediately reflected in the same rendered canvas. Invalid source edits cannot be published or silently repaired into a different layout.

### 15.4 Component registry

Each component has:

```text
component name
component API version
supported props schema
data binding schema
action/event schema
accessibility contract
responsive behavior
offline behavior
```

Initial groups:

```text
layout       Stack, Grid, SplitPane, Tabs, Section, Drawer
content      Text, Badge, EmptyState, Markdown
data         DataTable, Form, RecordCard, MetricCard, Chart
workflow     ApprovalQueue, StatusTimeline, ActionBar, Stepper
commerce     ProductGrid, CartPanel, PaymentPanel, ReceiptPreview
operations   ScannerInput, Calendar, Kanban, Map, Checklist
system       Loading, Error, PermissionDenied, OfflineBanner
```

A page is composable if it can combine components, resources, and actions without a new route component. Some specialized behavior may still need a registered component extension; the important rule is that the page author does not modify the core router.

Each component registration includes its name, semantic version, props/data schemas, allowed parents/page kinds, required capabilities/permissions, lazy loader, accessibility contract, responsive behavior, and offline behavior. Component implementations remain code; placement, configuration, data, and actions are manifest data.

Initial component groups:

```text
layout: Stack, Grid, SplitPane, Tabs, Section, Drawer
content: Text, Markdown, Badge, Avatar, EmptyState, Alert
data: DataTable, RecordCard, MetricCard, Chart, Timeline, Calendar
forms: Form, Field, LinkField, ChildTable, FileUpload, ScannerInput
workflow: StatusBadge, ActionBar, ApprovalQueue, Stepper, Checklist
commerce: ProductGrid, CartPanel, PaymentPanel, ReceiptPreview
operations: Kanban, Map, RoutePlan, Queue, ShiftBoard, DocumentScanner
system: Loading, ErrorState, PermissionDenied, OfflineBanner, SyncStatus
```

Each component must support loading, error, empty, responsive, keyboard, screen-reader, and offline states where applicable.

Component packages are stored in the application package registry or Kora Object Store and are signed by a trusted package key. Core components are bundled with `kora-ui`; trusted extension components are lazy-loaded only after signature, integrity hash, runtime range, and dependency checks pass. Tenant-authored manifests may reference registered components but may not upload executable component code.

Application packages are the signed deployment unit. Each immutable package version has a digest, signature, trusted signing-key identity, dependency metadata, and supported engine/provider/frontend ranges. The registry verifies integrity and signatures on upload and again before activation; activation also runs schema, dependency, permission, and compatibility checks. Invalid, revoked, or quarantined packages cannot activate. Upload, verification, activation, rollback, and revocation are audited, and rollback targets a previously verified immutable version.

If a component is unavailable or incompatible, the runtime renders a typed `UnsupportedComponent` state containing the component name, required version, available capabilities, and a retry/update action. It must not render a blank page or execute a fallback with elevated permissions.

### 15.5 Page lifecycle

```text
load route
  → fetch page manifest by package/name/version
  → verify signature and permission requirements
  → resolve components against client capability registry
  → fetch resources through API
  → render skeleton
  → render data
  → subscribe to realtime invalidation/events
  → apply optimistic/offline mutations where permitted
```

Realtime messages should invalidate or patch resource caches. They should not contain arbitrary UI instructions.

Detailed lifecycle:

```text
URL → route match → tenant/auth context → compatible manifest
    → hash/signature/schema verification → component capability check
    → lazy component chunks + parallel critical resources → skeleton
    → data render → command actions → realtime invalidation/patch
    → optimistic/offline mutation queue where allowed
```

The browser never receives raw NATS credentials or subjects. Runtime configuration is signed or fetched through an authenticated, origin-bound endpoint and is validated for tenant, package, API origin, and capability bindings before use:

```text
Browser → authenticated WebSocket/SSE gateway → Kora realtime service → NATS
```

Realtime messages are typed cache operations such as `resource.invalidated`; they do not contain arbitrary component names, JavaScript, or render instructions.

The frontend release pipeline must measure LCP, INP, CLS, time to shell, time to manifest, time to first useful data, initial compressed JS/CSS, route chunk size, manifest size, realtime invalidation latency, and offline queue flush time. Initial targets are LCP ≤2.5s p75, INP ≤200ms p75, CLS ≤0.1 p75, shell visible ≤1s on broadband, first useful data ≤2.5s, cached navigation ≤500ms, initial compressed JS ≤250KB, and typical manifests ≤50KB. These are acceptance targets, not claims about NATS performance.

### 15.6 Page versioning

Use independent version dimensions:

```text
Kora API version       /api/v1, /api/v2
Page manifest version  apiVersion + metadata.version
Component API version  component@2
Application package    package version
Schema version         doctype/migration version
Event contract         event type + integer version
```

Rules:

- API major versions are additive within a major and never silently change meaning.
- Page manifests pin component major versions.
- Component minor versions may add optional props; major versions may remove or change behavior.
- Every manifest declares minimum frontend runtime version and required capabilities.
- The server may serve the latest compatible manifest or a pinned version.
- Published manifests are immutable. A new page release creates a new version.
- Draft, preview, active, and retired are separate lifecycle states.
- Old manifests remain available until no supported client can use them.

Publication is an application-package operation:

```text
draft → validate → preview → approve → active → retired
```

Activation verifies components, queries, actions, permissions, referenced doctypes/fields, manifest hash/signature, and frontend runtime capability. Published manifests are immutable and served with ETags. Rollback activates a previous manifest rather than mutating the active one.

Example route resolution:

```text
/workspace/sales
→ package erp.sales
→ page sales-dashboard
→ active version compatible with frontend 2.x
→ manifest hash + ETag
```

Page ownership is explicit: a platform page belongs to the platform package, an application page belongs to an application package, and a tenant customization belongs to the tenant environment. Overrides are layered only in this order: platform default → application package → environment override. User preferences may change presentation state but may not change permissions, commands, or resource scopes.

### 15.7 New page types

The next page system should not distinguish “view” and “page” by separate rendering architectures:

```text
Page shell
  ├── list page
  ├── record page
  ├── dashboard page
  ├── workflow page
  ├── workspace page
  ├── public form page
  └── custom composed page
```

They are all manifests using different data/action presets. Existing doctype list/new/edit routes remain compatibility routes that resolve to generated manifests.

Required presets are `WorkspacePage`, `DashboardPage`, `ListPage`, `RecordPage`, `WorkflowPage`, `PublicFormPage`, and `CustomPage`. Generated CRUD pages are first-class manifests, not a separate legacy renderer; this is what makes every Kora page composable while preserving strong defaults.

### 15.8 Frontend extension packages

An application package may contribute page manifests, navigation entries, component registrations, schemas, query/action declarations, icons/design tokens, and permissions. It may not bypass the API client, access raw NATS, inject arbitrary scripts, register unscoped global routes, bypass permissions, or mutate another package's query cache.

```yaml
frontend:
  package: erp.logistics.ui
  version: 1.2.0
  runtime: ">=2.0.0 <3.0.0"
  components: [{name: RoutePlan, version: 1}]
  pages: [{name: dispatch-board, version: 1.0.0}]
```

The runtime validates and enables only compatible declarations.

### 15.9 Frontend implementation patterns and code structure

This section is normative for the Kora frontend. The goal is to make the safe path the default path and to prevent route-specific, adapter-specific, or client-authorized implementations from appearing over time.

#### 15.9.1 Approved framework and libraries

Kora UI uses the following baseline:

| Concern | Required choice | Reason |
|---|---|---|
| Application framework | React 19 with TypeScript | Matches the existing UI, provides typed component contracts, and supports the manifest renderer and package model. |
| Build/runtime | Vite | Fast standalone builds, explicit runtime configuration, simple OSS embedding, and Cloud deployment portability. |
| Routing | TanStack Router | Typed route parameters/search state, route-level loading/error boundaries, and compatibility with standalone and embedded deployments. |
| Server state | TanStack Query | Cache ownership, request deduplication, retries, invalidation, pagination, optimistic mutation control, and realtime invalidation. |
| Client state | Zustand, only for bounded UI/session state | Avoids putting server data or authorization decisions into a global store. |
| Tables | TanStack Table | Typed, headless tables that can be driven by manifest schemas and server-side pagination. |
| Styling | Tailwind CSS plus the approved Base UI/shadcn component layer | Shared accessible primitives, consistent tokens, and no page-specific styling systems. |
| Validation | TypeScript plus generated JSON Schema validators | Compile-time contracts and runtime validation for API, manifest, package, and user-provided data. |
| Unit/component tests | Vitest and React Testing Library | Fast deterministic tests for runtime, components, bindings, state transitions, and error behavior. |
| Browser tests | The project’s approved browser E2E harness | Required for authentication, permissions, manifest rendering, offline flows, and critical mutations. |

These choices are platform defaults. Adding another router, server-state library, global state library, styling system, form system, or component primitive requires an RFC amendment and migration plan. A package may use a dependency internally only if it cannot alter the public runtime contract or duplicate platform behavior.

#### 15.9.2 Required source structure

The frontend is organized by platform boundary, not by an uncontrolled collection of screens:

```text
ui/src/
  app/                 application bootstrap, providers, runtime config, error boundaries
  routes/              thin TanStack Router route adapters only
  layouts/             authenticated/public/console shells and navigation
  features/            user-facing vertical features; each owns view composition
  components/
    ui/                accessible, presentation-only primitives
    data/              tables, charts, pagination, resource-state components
    forms/             schema-driven field and form primitives
    views/             manifest component implementations and compatibility views
  manifest/
    schema/            manifest JSON Schemas and generated types
    runtime/           binding, resource, action, permission, and lifecycle execution
    registry/           component/query/action registration
    validation/        publish/load validation and compatibility checks
  api/
    client/             authenticated HTTP/engine client and response normalization
    queries/             TanStack Query option factories and query-key definitions
    commands/            typed mutation commands and idempotency handling
    realtime/            authenticated SSE/WebSocket client and typed invalidations
  auth/                 session store, route guards, capability snapshots; no business authorization
  state/                bounded UI preferences, drafts, and offline queue state only
  contracts/             generated API, command, event, and package types
  lib/                  pure utilities with no React or network side effects
  styles/               global tokens, Tailwind configuration, and global CSS
  test/                  test fixtures, fake providers, contract helpers, and accessibility helpers
```

Existing files may be migrated incrementally into these boundaries. New code must follow this structure. A route must not contain API implementation, raw fetch calls, authorization logic, manifest interpretation, or reusable business rules.

#### 15.9.3 Ownership and data-flow rules

The only permitted direction is:

```text
route → feature/layout → component or manifest runtime
                         → api query/command boundary
                         → Kora HTTP/engine contract
                         → TanStack Query cache
                         → typed realtime invalidation
```

- TanStack Query owns server data, loading/error states, pagination, and cache invalidation.
- Zustand owns only ephemeral UI state, user preferences, session presentation state, draft metadata, and explicitly designed offline-queue state. It must not become a second server cache.
- The API layer owns authentication headers/cookies, deadlines, idempotency keys, ETags, response-envelope parsing, stable error mapping, and correlation IDs.
- The manifest runtime owns manifest validation, safe binding evaluation, component lookup, resource dependencies, action declaration, and capability checks.
- The backend owns permissions, field visibility, record access, workflow rules, computed values, conflict decisions, and mutation validation.
- Components receive typed props and callbacks. They must not read raw URL parameters, call `fetch`, construct NATS subjects, inspect JWT claims, or make permission decisions.
- Query keys are created only by centralized factories and must include every tenant/site, package, version, and parameter that affects the result.
- Mutations use typed commands, server idempotency keys, `base_version`/`ETag` where applicable, and invalidate or update affected queries only after the server response succeeds.
- Realtime messages are typed invalidation/patch events. They never carry executable UI instructions or authorize a client mutation.

#### 15.9.4 Route, feature, and manifest rules

Routes are thin adapters. A route may select a feature, load route parameters, define a loader boundary, and render a page shell. It may not implement business workflows or duplicate a manifest renderer.

Feature modules must expose a small public surface:

```text
feature/
  components/       feature-specific composition
  queries.ts         query option factories
  commands.ts       typed mutations
  contracts.ts      feature-local types derived from canonical contracts
  permissions.ts    UI capability hints only; never authorization
  index.ts          approved public exports
```

All new CRUD and dashboard screens use a manifest preset or a registered component. A bespoke route component is allowed only for platform shells, authentication, package administration, and interactions that cannot be represented safely by the manifest contract. Such exceptions require a documented reason and contract tests.

#### 15.9.5 State, forms, and mutation safety

- Every async operation has explicit `idle`, `loading`, `success`, `empty`, `error`, `offline`, and `conflict` behavior where applicable.
- Forms are generated from canonical doctype/command schemas. Client validation improves feedback but never replaces server validation.
- Draft state is local, bounded, versioned, and marked with its site, user, package, schema version, and record version. Sensitive fields are excluded from browser persistence unless explicitly permitted by retention policy.
- Optimistic updates are allowed only for reversible, non-sensitive presentation state or operations with an explicit rollback contract. Financial, stock, payment, permission, and workflow effects wait for authoritative server acceptance.
- Every mutation displays typed success, validation, permission, timeout, conflict, offline, and retry states. It must not treat an HTTP success transport status as business success without checking the response envelope.
- Destructive or high-impact actions use the server-provided approval/recent-auth contract and show the exact evaluated action before submission.

#### 15.9.6 Forbidden frontend patterns

The following are prohibited:

- raw `fetch` or direct WebSocket/NATS use outside `api/`;
- API calls from presentational components;
- server data stored in Zustand or module-level mutable variables;
- permission checks that grant access, client-only role checks, or hidden security controls;
- arbitrary JavaScript, HTML, SQL, URLs, or expressions in manifests;
- string-built query keys, NATS subjects, or command names from unvalidated user input;
- silent `any` casts at API, manifest, auth, or mutation boundaries;
- mutation retries without idempotency keys;
- optimistic updates for irreversible business effects;
- global event buses used as a second cache or workflow engine;
- duplicated tool catalogs, query schemas, action definitions, or error maps;
- secrets, provider tokens, raw prompts, sensitive tool results, or full documents in browser logs, metrics, or local storage.

#### 15.9.7 Frontend quality gates

Every frontend feature must pass:

1. TypeScript strict compilation and linting with no new boundary escapes.
2. Manifest/schema validation and contract-parity tests where manifests or generated pages are involved.
3. Unit tests for loading, empty, error, permission-denied, conflict, offline, retry, and duplicate-response behavior.
4. Accessibility tests for keyboard navigation, focus management, labels, contrast, and screen-reader semantics.
5. Security tests for XSS, unsafe URLs, tenant/site switching, stale sessions, unauthorized actions, and manifest injection.
6. E2E tests for the complete user-critical flow, including server rejection and reconnect behavior.
7. Performance checks for route chunk, manifest, query, and interaction budgets.
8. Review confirmation that all mutations use canonical commands, idempotency, deadlines, and server-authoritative responses.
9. Builder/runtime parity tests proving that a manifest saved by the builder renders through the production renderer with the same structure, layout rules, responsive breakpoints, data states, accessibility semantics, and command behavior in draft preview and active-page mode. The tests cover insert, reorder, nest, configure, undo/redo, source edit, reload, publish, rollback, and invalid-manifest recovery.

The frontend build must fail when a manifest references an unknown component/query/action, when a package exceeds its declared compatibility range, when generated contracts drift, or when an extension imports prohibited platform internals. These checks are release gates, not advisory documentation.

### 15.10 Frontend product and UX specification

The frontend must make Kora feel simple even when the runtime is capable of workflows, offline operation, realtime updates, packages, agents, and Cloud infrastructure. Complexity belongs in progressive disclosure, sensible defaults, guided setup, and contextual detail—not in the primary navigation or every screen.

Kora's UX north star is: **make the right thing obvious, make the first success quick, and make every change reversible**. A business user should be able to create and adjust a useful screen without knowing that Kora uses manifests, components, schemas, queries, NATS, or packages. Technical concepts remain available to experts in an advanced/source view, but they are never prerequisites for normal setup or customization.

#### 15.10.1 User-facing mental model

Users should understand Kora through four concepts:

```text
Workspace     the work I do every day
Records       the business information I manage
Tasks         approvals, conflicts, sync work, and actions needing attention
Settings      configuration, access, integrations, and administration
```

Cloud adds a fifth concept:

```text
Operations    deployments, environments, usage, health, credentials, and billing
```

The default navigation must not expose NATS, JetStream, actors, KV, consumer names, provider profiles, or implementation topology to ordinary users. Those details appear only in Admin/Operations views with contextual explanations.

#### 15.10.2 Required Kora OSS pages

The OSS application must provide these page capabilities. A deployment may hide pages for which the package or permissions do not provide data, but it must not create a second navigation model.

| Area | Required pages/capabilities | UX expectation |
|---|---|---|
| Workspace | Home/dashboard, recent records, saved views, quick create, global search | One primary goal, urgent work first, useful blank slate, clear next action. |
| Records | List, record, create/edit, related records, history/activity, attachments | Server-side filtering/sorting, visible record state, optimistic concurrency, readable audit trail. |
| Tasks | Approvals, workflow tasks, sync conflicts, failed operations | Action-oriented queue with status, owner, due state, and safe resolution flow. |
| Offline | Device status, queued operations, conflicts, last sync, supported offline actions | Always show online/offline/syncing/blocked state without interrupting normal work. |
| Insights | Dashboard, reports, analytics views | Operational dashboards show what needs attention; analytical views support drill-down. |
| Administration | Users/roles, doctypes, permissions, workflows, views/pages, scripts/extensions | Advanced settings use grouped sections, safe defaults, previews, validation, and audit. |
| Help/status | Contextual help, keyboard shortcuts, system status, release/capability status | Explain impact and recovery in plain language; do not expose internal stack traces. |

Generated CRUD pages are the default. A package may add a domain page only through a manifest and registered component/query/action contract.

#### 15.10.3 Required Kora Cloud pages

Cloud must use the same Kora workspace conventions and add a separate Operations area:

| Area | Required pages/capabilities | UX expectation |
|---|---|---|
| Cloud home | Organization overview, environment/site health, urgent incidents, usage summary | Status at a glance with actionable exceptions, not infrastructure dashboards by default. |
| Organizations | Members, roles, teams, invitations, identity providers | Safe defaults, clear scope labels, recent-auth for sensitive changes. |
| Environments/sites | Site list, site overview, deployment status, package/version, domains | One deployment lifecycle view with clear `creating`, `ready`, `degraded`, `suspended`, and `failed` states. |
| Deployments | Deployment details, rollout history, package activation, compatibility, rollback | Guided rollout with preflight checks, progress, evidence, and reversible rollback. |
| NATS/infrastructure | Registered deployments, connectivity, streams/KV/Object Store, permissions, backup evidence | Advanced Operations-only view; lead with health and next action, reveal technical detail on demand. |
| Workers | Pool capacity, tenant fairness, lag, leases, failures | Show impact and remediation, not raw consumer internals unless expanded. |
| Security | Credentials, secret references, rotations, revocations, audit events, isolation evidence | Scope, owner, expiry, last use, and emergency action are always visible. Secret values are never shown. |
| Usage/billing | Usage, budgets, provider attempts, estimates, reconciliations, invoices, corrections | Distinguish estimated, finalized, corrected, and disputed amounts. |
| Recovery | Backups, restore tests, RPO/RTO, deletion workflows, retention/legal holds | Show evidence and completion state; never imply recovery readiness without a verified test. |

Cloud pages must not duplicate engine pages for documents, workflows, agent runs, tools, or permissions. They link into the tenant engine through the canonical gateway and preserve actor, site, delegation, operation, and deadline context.

#### 15.10.4 Mandatory UX patterns

The following patterns are required defaults, based on established enterprise interaction practice and the UI pattern catalog:

- **Dashboard:** design each dashboard around one user goal; put urgent/actionable items first; use cards only for distinct concepts; use lists for homogeneous records; provide drill-down.
- **Progressive disclosure:** show the common path first; place advanced model, provider, retention, NATS, and policy options behind labeled “Advanced” sections with concise explanations.
- **Good defaults:** preselect safe, common options; never default to destructive, expensive, public, or broad-permission settings.
- **Wizard:** use 3–7 steps for site creation, package activation, provider setup, deployment, and offline enrollment; preserve entered data, show progress, validate before advancing, and allow back navigation.
- **Input feedback:** validate on blur and submit; show inline actionable errors; preserve values; distinguish validation, permission, conflict, timeout, offline, and dependency failures.
- **Autosave:** use for drafts and configuration editors only when invalid data is not persisted; show `Saving`, `Saved`, `Unsaved changes`, and `Failed to save`; retain explicit Save for high-impact activation.
- **Blank slate:** explain what will appear, why it matters, and provide one primary call to action. Never show an unexplained empty table.
- **Tables:** keep filtering and sort visible, show result counts, preserve state in typed URL search parameters, support clear-all, and use server-side pagination.
- **Tabs:** use tabs only for peer views in one mental model; keep labels short; avoid nested tabs; use a stepper or sections for sequential setup.
- **Inline help:** explain why a setting matters and what a safe choice does; link to deeper documentation without blocking the task.
- **Confirmation:** reserve modal confirmation for irreversible/high-impact actions; show exact target, scope, values, consequences, and required recent-auth/approval.
- **Feedback:** every command produces visible accepted/completed/rejected/conflict/failed status with a recovery action and operation link.
- **Focus and navigation:** preserve focus after dialogs, mutations, route changes, and validation errors; support keyboard navigation and deep links.

The UI must prefer a calm hierarchy: one primary action per view, one secondary action group, and advanced actions in an overflow or details panel. More capability must not mean more simultaneous controls.

#### 15.10.5 Accessibility and inclusive design standard

Kora targets WCAG 2.2 AA and follows the WAI-ARIA Authoring Practices for tabs, dialogs, accordions, menus, comboboxes, grids, and disclosure controls. This includes keyboard operation, visible focus, semantic labels, focus trapping/restoration in dialogs, status announcements, reduced motion support, sufficient contrast, text alternatives, target size, and responsive zoom/reflow. Accessibility is tested with automated checks and human keyboard/screen-reader review; it is not satisfied by adding ARIA labels after implementation.

#### 15.10.6 Simplicity and responsive behavior

Every page must define:

```text
primary user goal
primary action
secondary actions
empty state
loading state
error/recovery state
permission-denied state
offline state
mobile/tablet layout
keyboard/focus behavior
```

Desktop uses a stable shell with a compact navigation rail/sidebar and a focused content region. Tablet collapses navigation and preserves task context. Mobile prioritizes one-column task flows, sticky primary actions, readable tables/cards, and bottom-sheet or full-screen details. Technical diagnostics are secondary panels, not the default page content.

No page may require the user to understand event buses, query caches, package manifests, or NATS to complete a normal business task.

#### 15.10.7 Visual builder experience

The page builder is a first-class Administration capability for composing workspaces, dashboards, lists, registers, forms, and custom pages. It must feel like arranging a real application, not configuring a serialized document. The default workflow is:

```text
Choose a goal → choose a data source → start from a good layout → add blocks
→ configure in context → verify at each device size → preview → publish
```

The builder must keep the user oriented at all times:

- the canvas shows the actual page hierarchy and a stable reading order;
- the selected component is visibly connected to its tree item and inspector;
- empty regions explain what belongs there and offer valid next components;
- the inspector shows only properties valid for the selected component and current context;
- changes are reflected immediately in the canvas and are marked `Unsaved`, `Saving`, `Saved`, or `Failed`;
- the preview link opens the same draft version with the same route, permissions, responsive behavior, and runtime states;
- publishing is a deliberate step with a preflight checklist, immutable version, and rollback path.

The builder should use progressive disclosure: common layout, content, binding, and action controls are visible first; advanced rules, responsive overrides, permissions, caching, offline policy, and event behavior are grouped behind clear sections. It must support keyboard-only operation, focus restoration, accessible drag-and-drop alternatives, and a mobile/tablet preview without requiring the user to understand CSS grid, React, JSON, or NATS.

The builder is not required to support pixel-perfect freeform design. Kora values deterministic, responsive business layouts that render consistently across browsers and devices. Any component that needs bespoke positioning must declare its constraints and fallback behavior in its registry contract.

#### 15.10.8 Simple setup and direct customization

Every important setup flow must begin with a working result, not an empty configuration screen. A first-time POS setup uses a short guided flow with one decision per step:

```text
1. Tell us what you sell
2. Add or choose your products
3. Choose how customers pay
4. Check your receipt and business details
5. Start selling
```

The flow uses good defaults, sample data, plain-language questions, and a visible completion meter. Optional settings such as tax rules, receipt numbering, printer configuration, staff permissions, and offline policy are available after the first successful sale or behind an explicit “More settings” step. Users can go back without losing work. Setup may be resumed later, and each step explains why the information is needed in one short sentence.

If the deployment supports offline POS, setup includes a simple, opt-in choice such as `Keep selling when the internet is down`. The supporting copy is direct: `Your sales stay on this device and sync when you reconnect.` The user does not need to understand operation logs, cursors, tombstones, branches, or schema versions. Kora shows whether the device is `Ready for offline use`, `Syncing`, `Up to date`, or `Needs attention`, and explains what the user can do next.

The POS starter screen must be ready to use immediately after setup. Its default arrangement is a clear, responsive workspace containing product categories, product cards, the current cart, totals, payment actions, and receipt/transaction status. The same screen adapts predictably: a two-area register on wide screens, a focused one-column flow on phones, and an explicit tablet arrangement. The user may choose `Auto`, `Portrait`, or `Landscape` where the device supports orientation control; changing orientation reflows the same cards into the defined layout rules and never creates a new or random arrangement.

Customization is direct and reversible:

- users select a visible card or section in the real screen and choose `Edit`, `Move`, `Resize`, `Hide`, `Duplicate`, or `Remove`;
- moving a card shows valid drop locations and a clear insertion preview; the saved order is deterministic;
- resize uses simple choices such as `Small`, `Medium`, `Wide`, or `Full width`, not CSS/grid terminology;
- the user can add a block from a short, searchable list of useful choices such as `Product list`, `Cart`, `Totals`, `Payment`, `Recent sales`, and `Notes`;
- every change appears immediately in the screen, can be undone, and can be reset to the recommended arrangement;
- advanced data bindings, actions, visibility rules, and responsive overrides stay behind “More options” and are explained in user language;
- a persistent `Preview` action shows the screen as a cashier or customer will see it, while `Save draft` and `Publish` make the change lifecycle clear.
- when offline POS is enabled, the screen keeps approved local work available, labels queued sales clearly, and never presents a locally queued sale as centrally completed until sync is accepted;
- when connected, live product, stock, payment, task, and notification changes appear without a manual refresh, with a compact connection indicator and an accessible notification center.

There must be no dead-end blank canvas, unexplained JSON, or requirement to understand a component tree before a user can make a useful change. When a choice is unavailable, the UI explains what is missing and offers the next action, for example: “Add a product before you arrange the Product list.”

#### 15.10.9 Product language and interaction copy

User-facing copy must be short, concrete, consistent, and written at approximately a grade-seven reading level. Prefer the user's task and outcome over implementation language:

| Avoid in normal UI | Use instead |
|---|---|
| View / manifest | Screen |
| Component | Block or card |
| Layout configuration | Arrange your screen |
| Bind data | Choose what to show |
| Execute action | What should happen |
| Publish manifest | Make this screen live |
| Invalid configuration | Something needs fixing |
| Permission denied | You do not have access to this yet |
| Validation failed | Check this before continuing |
| Delete component | Remove card |

Buttons describe the outcome: `Start selling`, `Add product`, `Arrange screen`, `Save draft`, `Make screen live`, `Try again`, and `Keep editing`. Errors name the problem and the next step; they never blame the user or expose codes. Empty states show an example, explain why the area matters, and offer one primary action. Autosave and realtime status use human feedback such as `Saving`, `Saved just now`, `Offline — changes will sync`, and `Connection restored`.

The same concept must use the same word everywhere. The POS user sees `Screen`, `Card`, `Arrange`, `Preview`, `Save draft`, and `Make live`; administrators may see the underlying manifest and component terms only in advanced tooling, documentation, and diagnostics.

### 15.11 Frontend data-flow and runtime integration

The frontend follows the RFC’s canonical path:

```text
user action or route
  → authenticated Kora session and server-resolved site
  → typed query/command contract
  → API/engine gateway with deadline, correlation ID, and idempotency key
  → SQL-authoritative result or accepted operation
  → TanStack Query cache/update
  → typed realtime invalidation or operation progress
  → visible completed/rejected/conflict/failed state
```

The browser never treats NATS, KV, Object Store, or a client cache as authoritative business state. All commands use the canonical executor and backend authorization from §§7, 10.4, and 16.2. The frontend may render capability hints, but the server decides permissions, field visibility, approval, workflow transitions, and offline eligibility.

#### 15.11.1 Realtime/WebSocket/SSE contract

The browser connects only to an authenticated realtime gateway over WebSocket or SSE. The gateway translates typed Kora events into a small frontend protocol:

```json
{
  "type": "resource.invalidated",
  "version": 1,
  "site": "server-bound",
  "resource": "sales-invoices",
  "scope": {"query_key_hash": "..."},
  "operation_id": null,
  "correlation_id": "..."
}
```

Allowed browser messages are typed subscription, resume, ping, and acknowledge messages. The browser cannot supply arbitrary NATS subjects, tenant/site scope, actor identity, commands, or render instructions. The gateway derives subscriptions from authenticated context and package capabilities.

Realtime is an optimization and coordination channel, not the source of truth. On connect, reconnect, resume failure, missed sequence, or authorization change, the client refetches authoritative queries. Events carry bounded sequence/cursor metadata; clients deduplicate by event ID and ignore stale invalidations. Realtime updates are coalesced per tenant/resource and must not cause unbounded render or refetch loops.

Required connection states are `connecting`, `connected`, `degraded`, `reconnecting`, `offline`, `unauthorized`, and `closed`. The UI shows a compact status indicator with details available on demand. Reconnect uses bounded exponential backoff with jitter, respects page visibility and network state, and never extends the original command deadline.

For live POS and workspace pages, the preferred browser transport is an authenticated WebSocket. SSE is an allowed fallback for read-only event delivery when WebSocket is unavailable. Both transports use the same typed event contract and authoritative-refetch rules. The browser never connects directly to NATS and never chooses raw subjects; the gateway derives subscriptions from the authenticated user, site, branch, package, page capabilities, and permitted resources.

The WebSocket protocol must support:

- authenticated connect and scoped subscription negotiation;
- connection heartbeats, bounded reconnect with jitter, and resume from the last acknowledged event cursor;
- event IDs, sequence/cursor metadata, deduplication, stale-event rejection, and missed-event recovery;
- typed resource invalidations and patches for products, stock, transactions, payments, workflow tasks, sync state, and operational health;
- typed operation progress for queued, syncing, accepted, completed, failed, conflict, cancelled, and expired operations;
- typed notification events with severity, title, short message, timestamp, related record/operation, read state, and an allowed action;
- notification acknowledgement/read state through canonical commands, with idempotency and server authorization;
- authoritative refetch after reconnect, resume failure, missed events, permission changes, or a terminal operation event.

Realtime notifications are user-facing feedback, not a second business state store. Toasts are reserved for timely, low-risk updates; important items also appear in the notification center and the relevant task, record, or sync view. Notifications must be deduplicated, scoped to the current tenant/site/branch, safe to display offline, and redacted according to the event payload classification. They must never contain secrets, raw provider payloads, arbitrary HTML, executable instructions, or an implied business completion that the server has not confirmed.

#### 15.11.2 Offline and synchronization contract

Offline support is capability-based, not an implicit promise that every page works offline. Each manifest declares `offline: unsupported|read_only|queue_writes|full_slice` and the exact resources/actions allowed.

The POS starter is a supported `full_slice` candidate when the backend advertises the required capability. Its offline contract includes the product catalog snapshot, permitted prices and tax rules, cashier/session context, cart and payment workflow, device/branch identity, approved commands, operation queue, sync cursor, and conflict records. The page must make the supported boundary clear: a cashier can continue approved work, but cannot invent new server capabilities or assume that an offline operation is complete centrally.

The standalone UI uses a service worker for versioned assets and IndexedDB for bounded, encrypted-at-rest-where-supported offline data, operation queues, cursors, tombstones, schema bundles, and conflict records. Local storage is not used for structured business data or secrets. Background sync is an optimization; foreground retry is always available.

Offline flow:

```text
online snapshot + compatible schema
  → explicit device/branch enrollment and scoped credential
  → local read/cache and approved local mutation
  → durable operation queue with operation_id/base_version
  → offline/conflict status visible to user
  → reconnect/authenticate/schema check
  → idempotent push and cursor pull
  → accepted/rejected/conflict result
  → user-guided retry/merge/manual review/discard
```

The UI must never claim a business operation is complete before central acceptance. It displays `Queued`, `Syncing`, `Accepted`, `Conflict`, `Rejected`, or `Needs review`. A conflict view shows the local operation, server state, reason, available resolution, and whether authorization is required. Discard removes only the local effect and retains the auditable conflict as defined in §12.4.

Offline data is scoped to user, site, branch, device, package, and schema version; it has quotas, expiry, revocation behavior, and a clear “remove local data” action. Sensitive fields and credentials are excluded unless explicitly allowed by policy. Logout, device revocation, tenant suspension, schema incompatibility, or package retirement blocks further offline writes and triggers controlled cleanup.

When connectivity returns, the POS UI visibly moves through `Syncing`, `Up to date`, or `Needs attention`. It uses the shared sync coordinator and the backend’s cursor-based push/pull protocol; pages must not create their own retry or reconciliation loops. Rejected sales or stale updates remain visible as conflicts with a plain-language explanation and a safe next step. The WebSocket reconnect path and offline sync path converge on the same authoritative queries and operation records so the user does not see contradictory states.

### 15.12 Frontend deployment modes

Kora supports one frontend artifact with deployment-specific runtime configuration. A separate frontend instance is allowed and preferred for Cloud when independent scaling, CDN caching, release cadence, or tenant isolation requires it.

| Mode | Frontend deployment | Runtime behavior |
|---|---|---|
| Embedded OSS | Go serves the built Vite assets | Same React app; runtime config binds the local Kora API and site context. No NATS credentials reach the browser. |
| Standalone OSS | Static assets served by any compliant web server/CDN | Authenticated API/realtime endpoints are supplied by signed or authenticated runtime config; CORS, cookie, CSRF, and origin policy are explicit. |
| Cloud shared UI | One versioned static frontend deployment serving many tenants | Tenant/site context is resolved server-side; package/manifest capabilities are scoped per environment; no tenant data is cached across scopes. |
| Cloud dedicated UI | Separate frontend instance per organization/site/region when required | Uses the same artifact and contracts with isolated runtime config, cache namespace, deployment, CSP, and release/rollback controls. |

The frontend must not fork business behavior between modes. Only API base URL, realtime endpoint, package capability snapshot, feature status, region, and deployment metadata vary. A deployment becomes usable only after runtime config signature/origin verification, health checks, package compatibility, and authenticated read-only API/realtime checks pass.

### 15.13 Frontend standards and references

The implementation follows these external standards and practices:

- WCAG 2.2 AA for accessibility.
- WAI-ARIA Authoring Practices for interactive widget behavior.
- HTML semantics, keyboard/focus conventions, and progressive enhancement practices.
- HTTP caching and conditional requests using `ETag`/`If-Match` as defined in §16.1.
- WebSocket/SSE browser transport with an application-level typed protocol, reconnect, resume, deduplication, and authoritative refetch rules defined in §15.11.1.
- PWA/service-worker and IndexedDB patterns for offline assets/data, with bounded storage, schema migration, explicit eviction, and foreground fallback as defined in §15.11.2.
- The established UX patterns in §15.10: Dashboard, Cards, Progressive Disclosure, Good Defaults, Input Feedback, Autosave, Blank Slate, Wizard, Inline Help, Tables, Tabs, Confirmation, and Feedback.

These references guide implementation but do not replace Kora’s contracts. If a library or browser capability cannot meet the security, accessibility, offline, or data-authority rules in this RFC, the Kora rule takes precedence.

### 15.14 Frontend component inventory

The frontend must build a small, composable component system. Components are grouped by responsibility and consumed by generated pages, feature pages, and application packages. Components do not fetch data or make authorization decisions; they receive typed data, query state, and callbacks from feature or manifest-runtime adapters.

#### 15.14.1 Application shell components

Required platform components:

```text
AppShell
AuthenticatedShell
PublicShell
ConsoleShell
Sidebar / NavigationRail
MobileNavigation
Breadcrumbs
PageHeader
PageContainer
CommandPalette
GlobalSearch
UserMenu
SiteSwitcher
EnvironmentSwitcher
CapabilityStatus
ConnectionStatus
NotificationCenter
HelpLauncher
ErrorBoundary
RoutePending
NotFound
PermissionDenied
UnsupportedCapability
```

The shell owns navigation, responsive layout, session presentation, site/environment context, global status, and error boundaries. It does not own business data or feature-specific queries.

#### 15.14.2 Data display components

Required reusable data components:

```text
ResourceState
LoadingState
EmptyState
ErrorState
OfflineState
ConflictState
PermissionState
DataTable
DataTableToolbar
ColumnPicker
FilterBuilder
FilterChips
SortControl
Pagination
CursorPagination
ListView
CardGrid
StatCard
MetricCard
Chart
Timeline
ActivityFeed
StatusBadge
ProgressBar
HealthSummary
AuditTrail
RecordSummary
RelatedRecords
AttachmentList
```

Every data component must support the states relevant to its contract: loading, empty, error, permission denied, stale, offline, conflict, and refreshing. `DataTable` must support server-side cursor pagination, visible filters, sort indicators, result counts, column visibility, keyboard navigation, responsive fallback, and a meaningful blank slate.

#### 15.14.3 Form and mutation components

Required schema-driven form components:

```text
Form
FormSection
FormField
FieldLabel
FieldDescription
FieldError
TextInput
NumberInput
MoneyInput
DateInput
DateRangeInput
Select
Combobox
MultiSelect
Checkbox
Switch
RadioGroup
FileUpload
RichTextInput
CodeEditor
ChildTableEditor
ComputedField
FieldPermissionState
FormActions
SaveState
UnsavedChangesGuard
ConflictResolver
ApprovalDialog
RecentAuthPrompt
DestructiveActionDialog
```

Forms derive field type, labels, required state, visibility, editability, validation, and data classification from canonical doctype/command schemas. The UI may hide a field for usability, but only the backend may enforce field permission. Form components must expose typed values and validation results rather than submitting raw arbitrary objects.

#### 15.14.4 Workflow, operation, and offline components

Required components for durable runtime behavior:

```text
WorkflowStepper
WorkflowActionMenu
OperationStatus
OperationProgress
ApprovalQueue
TaskQueue
RetryAction
CancelAction
SyncStatus
OfflineBanner
OfflineQueue
SyncQueueItem
SyncConflictList
SyncConflictDetail
LocalDataSummary
DeviceEnrollment
DeviceRevocation
ReconciliationResult
```

Long-running commands must render `accepted` separately from `completed`. The user must be able to inspect operation status, retry only when the server marks the operation retryable, cancel where supported, and navigate to the resulting record or conflict.

#### 15.14.5 Manifest components

The initial manifest registry must provide safe presets and primitives:

```text
WorkspacePage
DashboardPage
ListPage
RecordPage
WorkflowPage
PublicFormPage
CustomPage

Stack
Grid
Section
Tabs
Accordion
Card
Text
Heading
Markdown
DataTable
Chart
Metric
RecordLink
Form
ActionButton
ActionMenu
ResourceBoundary
RealtimeBoundary
OfflineBoundary

ViewBuilder
BuilderCanvas
ComponentPalette
ComponentTree
PropertyInspector
BindingPicker
ActionPicker
ResponsivePreview
DraftStatus
PublishPreflight
```

Each component declares its prop schema, data-binding schema, supported states, accessibility behavior, permissions/capability requirements, offline behavior, and responsive behavior. A component cannot introduce a new query, command, URL, HTML renderer, or realtime subscription through untyped props.

#### 15.14.6 Component acceptance requirements

Every reusable component must have:

- a typed props contract;
- loading, empty, error, permission, and offline behavior where relevant;
- keyboard and screen-reader behavior;
- responsive behavior at desktop, tablet, and mobile widths;
- visual states for default, hover, focus, disabled, pending, success, warning, and failure;
- a unit test and an interaction test;
- a manifest registration entry if it is package-consumable;
- no direct network access, NATS access, browser storage of secrets, or authorization logic.

### 15.15 Frontend data-fetching and state patterns

The frontend uses one data-access model. Server state is fetched through the API/engine client and managed by TanStack Query. Local UI state is managed by component state or bounded Zustand stores. There is no third data cache.

#### 15.15.1 Query architecture

Every query is defined in `api/queries/` as a typed option factory:

```ts
const recordQuery = (input: RecordQueryInput) => queryOptions({
  queryKey: queryKeys.records.detail(input),
  queryFn: ({ signal }) => api.records.get(input, { signal }),
  staleTime: 30_000,
})
```

Feature components consume the factory through `useQuery`, route loaders, or prefetch helpers. They do not write query keys or fetch functions inline.

Every query definition must specify:

```text
input schema
query-key factory
request deadline
stale time
cache time
retry policy
offline policy
authorization scope
redaction/data-classification policy
invalidation tags
```

Query keys must include all result-affecting context:

```text
[site, packageVersion, resource, queryName, normalizedInput, schemaVersion]
```

The site and tenant context must come from authenticated runtime context, not a free-form component parameter. Query-key hashing must be deterministic and must not include secrets or full document bodies.

#### 15.15.2 Query categories

The implementation must use these query categories:

| Category | Pattern | Example |
|---|---|---|
| Identity/context | Short-lived query, centrally invalidated on session/site change | Current user, site, capabilities, package snapshot |
| Reference data | Long stale time, explicit invalidation on config change | Doctype schema, roles, navigation, statuses |
| Lists | Cursor pagination, server filtering/sorting, URL-preserved view state | Records, users, tasks, audit events |
| Details | Record/version-aware query, related-resource prefetch | Sales invoice, workflow instance |
| Dashboards | Parallel bounded queries or one typed aggregate query | KPIs, health, usage summary |
| Operations | Poll or realtime-resume query until terminal status | Deployment, import, agent run, sync job |
| Offline | Local snapshot query with server reconciliation metadata | Cached records, queued operations, conflicts |
| Realtime | Subscription-driven invalidation followed by authoritative refetch | Record list, task queue, health state |

#### 15.15.3 Fetching rules

- Use `AbortSignal` for every request and propagate the original deadline.
- Use `ETag`/`If-None-Match` for cache validation and `If-Match`/record version for mutations.
- Use opaque cursors; never calculate or expose database offsets as business cursors.
- Keep filters, sorting, view mode, and page size in typed URL search parameters for shareability and back/forward behavior.
- Prefetch only predictable next-step data and only after authorization/capability checks.
- Use bounded retries with jitter only for retryable dependency/network errors. Never retry permission, validation, conflict, or non-idempotent mutation failures automatically.
- Query errors must preserve stable server error codes and expose an actionable recovery path.
- Dependent queries use explicit `enabled`/dependency conditions and never execute with incomplete site, identity, or package context.
- Lists must use virtualization only when measured row counts require it; virtualization must preserve keyboard navigation and screen-reader semantics.
- Sensitive data must use memory-only cache policy unless the contract explicitly permits encrypted offline persistence.

#### 15.15.4 Mutation architecture

Every mutation is defined in `api/commands/` as a typed command factory. It must create or receive an idempotency key, correlation ID, deadline, and concurrency token:

```text
form/feature action
  → typed command input validation
  → server idempotency key
  → authenticated API client
  → accepted/completed/conflict/rejected/failed result
  → cache update or invalidation
  → visible feedback and operation link
```

Mutation hooks must define:

```text
input schema
command type/version
idempotency behavior
deadline
concurrency token
optimistic-update policy
success invalidations
conflict behavior
offline eligibility
retry policy
```

Optimistic updates are forbidden for financial, payment, stock, permission, workflow, deletion, external-send, and other irreversible effects. Those operations show pending/accepted state until the authoritative response arrives.

#### 15.15.5 Invalidation and cache updates

Cache updates follow this priority:

1. Apply the authoritative mutation response when it contains the complete updated resource.
2. Otherwise invalidate the exact affected detail and list queries.
3. Invalidate related aggregates only when the command contract declares the relationship.
4. Never invalidate the entire cache as a default response to one mutation.

Realtime events use the same invalidation registry. An event identifies a typed resource and scope; it does not provide arbitrary query keys. The registry maps it to affected queries, coalesces bursts, and refetches only when the page is visible or the data is marked important.

#### 15.15.6 Realtime and long-running operation fetching

For accepted commands, the frontend uses an operation query:

```text
submit command → receive operation_id
  → subscribe to typed operation events
  → update operation query/cache
  → refetch authoritative result on terminal event
  → show completed, failed, cancelled, conflict, or expired state
```

If realtime is unavailable, the operation query falls back to bounded polling. Polling stops at a terminal state, page unmount, authorization loss, or deadline. Reconnect always verifies state from the server rather than trusting the last browser event.

#### 15.15.7 Offline data-fetching pattern

Offline queries are read from a versioned local repository behind the same query interface:

```text
useQuery(queryOptions)
  → online: authenticated API/engine query
  → offline: permitted local snapshot
  → reconnect: server refresh + cursor reconciliation
```

Offline writes use a durable operation repository, not a TanStack Query mutation cache alone. Each queued item stores operation ID, command/version, site, branch, device, schema/package version, base version, payload classification, created time, attempt state, and last error. Payloads are encrypted/protected according to device policy and expire according to retention rules.

The frontend must expose a single sync coordinator responsible for authentication, schema compatibility, push ordering, retry backoff, duplicate acknowledgements, partial responses, conflicts, tombstones, and cleanup. Individual pages may enqueue an approved command but may not implement their own sync loop.

#### 15.15.8 Data patterns that must not be used

- Fetching directly in `useEffect` for normal server data.
- One-off query keys written inside pages.
- Global Zustand stores containing records, lists, permissions, or server responses.
- `localStorage` for credentials, structured business records, operation queues, or sensitive AI content.
- Infinite polling when a realtime/operation subscription or bounded refresh is available.
- Blind `invalidateQueries()` without a scoped key or declared invalidation tag.
- Automatic retry of mutations without idempotency protection.
- Treating cached or realtime data as proof of authorization or completion.
- Sharing query caches between tenants, sites, environments, packages, or user sessions.

## 16. Application and contract versioning

Every package has:

```yaml
name: erp.sales
version: 1.4.0
requires:
  kora: ">=1.0.0 <2.0.0"
  ui-runtime: ">=2.0.0 <3.0.0"
events:
  publishes:
    - type: kora.sales.order.submitted
      version: 1
```

Compatibility rules:

- Doctypes and fields are migrated forward through explicit migrations.
- Events are never changed in place; publish a new event version or a new event type.
- Consumers declare supported event versions.
- Commands are backward-compatible within a major version and reject unknown required fields.
- Page manifests are immutable and content-addressed by hash.
- Worker consumer names include contract major, e.g. `analytics-v1`.
- Stream retention must cover the maximum supported replay window for projections.
- Package activation is staged: validate → migrate → publish config pointer → warm workers → activate.

### 16.1 HTTP API contract

All API responses use this envelope:

```json
{
  "data": {},
  "operation_id": "01J...",
  "correlation_id": "req-123",
  "status": "completed",
  "error": null,
  "meta": {"next_cursor": null}
}
```

Errors use `{code, message, fields, retryable, details}`. List APIs use opaque cursor pagination with `limit`, `next_cursor`, and `has_more`; filters and sorting are validated against the doctype/query schema. `ETag` and `If-Match` protect record updates. Bulk commands return per-item results and never hide partial failure. Long-running operations expose `GET /operations/{id}` and emit `operation.completed` or `operation.failed` events.

### 16.2 Permission precedence

```text
NATS authorization → infrastructure subject isolation
Kora backend authorization → authoritative business access
Frontend permissions → visibility/usability hints only
```

A frontend permission can hide or disable a control but can never grant access. Every command is re-authorized at the backend using the authenticated actor, site, role, record, field, and operation context. A denied backend command returns `PERMISSION_DENIED` regardless of manifest configuration or NATS identity.

## 17. Authentication and security model

### 17.1 Current authentication status

The current Kora runtime supports only these first-party methods:

```text
password     email + password against _kora_user
magic_link   email magic link with a short-lived verification token
```

They are exposed through the existing session routes and `/api/auth/providers`. The provider list is currently hardcoded and there is no implemented OIDC, OAuth2, SAML, LDAP, SCIM, WebAuthn/passkey, or social-provider integration. The `golang.org/x/oauth2` dependency alone does not constitute provider support.

Until the gates in §19 pass, new providers are `planned` or `experimental`; they must not be advertised as supported merely because the UI can render a provider button.

### 17.2 Provider-neutral authentication architecture

Authentication establishes a principal. Authorization remains Kora's kernel responsibility. Provider adapters may authenticate or provision identities, but they may not assign Kora roles, bypass site isolation, execute business commands, or decide workflow permissions.

```text
browser/mobile/external client
  → auth discovery: configured public provider metadata
  → provider adapter: challenge/callback/token verification
  → normalized identity assertion
  → identity resolver: tenant/site policy + account-link policy
  → user/account reconciliation transaction
  → Kora session or scoped channel session
  → kernel authorization and normal workflows
```

The canonical external identity key is `provider_instance_id + issuer + subject`. Email is an attribute, not an identity key. Email-based linking is disabled by default and is allowed only when the provider supplies a verified email, tenant policy permits it, and the user completes a recent-authenticated account-link flow. Never merge accounts solely because lower-cased email addresses match.

Provider configuration is scoped to an organization/environment/site and contains only non-secret metadata in the public discovery response:

```yaml
auth_provider:
  id: entra-main
  kind: oidc
  enabled: true
  display_name: Microsoft Entra ID
  issuer: https://login.microsoftonline.com/<tenant>/v2.0
  client_id: public-client-id
  secret_ref: secret://auth/entra-main/client-secret
  scopes: [openid, profile, email]
  allowed_domains: [example.com]
  allowed_groups: []
  role_mapping_ref: auth/entra-main/roles-v1
  provisioning: just_in_time
  account_linking: verified_email_plus_recent_auth
  data_residency: eu-west
```

Secrets are stored through the existing secret store or Cloud secret manager. They never appear in provider discovery, browser configuration, logs, traces, manifests, sessions, or identity records.

### 17.3 Provider families and required profiles

Kora uses one provider registry and adapter contract for all provider families. OIDC is the preferred integration for enterprise identity providers; provider-specific SDKs are allowed only inside adapters.

| Family | Examples | Required protocol and policy |
|---|---|---|
| Local password | Kora built-in | Argon2id or the approved password scheme, breach/rate-limit controls, password reset, session revocation, and audit. |
| Magic link | Kora built-in | Single-use hashed token, short expiry, anti-enumeration response, redirect binding, replay prevention, and audit. |
| OIDC | Microsoft Entra ID, Google Workspace, Okta, Auth0, Keycloak, GitHub | Authorization Code + PKCE, issuer discovery, exact redirect URI, nonce/state/PKCE verification, JWKS rotation, issuer/audience/azp validation, verified claims, and provider logout policy. |
| OAuth2 login | Providers without OIDC discovery | Authorization Code + PKCE only. OAuth2 alone does not define identity; the profile must declare a trusted user-info endpoint, claim mapping, issuer/account binding, and verification policy. Implicit and password grants are prohibited. |
| SAML 2.0 | Enterprise IdPs and legacy federations | Signed AuthnRequest/Response, metadata validation, audience/recipient/ACS checks, clock-skew policy, certificate rotation, replay cache, encrypted assertions where required, and explicit NameID mapping. |
| LDAP/Active Directory | Customer-managed directory | TLS/LDAPS, certificate validation, bounded search base/filter, bind identity rotation, connection/time limits, group-to-role mapping, fail-closed outage behavior, and no password persistence. |
| WebAuthn/passkeys | Platform authenticators/security keys | RP ID/origin binding, challenge expiry, user verification policy, counter/clone detection, recovery flow, revocation, and credential-management audit. |
| Social login | Google, Microsoft, Apple, GitHub, etc. | Implement through OIDC where available; use provider-specific OAuth only with explicit verified identity claims and consumer-account linking policy. |
| Custom provider | Customer or extension provider | OIDC profile first. A custom adapter must implement the normalized contract, pass security tests, declare data handling, and be signed/approved before activation. Arbitrary callback scripts and client-supplied identity claims are prohibited. |
| Provisioning | SCIM 2.0, directory sync, admin import | Provisioning is separate from authentication. SCIM bearer tokens are hashed/rotated, operations are idempotent, deprovisioning disables access and revokes sessions, and role assignment remains policy-controlled. |

Provider profiles must declare issuer/endpoint metadata, protocol, supported flows, claim mappings, subject stability, email verification semantics, tenant binding, scopes, data residency, retention, logout behavior, JIT/SCIM provisioning, role/group mapping, rate limits, timeout, and key/certificate rotation behavior.

### 17.4 Normalized provider contract

```go
type AuthProvider interface {
    Discovery(ctx context.Context, req DiscoveryRequest) (DiscoveryResponse, error)
    Begin(ctx context.Context, req AuthBeginRequest) (AuthChallenge, error)
    Complete(ctx context.Context, req AuthCallbackRequest) (IdentityAssertion, error)
    Revoke(ctx context.Context, req RevokeRequest) error
    Health(ctx context.Context) (ProviderHealth, error)
}

type IdentityAssertion struct {
    ProviderInstanceID string
    Issuer             string
    Subject            string
    Email              string
    EmailVerified      bool
    FullName           string
    Claims             json.RawMessage
    AuthenticatedAt    time.Time
    Amr                []string
    Acr                string
    SessionBinding     string
}
```

Adapters must normalize provider errors to stable codes such as `AUTH_CANCELLED`, `INVALID_STATE`, `INVALID_NONCE`, `INVALID_ISSUER`, `INVALID_AUDIENCE`, `KEY_ROTATION_FAILED`, `IDENTITY_UNVERIFIED`, `ACCOUNT_LINK_REQUIRED`, `PROVIDER_UNAVAILABLE`, `DIRECTORY_TIMEOUT`, and `AUTH_POLICY_DENIED`. The adapter owns protocol validation; the identity resolver owns Kora account mapping; the kernel owns authorization.

### 17.5 Identity, session, and reconciliation data

The existing `_kora_user` and `_kora_session` tables remain compatible records, but federated identity must not be encoded only in email or session JSON. Add migrations for:

```text
_kora_auth_provider
  id, organization_id, environment_id, site, kind, config_version,
  public_config, secret_ref, enabled, policy_version, created_at, updated_at

_kora_auth_identity
  id, provider_instance_id, issuer, subject, user_name, email_snapshot,
  email_verified, claims_hash, last_authenticated_at, disabled_at, created_at

_kora_auth_link_request
  id, user_name, target_identity, requested_by, auth_session_id,
  state_hash, expires_at, status, decided_at

_kora_auth_attempt
  id, provider_instance_id, site, channel, state_hash, nonce_hash,
  pkce_hash, redirect_uri, started_at, completed_at, outcome, error_code

_kora_auth_event
  id, site, principal_id, subject_user_id, provider_instance_id,
  event_type, correlation_id, metadata, occurred_at

_kora_auth_provisioning_cursor
  provider_instance_id, cursor, version, last_success_at, last_error
```

Provider state (`state`, nonce, PKCE verifier, SAML request ID, replay key) is short-lived, single-use, stored hashed where possible, bound to site/provider/redirect/session, and never trusted from the callback alone. Identity reconciliation is one transaction: resolve the canonical identity, create or update the local user under policy, apply only approved profile fields, record the identity link and auth event, create/revoke the Kora session, and publish an outbox event. A failed transaction creates no session and no partial identity link.

JIT provisioning may create a disabled or pending user when policy requires approval. SCIM/directory deprovisioning disables the user, revokes all sessions/channel sessions, invalidates cached authorization, cancels or pauses sensitive pending approvals according to workflow policy, and emits an audit event. Provider claim changes never silently overwrite administrator-owned fields or roles.

### 17.6 Authentication flows and endpoints

The stable public contract is:

```text
GET  /api/auth/providers
GET  /api/auth/providers/{id}/authorize
GET  /api/auth/providers/{id}/callback
POST /api/auth/providers/{id}/logout
POST /api/auth/link/{id}/begin
POST /api/auth/link/{id}/complete
POST /api/auth/sessions/revoke-all
GET  /api/auth/me
```

`/api/auth/providers` returns only enabled, policy-eligible public metadata and capability flags. Authorize/callback endpoints validate site binding, exact redirect URI, state, nonce, PKCE, issuer, audience, signature, token expiry, and provider instance before reconciliation. Callback errors are opaque to prevent account enumeration; detailed errors go to correlation-linked audit logs.

Browser sessions use secure, HttpOnly, SameSite cookies, CSRF protection for state-changing requests, session rotation after login and privilege elevation, bounded lifetime, idle timeout where configured, and server-side revocation. Channel sessions are separate scoped bearer credentials with expiry, audience, permissions, revocation, and audit. A provider access token is never reused as a Kora session token.

Site binding is resolved from the authenticated server session, deployment routing, or verified provider configuration. It must not be accepted from callback query parameters, redirect state supplied by the client, channel payloads, or browser-controlled tenant fields. Callback state is an opaque, single-use server record bound to the provider, expected site, redirect URI, and initiating session.

### 17.7 Authentication and existing Kora workflows

Authentication does not replace Kora authorization. After login, every normal command, agent tool, workflow transition, offline operation, MCP call, and channel operation receives the normalized `ActorContext` from §7.3 and is re-authorized by the kernel.

- Role/group claims map through signed, versioned, deny-by-default policies bound to the organization/environment/site; provider claims never directly become arbitrary Kora roles. A mapping change requires integrity verification, audit, and policy-version capture on affected sessions and approvals.
- A role or account disable invalidates session/cache state and is checked again for pending approvals and delayed commands.
- Approval records bind to principal, subject user, provider-independent user ID, command fingerprint, record version, and policy version.
- External identity events are audit/outbox events and do not directly mutate financial, stock, payment, workflow, or schema state.
- Login, linking, unlinking, provisioning, deprovisioning, failed verification, session revocation, and provider configuration changes are observable and auditable.
- Offline credentials are separately scoped and may be issued only after local policy permits them; external provider tokens are never stored on branch devices.

### 17.8 Provider migration and reconciliation

Provider migration is a durable control-plane workflow, never a bulk email update:

```text
register provider → validate metadata/keys → dry-run claim mapping
  → discover candidate identities → review collisions
  → activate sign-in → reconcile JIT/SCIM changes
  → monitor failures → rotate/retire old provider
```

The reconciliation report must include linked, created, pending, disabled, conflicting, unverifiable, and unchanged identities. Conflicts are retained for review; they are never resolved by last-write-wins. During migration, existing password/magic-link users remain usable unless policy explicitly disables them. Provider retirement stops new authentication, preserves identity links and audit history, revokes provider sessions/tokens, and requires a recovery provider or approved account-recovery process before access is removed.

### 17.9 Authentication acceptance criteria

Authentication is production-supported only when:

1. Password and magic-link flows continue to pass existing session, CSRF, rate-limit, expiry, and revocation tests.
2. OIDC authorization-code + PKCE works with issuer validation, nonce/state, JWKS rotation, verified claims, logout, and failure recovery.
3. OAuth2-only, SAML, LDAP, WebAuthn, social, custom, and SCIM profiles are either implemented and gated by their profile tests or explicitly reported as `planned`.
4. The same canonical identity maps consistently across browser, API, MCP, channel, offline, and agent operations.
5. Account linking requires recent authentication, explicit user intent, verified identity policy, collision handling, and audit.
6. Provider failure, key rotation, callback replay, duplicate callback, timeout, clock skew, and revoked-session tests pass.
7. Disable/deprovision revokes sessions and cached authorization and applies the defined policy to pending approvals and delayed commands.
8. Cross-tenant provider configuration, identity lookup, callback, session, cache, and provisioning isolation tests pass.
9. Provider secrets and tokens do not appear in browser payloads, logs, traces, manifests, sessions, or identity records.
10. Migration produces a reconciliation report and never silently merges, overwrites, or deletes identities.

### 17.10 OSS and Cloud security model

#### OSS

- NATS credentials are server-side configuration.
- Browser clients use Kora sessions/tokens, never NATS credentials.
- Extension workers receive scoped credentials and subjects.
- Script execution remains sandboxed by existing Kora policy.
- Page manifests are validated against a component and query allowlist.

#### Cloud

- One NATS account per tenant or isolation domain.
- Separate operator, system, worker, extension, branch, and user credentials.
- Subject imports/exports are explicit and deny-by-default.
- TLS for client, cluster, and leaf connections.
- Per-tenant quotas for stream storage, consumers, messages, workers, and object storage.
- Audit credential issuance, package activation, page publication, and subject policy changes.
- Audit reads are operator/compliance-only, separately credentialed, and logged. `KORA_AUDIT` has an explicit retention class, access policy, backup policy, and deletion/legal-hold behavior.

Cloud production requirements:

- Tenant isolation is tested at SQL, NATS account, Object Store, KV, cache, logs, traces, metrics, and backup boundaries. A tenant identifier in an envelope is not sufficient isolation.
- Every secret and provider credential has an owner, purpose, scope, created/rotated/revoked timestamps, and emergency revocation path. Credentials are never persisted in prompts, manifests, messages, or traces.
- Data residency and provider routing policy are enforced before a request leaves the selected region. A provider profile that cannot satisfy residency or retention policy is ineligible, not merely lower priority.
- AI content, summaries, tool arguments, retrieved documents, and attachments have retention classes and deletion workflows. Deletion covers SQL, Object Store, KV checkpoints, caches, backups according to the declared legal/operational retention policy.
- Idempotency receipts and response envelopes have enforced expiry and access controls; retention and backup policy must not silently outlive the declared expiry for sensitive responses.
- Cloud defines RPO/RTO per deployment tier, backup frequency, restore verification, regional failover, NATS stream recovery, and the maximum accepted data-loss window. These values are configuration and acceptance-test inputs, not marketing claims.
- Suspension stops new work, prevents provider calls and external delivery, preserves auditable state, and defines how already-running operations are cancelled or drained. Tenant deletion is a durable workflow with a verified completion receipt.

## 18. Observability and operations

Every command, event, task, and actor message carries:

```text
message_id
correlation_id
causation_id
site
actor/worker identity
contract type/version
attempt
```

Required metrics:

- command latency and timeout rate;
- outbox age and publish failure rate;
- stream lag per consumer;
- redelivery count;
- dead-letter count;
- actor active count and lease churn;
- workflow duration by state;
- sync backlog and conflict count;
- page manifest load/error/cache rate;
- component capability mismatch count.
- AI run admission/rejection by reason;
- provider request latency, timeout, retry, rate-limit, safety-block, and fallback rate;
- input/output/cache/reasoning token usage and estimated/finalized cost;
- budget reservation conflicts, quota exhaustion, and cost-cap near misses;
- tool authorization denials, approval wait/expiry, recent-auth failures, and duplicate attempts;
- agent run duration, step duration, cycle termination, checkpoint recovery, and resume success;
- prompt/context size, compaction count, redaction failures, and content-retention deletion lag;
- tenant isolation, credential revocation, backup verification, and restore-test status.

Every operational metric is tagged only with bounded-cardinality dimensions. Raw prompts, tool arguments, document bodies, secrets, and full provider responses are not metric labels. Logs and traces use the same correlation/run/step/provider-attempt IDs, with content recording opt-in and redacted.

Required operational tools:

```text
kora events inspect
kora stream status
kora consumer lag
kora outbox retry
kora dead-letter replay
kora actor inspect
kora sync conflicts
kora page validate
kora package activate --dry-run
```

### 18.1 Implementation risk register

The following risks must be addressed in the indicated phase and covered by automated tests, metrics, or operational evidence. Severity is `critical`, `high`, `medium`, or `low`. Scope is `OSS`, `Cloud`, or `both`.

| ID | Risk | Severity / scope | Required specification or implementation change | Blocking gate |
|---|---|---|---|---|
| R-01 | Cross-tenant access through shared workers or credentials | critical / Cloud | A worker credential must be tenant-scoped. Shared pools require tenant-aware execution context, fair scheduling, per-tenant concurrency limits, and isolation tests. | §19.7, before Cloud `supported` |
| R-02 | Outbox event published under the wrong tenant account | critical / both | Resolve NATS account and credentials from each outbox row’s site. Never use a process-wide tenant/account setting. Add tenant A/B publish tests. | §19.7, before NATS production use |
| R-03 | Wildcard or unauthorized realtime/sync subjects | high / both | Strictly validate site, channel, branch, and direction. Browsers must never construct arbitrary subjects; gateways authorize each subscription and command. | §19.7 and §21.25 |
| R-04 | Sensitive tenant identifiers exposed in subjects | medium / Cloud | Use opaque subject prefixes or account boundaries where site identifiers are sensitive. Treat envelope site fields as metadata, never authorization. | §14.2.1 security review |
| R-05 | KV pointer poisoning redirects workers to weaker policy/config | high / both | Policy and configuration pointers must reference signed, immutable, content-addressed snapshots verified before activation. | §19.4 and package activation tests |
| R-06 | Full documents leak through event fan-out | high / both | Define event projections and field classifications. Full documents are opt-in and limited to explicitly authorized consumers; default events use minimized payloads. | Phase 1 schema freeze |
| R-07 | Rejected offline operations are silently deleted | high / both | `discard` may remove only the local business effect. The conflict, original operation, authorization, and resolution event remain retained and auditable. | §19.3, before offline `supported` |
| R-08 | Client timestamps influence business ordering | medium / both | `occurred_at` is audit/display metadata only. Server receipt time, aggregate version, and fencing/version checks are authoritative. | §19.3 |
| R-09 | OAuth callback or channel input changes site identity | high / both | Resolve site from authenticated server-side routing/session context. Never trust callback/query/channel site or user claims without verified binding. | §19.4 and §19.7 |
| R-10 | Role/group claims grant unintended privileges | high / both | Role mappings are signed, versioned, deny-by-default artifacts. Claims alone never create permissions. | §19.4 |
| R-11 | Approval authorizes different tool arguments than the user reviewed | critical / both | Approval fingerprints include the complete evaluated action, target, fields, values, site, and actor context. Any divergence returns to `pending_approval`. | §19.4 and §21.26–27 |
| R-12 | Spoofed channel identity executes as another user | critical / Cloud | Verify provider signatures/replay protection and re-establish authenticated channel identity for every run. Do not accept channel-supplied site/user authority. | §19.4 and §14.4.6 |
| R-13 | AI cost estimates under-admit expensive runs | high / both | Enforce hard per-run token, tool-call, round, payload, and wall-clock limits in addition to reservations. Reservations are admission bounds, not guarantees. | §19.5 |
| R-14 | AI content survives beyond its retention policy | high / Cloud | Define retention classes and deletion workflows across SQL, KV, Object Store, caches, logs, and backups. Verify deletion and legal-retention exceptions end to end. | §19.7 |
| R-15 | Manifest data rendering enables XSS or unsafe navigation | high / both | Sanitize Markdown/rich text by default, enforce URL policies and CSP, and prohibit raw HTML/executable bindings from manifests. | §21.8, §21.24 |
| R-16 | Runtime configuration redirects the UI to an untrusted service | medium / both | Sign runtime configuration or serve it through an authenticated endpoint; validate origin, tenant, package, and API bindings. | Phase 4 package tests |
| R-17 | Poison messages cause redelivery storms | medium / both | Configure exponential redelivery backoff, maximum deliveries, DLQ handling, and redelivery-rate alerts. | §19.2 and §18 metrics |
| R-18 | Outbox, projection, timer, or webhook contention limits throughput | high / both | Batch outbox/timer claims, add required composite indexes, micro-batch projections, and isolate webhook delivery lanes. Measure p95 latency and queue depth. | §19.1–2 and Phase 1 load tests |
| R-19 | Aggregate ordering creates hot-key serialization | medium / both | Measure per-key throughput and mailbox depth. Support domain-specific sharding where needed and document backpressure behavior. | §19.1 |
| R-20 | Shared worker pools create noisy-neighbor starvation | high / Cloud | Enforce per-tenant quotas, concurrency limits, fair scheduling, and tenant-level lag alerts. | §19.7 |
| R-21 | Cloud resource proliferation exceeds NATS/control-plane capacity | medium / Cloud | Establish tested limits for accounts, streams, consumers, KV buckets, and Object Store namespaces. Define tiered provisioning topology. | Phase 6 scale spike |
| R-22 | Sequential onboarding creates excessive provisioning latency | medium / Cloud | Parallelize independent provisioning steps while retaining resumable, idempotent jobs and dependency fencing. | Phase 6 onboarding metrics |
| R-23 | AI usage events become a second high-volume ingestion path | high / Cloud | Batch or roll up usage projections with bounded lag while retaining immutable provider-attempt records and replayable billing results. | §19.5 and §14.4.7 |
| R-24 | Budget counters and KV sessions become hot keys or grow without bound | medium / both | Use TTLs, size/history limits, quotas, bounded writes, and sharded reservations where measured. All KV state must be reconstructable from SQL/events. | §19.1, §19.5 |
| R-25 | Audit and JetStream recovery boundaries are undefined | high / Cloud | Define audit read credentials, retention classes, backup/restore drills, RPO/RTO, and projection rebuild verification. | §19.7 |
| R-26 | Leaf credentials remain usable after branch/device revocation | high / Cloud | Make leaf credentials short-lived, revocable, rotated, and audited; test already-connected and reconnecting devices. | §19.7 |

#### Risk acceptance rules

- A risk marked `critical` or `high` may not be waived by a feature flag when it affects tenant isolation, authorization, identity, credentials, or durable business effects.
- A deferred risk requires an owner, target phase, mitigation, and capability status of `planned` or `experimental`.
- Every resolved risk requires evidence in the phase record: contract/test reference, deployment mode, measured result, and residual limitation.
- A performance risk may be accepted only with measured limits, backpressure behavior, alert thresholds, and a documented scaling path.
- The implementation must update this register when a new trust boundary, persistence layer, adapter, or deployment topology is introduced.

## 19. Validation gates

The following gates are mandatory before generalizing the corresponding patterns:

1. **Actor fencing:** run a spike with lease expiry, late renewal, concurrent ownership, JetStream redelivery, and SQL compare-and-swap updates. Exit requires stale fencing tokens to be rejected for state, business effects, and outbox writes; duplicate delivery must be idempotent; and p95 transition latency must be measured. If fencing adds more than approximately 50 ms p95, synchronous workflows remain in the kernel and actors are limited to async-only workflows.
2. **Durable timers:** run a spike implementing the SQL timer table and scheduler described in §10. Exit requires restart-safe claiming and publication, idempotent `timer_id` delivery, bounded clock-skew behavior, and retry/dead-letter recovery. Do not generalize actors until this gate passes.
3. **Offline sync:** complete one POS or warehouse slice using `base_version`, tombstones, schema-version gates, conflict records, and partial-response recovery. Exit requires repeatable idempotent push/pull, explicit stale-write conflicts, safe schema rejection, and tombstone retention for the supported offline window. Do not generalize the sync model until this gate passes.
4. **AI authorization and approval:** prove that every adapter reaches the shared executor, permissions are checked per tool call, recent-auth is server-verified, and confirmation-required tools cannot execute without an actor-bound durable approval. The approval test must include complete argument/target fingerprints, record versions, delegation, policy changes, role changes, and channel identity changes while an approval is pending.
5. **AI provider and metering:** prove bounded provider timeouts, model/provider validation, fallback recording, usage persistence, budget reservations, cost reconciliation, and no provider call after a failed budget reservation.
6. **MCP readiness:** either keep MCP explicitly `experimental/validation-only`, or prove authenticated stdio/network execution, real command/query behavior, authorization, confirmation, idempotency, audit, and credential scoping. Do not advertise stubbed MCP as supported.
7. **Cloud isolation and recovery:** run cross-tenant SQL/NATS/KV/Object Store/cache/log/backup isolation tests, opaque-subject and gateway subscription tests, per-row outbox account-binding tests, shared-worker fairness tests, already-connected leaf credential revocation tests, audit access/retention tests, deletion tests, restore tests, and regional/service restart tests against declared RPO/RTO targets.
8. **Contract parity:** generate schemas from the canonical registry and compare chat, channel, MCP, SDK, UI, HTTP, and NATS projections for names, arguments, versions, permissions, safety, pagination, and error behavior.
9. **Builder/runtime parity:** create a page in the visual builder, save the normalized manifest, open it in standalone preview, publish it, and open the active route at desktop, tablet, and mobile widths. Exit requires the same component tree, semantic layout, bindings, permission outcomes, loading/empty/error/offline states, and command behavior; builder-only coordinates or placeholder orientation do not count as a passing implementation.

## 20. Implementation phases

Audit note: phases 0, 1, and 2 are implemented in the current codebase and covered by contract, outbox, local-provider, and NATS-provider tests. Phase 3 is now the next active implementation target.

### Phase 0 — contract extraction

- Add `EventEnvelope`, `CommandEnvelope`, and provider-neutral interfaces.
- Keep current local WAL behavior behind `LocalProvider`.
- Add event IDs, versions, correlation IDs, and idempotency tests.
- Replace pointer-based async hook requests with serializable DTOs.
- Mark current chat and MCP behavior as experimental until shared authorization, confirmation, timeout, and identity gates pass.
- Extract one `ToolCatalog` from the registry and make chat/channel/MCP projections contract-test against it.
- Add provider-profile validation, request deadlines, run/step IDs, and immutable usage records before adding more AI tools.

### Phase 1 — outbox and worker contracts

- Add `_kora_outbox` and consumer receipts.
- Move analytics, webhooks, and async hooks behind worker interfaces.
- Ensure local provider uses the same delivery semantics as NATS.
- Stop silently dropping critical async work; expose failure/overflow metrics.

### Phase 1A — authentication foundation

- Replace hardcoded provider responses with a provider registry and public capability discovery.
- Preserve password and magic-link behavior while migrating sessions, revocation, rate limits, and audit to the normalized auth model.
- Add provider configuration, secret references, identity links, auth attempts, auth events, account-link requests, and provisioning cursors.
- Implement the OIDC authorization-code + PKCE profile first, including issuer/JWKS validation, verified claims, JIT provisioning, logout, and failure recovery.
- Implement shared identity reconciliation and fail-closed actor resolution before adding SAML, LDAP, WebAuthn, social, or custom adapters.
- Add cross-site, cross-tenant, callback-replay, key-rotation, session-revocation, account-linking, and deprovisioning tests.

### Phase 2 — NATS provider

- Add `nats.go` provider and configuration.
- Provision streams/consumers idempotently.
- Implement request/reply command services.
- Implement JetStream publisher/consumer tests with a real local NATS server.
- Add CLI setup and Docker Compose NATS profile.

### Phase 3 — workflow actors

- Define workflow instance state and the SQL-backed timer scheduler described in §10.
- Implement actor lease/fencing with KV plus SQL state and measure stale-owner rejection under redelivery.
- Migrate one approval workflow end-to-end.
- Add replay, retry, dead-letter, and recovery tests.
- Implement the AI run actor using the same fencing, durable timer, approval, and recovery contracts; do not create a separate chat state machine.

### Phase 3A — AI safety and metering gate

- Move authorization, confirmation, recent-auth, identity, idempotency, and audit into the shared tool executor.
- Wire provider deadlines and bounded HTTP transport timeouts.
- Add atomic scoped budget reservations, per-run limits, and provider-attempt usage events.
- Implement server-side conversations/runs with resume, cancel, compaction checkpoints, and retention policy.
- Replace adapter-specific tool generators with catalog projections and decide whether MCP is validation-only or executable per deployment.
- Pass adversarial prompt-injection, cycle/stall, malformed-provider, duplicate-delivery, and provider-failover tests.

### Phase 4 — standalone composable UI

- Create a standalone `kora-ui` build with runtime configuration injection.
- Define JSON Schemas for page manifests, resources, actions, components, and package metadata.
- Implement the component registry with lazy loading and capability negotiation.
- Convert the dashboard and one module workspace to manifests.
- Make existing CRUD list/new/edit routes resolve through generated compatibility manifests.
- Add manifest validation at publish and load time, content hashes, ETags, and signature verification.
- Add draft/preview/active/retired lifecycle and rollback to immutable versions.
- Add one installable frontend package that contributes a page and component.
- Add manifest renderer contract tests for permissions, unsupported versions, malformed data, loading, error, empty, responsive, and offline states.

### Phase 4A — visual builder and live application surface

- Make the builder an authoring mode of the production renderer and persist normalized semantic manifests rather than editor coordinates or screenshots.
- Add guided templates, schema-aware component palette, semantic component tree, constrained drag/drop, keyboard placement, and a schema-generated property inspector.
- Add binding/resource/action pickers derived from canonical DocType and package capabilities, with inline validation and publish preflight.
- Add exact desktop/tablet/mobile preview, real permission evaluation, representative resource states, draft autosave, undo/redo, source/visual parity, and immutable preview/publish/rollback flows.
- Deliver a POS starter screen that becomes usable after a short five-step setup, with sample data, safe defaults, resumable progress, and no required technical decisions.
- Support direct POS card customization: move, resize, hide, duplicate, remove, add, undo, reset, and preview, with deterministic wide-screen, tablet, portrait, landscape, and phone arrangements.
- Connect the POS starter to the backend offline capability and shared sync coordinator, including local snapshots, queued approved operations, cursors, conflicts, revocation, and clear `Ready`, `Syncing`, `Up to date`, and `Needs attention` states.
- Add authenticated WebSocket transport through the realtime gateway with scoped subscriptions, heartbeat, resume cursor, deduplication, missed-event recovery, authoritative refetch, and SSE fallback where appropriate.
- Add realtime notifications for product, stock, payment, task, sync, and operation changes with notification-center history, read/ack commands, severity, related record/action, redaction, and offline-safe display.
- Apply the product language rules in §15.10.9 across setup, builder, POS, empty, error, offline, and publish flows.
- Add typed realtime connection status, cache invalidation, operation progress, reconnect/resume, and authoritative refetch to every live page preset.
- Verify builder-created pages through browser tests from composition to active route, including reload, reconnect, invalid data, permission changes, and rollback.
- Keep Phase 4A `planned` until the builder/runtime parity gate and live-page reconnect tests pass.

### Phase 5 — offline vertical slice

- Implement device/branch operation log with schema gates, tombstone retention, and explicit conflict states.
- Implement local apply, central intake, acknowledgement, and conflict records.
- Deliver one POS or warehouse workflow offline-first.
- Add branch sync observability and reconciliation UI.

### Phase 6 — Cloud control/data planes

- Add NATSDeployment registration, credential references, tenant accounts, quotas, and stream/KV/Object Store bootstrap against operator-hosted NATS.
- Add NATS compatibility/permission/backup validation and explicit unreachable/incompatible/draining states.
- Add managed worker placement/autoscaling.
- Add package registry and deployment rollout.
- Add managed backups, observability, billing, and regional placement.
- Keep any future Kora-managed NATS implementation behind the same deployment-provider contract; it is not required for the first Cloud release.

## 21. Acceptance criteria

The first production-ready milestone is complete when:

1. A document transaction and its event are never separated by a crash without recovery through the outbox.
2. Analytics, webhooks, and async hooks can run as separate processes.
3. Restarting a worker causes safe redelivery, not lost or duplicated business effects.
4. The same tests pass with LocalProvider and NATSProvider.
5. A workflow can pause, restart, retry, and resume from persisted state.
6. A page can be added by publishing a manifest and package metadata without adding a React route component.
7. A frontend can reject unsupported component/page versions cleanly.
8. A page manifest cannot execute arbitrary JavaScript, SQL, URLs, or unauthorized commands.
9. A standalone frontend can run against Kora through runtime configuration without rebuilding the Go server.
10. A branch can create approved offline operations and synchronize them with explicit conflict handling.
11. The frontend meets its performance targets on representative low-end mobile hardware.
12. Kora OSS runs without Cloud and without NATS in embedded mode.
13. Kora Cloud can provision the same application package against operator-hosted NATS and workers; managed NATS is optional and does not change application contracts.
14. Command responses, API errors, pagination, concurrency, and long-running operations conform to the defined contracts.
15. Aggregate ordering and duplicate delivery tests pass under concurrent publishers and consumer restarts.
16. Outbox lease expiry, replay, dead-letter, and operator recovery are tested for every supported SQL dialect.
17. Actor fencing prevents stale owners from committing state or business effects.
18. Offline push/pull is cursor-repeatable, idempotent, and returns explicit conflict records for rejected operations.
19. Timer scheduling survives scheduler and worker restarts and duplicate timer delivery produces one business effect.
20. In-process and NATS-dispatched commands use the same kernel authorization and idempotency behavior.
21. Application packages cannot activate without valid signatures, integrity hashes, and compatibility checks.
22. Cloud provisioning resumes idempotently after control-plane restart and does not mark a deployment active before all health gates pass.
23. An embedded package can run in Cloud without application-code changes.
24. Manifest bindings reject executable expressions and unsupported data paths.
25. Cloud NATS credentials cannot publish or subscribe outside their generated tenant/service subject policy.
43. Cloud registers and validates operator-hosted NATS before activating a deployment, and reports NATS loss/incompatibility explicitly without silently switching durability modes.
44. Cloud provisioning is idempotent and resumable across database, runtime, NATS account, streams, KV, Object Store, credentials, DNS/TLS, and health resources.
45. Cloud channels submit authenticated engine commands/runs and do not duplicate engine tools, authorization, agent state, or business execution.
46. Cloud usage and billing projections consume immutable engine usage/cost events and preserve estimated, reconciled, corrected, and provider-fallback records.
47. Tenant isolation tests cover SQL, NATS accounts/subjects, KV, Object Store, cache, logs, traces, metrics, backups, credentials, and usage data.
48. A non-technical administrator can create a useful page from a template, data source, component palette, and schema-backed inspector without writing code.
49. The builder saves a normalized semantic manifest; it does not persist screenshots, DOM snapshots, arbitrary coordinates, or editor-only orientation metadata.
50. Builder draft preview and the active route use the same production renderer, component registry, layout rules, design tokens, permissions, responsive behavior, and resource states.
51. Builder-created pages pass insert, reorder, nesting, binding, action, undo/redo, reload, source-edit, preview, publish, rollback, and invalid-manifest recovery tests.
52. A live page receives typed realtime invalidations or operation progress, exposes connection/degraded/reconnecting state, and performs authoritative refetch after reconnect or missed events.
53. Realtime delivery never grants authorization, accepts arbitrary browser subjects, executes UI instructions, or causes unbounded refetch/render loops.
54. Desktop, tablet, and mobile output is deterministic for the same manifest and uses explicit responsive rules rather than client-specific random orientation or freeform coordinates.
55. Builder accessibility covers keyboard composition, focus management, tree/inspector relationships, drag-and-drop alternatives, preview semantics, and screen-reader announcements.
56. Publish preflight blocks unknown components, invalid bindings, unreachable actions, unresolved permissions, incompatible responsive rules, and failed accessibility/security checks.
57. A first-time user can reach a usable POS register through the short setup flow without writing code or making advanced configuration decisions, and can resume setup without losing entered data.
58. POS users can move, resize, hide, duplicate, remove, and add cards directly on the real screen, with visible drop targets, immediate preview, undo, and reset-to-default.
59. POS arrangement is deterministic across wide, tablet, phone, portrait, and landscape modes; orientation changes reflow the same screen instead of producing a new or random layout.
60. Setup, builder, POS, empty, error, offline, and publish copy passes a plain-language audit: one clear action, consistent terminology, constructive recovery guidance, no blame, no unexplained codes, and no required engine jargon.
61. When advertised by the backend, POS can continue approved work offline from a compatible local snapshot, queue operations durably, and reconcile through the shared cursor-based sync protocol.
62. Offline POS never labels a locally queued operation as centrally complete; it shows `Queued`, `Syncing`, `Accepted`, `Conflict`, `Rejected`, or `Needs attention` with a safe next step.
63. Connected POS uses the authenticated WebSocket gateway for scoped realtime updates, operation progress, and reconnect/resume; it deduplicates events, recovers missed events, and refetches authoritative state after reconnect or sequence loss.
64. Realtime notifications are typed, scoped, redacted, deduplicated, accessible, and actionable; read/acknowledge state uses authorized idempotent commands and important notifications remain visible in the notification center.
26. Every agent tool call is re-authorized by the shared kernel executor, including chat, MCP, channel, SDK, UI, and NATS callers.
27. Confirmation and recent-auth requirements are enforced by durable protocol state and cannot be satisfied by prompt text or client-only headers.
28. Provider requests have bounded deadlines; model/provider/profile pairs and credential/account precedence are validated before network I/O.
29. AI usage, cost estimates, reconciliations, corrections, quotas, and attribution survive restart and are queryable at run and provider-attempt granularity.
30. The tool catalog has parity tests across chat, MCP, SDK, UI, and channel projections; unsupported or validation-only adapters are advertised as such.
31. AI conversations and runs can resume after channel disconnect, worker restart, provider timeout, and compaction, subject to retention policy.
32. Prompt injection, sensitive-summary retention, cycle detection, typed tool errors, and malformed provider payloads have regression and adversarial tests.
33. Password and magic-link authentication preserve their current behavior while using the normalized provider/session/audit contracts.
34. `/api/auth/providers` is generated from enabled provider configuration and never exposes secrets or disabled providers.
35. OIDC authorization-code + PKCE validates state, nonce, issuer, audience, redirect URI, JWKS, token expiry, verified claims, and provider policy before creating a session.
36. OAuth2-only, SAML, LDAP, WebAuthn, social, custom, and SCIM capabilities are individually marked `planned`, `experimental`, or `supported` with profile-specific tests.
37. Canonical identity links use provider instance, issuer, and subject; email matching alone never merges accounts.
38. Account linking, unlinking, provider migration, and identity collisions require explicit policy, recent authentication, audit, and reconciliation results.
39. Provider disable/deprovision revokes sessions, cached authorization, channel credentials, and applies the defined policy to pending approvals and delayed commands.
40. Provider secrets, access tokens, callback state, identity claims, and sensitive authentication data are absent from browser payloads, logs, traces, manifests, sessions, and backups except where retention policy explicitly permits protected storage.
41. Cross-tenant provider, callback, identity, session, cache, provisioning, and account-linking isolation tests pass.
42. Provider migration and retirement are resumable, idempotent, collision-aware, and never silently delete or merge identities.

### 21.1 Implementation handoff checklist

The next implementation agent must proceed in this order:

1. **Freeze contracts:** create generated JSON Schemas/Go types for envelopes, actor context, errors, command results, task receipts, tool descriptors, usage events, approvals, and cursors. Add contract version tests.
2. **Build authentication foundation:** implement the provider registry, preserve password/magic-link behavior, add normalized identity/session/audit records, and ship OIDC + PKCE before other provider families.
3. **Freeze identity and idempotency:** implement principal/service-principal resolution, delegation, recent-auth evidence, operation identity, request fingerprints, response replay, expiry, and fail-closed behavior.
4. **Build the shared executor:** move authorization, channel allowlists, confirmation, recent-auth, audit, idempotency, and typed results into one executor used by every adapter.
5. **Build the provider boundary:** validate auth and AI provider/profile tuples, enforce deadlines, normalize responses/errors/usage, record attempts, and implement budget reservations before network calls.
6. **Build durable AI runs:** implement SQL records, KV projections, leases/fencing, checkpoints, timers, resume/cancel, compaction, retention, and channel delivery cursors.
7. **Consolidate adapters:** generate auth discovery, chat, channel, MCP, SDK, UI, and HTTP schemas from canonical registries; mark incomplete providers/adapters according to capability status.
8. **Add NATS and outbox parity:** run the same contract and recovery tests with LocalProvider and NATSProvider, including duplicate delivery and dead-letter replay.
9. **Add Cloud controls:** implement the normative §14.4 specification: register and validate operator-hosted NATS, provision tenant resources, reconcile deployments with fencing, preserve delegated identity, route channels to engine runs, project usage/cost events, enforce quotas/suspension, and pass backup/restore, deletion, isolation, and RPO/RTO tests. Do not require Kora-managed NATS.

Each step must deliver code, migrations/schemas, tests, metrics, operational commands, and a short evidence record. Do not proceed to the next step with a failing security or data-integrity gate. If a capability is incomplete, update its status to `planned` or `experimental` and ensure public documentation does not describe it as supported.

The following decisions are intentionally deferred, but must be resolved before the dependent capability is supported: exact SQL dialect implementation details; provider-specific price-sheet ingestion; regional provider availability; final SSE/WebSocket wire format; offline conflict UI; and Cloud commercial plan limits. Deferred decisions must have an owner, target phase, and compatibility-safe default.

## 22. Reference implementations and reading

These references informed the design:

- [NATS JetStream](https://docs.nats.io/concepts/jetstream): streams, durable consumers, replay, acknowledgement, and retention.
- [NATS Request-Reply](https://docs.nats.io/nats-concepts/core-nats/reqreply): scalable service request/reply and queue groups.
- [NATS Services](https://docs.nats.io/using-nats/developer/services): service metadata, names, versions, and endpoints.
- [NATS KV](https://docs.nats.io/nats-concepts/jetstream/key-value-store): atomic create/update, watches, history, and limits.
- [NATS Security](https://docs.nats.io/nats-concepts/security): accounts, subject permissions, JWT/NKey authentication, TLS, and encryption at rest.
- [NATS Leaf Nodes](https://docs.nats.io/running-a-nats-service/configuration/leafnodes): local/edge routing and controlled imports/exports.
- [NVIDIA Cloud Functions architecture](https://docs.nvidia.com/nvcf/architecture-overview): independently scalable planes using JetStream as a durable request buffer and state-machine-oriented workload lifecycle.
- [NATS Execution Engine](https://nats.io/blog/introducing_nex/): the direction of using NATS as both connectivity and workload execution/control fabric; treat this as inspiration, not a required dependency.
- [Shopify theme architecture](https://shopify.dev/docs/storefronts/themes/architecture): layouts, templates, sections, blocks, and app extensions.
- [Shopify JSON templates](https://shopify.dev/docs/storefronts/themes/architecture/templates/json-templates): data-defined page composition with reorderable sections and blocks.
- [Salesforce Lightning App Builder](https://help.salesforce.com/s/articleView?id=platform.lightning_app_builder_customize_lex_pages.htm&language=en_US&type=5): metadata-driven enterprise home and record pages.
- [Salesforce Lightning component metadata](https://developer.salesforce.com/docs/platform/lwc/guide/use-config-for-app-builder): component availability, page-type restrictions, and design configuration.
- [Backstage frontend plugins](https://backstage.io/docs/frontend-system/architecture/plugins/): installable pages, navigation, APIs, cards, and extension features.
- [Backstage frontend extensions](https://backstage.io/docs/next/frontend-system/architecture/extensions/): composable and replaceable frontend extension points.
- [TanStack Router code splitting](https://tanstack.com/router/latest/docs/guide/code-splitting): critical route configuration and lazy page components.
- [TanStack Router deferred data loading](https://tanstack.com/router/latest/docs/guide/deferred-data-loading): progressive loading of critical and non-critical data.
- [JSON Forms](https://jsonforms.io/): schema-driven forms, UI schemas, renderer registries, and custom widgets.
- [NIST AI Risk Management Framework](https://www.nist.gov/itl/ai-risk-management-framework): lifecycle governance, mapping, measurement, and management for trustworthy AI.
- [NIST Generative AI Profile](https://doi.org/10.6028/NIST.AI.600-1): generative-AI-specific risk and control guidance.
- [OpenTelemetry GenAI semantic conventions](https://github.com/open-telemetry/semantic-conventions-genai): provider, model, agent, tool, retrieval, token, latency, and error telemetry.
- [FinOps FOCUS](https://focus.finops.org/focus-specification/): normalized, multi-provider cost and usage reporting and attribution.
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0-18.html): identity claims and authentication over OAuth 2.0.
- [OpenID Connect Discovery 1.0](https://openid.net/specs/openid-connect-discovery-1_0-22.html): issuer metadata and endpoint discovery.
- [OAuth 2.0 Security BCP, RFC 9700](https://datatracker.ietf.org/doc/html/rfc9700): current OAuth security requirements, including authorization-code and PKCE guidance.
- [Web Authentication Level 3](https://www.w3.org/TR/webauthn-3/): passkeys and public-key authentication ceremonies.
- [System for Cross-domain Identity Management, RFC 7644](https://www.rfc-editor.org/rfc/rfc7644): SCIM provisioning protocol.
- [SAML 2.0 Core](https://docs.oasis-open.org/security/saml/v2.0/saml-core-2.0-os.pdf): SAML assertions, protocol messages, and security model.
- [LDAP Protocol, RFC 4511](https://www.rfc-editor.org/rfc/rfc4511): LDAP protocol and directory interoperability baseline.

## 23. Final design principle

Kora should be easy to use as a monolith and powerful as a distributed runtime:

```text
simple deployment → local provider → ordinary synchronous development
serious deployment → NATS provider → durable workers, actors, replay, offline
```

The platform must hide distributed-systems machinery behind Kora commands, workflows, events, page manifests, and package contracts. That is how the architecture increases feature velocity instead of merely adding infrastructure complexity.

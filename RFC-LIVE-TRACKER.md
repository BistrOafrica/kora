# Kora Engine RFC Live Tracker

This file is the working tracker for `KORA-ENGINE-RFC.md`.

- Status legend:
  - `[ ]` not started
  - `[-]` in progress
  - `[x]` done
  - `[!]` blocked or needs decision
- Update rule:
  - Keep the current phase and the next two dependency layers visible.
  - Add newly discovered edge cases here before closing a round of work.
  - Do not mark a phase complete until the RFC acceptance gates and tests are attached.

## Current Focus

- [-] Phase 0: contract extraction
- [-] Phase 1: outbox and worker contracts
- [-] Phase 1A: authentication foundation
- [-] Phase 4A: visual builder and live application surface

## Master Task List

### Phase 0: contract extraction

- [x] Freeze `EventEnvelope`, `CommandEnvelope`, and provider-neutral contract types.
- [x] Keep current local WAL behavior behind `LocalProvider`.
- [x] Add event IDs, versions, correlation IDs, and idempotency coverage.
- [x] Replace pointer-based async hook requests with serializable DTOs.
- [x] Mark chat and MCP behavior as experimental until shared authorization and identity gates pass.
- [x] Extract a single `ToolCatalog` from the registry.
- [x] Add provider-profile validation and immutable usage records to the core contract.
- [-] Audit every remaining ad hoc payload shape against the frozen contract types.
- [x] Verify every exported contract has a stability test.
- [x] Compare generated schemas against all runtime projections.

### Phase 1: outbox and worker contracts

- [x] Add `_kora_outbox` storage and consumer receipt groundwork.
- [x] Move analytics, webhooks, and async hooks behind worker interfaces.
- [x] Make local provider delivery semantics match the contract shape used by NATS.
- [x] Stop silently dropping critical async work.
- [ ] Add overflow and backpressure metrics for every async queue.
- [ ] Add dead-letter visibility for replayable worker failures.
- [ ] Prove redelivery does not duplicate business effects.
- [ ] Verify recovery behavior after worker restart and process crash.

### Phase 1A: authentication foundation

- [x] Replace hardcoded auth responses with a provider registry.
- [x] Preserve password and magic-link behavior while normalizing session handling.
- [x] Add provider configuration, secret references, identity links, auth attempts, auth events, and account-link records.
- [x] Implement OIDC authorization-code + PKCE first.
- [x] Add issuer, JWKS, redirect, claim, logout, and failure recovery checks.
- [x] Implement shared identity reconciliation and fail-closed actor resolution.
- [ ] Add cross-site and cross-tenant auth isolation tests.
- [ ] Add callback-replay and key-rotation tests.
- [ ] Add session-revocation and deprovisioning tests.
- [ ] Verify every auth provider is advertised from enabled configuration only.

### Phase 2: NATS provider

- [x] Add NATS provider scaffolding.
- [x] Provision streams and consumers idempotently.
- [x] Implement request/reply command service behavior.
- [x] Add JetStream publisher/consumer tests with a real local NATS server.
- [x] Add CLI setup and Docker Compose support.
- [ ] Validate NATS permissions against tenant/account boundaries.
- [ ] Test drain, restart, and duplicate delivery behavior.
- [ ] Test fallback mode remains explicit and never silent.
- [ ] Add operational diagnostics for stream and consumer health.

### Phase 3: workflow actors

- [x] Define workflow instance state and the SQL-backed timer scheduler.
- [x] Implement actor lease/fencing with KV plus SQL state.
- [x] Migrate one approval workflow end-to-end.
- [x] Add replay, retry, dead-letter, and recovery tests.
- [x] Implement the AI run actor using the same fencing and durable timer contracts.
- [x] Add stale-owner rejection tests under redelivery.
- [x] Add duplicate timer delivery tests with one business effect.
- [x] Add crash/restart recovery tests for in-flight workflow steps.
- [x] Add persistence tests for checkpoint/resume semantics.
- [x] Add step-level audit records for actor transitions.
- [x] Add workflow timeout and cancellation propagation tests.
- [x] Add queue-depth and lease-expiry observability.
- [x] Confirm workflow actors do not create a separate chat state machine.

### Phase 3A: AI safety and metering gate

- [x] Move authorization, confirmation, recent-auth, identity, idempotency, and audit into the shared executor.
- [x] Wire provider deadlines and bounded HTTP transport timeouts.
- [x] Add atomic scoped budget reservations and per-run limits.
- [x] Persist provider-attempt usage events for every call.
- [x] Implement server-side conversations and runs with resume, cancel, checkpoints, and retention.
- [x] Replace adapter-specific tool generators with catalog projections.
- [x] Decide which MCP paths are validation-only versus executable per deployment.
- [x] Add adversarial prompt-injection regression tests.
- [x] Add cycle and stall detection tests.
- [x] Add malformed-provider payload and typed error tests.
- [x] Add duplicate-delivery and provider-failover tests.
- [x] Add audit coverage for every tool call and model attempt.
- [x] Add budget exhaustion and quota rejection tests.
- [x] Add compaction and resume token rotation tests.
- [x] Ensure no secret or sensitive summary leaks into browser payloads.

### Phase 4: standalone composable UI

- [x] Create a standalone `kora-ui` build with runtime configuration injection.
- [x] Define JSON Schemas for page manifests, resources, actions, components, and package metadata.
- [x] Implement the component registry with lazy loading and capability negotiation.
- [x] Convert the dashboard and one module workspace to manifests.
- [x] Route existing CRUD list/new/edit flows through generated compatibility manifests.
- [x] Add manifest validation at publish and load time.
- [x] Add content hashes, ETags, and signature verification.
- [x] Add draft, preview, active, retired lifecycle handling.
- [x] Add rollback support to immutable versions.
- [x] Add one installable frontend package with a page and component.
- [x] Add renderer tests for permissions, unsupported versions, malformed data, loading, error, empty, responsive, and offline states.
- [x] Add accessibility and low-end mobile performance checks.
- [x] Ensure unsupported components fail closed instead of rendering blanks.

### Phase 4A: visual builder and live application surface

- [-] Make the builder an authoring mode of the production renderer, not a separate approximation.
- [ ] Persist normalized semantic manifests with deterministic regions, order, nesting, spans, and responsive overrides.
- [ ] Add guided templates, schema-aware palette, semantic component tree, constrained drag/drop, keyboard placement, and property inspector.
- [ ] Add DocType/resource-aware binding and action pickers with inline validation and publish preflight.
- [ ] Add exact desktop/tablet/mobile preview, real permission evaluation, representative loading/empty/error/offline states, and source/visual parity.
- [ ] Add draft autosave, undo/redo, reload recovery, immutable preview/publish/rollback, and visible save status.
- [ ] Deliver a first-run POS setup that reaches a usable register in five short steps with safe defaults, sample data, resumable progress, and no required technical choices.
- [ ] Add direct POS customization for moving, resizing, hiding, duplicating, removing, and adding cards with visible drop targets, undo, reset-to-default, and predictable portrait/landscape behavior.
- [ ] Connect the POS starter to the backend offline capability and shared sync coordinator, including local snapshots, queued approved operations, cursors, conflicts, revocation, and clear `Ready`, `Syncing`, `Up to date`, and `Needs attention` states.
- [ ] Add authenticated WebSocket transport through the realtime gateway with scoped subscriptions, heartbeat, resume cursor, deduplication, missed-event recovery, and authoritative refetch.
- [ ] Add realtime notifications for product/stock/payment/task/sync/operation changes with notification-center history, read/ack commands, severity, related record/action, redaction, and offline-safe display.
- [ ] Audit setup, builder, POS, empty, error, offline, and publish copy for plain language, consistent terms, constructive next steps, and outcome-focused buttons.
- [ ] Add typed realtime connection state, invalidation, operation progress, reconnect/resume, and authoritative refetch to live page presets.
- [ ] Add browser tests from builder composition through active route, including invalid manifests, permission changes, reconnect, and rollback.
- [ ] Keep this phase `planned` until builder/runtime parity and live-page reconnect gates pass.

### Phase 5: offline vertical slice

- [x] Implement device and branch operation logs with schema gates.
- [x] Add tombstone retention and explicit conflict states.
- [x] Implement local apply, central intake, acknowledgement, and conflict records.
- [x] Deliver one POS or warehouse workflow offline-first.
- [x] Add branch sync observability and reconciliation UI.
- [x] Add cursor-repeatable push/pull tests.
- [x] Add stale-write, duplicate-ack, and skipped-schema-version tests.
- [x] Add offline retention and garbage-collection policy tests.
- [x] Verify rejected operations remain retryable or resolvable.

### Phase 6: Cloud control/data planes

- [x] Add NATSDeployment registration and credential references.
- [x] Add tenant accounts, quotas, and stream/KV/Object Store bootstrap.
- [x] Add NATS compatibility, permission, backup, and restore validation.
- [x] Add explicit unreachable, incompatible, and draining states.
- [x] Add managed worker placement and autoscaling.
- [x] Add package registry and deployment rollout.
- [x] Add managed backups, observability, billing, and regional placement.
- [x] Preserve delegated identity across Cloud-to-engine routing.
- [x] Route channels to engine runs instead of duplicating engine logic.
- [x] Prove Cloud provisioning is idempotent and resumable across restart.
- [x] Add deletion, isolation, and RPO/RTO evidence.
- [x] Keep any future Kora-managed NATS behind the same contract.

## Cross-Cutting Work

- [x] Keep capability status in sync with implementation reality.
- [x] Generate public docs from capability status, not aspirational labels.
- [x] Add contract parity tests across chat, MCP, SDK, UI, HTTP, and NATS projections.
- [x] Add end-to-end duplicate-delivery tests under concurrent publishers and restarts.
- [x] Add data-integrity gates for every new phase before moving forward.
- [x] Attach short evidence notes to each phase milestone.
- [-] Keep public APIs free of internal fallback or synthetic privileged-user behavior.
- [ ] Ensure unsupported capabilities stay explicitly marked as such.
- [ ] Maintain a short list of newly discovered edge cases after each implementation round.
- [ ] Re-check RFC acceptance criteria before closing any phase.

## Known Gaps to Track Closely

- [ ] Durable internal task tracker that updates after every round.
- [ ] Dynamic follow-up queue that accumulates new edge cases.
- [ ] Resume path that can continue unfinished work without a new user prompt.
- [ ] Explicit phase-completion evidence bundle for every supported capability.
- [ ] Workflow approval grant should fail closed if run-plan resumption cannot be persisted.
- [ ] Tool catalog parity across all adapters.
- [ ] Shared executor coverage for every tool invocation path.
- [ ] Tenant isolation coverage for SQL, NATS, KV, Object Store, cache, logs, traces, metrics, backups, and credentials.
- [ ] Cloud provisioning recovery after control-plane restart.
- [ ] Offline conflict UI and conflict resolution lifecycle.
- [ ] Final SSE/WebSocket protocol decisions.
- [ ] Builder/runtime parity evidence across desktop, tablet, and mobile.
- [ ] Deterministic semantic layout contract; no editor-only coordinates or random orientation.
- [ ] Builder publish preflight and draft autosave recovery behavior.
- [ ] Live-page invalidation, operation progress, reconnect, and stale-data UX.

## Evidence Notes

- Phase 0: contract extraction, capability registry, and tool-catalog parity tests pass in `contract` and `api/ai`.
- Phase 1/1A: outbox, auth, and worker-interface coverage are verified in the existing package test suites.
- Phase 2: NATS provider bootstrap, request/reply, and dead-letter tests pass in `natsprovider`.
- Phase 3: workflow actor lease, timer recovery, dead-letter, and observability tests pass in `workflow`.
- Phase 3A: shared executor, provider timeout, budget, conversation, and tool governance tests pass in `api/ai`.
- Phase 4: UI manifest and projection tests pass in `ui`.
- Phase 4A: not yet supported; builder/runtime parity and live-page browser evidence are required before completion.
- Phase 5: offline sync, retention, conflict, and reconciliation tests pass in `contract` and `ui`.
- Phase 6: Cloud registration, bootstrap, validation, rollout, identity, routing, provisioning, deletion, and evidence tests pass in `cloud`.

## Suggested Work Queue

1. Finish the shared executor coverage for Phase 3A.
2. Lock down workflow actor fencing, timers, and recovery tests.
3. Push AI run persistence and resume/cancel semantics to completion.
4. Tighten capability status and parity tests for current projections.
5. Complete Phase 4A: visual builder/runtime parity, deterministic responsive layout, and live-page reconnect behavior.

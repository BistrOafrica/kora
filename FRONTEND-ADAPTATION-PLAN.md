# Frontend Adaptation Plan for the Kora Engine RFC

## What the current frontend already does

The UI already has several useful seams that align with the RFC:

- A server-fed view runtime for dynamic pages in `ui/src/components/views/ViewRenderer.tsx`.
- A component registry and capability gating in `ui/src/components/views/registry.tsx`.
- A shared API client in `ui/src/lib/api/client.ts`.
- Server-driven view metadata and validation helpers in `ui/src/lib/api/views.ts`, `ui/src/lib/view-manifest.ts`, and `ui/src/lib/page-manifests.ts`.
- An offline queue primitive in `ui/src/lib/offline-queue.ts` and a POS route that uses it.

That means the UI does not need a rewrite. It needs a contract shift:

- from route-first pages to manifest-first pages,
- from implicit CRUD screens to typed command/query actions,
- from polling-only updates to realtime invalidation and operation progress,
- from local-only offline behavior to policy-driven offline capability,
- from ad hoc component config to versioned component manifests with capability checks.

## Gaps to close

The main gaps between the current frontend and the RFC are:

1. The router still owns too much application shape. Dynamic view routes exist, but the app is not yet a true page-manifest runtime.
2. Data fetching is mostly list/document oriented. The RFC wants named queries, typed commands, pagination cursors, capability-aware tool catalogs, and operation envelopes.
3. Realtime is not a first-class browser contract yet. The RFC expects authenticated SSE/WebSocket state, typed invalidations, reconnect, and resume.
4. Offline support is mostly a queue. The RFC wants per-manifest offline policy, conflict handling, bounded persistence, and explicit sync status.
5. The UI does not yet consume the backend capability snapshot as the source of truth for page availability, commands, tool actions, or component support.
6. Current manifest validation is mostly structural. The RFC requires schema-validated manifests with component capability requirements, offline behavior, and version gates.

## Adaptation strategy

### 1. Make the frontend manifest-first

Treat the router as a shell, not the source of business navigation.

Work items:

- Add a page manifest loader that resolves route, package, version, and capability snapshot from the server.
- Render pages from manifest data before deciding which concrete component tree to mount.
- Convert `ViewRenderer` into a compatibility layer inside the new manifest runtime instead of the primary entry point.
- Add explicit unsupported states for missing manifest version, missing component capability, unsupported layout, and retired page status.

Outcome:

- New pages can be introduced by backend manifests without shipping a new route.
- The UI can fail closed when a manifest is incompatible instead of rendering partial or stale content.

### 2. Replace ad hoc CRUD assumptions with typed operations

The RFC’s command/query model should become the frontend’s default interaction model.

Work items:

- Introduce a typed operation client that can represent `completed`, `accepted`, `rejected`, `conflict`, and `failed`.
- Add a shared operation envelope type that carries `operation_id`, `correlation_id`, `status`, and structured errors.
- Model explicit command forms and query cards instead of assuming every action is a document mutation.
- Preserve idempotency keys across retries and offline replays.

Outcome:

- The UI can distinguish “request accepted” from “business completed.”
- Long-running workflows, approvals, and background jobs become visible and resumable.

### 3. Add realtime as an invalidation and progress channel

The browser should subscribe to authenticated realtime updates, not raw data streams.

Work items:

- Build a browser realtime client for SSE/WebSocket with `connecting`, `connected`, `degraded`, `reconnecting`, `offline`, `unauthorized`, and `closed` states.
- Use typed invalidations for list/document caches instead of broad refetches.
- Use progress events for accepted operations, workflow transitions, approvals, and agent runs.
- Keep server state authoritative; realtime updates should trigger refetches or targeted cache patches, not replace canonical data.

Outcome:

- The UI updates faster without trusting the browser cache as business truth.
- Users can see operation progress and connection state instead of waiting blind.

### 4. Make offline a capability, not a blanket mode

Offline should be declared by manifest and enforced by runtime policy.

Work items:

- Extend page manifests with offline policy: `unsupported`, `read_only`, `queue_writes`, or `full_slice`.
- Persist queued operations with `device_id`, `branch_id`, `operation_id`, `base_version`, and timestamps.
- Add conflict record rendering and explicit resolution actions.
- Keep offline storage bounded and scoped by site, package, schema version, and device.

Outcome:

- Offline works where it is safe and intended.
- The UI can show queued work, conflict state, and sync health without pretending all pages are offline-capable.

### 5. Promote capability snapshots to a first-class input

The backend capability registry should drive UI behavior.

Work items:

- Load runtime capability snapshots at startup and on invalidation.
- Gate pages, actions, and component variants on backend-advertised capabilities.
- Hide or downgrade unsupported features instead of rendering dead controls.
- Use capability metadata for admin surfaces, tool palettes, workflow actions, and AI-assisted interactions.

Outcome:

- The frontend reflects the real deployment shape instead of assuming all modules are available everywhere.

### 6. Harden manifest and component contracts

The manifest layer needs to become explicit about what is allowed.

Work items:

- Expand manifest schema validation to include component version, required capabilities, permissions, allowed parents, and offline behavior.
- Keep arbitrary executable code out of tenant manifests.
- Ensure unsupported component types render a typed fallback, not an empty section.
- Standardize loading, empty, error, permission-denied, offline, conflict, and stale states for every component contract.

Outcome:

- Manifest loading becomes deterministic and safe.
- Unsupported pages degrade visibly and consistently.

### 7. Separate shell state from server state

The RFC is explicit that the shell owns presentation state, not business truth.

Work items:

- Keep Zustand or similar stores for ephemeral UI state only.
- Move durable application state into query caches, operation state, or offline queue state.
- Add a dedicated connection/status area in the shell for auth, realtime, offline, and sync status.
- Ensure route changes invalidate the right caches and do not leak tenant/site state.

Outcome:

- The app becomes easier to reason about and safer under reconnect/offline conditions.

## Recommended execution phases

### Phase 1: Contract alignment

- Define the frontend contract types for manifest, capability snapshot, operation envelope, realtime event, and offline queue entry.
- Add schema validation and compatibility tests.
- Mark unsupported states explicitly in the UI.

### Phase 2: Manifest runtime

- Replace route-centric page selection with manifest-driven rendering.
- Make page resolution version-aware and capability-aware.
- Teach the view runtime to honor server policy, not local assumptions.

### Phase 3: Typed operations and progress

- Add command/query abstractions that understand `accepted` and `completed`.
- Add operation progress screens, retry handling, and idempotency preservation.
- Introduce typed error surfaces for validation, permission, conflict, timeout, and dependency failures.

### Phase 4: Realtime and invalidation

- Add authenticated SSE/WebSocket transport.
- Wire typed invalidations into TanStack Query cache management.
- Add reconnect, resume, and bounded refetch rules.

### Phase 5: Offline slices

- Expand offline to one vertical slice first, likely POS or warehouse.
- Support queueing, partial sync, conflict records, and explicit replay.
- Keep the rest of the app read-only or online-only until proven.

### Phase 6: Capability-driven UX

- Drive admin menus, component availability, and tool surfaces from backend capabilities.
- Add progressive disclosure so advanced runtime features do not overwhelm the primary navigation.
- Remove duplicate client-side assumptions about available modules.

## Concrete priorities in the current codebase

These are the most direct changes to make next:

1. Add a `capabilities` and `offline` field to the frontend runtime config, then use it in route and component gating.
2. Introduce a typed page manifest contract that supersedes the current local `PageManifest` shape.
3. Replace `fetchList`-only view data loading with named query support and operation envelopes.
4. Add a realtime client module and a small connection indicator in the shell.
5. Extend the offline queue from a POS helper into a generic operation queue with conflict metadata.
6. Make unsupported component/page states explicit and user-visible.

## Risks to watch

- Do not let the frontend become a second authority for permissions or business state.
- Do not encode business logic in component props that should live in server contracts.
- Do not let offline persistence exceed the policy envelope for the page or site.
- Do not add raw NATS assumptions to the browser; all transport should remain behind authenticated HTTP/SSE/WebSocket gateways.
- Do not keep both old route-first and new manifest-first models without a single compatibility boundary, or the UI will drift.

## Bottom line

The frontend should evolve from a route-driven CRUD shell into a contract-driven runtime shell:

- manifests describe what can render,
- capabilities describe what is allowed,
- commands describe what can happen,
- realtime describes what changed,
- offline describes what may be queued,
- the server remains the authority for all business decisions.


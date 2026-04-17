# Design Alternatives — MCP Discovery Server

Phase 3 output. Unbiased alternatives across six decision areas, then per-area recommendations and a cross-area coherence check.

Anchors:
- Problem statement: v1 = 4 hand-coded discovery tools, service-account Party kind, dual auth (OAuth 2.1 + API keys), Streamable HTTP on `/api/mcp`, p95 < 100ms, self-register in default project.
- Personas: **Anya** (backend LLM app, 95% of volume) drives ergonomics and latency; **Karol** (IDE / Claude.ai) drives OAuth UX; **Priya** (operator) drives the service-account lifecycle and audit surface.
- Strategic arc: v1 hand-coded → v1.5 OpenAPI emission → v2 OpenAPI-to-MCP translator.

---

## Decision Area 1 — How does AgentLens expose REST handler logic to MCP tools?

v1 has 4 tools, all read-only, all already implemented behind chi handlers (`GET /capabilities`, `GET /catalog/{id}`, `GET /catalog/{id}/card`). The question is the *seam* between MCP tool handlers and that logic — it sets the precedent for v1.5 / v2 reuse.

### Alt 1A — Direct function calls

The MCP plugin imports `internal/api` and calls handler functions directly, passing a synthesized `http.Request` / `http.ResponseWriter` or extracting helper functions. Fastest path — no refactoring, no new abstraction.

**Pros**
- Zero refactor. v1 ships in days on the REST side.
- Reuses every piece of existing code (validation, permission checks, JSON shaping) verbatim.
- Minimal risk of behavioral drift between REST and MCP — they literally run the same code.

**Cons**
- Violates AgentLens's own layer boundaries (`arch-go.yml`: plugins must not import `api/*`). Would require an explicit arch-go exception or a layer-rule rewrite.
- Handler functions take `http.ResponseWriter`; stubbing one to capture JSON for an MCP tool result is ugly and leaks HTTP semantics into the plugin.
- Permission checks today are middleware, not function-level — bypassing middleware means re-implementing `RequirePermission` inline in the MCP path (or wrapping `http.Handler`).
- Every future tool pays the same awkwardness tax; v2 translator has nothing clean to latch onto.

**When right**
- One-off throwaway prototype with a hard deadline and no v1.5/v2 intent.

**Complexity**: S (1–2 days). **Risk**: architectural debt compounds every new tool.
**Dependencies**: blocks clean Area 6 options (v2 translator needs a service-layer seam, not an HTTP-handler seam).

### Alt 1B — Call through the store layer

MCP tool handlers skip the API layer entirely and call `internal/store` (e.g., `ListCapabilities`, `GetEntry`, `GetCard`) directly, then apply permission checks and project-scope filtering inline in the plugin.

**Pros**
- Respects layer boundaries cleanly — plugins already allowed to import `kernel + foundation`; store is infrastructure accessible from the composition root.
- Fastest runtime path — no HTTP round-trip, no JSON serialize/parse.
- No refactor of `internal/api`.

**Cons**
- Re-implements non-trivial logic that currently lives in handlers: project-filter application, `CatalogEntry.MarshalJSON()` flattening shape, pagination cursor handling, error-to-status mapping. Every re-implementation is a source of drift.
- Permission enforcement must be re-wired — `RequirePermission` is chi middleware, not a store-layer check. The plugin has to construct the permission-check context by hand.
- Over the strategic arc, this is the alternative that leaves the most "REST behavior the translator must independently replicate" in v2.

**When right**
- Read path is trivial (one store call in, one response shape out) *and* there is no intent to reuse for non-read tools later. For v1's 4 read tools this is nearly true, but 'nearly' is the trap.

**Complexity**: S–M (3–4 days including ctx-building for permission checks).
**Risk**: silent divergence when a REST handler adds logic (e.g., a new filter) that the MCP path doesn't pick up.
**Dependencies**: forces Area 5 toward "plugin calls store.Create() directly" (consistent), forces Area 6 away from translator (MVP translator would have to also call store directly, re-implementing half of `internal/api`).

### Alt 1C — In-process HTTP loopback

The MCP tool handler builds an `http.Request` and invokes the chi router via `router.ServeHTTP(recorder, req)` in-process, then reads the JSON body out of the recorder. Auth is carried by synthesizing the `Authorization` header from the MCP request's bearer token.

**Pros**
- All existing middleware fires automatically: `RequireAuth`, `RequirePermission`, `otelhttp`, `Recovery`, `Logger`, CORS (harmlessly no-ops in loopback), RequestID. Zero re-implementation.
- Perfect behavioral parity — MCP is literally calling the same HTTP path a browser would.
- Works with arch-go rules: plugin imports only `http.Handler` from `kernel`, not `api/*` types.
- Keeps a clear seam for v1.5/v2: once AgentLens has OpenAPI, the translator can do exactly the same thing — synthesize an HTTP call against the router.

**Cons**
- Non-zero overhead per tool call — `httptest.NewRecorder()` allocates, JSON gets marshaled then parsed back out. For 4 read endpoints on a p95 < 100ms budget this is fine (measured overhead typically <1ms), but it's measurable.
- Error handling is via HTTP status codes — the plugin has to map 403 → MCP scope-challenge, 404 → tool error, etc. Spec-mandated anyway (MCP auth errors are HTTP-shaped), but it's glue code.
- Slightly awkward logging — every tool call shows up in the HTTP access log as a local 127.0.0.1 request unless filtered.

**When right**
- v1 has a mix of read and mutate-capable endpoints, or when behavioral parity with REST is a primary safety property, or when the v1.5/v2 arc will eventually produce an OpenAPI-aware translator that needs the same seam.

**Complexity**: M (4–6 days including the header-synthesis adapter, error-code mapping, and one round of latency measurement).
**Risk**: performance at scale on destructive tools (not in v1 scope — a v2+ concern). Double-logging if not filtered.
**Dependencies**: natural fit with any Area 6 option because the translator can reuse the same loopback pattern. Compatible with Area 5 self-registration via any mechanism.

### Alt 1D — Extract shared service layer

Refactor `internal/api/handlers.go` into thin HTTP wrappers that call `internal/service/*` methods (one per resource). MCP tools and REST handlers both call the same service methods; service returns domain values, each transport layers on its own serialization.

**Pros**
- Cleanest long-term architecture. Services become the canonical business-logic layer; REST and MCP are parallel presentation layers.
- Directly enables v2 translator (the translator generates MCP bindings that call services) and v1.5 OpenAPI emission (a framework like huma typically emits handlers that delegate to a service).
- Permission checks move out of middleware into service methods — consistent enforcement across transports, no more "is it middleware or handler logic?" ambiguity.
- Testability improves: services are testable without HTTP, without MCP.

**Cons**
- Large scope increase for v1: touches all 11 catalog endpoints + capabilities (even though only 4 are exposed as MCP tools, refactoring just those 4 leaves the codebase inconsistent). Pragmatically 2–3 weeks of refactor + regression testing.
- Expands v1 far past what the problem statement scoped ("hand-coded MCP plugin that maps 4 discovery operations onto existing REST handler logic. Ships in weeks").
- Opens questions about transaction boundaries, error types (service-layer errors mapped to both HTTP status and JSON-RPC codes), context propagation that aren't yet decided.
- Regressions possible in unrelated endpoints.

**When right**
- This is the work scheduled for v1.5. Doing it *as* v1 is a scope-change from "hand-coded MCP server" to "refactor the API layer + add MCP." Only the right call if the team explicitly chooses to collapse v1 and v1.5.

**Complexity**: L (2–3 weeks). **Risk**: v1 slips; regressions in untouched REST endpoints.
**Dependencies**: must pair with Area 6 "v1 is the translator MVP" or "v1 is hand-coded but shaped for v2 drop-in" — anything less and the service-layer refactor is overkill for v1.

### Alt 1E — Lightweight `ToolAdapter` type

Introduce a small `ToolAdapter` struct in the MCP plugin that holds references to the four narrow dependencies each v1 tool actually needs (e.g., `store.ListCapabilities`, `store.GetEntry`, `store.GetCard`, `auth.CheckPermission`). No refactor of `internal/api`; no loopback; but a deliberate abstraction the plugin owns.

**Pros**
- Respects layer boundaries (adapter imports `store + auth`, both foundation/infra).
- Gives the plugin a clean test surface (mock `ToolAdapter`).
- Small, contained, reversible — if a future decision picks loopback or service-layer, the adapter is a thin shim that's trivial to swap.
- Keeps the MCP plugin self-contained: everything a tool needs is in one place.

**Cons**
- Still re-implements the per-tool bits of handler logic that live in `internal/api` (project-filter application, JSON flattening via `MarshalJSON`, pagination defaults). Drift risk lower than Alt 1B because the surface is smaller (4 tools) but non-zero.
- Permission checks have to be manually threaded through the adapter rather than guaranteed by middleware.
- For v2 translator, the adapter is a dead end — translator output won't target this adapter shape.

**When right**
- v1 is a contained bet on the 4 discovery tools, and the team wants a cheap, reversible seam that doesn't paint the codebase into any direction for v2.

**Complexity**: S–M (3–5 days). **Risk**: same drift risk as 1B but bounded to 4 read tools.
**Dependencies**: flexible — works with any Area 5/6 option, doesn't block future refactor.

### Recommendation: **Alt 1C — In-process HTTP loopback**

**Rationale**
- **Anya's p95 < 100ms budget** is the critical constraint. Loopback overhead is <1ms per call (httptest recorder + local JSON round-trip); existing SQL endpoints already account for the bulk of latency. The budget is met.
- **Middleware reuse is automatic**. The problem statement mandates `RequirePermission`, OTel spans, and audit context. Loopback gets all three free. Alt 1B/1E require re-wiring these by hand — more code, more drift, more gotchas.
- **Parity with REST is a safety property** for Priya — whatever a user sees in the UI is exactly what they see via MCP. No "the MCP version filters projects slightly differently."
- **No arch-go violations**. Plugin imports `http.Handler` from kernel; nothing from `api/*`.
- **v1 ships in weeks, not months**. No refactor of `internal/api`. Leaves Alt 1D (service layer) as the honest v1.5 work it should be.
- **Keeps v2 options open**. A future translator can emit tools that either call services (if 1D ships in v1.5) *or* do loopback against generated REST routes. Both paths remain live.

The only real cost is the 1ms-ish overhead, which is invisible in Anya's budget, and the one-time error-mapping adapter (401/403/404 → MCP error codes), which is ~50 lines.

---

## Decision Area 2 — Service-account identity data model

Problem statement confirmed: service accounts are a new Party kind. Implementation details still open.

### Alt 2A — Reuse `parties` table, add `api_client_credentials` table

`parties.kind = 'service_account'` joins the existing `person | group | project` enum. A new `api_client_credentials` table stores one-or-more API keys per service account: `(id, party_id FK, secret_hash bcrypt, created_at, last_used_at, expires_at NULL, revoked_at NULL, label, created_by_user_id)`. PartyRelationship graph carries project memberships (same as users/groups).

**Pros**
- Stays loyal to ADR-001's Party archetype — the codebase's existing mental model. Anyone who understands users/groups/projects instantly understands service accounts.
- Project-membership reuse is automatic: `PartyRelationship(service_account → project, role=project:viewer)` uses the same code path as person → project.
- Separating credentials from identity allows multiple keys per account (rotate by issuing a new one, revoke the old) — required for zero-downtime rotation per Priya's revocation journey.
- Audit trail is natural: `party_id` is the principal ID in logs, `credential_id` is the key used. Supports Priya's "who called X last week" query.
- Credentials table can store extras (label "doc-pipeline-prod", `last_used_at` for stale-key detection, `expires_at` for forced rotation) without bloating the parties table.

**Cons**
- Two new tables (well — one new table, one new enum value and schema migration). Slightly more code than 2B.
- `parties.kind` grows; code that switches on kind must handle service_account.
- Conceptually, "a Party is a person/group/project/service_account" stretches the original archetype — service accounts are really a credential-holder that happens to look like a party for project-membership purposes.

**When right**
- The design wants clean separation of identity from credentials, expects multiple keys per account, and plans to evolve the credential schema (e.g., add JWT Bearer Assertion keys in v1.5).

**Complexity**: M (1 week: migration, schema, CRUD, auth middleware updates).
**Risk**: initial migration must be safe for both SQLite and PostgreSQL — standard for AgentLens but still attention.
**Dependencies**: independent of Area 1. Required by Area 3 (OAuth Client Credentials extension uses this table). Compatible with any Area 5.

### Alt 2B — Reuse `users` table with `is_service_account` flag

Add `users.is_service_account BOOLEAN` + `users.api_key_hash` (bcrypt). A service account is a user row with `is_service_account=true`, no login password, and an API key in place.

**Pros**
- Smallest schema change. One new column, one new flag.
- JWT/auth plumbing is 100% reused — a service account gets a JWT just like a user, `sub` is the user ID.
- RBAC table reuse is free.

**Cons**
- Muddies semantics permanently. "User" in code ambiguously means "human user" or "principal-of-any-kind." Every `users` query needs a `WHERE NOT is_service_account` or a conscious choice to include. This is the kind of debt that's fine day one and painful at month 18.
- Violates ADR-001 (Party archetype). Service accounts aren't users and shouldn't live in `users`.
- Project-membership requires a workaround — either make the service account's user a party-of-kind-person (wrong) or bypass parties for service accounts (inconsistent).
- Hard to have multiple API keys per account (where does the second hash live?). Rotation story is "delete and recreate the whole account" → breaks Priya's zero-downtime rotation.
- UI has to hide service-account users from the human-user list, hide human users from the service-account list — duplicating the view twice.

**When right**
- Zero-cost MVP with no service-account lifecycle requirements beyond create/delete. Not AgentLens's case (Priya's journey includes rotation, audit, project-scoped membership).

**Complexity**: S (2–3 days). **Risk**: persistent semantic confusion; painful to unwind later.
**Dependencies**: conflicts with Area 3 Client Credentials extension — that extension expects `client_id` (not user ID) as the principal identifier.

### Alt 2C — Pure party-graph: credentials as relationship attributes

No credentials table at all. API key hash lives as a property of a `PartyRelationship(service_account → project, role)` edge, or in `parties.metadata` JSON.

**Pros**
- Schema-minimal (no new tables).
- Fits "everything is a party" ideology.

**Cons**
- Unconventional, un-idiomatic for AgentLens and for auth in general. Bcrypt hashes don't belong in a metadata JSON blob — they need indexed lookup (`WHERE secret_hash = ?` at auth time), which JSON-property search is slow for.
- Can't model "multiple keys per account," "rotate key" cleanly — an edge attribute is 1:1 with the edge.
- Revocation semantics are bizarre: revoke → delete relationship → service account loses project access, not "this specific credential is dead."
- No natural place for `last_used_at`, `expires_at`, `created_by_user_id`. Everything becomes a JSON shape to maintain.

**When right**
- Research/prototype. Not production.

**Complexity**: S–M (cheap to write, painful to query).
**Risk**: performance (non-indexed JSON lookups at every auth), evolvability, deviates from idiomatic auth patterns.
**Dependencies**: incompatible with Area 3 Client Credentials — that spec expects credentials as first-class entities.

### Recommendation: **Alt 2A — `parties` table + new `api_client_credentials` table**

**Rationale**
- **Priya's journey explicitly requires rotation + instant revocation + audit-by-principal + project-scoped membership** (personas.md §3). Only 2A supports all four cleanly.
- **Multiple keys per account** is non-negotiable for zero-downtime rotation (issue new → deploy → revoke old). 2B forces "delete and recreate the account," which Anya's running service would see as auth failure. 2C makes "one account = one key" worse.
- **ADR-001 loyalty**. The codebase already treats Party as the identity archetype; service accounts extend it naturally. 2B breaks the archetype and invites year-of-debt.
- **OAuth Client Credentials extension (Area 3)** expects `client_id` as a distinct identifier, not a user ID. `api_client_credentials.id` is that identifier.
- **Audit query shape** (`SELECT * FROM audit_log WHERE principal_id = <party_id>`) works uniformly across all principal kinds.

Ship with 2A day one. The one-week cost is paid back within the first month by cleaner code everywhere and Priya's operator journey actually working.

---

## Decision Area 3 — OAuth 2.1 Authorization Server ambition

MCP 2025-11-25 requires AgentLens to be an OAuth 2.1 Resource Server. The Authorization Server (AS) it delegates to is a separate question.

### Alt 3A — Minimal AS: authcode + PKCE, pre-registered clients only

Implement `/oauth/authorize` + `/oauth/token` + `/.well-known/oauth-authorization-server` + `/.well-known/oauth-protected-resource`. Clients are a hard-coded list in config (initially Claude.ai, Claude Code, Cursor, VS Code Copilot redirect URIs). New client types require a config change + AgentLens redeploy.

**Pros**
- Smallest surface area. ~500–800 lines of Go.
- All known mainstream clients supported out-of-box by pre-registering their redirect URIs.
- No DCR endpoint → no open-registration DoS/abuse risk.
- Debug-friendly: operator can inspect the exact client list.

**Cons**
- Every new MCP client that isn't in the pre-registered list is a support ticket + config change + deploy. Bad for Karol's "paste URL → works" journey if his client is novel or self-built.
- Research shows DCR is widely supported (Claude.ai, Claude Code, Cursor, VS Code, Claude Desktop all support it). Skipping DCR *works* for mainstream but is fragile.

**When right**
- Tightly-controlled enterprise deployment where the operator wants to whitelist exactly which MCP clients can connect.

**Complexity**: M (1–1.5 weeks). **Risk**: onboarding friction for non-mainstream clients.
**Dependencies**: independent.

### Alt 3B — Standard: DCR (RFC 7591)

3A plus `/oauth/register` per RFC 7591. Any MCP client can POST its metadata and receive a `client_id` + (optional) `client_secret`. Persistent `oauth_clients` table. Optional rate-limit and/or admin-approval gate on registration.

**Pros**
- Directly enables Karol's "paste URL → works" journey. Claude.ai, Claude Code, Cursor, VS Code all use DCR flow — Karol pastes the URL, his client registers itself silently, OAuth proceeds.
- Research-documented as the lingua franca for MCP client onboarding.
- AgentLens ships with a complete dev-grade AS; operators get a real OAuth server they can trust.

**Cons**
- Open DCR endpoint needs rate-limiting + potentially a CAPTCHA or admin-approval gate to prevent abuse (thousands of fake clients).
- More surface to secure and audit. ~500 additional lines on top of 3A (endpoint, persistence, admin UI for listing/revoking registered clients).
- Adds a new table (`oauth_clients`) with its own lifecycle.

**When right**
- v1 wants zero-config OAuth for mainstream clients and for future clients the team hasn't seen yet. Matches the spec's canonical flow.

**Complexity**: M–L (1.5–2 weeks total including admin list/revoke UI).
**Risk**: DCR abuse if rate-limit is missing; mitigated by sensible defaults (e.g., 10/minute per IP, admin-approval mode config flag).
**Dependencies**: independent. Composes cleanly with Area 2A (service accounts live alongside DCR'd OAuth clients — they're distinct concepts).

### Alt 3C — Full: DCR + CIMD (Client ID Metadata Documents, RFC draft)

3B plus Client ID Metadata Documents per the 2025-11-25 addition. CIMD lets clients publish a signed metadata document at a URI and use the URI as their client identifier, eliminating per-AS registration state.

**Pros**
- Cutting-edge spec compliance. Claude.ai, VS Code + Copilot, Archestra support CIMD today.
- Reduces server-side client state (no `oauth_clients` row per client — the client's metadata URI is the identity).
- Enterprise polish signal.

**Cons**
- Spec is IETF draft, still evolving — maintenance burden.
- Extra complexity for marginal user benefit over DCR in v1 (both work zero-config for mainstream clients).
- Signature verification / metadata URI fetching / caching is a non-trivial subsystem.

**When right**
- v1.5 or v2, when AgentLens has enterprise deployments and operators ask for it. Premature for v1.

**Complexity**: L (2.5–3 weeks on top of 3B).
**Risk**: draft RFC churn; maintenance cost disproportionate to v1 benefit.
**Dependencies**: additive over 3B. Can ship as a feature flag later.

### Alt 3D — Delegate to external IdP

AgentLens does not run an AS. Operators deploy Keycloak / Authentik / Auth0 / Dex in front of AgentLens; AgentLens validates JWTs issued by the external IdP, audience-bound to AgentLens's canonical MCP URI. AgentLens becomes a pure Resource Server.

**Pros**
- Zero AS implementation in AgentLens. Defers all OAuth concerns to mature third-party software.
- Enterprise deployments often already have an IdP they want to use.
- Security of the AS is someone else's problem (Keycloak's security team).

**Cons**
- Kills the self-hosted simplicity story. AgentLens positions as "single binary, ships in minutes"; adding "deploy Keycloak first" is a major onboarding regression. Problem statement: "AgentLens becomes its own AS. Minimal new endpoints. Reuse JWT issuance."
- Karol's "paste URL → works" journey breaks — he has to first learn which IdP the company uses, which realm, which client.
- Priya's operator surface doubles — now she runs AgentLens *and* Keycloak.
- Service-account credentials (Area 2) have to live *somewhere* — either in the IdP (which means AgentLens has no first-class service account concept) or in AgentLens (which splits identity across two systems).

**When right**
- Enterprise deployment where SSO with a specific corporate IdP is mandatory. v1.5+ feature.

**Complexity**: S (as a feature — "we validate external JWTs"), L (as a replacement for the entire auth story).
**Risk**: adoption friction.
**Dependencies**: conflicts with v1 self-hosted simplicity goal.

### Alt 3E — Hybrid: built-in AS default, external-IdP delegation via config

AgentLens ships with a built-in minimal AS (3A or 3B). A config flag (`auth.external_idp.enabled`) switches to external-IdP mode — AgentLens's AS endpoints are disabled, JWT validation looks at the configured IdP's JWKS.

**Pros**
- Captures the self-hosted simple-start story *and* the enterprise external-IdP story.
- Operators pick their path; no either/or.
- 3E + 3B is the most flexible configuration for v1.

**Cons**
- More code (both paths exist), more matrix to test.
- Docs complexity — two deployment models to explain.
- External-IdP path still needs design work (OIDC Discovery, audience claim mapping, scope mapping from IdP roles).

**When right**
- v1.5+, once the v1 built-in AS is proven.

**Complexity**: L total (3A/B + 3D's external path).
**Risk**: feature creep; scope change from v1.

### Recommendation: **Alt 3B — DCR-enabled minimal AS**

**Rationale**
- **Karol's primary journey ("paste URL → works")** depends on DCR. The persona exists to validate that the OAuth layer is right; failing DCR fails that validation.
- **Research directly supports this**: all mainstream MCP clients (Claude.ai, Claude Code, Cursor, VS Code, Claude Desktop) use DCR. Research doc §3 table. Pre-registered-only (3A) means every new client is a deployment.
- **Problem statement §Constraints**: "AgentLens becomes its own AS. Minimal new endpoints (`/oauth/authorize`, `/oauth/token`, optional `/oauth/register`)." The "optional /oauth/register" line in the problem statement is really a decision deferred to here; DCR is non-optional for Karol's journey.
- **CIMD (3C)** is 2025-11-25-new, IETF draft, marginal over DCR for v1. Defer.
- **External IdP (3D)** breaks self-hosted simplicity; Hybrid (3E) is right for v1.5.
- DCR abuse risk is mitigated by a default rate-limit (10 registrations / 5 min / IP) and a config flag `auth.oauth.dcr_requires_approval=false` (can be toggled true in enterprise deployments).

Ship 3B. Plan 3E for v1.5. Plan 3C for v2 or on-demand.

---

## Decision Area 4 — MCP protocol: DIY vs. library dependency

Research attempted to verify `mark3labs/mcp-go` and `metoro-io/mcp-golang` but GitHub was blocked. Decision has to hedge on that uncertainty.

### Alt 4A — Adopt a mature Go MCP library

Pick `mark3labs/mcp-go` (most likely candidate based on naming/search signal) — take the library wholesale: JSON-RPC framing, Streamable HTTP transport, session management, tool/resource/prompt primitives, schema types.

**Pros**
- Fastest to production tool calls — library handles all protocol minutiae.
- Community-maintained; gets spec updates (2025-11-25 → 2026-xx-xx) when library updates.
- Less Go code to maintain in AgentLens.

**Cons**
- Dependency risk (verification blocked): unknown license, unknown maintenance velocity, unknown security posture, unknown Go-version compatibility with AgentLens's Go 1.26.1 + CGO requirement.
- Libraries opinionate on route registration, middleware, context propagation — may not integrate cleanly with chi + AgentLens's existing middleware stack.
- 2025-11-25 additions (CIMD, `MCP-Session-Id` rename, `Origin` 403, SSE resumability) — unknown whether current library versions support them.
- Adding a dep adds CVE scanning surface (Trivy, CodeQL) for AgentLens's CI.
- Go ecosystem has had prior MCP-lib churn; libraries may change APIs breaking AgentLens every few months.

**When right**
- Library verification passes: Apache 2 / MIT license, active commits, compatibility with Go 1.26.1, 2025-11-25 spec support, clean chi integration.

**Complexity**: S upfront (add dep, wire 4 tools), but unknown long-term. **Risk**: dep surprise.
**Dependencies**: independent.

### Alt 4B — Implement the MCP wire protocol directly

Write the Streamable HTTP transport, JSON-RPC 2.0 framing, session management, `initialize`/`tools/list`/`tools/call` handlers, SSE event framing. Research estimated 1500–2000 lines.

**Pros**
- Zero new deps. Trivy/CodeQL surface unchanged. License/security story unchanged.
- Full control over integration with chi + middleware + telemetry + audit — no impedance mismatch.
- AgentLens owns the code; updates to 2025-11-25 / future specs are an in-house change.
- Narrow scope: only JSON-RPC methods that v1 actually uses (`initialize`, `ping`, `tools/list`, `tools/call`, `logging/setLevel`). Resources and prompts can be stubbed as "not supported."
- Tight control over OTel spans, audit logging, error mapping — no library-imposed patterns.

**Cons**
- 1500–2000 lines is real code. ~3 weeks of implementation + testing.
- Spec-update burden falls on AgentLens maintainers. When MCP 2026-xx-xx ships, *someone* has to implement the delta.
- Bugs in wire-protocol handling ship with AgentLens (no community shakeout).
- Lots of small conformance details (SSE resumability event IDs, Last-Event-ID semantics, initialize version negotiation) that are easy to get subtly wrong.

**When right**
- Library verification fails or conditions aren't met, *or* the team values full control over dependency minimization and has capacity for the 3-week investment.

**Complexity**: L (3 weeks). **Risk**: spec-conformance bugs; ongoing maintenance.
**Dependencies**: independent of other areas; enables the cleanest integration shape in Area 1 and Area 5.

### Alt 4C — Thin wrapper: library for JSON-RPC + transport, DIY tool layer

Use a small dep for JSON-RPC 2.0 (plenty of mature libraries: `ethereum/go-ethereum/rpc`, `sourcegraph/jsonrpc2`, `creachadair/jrpc2`) plus a lightweight SSE helper. Write the MCP-specific layer (`initialize`, `tools/list`, `tools/call`, session management, audience validation) in AgentLens.

**Pros**
- Borrows the well-understood parts (JSON-RPC wire format, transport framing) from mature libs that aren't MCP-specific.
- MCP-specific surface stays in AgentLens — full control over audit, middleware, session management.
- No dependency on a library whose MCP tracking is unclear.
- ~600–900 lines of AgentLens code on top of deps.

**Cons**
- Still takes a week+ of work on the MCP-specific layer.
- Two deps instead of one (or zero).
- SSE helper may end up DIY anyway because MCP's SSE semantics (event IDs, `retry`, Last-Event-ID polling) are non-standard enough.

**When right**
- Team wants the protocol-minutiae reuse without coupling MCP semantics to an external library's tracking of the spec.

**Complexity**: M (1.5–2 weeks). **Risk**: moderate — less code than 4B, more than 4A.
**Dependencies**: independent.

### Alt 4D — Copy-vendor: inline a small subset of a library with attribution

Find a well-written Go MCP library (Apache 2 / MIT), extract just the wire/transport/session code, inline it into `internal/mcp/wire/` with LICENSE attribution, adapt to AgentLens's shape. No external dep.

**Pros**
- Middle ground: borrows shape/tests/insight from existing work without taking a runtime dep.
- Go module graph unchanged — Trivy, CodeQL, govulncheck surface unchanged.
- Fork semantics: AgentLens owns the code; upstream updates come as manual merges when useful.

**Cons**
- License compliance requires attention (Apache 2 notice requirements are subtle; MIT is simpler).
- Manual merge from upstream is work; might fork over time.
- Choosing the right source library requires the verification Research couldn't do.

**When right**
- Library exists, is well-written, has a compatible license, but dependency risk is too high.

**Complexity**: M (1–1.5 weeks for the initial copy + adapt).
**Risk**: one-time audit burden; maintenance cost between 4A and 4B.
**Dependencies**: independent.

### Recommendation: **Alt 4B — Implement the wire protocol directly**

**Rationale**
- **Dependency verification is blocked** (research explicitly notes this). AgentLens's CI already enforces Trivy HIGH+CRITICAL gating; taking an unverified MCP dep is a direct security-posture risk.
- **Scope is bounded**: v1 needs `initialize`, `ping`, `tools/list`, `tools/call`, and hooks for `notifications/tools/list_changed`. Resources and prompts can return `not_supported`. This trims the 1500–2000 estimate to ~1000–1200 lines of focused code.
- **Integration with existing chi + OTel + auth middleware is tightest when AgentLens owns the code**. Alt 1C (HTTP loopback) depends on MCP tool handlers being well-integrated with chi; AgentLens-owned code makes that trivial.
- **Spec-update cost is real but bounded** — MCP spec revisions run 4–6 months apart, deltas are manageable. AgentLens already tracks spec versions in its parser plugin.
- **Problem statement favors conservatism**: single self-hosted binary, minimal dep graph, CGO-tight build. Adding an unverified MCP library goes against that grain.
- **Fallback path**: if implementation effort overruns and the team finds a verified library at Phase 4+ review, switching from 4B to 4A is a drop-in — the 4 tool handlers are transport-agnostic.

Ship 4B. Revisit with library survey at v1.5 when the ecosystem has matured. Use `creachadair/jrpc2` or similar for JSON-RPC is a reasonable micro-dep (part of Alt 4C) if prototyping shows JSON-RPC 2.0 framing is a time-sink — keep that door open.

---

## Decision Area 5 — Self-registration mechanism

The MCP plugin must present itself as a first-class catalog entry in the default project. Four mechanisms.

### Alt 5A — Plugin calls `store.Create()` directly at `Init()`

MCP plugin's `Init()` constructs a synthetic `CatalogEntry` with `AgentType{Protocol: "mcp", Endpoint: "agentlens:mcp-discovery", Capabilities: [MCPTool{...}×4]}`, calls `store.Create(ctx, entry)`. Existing `SQLStore.CreateEntry()` auto-assigns to default project.

**Pros**
- Minimal code — ~30 lines in Init().
- Reuses existing auto-assign-to-default path. Zero new store semantics.
- Idempotent via upsert-by-endpoint (UNIQUE constraint on `endpoint`).
- `AgentKey = SHA256("mcp" + "agentlens:mcp-discovery")` is deterministic; updates on restart replace prior registration cleanly.

**Cons**
- Plugin has to import `store` — already allowed, no arch-go issue.
- Card content is constructed imperatively in Go, not declaratively from a card JSON file. Slight maintenance mismatch with how other agent types appear in the catalog (via parser plugins consuming JSON cards).
- If the MCP plugin's tool surface changes across versions, registration updates happen at restart — fine, but not "live-reloadable."

**When right**
- Default, lowest-friction choice for infrastructure plugins that need to register themselves.

**Complexity**: S (1–2 days).
**Risk**: minimal.
**Dependencies**: compatible with any Area 1/2/3/4/6.

### Alt 5B — Publish `/.well-known/mcp/server.json` and dogfood discovery

MCP plugin serves a card JSON at `/.well-known/mcp/server.json`. AgentLens's existing discovery subsystem (the `static` source plugin or a new `self` source) polls this URL and creates the catalog entry via the normal discovery path (parser → AgentType → CatalogEntry).

**Pros**
- Dogfoods AgentLens's discovery flow — MCP server is discovered like any other agent. Conceptually elegant.
- The card is a declarative JSON file, not imperative Go code. Easier to review.
- If the server's tool surface changes at runtime, the card URL reflects it — discovery picks it up on the next poll.

**Cons**
- Requires adding a `self` source plugin (new code path for something already solved by 5A).
- Timing: discovery runs after `InitAll()`; so the catalog entry doesn't exist until the first poll. First-request race where Anya's MCP client connects, lists tools, but the server isn't yet "in the catalog." Mild UX edge.
- More moving pieces — a bug in discovery means the MCP server is invisible in the catalog even though it's running.
- The MCP plugin would have to own both the JSON card and the imperative tool handlers — duplicated source of truth for the tool shapes.

**When right**
- Design wants maximum consistency with external-agent discovery flow and accepts the new `self` source plugin cost.

**Complexity**: M (1 week including new source plugin, polling config, race handling).
**Risk**: race with startup; duplicated truth.
**Dependencies**: compatible with any Area 1/4. Slight tension with Area 6 translator (translator output → static-style card at `.well-known` is fine, but the polling-to-self indirection is extra weight).

### Alt 5C — Plugin emits a "push" event to the discovery manager

New code path: MCP plugin calls `discoveryManager.Push(entry)` at `Init()`. The discovery manager treats it like a push-source discovery event and routes it through the normal CatalogEntry creation pipeline.

**Pros**
- Single entry point through discovery — any future plugin that wants to self-register uses the same `Push()` API.
- Declaratively says "I'm a new catalog entry, process me."

**Cons**
- Adds a new API on the discovery manager that didn't exist. Scope creep for v1.
- The existing push source (if any) already uses `store.Create()` under the hood — wrapping that in a new method is a rename with extra hops.
- The "push" concept is overloaded — `SourcePush` in the data model means "an external system pushed us this entry," not "a plugin registered itself."

**When right**
- Multiple plugins need self-registration and the team wants a single named API for it. Not AgentLens's case today.

**Complexity**: M (1 week including new manager method + tests).
**Risk**: semantic ambiguity with existing push source.
**Dependencies**: independent.

### Alt 5D — No self-registration; operator creates catalog entry manually

Document that Priya creates the MCP server's catalog entry via the UI after deployment. Plugin ships without any registration logic.

**Pros**
- Simplest code (none).
- Operator has explicit control over when/how the MCP server appears in catalog.

**Cons**
- Misses the problem statement's explicit success criterion: "The MCP server appears as a catalog entry in the existing AgentLens UI." Success is gated on operator remembering to do this.
- Loses the dogfood story — AgentLens has a service discovery catalog but its own MCP server isn't in it by default.
- Priya's journey adds a step.

**When right**
- Never for v1. This alternative exists only to show the no-op baseline.

**Complexity**: S (docs only).
**Risk**: missed success criterion.
**Dependencies**: none.

### Recommendation: **Alt 5A — Direct `store.Create()` at Init()**

**Rationale**
- **Problem statement success criterion #4**: "The MCP server appears as a catalog entry in the existing AgentLens UI." 5A satisfies this automatically and reliably.
- **Codebase analysis §7** literally calls this out: "Self-registration feasible: an MCP plugin calling `store.Create()` during `Init()` registers itself... Auto-assignment to default project already happens in `SQLStore.CreateEntry()`." Path of least resistance, fully supported.
- **~30 lines of code** vs. a new source plugin (5B) or new manager API (5C).
- **Idempotency is already solved** by `AgentKey = SHA256(protocol+endpoint)` + UNIQUE endpoint constraint.
- 5B is elegant but premature — it's a nice v2 dogfood story when more plugins self-register. Not worth v1 scope.
- 5D fails the success criterion.

Ship 5A.

---

## Decision Area 6 — OpenAPI-to-MCP translator: where does v1 stop and v2 start?

Strategic arc is v1 hand-coded → v1.5 OpenAPI emission → v2 translator. This area decides how much of v2 seeps into v1.

### Alt 6A — Strict three-release sequence

v1 hand-codes 4 tools. v1.5 adopts OpenAPI emission (huma or similar, independent of MCP). v2 builds the translator and replaces hand-coded tools with generated ones. Each release is a clean independent shipment.

**Pros**
- Matches problem-statement strategic arc literally.
- Each release has tight scope; each releases independently and gets its own dedicated testing.
- No speculative abstractions in v1.
- Lowest risk per release.

**Cons**
- Throwaway work: the hand-coded v1 tool-registration code gets replaced in v2. 4 tools × some glue = minor waste.
- Risk that v1.5 / v2 get descoped under pressure and hand-coded tools become the de-facto long-term answer.

**When right**
- Team wants clean, independently-shippable releases with minimum speculative design. Default for cautious shipping.

**Complexity** (v1): S (just the 4 tools).
**Risk**: minor throwaway code at v2. Descoping risk.
**Dependencies**: compatible with any Area 1.

### Alt 6B — v1 is the translator MVP

Write a minimal OpenAPI stub (YAML/Go) describing just the 4 discovery endpoints. Ship a translator that reads this stub and generates MCP tool handlers at startup. v1 ships with the translator's bones in place; v1.5 grows the OpenAPI coverage; v2 is about wide adoption.

**Pros**
- Translator exists from day one. v2 is refinement, not a new build.
- Hand-coded tool registration goes away — everything is generated.
- Forces the Area 1 seam to be service-layer or loopback (generator output has to bind to *something*; that something is the answer).

**Cons**
- v1 scope balloons significantly — translator + OpenAPI stub + code generator architecture + 4 tools all in one release. Weeks → months.
- The OpenAPI-to-MCP translation is non-trivial: schema conversion, error-code mapping, pagination patterns, auth challenges. Solving it generally for 4 endpoints is 80% of solving it for 61 endpoints.
- If the translator has bugs, all 4 tools are broken at once.
- Problem statement explicitly says: "v1 (this design): hand-coded MCP plugin that maps 4 discovery operations onto existing REST handler logic. Ships in weeks." 6B contradicts this.

**When right**
- Team is willing to slip v1 by 2–3 months to ship the translator earlier. Bet on the translator's strategic value > the cost of the extra v1 time.

**Complexity**: L (2–3 months).
**Risk**: scope overrun; translator bugs block v1.
**Dependencies**: forces Area 1 away from 1A/1B/1E (which are hand-coded glue) toward 1C (loopback — translator emits HTTP-routed calls) or 1D (service — translator emits service-method calls).

### Alt 6C — Parallel R&D track (dual-ship)

v1 is hand-coded (Strict Alt 6A's v1). In parallel, a separate R&D track (different engineer or time-box) prototypes the OpenAPI-to-MCP translator on the same 4 endpoints but as a throwaway research spike. Both exist side-by-side; after v1 ships, the R&D output feeds v1.5 / v2 design decisions.

**Pros**
- v1 schedule unaffected. Hand-coded tools ship fast.
- Research spike validates the translator approach with real code before v1.5 commits architectural direction.
- Insights from the spike inform the v1.5 OpenAPI framework choice (huma vs. chi-openapi vs. manual registry).

**Cons**
- Requires parallel engineering bandwidth. Not always available.
- Risk that the spike's output and v1's hand-coded tools diverge; merging insights at v1.5 is its own work.
- If the spike never ships, it was overhead.

**When right**
- Team has bandwidth, wants early learning on the translator without coupling it to v1.

**Complexity** (v1): S (same as 6A). Spike: M.
**Risk**: bandwidth availability.
**Dependencies**: Area 1 for v1 can be any option; spike should use 1D (service layer) or 1C (loopback) to inform v1.5 direction.

### Alt 6D — v1 hand-coded but shaped for v2 drop-in

v1 hand-codes the 4 tools but behind a `ToolRegistry` interface that treats each tool as `{Name, Description, InputSchema, Handler}`. The handler is hand-written in v1; in v2, the translator emits the same shape. v1 is hand-coded; v2 replaces the registration source, not the abstraction.

**Pros**
- Very little extra work over Alt 6A (~50 lines of abstraction in v1).
- Makes v2 a drop-in: translator produces `ToolRegistry` entries; v1 hand-coded entries are deleted.
- No throwaway code in v1 — the abstraction is useful even without a translator (easy testing, tool discovery, metrics-per-tool registration).
- Respects the problem-statement's "ships in weeks" pace.

**Cons**
- Small speculative abstraction cost (the registry interface). Mitigated because AgentLens already uses registry patterns (capability registry in `model/capability.go`).
- Requires a small amount of forward-looking design — must define the tool-handler signature well enough that generated handlers can conform.
- If v2 is cancelled, the abstraction is minor over-engineering.

**When right**
- Team wants v1 on schedule *and* wants v2 to be a plug-in rather than a rewrite. Most cases.

**Complexity** (v1): S (4 tools + ~50 lines of registry).
**Risk**: minor over-engineering if v2 never happens.
**Dependencies**: pairs cleanly with Area 1C (loopback) — registry entries point at HTTP routes — or 1D (service) — registry entries point at service methods.

### Recommendation: **Alt 6D — Hand-coded but shaped for v2 drop-in**

**Rationale**
- **Problem statement strategic arc is explicit and paced**: v1 ships in weeks. 6B breaks this. 6A is correct but leaves v1 code that gets thrown away (minor but real). 6C requires bandwidth AgentLens may not have.
- **6D's abstraction cost is tiny** (~50 lines for a tool registry) and matches existing AgentLens patterns (capability registry in `model/capability.go`, plugin registry in kernel).
- **Makes v2 cheap**. Translator emits `ToolRegistry` entries; hand-coded entries get deleted. v2 is an hour of diff, not a week of replatforming.
- **Testing benefit immediately**: the tool registry becomes the natural place to attach per-tool metrics, audit wiring, schema validation — improving v1 itself.
- **Doesn't foreclose anything**. If the team later chooses 6B (translator-first) or 6C (parallel spike), 6D's abstraction is reusable.

Ship 6D.

---

## Cross-Area Coherence

Recommendations in concert: **1C + 2A + 3B + 4B + 5A + 6D**.

### Coherent combinations

1. **1C (HTTP loopback) + 6D (registry)**: A `ToolRegistry` entry in v1 is `{Name, Description, Schema, HTTPRoute, HTTPMethod}`. The handler builds an in-process request against the chi router. v2's translator emits `ToolRegistry` entries with the same shape — no code change in the registry layer, only in the generator. Natural fit.

2. **2A (service accounts in parties + api_client_credentials) + 3B (DCR OAuth AS)**: Service accounts and OAuth clients are *distinct* identity concepts. Service accounts authenticate by bearer API key; OAuth clients authenticate by `client_id` + PKCE flow on behalf of a human user. Both result in a JWT with `aud = MCP canonical URI`, `sub = principal ID`, `scope = permissions`. The downstream MCP tool handler doesn't care which kind of principal — it checks the token + permissions. Unified.

3. **4B (wire protocol DIY) + 1C (loopback) + 5A (direct store.Create)**: Owning the wire protocol means AgentLens's MCP plugin has full control over its request/response path. Loopback against chi is a ~30-line adapter inside that owned code. Self-registration is a 30-line `store.Create()` at Init. All of it lives cleanly in `internal/mcp/` or `plugins/mcp-server/`.

4. **6D (registry) + 4B (DIY wire)**: Since AgentLens owns the wire protocol, the `ToolRegistry` interface can be designed exactly to fit — no library-imposed tool-registration shape to fight. The MCP plugin's `tools/list` handler walks the registry; `tools/call` dispatches via the registry. Tight.

### Conflicting combinations to avoid

- **6B (v1 is translator MVP) + 1A/1B/1E (handler-import / store-direct / adapter)**: A translator needs a *seam* to emit bindings against. Handler-import and adapter-type are too ad-hoc to target; store-direct requires the translator to re-implement project-filtering and response shaping. If 6B, then Area 1 must be 1C (loopback — translator emits HTTP-route bindings) or 1D (service — translator emits service-method bindings). This is the main cross-area constraint.

- **3D (delegate to external IdP) + 2A (first-class service accounts)**: If the AS is external, service account credentials live externally too. AgentLens's `api_client_credentials` table becomes a second, conflicting source of truth. Resolve by either centralizing on the IdP (kills service-account UX for Priya) or keeping 3B/3E as the AS story.

- **4A (adopt library) + Area 5's dogfood self-registration (5B)**: A chosen library may or may not expose the `/.well-known/mcp/server.json` card shape AgentLens needs for discovery-polling. Combining "unknown library integration" + "polling discovery" multiplies unknowns. If 4A is chosen, prefer 5A to bound risk.

- **1D (full service-layer refactor) + 6A (strict three-release)**: Doing 1D is v1.5 work. Doing it in v1 while sticking to 6A's "hand-coded v1" contradicts the three-release pacing — it quietly moves v1.5 into v1. Either pick 1D + explicitly compress v1/v1.5, or pick 6A + 1C/1E.

### Summary stack (recommended)

```
Area 1: HTTP loopback (1C)             — zero-refactor seam; middleware & arch-go clean
Area 2: parties + api_client_credentials (2A) — identity + rotation + audit; Priya-ready
Area 3: DCR AS (3B)                    — Karol "paste URL → works" journey works
Area 4: DIY wire protocol (4B)         — no unverified deps; ~1000-1200 lines AgentLens owns
Area 5: store.Create() at Init (5A)    — 30 lines; success criterion #4 automatic
Area 6: hand-coded + tool registry (6D) — ships in weeks; v2 translator is drop-in
```

Total v1 effort: roughly 5–7 weeks for a single engineer, compatible with the problem statement's "ships in weeks" pacing if the DIY wire protocol is the long pole and can be partially parallelized with the OAuth AS work.

Nothing in this stack forecloses v1.5 (OpenAPI emission) or v2 (translator). The registry abstraction is the forward link; the loopback seam is the translator's binding target; the service-account + DCR identity story covers Priya and Karol without refactor.

---

*Written 2026-04-17. Phase 3 output. Recommendations are decisive; defer to user approval at Phase 4 convergence.*

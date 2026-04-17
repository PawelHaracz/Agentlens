# Design Context — MCP Management Tool for AgentLens

Phase 1 synthesis of all context sources. Produced 2026-04-17.

---

## Source Inventory

1. `codebase-analysis.md` — structure of AgentLens backend, existing MCP parser plugin, kernel lifecycle, auth model, API surface, catalog model, projects, self-registration paths
2. `research-mcp-auth-transport.md` — MCP spec (current version `2025-11-25`), Streamable HTTP transport, OAuth 2.1 Resource Server model, client UX patterns, multi-tenancy approaches, sessions-MUST-NOT-be-auth
3. `research-mcp-admin-tool-design.md` — tool vs resource vs prompt primitives, granularity trade-offs, safety patterns (`elicitation/create`, `confirm` flag, annotations), schema design, audit expectations
4. `research-mcp-gateway.md` — OpenAPI-to-MCP translation problem (blocked on GitHub-source verification; MCP spec portion verified)

---

## 1. What AgentLens is today (relevant to this design)

- **Self-hosted AI agent catalog**, Go 1.26.1 + chi + GORM (SQLite/Postgres) + React SPA (embedded). Microkernel + plugins architecture (ADR-003).
- **Domain model**: `AgentType` (protocol + endpoint, SHA256 `AgentKey`) → `CatalogEntry` (REST-facing wrapper) → `Capability[]` (discriminated by `kind`: `mcp.tool`, `mcp.resource`, `mcp.prompt`, `a2a.skill`). Capabilities persisted in one unified table. (ADR-001, ADR-008)
- **Protocol support today**: A2A parser + MCP parser. Both are *parsers* — they take a card JSON and produce `AgentType + Capability[]`. Neither hosts an MCP or A2A *server*.
- **REST admin API**: 61 endpoints covering catalog CRUD, validation, registration, import, lifecycle, probing, capability discovery, users, roles, settings, groups, projects, party-project membership.
- **Auth**: JWT (bcrypt, refresh-token) + 10 `PermXxx` constants (`catalog|users|roles|settings : {read,write,delete}`). `RequireAuth` + `RequirePermission` middleware. No API keys. No OAuth 2.1 server.
- **Projects**: `Party` archetype (`kind = person|group|project`). Default system project created at migration 007. Every catalog entry auto-assigned to default project via `SQLStore.CreateEntry()`. Project-scoped roles (`project:owner|developer|viewer`) map to existing global permissions. Per-project middleware enforcement NOT implemented — endpoints filter by query param only.
- **OpenAPI spec**: **does not exist**. Routes defined imperatively in `internal/api/router.go`. No schema generator.
- **Observability**: OTel (traces/metrics/logs) as infrastructure in `internal/telemetry/` per ADR-009. Initialized before plugins, shut down after.

## 2. What the MCP spec requires (for server design)

- **Current protocol version**: `2025-11-25`. Advertise via `MCP-Protocol-Version` header.
- **Transport**: **Streamable HTTP** for network-accessible servers. Single endpoint, POST (JSON or SSE response) + GET (server-to-client SSE). HTTP+SSE is deprecated. stdio is for subprocess-launched servers only.
- **Auth**: MCP servers are **OAuth 2.1 Resource Servers**. Required (MUST, 2025-11-25):
  - `Authorization: Bearer` on every HTTP request
  - Audience binding (RFC 8707) — `aud` claim = canonical MCP server URI; reject tokens for other audiences
  - Protected Resource Metadata at `/.well-known/oauth-protected-resource` advertising `authorization_servers[]`
  - AS Metadata at `/.well-known/oauth-authorization-server` with `code_challenge_methods_supported: ["S256"]`
  - PKCE `S256` required when client is capable
  - 401 → `WWW-Authenticate: Bearer resource_metadata="..."`
  - 403 → scope challenge `Bearer error="insufficient_scope", scope="..."`
  - **Sessions MUST NOT be used for authentication**
  - **Token passthrough to upstream MUST NOT happen** — server must obtain its own token when calling upstream
  - `Origin` validation → 403 on invalid
- **Primitives**: Tools (model-invoked actions), Resources (application/user-exposed reads), Prompts (user-triggered templates). Dynamic updates via `notifications/tools/list_changed`.
- **Safety primitives**: `elicitation/create` is the spec-native way to ask for user confirmation on destructive actions. `confirm: true` flag is the fallback for clients that don't support elicitation.
- **Schema dialect**: JSON Schema 2020-12 (SEP-1613).
- **Tool name format**: short identifiers (SEP-986).

## 3. What the research says about tool design

- **One tool per `(resource, verb)`**: avoid omnibus `agentlens_api(method, path, body)` tools. They defeat schema, naming, and annotation principles.
- **Fine granularity** (e.g., `catalog_create`, `catalog_delete`) over coarse (`manage_catalog`) — LLMs discover and pick correctly only with narrowed inputs.
- **Read-only mode** filters `tools/list` via server-side `readOnlyHint:true`; re-emit list on mode toggle via `notifications/tools/list_changed`.
- **Destructive ops** use `elicitation/create` primarily; `confirm: true` as fallback.
- **Schema**: narrow enums, required fields explicit, avoid `anyOf`/`oneOf`, clear tool descriptions.
- **Pagination**: opaque cursors preferred over offset (LLMs struggle with offsets).
- **Audit**: token's `sub` + `scope` is all the server gets about the caller; log `user_id`, `tool_name`, `scope_granted`, `project_id`, `arguments` (sanitized), `outcome`.

## 4. Three alternatives the user floated

1. **Standalone MCP server** exposing AgentLens management operations
2. **Auto-registered into default project** on startup (variant of #1 with integration twist)
3. **Generalized MCP gateway** that wraps AgentLens REST/OpenAPI surface into MCP tools automatically

## 5. Cross-reference insights

### A. Gateway alternative has an OpenAPI prerequisite

AgentLens has no OpenAPI spec today. A general-purpose OpenAPI-to-MCP gateway would require:
1. Adopting an OpenAPI emission framework (huma is a plausible candidate) — a **cross-cutting refactor of all chi handlers**, unrelated to MCP
2. Building or adopting an OpenAPI-to-MCP translator
3. Shipping all 61 endpoints as tools — LLM-hostile tool-list bloat (research shows fine granularity is preferred, but this goes *too* fine and undifferentiated)
4. Adding a safety layer over the 9 destructive endpoints

Estimated cost: 3-6 months before first useful tool ships.

**In contrast, a hand-coded MCP server over a curated 15-25 tool subset is weeks of work**.

### B. Auto-register-into-default-project is a natural extension

AgentLens already auto-assigns catalog entries to the default project (`SQLStore.CreateEntry()`). An MCP plugin calling `store.Create()` during `Init()` registers itself as a `CatalogEntry` with `AgentType{Protocol: "mcp", Endpoint: "agentlens:mcp-management", Capabilities: [MCPTool{...}]}`. Zero new code paths needed.

This doesn't conflict with alternatives 1 or 3 — it's a cross-cutting feature that could apply to any of them.

### C. AgentLens is both Resource Server and Authorization Server

Per MCP spec 2025-11-25, MCP server needs an AS it points to. AgentLens could:
- Be its own AS (minimal new surface: `/oauth/authorize`, `/oauth/token`, optionally `/oauth/register`; reuse existing JWT issuance)
- Delegate to an external IdP (bigger integration cost; not needed for self-hosted use case)

Self-AS is the clear MVP. Keeps ops simple; matches the self-hosted product positioning.

### D. Transport architecture: in-process vs. separate listener

Two viable designs:
1. **Shared HTTP listener**: mount MCP endpoint (`/api/mcp`) on the same chi router as REST API. Reuses all middleware (Recovery, Logger, CORS, RequestID, otelhttp). Single port, single TLS cert, single auth surface.
2. **Separate listener**: MCP plugin opens its own port (e.g., `:9003`). Clean isolation but double ops burden.

Shared is clearly better for the self-hosted audience. The MCP plugin becomes a route contributor via `kernel.RegisterRoutes("/api/mcp", handler)`.

### E. Plugin type verdict

Per codebase analysis: MCP server is **infrastructure**, not parser/source/cardstore. Two integration patterns both workable:
- `internal/mcp/` (parallel to `internal/telemetry/`, initialized in composition root)
- Custom plugin modeled after `plugins/health/` (plugin lifecycle + background listener)

Plugin model is more consistent with the existing surface (parsers, sources, health are all plugins). Infrastructure model is more consistent with ADR-009 (telemetry is infra because it runs before other plugins). **Leaning plugin** because MCP server doesn't need to run before other plugins — it depends on them (needs store, auth, route registration).

### F. Safety surface

9 destructive endpoints in scope. Options:
- **Exclude all destructive ops from MCP MVP** (scope-gated: only expose tools the user's token has permission for; and ship with a `read-only mode` toggle by default)
- **Include with `elicitation/create` confirmation** (rich UX; requires client support)
- **Include with `confirm: true` flag** (works everywhere; weaker UX)
- **Ship in pairs** (e.g., `catalog_prepare_delete` returns a plan; `catalog_confirm_delete` executes with a nonce from the preview)

### G. Multi-project scoping — two viable MCP patterns

- **Path-scoped endpoints**: `/projects/{project_id}/mcp` → distinct OAuth resources per project. Spec-sanctioned (RFC 8707). Clean isolation; each client-session is bound to one project.
- **Single endpoint + project arg**: `project_id` as required tool argument OR as token scope claim validated per request. Works with existing single-listener model; more flexible for users who switch projects.

## 6. Implications for the design

1. **Favor a focused MCP server over a gateway for v1**. AgentLens has no OpenAPI to wrap; adopting one is orthogonal work. Ship a hand-coded MCP server with curated tools. If a gateway is desired later, it's additive (Phase 2+) and depends on OpenAPI adoption.
2. **Position the MCP server as an AgentLens plugin** (like `plugins/health/`), not as a separate process. Integrates into the chi router, reuses auth middleware and OTel instrumentation.
3. **Self-registration as a default-project catalog entry is easy and natural** — it gives the MCP tool a first-class presence in the UI and in the capability index. Do it.
4. **AgentLens becomes its own OAuth 2.1 Authorization Server**. Minimal new endpoints (`/oauth/authorize`, `/oauth/token`, optional `/oauth/register`). Reuse JWT issuance. New `aud` claim = canonical MCP URI; new `mcp:*` scopes mapped to existing RBAC.
5. **Destructive operations get spec-native safety** — `elicitation/create` first, `confirm: true` flag fallback. Ship in read-only mode by default; make destructive tools opt-in via config.
6. **Multi-project strategy needs a decision in Phase 2**. MVP can ship single-endpoint + user's session scoped to their default project; path-scoped endpoints are a Phase 2 enhancement once power-users need them.

## 7. Open questions for Phase 2 (problem exploration)

- **Who is this for?** Which user(s) does the MCP tool primarily serve — developers managing their local catalog via Claude Code/Cursor, ops engineers automating via CI, or product managers using Claude.ai? This shapes scope and UX.
- **What workflows?** Is the goal "ask Claude to register an agent" (registration ergonomics) or "ask Claude to audit my catalog" (read-heavy analysis) or "fully automate catalog administration" (ops)?
- **Transport scope for MVP** — only Streamable HTTP (remote), or also stdio (for running AgentLens locally with a local MCP client subprocess)?
- **Project-scoping semantics** — default project on session or path-scoped endpoints?
- **Destructive operations in MVP** — exclude, include with confirmation, or include but gated by config/mode?
- **Programmatic access** — do we need OAuth Client Credentials for CI/CD tokens at MVP?
- **Gateway path as explicit non-goal for now** — can we agree to defer it to Phase 2+ (conditional on adopting huma/OpenAPI)?

---

*Written 2026-04-17. Phase 1 complete.*

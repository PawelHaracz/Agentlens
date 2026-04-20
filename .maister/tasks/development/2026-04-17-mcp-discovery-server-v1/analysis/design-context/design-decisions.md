# Design Decisions — MCP Discovery Server for AgentLens

Phase 5 output. Approved 2026-04-17.

Full alternatives and trade-off analysis in `analysis/alternatives.md`. This document records the selected approach, rationale, and trade-offs accepted.

---

## Stack summary

| Area | Choice | One-liner |
|---|---|---|
| 1. REST→MCP seam | **Alt 1C** In-process HTTP loopback | Tool handlers invoke chi router via `ServeHTTP(recorder, req)`; all middleware fires. |
| 2. Service-account identity | **Alt 2A** parties + api_client_credentials | `parties.kind = 'service_account'` + new credentials table with bcrypt hashes + rotation/audit fields. |
| 3. Authentication / AS | **Alt 3F-Dex** Bundled Dex as federation AS | AgentLens owns authorization; Dex owns auth federation (OIDC/SAML/OAuth); basic auth + API keys are parallel paths. |
| 4. Wire protocol | **Alt 4B-Plugin** DIY wire as replaceable plugin | `plugins/mcpserver/` modeled after `plugins/health/`; `WireImpl` interface for future library swap; enable flag. |
| 5. Self-registration | **Alt 5A** `store.Create()` at plugin Init | ~30 lines. Synthetic `CatalogEntry`, auto-assigns to default project, idempotent via `AgentKey`. |
| 6. Translator sequencing | **Alt 6D** Hand-coded behind `ToolRegistry` interface | v1 tools registered as `{Name, Desc, InputSchema, Handler}`; v2 translator emits same shape — drop-in. |

---

## Rationale (decision-by-decision)

### Area 1 — In-process HTTP loopback (1C)

- **Anya's p95 < 100ms budget** is non-negotiable. Loopback overhead is <1ms per call; SQL endpoints dominate actual latency. Budget met.
- **Middleware reuse** is automatic: `RequireAuth`, `RequirePermission`, `otelhttp`, `Recovery`, `Logger`, `RequestID` all fire without re-implementation.
- **Parity with REST** is a safety property for Priya — UI and MCP see the same results, no silent divergence on new filters.
- **No arch-go violations** — plugin imports `http.Handler` from kernel, nothing from `api/*`.
- **Keeps v2 options open** — translator can bind to the same loopback seam, or to services if v1.5 refactors.

**Trade-off accepted**: ~50 lines of error-mapping glue (401→401+scope-challenge, 403→insufficient_scope, 404→tool-error, 409→structured error). Documented in Phase 6.

### Area 2 — parties + api_client_credentials (2A)

- **Priya's journey** requires rotation + instant revocation + audit-by-principal + project-scoped membership. Only 2A supports all cleanly.
- **Multi-key rotation** — issue new, deploy, revoke old. Requires multiple rows per account; flag-on-user (2B) breaks this.
- **ADR-001 Party archetype loyalty** — service accounts are first-class Parties alongside person/group/project.
- **Audit shape**: `principal_id = party_id` applies uniformly across kinds.

**Trade-off accepted**: one new enum value on `parties.kind` + new table. ~1 week of schema + CRUD + admin UI work.

### Area 3 — Bundled Dex as federation AS (3F-Dex)

- **Karol's "paste URL → works" journey** is the OAuth UX validator. Dex supports DCR and enterprise connectors (SAML, OIDC, LDAP, GitHub, Google, OAuth 2.0) out of the box — AgentLens doesn't need to build any of that.
- **Self-hosted simplicity retained** — Dex ships in the Helm chart + docker-compose bundle; operators get federation without running a separate IdP.
- **Operators choose**: Dex enabled (default) or disabled (basic auth + API keys only) via config flag.
- **Authorization stays in AgentLens** — Parties, RBAC, project memberships unchanged; only authentication federation offloads to Dex.
- **Basic auth (username/password) preserved** as break-glass path for admin bootstrap and operators who don't want federation.

**Trade-offs accepted**:
- Runtime dependency on Dex (deployed as sidecar in Helm, adjacent container in compose).
- New `user_external_identities` mapping table (`provider, external_sub, user_id`).
- JIT provisioning flow (configurable auto-create-on-first-login vs. admin-approval-queue).
- Health endpoint extended to report `federation.enabled/reachable` + last-checked timestamp.
- Dex DCR compliance with MCP 2025-11-25 requires verification during Phase 6 (gating item).

### Area 4 — DIY wire protocol as replaceable plugin (4B-Plugin)

- **Dependency verification was blocked** during Phase 1 research — taking an unverified Go MCP library risks AgentLens's Trivy HIGH+CRITICAL gating and CGO-tight build.
- **Scope trimmed** — v1 needs `initialize`, `ping`, `tools/list`, `tools/call`, `notifications/tools/list_changed`. Resources/prompts return `not_supported`. ~1000-1200 lines.
- **Plugin encapsulation** — lives in `plugins/mcpserver/`, modeled after `plugins/health/`. Completely disable-able via config flag; nothing else depends on plugin presence.
- **`WireImpl` interface** — internal abstraction so Alt 4A (library) or 4C (thin wrapper) can drop-in replace 4B if/when verified. One line swap in composition root.
- **Plugin owns its routes** — `/api/mcp` (tool calls), `/api/mcp/status` (health/metrics echo), `/.well-known/oauth-protected-resource` (spec-mandated). Optional `/api/mcp/admin/*` deferred to Phase 6 decision.

**Trade-off accepted**: 3 weeks implementation + in-house spec-update burden. Fallback to library is always available.

### Area 5 — store.Create at Init (5A)

- **Success criterion #4** ("MCP server appears as catalog entry in UI") is satisfied automatically.
- **30 lines of code**. Idempotent via `AgentKey = SHA256("mcp" + "agentlens:mcp-discovery")` + UNIQUE endpoint constraint.
- **Auto-assign to default project** already happens in `SQLStore.CreateEntry()`.

**Trade-off accepted**: imperative Go rather than declarative JSON card. No runtime tool-surface reloads (restart-to-update). Minor. 5B's dogfood-via-discovery is a nice v2 story when more plugins self-register.

### Area 6 — Hand-coded + ToolRegistry (6D)

- **Ships on v1 schedule** — no speculative translator build.
- **~50 lines of abstraction** over strict 6A, mirroring existing registry patterns (`model/capability.go` capability registry, `kernel` plugin registry).
- **v2 translator is drop-in** — emit `ToolRegistry` entries; delete hand-coded entries. Hour of diff, not week of replatforming.
- **Immediate benefits** — registry is the natural home for per-tool metrics, schema validation, audit wiring.

**Trade-off accepted**: small speculative abstraction if v2 never ships. Cost is bounded (~50 lines) and the abstraction improves v1 regardless.

---

## Cross-area coherence (verified)

- **1C + 6D** → ToolRegistry entries carry `{Name, Desc, Schema, HTTPRoute, HTTPMethod}`; v2 translator emits same shape without touching registry.
- **2A + 3F-Dex** → Two distinct auth paths (API key direct; Dex JWT via OIDC) produce the same downstream context (`principal_id` + `permissions`); MCP handlers are path-agnostic.
- **3F-Dex + 1C** → `RequireAuth` middleware validates Dex JWTs (verifies `iss` = Dex, `aud` = MCP canonical URI, signature via Dex JWKS). Loopback is transport-neutral.
- **4B-Plugin + 5A + 6D** → Plugin owns wire + self-registers + hosts ToolRegistry. One cohesive unit, disable-able, each seam independently swappable.

No conflicts detected.

---

## Rough v1 effort (single engineer)

| Work item | Effort |
|---|---|
| Wire protocol + Streamable HTTP transport | 3 weeks |
| Service-account identity (migration, CRUD, admin UI) | 1 week |
| Dex integration (config, JWKS, user_external_identities, JIT, health) | 1.5 weeks |
| 4 tools + ToolRegistry + HTTP loopback glue | 1 week |
| Self-registration + Protected Resource Metadata + /api/mcp/status | 0.5 week |
| Helm/docker-compose updates with Dex bundle | 0.5 week |
| Integration tests + docs | 1 week |
| **Total** | **~8-9 weeks** |

---

## Items deferred to v1.5 / v2

- OpenAPI emission framework adoption (huma or equivalent) — v1.5
- CIMD (Client ID Metadata Documents) support — v1.5+
- OAuth Client Credentials extension for CI/CD (RFC 7523 JWT Bearer Assertions) — v1.5
- Path-scoped MCP endpoints per project (`/projects/{id}/mcp`) — v1.5+ if demand
- Generalized OpenAPI-to-MCP translator — v2
- Dex federation expansion (multiple connectors, custom claim mapping UI) — as needed

---

*Written 2026-04-17. Phase 5 complete.*

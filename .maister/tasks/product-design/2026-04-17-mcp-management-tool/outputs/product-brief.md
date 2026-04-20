# Product Brief — MCP Discovery Server for AgentLens

Assembled 2026-04-17. Handoff artifact for development orchestration.

This brief is a condensed summary document. Detailed analysis lives in the linked artifacts under `analysis/`.

---

## Layer 0 — Core Brief

### Problem statement

AgentLens is a service discovery platform for AI agents, but today the only way an LLM-powered app can query the catalog is through AgentLens's REST API. This requires app developers to write glue code, handle auth, and translate responses into LLM tool-call results. Meanwhile, every major MCP client (Claude.ai, Claude Code, Cursor, VS Code + Copilot) natively consumes MCP tools without glue. This integration gap makes AgentLens harder to adopt in production LLM apps than it needs to be.

Strategically, the goal is a generalized **OpenAPI-to-MCP translator** (v2). AgentLens will be the first consumer (eating its own dogfood), but the translator's value extends beyond AgentLens.

Full problem statement: `analysis/problem-statement.md`.

### Target users

- **Primary — Anya (backend LLM app developer)**: production LLM service that needs runtime agent discovery. Drives p95 < 100ms and dual-auth requirements.
- **Secondary — Karol (IDE/Claude.ai developer)**: discovers agents while coding; drives OAuth "paste-URL-works" UX.
- **Enabling — Priya (platform operator)**: provisions service accounts, manages project boundaries, runs audits.

Persona cards + journeys: `analysis/personas.md`.

### Feature overview

A **read-only MCP discovery server** embedded in AgentLens as a plugin. Four tools covering agent search, per-agent lookup, capability browse, and raw-card fetch. Dual auth: service-account API keys (backend apps) + OAuth 2.1 via Dex (human clients). Principal-based project scoping with default-project-readable-by-everyone tier. Self-registers as a first-class catalog entry at startup.

### Constraints (non-negotiable)

- **Spec compliance**: MCP protocol 2025-11-25. Streamable HTTP, `Origin` → 403, `MCP-Protocol-Version` echo, `MCP-Session-Id` rules, sessions-MUST-NOT-be-auth, no token passthrough.
- **Security**: reuse existing JWT + bcrypt + RBAC; audience binding (`aud` = canonical MCP URI) prevents REST-UI tokens from working on MCP; PKCE S256; service-account secrets bcrypt cost 12, shown once.
- **Architecture**: plugin pattern modeled after `plugins/health/`. Respects arch-go layer boundaries (plugins import only kernel + foundation).
- **Observability**: OTel spans/metrics under `agentlens_mcp_*`; structured audit log per tool invocation.
- **Performance**: p95 < 100ms measured against ≥1000-entry catalog on stock SQLite.
- **Deployment**: single binary, plugin completely disable-able via config.

### Success criteria (v1)

1. Backend LLM app connects to `/api/mcp` with service-account API key; calls `agent_search("pdf extraction")`; receives JSON-RPC result across accessible projects + default project.
2. Developer adds AgentLens as Claude Code / Cursor / Claude.ai Custom Connector; completes OAuth via Dex; uses the same 4 tools.
3. Tool p95 < 100ms against 1000-entry catalog.
4. MCP server appears as a catalog entry in the AgentLens UI (self-registered).
5. Every tool invocation produces an audit log entry with principal, tool, scoped projects, outcome.

### Acceptance criteria (condensed from spec)

- [ ] `mcp_server.enabled=true` + `federation.enabled=true` + `federation.provider=dex` boots cleanly.
- [ ] `/api/mcp/status` returns correct JSON authenticated and unauthenticated.
- [ ] `/.well-known/oauth-protected-resource` returns Dex issuer URL in `authorization_servers`.
- [ ] Service-account created via UI returns one-time secret in `agentlens_sk_<id>.<secret>` format; secret is bcrypt-hashed at rest.
- [ ] `Authorization: Bearer agentlens_sk_...` succeeds; revoked key fails 401.
- [ ] Dex-issued JWT with correct `aud` succeeds; wrong `aud` fails 401.
- [ ] `initialize` → `tools/list` → `tools/call` sequence works over Streamable HTTP.
- [ ] All 4 tools return expected shapes; errors map to MCP error codes per §5.9.
- [ ] Self-registered catalog entry visible in UI after first boot, removed/offline on plugin stop.
- [ ] JIT provisioning default=false produces admin-approval-queue entries on first federated login.
- [ ] Migration 008 runs on both SQLite and PostgreSQL.
- [ ] `make test`, `make e2e-test`, `make arch-test`, `helm lint --strict` all pass.

### Out-of-scope for v1 (explicit non-goals)

- Admin / write / destructive MCP operations (user mgmt, role mgmt, catalog CRUD, settings).
- Semantic search / embeddings.
- Per-project path-scoped MCP endpoints (`/projects/{id}/mcp`).
- stdio transport.
- Sidecar / separate-binary deployment.
- OpenAPI emission framework adoption for AgentLens REST surface.
- Generalized OpenAPI-to-MCP translator (v2 scope).
- CIMD (Client ID Metadata Documents); OAuth Client Credentials RFC 7523 JWT Bearer Assertions.

### Strategic arc

- **v1** (this design): hand-coded 4 discovery tools + plugin-owned wire + Dex federation + service-account identity. Ships in 8-9 weeks (single engineer).
- **v1.5**: adopt OpenAPI emission framework (e.g., huma) for REST surface. Independent of MCP.
- **v2**: generalized OpenAPI-to-MCP translator consuming v1.5's spec. Replaces hand-coded tools with generated ones.

---

## Layer 1 — Persona summary

Full detail in `analysis/personas.md`.

| Persona | Role | Journey driver |
|---|---|---|
| **Anya** (primary) | Backend LLM app developer | Runtime agent discovery with p95<100ms; service-account + env-var setup; no glue code |
| **Karol** (secondary) | IDE/Claude.ai developer | Paste URL to Custom Connector → OAuth via Dex → tools work |
| **Priya** (enabling) | Platform operator | Create/revoke service accounts; manage project memberships; approve external identities; audit usage |

---

## Layer 2 — Design decisions summary

Full alternatives catalog: `analysis/alternatives.md`. Selected approaches + rationale: `analysis/design-decisions.md`.

| Area | Choice | One-liner |
|---|---|---|
| 1. REST→MCP seam | **1C** In-process HTTP loopback | Tools invoke chi router via `ServeHTTP(recorder, req)`; all middleware fires automatically. |
| 2. Service-account identity | **2A** parties + api_client_credentials | New `parties.kind='service_account'` + credentials table with rotation/audit fields. |
| 3. Authentication / AS | **3F-Dex** Bundled Dex federation | AgentLens owns authorization; Dex owns authentication federation; basic auth + API keys as parallel paths. |
| 4. Wire protocol | **4B-Plugin** DIY wire as replaceable plugin | `plugins/mcpserver/` with `WireImpl` interface for future library swap; enable flag. |
| 5. Self-registration | **5A** `store.Create()` at Init | ~30 lines. Idempotent via AgentKey. Auto-assigns to default project. |
| 6. Translator sequencing | **6D** Hand-coded behind `ToolRegistry` | v1 tools register as `{Name, Description, InputSchema, Handler}`; v2 translator emits same shape — drop-in. |

Trade-offs accepted (from `design-decisions.md`):

- ~50 lines error-mapping glue for HTTP→MCP status translation.
- One new `parties.kind` enum value + `api_client_credentials` table (~1 week).
- Runtime dependency on Dex (bundled in Helm + docker-compose); Dex DCR spec compliance gated in implementation.
- 3 weeks DIY wire protocol; in-house spec-update burden.
- Imperative Go self-registration (not declarative card JSON).
- ~50 lines speculative `ToolRegistry` abstraction if v2 never ships.

---

## Layer 3 — Mockup references

Phase 7 (visual prototyping) skipped. Backend-focused design; no user-facing UI in the MCP data path. Admin UI for service-account management (§8.3.3 of spec) follows existing `web/src/routes/admin/` conventions — mockups not required; shadcn/ui components suffice.

---

## Specification (implementation-ready)

Full specification in `analysis/feature-spec.md`. 8 sections:

1. **Data Model & Migrations** — `parties.kind=service_account`, `api_client_credentials` table, `user_external_identities` table, migration 008, secret format `agentlens_sk_<id>.<secret>`, JIT admin-approval-queue default.
2. **Configuration** — typed `FederationProvider` discriminator, single-instance federation (`provider`+`instance`+`common`), `MCPServerConfig`, env-var overrides, fail-fast validation.
3. **Authentication Flows** — three paths normalized to `Principal`: API key / local JWT / federation JWT. Spec-compliant 401 + WWW-Authenticate, scope challenges, Protected Resource Metadata, timing-safe.
4. **Authorization Model** — principal+project scoping, `ScopeByAccessibleProjects` middleware, `CatalogFilter.ProjectIDs`, default-project public-reads rule, 3 new `service_accounts:*` permissions.
5. **MCP Plugin & Wire Protocol** — `plugins/mcpserver/` package layout, `WireImpl` interface + DIY impl, Streamable HTTP (POST+GET), session management, 5 JSON-RPC handlers, HTTP loopback adapter, error code mapping, `/api/mcp/status`.
6. **ToolRegistry & 4 Tool Specs** — `ToolEntry` struct, JSON Schema 2020-12 input schemas, LLM-facing descriptions for `agent_search`, `agent_get`, `capabilities_list`, `agent_card`. v2 translator compatibility.
7. **Self-Registration & Observability** — idempotent `store.Create()` at Init, OTel span hierarchy, `agentlens_mcp_*` metrics, structured audit log with secret scrubbing, federation health loop, `/readyz` extension, operator alert recipes.
8. **Deployment & Operations** — Helm chart 0.3.0 (Dex as subchart), values, render validation, CI values file, docker-compose reference, service-account admin REST routes + UI, external-identity mapping routes, bootstrap UX, upgrade path, PR-checklist coverage.

### v1 effort estimate

| Work item | Effort |
|---|---|
| Wire protocol + Streamable HTTP transport | 3 weeks |
| Service-account identity (migration, CRUD, admin UI) | 1 week |
| Dex integration (config, JWKS, user_external_identities, JIT, health) | 1.5 weeks |
| 4 tools + ToolRegistry + HTTP loopback glue | 1 week |
| Self-registration + Protected Resource Metadata + `/api/mcp/status` | 0.5 week |
| Helm/docker-compose with Dex bundle | 0.5 week |
| Integration tests + docs | 1 week |
| **Total** | **~8-9 weeks (single engineer)** |

---

## References (all design artifacts)

| File | Purpose |
|---|---|
| `analysis/problem-statement.md` | Full problem, in/out-of-scope, constraints, success criteria, strategic arc |
| `analysis/personas.md` | Anya, Karol, Priya — goals, pain points, journeys |
| `analysis/design-context.md` | Unified context synthesis (codebase + MCP spec + research) |
| `analysis/codebase-analysis.md` | AgentLens structure (4 Explore agents' findings consolidated) |
| `analysis/research-mcp-auth-transport.md` | MCP 2025-11-25 spec — transport, auth, multi-tenancy, observability |
| `analysis/research-mcp-admin-tool-design.md` | Tool design principles (primitive selection, safety, schema, audit) |
| `analysis/research-mcp-gateway.md` | OpenAPI-to-MCP gateway landscape (partial — GitHub was blocked during research) |
| `analysis/alternatives.md` | Full decision-area catalog (22 alternatives across 6 areas) |
| `analysis/design-decisions.md` | Selected approaches + rationale + trade-offs accepted |
| `analysis/feature-spec.md` | Implementation-ready spec (8 sections) |

---

## Handoff to development

To start implementation:

1. Clear context or start a new session.
2. Run: `/maister:development .maister/tasks/product-design/2026-04-17-mcp-management-tool/`

The development orchestrator will detect the product-design task type and route the brief + spec through planning → implementation → verification phases.

Suggested task-group breakdown for planning phase:

- **Group A: Data layer** — migration 008, service-account CRUD, api_client_credentials store, external-identity store.
- **Group B: Auth pipeline** — Principal abstraction, dispatch, API-key validation path, federation provider interface + Dex impl, PRM endpoint.
- **Group C: MCP plugin + wire** — plugin scaffold, DIY wire protocol, session store, JSON-RPC handlers, /status endpoint.
- **Group D: ToolRegistry + 4 tools** — registry, loopback adapter, error mapping, 4 tool entries.
- **Group E: Authorization middleware** — `ScopeByAccessibleProjects`, `CatalogFilter.ProjectIDs`, `service_accounts:*` permissions seed.
- **Group F: Self-registration + observability** — store.Create at Init, OTel spans/metrics, audit logger, federation health loop, /readyz extension.
- **Group G: Admin UI** — ServiceAccountsPage, ServiceAccountDetailPage, PendingIdentitiesPage + backing REST routes.
- **Group H: Deployment** — Helm 0.3.0 + Dex subchart, docker-compose reference, ci-values, docs.

Groups A, B, C, D are sequential dependencies. E, F, G, H can parallelize after C ships.

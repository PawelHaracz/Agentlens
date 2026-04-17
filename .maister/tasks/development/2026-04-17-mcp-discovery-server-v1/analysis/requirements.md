# Requirements — MCP Discovery Server v1

## Initial description

Implement MCP Discovery Server v1 per the product-design brief at `.maister/tasks/product-design/2026-04-17-mcp-management-tool/`.

**Scope** (handed off from product-design):
- Read-only MCP server embedded in AgentLens as a plugin (`plugins/mcpserver/`).
- 4 discovery tools: `agent_search`, `agent_get`, `capabilities_list`, `agent_card`.
- Dual auth: service-account API keys (`agentlens_sk_<id>.<secret>`) + OAuth 2.1 via bundled Dex federation.
- New identity type: `parties.kind=service_account` + `api_client_credentials` table + `mcp_sessions` table (migration 010).
- Principal abstraction normalizing 3 auth paths (API key / local JWT / federation JWT).
- Streamable HTTP transport per MCP 2025-11-25 spec.
- HTTP loopback adapter: tools invoke chi router via `chi.ServeHTTP(recorder, req)` using new `Kernel.Router()` accessor.
- `ToolRegistry` abstraction prepared for v2 OpenAPI-to-MCP translator compatibility.
- Self-registration as catalog entry at plugin `Init()`.
- Admin UI for service-account + external-identity management.
- Helm chart bump 0.2.0 → 0.3.0 with Dex as conditional subchart.
- `docker-compose.dev.yml` with Dex service for local development.

## Persona & user-journey requirements (from design-context/personas.md)

| Persona | Role | Primary requirement |
|---|---|---|
| **Anya** (primary) | Backend LLM app dev | Runtime agent discovery with p95<100ms; service-account + env-var setup; no glue code |
| **Karol** (secondary) | IDE/Claude.ai developer | Paste AgentLens URL into Custom Connector → OAuth via Dex → 4 tools work |
| **Priya** (enabling) | Platform operator | Create/revoke service accounts; manage project memberships; approve federated identities; audit usage |

## Existing code reuse (from codebase-analysis.md)

| Concern | Template file | Pattern |
|---|---|---|
| Plugin scaffold | `plugins/health/health.go` | Plugin struct, New(cfg) factory, Init DI, goroutine run loop, Stop no-op, NewForTest helper, minimal interface DI |
| Migration (dual-dialect + data seed) | `internal/db/migrations.go` migration007 | AutoMigrate + CREATE UNIQUE INDEX IF NOT EXISTS + INSERT…WHERE NOT EXISTS + tx.Name()=="postgres" branch |
| Auth middleware | `internal/api/auth_middleware.go` | `RequireAuth(jwtService)` + `RequirePermission(perm)` curry; Bearer-or-cookie extraction; context helpers |
| Permission constants | `internal/auth/permissions.go` | Const strings `resource:action`; seeded in migration003 roles |
| Party model | `internal/model/party.go` + `internal/store/party_store.go` | `PartyKind` enum; CreatePersonForUser idempotent; migration007 seeds default project |
| Catalog read | `internal/store/sql_store_query.go` + `internal/model/agent.go:MarshalJSON` | Filter struct with ProjectID; LEFT JOIN catalog_project_memberships; flat JSON |
| store.Create | `internal/store/sql_store.go` | AgentKey SHA256(protocol+endpoint); auto-assign to default project in txn; Source=SourcePush protected |
| Config discriminator | `internal/config/config.go` DatabaseConfig | Typed `{Dialect, SQLite, Postgres}` + switch branch in main.go + applyXxxEnv funcs |
| Telemetry metrics | `internal/telemetry/metrics.go` HealthMetrics | `agentlens.<component>.<metric>`; Int64Counter/Float64Histogram/ObservableUpDownCounter |
| Admin UI CRUD | `web/src/pages/SettingsPage.tsx` UsersTab | shadcn Tabs+Table+Dialog; useState-driven forms; lucide-react icons |
| Helm subchart | `deploy/helm/agentlens/Chart.yaml` | bitnami/postgresql ~16.x with `condition: postgresql.enabled` + Chart.lock pin + values threading |
| Graceful shutdown | `internal/server/server.go` + `cmd/agentlens/main.go` defer chain | LIFO: plugins stop → telemetry shutdown |
| E2E helpers | `e2e/tests/helpers.ts` | `loginViaAPI`, `loginViaUI`, `authHeader`, `BASE`, port 18080 |

## Visual assets

None provided. Backend-dominant feature; admin UI follows existing `SettingsPage.tsx` conventions (shadcn/ui primitives). `analysis/visuals/` intentionally empty.

## Functional requirements (from feature-spec 8 sections, condensed)

### Data model & migrations (§1)
- Migration 010 adds: `parties.kind='service_account'` enum value, `api_client_credentials` table, `mcp_sessions` table, `user_external_identities` table.
- Dual-dialect (SQLite + PostgreSQL) per backend standards.
- Secret format: `agentlens_sk_<client_id>.<raw_secret>`; bcrypt hashed at rest.
- JIT admin-approval-queue default=false.

### Configuration (§2)
- Typed `MCPServerConfig{Enabled, Bind, AllowedOrigins[], SessionTTL, Resource, PublicURL}`.
- Typed `FederationConfig{Enabled, Provider (discriminator="dex"), Dex, Common}`.
- Env overrides: `AGENTLENS_MCP_*`, `AGENTLENS_FEDERATION_*`.
- Fail-fast `Validate()` methods.

### Authentication flows (§3)
- Three paths normalized to `Principal`: API key / local JWT / federation JWT.
- Spec-compliant 401 + `WWW-Authenticate` challenges.
- `/.well-known/oauth-protected-resource` returns Dex issuer URL.
- Audience binding: Dex-issued `aud` = MCP canonical URI (REST-UI tokens do NOT work on MCP).
- PKCE S256 required.

### Authorization model (§4)
- `ScopeByAccessibleProjects` middleware uses Principal + catalog filter.
- New permissions: `service_accounts:read`, `service_accounts:write`, `service_accounts:revoke`.
- Default project = public-reads rule preserved.

### MCP plugin & wire (§5)
- `plugins/mcpserver/` layout modeled on `plugins/health/`.
- `WireImpl` interface + DIY Streamable HTTP impl (POST + GET, session management per MCP-Session-Id).
- `MCP-Protocol-Version` echo, Origin→403 scoped middleware.
- Sessions: DB-backed (`mcp_sessions`), soft-delete revocation.
- 5 JSON-RPC handlers: `initialize`, `tools/list`, `tools/call`, `ping`, `resources/list` (stub).
- HTTP loopback adapter via `Kernel.Router()` accessor.
- Error code mapping per §5.9.
- `/api/mcp/status` authenticated + unauthenticated branches.

### ToolRegistry & 4 tools (§6)
- `ToolEntry{Name, Description, InputSchema (JSON Schema 2020-12), Handler}`.
- Tools: `agent_search(query, protocol?, project?, limit?)`, `agent_get(id)`, `capabilities_list(agent_id)`, `agent_card(agent_id)`.
- LLM-facing descriptions optimized for tool selection.
- v2 translator-compatible shape.

### Self-registration & observability (§7)
- Idempotent `store.Create()` at Init creates MCP plugin's own catalog entry.
- OTel spans: `agentlens.mcp.*` (per-tool `agentlens.mcp.tool_call`, `agentlens.mcp.session.*`).
- Metrics: `agentlens.mcp.invocations.total`, `agentlens.mcp.tool_calls.total{tool,status}`, `agentlens.mcp.dex_health.status` gauge, `agentlens.mcp.active_sessions` gauge.
- Structured audit log per tool invocation with principal + secret scrubbing.
- Federation health loop + `/readyz` chain (DB ping + Dex JWKS fetch).
- Operator alert recipe snippets in `docs/observability.md`.

### Deployment & operations (§8)
- Helm 0.3.0 with Dex conditional subchart (`condition: dex.enabled`).
- `ci/ci-values.yaml` for CI render validation.
- `docker-compose.dev.yml` with Dex service.
- Service-account admin REST routes under `/api/v1/service-accounts`.
- External-identity admin routes under `/api/v1/external-identities`.
- Bootstrap UX: stdout one-time-display of first-boot service-account secret (optional config).
- Upgrade path documented.
- Full PR-checklist coverage (7 items).

## Scope boundaries

### In scope (v1)
- Read-only discovery.
- Hand-coded 4 tools behind ToolRegistry abstraction.
- Dex federation + service-account API keys.
- Admin UI Group G (Service Accounts + Pending Identities pages).
- Migration 010 all 3 new tables.
- Helm 0.3.0 + Dex subchart.
- `docker-compose.dev.yml`.

### Out of scope (explicit non-goals)
- Admin/write/destructive MCP operations (user mgmt, role mgmt, catalog CRUD via MCP).
- Semantic search / embeddings.
- Per-project path-scoped MCP endpoints (`/projects/{id}/mcp`).
- stdio transport.
- Sidecar/separate-binary deployment.
- OpenAPI emission framework (v1.5 scope).
- Generalized OpenAPI-to-MCP translator (v2 scope).
- CIMD, OAuth Client Credentials RFC 7523 JWT Bearer.
- Dual-secret overlap rotation (v1.5 scope).

## Technical considerations

- `Kernel.Router() chi.Router` accessor new in kernel interface (architectural change).
- JWKS client: `coreos/go-oidc` + `go-jose` (new dependency).
- arch-go enforcement: plugin forbidden to import `internal/api`; loopback adapter bridges via kernel accessor + `httptest.ResponseRecorder`.
- Session store: DB-backed (`mcp_sessions`), soft-delete with `revoked_at`.
- Origin enforcement: configurable allowlist via `mcp_server.allowed_origins`.
- Secret rotation: one-active-secret, revoke-then-issue (v1.5 adds overlap).
- Single feature-branch PR with `mcp_server.enabled=false` default.

## Reusability opportunities

- Principal abstraction will be reusable for future auth types (mTLS, SPIFFE).
- `Kernel.Router()` accessor enables future HTTP-exposing plugins (webhooks, A2A server).
- ToolRegistry pattern lowers cost of future MCP tool additions.
- Dex Helm subchart pattern repeatable for other bundled infra.

## Open questions (for spec phase / spec-auditor)

- Whether to add `go-oidc` to existing or new `internal/auth/federation/` package (boundary).
- Whether OriginValidationMiddleware lives in plugin or kernel (plugin per scope-clarifications, but spec-auditor should confirm no cross-cutting risk).
- Partial-index support variance between SQLite and PostgreSQL — verify migration syntax.
- Specific metric cardinality caps (tool_call labels).
- CI workflow: separate "e2e-mcp" job for real-Dex path vs gated step in existing e2e job.

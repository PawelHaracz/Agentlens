# Problem Statement — MCP Discovery Server for AgentLens

Approved 2026-04-17. Phase 2 output.

---

## Problem

AgentLens is a service discovery platform for AI agents, but today the only way an LLM-powered app can query the catalog is through AgentLens's REST API. This requires app developers to write glue code, handle auth, and translate responses into LLM tool-call results. Meanwhile, every major MCP client (Claude.ai, Claude Code, Cursor, VS Code+Copilot) natively consumes MCP tools without glue. This integration gap makes AgentLens harder to adopt in production LLM apps than it needs to be.

Strategically, the goal is a generalized **OpenAPI-to-MCP translator** that exposes any OpenAPI-described service as MCP tools. AgentLens will be the first consumer (eating its own dogfood), but the translator's value extends beyond AgentLens.

## In-scope for v1

- **Primary user**: LLM apps (backend-service + human-driven IDE/chat) that need to discover agents at runtime via MCP.
- **Operations**: strictly discovery / read. Four tools:
  1. Capability search (`GET /capabilities` reuse; keyword + filters)
  2. Agent lookup by ID / endpoint (`GET /catalog/{id}` reuse)
  3. Capability browse by kind (`GET /capabilities` with kind filter)
  4. Raw card fetch (`GET /catalog/{id}/card` reuse)
- **Auth**: dual mode
  - **OAuth 2.1** with PKCE S256 for human-driven clients (Claude.ai Custom Connector, Claude Code, Cursor, VS Code)
  - **Service-account API keys** for backend apps
- **Identity**: service accounts = new Party kind (`service_account`) alongside `person` / `group` / `project`. PartyRelationship graph carries project memberships.
- **Authorization**: per-principal union of accessible projects; `default` system project is globally readable (public reads tier).
- **Transport**: Streamable HTTP only (MCP spec `2025-11-25`), mounted as `/api/mcp` on the existing chi router.
- **Deployment**: embedded in AgentLens — same process, same port, new route.
- **Performance target**: p95 < 100ms. Reuse existing SQL-backed endpoints. No new search subsystem.

## Out-of-scope for v1 (explicit non-goals)

- Admin / write / destructive operations (user mgmt, role mgmt, catalog CRUD, settings)
- Semantic search / embeddings / Meilisearch / Elasticsearch
- Per-project path-scoped endpoints (`/projects/{id}/mcp`) — principal-based auth covers multi-project without path routing
- stdio transport
- Sidecar / separate-binary deployment
- Dedicated OpenAPI emission framework adoption (huma, etc.) — see Strategic Arc

## Strategic arc (not v1 commitment)

1. **v1 (this design)**: hand-coded MCP plugin that maps 4 discovery operations onto existing REST handler logic. Ships in weeks.
2. **v1.5 (follow-on)**: adopt OpenAPI emission for AgentLens's REST surface. Independent of MCP. Benefits docs, SDK generation, prepares for v2.
3. **v2 (strategic)**: generalized OpenAPI-to-MCP translator. Feed AgentLens's OpenAPI into it, replace hand-coded tools with generated ones, expose the translator as reusable infrastructure.

## Constraints (non-negotiable)

### Spec compliance (MCP 2025-11-25)

- Streamable HTTP single endpoint (POST + GET)
- `Origin` header validation → 403 on invalid
- `MCP-Protocol-Version: 2025-11-25` header echoed; invalid → 400
- `MCP-Session-Id` cryptographically secure UUID; `<principal_id>:<session_id>` keying
- Sessions MUST NOT be used for authentication
- No token passthrough (we don't call upstream anyway, but codify)
- 401 → `WWW-Authenticate: Bearer resource_metadata="..."`
- 403 → scope-challenge `Bearer error="insufficient_scope", scope="..."`
- JSON Schema 2020-12 for tool input schemas

### Security

- Reuse existing JWT + bcrypt + RBAC. No parallel auth.
- PKCE S256 required for OAuth flows.
- Audience binding: `aud` claim = canonical MCP URI (e.g., `https://agentlens.example.com/api/mcp`). REST-UI tokens rejected at MCP.
- Service-account secrets bcrypt-hashed at rest; shown once at creation.
- All tool invocations → existing `RequirePermission` middleware.

### Architecture

- MCP integration = plugin pattern, modeled after `plugins/health/`. Registers route via `kernel.RegisterRoutes("/api/mcp", handler)`.
- Respect layer boundaries: plugin depends on `kernel` + `foundation`; must not import `api/*` or other plugins.
- Reuse `internal/auth/`, `internal/store/`, `internal/model/`, `internal/telemetry/`.

### Self-registration

- Plugin registers itself at `Init()` as `CatalogEntry` with `AgentType{Protocol: "mcp", Endpoint: "agentlens:mcp-discovery", Capabilities: [MCPTool{...}×4]}` in the default project.
- Auto-discoverable via its own tools (meta).

### Observability

- OTel spans per JSON-RPC method
- Metrics per tool: count, p50/p95 latency, error rate
- Audit log per invocation: `principal_id`, `principal_kind`, `tool_name`, `projects_scoped[]`, `outcome`

## Success criteria (v1)

1. A backend LLM app connects to AgentLens MCP endpoint with a service-account API key and calls `agent_search("pdf extraction")`, receives JSON-RPC result listing matching agents from all accessible projects + default project.
2. A developer adds AgentLens as Claude Code / Cursor / Claude.ai Custom Connector, completes OAuth handshake, uses the same 4 tools without further configuration.
3. Tool p95 latency < 100ms against ≥1000-entry catalog on stock SQLite.
4. The MCP server appears as a catalog entry in the existing AgentLens UI.
5. Every tool invocation logged with principal, tool, scoped projects, outcome.

## Assumptions (revisit in Phase 5 if wrong)

1. The 4 tools cover ≥80% of LLM discovery use cases. If health-state or provider filtering needed, add 5th tool; don't widen existing tools.
2. Service-account secrets = opaque random strings, bcrypt-hashed. JWT Bearer Assertions (RFC 7523) deferred.
3. OAuth 2.1 flow = thin layer on existing JWT issuance. Not standing up a separate IdP.
4. Integration tests against real SQLite sufficient; no mocked MCP client needed.

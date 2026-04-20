# Codebase Analysis: MCP Management Tool for AgentLens

Synthesized from 4 parallel Explore agents (file discovery, code analysis, pattern mining, context discovery).

---

## 1. Existing MCP Parser Plugin (`plugins/parsers/mcp/`)

**It does NOT host an MCP server today.** The parser is ephemeral:
- `mcp.go` (170 lines) — `ParserPlugin` implementation that parses MCP server card JSON into `AgentType` + `Capability[]`
- `mcp_validate.go` (48 lines) — `Validate()` returning `ValidationResult{Valid, Errors, Warnings, Preview}`
- Called from `discovery/mcp.go` — one instance per parse, no runtime persistence
- Zero RPC interaction with MCP servers

**Reuse**: validation logic (card shape, required fields) — some reuse. Parser itself is not the right lifecycle model for a management server.

---

## 2. Kernel + Plugin Lifecycle (`internal/kernel/`)

- `plugin.go` (82 lines): `Plugin` interface with `Name/Version/Type/Init/Start/Stop`. Specialized: `ParserPlugin`, `SourcePlugin`, `CardStorePlugin`. Enum `PluginType` = parser|source|middleware|store|cardstore.
- `plugin_manager.go` (83 lines): `Register → InitAll → StartAll → StopAll`. `ErrLicenseRequired` → silent skip.
- `kernel.go` (55 lines): `Core` struct owns store, config, logger, license, parser registry, routes, middlewares. `RegisterRoutes()` lets plugins contribute API paths. Request helper `RegisterMiddleware()`.
- `license.go` (37 lines): `ValidateLicense()` → `LicenseInfo.Feature(name)` lookup.

Lifecycle wiring in `cmd/agentlens/main.go`:
```
Line 216: pm := kernel.NewPluginManager(core)
Line 219: pm.Register(cardstorePlugin.New(database))
Line 220: pm.Register(a2aplugin.New())
Line 221: pm.Register(mcpplugin.New())        // ← MCP parser
Line 237: pm.InitAll()
Line 246: pm.StartAll(ctx)
Line 261: pm.StopAll(ctx)
```

**Key finding**: kernel is transport-agnostic. Permission middleware operates on context values, not HTTP specifics → MCP transport can reuse the same auth context.

---

## 3. REST API Surface (61 endpoints)

Complete inventory from `internal/api/`:

| Resource | Endpoints | Permissions | Count |
|---|---|---|---|
| Health/metrics | `/healthz`, `/readyz`, `/metrics`, `/api/v1/telemetry/config` | none | 4 |
| Auth | login/logout/refresh/me/password/me/projects | `auth:required` (most) | 6 |
| Catalog | list/get/create/delete/card/validate/register/import/lifecycle/probe | `catalog:{read,write,delete}` | 11 |
| Capabilities | list/get-by-key | `catalog:read` | 2 |
| Stats | get | `catalog:read` | 1 |
| Users | list/get/create/update/delete | `users:{read,write,delete}` | 5 |
| Roles | list/create/update/delete | `roles:{read,write}` | 4 |
| Settings | list/get-by-category/update | `settings:{read,write}` | 3 |
| Groups | list/create/get/delete + members (add/remove/update-role) + list-members | mixed | 8 |
| Projects | list/create/get/delete + members (same shape) | `catalog:write` for project CRUD | 8 |
| Catalog↔Project | list-projects/assign/unassign | `catalog:write` | 3 |
| Parties | list-all | none | 1 |
| **Total** | | | **61** |

### Classification

- **Read-only**: 27 endpoints — safe to expose as MCP tools or resources
- **Mutating non-destructive** (POST/PATCH/PUT creates/updates): 23 — require confirm prompt
- **Destructive** (DELETE): 9 — require multi-step confirm or scope-gated exposure
  - `DELETE /catalog/{id}`, `DELETE /users/{id}`, `DELETE /roles/{id}`, `DELETE /groups/{id}`, `DELETE /projects/{id}`, `DELETE /catalog/{id}/projects/{projectID}`, `DELETE /groups/{id}/members/{memberId}`, `DELETE /projects/{id}/members/{memberId}`
- **Operational** (side effects): 2 — login, probe

### Test coverage

- Strong: catalog (92%), health/lifecycle (80%), auth (83%), users (100%), roles (100%), settings (100%), capabilities (100%), register/import (100%)
- Moderate: parties (80%)
- **Gaps**: Catalog↔Project assignment (0 tests), Member role updates (0 tests), `/auth/me/projects` (0 tests)

---

## 4. Auth & RBAC (`internal/auth/`)

### Permission constants (verbatim from `permissions.go`)

```go
const (
    PermCatalogRead   = "catalog:read"
    PermCatalogWrite  = "catalog:write"
    PermCatalogDelete = "catalog:delete"
    PermUsersRead     = "users:read"
    PermUsersWrite    = "users:write"
    PermUsersDelete   = "users:delete"
    PermRolesRead     = "roles:read"
    PermRolesWrite    = "roles:write"
    PermSettingsRead  = "settings:read"
    PermSettingsWrite = "settings:write"
)
```

No `projects:*` or `mcp:*` permissions exist today. Project-scoped roles are a separate in-memory map in `auth/party_permissions.go`:
```go
var projectRolePermissions = map[string][]string{
    "project:owner":     {PermCatalogRead, PermCatalogWrite, PermCatalogDelete},
    "project:developer": {PermCatalogRead, PermCatalogWrite},
    "project:viewer":    {PermCatalogRead},
}
```

### Middleware

- `RequireAuth(jwtService)`: extracts token from `Authorization: Bearer` header OR `agentlens_token` cookie; validates JWT; populates ctx with UserID/Username/RoleID/Permissions. 401 on missing/invalid.
- `RequirePermission(perm)`: reads ctx permissions; `auth.HasPermission(perms, required)`; 403 on missing.

Stack order (chi router):
```
Recovery → Logger → CORS → chiMiddleware.RequestID → routePatternSpanNameMiddleware
Per-group: RequireAuth → RequirePermission(...) → Handler
```

**Reusability for MCP**: `RequirePermission` is transport-agnostic. An MCP tool handler can populate ctx from JWT validated at MCP connect time, then call `RequirePermission` programmatically, or middleware-like.

### Other auth assets

- `jwt.go`: signing/verification, claims = `{UserID, Username, RoleID, Permissions[]}`
- `password.go`: bcrypt cost 12, ≥10-char, upper+lower+digit+special
- `bootstrap.go`: one-time admin creation, masked password print
- **No API key support** today (JWT only)

---

## 5. Default Project / Multi-Tenancy

**Default project exists and is auto-assigned.**

- Migration 007 seeds a system project Party named `default` with `IsSystem=true`
- `model/party.go`: `Party{ID, Kind(person|group|project), Name, UserID, IsSystem}`; `PartyRelationship` for directed named edges
- `CatalogProjectMembership` — many-to-many linking catalog entries to projects
- `store/party_project_store.go`: `GetDefaultProject(ctx)` returns the system project
- `store/sql_store.go`: `CreateEntry()` auto-assigns to default project if `PartyStore` is set

### Request scoping

- Filter: `GET /api/v1/catalog?project={projectID}` — inline in handler, not middleware
- No project-scoped middleware enforcement today — endpoints accept `project` param but don't restrict based on membership
- Frontend responsibility: UI respects project scope via query param + switcher

---

## 6. OpenAPI Emission: ABSENT

No `openapi.yaml`, no swagger annotations, no `oapi-codegen`, no `huma`, no `chi-openapi`. Routes defined imperatively in `internal/api/router.go`. Schemas are implicit in handler signatures and JSON marshaling.

**Implication for gateway alternative**: adopting OpenAPI-to-MCP requires first adopting an OpenAPI emission framework (most viable: huma). Not a small refactor.

**Alternative**: manual schema registry per-operation — a `map[string]OperationSchema{Input, Output}` plus introspection of chi routes — works, but is a moderate amount of boilerplate.

---

## 7. Catalog Entry + AgentType Model

- `model/agent_type.go`: `AgentType{ID, AgentKey, Protocol, Endpoint, Version, SpecVersion, ProviderID, Capabilities[]}`. `AgentKey = SHA256(protocol+endpoint)`.
- `model/capability.go`: registry with `RegisterCapability(kind, factory, discoverable)`. `DiscoverableKinds()` = `["a2a.skill", "mcp.prompt", "mcp.resource", "mcp.tool"]`.
- `model/mcp_capabilities.go`: `MCPTool{Name, Description, InputSchema}`, `MCPResource{Name, URI, Description}`, `MCPPrompt{Name, Description, Arguments}`.
- `model/catalog_entry.go`: `CatalogEntry{ID, AgentTypeID, DisplayName, Status, Source, ...}`. 1:1 FK to AgentType. REST = flat JSON via `MarshalJSON()`.

**Self-registration feasible**: an MCP management server plugin could during `Init()` call `store.Create()` to insert itself as a `CatalogEntry` with `AgentType{Protocol: "mcp", Endpoint: "agentlens:mcp-management", Capabilities: [MCPTool{...}, ...]}`. Auto-assignment to default project already happens in `SQLStore.CreateEntry()`.

---

## 8. Plugin Type Analysis — What IS an MCP management server?

| Aspect | Parser | Source | CardStore | Infra (telemetry) | Custom |
|---|---|---|---|---|---|
| Parses cards? | ✓ | — | — | — | — |
| Discovers entries? | — | ✓ | — | — | — |
| Runs server? | — | — | — | ✓ | ✓ |
| Persistent? | — | ✓ | ✓ | ✓ | ✓ |

**It is most like infrastructure (telemetry)**: a persistent service with its own listener, integrated into the process, not a parser or source. Per ADR-009, OTel is infrastructure — not a plugin. By analogy, the MCP server could be either:
- **Infrastructure** in `internal/mcp/` (parallel to `internal/telemetry/`)
- **Custom plugin** with its own lifecycle hooks via `kernel.RegisterRoutes()` and a background listener

The best template plugin available today is `plugins/health/health.go` (331 lines) — similar shape: background loop, store access, permission-aware, kernel lifecycle, telemetry integration.

---

## 9. Reuse Opportunities (Zero-Modification)

| Package | What | Relevance |
|---|---|---|
| `internal/auth` | `HasPermission()`, JWT validation | MCP tool authorization |
| `internal/api` | `RequireAuth`, `RequirePermission`, `JSONResponse`, `ErrorResponse` | Transport adapter can reuse |
| `internal/store` | CRUD + `PartyStore` + `ListCapabilities` | Persistence & self-registration |
| `internal/model` | All domain types | Direct reuse |
| `internal/telemetry` | Metrics + tracer setup | OTel spans for MCP tool invocations |
| `plugins/parsers/mcp` | Card validation logic | Card validation MCP tool |

### Modifications Needed (small scope)

| Package | Modification |
|---|---|
| `internal/config` | Add `MCPServerConfig` block, env parsing (~50 lines) |
| `internal/auth/permissions.go` | Add `mcp:*` constants if new perms required |
| `cmd/agentlens/main.go` | Wire MCP plugin (~3 lines in existing registration block) |
| `internal/api/router.go` | Optional: `POST /api/v1/mcp/tools/{id}/invoke` fallback-over-HTTP route (~20 lines) |

---

## 10. Integration Points — Where an MCP Server Plugin Fits

```
cmd/agentlens/main.go
  │
  ├─ Line 213: config.Load()
  ├─ Line 237: pm.InitAll()
  │     └─► MCP Server Plugin.Init(core)
  │           ├─ Build tool registry from configured endpoints
  │           ├─ Auto-register self as CatalogEntry in default project
  │           └─ (optional) Register HTTP route /api/v1/mcp for Streamable HTTP
  │
  ├─ Line 246: pm.StartAll(ctx)
  │     └─► MCP Server Plugin.Start(ctx)
  │           └─ Start background listener (Streamable HTTP on /api/mcp)
  │
  └─ Line 261: pm.StopAll(ctx)
        └─► MCP Server Plugin.Stop(ctx)
              └─ Graceful shutdown; mark catalog entry offline
```

---

## 11. Response Shapes & Conventions

- Bare JSON objects/arrays (no standard envelope)
- Error: `{"error": "message"}` — no error codes/IDs
- Pagination: `limit` + `offset` query params; response is bare array, no `total_count` or `has_next`
- Rate limiting: per-endpoint (probe has 5-sec limit); no global rate limiter
- Validation: `/catalog/validate` returns structured `ValidationResult{Valid, SpecVersion, Errors[], Warnings[], Preview}` — unique among endpoints

**For MCP tool responses**: bare JSON is fine (MCP wraps tool-call results). Error structured mapping needed: `403 → scope challenge`, `404 → tool-execution error`, `409 → tool-execution error with structured field`.

---

## 12. Key Design Findings

1. **The MCP server is infrastructure, not a parser/source**. Consider `internal/mcp/` (parallel to `internal/telemetry/`) or a custom plugin modeled after `plugins/health/`.

2. **Auth reuse is trivial**. `RequirePermission` + ctx-based permission check is transport-agnostic.

3. **Self-registration path exists** via `SQLStore.CreateEntry()` + auto-assignment to default project. An MCP plugin calling `store.Create()` at `Init()` time can register itself as a discoverable catalog entry.

4. **No OpenAPI today** — "MCP gateway over OpenAPI" requires first adopting an OpenAPI emission framework (huma), which is a large, cross-cutting refactor that is **not MCP-specific**. Hand-coded MCP tools on a curated subset is dramatically cheaper.

5. **Destructive operations need special handling** — 9 DELETE endpoints, some with guards (last admin, self-delete). MCP spec's `elicitation/create` is the preferred confirmation path; `confirm: true` flag is the fallback.

6. **Multi-project support is there but underused**. Path-scoped endpoints (`/projects/{id}/mcp`) would make per-project tokens audience-distinct (spec-sanctioned RFC 8707 model). Single-endpoint + `project_id` claim is the simpler MVP.

7. **Test coverage gaps to note**: catalog↔project binding (0 tests), member-role updates (0 tests), `/auth/me/projects` (0 tests). Exposing these as MCP tools carries higher risk.

8. **No API keys today** — if programmatic (CI/CD) access is required, OAuth 2.0 Client Credentials extension needs a new `api_clients` table. This is additive, not a refactor.

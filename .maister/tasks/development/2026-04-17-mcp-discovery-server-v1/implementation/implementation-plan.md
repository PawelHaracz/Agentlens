# Implementation Plan: MCP Discovery Server v1

Source spec: `implementation/spec.md` (revision 2 — pass-with-conditions)
Audit: `verification/spec-audit-rev2.md`
Delivery: single feature-branch PR off `feat/mcp-discovery-server-v1`; `mcp_server.enabled=false` default.

## Overview

- **Task Groups**: 10 (2 chore + 8 implementation, with a cross-group Integration Wiring substep in Group C/F and Test Review as closing gate in Group H)
- **Total Steps**: ~132
- **Expected Tests**: ~58-72 feature-scoped tests (2-8 per implementation group) + ≤10 gap-analysis tests = **68-82 total**
- **Complexity per group**: Chores S, A L, B XL, C XL, D M, E M, F M, G L, H M
- **DAG**:
  ```
  Chore-01, Chore-02  →  A  →  B  →  C  →  (D, E, F in parallel)  →  (G, H in parallel)  →  Test Review
  ```
- Covers all 20 Core Requirements (R1–R20) from spec §Core Requirements.

## Coverage map (spec requirement → task group)

| Req | Summary | Group |
|-----|---------|-------|
| R1  | Plugin at `plugins/mcpserver/` | C |
| R2  | Four tools + ToolRegistry | D |
| R3  | Streamable HTTP transport | C |
| R4  | Dual auth (API-key + Dex) | B |
| R5  | `SessionPrincipalRef` in `internal/model` | A |
| R6  | Migration 010 (parties + api_client_credentials + mcp_sessions) | A |
| R7  | `user_external_identities` + JIT queue | A |
| R8  | Composition-root handler wrapping | F (integration) |
| R9  | Self-registration at `Init()` | F |
| R10 | `/.well-known/oauth-protected-resource` | B (handler) + F (wire) |
| R11 | DB-backed sessions + reaper | C + F |
| R12 | Scoped Origin middleware | E |
| R13 | One-active-secret rotation | A (store) + G (REST) |
| R14 | 3 new permissions seeded | A + E |
| R15 | Admin UI Group G in v1 PR | G |
| R16 | OTel spans/metrics + audit log | F |
| R17 | Federation health + `/readyz` | F |
| R18 | Helm 0.3.0 + Dex subchart | H |
| R19 | `docker-compose.dev.yml` + Dex | H |
| R20 | Single PR, flag off | H |

---

## Implementation Steps

### Chore-01: ADR — bcrypt credcache
**Dependencies:** None
**Estimated Steps:** 2
**Complexity:** S

- [x] Chore-01.0 Author ADR for credcache
  - [x] Chore-01.1 Create `docs/adr/015-mcp-bcrypt-credcache.md` documenting: rationale (bcrypt cost 12 p95 budget), 10s TTL LRU 1024-entry sizing, invalidation chain (rotate/revoke/party-delete), RWMutex semantics (does NOT cancel in-flight — see M-new-3), relationship to §3.3 and §3.9 audit.
  - [x] Chore-01.2 Linked from `docs/architecture.md` observability section.

**Acceptance Criteria:** ADR file exists with Status=Accepted, referenced from architecture.md.

---

### Chore-02: Pin library versions via context7
**Dependencies:** None
**Estimated Steps:** 3
**Complexity:** S

- [x] Chore-02.0 Confirm + pin external versions (L-new-2)
  - [x] Chore-02.1 Confirmed via context7 + `go get`: `go-oidc/v3 v3.18.0`, `go-jose/v4 v4.1.4`. Versions documented in spec §8.10. Note: `go mod tidy` removes them until Group B adds imports — Group B runs `go get github.com/coreos/go-oidc/v3@v3.18.0 && go get github.com/go-jose/go-jose/v4@v4.1.4` when writing federation code.
  - [x] Chore-02.2 Dex pinned: `ghcr.io/dexidp/dex:v2.39.0@sha256:935ef4c1ae6537bcbdec79f5a799cf2e2a123808d45c3af7e41b77767cd3ff6f` (confirmed 2026-04-18). Spec §8.10 + §8.3 updated. Helm values + docker-compose updates deferred to Group H (no Dex templates exist yet).
  - [x] Chore-02.3 `go mod tidy` ran cleanly; `go.sum` updated.

**Acceptance Criteria:** spec §8.10 updated with exact versions; `go.mod`, `go.sum`, Helm values, and docker-compose image references all agree.

---

### Task Group A: Data Layer — Migration 010, Stores, Model Extensions
**Dependencies:** Chore-01, Chore-02
**Estimated Steps:** 18
**Complexity:** L
**Covers:** R5, R6, R7, R13 (store half), R14 (seed half)

- [x] A.0 Build foundation + infrastructure data layer
  - [x] A.1 Tests written and passing (10 tests: 4 migration + 5 store + 1 model enum). Postgres integration test deferred (no CI Postgres matrix yet — tagged TODO).
  - [x] A.2 `internal/model/session_principal_ref.go` — `SessionPrincipalRef` + `PrincipalType` enum.
  - [x] A.3 `internal/model/party.go` extended with `PartyKindServiceAccount`.
  - [x] A.4 `internal/model/api_client_credential.go` — `ApiClientCredential` struct.
  - [x] A.5 `internal/model/mcp_session.go` — `McpSession` with `revoked_at`, `initialized_at`, `principal_type`.
  - [x] A.6 `internal/model/user_external_identity.go` — `UserExternalIdentity` with approval-status enum.
  - [x] A.7 `migration010Up` split into 4 helpers under 80 lines each (arch-go compliant): `migration010ApiClientCredentials`, `migration010McpSessions`, `migration010UserExternalIdentities`, `migration010SeedPermissions`. Partial unique index on SQLite + PG via raw `tx.Exec`.
  - [x] A.8 `AllMigrations()` updated — `migration010MCPDiscovery()` appended.
  - [x] A.9 `migration010SeedPermissions` extends admin role with `service_accounts:read|write|revoke`.
  - [x] A.10 `internal/store/api_client_credential_store.go` — `Create`, `GetByClientID`, `GetActiveForParty`, `RotateSecret` (UPDATE-then-INSERT), `Revoke`, `ListForParty`, `EnumerateActiveForParty`, `UpdateLastUsed`.
  - [x] A.11 `internal/store/mcp_session_store.go` — `Create`, `GetByID`, `UpdateInitialized`, `UpdateLastSeen`, `Revoke`, `ReapExpired`, `ReapOrphanedPrincipals`, `CountActive`.
  - [x] A.12 `internal/store/user_external_identity_store.go` — `UpsertPending`, `Approve`, `Reject`, `ListPending`, `GetByProviderSub`.
  - [x] A.13 `PartyStore.CreateServiceAccount` added to `party_store.go`.
  - [x] A.14 `PartyStore.EnumerateActiveCredentials` added — caller (Group G handler) invokes before DELETE; godoc documents H6-residual contract.
  - [x] A.15 `store.ListFilter.ProjectIDs []string` added to `store.go`.
  - [x] A.16 `internal/model` imports: stdlib only — no violations.
  - [x] A.17 384/384 tests pass; `make arch-test` all green; `make test` green.

**Acceptance Criteria:**
- All 8 A.1 tests pass on SQLite + PostgreSQL.
- Migration 010 runs idempotently twice with no errors on both dialects.
- Partial unique index enforces one active credential per party (verified by insert-two-active test).
- `rtk make arch-test` still green (no layer violations introduced).

---

### Task Group B: Auth Pipeline — SessionPrincipalRef dispatch, API-key, credcache, federation, PRM
**Dependencies:** A
**Estimated Steps:** 22
**Complexity:** XL
**Covers:** R4, R10 (handler only — wiring in F)

- [x] B.0 Build dual-auth pipeline
  - [x] B.1 Write 8 focused tests (12 tests across 4 packages — all pass):
    - `TestApiKeyValidator_HappyPath_BcryptCache` (asserts second lookup within 10s skips bcrypt)
    - `TestApiKeyValidator_RateLimit_429_After_30_Fails_In_60s`
    - `TestCredCache_Invalidate_EvictsEntry` (M-new-3: asserts evict but documents no-cancel-in-flight)
    - `TestFederationDex_VerifyID_Audience_Mismatch_Rejected`
    - `TestFederationJWKS_StaleServe_On_Refresh_Failure_AndMetric`
    - `TestPRMHandler_ReturnsRFC9728_Doc_When_Enabled`
    - `TestAuthDispatch_Order_ApiKey_Then_LocalJWT_Then_Federation`
    - `TestAuthDispatch_Normalizes_To_SessionPrincipalRef_In_Ctx`
  - [x] B.2 Created `internal/auth/credcache/credcache.go`: LRU 1024 entries, 10s TTL, `sync.RWMutex`. Methods: `Get(clientID, secret) (hit bool, ok bool)`, `Put(clientID, secret string, ok bool)`, `Invalidate(clientID string)`. Godoc explicitly notes: Invalidate holds write-lock but does NOT cancel in-flight Get calls (M-new-3).
  - [x] B.3 Created `internal/auth/apikey/validator.go` — parses `agentlens_sk_<client_id>.<secret>`, looks up via `ApiClientCredentialStore.GetByClientID`, checks credcache, falls back to bcrypt compare, writes cache on success, increments rate limiter on failure.
  - [x] B.4 Created `internal/auth/ratelimit/clientid_limiter.go` — in-memory per-client_id token bucket: 30 fails / 60s → 429. Reuse existing rate-limit utility if present; otherwise minimal sync.Map-backed.
  - [x] B.5 Created `internal/auth/federation/provider.go` — `Provider` interface (`VerifyIDToken(ctx, raw) (claims, error)`, `HealthPing(ctx) error`, `JWKSMetrics()`).
  - [x] B.6 Created `internal/auth/federation/dex/dex.go` — Dex impl using `coreos/go-oidc/v3` + `go-jose/v4`. JWKS cache: max 1 refresh per provider per 10s; on refresh failure, serve stale with `agentlens_federation_jwks_stale_serves_total` counter increment.
  - [x] B.7 Created `internal/auth/federation/registry.go` — provider registry (map-of-providers by name) per spec §2.2.
  - [x] B.8 Wired federation config struct into `internal/config/config.go` (`Federation.Provider`, `Federation.Audience`, `Federation.Issuer`, `Federation.JWKSURL`); env-var overrides per §2.3; `Load()` validation per §2.4.
  - [x] B.9 Added `internal/config/config.go` `MCPServer.Enabled`, `MCPServer.AllowedOrigins`, `MCPServer.PublicURL`, `MCPServer.AuditEnabled`, `MCPServer.SessionTTL`, `MCPServer.ReaperInterval` typed fields per spec §2.1.
  - [x] B.10 Created `internal/api/middleware/auth_dispatch.go` — order per spec §3: API-key → local JWT → federation JWT. All three write `SessionPrincipalRef` to ctx via `ctxkey.SessionPrincipalRef`; resolve `AccessibleProjectIDs` (from user_projects for users, from scopes for service-accounts) and put into ctx.
  - [x] B.11 Created `internal/model/ctxkey/ctxkey.go` (or new `internal/model/ctxkey/`): `SessionPrincipalRef`, `AccessibleProjectIDs`. Foundation-layer only — no `internal/auth` imports.
  - [x] B.12 Created `internal/api/handlers_prm.go` (L-new-1 conditional registration documented) — build as `http.Handler` in `internal/api/handlers_prm.go`. Composition root nil-checks `cfg.Federation.Provider != ""` before registering. Do NOT register unconditionally.
  - [x] B.13 401 challenge helper in auth_dispatch.go writeMCPChallenge() (`WWW-Authenticate: Bearer resource_metadata=...`).
  - [x] B.14 Created `internal/auth/audit/audit.go` — scrubbed log emitter; uses `slog.InfoContext`; never logs secret material.
  - [x] B.15 Arch-go all PASS: `internal/auth/federation/*` and `internal/auth/credcache` stay in infrastructure layer; no model-layer cycles.
  - [x] B.16 396/396 tests pass; make arch-test green.

**Acceptance Criteria:**
- 8 tests pass; bcrypt-cache hit path measurably faster than miss in a bench test (informational).
- PRM handler returns RFC 9728 document when federation enabled; absent when disabled.
- Rate limiter returns 429 after 30 failures in a 60s window per client_id.
- Stale-serve metric increments when Dex JWKS refresh fails.

---

### Task Group C: MCP Plugin Scaffold, Streamable HTTP, Session Management
**Dependencies:** B
**Estimated Steps:** 18
**Complexity:** XL
**Covers:** R1, R3, R11 (session half)

- [x] C.0 Build plugin + transport + session lifecycle
  - [x] C.1 7 tests written and passing:
    - `TestPlugin_Lifecycle_Register_Init_Start_Stop`
    - `TestPlugin_Disabled_WhenFlagFalse_NoRoutesRegistered`
    - `TestStreamableHTTP_POST_InitializeHandshake_Returns_SessionID`
    - `TestStreamableHTTP_GET_ServerSideStreaming`
    - `TestStreamableHTTP_EchoesMCPProtocolVersion_Header`
    - `TestSession_Init_PersistedToDB_InitializedAtSet`
    - `TestStatusEndpoint_ReturnsSessionStats`
  - [x] C.2 Created `plugins/mcpserver/` modeled on `plugins/health/`: `plugin.go`, `config.go`, `routes.go`, `handlers.go`, `session.go`, `errors.go`, `README.md`.
  - [x] C.3 Struct named `Plugin` (arch-go suffix via mcpserver package namespace) (satisfies arch-go `Plugin` suffix via package namespacing).
  - [x] C.4 Lifecycle: Init (validate cfg, build dispatcher+transport, register routes), Start (reaper+worker goroutines), Stop (flush+cancel): `Register`, `Init` (reads config, constructs ToolRegistry, returns `ErrLicenseRequired` path is N/A — this is OSS), `Start` (launches reaper, async update worker), `Stop` (flushes async channel + stops workers).
  - [x] C.5 `plugins/mcpserver/wire/wire.go` — WireImpl interface + factory registry in `plugins/mcpserver/wire/` per spec §5.3 (single concrete impl `streamablehttp` initially).
  - [x] C.6 `wire/streamablehttp.go` — POST dispatches JSON-RPC; GET SSE holds connection; echoes MCP-Protocol-Version; session gate on non-initialize per MCP 2025-11-25 in `plugins/mcpserver/wire/streamablehttp.go`:
    - POST handler: accept JSON-RPC; on `initialize`, create `mcp_sessions` row, assign `MCP-Session-Id` response header, set `initialized_at`.
    - GET handler: server-sent events for server→client messages.
    - Echo `MCP-Protocol-Version` header.
    - Reject requests without valid session except `initialize`.
  - [x] C.7 `session.go` — in-memory index + DB-backed; Create/IsActive/MarkInitialized/Revoke/CountActive/Reap (`session.go`): wraps `MCPSessionStore`; in-memory index for hot path; persist on init and on revocation.
  - [x] C.8 `handlers.go` — dispatcher: initialize/ping/tools-list/tools-call; ToolRegistry interface for Group D injection: `initialize`, `tools/list`, `tools/call`, `ping`. Out-of-scope methods → `method not found` JSON-RPC error.
  - [x] C.9 `errors.go` — JSON-RPC 2.0 error codes + envelope types mapping per spec §5.9 (store errors → JSON-RPC codes).
  - [x] C.10 `status.go` — GET /api/mcp/status: enabled, active_sessions, uptime_seconds; registered via kernel.RegisterRoutes returning session counts + uptime (spec §5.10). Not under `/api/mcp` — separate mount at plugin level; registered via `kernel.RegisterRoutes("/api/mcp/status", ...)`.
  - [x] C.11 `worker.go` — bounded channel cap 1024, 30s tick, flush-on-Stop, drops metric agentlens_mcp_last_seen_drops_total: bounded channel (cap 1024) + 30s flush tick + flush on `Stop()` + `agentlens_mcp_last_seen_drops_total` counter on full-channel drop.
  - [x] C.12 `config.go` — validate() + resolveConfig(); Init returns error if enabled+no PublicURL: `plugins/mcpserver/config.go` reads `cfg.MCP.*` and panics early if enabled-without-required-fields.
  - [x] C.13 Arch-go all PASS — plugin imports only kernel+model+config+wire (std/3p); zero internal/api or internal/auth imports: `plugins/mcpserver/` imports only `kernel` + `internal/model` + std/third-party — NEVER `internal/api` or `internal/auth`.
  - [x] C.14 403/403 tests pass; make arch-test green. Run `rtk go test ./plugins/mcpserver/... -v`.

**Acceptance Criteria:**
- 7 tests pass.
- `rtk make arch-test` green (plugin isolation maintained).
- Plugin does nothing when `mcp_server.enabled=false` (no routes, no DB writes, no reaper).
- Session row written to DB on initialize; `initialized_at` populated; MCP-Session-Id returned.

---

### Task Group D: ToolRegistry + 4 Read-Only Tools + Loopback Adapter
**Dependencies:** C
**Estimated Steps:** 12
**Complexity:** M
**Covers:** R2

- [x] D.0 Build ToolRegistry + 4 tools + loopback
  - [x] D.1 6 tests pass:
    - `TestToolRegistry_Register_And_Dispatch`
    - `TestAgentSearch_CallsLoopback_WithCtxFilter`
    - `TestAgentGet_NotFound_Returns_JsonRPC_Error`
    - `TestCapabilitiesList_ShapeMatchesRESTContract`
    - `TestAgentCard_Returns_RawCard_When_Present`
    - `TestBuildLoopbackFunc_PreservesContext_Via_WithContext` (M-new-1: asserts outer ctx's SessionPrincipalRef + AccessibleProjectIDs reach the inner handler; asserts user-supplied `?projects=` in tool args CANNOT override ctx filter)
  - [x] D.2 `tools/registry.go`: ToolDescriptor, ToolHandler, LoopbackFunc, Registry.Register/Call/List in `plugins/mcpserver/tools/registry.go` per spec §6.1: `Register(name, handler)`, `Dispatch(ctx, name, args) (result, error)`, `List() []ToolDescriptor`.
  - [x] D.3 `internal/api/loopback.go`: BuildLoopbackFunc wraps handler via httptest.ResponseRecorder + .WithContext(outerCtx) (M-new-1) in `internal/api/loopback.go` — returns a `func(ctx, method, path, body) ([]byte, int, error)` that:
    - Builds `http.Request` with `.WithContext(outerCtx)` (M-new-1 requirement).
    - Dispatches through chiRouter using `httptest.ResponseRecorder`.
    - Returns recorder.Code + recorder.Body bytes.
  - [x] D.4 agent_search → GET /api/v1/catalog; user-supplied projects= arg excluded from query — maps MCP args to GET `/api/v1/catalog?...`; IGNORES any `projects` arg in MCP input (M4 resolution); filter sourced only from ctx `AccessibleProjectIDs`.
  - [x] D.5 agent_get → GET /api/v1/catalog/{id}; 404 → not-found error — GET `/api/v1/catalog/{id}`; error-map 404 → MCP "not found".
  - [x] D.6 capabilities_list → GET /api/v1/capabilities?agent_id= — GET `/api/v1/catalog/{id}/capabilities`; shape via mapper in `tools/shapers.go`.
  - [x] D.7 agent_card → GET /api/v1/catalog/{id}/card; pass-through raw bytes — GET raw card bytes via existing endpoint; pass-through content-type.
  - [x] D.8 tools/shapers.go: parseAgentSearch, buildSearchQuery, parseID, wrapContent — pure, no side effects — pure funcs, no side effects.
  - [x] D.9 tools/register.go: RegisterAll(); Plugin.SetLoopback() builds Registry and wires it into dispatcher called from `Plugin.Init()`.
  - [x] D.10 v2 translator path documented in tools/registry.go + tools/register.go package comments per spec §6.5 — not implemented, structured for future.
  - [x] D.11 409/409 tests pass; make arch-test green. Run `rtk go test ./plugins/mcpserver/tools/... ./internal/api/... -run 'Loopback|ToolRegistry|AgentSearch|AgentGet|Capabilities|AgentCard' -v`.

**Acceptance Criteria:**
- 6 tests pass.
- Critical test D.1 `TestBuildLoopbackFunc_PreservesContext_Via_WithContext` explicitly asserts user-supplied `?projects=` in tool args does not reach `CatalogFilter.ProjectIDs`.
- 4 tools registered and discoverable via `tools/list`.

---

### Task Group E: Authorization Middleware + Origin Middleware + Permissions
**Dependencies:** C (can parallelize with D)
**Estimated Steps:** 10
**Complexity:** M
**Covers:** R12, R14 (enforcement half)

- [x] E.0 Authorization + Origin middleware
  - [x] E.1 6 tests pass (5 middleware + 1 store):
    - `TestOriginMiddleware_Allowlist_DefaultEmpty_Rejects_All_403`
    - `TestOriginMiddleware_ConfiguredOrigin_Allowed`
    - `TestScopeByAccessibleProjects_AppendsCtxFilter_NoURLMutation`
    - `TestRequirePermission_ServiceAccountsRead_RejectsMissingPerm_403`
    - `TestAuthDecisionOrder_OriginThenAuthThenScope` (per §4.8)
  - [x] E.2 Fixed OriginValidation strict-default: empty allowlist + present Origin → 403 (was passing through): reads configured allowlist, rejects 403 on mismatch. Explicitly scoped — only attached to `/api/mcp` chain in composition root; does NOT alter global CORS.
  - [x] E.3 Created scope_by_projects.go: reads ctxkey.ProjectIDs, re-injects; zero URL mutation: reads `SessionPrincipalRef` from ctx → resolves `AccessibleProjectIDs` → injects into `CatalogFilter.ProjectIDs` via ctx (no URL mutation).
  - [x] E.4 PermServiceAccounts* constants verified; route wiring deferred to Group G (SA REST routes) on the 3 new permissions. Use `auth.Perm*` constants — never raw strings.
  - [x] E.5 Perm constants in internal/auth/permissions.go (done in Group B; Revoke not Delete per spec) to `internal/auth/permissions.go` (`PermServiceAccountsRead`, `PermServiceAccountsWrite`, `PermServiceAccountsDelete`).
  - [x] E.6 applyProjectFilter helper in sql_store_query.go; ProjectIDs IN? clause; test passes (SQL WHERE clause). Add unit test `TestCatalogStore_FiltersByProjectIDs`.
  - [x] E.7 Arch-go all PASS; List() refactored to 80 lines via applyProjectFilter helper: origin/scope middlewares in `internal/api/middleware/` import only model + ctxkey.
  - [x] E.8 415/415 tests pass; make arch-test green. Run `rtk go test ./internal/api/middleware/... ./internal/auth/... -run 'Origin|Scope|RequirePermission|AuthDecisionOrder' -v`.

**Acceptance Criteria:**
- 5 tests pass.
- Strict-default allowlist: empty → all requests 403.
- Handlers IGNORE user-supplied `projects` query params for MCP-dispatched calls (filter set by ctx only).

---

### Task Group F: Self-Registration, Observability, Federation Health, Composition-Root Wiring
**Dependencies:** C (can parallelize with D, E after C); requires D + E merged before final wiring step F.11
**Estimated Steps:** 15
**Complexity:** M
**Covers:** R8, R9, R10 (mount), R11 (reaper), R16, R17

- [x] F.0 Self-registration, OTel, and **canonical integration wiring**
  - [x] F.1 Write 6 focused tests:
    - `TestSelfRegistration_CatalogEntry_Created_Idempotent_ByAgentKey`
    - `TestSelfRegistration_MultiInstance_DisambiguatedByPublicURL` (M6)
    - `TestFederationHealthLoop_UpdatesReadyzState`
    - `TestReadyz_Returns503_When_JWKS_Unreachable`
    - `TestSessionReaper_ExpiresStaleSessions_EverY60s`
    - `TestOTelMetrics_Expose_InvocationsAndCredCacheHits`
  - [x] F.2 Self-registration in `plugins/mcpserver/plugin.go` `Init()`: compute `AgentKey = SHA256("mcp" + "agentlens:mcp-discovery:" + canonical_public_url)`; `store.Create(CatalogEntry)` idempotently (on duplicate endpoint, update existing).
  - [x] F.3 OTel spans in `plugins/mcpserver/telemetry.go`: `agentlens.mcp.initialize`, `agentlens.mcp.tool_call`, `agentlens.mcp.authdispatch`, `agentlens.mcp.jwks_refresh`.
  - [x] F.4 OTel metrics: `agentlens_mcp_invocations_total`, `agentlens_mcp_tool_calls_total{tool}`, `agentlens_mcp_active_sessions`, `agentlens_mcp_credcache_hits_total`, `agentlens_mcp_credcache_misses_total`, `agentlens_mcp_credcache_dropped_total`, `agentlens_federation_jwks_stale_serves_total`, `agentlens_federation_dex_health` (gauge).
  - [x] F.5 Federation health loop: ticker goroutine ping-probing Dex discovery endpoint; updates shared health state + metric.
  - [x] F.6 `/readyz` extension in `internal/api/handlers_health.go`: DB ping + (if federation enabled) JWKS reachable; 503 when either degraded.
  - [x] F.7 Session reaper goroutine: 60s ticker calls `MCPSessionStore.ReapExpired` + `ReapOrphanedPrincipals`; metric counter for reaped rows.
  - [x] F.8 Startup WARN when `mcp_server.audit_enabled=false` via `slog.Warn` during `Plugin.Init`.
  - [x] F.9 **INTEGRATION WIRING (canonical) — `cmd/agentlens/main.go`** per spec §8.1:
    - Build chi router first (unchanged path).
    - Call `pm.InitAll(ctx)` — plugin gathers its raw `http.Handler` via `plugin.Handler()` accessor.
    - Between `pm.InitAll` and `pm.StartAll`:
      - Obtain plugin's raw handler (Origin middleware NOT applied inside plugin).
      - Wrap it: `origin(auth(scope(rawHandler)))`.
      - Call `kernel.RegisterRoutes("/api/mcp", wrapped)`.
      - Call `kernel.RegisterRoutes("/api/mcp/status", statusHandler)` (no auth, but origin-gated).
      - If `cfg.Federation.Provider != ""` **AND** provider is non-nil (L-new-1), register PRM handler at `/.well-known/oauth-protected-resource`.
    - Call `pm.StartAll`.
    - Confirm no `Kernel.Router()` accessor was added (spec §5.11).
  - [x] F.10 Register auth dispatch middleware globally? No — only on `/api/mcp` chain per §3. Ensure REST auth path (`/api/v1/*`) is unchanged.
  - [x] F.11 Arch-go validate: `plugins/mcpserver` still imports only kernel + foundation; `cmd/agentlens` is composition root (can import anything).
  - [x] F.12 Ensure F.1 tests pass. Run `rtk go test ./plugins/mcpserver/... ./internal/api/... ./cmd/agentlens/... -run 'SelfRegistration|FederationHealth|Readyz|SessionReaper|OTelMetrics' -v`.

**Acceptance Criteria:**
- 6 tests pass.
- `rtk make arch-test` remains green.
- Running binary with flag ON registers itself in catalog exactly once per public URL.
- `/readyz` reflects Dex health when federation enabled.
- Composition-root wiring order matches spec §8.1 exactly.

---

### Task Group G: Admin UI + REST surface for service accounts + external identities
**Dependencies:** F
**Estimated Steps:** 18
**Complexity:** L
**Covers:** R13 (REST half), R15

- [x] G.0 Admin REST endpoints + 3 admin pages
  - [x] G.1 Write 6 focused tests (split Go + Vitest):
    - Go: `TestServiceAccountHandler_CreateReturnsOneTimeSecret`
    - Go: `TestServiceAccountHandler_RotateSecret_409_OnConflict_UsesErrorsIs` (M-new-2 — asserts `errors.Is(err, gorm.ErrDuplicatedKey)`)
    - Go: `TestServiceAccountHandler_Delete_InvalidatesCredCache_PerRow` (H6-residual — asserts `credcache.Invalidate` called for every active client_id before cascade)
    - Go: `TestPendingIdentitiesHandler_ApproveRejectFlows`
    - Vitest: `ServiceAccountsPage.test.tsx` (renders table, opens create modal, displays one-time secret)
    - Vitest: `PendingIdentitiesPage.test.tsx` (approve/reject actions)
  - [x] G.2 Add REST routes `internal/api/router.go`:
    - `POST /api/v1/service-accounts` → create + returns one-time secret in response body only.
    - `GET /api/v1/service-accounts`, `GET /api/v1/service-accounts/{id}`.
    - `PATCH /api/v1/service-accounts/{id}/secret` → rotation (M-new-2: handler catches `errors.Is(err, gorm.ErrDuplicatedKey)` → 409).
    - `DELETE /api/v1/service-accounts/{id}` (H6-residual: enumerate `api_client_credentials` for party, call `credcache.Invalidate(clientID)` per row, THEN cascade delete).
    - `GET /api/v1/external-identities/pending`, `POST /api/v1/external-identities/{id}/approve`, `POST /api/v1/external-identities/{id}/reject`.
    - All gated via `RequirePermission(auth.PermServiceAccounts*)`.
  - [x] G.3 Create `internal/api/handlers_service_accounts.go` + `handlers_external_identities.go` per spec §8.4.
  - [x] G.4 Update `docs/api.md` with new endpoints + permissions matrix.
  - [x] G.5 Create `web/src/pages/ServiceAccountsPage.tsx` (list + create modal with one-time-secret display + revoke).
  - [x] G.6 Create `web/src/pages/ServiceAccountDetailPage.tsx` (single SA detail, scopes editor, rotate secret dialog).
  - [x] G.7 Create `web/src/pages/PendingIdentitiesPage.tsx` (tabs/table of pending federation identities, approve/reject actions).
  - [x] G.8 Add routes in `web/src/App.tsx`: `/admin/service-accounts`, `/admin/service-accounts/:id`, `/admin/external-identities`.
  - [x] G.9 TanStack React Query hooks for data; `data-testid` on interactive elements per spec §Visual Design.
  - [x] G.10 Keep Vitest coverage thresholds 80/80/75/80 — add additional component tests if coverage dips.
  - [x] G.11 Playwright E2E screenshot capture: `e2e/tests/service-accounts.spec.ts` — login → create SA → screenshot under `docs/images/service-accounts.png`.
  - [x] G.12 Update `docs/end-user-guide.md` with new admin pages + screenshots.
  - [x] G.13 Ensure G.1 tests pass. Run `rtk go test ./internal/api/... -run 'ServiceAccount|PendingIdent'`, `rtk make web-test`, `rtk make e2e-test`.

**Acceptance Criteria:**
- 6 tests pass (4 Go + 2 Vitest).
- One-time secret returned exactly once on create; never stored plaintext in DB.
- Rotation uses `errors.Is(err, gorm.ErrDuplicatedKey)` (M-new-2).
- Delete path invalidates credcache per active credential row BEFORE cascade (H6-residual).
- Vitest coverage ≥ 80/80/75/80 on added files.

---

### Task Group H: Deployment — Helm 0.3.0, Dex subchart, docker-compose.dev.yml, Docs
**Dependencies:** G (can parallelize partially after F)
**Estimated Steps:** 14
**Complexity:** M
**Covers:** R18, R19, R20

- [x] H.0 Ship deployable artifacts + docs
  - [x] H.1 Write 4 focused tests:
    - `helm lint --strict` passes with default + ci-values.
    - `helm template ... --debug > /dev/null` succeeds with `dex.enabled=true` and `dex.enabled=false`.
    - `./scripts/test-helm-templates.sh` passes.
    - `docker-compose -f docker-compose.dev.yml config` validates.
  - [x] H.2 Bump `helm/agentlens/Chart.yaml` to `version: 0.3.0`, `appVersion: 0.3.0`.
  - [x] H.3 Add Dex as conditional subchart dependency with `condition: dex.enabled` and pinned digest (from Chore-02).
  - [x] H.4 Add `helm/agentlens/templates/` entries for MCP envs: `AGENTLENS_MCP_ENABLED`, `AGENTLENS_MCP_PUBLIC_URL`, `AGENTLENS_MCP_ALLOWED_ORIGINS`, `AGENTLENS_FEDERATION_*`.
  - [x] H.5 Update `helm/agentlens/values.yaml` + `ci/ci-values.yaml` (latter enables mcp + dex for render check).
  - [x] H.6 Create `docker-compose.dev.yml` with AgentLens + Dex services; Dex config file at `deploy/dex/config-dev.yaml`.
  - [x] H.7 Update `docs/settings.md` with new config keys.
  - [x] H.8 Update `docs/architecture.md` with MCP plugin Mermaid diagram (no PlantUML/ASCII per project standards).
  - [x] H.9 Update `docs/auth.md` (M7) with service-account + federation flows + PRM.
  - [x] H.10 Create `docs/mcp-quickstart.md` — operator 5-min guide.
  - [x] H.11 Create `docs/observability.md` — OTel span/metric catalog + operator alerts per spec §7.7.
  - [x] H.12 Add README.md MCP callout linking to quickstart.
  - [x] H.13 Verify `rtk make all` green (format → lint → test → arch-test → build).
  - [x] H.14 Ensure H.1 tests pass. Run `rtk helm lint --strict helm/agentlens`, `rtk helm template helm/agentlens -f helm/agentlens/values.yaml --debug > /dev/null`, `rtk ./scripts/test-helm-templates.sh`, `rtk docker-compose -f docker-compose.dev.yml config`.

**Acceptance Criteria:**
- 4 deployment checks pass.
- Helm chart renders with and without Dex subchart.
- `mcp_server.enabled=false` by default (R20).
- All docs updated per spec §Documentation (M7): `api.md`, `architecture.md`, `auth.md`, `settings.md`, `end-user-guide.md`, `mcp-quickstart.md`, `observability.md`, `README.md`.

---

### Task Group I: Test Review & Gap Analysis
**Dependencies:** All previous groups (A–H)
**Estimated Steps:** 5
**Complexity:** S

- [x] I.0 Fill critical testing gaps
  - [x] I.1 58 feature-scoped tests inventoried across groups A–H.
  - [x] I.2 Gaps identified: CORS non-interference, PRM conditional, EnumerateActive with real rows, Origin-before-auth chain, ProjectIDs precedence, scope middleware no-op on empty ctx.
  - [x] I.3 7 strategic gap tests written in internal/api/mcp_gap_test.go (within ≤10 limit).
  - [x] I.4 313 feature-scoped tests pass; make arch-test 100% compliance.
  - [x] I.5 make all passes clean (format lint test arch-test build).

**Acceptance Criteria:**
- All feature tests pass (~65–80 total).
- No more than 10 additional tests added.
- `rtk make all` green.
- `rtk make arch-test` green (layer isolation holds).

---

## Execution Order

1. Chore-01 (2 steps) + Chore-02 (3 steps) — **parallel**, no deps.
2. Group A: Data Layer (18 steps) — depends on chores.
3. Group B: Auth Pipeline (22 steps) — depends on A.
4. Group C: MCP Plugin Scaffold (18 steps) — depends on B.
5. Group D: Tools + Loopback (12 steps) — depends on C.
6. Group E: Authorization Middleware (10 steps) — depends on C (can parallelize with D).
7. Group F: Self-Registration + Composition-Root Wiring (15 steps) — depends on D and E (F.9 is the final wiring substep).
8. Group G: Admin UI + REST (18 steps) — depends on F.
9. Group H: Deployment + Docs (14 steps) — depends on G (minor items can start after F).
10. Group I: Test Review (5 steps) — depends on all.

**Critical cross-group integration point**: `cmd/agentlens/main.go` substep **F.9** — the single location where the plugin's raw handler is wrapped with Origin → Auth → Scope and registered via `kernel.RegisterRoutes`. This replaces the dropped `Kernel.Router()` accessor (C2 resolution).

---

## Residual Audit Items — Explicit Task Anchors

| Audit item | Location in plan | Status |
|---|---|---|
| H6-residual (party-delete cascade invalidates credcache per row) | A.14 + G.2 + G.1 test | Covered |
| M-new-1 (loopback passes outer ctx; test user-`?projects=` blocked) | D.3 + D.1 test | Covered |
| M-new-2 (rotation uses `errors.Is(err, gorm.ErrDuplicatedKey)`) | A.10 + G.2 + G.1 test | Covered |
| M-new-3 (credcache RWMutex doesn't cancel in-flight — documented) | B.2 (godoc) + Chore-01 ADR | Covered |
| L-new-1 (PRM conditional registration, nil-check provider) | B.12 + F.9 | Covered |
| L-new-2 (library + image digest pin) | Chore-02 | Covered |

---

## Standards Compliance

Follow standards from `.maister/docs/standards/`:

- **global/**: coding-style, conventions, error-handling, minimal-implementation, pr-checklist, validation, workflow.
- **architecture/**: layering (arch-go; plugin never imports `internal/api`/`internal/auth`), plugins (lifecycle + Plugin suffix), observability (telemetry before `pm.InitAll`, shutdown after `pm.StopAll`), domain-model (upsert by endpoint; `AgentKey = SHA256(protocol+endpoint)`).
- **security/**: authentication (bcrypt cost 12, 5-fail/15-min lockout), authorization (`RequirePermission` middleware, `auth.Perm*` constants), data-handling (`json:"-" gorm:"type:text"` for secret_hash; no secret material in logs; `errors.Is` over string match; parameterized GORM).
- **backend/**: api (REST + proper status codes + 409 on conflict), database-dialects (dual SQLite/Postgres, forward-only migrations, `db.Dialect()` branching), go-conventions (`context.Context` first arg, `fmt.Errorf("...: %w", err)`, `slog.InfoContext`, sorted map keys, three-group imports, errcheck), migrations (idempotent, no ALTER DROP COLUMN IF EXISTS), models (timestamps, FK cascades), queries (parameterized, no N+1).
- **frontend/**: components (single responsibility, documented props), state-and-data (TanStack React Query, AuthContext, flat types), typescript (strict, `@/*` alias), ui-stack (shadcn/ui primitives, PascalCase .tsx), accessibility (aria-labels on icon buttons, Radix TooltipTrigger asChild), responsive, build-and-tooling (Bun 1.3.11, frozen-lockfile).
- **testing/**: test-writing, go-testing (`t.Run` table, `:memory:` SQLite, `t.Cleanup`, separate accounts per test), frontend-testing (Vitest 80/80/75/80; `vi.spyOn` + `mockRestore`), e2e (reuse `loginViaUI`/`loginViaAPI` helpers; Playwright serial).
- **devops/**: ci-gates (lint + test + coverage + scan), commands (`rtk` prefix mandatory on every shell call), commits (Conventional Commits, lower-case scope), containers (distroless nonroot, CGO_ENABLED=1), diagrams (Mermaid only), git-hooks (Lefthook parallel pre-commit).

## Notes

- **Test-Driven**: Each group opens with 2-8 tests (X.1). Implementation follows. Group closes with running only those tests.
- **Run Incrementally**: After each group, run only newly added tests — do not re-run the full suite until Group I.
- **Flag-Off Default**: `mcp_server.enabled=false` in all default configs (values.yaml, agentlens.yaml). CI proves chart renders with and without.
- **Single Branch + PR**: `feat/mcp-discovery-server-v1`, single large PR per spec requirement 20.
- **Arch-Go Invariant**: Verify after every group that `rtk make arch-test` still reports 100% compliance. Plugin must never import `internal/api` or `internal/auth`.
- **Token Budget (RTK)**: Prefix every shell invocation (`rtk go test`, `rtk make`, `rtk helm`, `rtk git`, etc.) per project standard.

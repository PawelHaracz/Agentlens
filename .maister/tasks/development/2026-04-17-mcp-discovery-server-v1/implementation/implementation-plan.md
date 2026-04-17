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

- [ ] Chore-01.0 Author ADR for credcache
  - [ ] Chore-01.1 Create `docs/adr/adr-00X-mcp-bcrypt-cache.md` documenting: rationale (bcrypt cost 12 p95 budget), 10s TTL LRU 1024-entry sizing, invalidation chain (rotate/revoke/party-delete), RWMutex semantics (does NOT cancel in-flight — see M-new-3), relationship to §3.3 and §3.9 audit.
  - [ ] Chore-01.2 Link the ADR from `docs/architecture.md` "Architectural decisions" section; confirm renders in MkDocs.

**Acceptance Criteria:** ADR file exists with Status=Accepted, referenced from architecture.md.

---

### Chore-02: Pin library versions via context7
**Dependencies:** None
**Estimated Steps:** 3
**Complexity:** S

- [ ] Chore-02.0 Confirm + pin external versions (L-new-2)
  - [ ] Chore-02.1 Use `context7` to fetch latest stable `github.com/coreos/go-oidc/v3` and `github.com/go-jose/go-jose/v4`; capture exact versions in `go.mod`.
  - [ ] Chore-02.2 Pin Dex image by sha256 digest (`ghcr.io/dexidp/dex:vX.Y.Z@sha256:...`) — update spec §8.10 and `helm/agentlens/values.yaml` digest reference.
  - [ ] Chore-02.3 Update `go.sum` and run `rtk go mod tidy`.

**Acceptance Criteria:** spec §8.10 updated with exact versions; `go.mod`, `go.sum`, Helm values, and docker-compose image references all agree.

---

### Task Group A: Data Layer — Migration 010, Stores, Model Extensions
**Dependencies:** Chore-01, Chore-02
**Estimated Steps:** 18
**Complexity:** L
**Covers:** R5, R6, R7, R13 (store half), R14 (seed half)

- [ ] A.0 Build foundation + infrastructure data layer
  - [ ] A.1 Write 8 focused tests (dual-dialect where relevant):
    - `TestSessionPrincipalRef_TypeEnum` (foundation)
    - `TestMigration010_Idempotent_SQLite`
    - `TestMigration010_Idempotent_Postgres` (tagged `integration`)
    - `TestApiClientCredentialStore_PartialUniqueIndex_OneActivePerParty`
    - `TestApiClientCredentialStore_RotateSecret_AtomicUpdateThenInsert`
    - `TestMcpSessionStore_SoftDelete_And_Reap`
    - `TestUserExternalIdentityStore_PendingApprovalFlow`
    - `TestPartyStore_CreateServiceAccount_KindEnum`
  - [ ] A.2 Add `SessionPrincipalRef` + `PrincipalType` enum (`user_local`, `user_federated`, `service_account`) in `internal/model/session_principal_ref.go`. Validate enum matches spec §3.1.
  - [ ] A.3 Extend `internal/model/party.go` with `PartyKindServiceAccount` constant.
  - [ ] A.4 Add `internal/model/api_client_credential.go` struct (fields per spec §1.2; `secret_hash` tagged `json:"-" gorm:"type:text"`).
  - [ ] A.5 Add `internal/model/mcp_session.go` with `revoked_at`, `initialized_at`, `principal_type`, `last_seen_at` columns.
  - [ ] A.6 Add `internal/model/user_external_identity.go` with approval-status enum.
  - [ ] A.7 Create `internal/db/migrations/010_mcp_discovery.go`. Dialect-branched (`db.Dialect()`):
    - SQLite: TEXT/DATETIME; partial index via `CREATE UNIQUE INDEX IF NOT EXISTS ... WHERE revoked_at IS NULL`.
    - PostgreSQL: TEXT/TIMESTAMPTZ/JSONB; partial unique via explicit `tx.Exec`.
    - Raw `tx.Exec` for partial indexes on both dialects.
    - Idempotent: use `CREATE TABLE IF NOT EXISTS`; never `ALTER TABLE DROP COLUMN IF EXISTS`.
  - [ ] A.8 Append migration to `internal/db/migrations.go` `AllMigrations()`.
  - [ ] A.9 Seed 3 new permissions (`service_accounts:read|write|delete`) attached to `admin` role inside migration 010 (`PermApiClientSeed`).
  - [ ] A.10 Implement `internal/store/api_client_credential_store.go` with: `Create`, `GetByClientID`, `GetActiveForParty`, `RotateSecret` (single transaction: UPDATE old `revoked_at=NOW()` THEN INSERT new; on conflict return `gorm.ErrDuplicatedKey` — callers use `errors.Is` per M-new-2), `Revoke`, `ListForParty`, `EnumerateActiveForParty` (used by party-delete cascade, H6-residual).
  - [ ] A.11 Implement `internal/store/mcp_session_store.go`: `Create`, `GetByID`, `UpdateInitialized`, `UpdateLastSeen` (async-safe), `Revoke`, `ReapExpired(before time.Time)`, `ReapOrphanedPrincipals`.
  - [ ] A.12 Implement `internal/store/user_external_identity_store.go`: `UpsertPending`, `Approve`, `Reject`, `ListPending`, `GetByProviderSub`.
  - [ ] A.13 Extend `internal/store/party_store.go` with `CreateServiceAccount(ctx, name)`.
  - [ ] A.14 **H6-residual**: `internal/store/party_store.go` `Delete(partyID)` path — before cascade, enumerate `api_client_credentials WHERE party_id = ?` and emit client_ids for credcache invalidation (return the slice to caller; caller — party handler in Group G — calls `credcache.Invalidate` per row). Document the contract in godoc.
  - [ ] A.15 Extend `internal/model/catalog.go` — `CatalogFilter.ProjectIDs []string` (context-supplied; M4 resolution).
  - [ ] A.16 Arch-go check: verify `internal/model` has no new imports outside stdlib.
  - [ ] A.17 Ensure A.1 tests pass. Run `rtk go test ./internal/model/... ./internal/db/... ./internal/store/... -run 'TestMigration010|TestApiClient|TestMcpSession|TestUserExternal|TestPartyStore_CreateService|TestSessionPrincipalRef'`.

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

- [ ] B.0 Build dual-auth pipeline
  - [ ] B.1 Write 8 focused tests:
    - `TestApiKeyValidator_HappyPath_BcryptCache` (asserts second lookup within 10s skips bcrypt)
    - `TestApiKeyValidator_RateLimit_429_After_30_Fails_In_60s`
    - `TestCredCache_Invalidate_EvictsEntry` (M-new-3: asserts evict but documents no-cancel-in-flight)
    - `TestFederationDex_VerifyID_Audience_Mismatch_Rejected`
    - `TestFederationJWKS_StaleServe_On_Refresh_Failure_AndMetric`
    - `TestPRMHandler_ReturnsRFC9728_Doc_When_Enabled`
    - `TestAuthDispatch_Order_ApiKey_Then_LocalJWT_Then_Federation`
    - `TestAuthDispatch_Normalizes_To_SessionPrincipalRef_In_Ctx`
  - [ ] B.2 Create `internal/auth/credcache/credcache.go`: LRU 1024 entries, 10s TTL, `sync.RWMutex`. Methods: `Get(clientID, secret) (hit bool, ok bool)`, `Put(clientID, secret string, ok bool)`, `Invalidate(clientID string)`. Godoc explicitly notes: Invalidate holds write-lock but does NOT cancel in-flight Get calls (M-new-3).
  - [ ] B.3 Create `internal/auth/apikey/validator.go` — parses `agentlens_sk_<client_id>.<secret>`, looks up via `ApiClientCredentialStore.GetByClientID`, checks credcache, falls back to bcrypt compare, writes cache on success, increments rate limiter on failure.
  - [ ] B.4 Create `internal/auth/ratelimit/clientid_limiter.go` — in-memory per-client_id token bucket: 30 fails / 60s → 429. Reuse existing rate-limit utility if present; otherwise minimal sync.Map-backed.
  - [ ] B.5 Create `internal/auth/federation/provider.go` — `Provider` interface (`VerifyIDToken(ctx, raw) (claims, error)`, `HealthPing(ctx) error`, `JWKSMetrics()`).
  - [ ] B.6 Create `internal/auth/federation/dex/dex.go` — Dex impl using `coreos/go-oidc/v3` + `go-jose/v4`. JWKS cache: max 1 refresh per provider per 10s; on refresh failure, serve stale with `agentlens_federation_jwks_stale_serves_total` counter increment.
  - [ ] B.7 Create `internal/auth/federation/registry.go` — provider registry (map-of-providers by name) per spec §2.2.
  - [ ] B.8 Wire federation config struct into `internal/config/config.go` (`Federation.Provider`, `Federation.Audience`, `Federation.Issuer`, `Federation.JWKSURL`); env-var overrides per §2.3; `Load()` validation per §2.4.
  - [ ] B.9 Add `internal/config/config.go` `MCPServer.Enabled`, `MCPServer.AllowedOrigins`, `MCPServer.PublicURL`, `MCPServer.AuditEnabled`, `MCPServer.SessionTTL`, `MCPServer.ReaperInterval` typed fields per spec §2.1.
  - [ ] B.10 Add dispatch middleware `internal/api/middleware/auth_dispatch.go` — order per spec §3: API-key → local JWT → federation JWT. All three write `SessionPrincipalRef` to ctx via `ctxkey.SessionPrincipalRef`; resolve `AccessibleProjectIDs` (from user_projects for users, from scopes for service-accounts) and put into ctx.
  - [ ] B.11 Add ctx keys in `internal/model/ctx.go` (or new `internal/model/ctxkey/`): `SessionPrincipalRef`, `AccessibleProjectIDs`. Foundation-layer only — no `internal/auth` imports.
  - [ ] B.12 **L-new-1**: PRM handler `/.well-known/oauth-protected-resource` — build as `http.Handler` in `internal/api/handlers_prm.go`. Composition root nil-checks `cfg.Federation.Provider != ""` before registering. Do NOT register unconditionally.
  - [ ] B.13 401/403 challenge helper per §3.7 (`WWW-Authenticate: Bearer resource_metadata=...`).
  - [ ] B.14 Audit scrubber: `internal/auth/audit/audit.go` — scrubbed log emitter; uses `slog.InfoContext`; never logs secret material.
  - [ ] B.15 Verify arch-go: `internal/auth/federation/*` and `internal/auth/credcache` stay in infrastructure layer; no model-layer cycles.
  - [ ] B.16 Ensure B.1 tests pass. Run `rtk go test ./internal/auth/... -v`.

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

- [ ] C.0 Build plugin + transport + session lifecycle
  - [ ] C.1 Write 7 focused tests:
    - `TestPlugin_Lifecycle_Register_Init_Start_Stop`
    - `TestPlugin_Disabled_WhenFlagFalse_NoRoutesRegistered`
    - `TestStreamableHTTP_POST_InitializeHandshake_Returns_SessionID`
    - `TestStreamableHTTP_GET_ServerSideStreaming`
    - `TestStreamableHTTP_EchoesMCPProtocolVersion_Header`
    - `TestSession_Init_PersistedToDB_InitializedAtSet`
    - `TestStatusEndpoint_ReturnsSessionStats`
  - [ ] C.2 Create package `plugins/mcpserver/` modeled on `plugins/health/`: `plugin.go`, `config.go`, `routes.go`, `handlers.go`, `session.go`, `errors.go`, `README.md`.
  - [ ] C.3 Struct name: `Plugin` (satisfies arch-go `Plugin` suffix via package namespacing).
  - [ ] C.4 Implement `kernel.Plugin` lifecycle: `Register`, `Init` (reads config, constructs ToolRegistry, returns `ErrLicenseRequired` path is N/A — this is OSS), `Start` (launches reaper, async update worker), `Stop` (flushes async channel + stops workers).
  - [ ] C.5 Add `WireImpl` interface + factory registry in `plugins/mcpserver/wire/` per spec §5.3 (single concrete impl `streamablehttp` initially).
  - [ ] C.6 Implement Streamable HTTP transport per MCP 2025-11-25 in `plugins/mcpserver/wire/streamablehttp.go`:
    - POST handler: accept JSON-RPC; on `initialize`, create `mcp_sessions` row, assign `MCP-Session-Id` response header, set `initialized_at`.
    - GET handler: server-sent events for server→client messages.
    - Echo `MCP-Protocol-Version` header.
    - Reject requests without valid session except `initialize`.
  - [ ] C.7 Implement DB-backed session management (`session.go`): wraps `MCPSessionStore`; in-memory index for hot path; persist on init and on revocation.
  - [ ] C.8 JSON-RPC dispatcher (`handlers.go`): `initialize`, `tools/list`, `tools/call`, `ping`. Out-of-scope methods → `method not found` JSON-RPC error.
  - [ ] C.9 Implement `errors.go` mapping per spec §5.9 (store errors → JSON-RPC codes).
  - [ ] C.10 Implement `/api/mcp/status` handler returning session counts + uptime (spec §5.10). Not under `/api/mcp` — separate mount at plugin level; registered via `kernel.RegisterRoutes("/api/mcp/status", ...)`.
  - [ ] C.11 Async `last_used_at` / `last_seen_at` updater: bounded channel (cap 1024) + 30s flush tick + flush on `Stop()` + `agentlens_mcp_last_seen_drops_total` counter on full-channel drop.
  - [ ] C.12 Config plumbing: `plugins/mcpserver/config.go` reads `cfg.MCP.*` and panics early if enabled-without-required-fields.
  - [ ] C.13 Arch-go verify: `plugins/mcpserver/` imports only `kernel` + `internal/model` + std/third-party — NEVER `internal/api` or `internal/auth`.
  - [ ] C.14 Ensure C.1 tests pass. Run `rtk go test ./plugins/mcpserver/... -v`.

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

- [ ] D.0 Build ToolRegistry + 4 tools + loopback
  - [ ] D.1 Write 6 focused tests:
    - `TestToolRegistry_Register_And_Dispatch`
    - `TestAgentSearch_CallsLoopback_WithCtxFilter`
    - `TestAgentGet_NotFound_Returns_JsonRPC_Error`
    - `TestCapabilitiesList_ShapeMatchesRESTContract`
    - `TestAgentCard_Returns_RawCard_When_Present`
    - `TestBuildLoopbackFunc_PreservesContext_Via_WithContext` (M-new-1: asserts outer ctx's SessionPrincipalRef + AccessibleProjectIDs reach the inner handler; asserts user-supplied `?projects=` in tool args CANNOT override ctx filter)
  - [ ] D.2 Define `ToolRegistry` interface in `plugins/mcpserver/tools/registry.go` per spec §6.1: `Register(name, handler)`, `Dispatch(ctx, name, args) (result, error)`, `List() []ToolDescriptor`.
  - [ ] D.3 Implement `api.BuildLoopbackFunc(chiRouter)` in `internal/api/loopback.go` — returns a `func(ctx, method, path, body) ([]byte, int, error)` that:
    - Builds `http.Request` with `.WithContext(outerCtx)` (M-new-1 requirement).
    - Dispatches through chiRouter using `httptest.ResponseRecorder`.
    - Returns recorder.Code + recorder.Body bytes.
  - [ ] D.4 Implement `agent_search` tool — maps MCP args to GET `/api/v1/catalog?...`; IGNORES any `projects` arg in MCP input (M4 resolution); filter sourced only from ctx `AccessibleProjectIDs`.
  - [ ] D.5 Implement `agent_get` tool — GET `/api/v1/catalog/{id}`; error-map 404 → MCP "not found".
  - [ ] D.6 Implement `capabilities_list` tool — GET `/api/v1/catalog/{id}/capabilities`; shape via mapper in `tools/shapers.go`.
  - [ ] D.7 Implement `agent_card` tool — GET raw card bytes via existing endpoint; pass-through content-type.
  - [ ] D.8 Create shapers/mappers in `plugins/mcpserver/tools/shapers.go` — pure funcs, no side effects.
  - [ ] D.9 Register all 4 tools in `plugins/mcpserver/tools/register.go` called from `Plugin.Init()`.
  - [ ] D.10 Document (in code comment) the v2 translator path per spec §6.5 — not implemented, structured for future.
  - [ ] D.11 Ensure D.1 tests pass. Run `rtk go test ./plugins/mcpserver/tools/... ./internal/api/... -run 'Loopback|ToolRegistry|AgentSearch|AgentGet|Capabilities|AgentCard' -v`.

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

- [ ] E.0 Authorization + Origin middleware
  - [ ] E.1 Write 5 focused tests:
    - `TestOriginMiddleware_Allowlist_DefaultEmpty_Rejects_All_403`
    - `TestOriginMiddleware_ConfiguredOrigin_Allowed`
    - `TestScopeByAccessibleProjects_AppendsCtxFilter_NoURLMutation`
    - `TestRequirePermission_ServiceAccountsRead_RejectsMissingPerm_403`
    - `TestAuthDecisionOrder_OriginThenAuthThenScope` (per §4.8)
  - [ ] E.2 Create `internal/api/middleware/origin.go`: reads configured allowlist, rejects 403 on mismatch. Explicitly scoped — only attached to `/api/mcp` chain in composition root; does NOT alter global CORS.
  - [ ] E.3 Create `internal/api/middleware/scope_by_projects.go`: reads `SessionPrincipalRef` from ctx → resolves `AccessibleProjectIDs` → injects into `CatalogFilter.ProjectIDs` via ctx (no URL mutation).
  - [ ] E.4 Wire `RequirePermission(auth.PermServiceAccountsRead|Write|Delete)` on the 3 new permissions. Use `auth.Perm*` constants — never raw strings.
  - [ ] E.5 Add permission constants to `internal/auth/permissions.go` (`PermServiceAccountsRead`, `PermServiceAccountsWrite`, `PermServiceAccountsDelete`).
  - [ ] E.6 Verify `internal/store/catalog_store.go` honors `CatalogFilter.ProjectIDs` (SQL WHERE clause). Add unit test `TestCatalogStore_FiltersByProjectIDs`.
  - [ ] E.7 Arch-go verify: origin/scope middlewares in `internal/api/middleware/` import only model + ctxkey.
  - [ ] E.8 Ensure E.1 tests pass. Run `rtk go test ./internal/api/middleware/... ./internal/auth/... -run 'Origin|Scope|RequirePermission|AuthDecisionOrder' -v`.

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

- [ ] F.0 Self-registration, OTel, and **canonical integration wiring**
  - [ ] F.1 Write 6 focused tests:
    - `TestSelfRegistration_CatalogEntry_Created_Idempotent_ByAgentKey`
    - `TestSelfRegistration_MultiInstance_DisambiguatedByPublicURL` (M6)
    - `TestFederationHealthLoop_UpdatesReadyzState`
    - `TestReadyz_Returns503_When_JWKS_Unreachable`
    - `TestSessionReaper_ExpiresStaleSessions_EverY60s`
    - `TestOTelMetrics_Expose_InvocationsAndCredCacheHits`
  - [ ] F.2 Self-registration in `plugins/mcpserver/plugin.go` `Init()`: compute `AgentKey = SHA256("mcp" + "agentlens:mcp-discovery:" + canonical_public_url)`; `store.Create(CatalogEntry)` idempotently (on duplicate endpoint, update existing).
  - [ ] F.3 OTel spans in `plugins/mcpserver/telemetry.go`: `agentlens.mcp.initialize`, `agentlens.mcp.tool_call`, `agentlens.mcp.authdispatch`, `agentlens.mcp.jwks_refresh`.
  - [ ] F.4 OTel metrics: `agentlens_mcp_invocations_total`, `agentlens_mcp_tool_calls_total{tool}`, `agentlens_mcp_active_sessions`, `agentlens_mcp_credcache_hits_total`, `agentlens_mcp_credcache_misses_total`, `agentlens_mcp_credcache_dropped_total`, `agentlens_federation_jwks_stale_serves_total`, `agentlens_federation_dex_health` (gauge).
  - [ ] F.5 Federation health loop: ticker goroutine ping-probing Dex discovery endpoint; updates shared health state + metric.
  - [ ] F.6 `/readyz` extension in `internal/api/handlers_health.go`: DB ping + (if federation enabled) JWKS reachable; 503 when either degraded.
  - [ ] F.7 Session reaper goroutine: 60s ticker calls `MCPSessionStore.ReapExpired` + `ReapOrphanedPrincipals`; metric counter for reaped rows.
  - [ ] F.8 Startup WARN when `mcp_server.audit_enabled=false` via `slog.Warn` during `Plugin.Init`.
  - [ ] F.9 **INTEGRATION WIRING (canonical) — `cmd/agentlens/main.go`** per spec §8.1:
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
  - [ ] F.10 Register auth dispatch middleware globally? No — only on `/api/mcp` chain per §3. Ensure REST auth path (`/api/v1/*`) is unchanged.
  - [ ] F.11 Arch-go validate: `plugins/mcpserver` still imports only kernel + foundation; `cmd/agentlens` is composition root (can import anything).
  - [ ] F.12 Ensure F.1 tests pass. Run `rtk go test ./plugins/mcpserver/... ./internal/api/... ./cmd/agentlens/... -run 'SelfRegistration|FederationHealth|Readyz|SessionReaper|OTelMetrics' -v`.

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

- [ ] G.0 Admin REST endpoints + 3 admin pages
  - [ ] G.1 Write 6 focused tests (split Go + Vitest):
    - Go: `TestServiceAccountHandler_CreateReturnsOneTimeSecret`
    - Go: `TestServiceAccountHandler_RotateSecret_409_OnConflict_UsesErrorsIs` (M-new-2 — asserts `errors.Is(err, gorm.ErrDuplicatedKey)`)
    - Go: `TestServiceAccountHandler_Delete_InvalidatesCredCache_PerRow` (H6-residual — asserts `credcache.Invalidate` called for every active client_id before cascade)
    - Go: `TestPendingIdentitiesHandler_ApproveRejectFlows`
    - Vitest: `ServiceAccountsPage.test.tsx` (renders table, opens create modal, displays one-time secret)
    - Vitest: `PendingIdentitiesPage.test.tsx` (approve/reject actions)
  - [ ] G.2 Add REST routes `internal/api/router.go`:
    - `POST /api/v1/service-accounts` → create + returns one-time secret in response body only.
    - `GET /api/v1/service-accounts`, `GET /api/v1/service-accounts/{id}`.
    - `PATCH /api/v1/service-accounts/{id}/secret` → rotation (M-new-2: handler catches `errors.Is(err, gorm.ErrDuplicatedKey)` → 409).
    - `DELETE /api/v1/service-accounts/{id}` (H6-residual: enumerate `api_client_credentials` for party, call `credcache.Invalidate(clientID)` per row, THEN cascade delete).
    - `GET /api/v1/external-identities/pending`, `POST /api/v1/external-identities/{id}/approve`, `POST /api/v1/external-identities/{id}/reject`.
    - All gated via `RequirePermission(auth.PermServiceAccounts*)`.
  - [ ] G.3 Create `internal/api/handlers_service_accounts.go` + `handlers_external_identities.go` per spec §8.4.
  - [ ] G.4 Update `docs/api.md` with new endpoints + permissions matrix.
  - [ ] G.5 Create `web/src/pages/ServiceAccountsPage.tsx` (list + create modal with one-time-secret display + revoke).
  - [ ] G.6 Create `web/src/pages/ServiceAccountDetailPage.tsx` (single SA detail, scopes editor, rotate secret dialog).
  - [ ] G.7 Create `web/src/pages/PendingIdentitiesPage.tsx` (tabs/table of pending federation identities, approve/reject actions).
  - [ ] G.8 Add routes in `web/src/App.tsx`: `/admin/service-accounts`, `/admin/service-accounts/:id`, `/admin/external-identities`.
  - [ ] G.9 TanStack React Query hooks for data; `data-testid` on interactive elements per spec §Visual Design.
  - [ ] G.10 Keep Vitest coverage thresholds 80/80/75/80 — add additional component tests if coverage dips.
  - [ ] G.11 Playwright E2E screenshot capture: `e2e/tests/service-accounts.spec.ts` — login → create SA → screenshot under `docs/images/service-accounts.png`.
  - [ ] G.12 Update `docs/end-user-guide.md` with new admin pages + screenshots.
  - [ ] G.13 Ensure G.1 tests pass. Run `rtk go test ./internal/api/... -run 'ServiceAccount|PendingIdent'`, `rtk make web-test`, `rtk make e2e-test`.

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

- [ ] H.0 Ship deployable artifacts + docs
  - [ ] H.1 Write 4 focused tests:
    - `helm lint --strict` passes with default + ci-values.
    - `helm template ... --debug > /dev/null` succeeds with `dex.enabled=true` and `dex.enabled=false`.
    - `./scripts/test-helm-templates.sh` passes.
    - `docker-compose -f docker-compose.dev.yml config` validates.
  - [ ] H.2 Bump `helm/agentlens/Chart.yaml` to `version: 0.3.0`, `appVersion: 0.3.0`.
  - [ ] H.3 Add Dex as conditional subchart dependency with `condition: dex.enabled` and pinned digest (from Chore-02).
  - [ ] H.4 Add `helm/agentlens/templates/` entries for MCP envs: `AGENTLENS_MCP_ENABLED`, `AGENTLENS_MCP_PUBLIC_URL`, `AGENTLENS_MCP_ALLOWED_ORIGINS`, `AGENTLENS_FEDERATION_*`.
  - [ ] H.5 Update `helm/agentlens/values.yaml` + `ci/ci-values.yaml` (latter enables mcp + dex for render check).
  - [ ] H.6 Create `docker-compose.dev.yml` with AgentLens + Dex services; Dex config file at `deploy/dex/config-dev.yaml`.
  - [ ] H.7 Update `docs/settings.md` with new config keys.
  - [ ] H.8 Update `docs/architecture.md` with MCP plugin Mermaid diagram (no PlantUML/ASCII per project standards).
  - [ ] H.9 Update `docs/auth.md` (M7) with service-account + federation flows + PRM.
  - [ ] H.10 Create `docs/mcp-quickstart.md` — operator 5-min guide.
  - [ ] H.11 Create `docs/observability.md` — OTel span/metric catalog + operator alerts per spec §7.7.
  - [ ] H.12 Add README.md MCP callout linking to quickstart.
  - [ ] H.13 Verify `rtk make all` green (format → lint → test → arch-test → build).
  - [ ] H.14 Ensure H.1 tests pass. Run `rtk helm lint --strict helm/agentlens`, `rtk helm template helm/agentlens -f helm/agentlens/values.yaml --debug > /dev/null`, `rtk ./scripts/test-helm-templates.sh`, `rtk docker-compose -f docker-compose.dev.yml config`.

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

- [ ] I.0 Fill critical testing gaps
  - [ ] I.1 Review tests from Groups A–H (~55–70 existing feature-scoped tests).
  - [ ] I.2 Analyze gaps for THIS feature specifically (focus areas: auth decision order, rotation atomicity, credcache invalidation, loopback ctx preservation, Dex stale-serve, origin strict-default).
  - [ ] I.3 Write up to 10 additional strategic tests (prioritize: multi-dialect migration idempotency end-to-end, chained auth + scope integration, PRM conditional registration, CORS non-interference with global `*`).
  - [ ] I.4 Run feature-scoped test suite: `rtk go test ./internal/{model,db,store,auth,api}/... ./plugins/mcpserver/... ./cmd/agentlens/... -run '<feature regex>'` plus `rtk make web-test` + `rtk make e2e-test`.
  - [ ] I.5 Run `rtk make all` end-to-end — must pass clean.

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

# Gap Analysis: MCP Discovery Server v1

## Summary

- **Risk Level**: High
- **Estimated Effort**: High (~8-9 weeks single engineer per design-decisions)
- **Detected Characteristics**: `modifies_existing_code`, `creates_new_entities`, `involves_data_operations` (dominant: new-entity creation with surgical modifications to kernel/router/auth/store)

## Task Characteristics

- Has reproducible defect: **no** (greenfield feature)
- Modifies existing code: **yes** (kernel interface extension, router wiring, auth middleware, CORS scoping, migration chain, Helm chart)
- Creates new entities: **yes** (plugins/mcpserver, federation package, 2 new tables + 1 enum value, ToolRegistry, WireImpl, Principal abstraction, PRM endpoint, admin UI pages)
- Involves data operations: **yes** (CRUD on service accounts, api_client_credentials, user_external_identities; self-registration CREATE on catalog_entries)
- UI heavy: **no** (backend-dominant; only 3 small admin UI pages — incidental)

## Gaps Identified

### Missing Features (new capability)

1. **MCP wire protocol plugin** — no `plugins/mcpserver/` exists; DIY Streamable HTTP + JSON-RPC + session mgmt must be built ground-up. Template: `plugins/health/`.
2. **Federation / OIDC authentication** — no existing OIDC, JWKS, or external-IdP code. New dep (`coreos/go-oidc` + `go-jose`) must be vetted against Trivy gate.
3. **Service-account identity** — `PartyKind` has no `service_account`; `api_client_credentials` table does not exist; no API-key bearer validation path.
4. **External identity mapping** — no `user_external_identities` table; no JIT/admin-approval-queue flow.
5. **Protected Resource Metadata endpoint** — no `/.well-known/oauth-protected-resource` route; no `/.well-known/*` routing currently wired.
6. **Principal abstraction** — current auth context differentiates user-vs-not; three-path normalization (API key / local JWT / federation JWT → `Principal{kind, id, permissions}`) is missing.
7. **Scope-by-accessible-projects middleware** — no existing `CatalogFilter.ProjectIDs` plumbing; default-project-public-reads rule is new.
8. **ToolRegistry + 4 tools** — no registry pattern for MCP tools; 4 hand-coded entries + HTTP loopback adapter + HTTP→MCP error-code mapping do not exist.
9. **Self-registration at plugin Init** — `plugins/health/` does not self-register; new idempotent `store.Create()` pattern.
10. **Dex deployment bundle** — no Dex subchart; no docker-compose.dev.yml reference.
11. **MCP-specific observability** — `agentlens_mcp_*` metrics, per-tool audit log, federation health loop, `/readyz` extension all new.
12. **Bootstrap admin approval queue UI** — `ServiceAccountsPage`, `ServiceAccountDetailPage`, `PendingIdentitiesPage` do not exist.

### Incomplete Features (modify existing)

1. **Kernel router wiring (structural gap)** — `kernel.RegisterRoutes` exists but `router.go` does not mount plugin routes. Resolution (clarification #1): add `Kernel.Router() chi.Router` accessor. Requires interface change propagated through `internal/kernel/kernel.go` and consumers.
2. **CORS middleware is `*`** — MCP spec mandates `Origin` → 403 for cross-origin JSON-RPC. Current global `CORSMiddleware` must be scoped: MCP routes need a stricter middleware; global behavior must remain unchanged for REST/UI.
3. **Migration chain drift** — spec references "migration 008"; codebase highest is 9. Resolution (clarification #3): use **010**. All references in spec, docs, tests must be re-pointed.
4. **Auth dispatch** — existing `RequireAuth` only validates local JWT; must fan-out to API-key and federation-JWT paths. Risk of breaking existing REST auth if not additive-only.
5. **Audit logging** — existing slog patterns must extend to cover per-tool invocation with principal scrubbing (no secret leak in log fields).

### Behavioral Changes Needed

- `/healthz` and `/readyz` must extend to report `federation.enabled/reachable` + `last_checked_at`.
- `SQLStore.CreateEntry` already auto-assigns to default project — reuse unchanged; self-registration side-effect inherits this.
- `arch-go.yml` forbids `plugins/*` importing `internal/api`. Loopback adapter invokes chi router via `Kernel.Router().ServeHTTP(recorder, req)` — stays within kernel+foundation. Verify arch-go passes after interface expansion.

## Change Type

- **Classification**: Mixed — predominantly **additive** (one new plugin, 2 new tables, new middleware layer, new federation package) with targeted **modificative** surgery on kernel interface (`Router()` accessor), migration chain, CORS, and `RequireAuth` dispatch.
- **Compatibility requirements**: **Strict**. All existing REST/UI flows must be unchanged. Plugin ships behind `mcp_server.enabled=false` default so main stays green on merge.

## Integration Points

| Integration Point | Type | Notes |
|---|---|---|
| `internal/kernel/kernel.go` — `Kernel.Router()` | Interface extension | Clarification #1 resolved; leaks `chi.Router` into kernel |
| `internal/api/router.go` middleware stack | Modify | Scoped `Origin`-enforcing middleware on `/api/mcp`; keep global CORS untouched |
| `internal/api/auth_middleware.go` | Extend | Dispatch to API-key / federation validators; Principal normalization |
| `internal/db/migrations.go` | Append | Migration 010 (not 008 as spec says) |
| `internal/model/party.go` | Extend | `PartyKindServiceAccount` |
| `internal/store/party_store.go` | Extend | `CreateServiceAccount` |
| `internal/store/sql_store.go` | Consume (unchanged) | Self-registration uses existing `CreateEntry` |
| `plugins/mcpserver/` | New package | Modeled on `plugins/health/`; owns wire, tools, /.well-known, /status |
| `internal/auth/federation/` | New package | Provider interface + Dex impl + JWKS |
| `internal/auth/principal.go` | New | Union type normalizing three auth paths |
| `cmd/agentlens/main.go` | Modify | Plugin registration; federation Start/Stop ordering vs `pm.InitAll` |
| `charts/agentlens/` | Modify + new subchart | Version bump + Dex subchart; `ci-values.yaml` |
| `arch-go.yml` | Possibly extend | New `namingRules` for `plugins/mcpserver/` internal packages |
| `web/src/routes/admin/` | New pages | 3 shadcn/ui pages |
| `web/src/types.ts` | Extend | Service-account, credential, external-identity response types |
| OTel meter/tracer registration | Extend | `agentlens_mcp_*` namespace |

## Patterns to Follow

- **Plugin scaffold** → `plugins/health/` (lifecycle, config gate, Register/Init/Start/Stop).
- **Migration pattern** → migration 007 (dual-dialect schema + seed + backfill); idempotent via `IF NOT EXISTS`.
- **Helm subchart** → bitnami/postgresql precedent in chart (conditional `postgresql.enabled`); mirror for Dex.
- **Admin UI** → existing `web/src/routes/admin/` conventions; shadcn/ui primitives only.
- **Capability registry** → `internal/model/capability.go` union-struct discriminator pattern (for ToolRegistry if discriminator needed).
- **Context-aware JWT validation** → existing `internal/auth/jwt.go` (extend via provider abstraction, not fork).

## Architectural Impact

- **Kernel surface area grows** — new `Router()` accessor permanently exposes chi. Acceptable per clarification but consumes the architecture budget; future kernel-abstraction work may need to re-hide.
- **New external runtime dep** — Dex as sidecar/subchart. First time AgentLens relies on out-of-process auth infrastructure. Health/readiness semantics now depend on Dex reachability when `federation.enabled=true`.
- **arch-go impact** — plugins/mcpserver imports only kernel+foundation (compliant). New `internal/auth/federation` sits in infrastructure layer (compliant). Requires re-run and possibly extended `namingRules` once 3+ plugin subpackages emerge.
- **Binary size / CGO** — `coreos/go-oidc` pulls `go-jose`; both pure-Go, no CGO impact. Trivy scan must pass CRITICAL+HIGH gate.

## Data Lifecycle Analysis

### Entity: Service Account (`parties` row, kind=`service_account`)

| Operation | Backend | UI | User Access | Status |
|---|---|---|---|---|
| CREATE | `POST /api/v1/service-accounts` (new) | `ServiceAccountsPage` (new) | Admin nav entry (new) | ❌ missing, in scope |
| READ  | `GET /api/v1/service-accounts[/{id}]` (new) | `ServiceAccountsPage`, `ServiceAccountDetailPage` | Admin nav + routing (new) | ❌ missing, in scope |
| UPDATE| membership changes via existing `PartyRelationship` CRUD | Detail page (new) | via detail page | ⚠️ partial (reuse + new UI) |
| DELETE| `DELETE /api/v1/service-accounts/{id}` (new) | Detail page action | via detail page | ❌ missing, in scope |

### Entity: API Client Credential (`api_client_credentials`)

| Operation | Backend | UI | User Access | Status |
|---|---|---|---|---|
| CREATE | `POST /api/v1/service-accounts/{id}/keys` (one-time secret) | Detail page "Issue key" | Button on detail page | ❌ missing, in scope |
| READ  | `GET /api/v1/service-accounts/{id}/keys` (metadata only, no secret) | Key list in detail page | Detail page | ❌ missing, in scope |
| UPDATE| n/a (immutable; rotate = create+revoke) | — | — | n/a by design |
| DELETE (revoke) | `DELETE .../keys/{keyID}` | Detail page action | Detail page | ❌ missing, in scope |

### Entity: User External Identity (`user_external_identities`)

| Operation | Backend | UI | User Access | Status |
|---|---|---|---|---|
| CREATE | Implicit on approved federation login (admin-approval-queue default) | `PendingIdentitiesPage` (new) | Admin nav (new) | ❌ missing, in scope |
| READ  | `GET /api/v1/external-identities` | `PendingIdentitiesPage` | Admin nav | ❌ missing, in scope |
| UPDATE| Approve/link via admin action | Approve button | Admin | ❌ missing, in scope |
| DELETE| `DELETE /api/v1/external-identities/{id}` | Detail action | Admin | ❌ missing, in scope |

### Entity: MCP catalog self-registration (`catalog_entries`)

| Operation | Backend | UI | User Access | Status |
|---|---|---|---|---|
| CREATE (idempotent at Init) | `store.Create()` via existing API | Appears in existing Catalog UI | Existing catalog page | ✅ Complete (reuses existing read/UI paths) |
| READ  | Existing catalog GET | Existing catalog UI | Existing nav | ✅ Complete |
| UPDATE/DELETE | Lifecycle — entry removed on plugin Stop | Existing UI reflects | Existing | ✅ Complete |

**Completeness**: 100% (across all four entities) *provided* the admin REST routes + 3 admin UI pages ship in the same PR (they are in scope per product-brief §8 and design-decisions area 2A). No orphaned operations if Group G (Admin UI) ships alongside Groups A+E.

**Risk**: If Group G slips from v1, service accounts become CREATE-able only via migration/seed — orphaned without UI. Gate: Group G must not be carved out.

**Completeness score**: 100% (in-scope) / 50% (if Group G slips).

## Issues Requiring Decisions

### Critical (Must Decide Before Proceeding)

1. **Dex E2E testing strategy** (clarification #5 still open)
   - Options: (A) JWKS stub only (`httptest.NewServer` serving static JWKS) for all tests including E2E; (B) Real Dex container in E2E via docker-compose (full DCR + OAuth roundtrip); (C) Hybrid — unit/integration use stub, E2E gets real Dex behind a CI flag.
   - Recommendation: **C (Hybrid)**.
   - Rationale: Unit/integration runs stay fast (stub). Real Dex in one E2E path catches DCR/issuer/aud binding regressions that stubs hide. Gated by CI flag keeps default `make e2e-test` time bounded. Full-stub (A) leaves the Dex DCR compliance gating item from design-decisions area 3F unverified at merge time.

2. **Session store location**
   - Options: (A) In-memory TTL-bounded `sync.Map` (ephemeral; lost on restart); (B) New DB table `mcp_sessions` (persistent across restarts).
   - Recommendation: **A (in-memory with TTL)**.
   - Rationale: MCP sessions are short-lived and spec explicitly says sessions-MUST-NOT-be-auth. Persistence buys nothing; adds a migration and write amplification on every JSON-RPC call. Risk is bounded: clients reconnect transparently. Revisit in v1.5 if HA deployment lands.

3. **Completeness of admin UI (Group G) in v1 PR**
   - Options: (A) Ship all 3 pages in the single PR; (B) Ship backend only, UI follow-up.
   - Recommendation: **A**.
   - Rationale: Without UI, the feature is orphaned for Priya (cannot create service accounts, cannot approve external identities). Backend-only leaves operators with curl-only provisioning — unacceptable for an enabling persona.

### Important (Should Decide)

4. **Helm chart version bump**
   - Options: (A) 0.2.0 → 0.3.0 (per spec §8); (B) 0.2.0 → 0.2.x patch.
   - Default: **A (0.3.0)**.
   - Rationale: New optional subchart (Dex) + new values keys are minor-version changes by SemVer; spec already commits to 0.3.0.

5. **Ship `docker-compose.dev.yml` in this PR**
   - Options: (A) Include alongside Helm chart for local Dex testing; (B) Defer to follow-up PR.
   - Default: **A (include)**.
   - Rationale: Needed for Karol's "paste URL → works" validation locally and for option C E2E strategy. Without it, contributors cannot reproduce federation flow without a cluster.

6. **Extend `arch-go.yml` naming rules for `plugins/mcpserver/` subpackages**
   - Options: (A) Add `namingRules` entries now for `plugins/mcpserver/{wire, tools, session, loopback}` once 3+ subpackages exist; (B) Defer until a second plugin adopts the same layout (CLAUDE.md rule: establish after 3+ instances).
   - Default: **B (defer)**.
   - Rationale: Only one plugin uses this layout; add when pattern recurs. Keeps arch-go.yml focused.

7. **Scoped CORS middleware implementation shape**
   - Options: (A) Per-route middleware registered by MCP plugin at Init (spec-mandated 403-on-Origin); (B) Global middleware with path-prefix branching.
   - Default: **A**.
   - Rationale: Route-scoped keeps global CORS (`*`) untouched, avoids regression risk to REST/UI. Plugin owns its policy.

8. **Migration-number alignment in spec document**
   - Options: (A) Update feature-spec.md references from 008 → 010 before implementation; (B) Leave spec as-is and track divergence in clarifications only.
   - Default: **A**.
   - Rationale: Spec is the source of truth for implementers; stale numbering will cause confusion during code review. One-line doc fix.

## Recommendations

- Land Groups A (data), B (auth), C (plugin+wire), D (tools) sequentially; parallelize E (authz middleware), F (observability), G (admin UI), H (deployment) once C is green.
- Build JWKS stub harness early (Group B) so Groups C/D unblock before Dex subchart (Group H) stabilizes.
- Add a Trivy pre-commit sanity run for `coreos/go-oidc` + `go-jose` before locking versions.
- Keep `mcp_server.enabled=false` default through PR review and flip to `true` only after E2E green on `main`.
- Before merge, run arch-go locally with the new `Kernel.Router()` accessor + new `internal/auth/federation/` package to confirm 100% layer compliance maintained.

## Risk Assessment

- **Complexity Risk**: High. DIY wire protocol (3 wks), OIDC integration (1.5 wks), cross-layer auth changes, and new runtime dependency compound. Mitigated by `WireImpl` interface (swap-out path) and feature flag default-off.
- **Integration Risk**: High. Five modification points on hot paths (kernel iface, router, CORS, RequireAuth dispatch, migrations). Any regression breaks existing REST/UI. Mitigated by additive-only discipline and the `mcp_server.enabled=false` default.
- **Regression Risk**: Medium. Biggest exposure is `RequireAuth` dispatch refactor and CORS scoping. Required: existing REST handler test suite must be exercised unchanged after both changes; add targeted tests for `Origin` rejection on `/api/mcp` only.
- **Supply-chain Risk**: Medium. Two new deps (`coreos/go-oidc`, `go-jose`). Both pure-Go, both mature, both widely used — but Trivy gate is a hard CI pass/fail.
- **Spec-compliance Risk**: Medium. MCP 2025-11-25 DCR + Dex compatibility flagged in design-decisions area 3F as needing verification in Phase 6. Decision #1 above (hybrid E2E) is the mitigation.

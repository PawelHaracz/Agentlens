# Spec Audit: MCP Discovery Server v1

Auditor: spec-auditor (independent)
Date: 2026-04-17
Spec: `implementation/spec.md`
Verdict: **fail** (blockers must be resolved before planning; see C1, C2)

## Verdict Summary

| Severity | Count |
|---|---|
| Critical | 2 |
| High | 6 |
| Medium | 7 |
| Low | 5 |

Top findings:
1. **C1 — arch-go layer violation**: plugin is specified to import `internal/auth` (both the `auth.Principal` type and the `federation.Provider` type from `internal/auth/federation`), but `arch-go.yml` line 97-104 forbids `**.plugins.**` from depending on `**.internal.auth`. Current codebase has zero plugins importing `internal/auth` (verified). Spec is self-contradictory — §5 claims "no arch-go violation" but §5.2 struct + §8 list `internal/auth` in the plugin import set, and `auth.Principal` threads through session store, loopback, and audit. Every path that resolves `Principal` server-side sits inside the plugin. Implementation cannot pass `rtk make arch-test` as drawn.
2. **C2 — `Kernel.Router() chi.Router` leaks an infrastructure type across the kernel interface and still does not solve plugin↔auth coupling**: `internal/kernel/kernel.go` currently exposes `RegisterRoutes(prefix, http.Handler)` and `RegisterMiddleware`. Adding `Router() chi.Router` forces the kernel to depend on the chi package (currently a pure-stdlib interface — only `net/http`), and the plugin must still build chain middleware (`RequireAuth`, `RequirePermission`, `ScopeByAccessibleProjects`) that live in `internal/api` — which the plugin cannot import. The spec hand-waves this in §5.2 ("Mount routes on k.Router() … with Origin middleware + RequireAuth + ScopeByAccessibleProjects") without specifying which layer *constructs* those middleware chains. No clear path to code.

---

## Critical

### C1. Plugin → `internal/auth` import forbidden by arch-go
**Spec ref**: §5.1 file layout (`plugins/mcpserver/loopback.go`, `session.go` referencing `*auth.Principal`), §5.2 struct `fedProvider federation.Provider`, §5.6 `SessionStore.Create(ctx, principal *auth.Principal)`, §8 "Layer boundaries" enumerating `internal/auth` as a permitted plugin import.
**Evidence**: `arch-go.yml:97-104`:
```
- package: "**.plugins.**"
  shouldNotDependsOn:
    internal:
      - "**.internal.api"
      - "**.internal.auth"
      - "**.internal.server"
      - "**.internal.service"
      - "**.cmd.**"
```
Grep `internal/auth` under `plugins/` → 0 files.
**Category**: Incorrect (architectural contradiction)
**Severity**: Critical — blocks arch-test green build; spec success criterion explicitly requires "arch-go: 100% layer compliance; plugin never imports `internal/api`" but silently permits `internal/auth`.
**Recommendation**: Either (a) relax `arch-go.yml` plugin rule to allow `internal/auth` (document as ADR, get approval), (b) move `Principal` into `internal/model` (foundation) and keep federation Provider behind a kernel-exposed interface, or (c) define `SessionPrincipalRef` (opaque struct: ID, Kind, PartyID, Permissions) in `internal/model`, resolve real `auth.Principal` inside middleware that lives in `internal/api`, and pass only the ref into the plugin. Option (c) preserves layering.

### C2. `Kernel.Router() chi.Router` introduces two problems
**Spec ref**: §5.11 "Added to `internal/kernel/kernel.go` `Kernel` interface"; §5.2 plugin `Init` mounts routes "with Origin middleware + RequireAuth + ScopeByAccessibleProjects".
**Evidence**: `internal/kernel/plugin.go:73-82` — `Kernel` interface today is `net/http`-only (no chi types). `RequireAuth`/`RequirePermission`/`ScopeByAccessibleProjects` will live in `internal/api` (the spec itself places `ScopeByAccessibleProjects` in §4.4 without pinning a package; all existing analogues in `internal/api/*_middleware.go`). Plugin cannot import them; kernel cannot import them.
**Category**: Ambiguous + Incorrect
**Severity**: Critical — the primary architectural novelty of the spec has no implementable wiring.
**Recommendation**: Option A — keep `RegisterRoutes(prefix, http.Handler)`; composition root (`cmd/agentlens/main.go`) wraps the plugin handler with REST middleware chain before calling `RegisterRoutes`. Plugin exports a raw `http.Handler`; middleware is injected by main. This keeps layering clean and removes the need for `Kernel.Router()`. Option B — expose `Kernel.AuthMiddleware() func(http.Handler) http.Handler` + `Kernel.PermissionMiddleware(perm string) …` — narrow, typed, no chi leak. Either way, drop the chi-typed accessor.

---

## High

### H1. Migration `Name` field does not exist
**Spec ref**: §1.5 `{Version: 10, Name: "mcp_discovery_v1", Up: ...}`.
**Evidence**: `internal/db/migrations.go:36-51` (migration009) shows `{Version, Description, Up}`. No `Name` field.
**Severity**: High — spec will not compile if taken literally.
**Recommendation**: Use `Description: "mcp_discovery_v1"` or rename the struct field if desired (separate change).

### H2. Partial index on SQLite requires ≥3.8.0 and will silently no-op on older drivers; behavior under `AutoMigrate` is not defined
**Spec ref**: §1.2 `CREATE UNIQUE INDEX … WHERE revoked_at IS NULL;   -- partial; PG + SQLite 3.8+`; §1.5 "Dialect-branched `CREATE INDEX IF NOT EXISTS` statements (partial indexes on both dialects)".
**Evidence**: `standards/backend/database-dialects.md` requires dialect-branch and forward-only migrations; `migrations.go` uses `Exec` for raw DDL when needed. Spec does not state whether GORM AutoMigrate struct tags or raw `Exec` produces the partial index, nor the exact dialect branch text.
**Severity**: High — "one active secret" invariant depends entirely on this index being present. Silent miss = security invariant violated at runtime.
**Recommendation**: Pin explicit `tx.Exec(...)` text for both dialects, add migration test asserting the index exists on both SQLite `:memory:` and (if CI adds it) PostgreSQL. Reject implementation if DDL does not appear in `sqlite_master` / `pg_indexes` post-migrate.

### H3. Rotation race with partial unique index
**Spec ref**: §1.6 step 3 — single transaction revokes current + inserts new row; partial UNIQUE (`revoked_at IS NULL`) enforces one active.
**Evidence**: Ordering within the transaction matters — if INSERT runs before UPDATE of `revoked_at`, both rows transiently have NULL revoked_at, violating the partial unique constraint inside the transaction. Some engines check constraints immediately (SQLite default), some defer (PG `DEFERRABLE`). Spec does not state ordering or deferral.
**Severity**: High — rotation may fail under concurrent calls or even single-call if ordering wrong.
**Recommendation**: Require UPDATE-first-then-INSERT ordering, documented in spec. Add concurrent-rotation test.

### H4. Scoped Origin middleware attachment not specified
**Spec ref**: §5.5 "Plugin-owned, scoped to `/api/mcp` only". §2.1 `AllowedOrigins`. 
**Evidence**: If plugin cannot touch `internal/api` (arch-go) and cannot mount chi Route groups (C2), how does a plugin attach middleware that is scoped only to `/api/mcp` and runs BEFORE `RequireAuth` for Origin-check but after chi's core router? Current kernel offers `RegisterMiddleware(func(http.Handler) http.Handler)` but that is *global*, not per-route.
**Severity**: High — ties directly to C1/C2; strict-default Origin check cannot be plugin-owned without a mechanism.
**Recommendation**: Resolve with C2 (composition root wraps handler with middleware stack, including Origin check configured via `cfg.MCPServerConfig.AllowedOrigins`).

### H5. `last_used_at` 30s buffered goroutine is undefined
**Spec ref**: §1.6.2 "async-updates `last_used_at` (30s buffered goroutine)".
**Evidence**: No detail on buffer size, flush-on-shutdown semantics, or what happens when the buffer fills. Could swallow audit-relevant data on crash.
**Severity**: High — affects forensics on compromised credentials.
**Recommendation**: Specify buffer size, drop-policy, shutdown-drain hook (wired into `Plugin.Stop`), and whether the same pattern is used for `user_external_identities.last_seen_at`.

### H6. `tools/call` loopback revalidates bearer — but API keys don't carry through unchanged
**Spec ref**: §5.8 "Bearer reconstruction: pass original `Authorization` header through. `RequireAuth` revalidates on loopback."
**Evidence**: API key format `agentlens_sk_<client_id>.<secret>` requires bcrypt compare (expensive: ~100ms at cost 12). Revalidating on every tool call doubles the cost (one for MCP entry, one for loopback). With p95 < 100ms SLO (§Success Criteria), one bcrypt already exceeds budget on commodity hardware; two compounds.
**Severity**: High — performance SLO unachievable as drawn.
**Recommendation**: Either (a) cache successful bcrypt outcomes keyed by `client_id + hash(secret)` with short TTL (10s), (b) have MCP plugin emit a short-lived internal JWT after first bcrypt success and pass that to loopback, or (c) lower cost to 10 (per bcrypt guidance, still strong). Currently `bcrypt cost 12` is written as non-negotiable in `standards/security/authentication.md` for passwords; API-secret is a different asset class — worth an ADR to pin cost.

---

## Medium

### M1. `coreos/go-oidc` + `go-jose` versions unconfirmed
**Spec ref**: §Key libraries "Confirm exact versions at implementation-planner phase." Carry-over warning listed in audit context.
**Severity**: Medium — blocks supply-chain review + CodeQL scan; also affects govulncheck gate.
**Recommendation**: Pin in spec before planning. Suggest `go-oidc/v3 ^3.10.0`, `go-jose/v4 ^4.0.x` (v3 is EOL soon).

### M2. JWKS cache invalidation on `kid` miss underspecified
**Spec ref**: §3.5 step 2 — "single forced refresh on miss".
**Evidence**: No jitter, no per-provider rate limit, no fallback on refresh failure (stale cache vs hard 503). DoS-surface via unknown `kid` storm.
**Severity**: Medium — security + availability.
**Recommendation**: Specify: max 1 refresh per provider per 10s, stale-cache-on-refresh-fail with metric increment.

### M3. Session `principal_type` enum of only 2 values loses fidelity
**Spec ref**: §1.4 `CHECK(principal_type IN ('service_account','user'))`. §3.1 principal has `AuthMethod` (`basic_jwt`/`federation:<kind>`/`api_key`).
**Evidence**: For audit review, knowing a session is `user` is not enough — was it local or federated? Collapses to user → fraud analysis harder.
**Severity**: Medium.
**Recommendation**: Store `auth_method` alongside `principal_type` or use a richer enum (`user_local`, `user_federated`, `service_account`).

### M4. `ScopeByAccessibleProjects` injection of `?projects=<csv>` is not plumbed
**Spec ref**: §4.4 "injects `?projects=<csv>` for downstream store filter. `CatalogFilter` gains `ProjectIDs []string`."
**Evidence**: Injecting query strings via `r.URL.RawQuery` mutation can collide with existing `?project=` singular param. No statement on precedence (user-supplied `?projects=` vs. middleware-injected) or on how bearer-scoped `ProjectIDs` are passed when `r.URL` already has a value.
**Severity**: Medium — authorization bypass risk if handler trusts user `?projects=`.
**Recommendation**: Pass project scope via `context.Context` value (`ctxAccessibleProjectIDs`), not URL rewrite. Handlers read from ctx, ignore/reject user `?projects=`.

### M5. Service account lockout / brute-force protection silent
**Spec ref**: §3.3 "constant-time compare"; `standards/security/authentication.md` specifies 5-fail / 15-min lockout for password auth.
**Evidence**: No rate limit or lockout on API-key guessing. 32-byte secret = 256-bit entropy so offline brute-force impractical, but online rate-limit still wise for noisy-client detection / credential stuffing via leaked prefixes.
**Severity**: Medium.
**Recommendation**: Add per-`client_id` failure counter → 429 after N fails / minute; document as hardening, not lockout (don't lock a production API key out).

### M6. `AgentKey` collision possibility when `endpoint = "agentlens:mcp-discovery"` literal
**Spec ref**: §1 core req 9; §7.1 self-registration.
**Evidence**: Static endpoint string across deployments means all AgentLens instances pushing to a shared catalog produce same `AgentKey`. For single-instance v1 fine, but spec says nothing about multi-instance behavior.
**Severity**: Medium (edge case, but v1 is about production operators).
**Recommendation**: Consider `agentlens:mcp-discovery:<canonical_url>` or `<instance_id>` to disambiguate.

### M7. PR checklist doc coverage incomplete
**Spec ref**: §Documentation table: `docs/observability.md` (new), `docs/mcp-quickstart.md` (new).
**Evidence**: `standards/global/pr-checklist.md` lists `docs/auth.md` as the canonical auth surface — not mentioned in spec's doc table. New permissions MUST update `docs/auth.md`, `docs/api.md`, `README.md` per standard.
**Severity**: Medium.
**Recommendation**: Add `docs/auth.md` row (new permissions + Principal + federation) and confirm `README.md` paragraph scope.

---

## Low

### L1. Referenced artifact missing: `analysis/codebase-analysis.md`
**Spec ref**: Linked artifacts list at top.
**Evidence**: `ls analysis/` shows `gap-analysis.md`, `requirements.md`, `clarifications.md`, `scope-clarifications.md`, `technical-clarifications.md` — no `codebase-analysis.md`. (It exists under `analysis/design-context/codebase-analysis.md`.)
**Recommendation**: Fix the path.

### L2. `mcp_sessions.principal_id` has no FK
**Spec ref**: §1.4 DDL.
**Evidence**: Unlike `api_client_credentials.party_id` which uses FK + ON DELETE CASCADE, `principal_id` is a free TEXT with no FK because it points to either a user or a party. When user/SA deleted, sessions become orphans.
**Recommendation**: Add reaper pass that revokes sessions whose principal is gone, or split into two nullable FKs.

### L3. `mcp_server.audit_enabled` is a foot-gun
**Spec ref**: §2.1 default true, but operator-togglable.
**Evidence**: Disabling audit in prod = no per-principal forensic trail. Should require explicit config to flip to false + warning log on startup.
**Recommendation**: Add startup WARN when `audit_enabled=false`.

### L4. `notifications/initialized` "No response. Marks handshake complete" — state transition not persisted
**Spec ref**: §5.7 table.
**Evidence**: If session state is DB-backed per §5.6, then handshake-complete is a state bit that should persist, otherwise reconnect-from-persistent-session cannot know whether initialize completed. Not in `mcp_sessions` schema.
**Recommendation**: Add `initialized_at` column or document acceptable behavior on restart.

### L5. `ghcr.io/dexidp/dex:v2.41.1` pin-date
**Spec ref**: §8.2.
**Evidence**: Today is 2026-04-17. `v2.41.1` dates from 2024. Unclear if this is intentional LTS pin or stale copy-paste from earlier brainstorming.
**Recommendation**: Confirm latest stable Dex tag at planning phase; pin with digest for reproducibility.

---

## Consistency Check vs Standards

| Standard | Compliance |
|---|---|
| `standards/architecture/layering.md` | **Fail** (C1, C2) |
| `standards/architecture/plugins.md` | Pass (suffix via package namespace) |
| `standards/architecture/observability.md` | Pass |
| `standards/architecture/domain-model.md` | Pass (source=push) |
| `standards/security/authentication.md` | Partial — H6 bcrypt cost conflict |
| `standards/security/authorization.md` | Pass (RequirePermission at registration) |
| `standards/security/data-handling.md` | Pass (json:"-", scrubbing documented) |
| `standards/backend/database-dialects.md` | Partial — H2, H3 (partial-index behavior, rotation ordering) |
| `standards/backend/go-conventions.md` | Pass |
| `standards/global/pr-checklist.md` | Partial — M7 (`docs/auth.md` missing) |

## Clarifications Requested

1. Is `internal/auth` permitted as a plugin import (ADR required)? If not, confirm approach from C1 recommendation.
2. Commit to one wiring approach: composition-root middleware wrapping, kernel-exposed typed middleware, or chi-router accessor. Only one survives arch-go.
3. Is API-key bcrypt cost negotiable (H6) or must MCP cache bcrypt results?
4. Migration test coverage: does CI add a Postgres path (currently SQLite only per codebase analysis design-context reference)? Partial index tested on which dialect(s)?
5. Principal-type enum: collapse to 2 values (spec) or split federated vs local (M3)?

---

## Conclusion

**Do not proceed to planning until C1 and C2 are resolved.** The arch-go violation and the hand-waving around middleware wiring are load-bearing — every downstream concrete (session store signatures, loopback, middleware chain, audit) depends on which resolution is chosen. Once C1/C2 are pinned, H1–H6 can be folded into the implementation plan as targeted tasks.

All other findings (M/L) are tractable within the current spec scope and do not block planning individually.

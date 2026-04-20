# ADR-015: MCP API-Key Credential Cache (credcache)

Date: 2026-04-17
Status: Accepted
Related: [ADR-005](005-authentication-and-session-management.md) (bcrypt cost 12 baseline), [ADR-003](003-microkernel-plugin-architecture.md) (plugin boundaries)

## Context

The MCP Discovery Server plugin (v1) exposes a read-only MCP endpoint at `/api/mcp`. Requests may authenticate with service-account API keys in the format `agentlens_sk_<client_id>.<secret>`. Secrets are stored as bcrypt hashes (cost factor 12, ~120–200ms per comparison on commodity hardware).

The tool-call flow has two sequential authentication points:

1. **MCP entry** — the composition-root middleware (`RequireAuthOrPrincipalDispatch`) validates the API key before dispatching to the MCP plugin handler.
2. **HTTP loopback** — the plugin re-issues a chi `ServeHTTP` call against the root router to delegate tool execution to existing catalog REST handlers. The loopback passes the original `Authorization` header, causing `RequireAuth` to re-run bcrypt validation.

With bcrypt cost 12, each comparison takes ~120–200ms. Two comparisons per tool call produces a floor of ~240–400ms — far above the p95 < 100ms performance SLO established in the product brief.

### Constraints

- bcrypt cost 12 is mandated by `standards/security/authentication.md` for *password* credentials. API-key secrets are a distinct asset class (machine-generated, high entropy, not memorised).
- The MCP plugin operates under strict arch-go layer isolation: `plugins/mcpserver/` may not import `internal/auth`. All credential resolution occurs in the composition root and `internal/auth/credcache/`.
- Revocation must take effect as quickly as practical — a delay window is acceptable only if explicitly documented.

## Decision

### credcache package

A new package `internal/auth/credcache/` provides an in-process, short-lived cache for successful API-key bcrypt verification results.

**Cache key**: `SHA-256(clientID + ":" + secret[:16])` — a fingerprint that does not store the secret itself but distinguishes distinct secrets for the same `clientID`.

**Cache value**: `*model.SessionPrincipalRef` (the resolved identity from a successful bcrypt comparison) plus `expiresAt time.Time`.

**TTL**: 10 seconds. After expiry the entry is removed lazily on next lookup; a background goroutine sweeps every 30 seconds.

**Capacity**: 1024 entries LRU. Eviction of the least-recently-used entry when full.

**Concurrency**: `sync.RWMutex`. Readers hold read lock only during key lookup + ref copy. Writers (Set, Invalidate) hold write lock. `Invalidate` does NOT cancel in-flight requests that have already completed a lookup and are executing downstream — documented acceptable staleness window ≤ max(TTL, longest in-flight request duration). In practice, any revoked credential that is already past the cache hit will complete its in-flight tool call and then fail on the next call once the TTL expires or the entry is invalidated.

**Metrics**:
- `agentlens.mcp.auth.credcache.hits.total` (Int64Counter)
- `agentlens.mcp.auth.credcache.misses.total` (Int64Counter)
- `agentlens.mcp.auth.credcache.evictions.total` (Int64Counter)

### bcrypt cost for API-key secrets

API-key secrets are 32-byte cryptographically random values (256-bit entropy), not human-chosen passwords. Offline brute-force is computationally infeasible regardless of cost factor. The security value of cost 12 for API keys accrues primarily at rest (if the database is exfiltrated) rather than from online rate protection. We retain cost 12 at rest and absorb the comparison cost into the credcache design — first comparison runs bcrypt; subsequent comparisons within the TTL window hit the cache.

This is explicitly a *different policy from the password-hashing rule in ADR-005*, which governs user passwords. The bcrypt cost selection for API-key secrets is documented here and does not require updating `standards/security/authentication.md`.

### Invalidation chain

The following events must call `credcache.Invalidate(clientID)`:

| Event | Code location |
|-------|--------------|
| Credential revoked via `DELETE /api/v1/service-accounts/{id}/credentials` | `internal/api/service_account_handlers.go` |
| Credential rotated via `PATCH /api/v1/service-accounts/{id}/secret` | `internal/store/api_client_credential_store.go:RotateSecret` (post-commit hook) |
| Service-account party deleted via `DELETE /api/v1/service-accounts/{id}` | `internal/api/service_account_handlers.go` — must enumerate `api_client_credentials WHERE party_id = ?` and call `credcache.Invalidate(clientID)` **per row before the DB cascade fires** (H6-residual from spec-audit-rev2.md) |

Failure to call `Invalidate` on party-delete leaves a staleness window ≤ TTL (10 seconds). The handler must iterate child credentials before issuing the DELETE to avoid the cascade removing the rows before enumeration.

### Staleness guarantees

After a revoke or rotate event, a previously-cached credential remains valid for at most `min(TTL_remaining, loopback_timeout)`. Under normal conditions this window is ≤ 10 seconds. This is acceptable for service-account credentials where:

- API keys are long-lived by design (no automatic expiry in v1).
- Revocation is an explicit operator action, not a time-based event.
- The 10-second window is documented and operators can mitigate by blocking at the network layer for immediate revocation requirements.

### Loopback handling

The loopback func (`api.BuildLoopbackFunc(chiRouter)`) passes the original request's `Authorization: Bearer agentlens_sk_...` header into the inner request. The inner `RequireAuth` middleware hits the credcache on the second pass (cache was populated on the MCP-entry pass). This produces a second bcrypt-free validation, contributing ~1ms overhead instead of ~150ms.

The inner request is constructed with `.WithContext(outerCtx)`, which carries `SessionPrincipalRef` + `ctxAccessibleProjectIDs`. Inner middleware is idempotent — re-reading the ctx values on the second pass yields the same authorisation result.

## Alternatives considered

**Internal short-lived JWT after first bcrypt** — After bcrypt success, the composition-root middleware could issue a signed short-lived JWT for the loopback call only. This avoids caching but adds key-management complexity (signing key lifecycle) and is harder to reason about in audit logs. Rejected: credcache is simpler and the TTL semantics are easier to document.

**Lower bcrypt cost to 10 for API keys** — Would reduce comparison time from ~150ms to ~25ms. Rejected for v1: the credcache approach keeps cost 12 at rest, which is the stronger security posture, without requiring a new config knob or standards amendment.

**Skip loopback, replicate catalog query directly in tools** — Would avoid the double-auth entirely. Rejected: would duplicate store query logic and bypass project-scoping middleware (`ScopeByAccessibleProjects`), creating a divergence risk between tool results and REST catalog results.

## Consequences

- `internal/auth/credcache/` is a new package at the infrastructure layer; it may be imported by the composition root and by the auth pipeline.
- The MCP plugin never imports `credcache` directly (arch-go boundary preserved).
- Operator documentation must describe the ≤ 10s staleness window for revoked API keys.
- The ADR and staleness window must be referenced in `docs/auth.md` under "Service-account credentials".
- `make test` covers: cache hit/miss, LRU eviction, invalidation-before-cascade ordering.

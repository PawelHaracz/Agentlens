# Spec Audit: MCP Discovery Server v1 — Revision 2

**Auditor**: spec-auditor (independent)
**Date**: 2026-04-17
**Spec**: `implementation/spec.md` (revision 2)
**Prior audit**: `verification/spec-audit.md` (rev 1 — fail)
**Verdict**: **pass with conditions**

## Verdict Summary

| Severity | Rev 1 | Rev 2 | Delta |
|---|---|---|---|
| Critical | 2 | 0 | −2 |
| High | 6 | 1 | −5 |
| Medium | 7 | 3 | −4 |
| Low | 5 | 2 | −3 |

Planning may proceed. Remaining findings tractable during planning/implementation.

---

## Resolution Verification (rev-1 blockers)

- **C1 — RESOLVED**. `model.SessionPrincipalRef` in internal/model (foundation). Plugin imports: kernel + model + config + store + telemetry only. Zero `internal/auth` / `internal/api`. arch-go rule satisfied.
- **C2 — RESOLVED**. `Kernel.Router()` accessor dropped. §8.1 gives concrete 9-step composition-root ordering: `pm.InitAll → build middleware → BuildLoopbackFunc(chiRouter) → mcpPlugin.SetLoopback → wrap handler → kernel.RegisterRoutes("/api/mcp", wrapped) → pm.StartAll`. Shutdown LIFO documented.
- **H1 — RESOLVED**. Migration `Description` field (not Name).
- **H2 — RESOLVED**. Raw `tx.Exec` dual-dialect partial-index DDL; migration test asserts presence via `sqlite_master` / `pg_indexes`.
- **H3 — RESOLVED**. UPDATE-then-INSERT rotation ordering pinned; concurrent-rotation test mandated.
- **H4 — RESOLVED**. Origin middleware at composition root, scoped to `/api/mcp` only.
- **H5 — RESOLVED**. Bounded channel 1024 + 30s tick + flush-on-Stop + drop-metric.
- **H6 — RESOLVED with ADR deferral**. `internal/auth/credcache/` 10s TTL LRU 1024; invalidate on rotate/revoke; bcrypt cost 12 retained; rate-limit 30 fails/60s → 429. ADR authored at planning.

All medium + low issues (M1-M7, L1-L5) resolved per previous revision.

---

## Remaining Findings

### High

**H6-residual — credcache invalidation on party-delete cascade is underspecified.**
FK `party_id ON DELETE CASCADE` removes DB rows, but in-memory `credcache` keyed by `clientID` retains entries up to 10s. During that window, a leaked secret of a just-deleted SA still authenticates.
- **Recommendation**: Add to §1.6 or §3.3: "On service-account party deletion, handler enumerates `api_client_credentials WHERE party_id = ?` and calls `credcache.Invalidate(client_id)` per row **before** DB cascade." User-delete for `user_local`/`user_federated` sessions is covered by 60s orphan reaper (§1.9) — acceptable.

### Medium

**M-new-1 — `BuildLoopbackFunc` ctx propagation underspecified.** Loopback must pass outer request's ctx (`ctxSessionPrincipalRef` + `ctxAccessibleProjectIDs`) into `httptest.NewRequest(...).WithContext(ctx)`. If fresh background ctx, `ScopeByAccessibleProjects` sees empty AccessibleProjectIDs — project scoping silently opens up.
- **Recommendation**: Add to §5.8: "`BuildLoopbackFunc` creates request with `.WithContext(outerCtx)`. Inner middleware is idempotent." Add test in Group E asserting user-supplied `?projects=` in LLM tool args cannot reach filter.

**M-new-2 — Rotation `409 Conflict` mapping dialect-dependent.** GORM surfaces SQLite `UNIQUE constraint failed` and PG `23505` as different driver errors. String-match on `err.Error()` violates go-conventions.
- **Recommendation**: Use `errors.Is(err, gorm.ErrDuplicatedKey)`.

**M-new-3 — credcache LRU eviction + TTL + invalidation race.** In-flight request past Lookup cannot be cancelled by concurrent Invalidate. For API keys the documented ADR tolerates 10s lingering — should be explicit.
- **Recommendation**: "credcache uses `sync.RWMutex`; Invalidate holds write-lock but does not cancel in-flight requests already past Lookup. Documented staleness window = ≤ max(TTL, longest in-flight request)."

### Low

**L-new-1 — PRM handler 404-when-disabled is wasteful.** Cleaner to conditionally register only when `cfg.Federation.Provider != ""`. §8.1 pseudo-code unconditionally passes provider to PRM handler; nil-check needed.

**L-new-2 — Dex digest + go-oidc/go-jose versions unpinned.** Planner-deferred; flagging to ensure capture.

---

## Standards Consistency (rev 2)

| Standard | Rev 1 | Rev 2 |
|---|---|---|
| layering.md | Fail | **Pass** |
| plugins.md | Pass | Pass |
| observability.md | Pass | Pass |
| domain-model.md | Pass | Pass |
| security/authentication.md | Partial | Pass (H6 via ADR) |
| security/authorization.md | Pass | Pass |
| security/data-handling.md | Pass | Pass |
| backend/database-dialects.md | Partial | **Pass** |
| backend/go-conventions.md | Pass | Partial (M-new-2) |
| global/pr-checklist.md | Partial | **Pass** |

No regression in previously-passing areas.

---

## Conclusion

Revision 2 resolves both rev-1 critical blockers (C1, C2) with concrete, verifiable mechanisms. All five rev-1 high items are fully or ADR-deferred resolved. One new high (H6-residual: credcache cascade ordering) and three mediums emerge from the tighter specification but are all tractable during planning/implementation.

**Recommended action**: Proceed to planner phase. Planner should address H6-residual, M-new-1, M-new-2 as concrete tasks (one sentence each in the plan) plus pick up the spec-deferred ADR, library pins, and Dex digest as pre-implementation chores.

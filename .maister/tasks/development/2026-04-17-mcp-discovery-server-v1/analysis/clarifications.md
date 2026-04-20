# Phase 1 Clarifications

Resolved 2026-04-17.

## 1. Kernel router wiring

**Decision**: Add `Kernel.Router() chi.Router` accessor.

Plugin obtains root chi router via `k.Router()` and calls `.Route("/api/mcp", ...)` in `Init()`. Accepts the trade-off that `chi.Router` type leaks into the kernel interface; simplest path to close the current plugin-HTTP wiring gap. Plugins can continue to use `k.RegisterRoutes` for the legacy path; new plugins prefer direct router access.

## 2. JWKS client library

**Decision**: `coreos/go-oidc` + `go-jose`.

Mature OIDC primitives (discovery, JWKS refresh, claim verification). Standard choice for Go OIDC integrations.

## 3. Migration version

**Decision**: Use **migration 010** (next free; codebase highest is 9).

Spec-referenced "migration 008" is stale relative to current codebase. Implementation updates all references to 010. Single-dialect assumption already covered by existing migration pattern (SQLite + PostgreSQL branching).

## 4. PR delivery strategy

**Decision**: **Single large feature-branch PR** off `feat/mcp-discovery-server-v1`.

Entire v1 (all 8 groups A–H) lands together. Accepts review/rollback cost in exchange for cross-cutting consistency (migration + model + plugin + router + middleware + UI must agree). Plugin ships behind `mcp_server.enabled=false` default so main stays green even if runtime issues surface post-merge.

## 5. Dex testing strategy (unresolved — defer to spec phase)

Handler/middleware unit tests should use a JWKS stub (`httptest.NewServer` serving static JWKS JSON). Whether E2E adds a real Dex container (full federation flow) or stays on the stub is an open question for the spec/spec-audit phase. Flagged for gap-analyzer and spec-auditor to resolve.

# Phase 2 Scope Clarifications

Resolved 2026-04-17.

## Critical decisions

### 1. Dex E2E testing strategy

**Decision**: **Hybrid** — JWKS `httptest` stub for unit/integration tests + one real-Dex container E2E path to verify DCR/issuer/aud spec compliance.

Balances fidelity (catches Dex DCR regressions flagged in design-decisions 3F) with CI speed. E2E gate for this single path can run on a dedicated workflow job.

### 2. MCP session store location

**Decision**: **New `mcp_sessions` DB table** (overrides gap-analyzer's in-memory recommendation).

Persists sessions across restarts. Added to migration 010 (alongside `api_client_credentials`). Requires dual-dialect support. Accepts extra migration scope + write amplification in exchange for: (a) restart-safe session continuity for long-running Claude.ai Custom Connector flows, (b) operator visibility via DB inspection, (c) session revocation via DELETE.

### 3. Admin UI (Group G) in v1 PR

**Decision**: **Ship in v1 PR**.

`ServiceAccountsPage`, `ServiceAccountDetailPage`, `PendingIdentitiesPage` land together with backend. Priya's operator journey is complete on merge. No orphaned REST CRUD.

## Important housekeeping (approved defaults)

### 4. Helm chart version: **0.2.0 → 0.3.0**

Minor-version bump per SemVer for new optional Dex subchart + new values keys.

### 5. Include `docker-compose.dev.yml` with Dex: **yes**

Ships local-dev compose file alongside feature. Needed for hybrid E2E path + Karol's local paste-URL flow.

### 6. arch-go.yml naming rules for MCP subpackages: **defer**

Only one plugin adopts the new `plugins/mcpserver/` layout. Per CLAUDE.md, codify naming rules only after 3+ instances establish a pattern.

### 7. Scoped CORS on `/api/mcp`: **plugin-registered per-route middleware**

Plugin owns Origin enforcement on its routes; global `CORSMiddleware` (`*`) untouched. Zero risk of REST/UI regression.

### 8. Spec migration renumber 008 → 010

Implementation must update all spec references during coding. Tracked for work-log.

## Carry-overs into specification (Phase 5)

- Session store schema (`mcp_sessions`: id, principal_id, created_at, last_seen_at, expires_at, protocol_version).
- Dex E2E CI job structure (separate workflow file vs gated step in existing E2E job).
- OriginValidationMiddleware shape (allowed origins config key or wildcard + request-method gating).

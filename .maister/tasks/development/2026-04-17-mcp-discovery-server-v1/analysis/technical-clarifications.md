# Phase 5 Technical Clarifications

Resolved 2026-04-17.

## 1. mcp_sessions revocation semantic

**Decision**: **Soft-delete with `revoked_at` column**.

Schema (migration 010):
```sql
CREATE TABLE mcp_sessions (
  id                TEXT PRIMARY KEY,
  principal_id      TEXT NOT NULL,
  principal_type    TEXT NOT NULL CHECK(principal_type IN ('service_account','user')),
  protocol_version  TEXT NOT NULL,
  created_at        {DATETIME|TIMESTAMPTZ} NOT NULL,
  last_seen_at      {DATETIME|TIMESTAMPTZ} NOT NULL,
  expires_at        {DATETIME|TIMESTAMPTZ} NOT NULL,
  revoked_at        {DATETIME|TIMESTAMPTZ} NULL
);
CREATE INDEX idx_mcp_sessions_principal ON mcp_sessions(principal_id, principal_type);
CREATE INDEX idx_mcp_sessions_active    ON mcp_sessions(expires_at) WHERE revoked_at IS NULL; -- partial on PG; full on SQLite
```

Rationale: audit trail preserved, idempotent revoke, enables "list revoked sessions" operator view, avoids UUID re-issue collisions.

## 2. `/api/mcp` Origin enforcement

**Decision**: **Configurable allowlist via `mcp_server.allowed_origins`**.

Config field:
```go
type MCPServerConfig struct {
    ...
    AllowedOrigins []string `yaml:"allowed_origins"`
}
```

Env var: `AGENTLENS_MCP_ALLOWED_ORIGINS` (comma-separated).

Semantics:
- Empty allowlist + missing `Origin` header → 403 (spec-compliant strict default)
- Empty allowlist + present `Origin` → 403 (strict posture; operator must explicitly allow origins)
- Non-empty allowlist + `Origin` in list → pass
- Non-empty allowlist + `Origin` not in list → 403

Documented in `docs/settings.md` and `docs/auth.md`. Operator sets e.g. `["https://claude.ai", "https://cursor.sh"]`.

## 3. `api_client_credentials` secret rotation

**Decision**: **One active secret per service-account, revoke-then-issue rotation**.

Schema (migration 010):
```sql
CREATE TABLE api_client_credentials (
  id            TEXT PRIMARY KEY,
  party_id      TEXT NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
  client_id     TEXT NOT NULL UNIQUE,      -- public identifier shown in secret format
  secret_hash   TEXT NOT NULL,              -- bcrypt cost 12
  scopes        TEXT NOT NULL DEFAULT '',
  created_at    {DATETIME|TIMESTAMPTZ} NOT NULL,
  last_used_at  {DATETIME|TIMESTAMPTZ} NULL,
  expires_at    {DATETIME|TIMESTAMPTZ} NULL,
  revoked_at    {DATETIME|TIMESTAMPTZ} NULL
);
CREATE UNIQUE INDEX idx_acc_active_per_party
  ON api_client_credentials(party_id) WHERE revoked_at IS NULL; -- partial
-- SQLite fallback: enforce uniqueness in application layer if partial index unsupported
```

Admin flow: `PATCH /api/v1/service-accounts/{id}/secret` = revoke current (set `revoked_at`) + insert new row in one transaction. Response returns new `agentlens_sk_<id>.<secret>` once. Brief overlap gap (old secret invalidated immediately) acceptable for v1; documented as known operator consideration.

Rotation-with-overlap deferred to v1.5 per product brief's non-goal list.

## Cross-cutting

- All new timestamp columns use `DATETIME` (SQLite) / `TIMESTAMPTZ` (PostgreSQL) per backend standards.
- Partial indexes: PostgreSQL gets proper `WHERE revoked_at IS NULL`; SQLite uses same syntax (supported since SQLite 3.8) — verified in migration 009 patterns.
- Secret format remains `agentlens_sk_<client_id>.<raw_secret>` (bcrypt-hashed at rest) per feature-spec §1.

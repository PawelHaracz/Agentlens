# Specification: MCP Discovery Server v1

Revision 2 — addresses spec-audit C1, C2, H1–H6, M1–M7, L1–L5. Implementation-ready. See §8.x for composition-root wiring that replaces the previously-proposed `Kernel.Router()` accessor.

Linked artifacts (read, do not duplicate):
- `analysis/requirements.md` — gathered requirements + reusability index
- `analysis/design-context/codebase-analysis.md` — structural findings
- `analysis/clarifications.md` — Phase 1 decisions
- `analysis/scope-clarifications.md` — Phase 2 decisions
- `analysis/technical-clarifications.md` — Phase 5 decisions
- `analysis/gap-analysis.md` — current vs. target state
- `analysis/design-context/feature-spec.md` — primary content source (§1–§8)
- `analysis/design-context/design-decisions.md` — selected alternatives (1C+2A+3F+4B+5A+6D)
- `analysis/design-context/problem-statement.md` — scope + constraints
- `analysis/design-context/personas.md` — Anya / Karol / Priya
- `verification/spec-audit.md` — audit report driving this revision

---

## Goal

Ship a read-only MCP Discovery Server as an in-process AgentLens plugin that exposes the catalog through 4 discovery tools (`agent_search`, `agent_get`, `capabilities_list`, `agent_card`) to LLM clients, authenticated via service-account API keys or OAuth 2.1 federation (bundled Dex), with full audit + OTel observability and zero-glue onboarding for three personas (backend dev, IDE dev, platform operator).

## User Stories

- As **Anya** (backend LLM app dev) I want to configure one env var (`AGENTLENS_API_KEY=agentlens_sk_<id>.<secret>`) and query AgentLens agents from my LLM app in <100ms p95, so I don't write glue code.
- As **Karol** (IDE / Claude.ai dev) I want to paste my AgentLens MCP URL into a Custom Connector, complete OAuth via Dex, and see the 4 tools in <5 minutes, so I can discover agents interactively.
- As **Priya** (platform operator) I want to create, rotate, and revoke service accounts; approve new federated identities; and audit every tool call per-principal, so I can run this for a team safely.

## Core Requirements

1. Read-only MCP server embedded as a plugin at `plugins/mcpserver/`, modeled on `plugins/health/`.
2. Four discovery tools (`agent_search`, `agent_get`, `capabilities_list`, `agent_card`) behind a `ToolRegistry` abstraction v2-translator-compatible.
3. Streamable HTTP transport per MCP 2025-11-25 (POST + GET on `/api/mcp`).
4. Dual authentication:
   - Service-account API keys with format `agentlens_sk_<client_id>.<secret>` (bcrypt cost 12 at rest; 10-second result cache for p95 budget — see §3.3).
   - OAuth 2.1 via bundled Dex federation (PKCE S256, audience binding, JWKS cache with rate-limited refresh).
5. `SessionPrincipalRef` abstraction in `internal/model` (foundation layer) normalizing 3 auth paths. Real `auth.Principal` resolution lives in composition-root middleware; only the opaque ref crosses into the plugin via `context.Context` — preserves arch-go layering.
6. New `parties.kind='service_account'` + `api_client_credentials` + `mcp_sessions` tables (migration 010, dual-dialect; explicit raw `tx.Exec` DDL for partial indexes).
7. `user_external_identities` table for Dex-sub → AgentLens-user mapping with admin-approval queue (JIT default off).
8. **Composition-root handler wrapping** (replaces proposed `Kernel.Router()` accessor): `cmd/agentlens/main.go` wraps the plugin's raw `http.Handler` with the REST middleware chain (Origin → Auth dispatch → ScopeByAccessibleProjects) and calls `kernel.RegisterRoutes("/api/mcp", wrappedHandler)`. Plugin imports only `kernel` + `foundation` — never `internal/api` or `internal/auth`.
9. Self-registration into catalog at `Init()` (idempotent via `AgentKey = SHA256(mcp+agentlens:mcp-discovery:{canonical_public_url})` — disambiguates multi-instance deployments).
10. `/.well-known/oauth-protected-resource` registered as a separate unauthenticated `http.Handler` at root (pre-`/api/v1`).
11. DB-backed `mcp_sessions` with soft-delete (`revoked_at`), `initialized_at` column, TTL reaper + orphan-principal reaper (60s tick).
12. Composition-root-attached scoped Origin middleware on `/api/mcp` (global `*` CORS untouched); configurable allowlist via `mcp_server.allowed_origins` (strict-default: empty list = all 403).
13. One-active-secret rotation model: `PATCH /api/v1/service-accounts/{id}/secret` — single transaction with explicit **UPDATE-then-INSERT** ordering (partial unique index enforces invariant; deferred-constraint dialects not required).
14. Three new permissions (`service_accounts:read|write|delete`) seeded on `admin` role in migration 010.
15. Admin UI Group G in v1 PR: `ServiceAccountsPage`, `ServiceAccountDetailPage`, `PendingIdentitiesPage` (shadcn/ui, Vitest coverage 80/80/75/80).
16. OTel spans (`agentlens.mcp.*`), metrics (`agentlens_mcp_*`), scrubbed audit log per tool invocation. Startup WARN when `mcp_server.audit_enabled=false`.
17. Federation health loop + `/readyz` chain (DB ping + Dex JWKS reachability).
18. Helm chart `0.2.0 → 0.3.0` with Dex as conditional subchart (`condition: dex.enabled`), Dex image pinned by digest.
19. `docker-compose.dev.yml` with Dex service for local dev + hybrid E2E path.
20. Single large feature-branch PR off `feat/mcp-discovery-server-v1`, `mcp_server.enabled=false` default.

## Visual Design

No mockups provided. Admin UI mirrors `web/src/pages/SettingsPage.tsx` conventions:
- shadcn/ui primitives (Tabs, Table, Dialog, Card, Badge, Button, Input, Select, Alert).
- TanStack React Query for data; route layout under `web/src/routes/admin/`.
- One-time secret display pattern: modal with copy-to-clipboard + explicit "I've saved this" gate; secret scrubbed from state on dismiss.
- `data-testid` attributes on interactive elements for Playwright screenshot capture (`docs/images/`).
- Icons: lucide-react (match existing usage).

---

## §1. Data Model & Migrations

### 1.1 `parties.kind` enum expansion

Add `PartyKindServiceAccount PartyKind = "service_account"` to `internal/model/party.go`. No schema change (column is TEXT/VARCHAR). Code-level touchpoints:
- `internal/api/party_handlers.go` — `RegisterPartyKindRoutes("service_accounts", ...)`.
- `internal/store/party_store.go` — `CreateServiceAccount(ctx, name) (*model.Party, error)`.
- `internal/auth/party_permissions.go` — reuse existing `project:viewer|developer|owner` roles.

### 1.2 `api_client_credentials` (new)

GORM-declared columns via AutoMigrate. Partial unique index created explicitly via raw `tx.Exec` per dialect (see §1.5) — GORM tag generation is not relied upon for partial indexes.

Column spec (dialect-branched types resolved by migration):

| Column | SQLite | PostgreSQL | Notes |
|---|---|---|---|
| `id` | TEXT PRIMARY KEY | TEXT PRIMARY KEY | UUID v4 |
| `party_id` | TEXT NOT NULL | TEXT NOT NULL | FK → parties.id ON DELETE CASCADE |
| `client_id` | TEXT NOT NULL UNIQUE | TEXT NOT NULL UNIQUE | public; shown in secret format |
| `secret_hash` | TEXT NOT NULL | TEXT NOT NULL | bcrypt cost 12 |
| `label` | TEXT NOT NULL | TEXT NOT NULL | |
| `scopes` | TEXT NOT NULL DEFAULT '' | TEXT NOT NULL DEFAULT '' | |
| `created_at` | DATETIME NOT NULL | TIMESTAMPTZ NOT NULL | |
| `created_by_user_id` | TEXT NULL | TEXT NULL | FK → users.id ON DELETE SET NULL |
| `last_used_at` | DATETIME NULL | TIMESTAMPTZ NULL | |
| `expires_at` | DATETIME NULL | TIMESTAMPTZ NULL | |
| `revoked_at` | DATETIME NULL | TIMESTAMPTZ NULL | |

Go model in `internal/model/api_client_credential.go`. `SecretHash` tagged `json:"-"` and `gorm:"type:text"` per `standards/security/data-handling.md`. Plaintext secret shown once at creation, never persisted.

### 1.3 `user_external_identities` (new)

GORM-declared. Post-AutoMigrate raw-DDL indexes (§1.5).

| Column | Type (SQLite / PG) | Notes |
|---|---|---|
| `id` | TEXT PRIMARY KEY | |
| `user_id` | TEXT NOT NULL | FK → users.id ON DELETE CASCADE |
| `provider` | TEXT NOT NULL | "dex" for v1 |
| `external_sub` | TEXT NOT NULL | |
| `external_iss` | TEXT NOT NULL | |
| `created_at` | DATETIME / TIMESTAMPTZ NOT NULL | |
| `last_seen_at` | DATETIME / TIMESTAMPTZ NULL | |

Composite unique: `(provider, external_iss, external_sub)`.

### 1.4 `mcp_sessions` (new — DB-backed)

| Column | Type (SQLite / PG) | Notes |
|---|---|---|
| `id` | TEXT PRIMARY KEY | |
| `principal_id` | TEXT NOT NULL | No FK (points to users or parties; resolved via reaper — see §1.9 L2) |
| `principal_type` | TEXT NOT NULL | CHECK IN ('user_local','user_federated','service_account') — M3 resolution |
| `protocol_version` | TEXT NOT NULL | |
| `created_at` | DATETIME / TIMESTAMPTZ NOT NULL | |
| `last_seen_at` | DATETIME / TIMESTAMPTZ NOT NULL | |
| `expires_at` | DATETIME / TIMESTAMPTZ NOT NULL | |
| `initialized_at` | DATETIME / TIMESTAMPTZ NULL | L4 — set on `notifications/initialized`; reconnect checks |
| `revoked_at` | DATETIME / TIMESTAMPTZ NULL | |

**`auth_method` column dropped** — `principal_type` now encodes it. Audit log still emits `auth_method` per-request (§3.9, §7.4).

Rationale: restart-safe sessions for long-running Claude.ai Custom Connector flows, operator DB inspection, soft-delete revocation, audit trail, richer forensics.

### 1.5 Migration 010 (idempotent, dual-dialect, forward-only)

Appended to `internal/db/migrations.go` `AllMigrations()`:

```go
{Version: 10, Description: "mcp_discovery_v1", Up: func(tx *gorm.DB) error { ... }}
```

**H1 resolution**: the existing struct field is `Description` (not `Name`).

Migration body order:
1. `tx.AutoMigrate(&model.APIClientCredential{}, &model.UserExternalIdentity{}, &model.MCPSession{})` — columns only; no partial indexes from GORM tags.
2. Dialect-branched raw `tx.Exec` DDL (H2 resolution — explicit text):

```go
switch tx.Dialector.Name() {
case "sqlite":
    tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_acc_active_per_party
             ON api_client_credentials(party_id) WHERE revoked_at IS NULL`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_acc_active
             ON api_client_credentials(revoked_at, expires_at)`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_ext_ident_user
             ON user_external_identities(user_id)`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_sessions_principal
             ON mcp_sessions(principal_id, principal_type)`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_sessions_active
             ON mcp_sessions(expires_at) WHERE revoked_at IS NULL`)
case "postgres":
    tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_acc_active_per_party
             ON api_client_credentials(party_id) WHERE revoked_at IS NULL`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_acc_active
             ON api_client_credentials(revoked_at, expires_at)`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_ext_ident_user
             ON user_external_identities(user_id)`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_sessions_principal
             ON mcp_sessions(principal_id, principal_type)`)
    tx.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_sessions_active
             ON mcp_sessions(expires_at) WHERE revoked_at IS NULL`)
}
```

3. Permission seed: `INSERT … WHERE NOT EXISTS` for `service_accounts:read|write|delete` into `permissions` table + grant to `admin` system role (pattern from migration007).
4. `developer` system role (if seeded) gets `service_accounts:read`.

**Migration test assertion (H2)**: post-migration, query `sqlite_master` (SQLite) / `pg_indexes` (PostgreSQL) to assert presence of `idx_acc_active_per_party` with `WHERE revoked_at IS NULL` clause. Reject implementation if DDL silently no-ops.

Forward-only per `standards/backend/database-dialects.md`. No Down function. Idempotent re-run verified (existing migration009 pattern).

### 1.6 Secret lifecycle

1. **Create**: admin `POST /api/v1/service-accounts/{id}/keys {label, scopes?}`. Server generates 32-byte random secret, bcrypt(cost=12), stores hash. Response **once** returns full `agentlens_sk_<client_id>.<secret>`.
2. **Use**: client sends `Authorization: Bearer agentlens_sk_<client_id>.<secret>`. Server splits on first `.`, looks up by `client_id`, bcrypt-compares (dummy hash on miss for timing safety; short-TTL cache on hit per §3.3), checks `revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`, schedules async `last_used_at` update via bounded channel (H5; see §1.8).
3. **Rotate** (H3 resolution): `PATCH /api/v1/service-accounts/{id}/secret` — single transaction with **strict ordering**:
   - **Step A**: `UPDATE api_client_credentials SET revoked_at = now() WHERE id = <current_active> AND revoked_at IS NULL`.
   - **Step B**: `INSERT` new row with `revoked_at = NULL`.
   - `credcache.Invalidate(client_id)` called post-commit (§3.3).
   - Partial unique index `idx_acc_active_per_party` enforces invariant atomically inside the transaction because UPDATE fires before INSERT — the new row never coexists with a non-revoked prior row.
   - Documented: brief gap where old secret is invalid before new is distributed is an operator consideration.
   - Concurrent-rotation test: two rotation calls in flight; one wins, other receives `409 Conflict` from unique-index violation.
4. **Revoke**: `DELETE /api/v1/service-accounts/{id}/keys/{keyID}` → sets `revoked_at = now()` + calls `credcache.Invalidate(client_id)`.

### 1.7 JIT provisioning

Default: **admin-approval-queue** (`federation.common.auto_provision_users=false`). Successful Dex login with no matching `user_external_identities` row → `403 Forbidden` + audit entry for admin review. Admin UI Pending Identities page links to existing user OR creates + links in one transaction. When `auto_provision_users=true`, first-login creates user + mapping atomically with `default_role_id`.

### 1.8 Bounded async update channels (H5)

Both `api_client_credentials.last_used_at` and `user_external_identities.last_seen_at` share the same pattern:

- Bounded buffered channel, **size 1024**.
- Dedicated goroutine drains and flushes batched UPDATEs on a **30-second tick**.
- On `Plugin.Stop()`: close input channel, drain pending updates synchronously before return (flush-on-shutdown).
- **Drop-policy**: when channel full, drop the update + increment `agentlens.mcp.credcache.dropped_updates.total` (for API keys) / `agentlens.mcp.federation.dropped_last_seen.total` (for external identities).
- Documented as best-effort forensic signal — if accurate `last_used_at` is mission-critical, operator must restart with larger buffer (exposed via future config — not v1).

### 1.9 Session reaper (L2)

Background goroutine started in `Plugin.Start(ctx)`:
- Scans `mcp_sessions` every 60s.
- **TTL reaper**: rows with `expires_at < now() AND revoked_at IS NULL` → set `revoked_at = now()`.
- **Orphan-principal reaper**: for each active row, verify `principal_id` resolves (users table for `user_local`/`user_federated`, parties table for `service_account`). Unresolved → `revoked_at = now()` + `slog.InfoContext(ctx, "mcp.session.reaper.orphan_revoked", ...)` audit line + `agentlens.mcp.sessions.orphan_revoked.total` metric.

### 1.10 Backward compatibility

Existing users, parties, projects, catalog entries, JWT flows, JSON shapes unchanged. Migration additive only.

---

## §2. Configuration

### 2.1 Typed config structs

```go
// internal/config/config.go
type MCPServerConfig struct {
    Enabled         bool          `yaml:"enabled"`          // default false
    ListenPath      string        `yaml:"listen_path"`      // default "/api/mcp"
    ProtocolVersion string        `yaml:"protocol_version"` // default "2025-11-25"
    PublicURL       string        `yaml:"public_url"`       // canonical public URL; required when Enabled
    AllowedOrigins  []string      `yaml:"allowed_origins"`  // empty = 403 (strict default)
    MaxSessions     int           `yaml:"max_sessions"`     // default 1000
    SessionTTL      time.Duration `yaml:"session_ttl"`      // default 4h
    RequestTimeout  time.Duration `yaml:"request_timeout"`  // default 30s
    AuditEnabled    bool          `yaml:"audit_enabled"`    // default true; startup WARN when false (L3)
}

type FederationProvider string
const (
    FederationProviderNone FederationProvider = ""
    FederationProviderDex  FederationProvider = "dex"
    FederationProviderOIDC FederationProvider = "oidc"
)

type FederationConfig struct {
    Provider FederationProvider       `yaml:"provider"`
    Common   FederationCommonConfig   `yaml:"common"`
    Instance FederationInstanceConfig `yaml:"instance"`
}
```

`FederationCommonConfig` fields: `UserIDClaim`, `EmailClaim`, `GroupsClaim`, `RoleMapping`, `AutoProvisionUsers`, `DefaultRoleID`, `AudiencePrefix`, `JWKSCacheTTL` (1h), `JWKSMinRefreshInterval` (10s — M2), `HealthCheckInterval` (30s), `HealthCheckTimeout` (5s).

`FederationInstanceConfig` fields: `IssuerURL`, `JWKSURL?`, `Options map[string]string`.

Per `standards/architecture/layering.md`, `config` package has no interfaces. Provider factory registry lives in `internal/auth/federation/`.

### 2.2 Provider registry

```go
// internal/auth/federation/registry.go
type FactoryFunc func(instance InstanceConfig, common CommonConfig) (Provider, error)
func RegisterProvider(kind config.FederationProvider, f FactoryFunc)
func BuildProvider(kind config.FederationProvider, instance, common) (Provider, error)
```

Dex provider self-registers in `internal/auth/federation/dex/dex.go` via `init()`. **Federation `Provider` interface is never imported by the plugin** (C1 resolution). `cmd/agentlens/main.go` owns provider lifecycle (build → Init → inject into middleware → Stop after `pm.StopAll`).

### 2.3 Environment variable overrides

| Env var | Maps to |
|---|---|
| `AGENTLENS_MCP_SERVER_ENABLED` / `_LISTEN_PATH` / `_PUBLIC_URL` / `_PROTOCOL_VERSION` / `_MAX_SESSIONS` / `_SESSION_TTL` / `_REQUEST_TIMEOUT` / `_AUDIT_ENABLED` | `mcp_server.*` |
| `AGENTLENS_MCP_ALLOWED_ORIGINS` (comma-separated) | `mcp_server.allowed_origins` |
| `AGENTLENS_FEDERATION_PROVIDER` | `federation.provider` |
| `AGENTLENS_FEDERATION_INSTANCE_ISSUER_URL` / `_JWKS_URL` / `_OPTIONS` (JSON map) | `federation.instance.*` |
| `AGENTLENS_FEDERATION_COMMON_*` | `federation.common.*` |
| `AGENTLENS_FEDERATION_COMMON_ROLE_MAPPING` (JSON) | `federation.common.role_mapping` |

New helpers: `applyMCPServerEnv`, `applyFederationEnv`.

### 2.4 Validation (fail-fast in `Load()`)

- `mcp_server.enabled=true` → `public_url` required, must be `https://` (or localhost); `max_sessions >= 1`, `session_ttl >= 1m`.
- `federation.provider != ""` → `instance.issuer_url` + `common.audience_prefix` required.
- `auto_provision_users=true` → `default_role_id` required.
- Fails startup with clear message; prevents half-configured auth.

### 2.5 Sample `agentlens.yaml`

Shown in `docs/settings.md`. Example with Dex + MCP enabled; example with generic OIDC; example with federation off + service-account-only mode.

### 2.6 Audit-disabled startup warning (L3)

When `mcp_server.audit_enabled == false` and `mcp_server.enabled == true`, `cmd/agentlens/main.go` emits at startup:

```
slog.WarnContext(ctx, "MCP audit logging disabled — forensic trail unavailable",
    "component", "mcpserver")
```

---

## §3. Authentication Flows

### 3.1 `SessionPrincipalRef` (foundation layer — C1 resolution)

```go
// internal/model/session_principal_ref.go
type SessionPrincipalKind string
const (
    SessionPrincipalKindUserLocal      SessionPrincipalKind = "user_local"
    SessionPrincipalKindUserFederated  SessionPrincipalKind = "user_federated"
    SessionPrincipalKindServiceAccount SessionPrincipalKind = "service_account"
)

type SessionPrincipalRef struct {
    ID                   string
    Kind                 SessionPrincipalKind
    PartyID              string
    Permissions          []string
    AccessibleProjectIDs []string
    AuthMethod           string   // "basic_jwt" | "federation:dex" | "api_key"
}
```

Plugin-facing. The plugin never sees `auth.Principal`. Middleware in composition root resolves the full `auth.Principal` (which may hold JWT claims, credential ID, refresh handles) and then builds the opaque ref + attaches to `r.Context()` via `ctxSessionPrincipalRef`.

Existing context keys (`ctxUserID`, `ctxUsername`, `ctxRoleID`, `ctxPermissions`) preserved for existing REST handlers.

### 3.2 `auth.Principal` (unchanged, internal/auth only)

```go
// internal/auth/principal.go (consumers: middleware, REST handlers, audit — never plugin)
type Principal struct {
    Kind                 model.SessionPrincipalKind
    ID, PartyID          string
    Username, Label      string
    Permissions          []string
    AccessibleProjectIDs []string
    AuthMethod           string
    CredentialID         string
    IssuedAt, ExpiresAt  time.Time
}

func (p *Principal) ToSessionRef() *model.SessionPrincipalRef { ... }
```

### 3.3 Service-account API-key flow (with short-TTL bcrypt cache — H6)

New package `internal/auth/credcache/`:

```go
type Entry struct {
    Ref       *model.SessionPrincipalRef
    ExpiresAt time.Time
}

type Cache interface {
    Lookup(clientID, secretFingerprint string) (*Entry, bool)
    Store(clientID, secretFingerprint string, ref *model.SessionPrincipalRef)
    Invalidate(clientID string)
}
```

- Backing: LRU, max 1024 entries.
- Key: `clientID + sha256(secret)[:16]` (full sha256, truncated — do not store secret itself).
- TTL: **10 seconds**.
- Hit → skip bcrypt entirely; use cached ref.
- Miss / expired → bcrypt compare; on success write cache.
- Invalidation hooks: rotate (§1.6.3), revoke (§1.6.4), party-deletion cascade.
- Metrics: `agentlens.mcp.auth.credcache.hits.total`, `agentlens.mcp.auth.credcache.misses.total`, `agentlens.mcp.auth.credcache.invalidations.total`.

**ADR-00X (deferred to planner phase, filename `adr-00X-mcp-bcrypt-cache.md`)**: "MCP authenticator may cache bcrypt results for API keys for ≤10s to meet p95<100ms SLO. bcrypt cost 12 retained. Trade-off: 10-second window of lingering-validity after revoke — acceptable for service-account API keys; unacceptable for interactive user passwords (which are not cached)."

**Per-`client_id` rate limiter (M5)**: in-memory sliding-window counter. **30 failures / 60s** → HTTP `429 Too Many Requests` for that `client_id` for the next 60 seconds. Does **not** lock the credential (API keys must remain production-usable). Metric: `agentlens.mcp.auth.ratelimit.tripped.total{client_id_hash=...}` (hashed label to bound cardinality). Documented as "hardening" in `docs/auth.md`.

Authentication steps (9):
1. Parse `Bearer` header; prefix `agentlens_sk_` triggers this flow.
2. Split on first `.` → `(client_id, secret)`.
3. Rate-limiter check → 429 if tripped.
4. Cache lookup → if hit, skip to step 8.
5. DB lookup by `client_id` → if absent, perform dummy bcrypt compare for timing safety, return 401.
6. Status checks (`revoked_at IS NULL`, `expires_at` future).
7. bcrypt compare; on success write cache.
8. Fetch party (must be `service_account`), resolve permissions (§4.1), resolve accessible projects (§4.2).
9. Build `Principal`, convert to `SessionPrincipalRef`, attach to `r.Context()`, schedule async `last_used_at` update (bounded channel — §1.8).

Error bodies: `401 invalid credentials` / `401 credential revoked` / `401 credential expired`. Never leak which of (unknown credential, wrong secret, revoked) failed.

### 3.4 Local JWT flow

Unchanged signature path. Wrap into `Principal{Kind=user_local, AuthMethod="basic_jwt"}`; resolve `AccessibleProjectIDs` uniformly in middleware.

### 3.5 Federation JWT flow (Dex)

All MUST per MCP 2025-11-25:
1. Parse header → extract `kid`; 401 if missing.
2. **JWKS lookup** (M2 resolution): `go-oidc/v3` + `go-jose/v4` verifier; cache TTL `JWKSCacheTTL` (default 1h); forced refresh on `kid` miss **rate-limited to max 1 per provider per `JWKSMinRefreshInterval` (10s default)**. On refresh failure: serve **stale cache** (if present) + increment `agentlens.mcp.jwks.stale_serves.total` + `slog.WarnContext` log. Only emit 503 when no cache exists and refresh fails.
3. Verify signature.
4. Claim checks: `iss` exact; `aud` starts with `Common.AudiencePrefix` (literal prefix, no wildcard); `exp > now`; `nbf <= now`; `iat <= now`.
5. Resolve user via `user_external_identities` (provider, iss, sub). Missing row → 403 + audit-queue entry (default) OR JIT-create (when `auto_provision_users=true`).
6. Group → role mapping (first match; in-memory override per request; no persist in v1).
7. Build `Principal{Kind=user_federated, AuthMethod="federation:dex", CredentialID=<jti>}`.
8. Async `last_seen_at` update via bounded channel (§1.8).

### 3.6 Protected Resource Metadata

`GET /.well-known/oauth-protected-resource` returns:
```json
{
  "resource": "<public_url>",
  "authorization_servers": ["<dex_issuer>"],
  "scopes_supported": ["mcp:discovery"],
  "bearer_methods_supported": ["header"]
}
```
Registered from `cmd/agentlens/main.go` as a **separate unauthenticated `http.Handler`** at root (pre-`/api/v1`) via `kernel.RegisterRoutes("/.well-known/oauth-protected-resource", prmHandler)`. 404 when federation disabled (handler returns 404 at runtime; still registered).

### 3.7 401 / 403 challenges

- Missing/invalid token on `/api/mcp` → `401` + `WWW-Authenticate: Bearer resource_metadata="…/.well-known/oauth-protected-resource", scope="mcp:discovery"`.
- Permission denied → `403` + `WWW-Authenticate: Bearer error="insufficient_scope", scope="catalog:read"`. Loopback adapter (§5) catches and translates to MCP tool-exec error per SEP-1303.

### 3.8 Session ≠ auth

Every JSON-RPC message MUST include `Authorization: Bearer …` — revalidated per request (hitting credcache for API keys). Server-side session state keyed by `<principal_id>:<session_id>` (defense against session-ID reuse across principals). Token expiry mid-session → next request 401; session remains for reconnection with fresh token until TTL.

### 3.9 Audit & timing

`auth.success` / `auth.failure` slog lines with `principal_id`, `principal_kind` (3-value enum), `auth_method`, `credential_id`, `request_id`, `remote_addr`. Never logs secrets. Constant-time on all identity-is-secret paths. Middleware scrubs any inbound payload fields matching secret regex before handing the (ctx, ref) to the plugin.

---

## §4. Authorization Model

### 4.1 Permission resolution

- **Users** (local or federated): `users.role_id → roles.permissions[]`; federated users may get in-memory-only role override via group mapping.
- **Service accounts**: no `role_id`. Permissions derive solely from `PartyRelationship` edges to projects (reuse existing `project:viewer|developer|owner`). `ResolveServiceAccountPermissions(ctx, partyID)` sums project-role permissions.

### 4.2 Accessible projects

`ResolveAccessibleProjects(ctx, partyID)` returns `member_of` project IDs ∪ default project ID (cached). Same code path for users (via person party) and service accounts.

### 4.3 Default project public-reads rule

System default project (`parties.kind='project' AND is_system=true`) readable by every authenticated principal. Enforced via synthesis in `ResolveAccessibleProjects` (not an ACL bypass). Writes still permission-gated. Unauthenticated still denied.

### 4.4 `ScopeByAccessibleProjects` middleware (M4 resolution — context-only, no URL mutation)

Applied to authenticated catalog/capability/stats routes via composition root (for MCP) and via `internal/api/router.go` (for REST UI — existing pattern). On incoming request:

1. Read `SessionPrincipalRef.AccessibleProjectIDs` from ctx.
2. Attach `ctxAccessibleProjectIDs []string` to `r.Context()`.
3. **If `?project=<id>` query param present**: check membership. Not in accessible set → 403.
4. Handler reads `ctxAccessibleProjectIDs` from ctx. `CatalogFilter.ProjectIDs` is populated **from ctx**, not from any URL param. Any `?projects=` user-supplied param is **ignored** (no rewrite, no shadowing).

`CatalogFilter` gains `ProjectIDs []string` field; store query adds `EXISTS(SELECT 1 FROM catalog_project_memberships WHERE ... IN (?))` (parameterized, dual-dialect).

Audit-log note: outbound log line records the effective `projects_scoped` (from ctx), which operators can correlate.

### 4.5 New permissions

```go
PermServiceAccountsRead   = "service_accounts:read"
PermServiceAccountsWrite  = "service_accounts:write"
PermServiceAccountsDelete = "service_accounts:delete"
```

Seeded on `admin` role at migration 010. `developer` gets read-only.

### 4.6 Tool → permission mapping

| Tool | Permission |
|---|---|
| `agent_search`, `agent_get`, `capabilities_list`, `agent_card` | `catalog:read` |

Loopback reuses existing REST middleware — no permission re-wiring per tool.

### 4.7 Enforcement via `RequirePermission`

All service-account + external-identity admin routes mounted with `RequirePermission(auth.PermServiceAccountsX)` / `auth.PermUsersWrite` at route registration per `standards/security/authorization.md`. No inline `auth.HasPermission` calls in handlers.

### 4.8 Decision order

1. Authenticated? (§3; 401)
2. Rate-limiter tripped? (§3.3; 429 — API-key only)
3. Project scope? (§4.4; 403)
4. Permission? (`RequirePermission`; 403 + scope challenge)
5. Resource exists? (handler; 404)
6. Execute.

---

## §5. MCP Plugin & Wire Protocol

### 5.1 Package layout (mirrors `plugins/health/`)

```
plugins/mcpserver/
├── plugin.go              // Plugin struct; Init/Start/Stop; satisfies kernel.Plugin; exports Handler() http.Handler
├── config.go              // plugin-local config access
├── wire/
│   ├── impl.go            // WireImpl interface + factory registry
│   └── diy/
│       ├── transport.go   // Streamable HTTP handler (POST + GET) — stdlib only
│       ├── session.go     // mcp_sessions store + TTL + orphan reaper (DB-backed); uses *model.SessionPrincipalRef
│       ├── jsonrpc.go     // JSON-RPC 2.0 framing + error codes
│       ├── handlers.go    // initialize, ping, tools/list, tools/call, notifications/initialized
│       └── sse.go         // SSE framing + Last-Event-ID resume (GET branch)
├── tool_registry.go       // ToolRegistry interface + default impl
├── loopback.go            // HTTP loopback adapter (uses injected LoopbackFunc — no kernel.Router import)
├── errors.go              // HTTP status → MCP error-code mapping
├── status.go              // /api/mcp/status handler
├── register.go            // self-registration into catalog at Init
└── mcpserver_test.go      // integration tests
```

**Changes from previous revision**:
- `origin.go` removed — Origin middleware lives in `cmd/agentlens/main.go` (composition root).
- `prm.go` removed — `/.well-known/oauth-protected-resource` handler built in `cmd/agentlens/main.go`.
- No chi imports. Plugin exports a raw `http.Handler`.
- Session store signatures use `*model.SessionPrincipalRef`, never `*auth.Principal`.

Naming: per `standards/architecture/plugins.md`, exported struct is `mcpserver.Plugin` (suffix rule satisfied via package namespace). Arch-go naming rules for MCP subpackages deferred (<3-instance threshold).

### 5.2 `kernel.Plugin` lifecycle

```go
type Plugin struct {
    cfg          config.MCPServerConfig
    store        store.Store
    sessionStore *wire.SessionStore   // DB-backed; uses *model.SessionPrincipalRef
    registry     ToolRegistry
    wire         wire.Impl
    loopback     LoopbackFunc         // injected by composition root
    metrics      *Metrics
    handler      http.Handler         // cached after Init
}

func (p *Plugin) Name() string             { return "mcpserver" }
func (p *Plugin) Version() string          { return "1.0.0" }
func (p *Plugin) Type() kernel.PluginType  { return kernel.PluginTypeMiddleware }

func (p *Plugin) Init(k kernel.Kernel) error {
    if !p.cfg.Enabled { return nil }
    // Build wire impl via factory registry
    // Build DB-backed session store (receives model.SessionPrincipalRef only)
    // Build ToolRegistry + register 4 discovery tools
    // Build p.handler = wire.Impl.ServeHTTP closure
    // Self-register catalog entry (non-fatal)
    return nil
}

// Exported accessor: composition root reads this AFTER InitAll, BEFORE StartAll.
func (p *Plugin) Handler() http.Handler { return p.handler }

func (p *Plugin) Start(ctx context.Context) error {
    go p.sessionStore.ReaperLoop(ctx) // 60s TTL + orphan reaper
    return nil
}

func (p *Plugin) Stop(ctx context.Context) error {
    p.sessionStore.Close()                      // flushes last_used_at + last_seen_at buffers (H5)
    return p.markSelfOffline(ctx)
}
```

**Plugin is forbidden (arch-go) from importing `internal/api` and `internal/auth`.** Loopback uses an injected `LoopbackFunc` (type lives in `internal/kernel` or foundation pkg — dependencies flow inward). Composition root builds the loopback closure using `internal/api` internals.

### 5.3 `WireImpl` interface + factory registry

```go
type ImplConfig struct { ProtocolVersion string; SessionTTL time.Duration; MaxSessions int }
type Impl interface {
    Kind() string
    ServeHTTP(w, r, deps Dependencies)
    ProtocolVersion() string
}
type Dependencies struct {
    Sessions *SessionStore
    Registry ToolRegistry
    AuditLog *telemetry.AuditLogger
    Loopback LoopbackFunc
    Metrics  *Metrics
}
type LoopbackFunc func(ctx context.Context, method, path, query, bearer string) (*LoopbackResponse, error)
```

`SessionPrincipalRef` is pulled from `r.Context()` (set by composition-root middleware). Factory registry pattern mirrors federation: `RegisterImpl("diy", factory)` from `wire/diy/diy.go` `init()`.

### 5.4 Streamable HTTP transport

Single endpoint `POST + GET /api/mcp`.

**POST** required headers: `Content-Type: application/json`, `Accept: application/json, text/event-stream`, `Authorization: Bearer …`, `MCP-Protocol-Version: 2025-11-25` (invalid → 400), `Origin: …` (enforced by composition-root middleware), `MCP-Session-Id: <uuid>` (present after initialize, absent on initialize). v1 always responds `Content-Type: application/json`; SSE reserved for future.

**GET** for server→client SSE: `Accept: text/event-stream`, `Authorization` revalidated every GET, `MCP-Session-Id`, optional `Last-Event-ID` resume. Server replies `text/event-stream`, monotonic event IDs, may close TCP + emit `retry: 5000`.

**Per-request checks** in order (most done in composition-root middleware, last two inside plugin):
1. Origin check (middleware).
2. Auth dispatch (middleware → builds `SessionPrincipalRef`).
3. `ScopeByAccessibleProjects` (middleware).
4. Plugin: protocol-version echo.
5. Plugin: session lookup (non-initialize).
6. Plugin: method dispatch.

### 5.5 Origin middleware (composition-root-owned)

Lives in `internal/api/middleware/origin.go` (new), attached only around the MCP handler in `cmd/agentlens/main.go`. Config-driven:
- Empty `AllowedOrigins` + any cross-origin request → `403` (strict default).
- Non-empty list + `Origin` in list → pass; else → `403`.

Global `CORSMiddleware` (`*`) untouched — zero risk to REST/UI.

### 5.6 DB-backed session management

`mcp_sessions` table (§1.4). `SessionStore` interface (plugin-layer type):
```go
func (s *SessionStore) Create(ctx context.Context, ref *model.SessionPrincipalRef, protocolVersion string) (*Session, error)
func (s *SessionStore) Get(ctx, sessionID, principalID string) (*Session, error) // verifies principal_id match + revoked_at IS NULL
func (s *SessionStore) Touch(ctx, sessionID) error                                // UPDATE last_seen_at
func (s *SessionStore) MarkInitialized(ctx, sessionID) error                      // UPDATE initialized_at (L4)
func (s *SessionStore) Revoke(ctx, sessionID) error                               // UPDATE revoked_at
func (s *SessionStore) ReaperLoop(ctx) error                                      // 60s tick; TTL + orphan revocation
```

Cross-principal session reuse attempts on leaked session IDs → 404 (never reveal existence). Revoked sessions retained for audit. Reconnect path (Claude.ai) may check `initialized_at` to decide whether to skip the initialize handshake.

### 5.7 JSON-RPC handlers (v1 scope)

| Method | Behavior |
|---|---|
| `initialize` | Handshake. Response: matched `protocolVersion`, `serverInfo`, `capabilities: {tools:{listChanged:true}}`. Assigns `MCP-Session-Id` header. `capabilities.resources=null`, `capabilities.prompts=null`. |
| `ping` | `{}`. |
| `tools/list` | `{tools: [...]}` from `ToolRegistry.List()` sorted by name (deterministic). |
| `tools/call` | Input `{name, arguments}`. Dispatch per §5.8. |
| `notifications/initialized` | No response. Calls `SessionStore.MarkInitialized(ctx, sessionID)`. |

Out-of-scope methods → `-32601` with `data.supported = [...]` list.

### 5.8 Tool-call execution path

```
tools/call → ToolRegistry.Lookup → validate args vs. InputSchema
         → ArgumentMapper → (method, path, query, body)
         → LoopbackFunc(ctx, method, path, query, bearer)
              (injected closure wraps: REST middleware chain → existing REST handler)
         → ResponseShaper maps (status, body) → MCP result
         → OTel span + audit log
         → Serialize and return
```

**Bearer reconstruction**: pass original `Authorization` header through the closure. Composition-root-built loopback re-runs the full REST middleware chain (auth → scope → permission), but auth hits `credcache` on the second pass — microseconds not milliseconds. ctx-threaded project scope (M4) carries through.

### 5.9 Error code mapping (errors.go)

| HTTP | JSON-RPC code | Shape |
|---|---|---|
| 200 | 0 | success |
| 400 | -32602 | jsonrpc_error (Invalid params) |
| 401 | -32000 | jsonrpc_error (Unauthorized) |
| 403 | -32001 | jsonrpc_error (Forbidden; `data.scope`) |
| 404 | — | tool_error (LLM may retry) |
| 409 | — | tool_error |
| 422 | — | tool_error |
| 429 | -32002 | jsonrpc_error |
| 500 | -32603 | jsonrpc_error |
| 503 | -32603 | jsonrpc_error |

`tool_error` = business-domain (LLM can correct via different args); `jsonrpc_error` = protocol/auth/infra. Aligns with SEP-1303.

### 5.10 `/api/mcp/status`

Authenticated → full JSON (plugin, sessions {active, max}, tools {registered, names}, federation {enabled, provider, reachable, last_checked}, self_registered {catalog_entry_id, in_default_project}). Unauthenticated → subset (no session count, no self_registered IDs).

### 5.11 No `Kernel.Router()` accessor (C2 resolution)

Reverted. Kernel interface stays pure-stdlib (`http.Handler` + existing `RegisterRoutes(prefix string, handler http.Handler)` + `RegisterMiddleware`). `chi.Router` does **not** leak into the kernel interface. See §8.1 for canonical composition-root wiring pattern.

---

## §6. ToolRegistry & 4 Tools

### 6.1 Interface

```go
type ToolEntry struct {
    Name           string    // SEP-986: [a-z0-9_]+
    Description    string    // LLM-facing
    InputSchema    JSONSchema
    HTTPMethod     string    // "GET" v1
    HTTPPath       string    // template with {param} placeholders
    ArgumentMapper ArgumentMapperFunc
    ResponseShaper ResponseShaperFunc
    Annotations    ToolAnnotations // ReadOnlyHint, IdempotentHint (both true v1)
}

type ToolRegistry interface {
    Register(entry ToolEntry) error
    Lookup(name string) (ToolEntry, bool)
    List() []ToolEntry
    Count() int
}
```

JSON Schema 2020-12. Shape designed for v2 OpenAPI→MCP translator.

### 6.2 Tool definitions

All four `GET` tools mapping to existing REST catalog/capability endpoints.

| Tool | REST endpoint | Required | Optional |
|---|---|---|---|
| `agent_search` | `GET /api/v1/catalog?q=&protocol=&status=&limit=&offset=` | — | query, protocol (a2a/mcp/a2ui), status (registered/active/degraded/offline/deprecated), limit (1–100, default 20), offset |
| `agent_get` | `GET /api/v1/catalog/{id}` or `?endpoint=` | one of id/endpoint | — |
| `capabilities_list` | `GET /api/v1/capabilities?kind=&q=&limit=&offset=` | — | kind (mcp.tool/mcp.resource/mcp.prompt/a2a.skill), query, limit (1–100, default 50), offset |
| `agent_card` | `GET /api/v1/catalog/{id}/card` | id | — |

Loopback dispatch carries ctx-threaded project scope (§4.4); user args cannot override it.

### 6.3 Mappers / shapers

Per tool: `ArgumentMapper` converts validated args → (query, path-params, body). `ResponseShaper` converts (body, status) → `[]MCPContent` + isError flag. v1 all shapers return `MCPContent{Type:"text"}`.

### 6.4 Registration at Init

`Plugin.registerDiscoveryTools(k)` constructs 4 `ToolEntry` slice and calls `registry.Register(...)` in loop.

### 6.5 v2 translator path (documented, not implemented)

When v2 ships: `registerDiscoveryTools` body disappears; translator registers same slice built from OpenAPI. v1 hand-coded tools are the compatibility reference.

---

## §7. Self-Registration & Observability

### 7.1 Self-registration (M6 — multi-instance-disambiguated)

At `Init()` (non-fatal on error):
- `endpoint = "agentlens:mcp-discovery:" + cfg.PublicURL`.
- `AgentKey = SHA256("mcp" + endpoint)`.
- `AgentType`: protocol=`mcp`, endpoint above, version=`1.0.0`, spec_version=`<cfg.ProtocolVersion>`, capabilities=`ToolRegistry.List()` mapped to `model.MCPTool`.
- `CatalogEntry`: display_name="AgentLens MCP Discovery Server", description, status=`active`, source=`push`.
- Idempotent via `FindByEndpoint` → update existing OR `store.Create` (default-project auto-assign).
- On `Stop`: mark offline (UpdateEntry with `status=offline`).

`source=push` protects the entry from discovery-plugin overwrites per `standards/architecture/domain-model.md`.

### 7.2 OTel spans

Namespace `agentlens.mcp.*`:

```
otelhttp (chi middleware)
  └─ mcp.rpc
       ├─ mcp.tools.call
       │    └─ mcp.tool.loopback
       │         └─ otelhttp (internal route via loopback)
       └─ mcp.session.touch
```

Attributes include `mcp.tool.name`, `mcp.session.id`, `mcp.protocol.version`, `auth.principal.{id,kind}` (3-value enum from M3), `auth.method`, `mcp.tool.result.{content_parts,is_error,status_code}`.

### 7.3 OTel metrics

Namespace `agentlens_mcp_*`:

Core:
- `rpc.count` (counter; method, outcome)
- `rpc.duration` (histogram)
- `tool_calls.total` (counter; tool, outcome)
- `tool_call.duration` (histogram; tool)
- `sessions.active` (up-down counter)
- `sessions.orphan_revoked.total` (counter — L2)
- `auth_failures.total` (counter; reason, auth_method)
- `federation_reachable` (observable gauge; provider)

Cache / rate-limit / JWKS (new in revision 2):
- `auth.credcache.hits.total` / `misses.total` / `invalidations.total` — H6
- `auth.ratelimit.tripped.total` (label: client_id_hash) — M5
- `credcache.dropped_updates.total` — H5 (API key `last_used_at` drops)
- `federation.dropped_last_seen.total` — H5
- `jwks.stale_serves.total` (label: provider) — M2

Per `standards/architecture/observability.md`, providers global via `otel.SetTracerProvider/SetMeterProvider`. Plugin obtains tracer/meter via `otel.Tracer("agentlens/mcpserver")`. Label cardinality bounded: tool ∈ {4}, outcome ∈ {6}, reason ∈ fixed enum, client_id_hash truncated to 8 hex chars (=65k buckets; bounded, acceptable for ops dashboards).

### 7.4 Audit log

Per tool invocation `slog.InfoContext(ctx, "mcp.tool.invoked", ...)` with `ToolInvocationAudit` fields: ts, request_id, session_id, principal_{id,kind} (3-value enum), auth_method, credential_id, tool_name, arguments (scrubbed via `redact.ScrubJSON`), projects_scoped (from ctx — M4), outcome, http_status, duration_ms, remote_addr. Secrets matching `(?i)(password|secret|token|key|credential)` → `***`.

Failed auth: `auth.failure` with `reason` + partial identification hint.

Session reaper revocations: `mcp.session.reaper.orphan_revoked` audit line (L2).

### 7.5 Federation health

`Provider.Start(ctx)` spawns `healthLoop` — every `HealthCheckInterval`, pings `{issuer}/.well-known/openid-configuration` with `HealthCheckTimeout`. Cached `Status{Reachable, LastChecked, LastError}` exposed via `LastStatus()`. Emits `federation_reachable` gauge.

### 7.6 `/readyz` extension

```go
if !deps.DB.Ping()                               → 503 "db unreachable"
if fedProvider != nil && !LastStatus().Reachable → 503 "federation provider unreachable"
if mcp != nil && !mcp.Ready()                    → 503 "mcp plugin not ready"
→ 200 ready
```

`/healthz` remains liveness-only.

### 7.7 Operator alert reference (docs)

In `docs/observability.md`:

| Alert | Condition | Severity |
|---|---|---|
| Tool error rate | `rate(agentlens_mcp_tool_calls_total{outcome="server_error"}[5m]) > 0.05` | warn |
| Auth failure spike | `rate(agentlens_mcp_auth_failures_total[5m]) > 5` | warn |
| Federation down | `agentlens_mcp_federation_reachable == 0 for 5m` | critical |
| JWKS stale serves | `rate(agentlens_mcp_jwks_stale_serves_total[5m]) > 0` | warn |
| Session capacity | `active/max > 0.8` | warn |
| Credcache dropped updates | `rate(agentlens_mcp_credcache_dropped_updates_total[5m]) > 0` | info |
| Rate limiter tripping | `rate(agentlens_mcp_auth_ratelimit_tripped_total[5m]) > 0` | info |
| p95 SLO | `histogram_quantile(0.95, …tool_call_duration…) > 0.1` | warn |

No dashboard JSON shipped.

---

## §8. Deployment & Operations

### 8.1 Composition-root wiring (canonical — replaces `Kernel.Router()`)

`cmd/agentlens/main.go` glue, executed after `pm.InitAll()` and before `pm.StartAll()`:

```
// 1. Build federation provider if enabled
provider := federation.BuildProvider(cfg.Federation.Provider, cfg.Federation.Instance, cfg.Federation.Common)
if provider != nil { provider.Init(ctx) }

// 2. Build credcache + rate limiter
credCache := credcache.New(1024, 10*time.Second)
rateLimiter := authrl.New(30, 60*time.Second)

// 3. Build middleware chain (all resolve to *model.SessionPrincipalRef in ctx)
authDispatch := authmw.RequireAuthOrPrincipalDispatch(jwtService, apiKeyResolver, provider, credCache, rateLimiter)
scope        := authmw.ScopeByAccessibleProjects(partyStore)
origin       := mcpmw.OriginValidation(cfg.MCPServer.AllowedOrigins)

// 4. Build loopback closure — captures chi router + bearer
loopback := api.BuildLoopbackFunc(chiRouter) // wraps httptest.NewRecorder

// 5. Inject loopback into plugin
mcpPlugin.SetLoopback(loopback) // called AFTER InitAll, BEFORE StartAll

// 6. Wrap plugin handler with middleware chain
mcpHandler := origin(authDispatch(scope(mcpPlugin.Handler())))

// 7. Register routes on kernel (no chi accessor needed)
kernel.RegisterRoutes("/api/mcp", mcpHandler)
kernel.RegisterRoutes("/.well-known/oauth-protected-resource", prmHandler(cfg, provider))

// 8. Startup WARN when audit disabled (L3)
if cfg.MCPServer.Enabled && !cfg.MCPServer.AuditEnabled {
    slog.WarnContext(ctx, "MCP audit logging disabled — forensic trail unavailable",
        "component", "mcpserver")
}

// 9. Now pm.StartAll()
```

**arch-go compliance**: `cmd/agentlens` is entrypoint layer — may import everything. Plugin stays clean (no `internal/api`, no `internal/auth`, no chi).

Shutdown order (LIFO): `pm.StopAll()` (plugin flushes last_used_at buffer + marks self offline) → `provider.Stop(ctx)` → `telemetry.Shutdown(ctx)`.

### 8.2 Helm chart (0.2.0 → 0.3.0)

```yaml
# deploy/helm/agentlens/Chart.yaml
apiVersion: v2
name: agentlens
version: 0.3.0
appVersion: 0.3.0
dependencies:
  - name: postgresql
    version: ~16.x
    repository: https://charts.bitnami.com/bitnami
    condition: postgresql.enabled
  - name: dex
    version: "~0.19.x"
    repository: https://charts.dexidp.io
    condition: dex.enabled
```

New `values.yaml` blocks: `mcpServer.*`, `federation.*`, `dex.*`. `mcpServer.enabled=false` default. `federation.enabled=false` default.

`templates/_helpers.tpl` adds `agentlens.federationAudiencePrefix` (defaults to `mcpServer.publicURL`) and `agentlens.validateConfig` (fails `helm template`/`helm lint --strict` on missing required values).

`templates/deployment.yaml` appends env var pass-through for all `AGENTLENS_MCP_*` and `AGENTLENS_FEDERATION_*` when flags on.

`ci/ci-values.yaml` adds full stack enabled. `scripts/test-helm-templates.sh` extended to assert new env vars + Dex deployment.

### 8.3 `docker-compose.dev.yml` (new)

Bundles AgentLens + Dex:
- `agentlens`: image with `AGENTLENS_MCP_SERVER_*` + `AGENTLENS_FEDERATION_*` env; depends_on Dex healthy.
- `dex`: **`ghcr.io/dexidp/dex:v2.41.1@sha256:<digest-to-confirm-at-planning>`** (L5 — implementer confirms latest stable Dex tag + digest at planning phase; pin by digest). Mounted `dex-config.yaml` (LDAP + GitHub connector examples commented). Healthcheck on `/dex/healthz/live`.

### 8.4 Admin REST surface

Service-account routes (under `/api/v1/service-accounts`, handlers in `internal/api/service_account_handlers.go`):

| Method | Path | Permission |
|---|---|---|
| `GET` | `/` | `service_accounts:read` |
| `POST` | `/` | `service_accounts:write` |
| `GET` | `/{id}` | `service_accounts:read` |
| `DELETE` | `/{id}` | `service_accounts:delete` |
| `GET` | `/{id}/keys` | `service_accounts:read` |
| `POST` | `/{id}/keys` | `service_accounts:write` |
| `DELETE` | `/{id}/keys/{keyID}` | `service_accounts:write` |
| `PATCH` | `/{id}/secret` | `service_accounts:write` (rotation — UPDATE-then-INSERT, H3) |
| `POST` | `/{id}/projects` | `service_accounts:write` |
| `DELETE` | `/{id}/projects/{projectID}` | `service_accounts:write` |

External-identity admin routes (under `/api/v1/admin/external-identities`, handlers in `internal/api/external_identity_handlers.go`):

| Method | Path | Permission |
|---|---|---|
| `GET` | `/pending` | `users:write` |
| `POST` | `/link` | `users:write` |
| `POST` | `/create-and-link` | `users:write` |
| `DELETE` | `/{id}` | `users:write` |

All mounted via `RequirePermission(...)` at route registration.

Key-creation response (one-time secret):
```json
{"id":"cred-uuid","label":"doc-pipeline-prod","secret":"agentlens_sk_cred-uuid.<32b>","created_at":"..."}
```

### 8.5 Admin UI (Group G — in v1 PR)

Under `web/src/routes/admin/`:
- `ServiceAccountsPage.tsx`, `ServiceAccountDetailPage.tsx`, `PendingIdentitiesPage.tsx`.

Route additions in `AppRouter.tsx`; nav entries in admin sidebar. shadcn/ui primitives; TanStack React Query; `@/*` alias; PascalCase `.tsx`. `.test.tsx` siblings with Vitest coverage 80/80/75/80.

### 8.6 Bootstrap / first-run UX

1. AgentLens starts with flags off. Basic-auth admin bootstrapped as today.
2. Operator sets `mcp_server.enabled=true` + `mcp_server.public_url` + (optionally) `federation.enabled=true` + Dex config, restarts.
3. Admin creates first service account via UI; secret shown once (Anya's flow).
4. Admin shares public URL + quickstart.
5. Anya sets `AGENTLENS_API_KEY=agentlens_sk_...`, starts her app, sees tools.
6. Karol pastes URL into Claude.ai Custom Connector → browser redirect to Dex → SSO → AgentLens receives unknown external identity → admin gets pending-identity notification.
7. Priya opens Pending Identities, links Karol's Dex identity to his AgentLens user; Karol retries and succeeds.

Documented in new `docs/mcp-quickstart.md` with commands + Playwright screenshots.

### 8.7 Upgrade path (0.2.0 → 0.3.0)

1. `helm upgrade agentlens oci://... --version 0.3.0` — migration 010 runs on pod start.
2. `mcp_server.enabled=false` default preserves current behavior.
3. `federation.enabled=false` default.
4. Rollback: flip flags off; new tables stay unused. Forward-only per standards.

### 8.8 Release

Semantic-release tags `v0.3.0` + `helm/v0.3.0`. Image `ghcr.io/agentlens/agentlens:0.3.0`; OCI chart `oci://ghcr.io/agentlens/charts/agentlens:0.3.0`. Dex is a dep, not re-published.

### 8.9 Smoke test (operator)

1. `kubectl logs` shows `auth.federation.enabled=true`, `mcp.plugin.started`, no audit-disabled warning.
2. `curl https://.../api/mcp/status` — expect JSON with `plugin.enabled=true`.
3. `curl https://.../.well-known/oauth-protected-resource` — expect JSON pointing to Dex issuer.
4. MCP client JSON-RPC `initialize` succeeds, `tools/list` returns 4 tools.

### 8.10 Pinned dependencies (M1 — confirm exact via context7 at planning)

Go module additions:
```
github.com/coreos/go-oidc/v3 v3.11.0   // or latest stable in the ^3.11.0 line
github.com/go-jose/go-jose/v4 v4.0.4   // or latest stable in the ^4.0.4 line
```

(`v3` of go-jose is EOL; target v4. Planner runs `context7` to confirm exact latest stable before go.mod update.)

---

## Reusable Components

### Existing code to leverage

| Concern | Source | Reuse strategy |
|---|---|---|
| Plugin scaffold | `plugins/health/health.go` | Copy Plugin struct + lifecycle shape |
| Dual-dialect migration + seed | `internal/db/migrations.go` (migration007) | Pattern: AutoMigrate + raw `tx.Exec` for partial indexes + INSERT … WHERE NOT EXISTS + dialect branch |
| Auth middleware | `internal/api/auth_middleware.go` | `RequireAuth(jwtService)` + `RequirePermission(perm)` curry; extend with Principal-building dispatch that emits `*model.SessionPrincipalRef` to ctx |
| Permission constants | `internal/auth/permissions.go` | Add 3 new `service_accounts:*` consts |
| Party model / store | `internal/model/party.go` + `internal/store/party_store.go` | New `PartyKindServiceAccount` + `CreateServiceAccount` helper |
| Catalog read path | `internal/store/sql_store_query.go` + `CatalogFilter` | Extend `CatalogFilter` with `ProjectIDs []string` |
| Self-registration | `internal/store/sql_store.go` `Create` | Idempotent upsert by AgentKey |
| Config discriminator pattern | `internal/config/config.go` `DatabaseConfig` | Shape `FederationConfig` identically |
| Telemetry metrics | `internal/telemetry/metrics.go` `HealthMetrics` | Copy shape; namespace `agentlens.mcp.*` |
| Admin UI CRUD | `web/src/pages/SettingsPage.tsx` UsersTab | shadcn Tabs+Table+Dialog + useState forms |
| Helm subchart pattern | `deploy/helm/agentlens/Chart.yaml` (bitnami/postgresql) | Chart.lock pin + `condition:` + values threading |
| Graceful shutdown | `internal/server/server.go` + `cmd/agentlens/main.go` | LIFO defer; `provider.Stop` slots after `pm.StopAll` |
| E2E helpers | `e2e/tests/helpers.ts` | `loginViaAPI`, `loginViaUI`, `authHeader`, `BASE`, port 18080 |

### New components required (with justification)

| New component | Why no existing reuse |
|---|---|
| `internal/model/session_principal_ref.go` (`SessionPrincipalRef`) | No foundation-layer opaque identity type exists; needed to cross into plugin without importing `internal/auth` (C1) |
| `internal/auth/principal.go` (`Principal`) | Normalizes 3 auth paths; stays inside `internal/auth` — never imported by plugin |
| `internal/auth/credcache/` | H6 — bcrypt is too slow per request to meet p95<100ms |
| `internal/auth/federation/` | Zero OIDC/JWKS code in repo; net-new |
| `internal/api/middleware/origin.go` | Scoped origin middleware lives in api layer now (composition root attaches) |
| `plugins/mcpserver/` | v1 feature scope; modeled on `plugins/health/` |
| `ToolRegistry` | v2 translator-compatibility mandates abstraction |
| `WireImpl` factory registry | Prepares for alternative wire impls |
| DB-backed `mcp_sessions` store | No session store exists; persistence required |
| `service_account_handlers.go` + `external_identity_handlers.go` | New admin REST surface |
| 3 admin UI pages | No existing management surface |
| `docker-compose.dev.yml` | Not present today |

---

## Technical Approach

### Integration strategy

- Plugin composes via existing kernel lifecycle `Register → InitAll → StartAll → StopAll`.
- Composition root (`cmd/agentlens/main.go`) builds `federation.Provider`, `credcache`, rate limiter, and wraps the plugin's raw `http.Handler` with REST middleware before calling `kernel.RegisterRoutes`.
- Plugin exports only `Handler() http.Handler` + `SetLoopback(fn)` — no chi, no api, no auth imports.
- `RequireAuth` grows dispatch step (token-prefix + iss-based) + emits `*model.SessionPrincipalRef` to ctx; existing context keys preserved.
- Admin REST routes mounted on existing chi router with `RequirePermission`.

### Data flow

```
LLM client ─POST /api/mcp─▶ Origin mw ─▶ Auth dispatch (ctx += SessionPrincipalRef)
       ─▶ ScopeByAccessibleProjects (ctx += AccessibleProjectIDs)
       ─▶ Plugin Handler ─▶ wire ServeHTTP ─▶ session store (uses ref from ctx)
       ─▶ JSON-RPC dispatch ─▶ tools/call: ToolRegistry.Lookup → ArgumentMapper
       ─▶ LoopbackFunc (composition-root-built closure; replays full REST chain)
            ─▶ auth revalidate (credcache hit) ─▶ scope (ctx) ─▶ RequirePermission ─▶ catalog handler
       ─▶ ResponseShaper ─▶ MCP result ─▶ OTel span + audit ─▶ serialize
```

### Layer boundaries (arch-go compliance — C1 + C2 resolved)

- `plugins/mcpserver/**` imports: `internal/kernel`, `internal/model`, `internal/config`, `internal/store`, `internal/telemetry`. **Never** `internal/api`, `internal/auth`, `cmd/**`, `chi`.
- `internal/auth/**` + `internal/auth/federation/**` + `internal/auth/credcache/**` — infrastructure layer (no api/plugins imports).
- `internal/api/service_account_handlers.go` + `external_identity_handlers.go` + `middleware/origin.go` — api layer.
- `cmd/agentlens/main.go` — entrypoint; imports everything; owns wiring.
- No `Kernel.Router()`. Kernel interface stays `net/http`-only.

### Architectural decision set

Revision 2 resolves:
- **C1** via `SessionPrincipalRef` in foundation + middleware-resolves-Principal-then-passes-ref.
- **C2** via composition-root handler wrapping (drops `Kernel.Router()` accessor).
- **H6** via `credcache` (10s TTL; ADR deferred to planner).

Previously-resolved decisions preserved:
- DB-backed `mcp_sessions` (Phase 2).
- Scoped Origin middleware, global CORS untouched (Phase 2 + 5).
- One-active-secret rotation (Phase 5; UPDATE-then-INSERT per H3).
- Single-PR delivery, flags off by default (Phase 5).
- `WireImpl` factory + provider registry (Phase 5).

---

## Implementation Guidance

### Testing Approach

**2–8 focused tests per implementation step group.** Test verification runs only new tests, not entire suite.

Expected test group split (~12 groups):

| Group | Scope | Tests |
|---|---|---|
| A | Migration 010 (dual-dialect; partial index assertions via sqlite_master/pg_indexes; seed) | 5–7 |
| B | Models + party enum expansion + secret store + `SessionPrincipalRef` | 4–6 |
| C | Principal + API-key flow + credcache (hit/miss/invalidate) + rate limiter | 6–8 |
| C2 | Federation flow (JWKS stub + rate-limited refresh + stale-serve) | 4–6 |
| D | Authz resolution + `ScopeByAccessibleProjects` (ctx-only) + default-project rule | 4–6 |
| E | Composition-root wiring + Origin middleware + loopback closure | 4–6 |
| F | MCP plugin lifecycle + session store (incl. `initialized_at`, reaper, orphan sweep) | 6–8 |
| G | ToolRegistry + 4 tools (schema validate + mappers + shapers + loopback) | 6–8 |
| H | Self-registration (multi-instance endpoint) + /status + /readyz federation chain | 4–6 |
| I | Service-account + external-identity REST handlers (incl. rotation UPDATE-then-INSERT + concurrent-rotation race) | 7–8 |
| J | Admin UI (Vitest .test.tsx per page) + Playwright data-testid screenshots | 4–6 |
| K | E2E: MCP Go client happy-path + hybrid real-Dex path in `e2e/mcp/` | 3–5 |

**Test kinds**:
- Go unit: table-driven `t.Run` + testify; co-located `_test.go`; `store.NewSQLiteStore(":memory:")`; `t.Cleanup` for DB close.
- Go integration: real chi router + real SQLite + mocked federation provider (JWKS `httptest` stub).
- Frontend Vitest: jsdom + Testing Library; coverage thresholds 80/80/75/80.
- Playwright E2E: serial single-worker; reuse `helpers.ts`; port 18080.
- Real-Dex E2E: dedicated workflow step.
- Lockout standard: separate accounts per test (for basic-auth paths).

### Standards Compliance

| Standard | Applicable rules |
|---|---|
| `standards/architecture/layering.md` | Plugin imports only foundation+infrastructure+kernel; **no** `internal/auth` or `internal/api`; kernel stays stdlib-only (no chi leak); 80-line/5-param/3-return fn limits |
| `standards/architecture/plugins.md` | `mcpserver.Plugin` in `plugins/mcpserver`; lifecycle `Register→Init→Start→Stop`; composition root wires loopback |
| `standards/architecture/observability.md` | Telemetry infrastructure; providers set globally; tracer/meter via `otel.Tracer(...)` |
| `standards/architecture/domain-model.md` | Self-registered entry `source=push`; AgentKey = SHA256(protocol+endpoint); endpoint incorporates PublicURL |
| `standards/security/authentication.md` | bcrypt cost 12 retained (ADR covers 10s credcache); constant-time; rate-limit on API-key (not a lockout) |
| `standards/security/authorization.md` | `RequirePermission` at route registration; `auth.Perm*` consts |
| `standards/security/data-handling.md` | `SecretHash`/secrets never logged; `json:"-"` + `gorm:"type:text"`; GORM parameterized; CORS `*` untouched (scoped Origin mw separate); error-leak discipline |
| `standards/backend/database-dialects.md` | Dialect-branched raw `tx.Exec` for partial indexes (H2); migration test asserts DDL present; forward-only; idempotent |
| `standards/backend/go-conventions.md` | `context.Context` first; `fmt.Errorf("… : %w", err)`; `slog.InfoContext(ctx, ...)` with `component=mcpserver`; three-group imports |
| `standards/backend/migrations.md` | `Description` field (not `Name`, H1); small focused change |
| `standards/backend/models.md` + `queries.md` | Timestamps; DB-level UNIQUE; parameterized IN bindings; index on FK + WHERE cols |
| `standards/frontend/ui-stack.md` + `state-and-data.md` + `accessibility.md` | shadcn/ui; TanStack React Query; PascalCase `.tsx`; a11y labels |
| `standards/frontend/build-and-tooling.md` | Bun 1.3.11; `tsc && vite build` |
| `standards/testing/go-testing.md` + `frontend-testing.md` + `e2e.md` | Table-driven subtests; 80/80/75/80 Vitest; Playwright serial port 18080 |
| `standards/devops/commits.md` + `ci-gates.md` + `containers.md` + `diagrams.md` | Conventional Commits; lint+test+build; CGO_ENABLED=1; Mermaid-only |
| `standards/global/minimal-implementation.md` | No speculative methods |
| `standards/global/pr-checklist.md` | 7-step gate + **`docs/auth.md`** (M7) |

### Branch + delivery

- Branch: `feat/mcp-discovery-server-v1`.
- Single large PR. `mcp_server.enabled=false` default.
- Conventional Commits.

### Documentation (PR-checklist — M7 resolution adds `docs/auth.md` + README callout)

| Doc | Content |
|---|---|
| `docs/api.md` | All new routes (method/path/schema/errors/permissions); 3 new permissions |
| `docs/architecture.md` | Mermaid: MCP plugin + Dex + federation + composition-root wiring + loopback |
| `docs/end-user-guide.md` | Service-account admin flow + pending-identity approval; data-testid screenshots |
| `docs/settings.md` | `mcp_server.*` + `federation.*` blocks with defaults + env vars |
| `docs/observability.md` (new) | Alert recipes from §7.7 |
| `docs/mcp-quickstart.md` (new) | 5-min getting-started for Anya + Karol |
| **`docs/auth.md` (M7)** | New permissions (`service_accounts:*`), `SessionPrincipalRef` + `Principal`, federation flow, service-account flow, per-client_id rate limiter (documented as hardening), credcache 10s TTL rationale |
| `README.md` (M7) | Short MCP callout paragraph pointing to `docs/mcp-quickstart.md` |

---

## Out of Scope

**Deferred (explicit non-goals for v1)**:
- Admin/write/destructive MCP operations.
- Semantic search / embeddings in `agent_search`.
- Per-project path-scoped MCP endpoints.
- stdio transport.
- Sidecar / separate-binary deployment.
- OpenAPI emission framework (v1.5).
- Generalized OpenAPI-to-MCP translator (v2).
- CIMD / RFC 7523 JWT-Bearer.
- Dual-secret overlap rotation (v1.5).
- `notifications/tools/list_changed` live update.
- Per-tool rate limiting (v1.5+).
- Image / binary `MCPContent` types.
- Group→role persistence from federation.
- Ship OTel dashboards.
- arch-go `namingRules` entries for MCP subpackages.
- Bcrypt-cache TTL > 10s or persistent cache.
- Operator-tunable last_used_at buffer size.

---

## Success Criteria

**Functional**:
- `make test` passes (Go unit + integration + arch-test + frontend Vitest coverage ≥80/80/75/80).
- `make e2e-test` passes including real-Dex E2E hybrid path.
- `helm lint --strict` + `helm template` + `scripts/test-helm-templates.sh` pass.
- MCP Go client `initialize → tools/list → tools/call (each of 4)` succeeds end-to-end.
- Anya scenario: env var → app starts → `agent_search` returns results (<100ms p95 local).
- Karol scenario: paste URL → Dex login → 4 tools visible in Claude.ai Custom Connector.
- Priya scenario: pending-identity → link-to-user → Karol retries → success; revoke SA key → next call 401 (within 10s of credcache TTL).

**Non-functional**:
- p95 tool-call latency < 100ms on local SQLite reference benchmark (credcache hit path; cold bcrypt allowed on first request).
- Zero secret leakage in slog output.
- arch-go: 100% layer compliance; plugin never imports `internal/api` or `internal/auth`.
- Migration 010 idempotent + partial indexes verified via `sqlite_master`/`pg_indexes`.
- `mcp_server.enabled=false` default preserves behavior.
- 80/80/75/80 Vitest thresholds maintained.

**Observability**:
- Every tool invocation → one OTel span (+ loopback child), one audit log, one metric counter.
- `/api/mcp/status` + `/readyz` reflect real-time federation + MCP plugin health.
- Audit-disabled startup WARN emitted when flagged off.
- Stale-JWKS serves visible via metric.

**Security**:
- Bcrypt-hashed secrets (cost 12 at rest; 10s result cache in memory with documented ADR).
- Strict-default Origin allowlist.
- Audience-bound Dex tokens.
- Global CORS untouched.
- Constant-time auth paths.
- Per-client_id rate limiter (30 fails/60s → 429).
- Rotation transaction UPDATE-then-INSERT ordering enforced.

**Delivery**:
- Single PR off `feat/mcp-discovery-server-v1`, Conventional Commits, all 7 PR-checklist items satisfied + `docs/auth.md` + `README.md` callout.
- Chart 0.3.0 + app 0.3.0 tagged by semantic-release post-merge.

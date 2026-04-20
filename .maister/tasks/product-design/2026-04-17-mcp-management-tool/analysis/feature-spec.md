# Feature Specification — MCP Discovery Server for AgentLens

Implementation-ready specification. Built section-by-section during Phase 6; approved sections are appended in order.

Linked artifacts (do NOT duplicate content here — reference them):
- `analysis/problem-statement.md` (scope, constraints, success criteria)
- `analysis/personas.md` (Anya, Karol, Priya)
- `analysis/design-decisions.md` (selected alternatives + rationale)
- `analysis/alternatives.md` (full trade-off catalog)
- `analysis/codebase-analysis.md` (existing AgentLens structure)
- `analysis/research-mcp-auth-transport.md` (MCP 2025-11-25 spec requirements)
- `analysis/research-mcp-admin-tool-design.md` (tool design principles)

---

## §1. Data Model & Migrations

This section defines the schema changes needed by the full stack.

### 1.1 Entities added

#### 1.1.1 `parties.kind` enum expansion

Existing `PartyKind` (`internal/model/party.go`):
```go
type PartyKind string
const (
    PartyKindPerson         PartyKind = "person"
    PartyKindGroup          PartyKind = "group"
    PartyKindProject        PartyKind = "project"
)
```

Add:
```go
    PartyKindServiceAccount PartyKind = "service_account"
```

No schema change to `parties` table — `parties.kind` is already `TEXT` (SQLite) / `VARCHAR` (Postgres). Go code switches on `kind` must handle the new value:
- `internal/api/party_handlers.go` → `RegisterPartyKindRoutes("service_accounts", ...)` for CRUD.
- `internal/store/party_store.go` → `CreateServiceAccount(ctx, name) (*model.Party, error)`.
- `internal/auth/party_permissions.go` → reuse existing project-role map (service accounts get same `project:viewer|developer|owner` roles as users).

#### 1.1.2 `api_client_credentials` (new table)

```sql
CREATE TABLE api_client_credentials (
    id                 TEXT PRIMARY KEY,              -- UUID v4
    party_id           TEXT NOT NULL,                 -- FK → parties.id, kind must be 'service_account'
    secret_hash        TEXT NOT NULL,                 -- bcrypt cost 12, matches password storage
    label              TEXT NOT NULL,                 -- human-readable, e.g. "doc-pipeline-prod-2026-04"
    created_at         DATETIME NOT NULL,             -- TIMESTAMPTZ in Postgres
    created_by_user_id TEXT NOT NULL,                 -- FK → users.id; audit trail
    last_used_at       DATETIME NULL,                 -- updated asynchronously (buffered write)
    expires_at         DATETIME NULL,                 -- optional soft expiry; NULL = no expiry
    revoked_at         DATETIME NULL,                 -- set on revoke; NULL = active
    FOREIGN KEY (party_id) REFERENCES parties(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX idx_api_client_credentials_party_active ON api_client_credentials(party_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_api_client_credentials_active ON api_client_credentials(revoked_at, expires_at);
```

**Go model** (`internal/model/api_client_credential.go`):
```go
type APIClientCredential struct {
    ID              string     `gorm:"primaryKey;type:text" json:"id"`
    PartyID         string     `gorm:"not null;type:text;index" json:"party_id"`
    SecretHash      string     `gorm:"not null;type:text" json:"-"`    // never serialize
    Label           string     `gorm:"not null;type:text" json:"label"`
    CreatedAt       time.Time  `json:"created_at"`
    CreatedByUserID string     `gorm:"type:text" json:"created_by_user_id"`
    LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
    ExpiresAt       *time.Time `json:"expires_at,omitempty"`
    RevokedAt       *time.Time `json:"revoked_at,omitempty"`
}
```

Plaintext secret is **never** stored; shown once at creation, then only the hash persists. `json:"-"` on `SecretHash` enforces non-serialization.

#### 1.1.3 `user_external_identities` (new table)

Maps Dex-issued subjects back to AgentLens `users` rows.

```sql
CREATE TABLE user_external_identities (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL,
    provider     TEXT NOT NULL,                      -- "dex" (single value for v1; keyed for future multi-IdP)
    external_sub TEXT NOT NULL,                      -- Dex's `sub` claim
    external_iss TEXT NOT NULL,                      -- Dex's `iss` claim; locks mapping to one Dex instance
    created_at   DATETIME NOT NULL,
    last_seen_at DATETIME NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE (provider, external_iss, external_sub)
);
CREATE INDEX idx_user_external_identities_user ON user_external_identities(user_id);
```

The `UNIQUE(provider, external_iss, external_sub)` constraint prevents two AgentLens users from claiming the same Dex identity. The `external_iss` in the key means if AgentLens is ever reconnected to a different Dex instance, old mappings are not silently trusted.

### 1.2 Migration 008 (idempotent, dual-dialect)

Appended to `internal/db/migrations.go` `AllMigrations()`:

```go
{
    Version: 8,
    Name:    "service_accounts_and_api_clients",
    Up: func(tx *gorm.DB) error {
        if err := tx.AutoMigrate(&model.APIClientCredential{}); err != nil {
            return fmt.Errorf("creating api_client_credentials: %w", err)
        }
        tx.Exec(`CREATE INDEX IF NOT EXISTS idx_api_client_credentials_party_active
                 ON api_client_credentials(party_id) WHERE revoked_at IS NULL`)
        if err := tx.AutoMigrate(&model.UserExternalIdentity{}); err != nil {
            return fmt.Errorf("creating user_external_identities: %w", err)
        }
        return nil
    },
},
```

Forward-only per `standards/backend/database-dialects.md`. No Down function; rollback via backup/restore.

### 1.3 Entity relationships

```
parties (kind=service_account)
   ↓ 1:N
api_client_credentials

users
   ↓ 1:N
user_external_identities

parties (kind=project)
   ↓ via PartyRelationship (existing)
parties (kind=person OR service_account) — project memberships reuse the existing PartyRelationship graph
```

Service-account project membership:
```sql
INSERT INTO party_relationships (from_party_id, to_party_id, relationship_name, from_role, to_role)
VALUES ('<sa-party-id>', '<project-party-id>', 'member_of', 'service_account', 'project:viewer');
```

No new relationship kind required.

### 1.4 Secret lifecycle

1. **Create**: admin calls `POST /api/v1/service-accounts/{id}/keys` (body `{label}`). Server generates 32-byte random secret, bcrypt-hashes at cost 12, stores hash + label. Response one-time includes `secret`.
2. **Use**: client sends `Authorization: Bearer agentlens_sk_<id>.<secret>`. Server splits on `.`, looks up `api_client_credentials.id`, bcrypt-compares, validates `revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`, updates `last_used_at` asynchronously (goroutine with 30s buffer).
3. **Rotate**: admin issues new credential via same POST. Existing key valid until explicit revoke.
4. **Revoke**: admin calls `DELETE /api/v1/service-accounts/{id}/keys/{keyID}` → sets `revoked_at = now()`. Invalid on next request.

Token format: `agentlens_sk_<credential_id>.<secret>`. Prefix `agentlens_sk_` is a hint for git-leak scanners and distinguishes from OAuth bearer tokens. The `.` separator keeps `<credential_id>` recoverable without full-table scan.

### 1.5 Backward compatibility

- Existing users, parties, projects, catalog entries unchanged.
- Existing JWT flows unchanged — `RequireAuth` still handles JWT-bearing requests; service-account-key path is additive.
- No existing JSON output shapes change.

### 1.6 JIT provisioning policy

v1 default: **admin-approval-queue**. A successful Dex login with no matching `user_external_identities` row returns `403 Forbidden` + creates a log entry for admin review. Admin UI lets admin link the new external identity to an existing user OR create + link in one action.

Config flag `federation.common.auto_provision_users=false` (default; see §2). When `true`, first-login auto-creates both a new `users` row AND the mapping in one transaction. Recommended only for small or fully-trusted deployments.

---

## §2. Configuration

### 2.1 Federation: one provider, one instance

Federation runs a single active provider at a time. No multi-provider composition. Config holds typed `provider` discriminator + one generic `instance` block + `common` cross-provider knobs. The factory registered under `provider` consumes whatever it needs from `instance`.

#### 2.1.1 Runtime interface

```go
// internal/auth/federation/provider.go
package federation

type Provider interface {
    Kind() string
    IssuerURL() string
    ValidateToken(ctx context.Context, raw string) (*Claims, error)
    HealthCheck(ctx context.Context) (Status, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type Claims struct {
    Subject   string
    Issuer    string
    Audience  []string
    Email     string
    Groups    []string
    Raw       map[string]any
    ExpiresAt time.Time
}

type Status struct {
    Reachable   bool
    LastChecked time.Time
    LastError   string
}
```

#### 2.1.2 Typed provider discriminator

```go
// internal/config/config.go
type FederationProvider string

const (
    FederationProviderNone FederationProvider = ""
    FederationProviderDex  FederationProvider = "dex"
    FederationProviderOIDC FederationProvider = "oidc"
)
```

Follows AgentLens's existing typed-enum pattern (`PartyKind`, `Protocol`, `LifecycleState`).

#### 2.1.3 Provider registry

```go
// internal/auth/federation/registry.go
package federation

type FactoryFunc func(instance InstanceConfig, common CommonConfig) (Provider, error)

var providerFactories = map[config.FederationProvider]FactoryFunc{}

func RegisterProvider(kind config.FederationProvider, factory FactoryFunc) {
    if _, exists := providerFactories[kind]; exists {
        panic(fmt.Sprintf("federation: provider %q already registered", kind))
    }
    providerFactories[kind] = factory
}

func BuildProvider(kind config.FederationProvider, instance InstanceConfig, common CommonConfig) (Provider, error) {
    f, ok := providerFactories[kind]
    if !ok {
        return nil, fmt.Errorf("federation: unknown provider %q (registered: %v)", kind, registeredKinds())
    }
    return f(instance, common)
}
```

Each provider package self-registers in `init()`:

```go
// internal/auth/federation/dex/dex.go
func init() {
    federation.RegisterProvider(config.FederationProviderDex, NewFromConfig)
}
func NewFromConfig(instance federation.InstanceConfig, common federation.CommonConfig) (federation.Provider, error) {
    if instance.IssuerURL == "" { return nil, errors.New("dex: issuer_url required") }
    return &Provider{...}, nil
}
```

### 2.2 Config types

```go
// internal/config/config.go
type FederationConfig struct {
    Provider FederationProvider       `yaml:"provider"`
    Common   FederationCommonConfig   `yaml:"common"`
    Instance FederationInstanceConfig `yaml:"instance"`
}

type FederationInstanceConfig struct {
    IssuerURL string            `yaml:"issuer_url"`
    JWKSURL   string            `yaml:"jwks_url,omitempty"`
    Options   map[string]string `yaml:"options,omitempty"` // provider-specific extras; each provider docs its keys
}

type FederationCommonConfig struct {
    UserIDClaim         string            `yaml:"user_id_claim"`          // default "sub"
    EmailClaim          string            `yaml:"email_claim"`            // default "email"
    GroupsClaim         string            `yaml:"groups_claim"`           // default "groups"
    RoleMapping         map[string]string `yaml:"role_mapping"`           // external-group → AgentLens role_id
    AutoProvisionUsers  bool              `yaml:"auto_provision_users"`   // default false
    DefaultRoleID       string            `yaml:"default_role_id"`        // required when AutoProvisionUsers=true
    AudiencePrefix      string            `yaml:"audience_prefix"`        // required
    JWKSCacheTTL        time.Duration     `yaml:"jwks_cache_ttl"`         // default 1h
    HealthCheckInterval time.Duration     `yaml:"health_check_interval"`  // default 30s
    HealthCheckTimeout  time.Duration     `yaml:"health_check_timeout"`   // default 5s
}
```

### 2.3 MCP server config

```go
type MCPServerConfig struct {
    Enabled         bool          `yaml:"enabled"`
    ListenPath      string        `yaml:"listen_path"`       // default "/api/mcp"
    ProtocolVersion string        `yaml:"protocol_version"`  // default "2025-11-25"; pinned
    MaxSessions     int           `yaml:"max_sessions"`      // default 1000
    SessionTTL      time.Duration `yaml:"session_ttl"`       // default 4h
    RequestTimeout  time.Duration `yaml:"request_timeout"`   // default 30s
    AuditEnabled    bool          `yaml:"audit_enabled"`     // default true
    CanonicalURL    string        `yaml:"canonical_url"`     // used as aud in token validation
}
```

### 2.4 Top-level `Config`

```go
type Config struct {
    // existing fields (Port, DataDir, LogLevel, LicenseKey, PollInterval,
    //                  Sources, Kubernetes, HealthCheck, Database, Auth, Telemetry)
    MCPServer  MCPServerConfig  `yaml:"mcp_server"`
    Federation FederationConfig `yaml:"federation"`
}
```

### 2.5 Startup build (composition root)

```go
// cmd/agentlens/main.go
var fedProvider federation.Provider
if cfg.Federation.Provider != config.FederationProviderNone {
    fedProvider, err = federation.BuildProvider(
        cfg.Federation.Provider,
        cfg.Federation.Instance,
        cfg.Federation.Common,
    )
    if err != nil { log.Fatalf("federation: %v", err) }
    if err := fedProvider.Start(ctx); err != nil { log.Fatalf("federation start: %v", err) }
    defer fedProvider.Stop(ctx)
}
// fedProvider → RouterDeps → RequireAuth picks it up
```

### 2.6 Environment variable overrides

| Env var | Maps to |
|---|---|
| `AGENTLENS_MCP_SERVER_ENABLED` / `_LISTEN_PATH` / `_PROTOCOL_VERSION` / `_MAX_SESSIONS` / `_SESSION_TTL` / `_REQUEST_TIMEOUT` / `_AUDIT_ENABLED` / `_CANONICAL_URL` | `mcp_server.*` |
| `AGENTLENS_FEDERATION_PROVIDER` | `federation.provider` |
| `AGENTLENS_FEDERATION_INSTANCE_ISSUER_URL` | `federation.instance.issuer_url` |
| `AGENTLENS_FEDERATION_INSTANCE_JWKS_URL` | `federation.instance.jwks_url` |
| `AGENTLENS_FEDERATION_INSTANCE_OPTIONS` | JSON map, e.g. `{"tenant_id":"acme"}` |
| `AGENTLENS_FEDERATION_COMMON_USER_ID_CLAIM` / `_EMAIL_CLAIM` / `_GROUPS_CLAIM` / `_AUTO_PROVISION_USERS` / `_DEFAULT_ROLE_ID` / `_AUDIENCE_PREFIX` / `_JWKS_CACHE_TTL` / `_HEALTH_CHECK_INTERVAL` / `_HEALTH_CHECK_TIMEOUT` | `federation.common.*` |
| `AGENTLENS_FEDERATION_COMMON_ROLE_MAPPING` | JSON map, e.g. `{"admins":"admin","developers":"developer"}` |

Parsing via existing `applyXxxEnv(cfg *XxxConfig)` helpers. Two new functions: `applyMCPServerEnv`, `applyFederationEnv`.

### 2.7 Validation (fail-fast on startup)

```go
func (c *Config) validateMCPServer() error {
    if !c.MCPServer.Enabled { return nil }
    if c.MCPServer.CanonicalURL == "" {
        return errors.New("mcp_server.canonical_url required when mcp_server.enabled")
    }
    if !isValidHTTPSURL(c.MCPServer.CanonicalURL) && !allowInsecureLocalhost(c.MCPServer.CanonicalURL) {
        return fmt.Errorf("mcp_server.canonical_url must be https:// (got %q)", c.MCPServer.CanonicalURL)
    }
    if c.MCPServer.MaxSessions < 1 {
        return errors.New("mcp_server.max_sessions must be >= 1")
    }
    if c.MCPServer.SessionTTL < time.Minute {
        return errors.New("mcp_server.session_ttl must be >= 1m")
    }
    return nil
}

func (c *Config) validateFederation() error {
    f := &c.Federation
    if f.Provider == FederationProviderNone { return nil }

    if f.Common.AudiencePrefix == "" {
        return errors.New("federation.common.audience_prefix required when federation.provider set")
    }
    if f.Common.AutoProvisionUsers && f.Common.DefaultRoleID == "" {
        return errors.New("federation.common.auto_provision_users=true requires default_role_id")
    }
    if f.Instance.IssuerURL == "" {
        return errors.New("federation.instance.issuer_url required when federation.provider set")
    }
    // Provider-kind validation is deferred to BuildProvider at startup (registry lookup).
    return nil
}
```

Both called from `Load()` before returning. Fail-fast prevents startup with half-configured auth.

### 2.8 Sample `agentlens.yaml`

```yaml
# Existing blocks...
port: 8080
data_dir: /var/lib/agentlens
log_level: info

auth:
  jwt_secret: ${AGENTLENS_AUTH_JWT_SECRET}
  session_duration: 24h

mcp_server:
  enabled: true
  canonical_url: https://agentlens.example.com/api/mcp
  listen_path: /api/mcp
  protocol_version: "2025-11-25"
  max_sessions: 1000
  session_ttl: 4h
  request_timeout: 30s
  audit_enabled: true

federation:
  provider: dex            # swap to "oidc" / "" without changing shape
  common:
    user_id_claim: sub
    email_claim: email
    groups_claim: groups
    auto_provision_users: false
    default_role_id: ""
    audience_prefix: https://agentlens.example.com/api/mcp
    jwks_cache_ttl: 1h
    health_check_interval: 30s
    health_check_timeout: 5s
    role_mapping:
      admins: admin
      developers: developer
  instance:
    issuer_url: http://agentlens-dex.agentlens.svc.cluster.local:5556/dex
    # jwks_url: ...
    # options:
    #   some_provider_flag: value
```

Switch providers: change `federation.provider: oidc` and update `federation.instance.issuer_url`. Nothing else moves.

### 2.9 Adding a new provider

1. New subpackage `internal/auth/federation/<kind>/`.
2. Implement `federation.Provider`.
3. Add the kind constant to `config.FederationProvider` enum.
4. Call `federation.RegisterProvider(config.FederationProvider<Kind>, NewFromConfig)` in package `init()`.
5. Document any `instance.options` keys in the package README.

No config schema change. No `Config` struct change. No MCP plugin change.

### 2.10 Docs

Update `docs/settings.md` with `mcp_server.*` + `federation.*` blocks. Include Dex and generic-OIDC examples side-by-side to demonstrate the abstraction. Per `standards/global/pr-checklist.md` item 6.

---

## §3. Authentication Flows

Three auth paths, one normalized `Principal` downstream. Federation JWTs, service-account API keys, local JWTs (existing basic-auth users).

### 3.1 Principal model

```go
// internal/auth/principal.go
package auth

type PrincipalKind string

const (
    PrincipalKindUser           PrincipalKind = "user"
    PrincipalKindServiceAccount PrincipalKind = "service_account"
)

type Principal struct {
    Kind                 PrincipalKind
    ID                   string      // users.id or parties.id (service account)
    PartyID              string      // always populated; for users this is their person-party
    Username             string      // users.username, or "" for service accounts
    Label                string      // service-account label or user display_name
    Permissions          []string    // effective permissions (global + project-scoped, resolved)
    AccessibleProjectIDs []string    // union of memberships + default project
    AuthMethod           string      // "basic_jwt" | "federation:dex" | "federation:oidc" | "api_key"
    CredentialID         string      // api_client_credentials.id (SA) or jti (JWT)
    IssuedAt             time.Time
    ExpiresAt            time.Time   // zero = no expiry (API keys)
}
```

Context keys (extend existing):

```go
// internal/api/auth_middleware.go
const (
    ctxUserID      ctxKey = "user_id"       // existing; populated from Principal.ID when kind=user
    ctxUsername    ctxKey = "username"      // existing
    ctxRoleID      ctxKey = "role_id"       // existing
    ctxPermissions ctxKey = "permissions"   // existing
    ctxPrincipal   ctxKey = "principal"     // NEW — full *Principal
)
```

Existing handlers that read old keys keep working. MCP + audit paths read `ctxPrincipal`.

### 3.2 Auth method dispatch

`RequireAuth` dispatches by token prefix:

- Starts with `agentlens_sk_` → service-account API key (§3.3).
- JWT shape (three dot-sep base64 parts) → parse header, check `iss`:
  - `iss` == local JWT issuer → basic-auth user flow (§3.4).
  - `iss` == `Federation.Instance.IssuerURL` AND federation enabled → federation flow (§3.5).
  - else → `401 invalid_token`.

Federation disabled + JWT with non-local `iss` → `401` with `WWW-Authenticate: Bearer realm="agentlens"` (no resource_metadata hint).

### 3.3 Service-account API-key flow

Token format: `Authorization: Bearer agentlens_sk_<credential_id>.<secret>`.

1. Parse — split on first `.`; fail 401 if format wrong.
2. Lookup — `SELECT * FROM api_client_credentials WHERE id = ?`. Miss → 401.
3. Status — `revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`. Fail 401.
4. Compare — `bcrypt.CompareHashAndPassword(row.SecretHash, []byte(secret))`. Always run against a dummy hash on miss to preserve timing.
5. Fetch party — `parties WHERE id = party_id AND kind = 'service_account'`. Missing → 500 (integrity).
6. Resolve permissions via `auth.ResolveServiceAccountPermissions(ctx, partyID)` (§4).
7. Resolve accessible projects — `PartyRelationship` union + default project.
8. Build `Principal{Kind=ServiceAccount, AuthMethod="api_key", CredentialID=<credential_id>}`.
9. Async `last_used_at` update — buffered channel, 30s flush interval, single UPDATE.
10. Permit.

Error bodies:

- Malformed / unknown / wrong secret → `401`, `{"error":"invalid credentials"}`.
- Revoked → `401`, `{"error":"credential revoked"}`.
- Expired → `401`, `{"error":"credential expired"}`.

### 3.4 Local JWT flow

Unchanged today. `RequireAuth` parses JWT, verifies signature with `auth.JWTService.secret`, extracts `{UserID, Username, RoleID, Permissions}`. New: wrap in `Principal{Kind=User, AuthMethod="basic_jwt"}`; resolve `AccessibleProjectIDs` uniformly in middleware (previously done on-demand in handlers).

### 3.5 Federation JWT flow

Triggered when: `iss` matches `Federation.Instance.IssuerURL` AND provider enabled.

Validation (all MUST per MCP 2025-11-25):

1. Parse header — extract `kid`; fail 401 if missing.
2. JWKS lookup — cache refreshed every `Common.JWKSCacheTTL`; on miss force refresh once; fail 401 if still missing.
3. Verify signature with JWK.
4. Claims validate:
   - `iss` equals `Federation.Instance.IssuerURL` (exact).
   - `aud` contains entry starting with `Federation.Common.AudiencePrefix` (literal prefix match, no wildcard).
   - `exp` > now.
   - `nbf` ≤ now (if present).
   - `iat` ≤ now (reject future tokens).
5. Resolve user — `user_external_identities WHERE provider=<configured> AND external_iss=<iss> AND external_sub=<sub>`.
   - Row found → fetch `users` → proceed.
   - Row missing + `AutoProvisionUsers=true` → create `users` + `user_external_identities` in one transaction, assign `DefaultRoleID`, proceed.
   - Row missing + `AutoProvisionUsers=false` → `403`, `{"error":"user not provisioned","hint":"contact admin"}`. Audit log entry with `external_iss`, `external_sub`, `email` for admin-approval queue.
6. Group/role mapping (if `Common.RoleMapping` non-empty and token has `Common.GroupsClaim`):
   - First match wins (sorted by key).
   - Override `users.role_id` in-memory for this request only (no persist in v1; persist flag deferred to v1.5 if requested).
7. Resolve permissions and accessible projects (§4).
8. Build `Principal{Kind=User, AuthMethod="federation:<kind>", CredentialID=<jti>}`.
9. Async `last_seen_at` update — same buffer pattern as §3.3.
10. Permit.

Error responses spec-compliant: `401` + `WWW-Authenticate: Bearer error="invalid_token", error_description="…", resource_metadata="…"`.

### 3.6 Protected Resource Metadata

MCP clients hitting `/api/mcp` without token get:

```http
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Bearer resource_metadata="https://agentlens.example.com/.well-known/oauth-protected-resource", scope="mcp:discovery"
```

The metadata endpoint (served by MCP plugin, §5):

```json
{
  "resource": "https://agentlens.example.com/api/mcp",
  "authorization_servers": ["http://agentlens-dex.agentlens.svc.cluster.local:5556/dex"],
  "scopes_supported": ["mcp:discovery"],
  "bearer_methods_supported": ["header"]
}
```

`authorization_servers[0]` pulled from `Federation.Provider.IssuerURL()` at plugin init. If federation disabled, endpoint returns `404`.

### 3.7 Scope challenges (403)

When `RequirePermission` denies on the MCP path:

```http
HTTP/1.1 403 Forbidden
WWW-Authenticate: Bearer error="insufficient_scope", scope="catalog:read"
Content-Type: application/json

{"error":"insufficient_scope","scope":"catalog:read"}
```

Loopback adapter in MCP plugin catches 403, extracts `scope`, translates into MCP tool-execution error per SEP-1303.

### 3.8 Session identity ≠ auth identity

MCP's `MCP-Session-Id` is NOT an auth artifact (spec: sessions MUST NOT be used for authentication). Every JSON-RPC message on MCP transport:

1. Must include `Authorization: Bearer …` — revalidated per request.
2. Server-side session state keyed by `<principal_id>:<session_id>` — prevents session-confusion.
3. Token expiry mid-session → next request 401; client re-auths. Session ID remains valid for reconnection with fresh token until session TTL.

### 3.9 Audit per auth event

```go
slog.InfoContext(ctx, "auth.success",
    "principal_id", p.ID,
    "principal_kind", p.Kind,
    "auth_method", p.AuthMethod,
    "credential_id", p.CredentialID,
    "request_id", middleware.GetReqID(ctx),
    "remote_addr", r.RemoteAddr,
)
```

Failed auth → `auth.failure` with `reason` (`"invalid_credentials"`, `"token_expired"`, `"insufficient_scope"`, etc.). Never logs token or secret.

### 3.10 Timing safety

All flows constant-time where identity is a secret:

- API-key unknown → bcrypt against dummy hash.
- JWT signature invalid → JWT library already constant-time.
- Federation JWT → same.

Invalid token and unknown principal return in same wall-clock band.

---

## §4. Authorization Model

### 4.1 Permission resolution per principal kind

**Users** (local or federated): `users.role_id` → `roles.permissions[]`. Federated users may temporarily override `role_id` via `Federation.Common.RoleMapping` (in-memory only, per-request; no persist in v1).

**Service accounts**: no `role_id` on the party row. Permissions come only from `PartyRelationship` edges to projects.

```go
// internal/auth/party_permissions.go
func ResolveServiceAccountPermissions(ctx context.Context, partyID string) ([]string, error) {
    // SELECT pr.to_role FROM party_relationships pr
    // WHERE pr.from_party_id = ? AND pr.relationship_name = 'member_of'
    //   AND EXISTS (SELECT 1 FROM parties WHERE id = pr.to_party_id AND kind = 'project')
    //
    // For each distinct to_role ("project:viewer"|"developer"|"owner"),
    // map via projectRolePermissions (existing) and UNION.
    // Return dedup []string.
}
```

Service accounts with no memberships → empty permission set. Default-project access is orthogonal (§4.3).

**Principle**: `Principal.Permissions` carries UNION of baseline + project-scoped. Computed once per request; `RequirePermission` is membership check.

### 4.2 Accessible project resolution

```go
func ResolveAccessibleProjects(ctx context.Context, partyID string) ([]string, error) {
    // 1. SELECT to_party_id FROM party_relationships
    //    WHERE from_party_id = ? AND relationship_name = 'member_of'
    //    AND EXISTS (SELECT 1 FROM parties p WHERE p.id = to_party_id AND p.kind = 'project')
    // 2. Append default project ID (cached in-memory at startup)
    // 3. Return dedup []string
}
```

Users → through their `person` party. Service accounts → through their `service_account` party. Same code path.

### 4.3 Default project: public-reads rule

The system default project (`parties.kind='project' AND is_system=true`) is readable by every authenticated principal regardless of explicit membership. Implemented via `ResolveAccessibleProjects` always appending the default project ID.

- Reads: default project's catalog entries visible to everyone authenticated.
- Writes: still permission-gated via `catalog:write` etc.
- Unauthenticated: still denied.

Not an ACL bypass — membership synthesis. Keeps authorization code uniform.

### 4.4 Project-scope filtering in read paths

#### 4.4.1 New middleware: `ScopeByAccessibleProjects`

```go
// internal/api/auth_middleware.go
func ScopeByAccessibleProjects(store store.Store) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            p, ok := auth.PrincipalFromContext(r.Context())
            if !ok {
                next.ServeHTTP(w, r)
                return
            }
            q := r.URL.Query()
            if requested := q.Get("project"); requested != "" {
                if !contains(p.AccessibleProjectIDs, requested) {
                    ErrorResponse(w, http.StatusForbidden, "project not accessible")
                    return
                }
            } else {
                q.Set("projects", strings.Join(p.AccessibleProjectIDs, ","))
                r.URL.RawQuery = q.Encode()
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

Applied to: catalog list/get/card, capabilities list/get, stats. Mounted via `r.With(ScopeByAccessibleProjects(store))` in the authenticated read-route group.

#### 4.4.2 Store-layer filter

```go
type CatalogFilter struct {
    // existing: Protocol, Status, Source, Provider, TextSearch, Limit, Offset, ProjectID
    ProjectIDs []string  // NEW: when non-empty, entries must be in at least one of these projects
}
```

Query change:

```sql
WHERE EXISTS (
    SELECT 1 FROM catalog_project_memberships cpm
    WHERE cpm.catalog_entry_id = ce.id
      AND cpm.project_party_id IN (?)
)
```

GORM `IN` binding parameterized — SQLite + PostgreSQL handle uniformly.

### 4.5 Permission constants (existing + 3 new)

Existing constants unchanged:

```go
PermCatalogRead, PermCatalogWrite, PermCatalogDelete
PermUsersRead,   PermUsersWrite,   PermUsersDelete
PermRolesRead,   PermRolesWrite
PermSettingsRead, PermSettingsWrite
```

New:

```go
PermServiceAccountsRead   = "service_accounts:read"
PermServiceAccountsWrite  = "service_accounts:write"
PermServiceAccountsDelete = "service_accounts:delete"
```

Seeded on `admin` system role at migration 008. Other system roles get no default service-account perms.

### 4.6 Permission requirements for MCP discovery tools

| Tool | Permission |
|---|---|
| `agent_search` | `catalog:read` |
| `agent_get` | `catalog:read` |
| `capabilities_list` | `catalog:read` |
| `agent_card` | `catalog:read` |

All four map directly to existing REST endpoints requiring `catalog:read`. Loopback reuses middleware — no permission re-wiring.

### 4.7 Default role assignments

| Principal created via | Default role/perms |
|---|---|
| Basic-auth user (admin bootstrap) | existing `admin` role |
| Basic-auth user (UI-created) | admin-selected from existing roles |
| Federated user (JIT) | `Federation.Common.DefaultRoleID` (required by config validation) |
| Service account (admin-created) | no role; project memberships are only permission source |

### 4.8 Authorization decision order (per request)

```
1. Authenticated?      → §3; 401 if not
2. Project scope?      → §4.4; 403 if requested project not accessible
3. Permission check?   → RequirePermission; 403 + scope challenge
4. Resource exists?    → handler; 404 if not
5. Execute             → handler business logic
```

Middleware order locked at route registration. MCP tools hit the same stack via HTTP loopback.

### 4.9 No project-scoped admin in v1

Existing `project:viewer|developer|owner` roles reused for service accounts. No new project-role kinds. `users:*`, `roles:*`, `settings:*`, `service_accounts:*` are GLOBAL — admin is admin across all projects. Project-scoped admin deferred to v1.5+.

---

## §5. MCP Plugin & Wire Protocol

### 5.1 Plugin package layout

```
plugins/mcpserver/
├── plugin.go           // Plugin struct; Init/Start/Stop; satisfies kernel.Plugin
├── config.go           // plugin-local config access
├── wire/
│   ├── impl.go         // WireImpl interface; registry
│   └── diy/
│       ├── transport.go    // Streamable HTTP handler (POST + GET)
│       ├── session.go      // session store + TTL reaper
│       ├── jsonrpc.go      // JSON-RPC 2.0 framing + error codes
│       ├── handlers.go     // initialize, ping, tools/list, tools/call, notifications
│       └── sse.go          // SSE event framing + Last-Event-ID resume
├── tool_registry.go    // ToolRegistry interface + default impl
├── loopback.go         // HTTP loopback adapter
├── errors.go           // HTTP status → MCP error code mapping
├── status.go           // /api/mcp/status handler
├── prm.go              // /.well-known/oauth-protected-resource handler
├── register.go         // self-registration into catalog at Init
└── mcpserver_test.go   // integration tests
```

Matches `plugins/health/` shape. Arch-go naming rule: exported struct implementing `kernel.Plugin` → `Plugin` suffix → `plugins/mcpserver.Plugin`.

### 5.2 `kernel.Plugin` lifecycle

```go
// plugins/mcpserver/plugin.go
type Plugin struct {
    cfg         config.MCPServerConfig
    fedProvider federation.Provider  // nil when federation disabled
    store       store.Store
    auditLog    *telemetry.AuditLogger
    registry    ToolRegistry
    wire        wire.Impl
    sessions    *wire.SessionStore
}

func New(cfg config.MCPServerConfig, fed federation.Provider, s store.Store, audit *telemetry.AuditLogger) *Plugin {
    return &Plugin{cfg: cfg, fedProvider: fed, store: s, auditLog: audit}
}

func (p *Plugin) Name() string             { return "mcpserver" }
func (p *Plugin) Version() string          { return "1.0.0" }
func (p *Plugin) Type() kernel.PluginType  { return kernel.PluginTypeMiddleware }

func (p *Plugin) Init(k kernel.Kernel) error {
    if !p.cfg.Enabled { return nil }  // no-op disable
    impl, err := wire.BuildImpl("diy", wire.ImplConfig{
        ProtocolVersion: p.cfg.ProtocolVersion,
        SessionTTL:      p.cfg.SessionTTL,
        MaxSessions:     p.cfg.MaxSessions,
    })
    if err != nil { return fmt.Errorf("mcpserver: %w", err) }
    p.wire = impl
    p.sessions = wire.NewSessionStore(p.cfg.MaxSessions, p.cfg.SessionTTL)
    p.registry = NewToolRegistry()
    if err := p.registerDiscoveryTools(k); err != nil { return err }
    k.RegisterRoutes(p.cfg.ListenPath, p.buildHTTPHandler())
    k.RegisterRoutes("/.well-known/oauth-protected-resource", p.buildPRMHandler())
    k.RegisterRoutes(p.cfg.ListenPath+"/status", p.buildStatusHandler())
    if err := p.selfRegister(ctx, p.store); err != nil {
        k.Logger().With("component", "mcpserver").WarnContext(ctx, "self-registration failed", "err", err)
        // Non-fatal: server runs without self-registration
    }
    return nil
}

func (p *Plugin) Start(ctx context.Context) error {
    if !p.cfg.Enabled { return nil }
    go p.sessions.ReaperLoop(ctx)
    return nil
}

func (p *Plugin) Stop(ctx context.Context) error {
    if !p.cfg.Enabled { return nil }
    p.sessions.Close()
    return p.markSelfOffline(ctx)
}
```

### 5.3 `WireImpl` interface

```go
// plugins/mcpserver/wire/impl.go
type ImplConfig struct {
    ProtocolVersion string
    SessionTTL      time.Duration
    MaxSessions     int
}

type Impl interface {
    Kind() string
    ServeHTTP(w http.ResponseWriter, r *http.Request, deps Dependencies)
    ProtocolVersion() string
}

type Dependencies struct {
    Sessions *SessionStore
    Registry ToolRegistry
    AuditLog *telemetry.AuditLogger
    Loopback LoopbackFunc
}

type LoopbackFunc func(ctx context.Context, method, path string, query url.Values, bearer string) (*LoopbackResponse, error)

type LoopbackResponse struct {
    StatusCode int
    Headers    http.Header
    Body       []byte
}
```

Registry pattern mirrors federation:

```go
var implFactories = map[string]ImplFactoryFunc{}

type ImplFactoryFunc func(cfg ImplConfig) (Impl, error)

func RegisterImpl(kind string, factory ImplFactoryFunc) { ... }
func BuildImpl(kind string, cfg ImplConfig) (Impl, error) { ... }
```

DIY impl self-registers in `wire/diy/diy.go`:

```go
func init() {
    wire.RegisterImpl("diy", func(cfg wire.ImplConfig) (wire.Impl, error) {
        return &DIYImpl{cfg: cfg}, nil
    })
}
```

### 5.4 Streamable HTTP transport

Single endpoint `POST + GET /api/mcp`.

#### 5.4.1 POST (client → server JSON-RPC)

Required headers:

- `Content-Type: application/json`
- `Accept: application/json, text/event-stream`
- `Authorization: Bearer <token>`
- `MCP-Protocol-Version: 2025-11-25` (invalid → 400)
- `Origin: <client-origin>` (invalid → 403)
- `MCP-Session-Id: <uuid>` (present after initialize; missing on initialize)

v1 always responds `Content-Type: application/json` (single response). `text/event-stream` streaming reserved for future.

#### 5.4.2 GET (server → client SSE)

- `Accept: text/event-stream`
- `Authorization: Bearer <token>` (validated every GET)
- `MCP-Session-Id: <uuid>`
- `Last-Event-ID: <id>` (resume)

Server replies `Content-Type: text/event-stream`, monotonic integer event IDs, may close TCP + send `retry: 5000` for client reconnect.

#### 5.4.3 Spec-mandated per-request checks

```go
func (d *DIYImpl) ServeHTTP(w http.ResponseWriter, r *http.Request, deps Dependencies) {
    // 1. Origin validation
    if !isValidOrigin(r.Header.Get("Origin")) {
        http.Error(w, "invalid origin", http.StatusForbidden)
        return
    }
    // 2. Protocol version echo
    if v := r.Header.Get("MCP-Protocol-Version"); v != "" && v != d.cfg.ProtocolVersion {
        http.Error(w, "unsupported MCP-Protocol-Version", http.StatusBadRequest)
        return
    }
    // 3. Auth — done by RequireAuth middleware; Principal in context
    // 4. Session — for non-initialize methods, require MCP-Session-Id
    // ... method dispatch ...
}
```

### 5.5 Session management

```go
// plugins/mcpserver/wire/session.go
type Session struct {
    ID            string
    PrincipalID   string
    PrincipalKind auth.PrincipalKind
    CreatedAt     time.Time
    LastActivity  time.Time
    EventID       uint64
}

type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session
    maxSize  int
    ttl      time.Duration
}

func (s *SessionStore) Create(principal *auth.Principal) (*Session, error) { ... }
func (s *SessionStore) Get(sessionID, principalID string) (*Session, error) { ... }  // verifies principalID match
func (s *SessionStore) ReaperLoop(ctx context.Context) { ... }  // 60s tick
```

Server-side state keyed by `<PrincipalID>:<SessionID>` — prevents cross-principal session reuse on leaked IDs.

### 5.6 JSON-RPC handlers (v1 scope)

| Method | Notes |
|---|---|
| `initialize` | Handshake. Server returns matched `protocolVersion`, `serverInfo`, `capabilities: {tools:{listChanged:true}}`. Assigns `MCP-Session-Id` in response header. v1: `capabilities.resources=nil`, `capabilities.prompts=nil`. |
| `ping` | Empty round-trip. Returns `{}`. |
| `tools/list` | `{tools: [...]}` from ToolRegistry, each `{name, description, inputSchema}`. Sorted by name for determinism. |
| `tools/call` | Input: `{name, arguments}`. Output: `{content: [{type:"text", text:"..."}]}` or `{isError:true, content:[...]}`. §6 specs per-tool shapes. |
| `notifications/initialized` | Client → server, no response. Marks initialize complete. |

Out of scope: `resources/*`, `prompts/*`, `sampling/*`, `roots/*`, `elicitation/*`, `logging/setLevel`. Return `-32601 Method not found` with `data: {"supported": ["initialize", "ping", "tools/list", "tools/call", "notifications/initialized"]}`.

### 5.7 Tool-call execution path

```
tools/call { name: "agent_search", arguments: {...} }
   │
   ▼
  ToolRegistry.Lookup("agent_search") → {InputSchema, HTTPMethod, HTTPPath, ArgumentMapper}
   │
   ▼
  Validate arguments against InputSchema (fail → -32602 Invalid params)
   │
   ▼
  ArgumentMapper(arguments) → (method, path, query, body)
   │
   ▼
  Loopback:
    1. Build http.Request with principal's bearer token
    2. httptest.NewRecorder()
    3. chiRouter.ServeHTTP(recorder, req)
    4. Extract recorder.Code + recorder.Body
   │
   ▼
  Map HTTP result → MCP result (see §5.9)
   │
   ▼
  Emit OTel span + audit log entry (§7)
   │
   ▼
  Serialize and return to client
```

### 5.8 Bearer token reconstruction for loopback

**Option A (chosen)**: pass original `Authorization: Bearer …` header through. `RequireAuth` revalidates on loopback. Overhead = one JWT verify or one bcrypt compare per tool call (<1ms); trivially correct.

**Option B (rejected)**: sentinel + Principal pointer in context, `RequireAuth` skips validation. Faster but introduces an auth-bypass mechanism — larger attack surface. Not worth the microseconds.

### 5.9 Error code mapping

```go
// plugins/mcpserver/errors.go
type ErrorMapping struct {
    HTTPStatus  int
    JSONRPCCode int
    ResultShape string  // "tool_error" | "jsonrpc_error"
}

var defaultMappings = []ErrorMapping{
    {200, 0, ""},
    {400, -32602, "jsonrpc_error"},  // Invalid params
    {401, -32000, "jsonrpc_error"},  // Unauthorized
    {403, -32001, "jsonrpc_error"},  // Forbidden / insufficient_scope (data: {scope})
    {404, 0, "tool_error"},          // Not found → tool error
    {409, 0, "tool_error"},          // Conflict → tool error
    {422, 0, "tool_error"},          // Validation → tool error
    {429, -32002, "jsonrpc_error"},  // Rate-limited
    {500, -32603, "jsonrpc_error"},  // Internal error
    {503, -32603, "jsonrpc_error"},  // Service unavailable
}
```

- `tool_error` = business-domain, LLM might correct by retrying with different args.
- `jsonrpc_error` = protocol/auth/infra, LLM cannot fix by changing args.

Aligns with SEP-1303.

### 5.10 `/api/mcp/status` endpoint

```http
GET /api/mcp/status
Authorization: Bearer ...   (optional; authenticated = more detail)

Response:
{
  "plugin": {
    "enabled": true,
    "version": "1.0.0",
    "protocol_version": "2025-11-25",
    "wire_impl": "diy"
  },
  "sessions": {
    "active": 42,
    "max": 1000
  },
  "tools": {
    "registered": 4,
    "names": ["agent_search", "agent_get", "capabilities_list", "agent_card"]
  },
  "federation": {
    "enabled": true,
    "provider": "dex",
    "reachable": true,
    "last_checked": "2026-04-17T16:07:27Z"
  },
  "self_registered": {
    "catalog_entry_id": "mcp-discovery-uuid",
    "in_default_project": true
  }
}
```

Unauthenticated callers get subset (no session counts, no self_registered.catalog_entry_id).

### 5.11 Testing requirements

- **Unit**: each JSON-RPC handler with table-driven cases.
- **Integration**: real chi router + real SQLite + mocked federation provider. Verify: initialize handshake, 4 tool calls, auth (API key + federated JWT), error mapping per code, session TTL expiry, session hijacking rejection, Origin 403, protocol-version echo, Protected Resource Metadata.
- **E2E**: Go MCP client integration test in `e2e/mcp/` (JSON-RPC client, full tool-call sequence). Not Playwright — MCP is not a browser surface.

---

## §6. ToolRegistry & 4 Tool Specs

### 6.1 ToolRegistry interface

```go
// plugins/mcpserver/tool_registry.go
type ToolEntry struct {
    Name           string           // SEP-986: snake_case, [a-z0-9_]+
    Description    string           // LLM-facing
    InputSchema    JSONSchema       // JSON Schema 2020-12
    HTTPMethod     string           // "GET" (v1 always)
    HTTPPath       string           // template with {param} placeholders
    ArgumentMapper ArgumentMapperFunc
    ResponseShaper ResponseShaperFunc
    Annotations    ToolAnnotations
}

type ArgumentMapperFunc func(args map[string]any) (url.Values, map[string]string, []byte, error)
type ResponseShaperFunc func(body []byte, status int) ([]MCPContent, bool, error)

type ToolAnnotations struct {
    ReadOnlyHint    bool
    DestructiveHint bool
    IdempotentHint  bool
}

type ToolRegistry interface {
    Register(entry ToolEntry) error
    Lookup(name string) (ToolEntry, bool)
    List() []ToolEntry  // sorted by Name (deterministic)
    Count() int
}

type JSONSchema struct {
    Type        string                    `json:"type"`
    Properties  map[string]JSONSchemaProp `json:"properties"`
    Required    []string                  `json:"required,omitempty"`
    Description string                    `json:"description,omitempty"`
    Schema      string                    `json:"$schema"`  // "https://json-schema.org/draft/2020-12/schema"
}

type JSONSchemaProp struct {
    Type        string          `json:"type"`
    Description string          `json:"description,omitempty"`
    Enum        []any           `json:"enum,omitempty"`
    Default     any             `json:"default,omitempty"`
    Minimum     *int            `json:"minimum,omitempty"`
    Maximum     *int            `json:"maximum,omitempty"`
    Format      string          `json:"format,omitempty"`
    Items       *JSONSchemaProp `json:"items,omitempty"`
}
```

`Register` enforces: unique Name, SEP-986 format, non-empty Description, schema Properties-Required key consistency.

### 6.2 Tool 1: `agent_search`

Maps to `GET /api/v1/catalog?q=<text>&protocol=<p>&status=<s>&limit=<n>&offset=<o>` (accessible-project scope injected by middleware).

Input schema:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "query":    {"type":"string","description":"Free-text search across agent display name, description, and capability names. Case-insensitive substring match. Empty string lists all accessible agents."},
    "protocol": {"type":"string","enum":["a2a","mcp","a2ui"],"description":"Filter by protocol. Omit to include all."},
    "status":   {"type":"string","enum":["registered","active","degraded","offline","deprecated"],"description":"Filter by lifecycle state."},
    "limit":    {"type":"integer","minimum":1,"maximum":100,"default":20},
    "offset":   {"type":"integer","minimum":0,"default":0}
  },
  "required": []
}
```

LLM-facing description:

> Search the AgentLens catalog for AI agents by free-text query, protocol, or lifecycle status. Returns up to 100 agents per call, each with their display name, protocol, endpoint, capabilities summary, and current health state. Use this as the starting point for discovering what agents are available — follow up with `agent_get` for full details on a specific agent, or `capabilities_list` to browse capability types.

ArgumentMapper: standard query-param mapping; integer casts from JSON numbers (float64). ResponseShaper: passthrough (body → single text content).

### 6.3 Tool 2: `agent_get`

Maps to `GET /api/v1/catalog/{id}` (ID path) OR `GET /api/v1/catalog?endpoint=<url>&limit=1` (endpoint lookup).

Input schema:

```json
{
  "type": "object",
  "properties": {
    "id":       {"type":"string","format":"uuid","description":"AgentLens catalog entry ID. Takes precedence over 'endpoint' if both provided."},
    "endpoint": {"type":"string","format":"uri","description":"Agent endpoint URL. Used when id unknown."}
  }
}
```

Constraint: at least one of `id` or `endpoint` required (enforced in ArgumentMapper).

LLM-facing description:

> Fetch the full detail for one agent in the AgentLens catalog. Provide either an `id` (from `agent_search` results) or an `endpoint` URL. Returns the agent's full profile: display name, protocol, endpoint, version, all capabilities with their schemas, lifecycle state, health metrics, and metadata. Use this after `agent_search` when you need complete info for one specific agent.

ResponseShaper: 404 on ID lookup → tool_error; empty array on endpoint lookup → tool_error with "agent not found for endpoint X" text.

### 6.4 Tool 3: `capabilities_list`

Maps to `GET /api/v1/capabilities?kind=<k>&q=<text>&limit=<n>&offset=<o>`.

Input schema:

```json
{
  "type": "object",
  "properties": {
    "kind":   {"type":"string","enum":["mcp.tool","mcp.resource","mcp.prompt","a2a.skill"],"description":"Filter to one capability kind. Omit to return all."},
    "query":  {"type":"string","description":"Free-text search across capability name and description."},
    "limit":  {"type":"integer","minimum":1,"maximum":100,"default":50},
    "offset": {"type":"integer","minimum":0,"default":0}
  }
}
```

LLM-facing description:

> List capabilities registered across agents in your accessible projects. Each capability is a discrete thing an agent can do: an MCP tool it exposes, an A2A skill, a prompt template. Filter by `kind` to focus on one category, or by `query` to search across names and descriptions. Each result includes the parent agent's ID so you can follow up with `agent_get`. Use this when you want to answer "is there any agent that can do X" — capability-first discovery is often more direct than agent-first.

### 6.5 Tool 4: `agent_card`

Maps to `GET /api/v1/catalog/{id}/card`.

Input schema:

```json
{
  "type": "object",
  "properties": {
    "id": {"type":"string","format":"uuid","description":"AgentLens catalog entry ID."}
  },
  "required": ["id"]
}
```

LLM-facing description:

> Fetch the raw protocol card (as JSON) for an agent. This is the full protocol-specific document the agent publishes (MCP server card, A2A agent card, etc.). Use this when you need to parse an agent's native spec — tool schemas, resource URIs, skill definitions — beyond AgentLens's normalized capability summary.

ResponseShaper: card endpoint returns raw bytes; pass through verbatim as text content.

### 6.6 Tool registration at plugin Init

```go
// plugins/mcpserver/register.go
func (p *Plugin) registerDiscoveryTools(k kernel.Kernel) error {
    tools := []ToolEntry{
        {Name:"agent_search",      Description:"Search the AgentLens catalog...",   InputSchema:agentSearchSchema,     HTTPMethod:"GET", HTTPPath:"/api/v1/catalog",              ArgumentMapper:searchMapper, ResponseShaper:passthroughShaper, Annotations:ToolAnnotations{ReadOnlyHint:true, IdempotentHint:true}},
        {Name:"agent_get",         Description:"Fetch the full detail...",           InputSchema:agentGetSchema,        HTTPMethod:"GET", HTTPPath:"/api/v1/catalog/{id}",          ArgumentMapper:getMapper,    ResponseShaper:getShaper,          Annotations:ToolAnnotations{ReadOnlyHint:true, IdempotentHint:true}},
        {Name:"capabilities_list", Description:"List capabilities...",               InputSchema:capabilitiesListSchema,HTTPMethod:"GET", HTTPPath:"/api/v1/capabilities",          ArgumentMapper:capsListMapper,ResponseShaper:passthroughShaper, Annotations:ToolAnnotations{ReadOnlyHint:true, IdempotentHint:true}},
        {Name:"agent_card",        Description:"Fetch the raw protocol card...",     InputSchema:agentCardSchema,       HTTPMethod:"GET", HTTPPath:"/api/v1/catalog/{id}/card",     ArgumentMapper:cardMapper,   ResponseShaper:cardShaper,        Annotations:ToolAnnotations{ReadOnlyHint:true, IdempotentHint:true}},
    }
    for _, t := range tools {
        if err := p.registry.Register(t); err != nil {
            return fmt.Errorf("registering %s: %w", t.Name, err)
        }
    }
    return nil
}
```

### 6.7 v2 translator compatibility

When v2 OpenAPI-to-MCP translator ships:

- `Name` from `operationId` (normalized to snake_case)
- `Description` from operation `description` + `summary`
- `InputSchema` derived from `parameters` + `requestBody`
- `HTTPMethod`, `HTTPPath` from operation position
- `ArgumentMapper` generated from parameter locations
- `ResponseShaper` defaults to passthrough; customized via OpenAPI extension `x-mcp-response`

v1 `registerDiscoveryTools` body disappears; translator registers same slice from spec.

### 6.8 Content type support

v1 tools return `MCPContent{Type:"text"}` only. Image and binary deferred.

### 6.9 Rate limiting per tool

v1: inherit REST rate limits (none beyond probe). Per-tool limits deferred to v1.5+.

### 6.10 Tool spec tests

Per tool: schema-validation test, mapper test, shaper test, loopback integration test. ~20 tests total across 4 tools.

---

## §7. Self-Registration & Observability

### 7.1 Self-registration at Init

```go
// plugins/mcpserver/register.go
const (
    selfProtocol = "mcp"
    selfEndpoint = "agentlens:mcp-discovery"
    selfVersion  = "1.0.0"
)

func (p *Plugin) selfRegister(ctx context.Context, s store.Store) error {
    agentKey := model.ComputeAgentKey(selfProtocol, selfEndpoint)
    caps := make([]model.Capability, 0, p.registry.Count())
    for _, t := range p.registry.List() {
        caps = append(caps, model.MCPTool{
            Name:        t.Name,
            Description: t.Description,
            InputSchema: t.InputSchema.Raw(),
        })
    }
    now := time.Now().UTC()
    agentType := &model.AgentType{
        ID:           uuid.NewString(),
        AgentKey:     agentKey,
        Protocol:     model.ProtocolMCP,
        Endpoint:     selfEndpoint,
        Version:      selfVersion,
        SpecVersion:  p.cfg.ProtocolVersion,
        Capabilities: caps,
    }
    entry := &model.CatalogEntry{
        ID:          uuid.NewString(),
        AgentTypeID: agentType.ID,
        DisplayName: "AgentLens MCP Discovery Server",
        Description: "Self-registered MCP server exposing AgentLens's own catalog as MCP tools.",
        Status:      model.LifecycleActive,
        Source:      model.SourcePush,
        CreatedAt:   now,
        UpdatedAt:   now,
    }
    if existing, _ := s.FindByEndpoint(ctx, selfEndpoint); existing != nil {
        entry.ID = existing.ID
        agentType.ID = existing.AgentTypeID
        return s.UpdateEntry(ctx, entry, agentType)
    }
    return s.Create(ctx, entry, agentType)
    // SQLStore.Create() auto-assigns to default project when PartyStore is set
}

func (p *Plugin) markSelfOffline(ctx context.Context) error {
    existing, err := p.store.FindByEndpoint(ctx, selfEndpoint)
    if err != nil || existing == nil { return nil }
    existing.Status = model.LifecycleOffline
    existing.UpdatedAt = time.Now().UTC()
    return p.store.UpdateEntry(ctx, existing, nil)
}
```

Non-fatal at Init — store unavailable → plugin logs warning, continues. Idempotent via `AgentKey` + UNIQUE endpoint constraint.

### 7.2 Tool-surface changes → catalog sync

Capabilities list rebuilt every Init → restart-to-update (consistent with other plugins). `notifications/tools/list_changed` for live updates is out of v1 scope; v1 tools are static.

### 7.3 OTel spans

```go
// plugins/mcpserver/wire/diy/handlers.go
func (d *DIYImpl) handleToolCall(ctx context.Context, req *JSONRPCRequest, deps wire.Dependencies) *JSONRPCResponse {
    tracer := otel.Tracer("agentlens/mcpserver")
    ctx, span := tracer.Start(ctx, "mcp.tools.call",
        trace.WithAttributes(
            attribute.String("mcp.tool.name", toolName),
            attribute.String("mcp.protocol.version", d.cfg.ProtocolVersion),
            attribute.String("mcp.session.id", sessionID),
            attribute.String("auth.principal.id", principal.ID),
            attribute.String("auth.principal.kind", string(principal.Kind)),
            attribute.String("auth.method", principal.AuthMethod),
        ),
    )
    defer span.End()
    // ... execute ...
    span.SetAttributes(
        attribute.Int("mcp.tool.result.content_parts", len(content)),
        attribute.Bool("mcp.tool.result.is_error", isError),
        attribute.Int("mcp.tool.result.status_code", httpStatus),
    )
    if isError { span.SetStatus(codes.Error, errMsg) }
}
```

Span hierarchy:

```
otelhttp (chi middleware)
  └─ mcp.rpc
       ├─ mcp.tools.call
       │    └─ mcp.tool.loopback
       │         └─ otelhttp (internal route)
       └─ mcp.session.touch
```

Tool calls show twice (MCP layer + REST layer via loopback). Intentional — correlates MCP latency with underlying REST path.

### 7.4 OTel metrics

Namespace `agentlens_mcp_*`.

```go
type Metrics struct {
    RPCCount            metric.Int64Counter
    RPCDuration         metric.Float64Histogram
    ToolCallCount       metric.Int64Counter
    ToolCallDuration    metric.Float64Histogram
    SessionsActive      metric.Int64UpDownCounter
    AuthFailures        metric.Int64Counter
    FederationReachable metric.Int64ObservableGauge
}
```

Dimensions:

- `method` — `initialize|ping|tools/list|tools/call|notifications/initialized`
- `tool` — tool name
- `outcome` — `success|tool_error|invalid_params|auth_error|server_error`
- `reason` — `invalid_credentials|token_expired|audience_mismatch|unknown_principal|...`
- `provider` — `dex|oidc|...`

Histogram buckets: default OTel HTTP buckets (5ms … 10s).

### 7.5 Audit log

```go
type ToolInvocationAudit struct {
    Timestamp      time.Time `json:"ts"`
    RequestID      string    `json:"request_id"`
    SessionID      string    `json:"session_id"`
    PrincipalID    string    `json:"principal_id"`
    PrincipalKind  string    `json:"principal_kind"`
    AuthMethod     string    `json:"auth_method"`
    CredentialID   string    `json:"credential_id"`
    ToolName       string    `json:"tool_name"`
    ArgumentsJSON  string    `json:"arguments"`       // scrubbed
    ProjectsScoped []string  `json:"projects_scoped"`
    Outcome        string    `json:"outcome"`
    HTTPStatus     int       `json:"http_status"`
    DurationMS     int64     `json:"duration_ms"`
    RemoteAddr     string    `json:"remote_addr"`
}
```

Emitted via `slog.InfoContext(ctx, "mcp.tool.invoked", ...)`. Goes to stdout JSON + optional OTel log bridge (consistent with ADR-009).

Secret scrubbing: keys matching `(?i)(password|secret|token|key|credential)` → `"***"`. Applied via `redact.ScrubJSON(raw)` helper. Defense-in-depth; v1 tools don't accept secrets but v2 may.

### 7.6 Failed-auth audit

At `RequireAuth` layer:

```go
slog.InfoContext(ctx, "auth.failure",
    "reason", reason,
    "auth_method_attempted", method,
    "remote_addr", r.RemoteAddr,
    "request_id", middleware.GetReqID(ctx),
    "external_sub", externalSubHint,  // only when partial identification possible
)
```

Never fabricate Principal fields when caller couldn't be identified.

### 7.7 Federation health monitoring

```go
// internal/auth/federation/dex/health.go
func (p *DexProvider) Start(ctx context.Context) error {
    go p.healthLoop(ctx)
    return nil
}

func (p *DexProvider) healthLoop(ctx context.Context) {
    ticker := time.NewTicker(p.healthInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            ctx2, cancel := context.WithTimeout(ctx, p.healthTimeout)
            err := p.pingDiscovery(ctx2)  // GET {issuer}/.well-known/openid-configuration
            cancel()
            p.recordHealth(err)
            p.metrics.FederationReachable.Observe(ctx, boolToInt64(err == nil),
                attribute.String("provider", p.Kind()))
        }
    }
}
```

- `Provider.HealthCheck(ctx)` — sync check, forces refresh.
- `Provider.LastStatus()` — cached status for `/api/mcp/status`.

### 7.8 `/api/mcp/status` field sources

| Field | Source |
|---|---|
| `plugin.enabled` | `cfg.Enabled` |
| `plugin.version` | `Plugin.Version()` |
| `plugin.protocol_version` | `cfg.ProtocolVersion` |
| `plugin.wire_impl` | `wire.Impl.Kind()` |
| `sessions.active` | `sessions.Count()` — authenticated only |
| `sessions.max` | `cfg.MaxSessions` |
| `tools.registered` | `registry.Count()` |
| `tools.names` | `registry.List()` sorted |
| `federation.enabled` | `fedProvider != nil` |
| `federation.provider` | `fedProvider.Kind()` |
| `federation.reachable` | `fedProvider.LastStatus().Reachable` |
| `federation.last_checked` | `fedProvider.LastStatus().LastChecked` — authenticated only |
| `self_registered.catalog_entry_id` | cached — authenticated only |
| `self_registered.in_default_project` | cached — authenticated only |

### 7.9 Readiness probe extension

```go
// internal/api/health_handlers.go
func Readyz(deps RouterDeps) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !deps.DB.Ping() { ErrorResponse(w, 503, "db unreachable"); return }
        if deps.FedProvider != nil && !deps.FedProvider.LastStatus().Reachable {
            ErrorResponse(w, 503, "federation provider unreachable")
            return
        }
        if deps.MCPPlugin != nil && !deps.MCPPlugin.Ready() {
            ErrorResponse(w, 503, "mcp plugin not ready")
            return
        }
        JSONResponse(w, 200, map[string]string{"status": "ready"})
    }
}
```

`/healthz` stays liveness-only. `/readyz` checks dependencies. Existing pattern preserved.

### 7.10 Operator alert reference (docs only, not shipped dashboards)

| Alert | Condition | Severity |
|---|---|---|
| Tool error rate | `rate(agentlens_mcp_tool_calls_total{outcome="server_error"}[5m]) > 0.05` | warn |
| Auth failure spike | `rate(agentlens_mcp_auth_failures_total[5m]) > 5` | warn |
| Federation provider down | `agentlens_mcp_federation_reachable == 0 for 5m` | critical |
| Session capacity near cap | `agentlens_mcp_sessions_active / max_sessions > 0.8` | warn |
| p95 SLO breach | `histogram_quantile(0.95, agentlens_mcp_tool_call_duration_seconds_bucket[10m]) > 0.1` | warn |

Ship in `docs/observability.md`. No dashboard JSON in v1.

### 7.11 Testing

- Self-registration: integration test asserts entry present + default-project-assigned after plugin Init.
- Tool-surface drift: modify registry, restart plugin, verify `entry.Capabilities` reflects new set.
- Audit log: tool call → capture stdout → assert JSON shape + no leaked secrets.
- Metrics: in-memory OTel meter provider, assert counter increments.
- Federation health: mock provider with flapping status, assert `last_checked` + `reachable` update.

---

## §8. Deployment & Operations

### 8.1 Helm chart changes

Chart `0.2.0` → `0.3.0`. Additions off-by-default; existing deployments upgrade without config.

#### 8.1.1 Dex as subchart

```yaml
# charts/agentlens/Chart.yaml
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

#### 8.1.2 New values

```yaml
# values.yaml
mcpServer:
  enabled: true
  canonicalURL: ""                    # REQUIRED when enabled
  protocolVersion: "2025-11-25"
  maxSessions: 1000
  sessionTTL: 4h
  requestTimeout: 30s
  auditEnabled: true

federation:
  enabled: false
  provider: ""
  common:
    audiencePrefix: ""                # defaults to mcpServer.canonicalURL at render
    autoProvisionUsers: false
    defaultRoleID: ""
    jwksCacheTTL: 1h
    healthCheckInterval: 30s
    healthCheckTimeout: 5s
    roleMapping: {}
  instance:
    issuerURL: ""
    jwksURL: ""
    options: {}

dex:
  enabled: false
  config:
    issuer: http://agentlens-dex:5556/dex
    storage:
      type: kubernetes
      config:
        inCluster: true
    web:
      http: 0.0.0.0:5556
    oauth2:
      skipApprovalScreen: false
      alwaysShowLoginScreen: true
    connectors: []
    staticClients:
      - id: agentlens-mcp
        redirectURIs:
          - https://agentlens.example.com/oauth/callback
        name: AgentLens MCP
        secret: CHANGEME
  service:
    type: ClusterIP
    port: 5556
```

#### 8.1.3 Rendering logic (`templates/_helpers.tpl`)

```yaml
{{- define "agentlens.federationAudiencePrefix" -}}
{{- if .Values.federation.common.audiencePrefix -}}
{{ .Values.federation.common.audiencePrefix }}
{{- else -}}
{{ .Values.mcpServer.canonicalURL }}
{{- end -}}
{{- end -}}

{{- define "agentlens.validateConfig" -}}
{{- if and .Values.mcpServer.enabled (not .Values.mcpServer.canonicalURL) -}}
{{- fail "mcpServer.canonicalURL is required when mcpServer.enabled=true" -}}
{{- end -}}
{{- if and .Values.federation.enabled (not .Values.federation.common.audiencePrefix) (not .Values.mcpServer.enabled) -}}
{{- fail "federation.enabled=true without mcpServer.enabled=true requires explicit federation.common.audiencePrefix" -}}
{{- end -}}
{{- end -}}
```

Called from top-level templates via `{{- include "agentlens.validateConfig" . -}}`. Fails `helm template` / `helm lint --strict`.

#### 8.1.4 Deployment env pass-through

```yaml
# templates/deployment.yaml (appended)
{{- if .Values.mcpServer.enabled }}
- name: AGENTLENS_MCP_SERVER_ENABLED
  value: "true"
- name: AGENTLENS_MCP_SERVER_CANONICAL_URL
  value: {{ .Values.mcpServer.canonicalURL | quote }}
- name: AGENTLENS_MCP_SERVER_MAX_SESSIONS
  value: {{ .Values.mcpServer.maxSessions | quote }}
- name: AGENTLENS_MCP_SERVER_SESSION_TTL
  value: {{ .Values.mcpServer.sessionTTL | quote }}
- name: AGENTLENS_MCP_SERVER_REQUEST_TIMEOUT
  value: {{ .Values.mcpServer.requestTimeout | quote }}
- name: AGENTLENS_MCP_SERVER_AUDIT_ENABLED
  value: {{ .Values.mcpServer.auditEnabled | quote }}
{{- end }}
{{- if .Values.federation.enabled }}
- name: AGENTLENS_FEDERATION_PROVIDER
  value: {{ .Values.federation.provider | quote }}
- name: AGENTLENS_FEDERATION_INSTANCE_ISSUER_URL
  value: {{ .Values.federation.instance.issuerURL | quote }}
- name: AGENTLENS_FEDERATION_COMMON_AUDIENCE_PREFIX
  value: {{ include "agentlens.federationAudiencePrefix" . | quote }}
- name: AGENTLENS_FEDERATION_COMMON_AUTO_PROVISION_USERS
  value: {{ .Values.federation.common.autoProvisionUsers | quote }}
{{- if .Values.federation.common.defaultRoleID }}
- name: AGENTLENS_FEDERATION_COMMON_DEFAULT_ROLE_ID
  value: {{ .Values.federation.common.defaultRoleID | quote }}
{{- end }}
{{- if .Values.federation.common.roleMapping }}
- name: AGENTLENS_FEDERATION_COMMON_ROLE_MAPPING
  value: {{ .Values.federation.common.roleMapping | toJson | quote }}
{{- end }}
{{- end }}
```

#### 8.1.5 CI values file

```yaml
# ci/ci-values.yaml (appended)
mcpServer:
  enabled: true
  canonicalURL: https://agentlens-ci.example.com/api/mcp

federation:
  enabled: true
  provider: dex
  instance:
    issuerURL: http://agentlens-dex:5556/dex
  common:
    audiencePrefix: https://agentlens-ci.example.com/api/mcp
    autoProvisionUsers: true
    defaultRoleID: "viewer"

dex:
  enabled: true
```

Extend `./scripts/test-helm-templates.sh` to grep for new env vars + Dex deployment.

### 8.2 Docker Compose bundling

```yaml
# docker-compose.yml (reference)
services:
  agentlens:
    image: ghcr.io/agentlens/agentlens:0.3.0
    environment:
      AGENTLENS_PORT: "8080"
      AGENTLENS_MCP_SERVER_ENABLED: "true"
      AGENTLENS_MCP_SERVER_CANONICAL_URL: "https://agentlens.local/api/mcp"
      AGENTLENS_FEDERATION_ENABLED: "true"
      AGENTLENS_FEDERATION_PROVIDER: "dex"
      AGENTLENS_FEDERATION_INSTANCE_ISSUER_URL: "http://dex:5556/dex"
      AGENTLENS_FEDERATION_COMMON_AUDIENCE_PREFIX: "https://agentlens.local/api/mcp"
    ports: ["8080:8080"]
    depends_on:
      dex:
        condition: service_healthy

  dex:
    image: ghcr.io/dexidp/dex:v2.41.1
    volumes:
      - ./dex-config.yaml:/etc/dex/config.docker.yaml:ro
    command: ["dex", "serve", "/etc/dex/config.docker.yaml"]
    ports: ["5556:5556"]
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:5556/dex/healthz/live"]
      interval: 10s
      timeout: 3s
      retries: 6
```

`dex-config.yaml` ships with LDAP + GitHub connector examples commented.

### 8.3 Service-account admin surface

#### 8.3.1 Service-account routes

| Method | Path | Permission |
|---|---|---|
| `GET` | `/api/v1/service-accounts` | `service_accounts:read` |
| `POST` | `/api/v1/service-accounts` | `service_accounts:write` |
| `GET` | `/api/v1/service-accounts/{id}` | `service_accounts:read` |
| `DELETE` | `/api/v1/service-accounts/{id}` | `service_accounts:delete` |
| `GET` | `/api/v1/service-accounts/{id}/keys` | `service_accounts:read` |
| `POST` | `/api/v1/service-accounts/{id}/keys` | `service_accounts:write` |
| `DELETE` | `/api/v1/service-accounts/{id}/keys/{keyID}` | `service_accounts:write` |
| `POST` | `/api/v1/service-accounts/{id}/projects` | `service_accounts:write` |
| `DELETE` | `/api/v1/service-accounts/{id}/projects/{projectID}` | `service_accounts:write` |

Handlers: `internal/api/service_account_handlers.go`. Reuse `party_store.go` + new `api_client_credentials` store.

Key-creation response (one-time secret):

```json
{
  "id": "cred-uuid",
  "label": "doc-pipeline-prod",
  "secret": "agentlens_sk_cred-uuid.3f8a...",
  "created_at": "2026-04-17T16:07:27Z"
}
```

#### 8.3.2 External-identity mapping routes

| Method | Path | Permission |
|---|---|---|
| `GET` | `/api/v1/admin/external-identities/pending` | `users:write` |
| `POST` | `/api/v1/admin/external-identities/link` | `users:write` |
| `POST` | `/api/v1/admin/external-identities/create-and-link` | `users:write` |
| `DELETE` | `/api/v1/admin/external-identities/{id}` | `users:write` |

Handlers: `internal/api/external_identity_handlers.go`.

#### 8.3.3 UI additions (`web/src/routes/admin/`)

- `ServiceAccountsPage.tsx` — list + create + delete.
- `ServiceAccountDetailPage.tsx` — credentials (label, created, last_used, revoke), project memberships, issue-key dialog (secret shown once, copy-to-clipboard).
- `PendingIdentitiesPage.tsx` — list pending external identities; link-to-existing-user or create-and-link form.
- Route additions in `AppRouter.tsx`; nav in admin sidebar.

Vitest `.test.tsx` sibling per page; 80/80/75/80 thresholds.

### 8.4 Bootstrap/first-run UX

1. AgentLens starts with `mcpServer.enabled=true` + `federation.enabled=true`. Basic-auth admin bootstrapped (existing; password printed once).
2. Admin logs into UI with basic auth.
3. Admin creates first service account via UI; secret shown once.
4. Admin receives canonical URL + audit dashboard hint.
5. Backend app (Anya) sets env vars, starts, sees tools.
6. Claude.ai flow (Karol): paste URL → browser redirect to Dex (via discovery) → Dex delegates to configured connector → SSO → AgentLens sees unknown external identity → admin gets pending-identity notification.
7. Priya opens Pending Identities, links Karol's Dex identity to his AgentLens user, Karol retries, works.

Documented in new `docs/mcp-quickstart.md` with commands + screenshots.

### 8.5 Docs updates

| Doc | Content |
|---|---|
| `docs/api.md` | All new routes (§8.3.1, §8.3.2) with method/path/schema/errors/permissions. |
| `docs/architecture.md` | Mermaid: MCP plugin + Dex + federation + loopback flow. |
| `docs/end-user-guide.md` | Service-account admin screenshots via Playwright `data-testid`; pending-identity approval flow. |
| `docs/settings.md` | `mcp_server.*` + `federation.*` with defaults + env vars. |
| `docs/observability.md` (new) | Alert recipes from §7.10, trace/metric/audit reference. |
| `docs/mcp-quickstart.md` (new) | 5-min getting-started for backend + IDE personas. |
| `README.md` | One paragraph + link to quickstart. |

### 8.6 Release cadence

Chart `0.3.0` releases alongside app `0.3.0`. Semantic-release tags both: `v0.3.0` (image at `ghcr.io/agentlens/agentlens:0.3.0`) + `helm/v0.3.0` (OCI chart at `oci://ghcr.io/agentlens/charts/agentlens:0.3.0`). Dex is a dep, not re-published.

### 8.7 Upgrade path (0.2.0 → 0.3.0)

1. `helm upgrade agentlens oci://... --version 0.3.0` — migration 008 runs on pod start; new tables created; no data migration.
2. `mcpServer.enabled=true` is new default but activating requires `canonicalURL` (validation fails render otherwise). Operators may set `mcpServer.enabled=false` on first upgrade for review.
3. Federation remains `enabled: false` in defaults — no auth behavior change unless opt-in.
4. Rollback safe: flip flags off, new tables stay unused. Forward-only migrations per `standards/backend/database-dialects.md`.

### 8.8 PR feature checklist coverage

| Step | Handled by |
|---|---|
| 1. `make test` passes | §5.11 + §6.10 + §7.11 |
| 2. `make e2e-test` passes | Go MCP client e2e in `e2e/mcp/` (§5.11) |
| 3. `docs/api.md` updated | §8.5 |
| 4. `docs/architecture.md` (Mermaid) | §8.5 |
| 5. `docs/end-user-guide.md` + screenshots | §8.5 |
| 6. `docs/settings.md` + config keys | §2.10 + §8.5 |
| 7. `make arch-test` passes | §5.1 plugin layout (imports only kernel + foundation) |

### 8.9 New permissions seeding

Migration 008 + role-seeder: grant `service_accounts:read|write|delete` to `admin`. `developer` built-in role (if seeded) gets `service_accounts:read`. No other system role changes.

### 8.10 Release smoke-test (operator)

1. `kubectl logs` AgentLens pod — see `auth.federation.enabled=true`, `mcp.plugin.started`.
2. `curl https://agentlens.example.com/api/mcp/status` — expect JSON with `plugin.enabled=true`.
3. `curl https://agentlens.example.com/.well-known/oauth-protected-resource` — expect JSON with `authorization_servers` pointing at Dex.
4. Issue service-account key via UI → `curl -H "Authorization: Bearer agentlens_sk_…" https://agentlens.example.com/api/mcp -d '{"jsonrpc":"2.0","id":1,"method":"initialize",...}'` — expect initialize response.
5. Call `agent_search` — expect non-error tool result.

Covered in `docs/mcp-quickstart.md`.

---

*Specification complete. 8 sections, ~1500 lines. Last updated 2026-04-17.*

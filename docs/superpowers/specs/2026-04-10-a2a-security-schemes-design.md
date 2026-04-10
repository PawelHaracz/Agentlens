# Feature 2.5 — A2A Security Schemes: Parsing, Storage, and Dashboard Display

**Tier:** 2 — SHOULD HAVE
**Effort:** M (3-4 days)
**Date:** 2026-04-10

---

## Goal

When a platform engineer discovers an agent via the catalog or the Capabilities tab, the immediate next question is: "How do I authenticate to this thing?" Today the answer is "go read the agent's documentation" — which is exactly the kind of friction AgentLens exists to eliminate.

This feature parses the `securitySchemes` and `securityRequirements` blocks from A2A Agent Cards (spec v0.3 and v1.0), stores them via the existing capability system and a new `SecurityStorePlugin`, exposes them via the API, and renders a clear "Authentication" section in the agent detail view that tells the integrator everything they need to connect.

## Why

- Every A2A Agent Card can declare how clients must authenticate: API keys, Bearer tokens, OAuth 2.0 flows, OIDC, or mTLS. Today AgentLens stores only a minimal summary (`A2ASecurityScheme{Type, Method, Name}`) as non-discoverable capability rows.
- Without security scheme display, every integration starts with manual doc diving. Multiply that by 50 agents in a catalog and it becomes a real bottleneck for platform teams.
- Competing registries (a2aregistry.org, Apicurio) do not surface auth requirements in a structured, actionable way. This is a clear UX win.
- Feature 2.3 (Capability-based Discovery) just landed — users are now finding agents by capability. The natural follow-up is "I found the right agent, now show me how to connect."

---

## Decisions Made During Design

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Rich security detail storage | **Hybrid: capability rows + SecurityStorePlugin** | `a2a.security_scheme` capability rows provide indexing/summary (type, method, name). A new `SecurityStorePlugin` stores the rich structured detail (OAuth flows, scopes, URLs). This follows the same pattern as raw cards: capability table for discovery, plugin for full data. |
| SecurityStorePlugin scope | **Protocol-agnostic** — works for A2A, MCP, A2UI | The plugin interface accepts any protocol. A2A is the first consumer. MCP and A2UI parsers can use the same plugin when their specs define security. |
| SecurityStorePlugin default impl | **Built-in SQLite-compatible OSS implementation** in `plugins/securitystore/` | Ships a `security_details` table with JSON TEXT storage, same as cardstore. Enterprise can swap in a Postgres-optimized version. |
| `securityRequirements` storage | **New non-discoverable capability kind** (`a2a.security_requirement`) | Stores top-level security requirements as capability rows in the existing `capabilities` table. Consistent with the pattern used for `a2a.extension`, `a2a.interface`, `a2a.signature`. |
| Per-skill security requirements | **Separate `a2a.security_requirement` capability rows per skill** | Per-skill security overrides are stored as distinct capability rows, not embedded in `A2ASkill` properties. Keeps capability rows normalized and queryable. |
| A2A spec version handling | **Support both v0.3 array + v1.0 named map** | Current parser handles `securitySchemes` as `[]fullSecurity` (v0.3 array format). A2A v1.0 uses a named map `map[string]SecurityScheme`. Parser detects format and handles both. v0.3 entries get auto-generated names from type field. |
| `authSummary` in list response | **Computed at handler level** from `a2a.security_scheme` capability rows | No new column, no migration. Handler iterates the agent's capabilities to build a lightweight summary. |
| CatalogEntry changes | **None** | Security data belongs to `AgentType` (Product Archetype). CatalogEntry never directly holds capabilities. No new columns on `catalog_entries`. |
| Domain model location | **Enrich existing `A2ASecurityScheme`** + new `A2ASecurityRequirement` in `internal/model/a2a_capabilities.go` | No new `security.go` file in model. Security types are A2A capability types — they belong with the other A2A capabilities. |
| Deprecated OAuth flows | **Parse but never display** | Implicit and Password flows are parsed for completeness, marked `Deprecated = true` in the model. Frontend omits them entirely — no strikethrough, no warning. |
| Migration approach | **Go migration function** in `internal/db/migrations.go` | Not SQL files. The project uses Go migration functions registered in `AllMigrations()`. New migration creates the `security_details` table if the SecurityStorePlugin doesn't handle its own schema (following cardstore pattern, the plugin does its own `AutoMigrate`). |

---

## Scope (in)

1. Enrich `A2ASecurityScheme` model with full A2A v1.0 security scheme fields (OAuth flows, scopes, URLs, API key details, OIDC, mTLS)
2. New `A2ASecurityRequirement` capability kind (`a2a.security_requirement`, non-discoverable)
3. New `SecurityStorePlugin` kernel interface — protocol-agnostic, stores rich structured security detail
4. Built-in `plugins/securitystore/` implementation with `security_details` table
5. A2A parser extension: parse both v0.3 array and v1.0 named map `securitySchemes`, parse `securityRequirements[]`
6. Per-skill `securityRequirements` parsing into separate capability rows
7. API: `authSummary` derived field in `GET /api/v1/catalog` list response
8. API: rich security detail in `GET /api/v1/catalog/{id}` detail response
9. Dashboard: "Authentication" section in agent detail view with scheme cards, connection recipe, copy buttons
10. Per-skill security display on skill rows in agent detail
11. Catalog list: small auth indicator badge per row

## Scope (out)

- AgentLens performing OAuth flows on behalf of the user (AgentLens is a registry, not a gateway)
- Token storage, secret management, or credential vaulting
- Validating that the declared auth endpoint is reachable (health probe already covers endpoint liveness)
- MCP server auth parsing (MCP spec handles auth differently — separate backlog item)
- JWS signature verification of the Agent Card itself (`signatures[]` — Tier 3)
- Modifying or overriding the security config declared by the agent (read-only display)
- New Postgres `JSONB` column — `TEXT` for dialect symmetry, same as features 1.6 and 2.2

---

## Backend Changes

### 1. Enrich `A2ASecurityScheme` capability model

File: `internal/model/a2a_capabilities.go`

The existing `A2ASecurityScheme` carries only `Type`, `Method`, `Name`. Enrich it with fields for all five A2A v1.0 scheme types:

```go
// A2ASecurityScheme represents a security scheme for A2A communication.
// kind: "a2a.security_scheme"
//
// This is a union type discriminated by Type. Only fields relevant to the
// scheme type are populated. Consumers switch on Type to determine which
// fields to read.
type A2ASecurityScheme struct {
    // Common
    SchemeName  string `json:"scheme_name"`           // key from Agent Card's securitySchemes map
    Type        string `json:"type"`                   // "apiKey" | "http" | "oauth2" | "openIdConnect" | "mutualTls"
    Description string `json:"description,omitempty"`

    // apiKey
    APIKeyLocation string `json:"api_key_location,omitempty"` // "header" | "query" | "cookie"
    APIKeyName     string `json:"api_key_name,omitempty"`     // e.g. "X-API-Key"

    // http
    HTTPScheme   string `json:"http_scheme,omitempty"`   // "Bearer" | "Basic" | "Digest"
    BearerFormat string `json:"bearer_format,omitempty"` // e.g. "JWT"

    // oauth2
    OAuthFlows        []A2AOAuthFlow `json:"oauth_flows,omitempty"`
    OAuth2MetadataURL string         `json:"oauth2_metadata_url,omitempty"`

    // openIdConnect
    OpenIDConnectURL string `json:"openid_connect_url,omitempty"`

    // Backward compat (v0.3 parser used these)
    Method string `json:"method,omitempty"`
    Name   string `json:"name,omitempty"`
}
```

New supporting type:

```go
// A2AOAuthFlow represents a single OAuth 2.0 flow variant.
type A2AOAuthFlow struct {
    FlowType         string            `json:"flow_type"`                    // "authorizationCode" | "clientCredentials" | "deviceCode" | "implicit" | "password"
    AuthorizationURL string            `json:"authorization_url,omitempty"`
    TokenURL         string            `json:"token_url,omitempty"`
    RefreshURL       string            `json:"refresh_url,omitempty"`
    DeviceAuthURL    string            `json:"device_auth_url,omitempty"`    // deviceCode flow only
    Scopes           map[string]string `json:"scopes,omitempty"`             // scope -> description
    Deprecated       bool              `json:"deprecated,omitempty"`         // true for implicit/password
}
```

The enriched `A2ASecurityScheme` continues to implement `Capability` via `Kind() string` and `Validate() error`. The `Validate()` method checks that `Type` is non-empty.

**Capability name column mapping:** The store's `capabilityToRow` extracts the `name` column from the capability's JSON `"name"` field. For backward compatibility, the existing `Name` field is kept and populated: for v1.0, `Name = SchemeName` (the map key); for v0.3, `Name` retains the existing v0.3 value. The `SchemeName` field is the canonical key for referencing schemes in `SecurityRequirement`. The `Name` field is what `capabilityToRow` uses for the `name` DB column. Both are set by the parser.

### 2. New `A2ASecurityRequirement` capability kind

File: `internal/model/a2a_capabilities.go`

```go
// A2ASecurityRequirement declares which scheme(s) a client MUST use.
// kind: "a2a.security_requirement"
//
// Map key = scheme name (matching a key in SecuritySchemes).
// Map value = list of required scopes (empty = no specific scopes).
// Multiple A2ASecurityRequirement entries on an AgentType are OR'd:
// the client must satisfy at least one complete entry.
type A2ASecurityRequirement struct {
    Schemes  map[string][]string `json:"schemes"`
    SkillRef string              `json:"skill_ref,omitempty"` // non-empty = per-skill override
}

func (a *A2ASecurityRequirement) Kind() string { return "a2a.security_requirement" }
func (a *A2ASecurityRequirement) Validate() error {
    if len(a.Schemes) == 0 {
        return errors.New("a2a.security_requirement: schemes must not be empty")
    }
    return nil
}
```

Register in `init()`:

```go
RegisterCapability("a2a.security_requirement", func() Capability { return &A2ASecurityRequirement{} }, false)
```

`SkillRef` links per-skill requirements to their parent skill. When empty, the requirement applies to the entire agent. When non-empty, it contains the skill name and overrides the top-level requirements for that specific skill.

### 3. New `SecurityStorePlugin` kernel interface

File: `internal/kernel/plugin.go`

Add new plugin type constant:

```go
PluginTypeSecurityStore PluginType = "securitystore"
```

Add the specialized interface:

```go
// SecurityStorePlugin persists structured security detail keyed by AgentTypeID.
// Protocol-agnostic — works for A2A, MCP, and A2UI.
type SecurityStorePlugin interface {
    Plugin
    StoreSecurityDetail(ctx context.Context, agentTypeID string, detail *model.SecurityDetail) error
    GetSecurityDetail(ctx context.Context, agentTypeID string) (*model.SecurityDetail, error)
}
```

File: `internal/kernel/kernel.go` — add field and accessor:

```go
type Core struct {
    // ...existing fields...
    securityStore SecurityStorePlugin
}
```

```go
// SecurityStore returns the registered security store plugin, or nil if not loaded.
func (c *Core) SecurityStore() SecurityStorePlugin { return c.securityStore }
```

File: `internal/kernel/plugin.go` — extend `Kernel` interface:

```go
type Kernel interface {
    // ...existing methods...
    SecurityStore() SecurityStorePlugin
}
```

File: `internal/kernel/core_registry.go`:

```go
// RegisterSecurityStore registers the security store plugin.
func (c *Core) RegisterSecurityStore(p SecurityStorePlugin) {
    c.securityStore = p
}
```

File: `internal/kernel/plugin_manager.go` — add auto-registration in `InitAll()`:

```go
// Register security store plugins with the kernel
if ssp, ok := p.(SecurityStorePlugin); ok {
    pm.core.RegisterSecurityStore(ssp)
}
```

### 4. SecurityDetail transport model

File: `internal/model/security_detail.go`

```go
// SecurityDetail holds the full structured security metadata for an AgentType.
// It is a pure transport struct returned by SecurityStorePlugin — NOT a GORM
// model and has no database tags. Persistence is handled by the plugin's
// internal row struct.
type SecurityDetail struct {
    AgentTypeID string `json:"agent_type_id"`
    Protocol    string `json:"protocol"`

    // Full structured security schemes keyed by scheme name.
    // For A2A v0.3 (array format), keys are auto-generated from type field.
    Schemes map[string]SecuritySchemeDetail `json:"schemes"`

    // Top-level security requirements.
    // Multiple entries are OR'd (client must satisfy at least one).
    // Schemes within a single entry are AND'd.
    Requirements []SecurityRequirementDetail `json:"requirements,omitempty"`

    // Per-skill security overrides. Key = skill name.
    SkillRequirements map[string][]SecurityRequirementDetail `json:"skill_requirements,omitempty"`
}

// SecuritySchemeDetail is the rich representation of one security scheme.
// Protocol-agnostic wrapper — the fields cover A2A scheme types.
// Future MCP/A2UI security will extend this or use the same fields.
type SecuritySchemeDetail struct {
    Type        string `json:"type"`
    Description string `json:"description,omitempty"`

    // apiKey
    APIKeyLocation string `json:"api_key_location,omitempty"`
    APIKeyName     string `json:"api_key_name,omitempty"`

    // http
    HTTPScheme   string `json:"http_scheme,omitempty"`
    BearerFormat string `json:"bearer_format,omitempty"`

    // oauth2
    OAuthFlows        []OAuthFlowDetail `json:"oauth_flows,omitempty"`
    OAuth2MetadataURL string            `json:"oauth2_metadata_url,omitempty"`

    // openIdConnect
    OpenIDConnectURL string `json:"openid_connect_url,omitempty"`

    // mutualTls — no additional fields
}

// OAuthFlowDetail holds one OAuth 2.0 flow variant.
type OAuthFlowDetail struct {
    FlowType         string            `json:"flow_type"`
    AuthorizationURL string            `json:"authorization_url,omitempty"`
    TokenURL         string            `json:"token_url,omitempty"`
    RefreshURL       string            `json:"refresh_url,omitempty"`
    DeviceAuthURL    string            `json:"device_auth_url,omitempty"`
    Scopes           map[string]string `json:"scopes,omitempty"`
    Deprecated       bool              `json:"deprecated,omitempty"`
}

// SecurityRequirementDetail declares which schemes a client must satisfy.
type SecurityRequirementDetail struct {
    Schemes map[string][]string `json:"schemes"`
}
```

Notes:
- `SecurityDetail` is a pure transport struct like `model.RawCard`. NOT a GORM model.
- `SecuritySchemeDetail` and `OAuthFlowDetail` mirror `A2ASecurityScheme`/`A2AOAuthFlow` fields but live in a protocol-agnostic wrapper. This avoids coupling the plugin interface to A2A-specific types.
- **Why two similar types?** `A2ASecurityScheme` is a `Capability` implementation — it participates in the capability registry, has `Kind()`/`Validate()`, and is serialized into capability rows with `name`/`description`/`properties` columns. `SecuritySchemeDetail` is a flat data struct stored as a single JSON blob by the plugin. The parser builds both from the same source data. The capability is for indexing; the detail is for display.
- The plugin stores/retrieves `SecurityDetail` as a single JSON blob per `AgentTypeID`.

### 5. Built-in SecurityStore plugin

File: `plugins/securitystore/plugin.go`

Follows the exact `cardstore` plugin pattern:

```go
package securitystore

// Plugin implements kernel.SecurityStorePlugin.
type Plugin struct {
    database *db.DB
}

// securityDetailRow is the GORM model for the security_details table.
type securityDetailRow struct {
    AgentTypeID string    `gorm:"primaryKey;type:text"`
    Data        string    `gorm:"not null;type:text;default:'{}'"`  // JSON TEXT
    UpdatedAt   time.Time `gorm:"not null"`
}

func (securityDetailRow) TableName() string { return "security_details" }
```

Methods:
- `New(database *db.DB) *Plugin`
- `Name() string` — returns `"security-store"`
- `Version() string` — returns `"1.0.0"`
- `Type() kernel.PluginType` — returns `kernel.PluginTypeSecurityStore`
- `Init(_ kernel.Kernel) error` — calls `MigrateSchema` (AutoMigrate, idempotent)
- `Start`, `Stop` — no-ops
- `StoreSecurityDetail(ctx, agentTypeID, detail)` — marshals `SecurityDetail` to JSON, upserts by PK using GORM `clause.OnConflict`
- `GetSecurityDetail(ctx, agentTypeID)` — reads row, unmarshals JSON into `*model.SecurityDetail`, returns `gorm.ErrRecordNotFound` if missing

File: `plugins/securitystore/migrate.go`

```go
func (p *Plugin) MigrateSchema(_ context.Context) error {
    return p.database.AutoMigrate(&securityDetailRow{})
}
```

### 6. A2A parser extension

File: `plugins/parsers/a2a/validation.go`

Extend `fullCard` to support v1.0 named map format. The parser must detect whether `securitySchemes` is an array (v0.3) or a map (v1.0):

```go
type fullCard struct {
    // ...existing fields...
    SecuritySchemes    json.RawMessage     `json:"securitySchemes,omitempty"`    // array (v0.3) OR map (v1.0)
    SecurityRequirements []json.RawMessage `json:"securityRequirements,omitempty"` // v1.0
}
```

Remove the existing typed `[]fullSecurity` for `SecuritySchemes` — use `json.RawMessage` for deferred parsing.

File: `plugins/parsers/a2a/a2a.go`

The `Parse` method extension:

1. After `buildA2AMetaCaps(&card)`, call new `buildSecurityCaps(&card)`:
   - Try to unmarshal `card.SecuritySchemes` as `map[string]json.RawMessage` (v1.0). If that fails, try `[]fullSecurity` (v0.3).
   - For v1.0: iterate the map, use the key as `SchemeName`, peek at `"type"` field, unmarshal into enriched `A2ASecurityScheme` with all fields populated.
   - For v0.3: iterate the array, auto-generate `SchemeName` from `Type` (e.g., `"apiKey"` -> `"apiKeyAuth"`, `"http"` -> `"httpAuth"`).
   - For OAuth2 flows: if `flowType` is `"implicit"` or `"password"`, set `Deprecated = true` and log a warning.
   - For unknown scheme types: log warning, store with `Type` and `Description` only — do not error.
   - Append each `*model.A2ASecurityScheme` to capabilities.

2. Call new `buildSecurityRequirements(&card)`:
   - Parse `card.SecurityRequirements` array.
   - Each entry is `map[string][]string` (scheme name -> required scopes).
   - Verify each referenced scheme name exists in the parsed schemes (warn if not, do not error).
   - Create `*model.A2ASecurityRequirement` with `SkillRef = ""` (top-level).

3. Parse per-skill `securityRequirements`:
   - Extend `fullSkill` to include `SecurityRequirements []json.RawMessage`.
   - For each skill with non-empty security requirements, create `*model.A2ASecurityRequirement` entries with `SkillRef = skill.Name`.

4. If the kernel has a `SecurityStorePlugin` registered, build a `model.SecurityDetail` from the parsed data and call `StoreSecurityDetail`. The parser gets access to the kernel via `Init(k kernel.Kernel)`:
   - Add a `kernel kernel.Kernel` field to the A2A parser's `Plugin` struct (currently it has only `initialized bool`).
   - In `Init(k kernel.Kernel)`, store `p.kernel = k`.
   - `Parse` is a pure function today — it cannot call the plugin directly. Instead, the caller (discovery manager / handler) is responsible for calling `SecurityStore().StoreSecurityDetail()` after a successful parse, same way the discovery manager calls `CardStore().StoreCard()` after parsing. The parser returns the `SecurityDetail` as a new field on `AgentType`:

   ```go
   type AgentType struct {
       // ...existing fields...
       SecurityDetail *SecurityDetail `json:"-" gorm:"-"` // transient, not persisted by GORM
   }
   ```

   The discovery manager and the registration handler check for `agentType.SecurityDetail != nil` and call `kernel.SecurityStore().StoreSecurityDetail()` if available — following the exact pattern used for `RawBytes` and `CardStore().StoreCard()`.

### 7. API changes — detail endpoint

File: `internal/api/handlers.go`

Extend `GetEntry` handler to include security detail in the response:

- After fetching the `CatalogEntry`, check if `kernel.SecurityStore()` is non-nil.
- If available, call `GetSecurityDetail(ctx, entry.AgentTypeID)`.
- If detail exists, add `securityDetail` to the response via `catalogEntryJSON`.
- If no detail (not found or plugin not registered), omit the field.

Extend `catalogEntryJSON` in `internal/model/agent.go`:

```go
type catalogEntryJSON struct {
    // ...existing fields...
    SecurityDetail *SecurityDetail `json:"security_detail,omitempty"`
}
```

**Important:** The `MarshalJSON()` method on `CatalogEntry` calls `toCatalogEntryJSON()` which today has no access to external data like `SecurityDetail`. The handler must NOT use `MarshalJSON()` for the detail endpoint. Instead, the handler should:
1. Call `entry.SyncFromDB()`
2. Call `entry.toCatalogEntryJSON()` — but this is unexported. Two options:
   a. Export it as `ToCatalogEntryJSON()` and add a `SecurityDetail` field to `catalogEntryJSON`.
   b. Build the response DTO directly in the handler, composing `catalogEntryJSON` + `security_detail`.

Option (a) is cleaner — export the method and extend the DTO. The handler passes `SecurityDetail` into the exported method. For the list endpoint, it passes `nil` (omitted from JSON). Existing callers that use `json.Marshal(entry)` (which calls `MarshalJSON`) continue to work — they just don't include security detail.

### 8. API changes — list endpoint `authSummary`

File: `internal/api/handlers.go`

Extend `ListCatalog` handler to compute `authSummary` for each entry:

- After loading entries (which already includes capabilities via `loadCapabilities`), iterate each entry's `AgentType.Capabilities`.
- Filter for `a2a.security_scheme` kind capabilities.
- Build `authSummary` from the capability data:
  - `types`: array of `"<type>:<subtype>"` strings (e.g., `"http:Bearer"`, `"apiKey"`).
  - `label`: human-readable join of type names (max 40 chars). Zero schemes -> `"Open (no auth)"`.
  - `required`: `true` if any `a2a.security_requirement` capabilities exist, `false` otherwise.

Add `authSummary` to `catalogEntryJSON`:

```go
type catalogEntryJSON struct {
    // ...existing fields...
    AuthSummary *authSummaryJSON `json:"auth_summary,omitempty"`
}

type authSummaryJSON struct {
    Types    []string `json:"types"`
    Label    string   `json:"label"`
    Required bool     `json:"required"`
}
```

The `authSummary` is computed at the handler level, not stored. It is a derived field populated only in the list response.

### 9. Wiring in main.go

File: `cmd/agentlens/main.go`

```go
import securitystorePlugin "github.com/PawelHaracz/agentlens/plugins/securitystore"

// After cardstorePlugin registration:
pm.Register(securitystorePlugin.New(database))
```

The `PluginManager.InitAll()` auto-registers it via the `SecurityStorePlugin` type assertion (step 3 above).

---

## Frontend Changes

### Agent detail view — "Authentication" section

Add a new section in the agent detail view, positioned below the health section and above protocol extensions. Use shadcn `Card` as the container.

#### Scheme cards

Each declared security scheme rendered as a shadcn `Card` inside the section. Content varies by type:

**HTTP Auth (`type: "http"`):**
- Badge: scheme value capitalized — "Bearer JWT", "Basic Auth", "Digest Auth"
- Icon: `lucide-react` `KeyRound`
- Body: `description` if present
- If Bearer + JWT: note "Expects a JWT in the Authorization header"

**API Key (`type: "apiKey"`):**
- Badge: "API Key"
- Icon: `lucide-react` `Key`
- Body: `description` if present
- Detail line: `Location: {location} / Name: {name}`
- Copy button: copies the header string `{name}: <your-key>`

**OAuth 2.0 (`type: "oauth2"`):**
- Badge: "OAuth 2.0"
- Icon: `lucide-react` `ShieldCheck`
- Sub-badges for each non-deprecated flow: "Authorization Code", "Client Credentials", "Device Code"
- URLs rendered as clickable links (open in new tab): Authorization URL, Token URL, Device Authorization URL
- If `oauth2MetadataUrl` is present: prominent link "OAuth 2.0 Metadata" opening in new tab
- Scopes table: two columns `Scope | Description`, only if scopes are non-empty
- Deprecated flows (Implicit, Password): not rendered at all

**OpenID Connect (`type: "openIdConnect"`):**
- Badge: "OIDC"
- Icon: `lucide-react` `Fingerprint`
- Clickable link: "OpenID Connect Discovery" pointing to `openIdConnectUrl`

**Mutual TLS (`type: "mutualTls"`):**
- Badge: "mTLS"
- Icon: `lucide-react` `Lock`
- Default text: "This agent requires mutual TLS. Configure your client certificate before connecting."

#### Security requirements banner

If `requirements` is non-empty, render a highlighted box above the scheme cards:
- Title: "Required to connect"
- For each requirement entry: list scheme names (linked to scheme card via anchor) and their required scopes
- Multiple entries: show "Any of the following combinations:" (entries are OR'd per A2A spec, schemes within entry are AND'd)

#### Connection recipe

Below scheme cards, a generated `curl` example using placeholder values:
- Endpoint URL from the agent's first `a2a.interface` capability
- Auth headers derived from the first `securityRequirement` entry
- Render in code block with Copy button

#### No auth declared state

- A2A entries with empty security: shadcn `Alert` with `variant="default"`: "This agent does not declare any authentication requirements."
- MCP entries: shadcn `Alert`: "MCP servers declare authentication at the transport level, not in the server card."

### Per-skill security in agent detail

On skill rows, if a skill has per-skill `securityRequirements` (detected via `a2a.security_requirement` capabilities with matching `SkillRef`), show a small inline badge:
- "Custom auth" with shadcn `HoverCard` showing per-skill requirements
- If different from top-level: "Overrides agent-level auth for this skill"

### Catalog list view — auth badge

In the catalog list table, add a compact auth indicator using `authSummary`:
- Column position: after spec version badge, before last seen
- Content: compact badge from `authSummary.label`, truncated to 25 chars with tooltip
- Color: `variant="outline"` (neutral)
- No schemes: "Open" badge with `variant="secondary"`

### Capabilities tab — auth indicator

On the Capability detail view (feature 2.3), add "Auth" column to agent rows showing the same `authSummary.label` badge.

### New frontend files

```
web/src/routes/catalog/detail/
  AuthenticationSection.tsx      (main section component)
  SchemeCard.tsx                 (renders one SecuritySchemeDetail)
  SecurityRequirementsBanner.tsx ("Required to connect" box)
  ConnectionRecipe.tsx           (generated curl example)
  PerSkillAuthBadge.tsx          (inline badge for skill-level security)
web/src/components/
  AuthBadge.tsx                  (compact badge for list views, reused in catalog + capabilities)
web/src/lib/
  securityUtils.ts              (buildAuthSummary, generateCurlRecipe, formatScopesLabel)
```

---

## Tests

### Backend — parser

Table-driven tests in `plugins/parsers/a2a/`. Add fixture files in `testdata/`:

1. **v10_bearer_only.json** — one `http` scheme with Bearer + JWT, one security requirement. Expected: `A2ASecurityScheme` with enriched fields, `A2ASecurityRequirement` capability created.
2. **v10_apikey_header.json** — one `apiKey` scheme in header. Expected: `APIKeyLocation = "header"`, `APIKeyName` populated.
3. **v10_oauth2_authcode.json** — OAuth2 with AuthorizationCode flow, `oauth2MetadataUrl`, scopes. Expected: flow parsed, URLs present, scopes map populated.
4. **v10_oauth2_deprecated_flows.json** — OAuth2 with Implicit + Password + ClientCredentials. Expected: Implicit and Password marked `Deprecated = true`, ClientCredentials normal.
5. **v10_oauth2_device_code.json** — OAuth2 with DeviceCode flow. Expected: `DeviceAuthURL` populated.
6. **v10_oidc.json** — OpenIdConnect scheme. Expected: `OpenIDConnectURL` populated.
7. **v10_mtls.json** — MutualTLS scheme with no extra fields. Expected: type set, no error.
8. **v10_multiple_schemes.json** — Three schemes (Bearer + APIKey + OAuth2), two security requirements. Expected: all three parsed as capabilities, both requirements as `a2a.security_requirement` capabilities.
9. **v10_skill_security.json** — Top-level Bearer + one skill with its own APIKey requirement. Expected: skill-level `A2ASecurityRequirement` with `SkillRef = skill.Name`.
10. **v10_no_security.json** — No `securitySchemes` field. Expected: no security capabilities, no error.
11. **v10_unknown_scheme_type.json** — Scheme with `"type": "customAuth"`. Expected: parsed with type and description only, log warning.
12. **v03_security_array.json** — v0.3 format `securitySchemes` as array. Expected: auto-generated `SchemeName`, backward-compatible parsing.

### Backend — SecurityStorePlugin

- Round-trip: store `SecurityDetail`, read back, verify JSON equality (SQLite)
- Missing entry: `GetSecurityDetail` returns `gorm.ErrRecordNotFound`
- Upsert: store twice for same `agentTypeID`, verify latest data returned
- Empty schemes map: valid state, no error

### Backend — handler

- `GET /catalog/{id}` includes `security_detail` when SecurityStorePlugin has data
- Same endpoint omits `security_detail` when no security data stored
- `GET /catalog` list includes `auth_summary` with correct `types`, `label`, `required`
- `auth_summary.label` for no schemes = `"Open (no auth)"`
- `auth_summary.required` reflects presence of `a2a.security_requirement` capabilities

### Frontend — unit

- `SchemeCard` renders correct content for each of 5 scheme types (snapshot tests)
- `ConnectionRecipe` generates correct curl for Bearer + APIKey combination
- `AuthBadge` renders "Open" when `required == false`
- `buildAuthSummary` utility: multiple schemes -> joined label, zero schemes -> "Open (no auth)"
- `PerSkillAuthBadge` shows "Custom auth" only when skill has own requirements

### E2E — Playwright

1. Seed an A2A agent with Bearer + APIKey schemes and two security requirements
2. Navigate to agent detail -> "Authentication" section visible
3. Verify two scheme cards rendered with correct badges
4. Verify "Required to connect" banner shows requirement
5. Verify connection recipe curl present with Copy button
6. Copy button works (clipboard API assertion)
7. Seed an A2A agent with no security schemes -> verify informational alert
8. Seed an MCP entry -> verify MCP-specific alert
9. On catalog list -> verify auth badge column shows "Bearer JWT + API Key" for first agent, "Open" for second
10. Seed agent with per-skill security -> verify skill row shows "Custom auth" badge with hover card

---

## Acceptance Criteria

1. All 5 A2A v1.0 security scheme types parse correctly from both v0.3 array and v1.0 named map formats
2. Deprecated OAuth flows (Implicit, Password) are parsed but never displayed in the UI
3. Unknown scheme types do not cause parse failure — stored with type + description, warning logged
4. `SecurityStorePlugin` interface added to kernel, with built-in SQLite implementation
5. Parser stores rich security detail via `SecurityStorePlugin` after parsing
6. `GET /catalog/{id}` returns `security_detail` as structured JSON when available
7. `GET /catalog` list includes `auth_summary` for each entry (computed at handler level)
8. Agent detail shows "Authentication" section with per-scheme cards, badges, and clickable URLs
9. Connection recipe curl is generated and copyable
10. Per-skill security requirements are displayed when present and differ from agent-level
11. Entries with no declared security show appropriate informational alert (different for A2A vs MCP)
12. Catalog list and Capabilities detail both show compact auth badges
13. `a2a.security_requirement` registered as non-discoverable capability kind
14. Per-skill security requirements stored as separate `a2a.security_requirement` capability rows with `SkillRef`
15. At least 12 parser fixture files with table-driven tests, all green
16. SecurityStorePlugin round-trip tests pass
17. Playwright E2E green on CI
18. `docs/api.md` updated for new response fields
19. `docs/end-user-guide.md` updated for Authentication section

---

## Known Traps

1. **Do NOT rename `CatalogEntry` to `Agent`.** Seven features in. The streak must continue.
2. **Do NOT add columns to `catalog_entries` for security data.** Security belongs to `AgentType` (Product Archetype). The `SecurityStorePlugin` stores detail keyed by `AgentTypeID`.
3. **Do NOT implement a `SecurityScheme` Go interface with per-type structs.** Go interface dispatch makes JSON round-tripping painful. Use a single struct with a `Type` discriminator and type-specific fields — the union pattern used for all existing A2A capabilities.
4. **Do NOT display deprecated OAuth flows.** Parse them, mark `Deprecated = true`, but frontend never renders Implicit or Password flows.
5. **Do NOT validate that auth endpoints are reachable.** This feature displays declared config. Endpoint reachability is the health prober's job (feature 1.2).
6. **Do NOT add security parsing to the MCP parser.** MCP handles auth differently. Leave MCP entries with empty security capabilities. Dashboard handles this gracefully.
7. **Do NOT require `securitySchemes` to be present.** OPTIONAL in the A2A spec. Many agents are publicly accessible.
8. **Do NOT invent a `SecurityLevel` enum (high/medium/low).** No basis for ranking. Show what the agent declares, do not editorialize.
9. **Do NOT put full `securitySchemes` in the list response.** Use the lightweight `authSummary` derived field.
10. **Do NOT use `localStorage` to cache auth tokens or secrets.** AgentLens is a registry, not an authenticator.
11. **Do NOT use SQL files for migrations.** The project uses Go migration functions in `internal/db/migrations.go`. The SecurityStore plugin handles its own schema via `AutoMigrate` (following cardstore pattern).
12. **Do NOT use raw HTML for scheme cards.** Use shadcn `Card`, `Badge`, `HoverCard`, `Tooltip`, `Alert`.
13. **Do NOT couple `SecurityStorePlugin` to A2A.** The interface uses `model.SecurityDetail`, not `model.A2ASecurityScheme`. MCP and A2UI parsers will use the same plugin.
14. **Do NOT violate arch-go function limits:** max 80 lines per function, 5 params, 3 return values, 10 public functions per file.
15. **Do NOT bypass the capability system.** `A2ASecurityScheme` capabilities provide indexing. `SecurityStorePlugin` provides rich detail. Both are needed — one does not replace the other.

---

## Execution Order (suggested commit sequence)

1. **Model:** Enrich `A2ASecurityScheme` with full fields in `a2a_capabilities.go`. Add `A2AOAuthFlow`. Add `A2ASecurityRequirement` + `init()` registration. Compiles, zero logic.
2. **Model:** Add `SecurityDetail`, `SecuritySchemeDetail`, `OAuthFlowDetail`, `SecurityRequirementDetail` to `internal/model/security_detail.go`. Add transient `SecurityDetail *SecurityDetail` field to `AgentType` (`json:"-" gorm:"-"`). Pure transport structs.
3. **Kernel:** Add `PluginTypeSecurityStore`, `SecurityStorePlugin` interface, `Core.securityStore` field, `SecurityStore()` accessor, `RegisterSecurityStore()`. Extend `InitAll()` with type assertion.
4. **Plugin:** `plugins/securitystore/` — plugin struct, GORM model, `MigrateSchema`, `StoreSecurityDetail`, `GetSecurityDetail`. Plugin tests.
5. **Wiring:** Register `securitystorePlugin.New(database)` in `cmd/agentlens/main.go`.
6. **Parser:** Refactor `fullCard.SecuritySchemes` from `[]fullSecurity` to `json.RawMessage`. Add `SecurityRequirements` field. Implement dual-format parsing (v0.3 array + v1.0 map). Add `buildSecurityCaps()` and `buildSecurityRequirements()`. Parser stores to `SecurityStorePlugin` if available. All 12 fixture files + table-driven tests.
7. **Parser:** Per-skill `securityRequirements` parsing. Extend `fullSkill`, create `A2ASecurityRequirement` with `SkillRef`. Tests.
8. **Handler:** Extend `GetEntry` to include `security_detail` from SecurityStore. Handler tests.
9. **Handler:** Add `authSummary` to `ListCatalog` response. Utility function + tests.
10. **Frontend utility:** `securityUtils.ts` — `buildAuthSummary`, `generateCurlRecipe`, `formatScopesLabel`. Unit tests.
11. **Frontend:** `SchemeCard` + `SecurityRequirementsBanner` + `ConnectionRecipe` components. Snapshot tests.
12. **Frontend:** `AuthenticationSection` composed from above, wired into agent detail view.
13. **Frontend:** `AuthBadge` in catalog list (reuses `authSummary`).
14. **Frontend:** `AuthBadge` in Capabilities detail view agent rows.
15. **Frontend:** `PerSkillAuthBadge` on skill rows in agent detail.
16. **Frontend:** Empty states — "no auth declared" alerts for A2A and MCP.
17. **E2E tests** in Playwright.
18. **Docs:** Update `docs/api.md`, `docs/end-user-guide.md`.

Each step is its own commit.

---

## Architectural Notes

### Product Archetype alignment

Security data follows the Product Archetype separation:
- `AgentType` (= ProductType) owns capabilities including `a2a.security_scheme` and `a2a.security_requirement`.
- `CatalogEntry` wraps `AgentType` with catalog concerns. It never directly holds security data.
- The `SecurityStorePlugin` keys data by `AgentTypeID`, not `CatalogEntry.ID`.
- The `authSummary` in the list response is derived at the handler level from the agent's capabilities — no storage on CatalogEntry.

### Microkernel alignment

The `SecurityStorePlugin` follows the exact pattern established by `CardStorePlugin`:

| Aspect | CardStorePlugin | SecurityStorePlugin |
|--------|----------------|---------------------|
| Plugin type | `cardstore` | `securitystore` |
| Transport model | `model.RawCard` | `model.SecurityDetail` |
| Internal GORM model | `rawCardRow` | `securityDetailRow` |
| Table | `raw_cards` | `security_details` |
| Key | `AgentTypeID` | `AgentTypeID` |
| Storage format | `[]byte` (blob) | `string` (JSON text) |
| Migration | `AutoMigrate` in `Init()` | `AutoMigrate` in `Init()` |
| Wiring | `main.go` registration | `main.go` registration |
| Auto-registration | `plugin_manager.go` type assertion | `plugin_manager.go` type assertion |

### Capability system alignment

Security data participates in the capability system at two levels:

1. **Indexing level** — `a2a.security_scheme` and `a2a.security_requirement` capability rows in the `capabilities` table. These provide the data for `authSummary` computation and allow SQL queries like "which agents require OAuth?" without parsing JSON.

2. **Detail level** — `SecurityStorePlugin` stores the full structured data with OAuth flows, scopes, URLs. This provides the data for the "Authentication" section in the detail view.

The two levels serve different consumers and never conflict. Capability rows are written by the store during `Create`/`Update` (existing flow). SecurityStore is written by the parser after parsing (new flow, parallel to CardStore).

### Layer boundaries (arch-go)

| New code | Layer | Dependencies |
|----------|-------|-------------|
| `internal/model/a2a_capabilities.go` (enriched) | Foundation | None |
| `internal/model/security_detail.go` | Foundation | None |
| `internal/kernel/plugin.go` (SecurityStorePlugin) | Core | Foundation |
| `internal/kernel/kernel.go` (accessor) | Core | Foundation |
| `internal/kernel/core_registry.go` (register) | Core | Foundation |
| `internal/kernel/plugin_manager.go` (assertion) | Core | Foundation |
| `plugins/securitystore/` | Plugins | Kernel + Foundation |
| `plugins/parsers/a2a/` (extended) | Plugins | Kernel + Foundation |
| `internal/api/handlers.go` (extended) | API | Core + Infrastructure |

---

## Out-of-band Notes

- The connection recipe is deliberately a template, not a working command. Using `<token>` and `<key>` placeholders makes it clear the user must supply real credentials.
- `securityRequirements` entries are OR'd per the A2A spec (client must satisfy at least one complete entry). Within a single entry, schemes are AND'd. The UI must communicate this clearly.
- The auth badge in the list view is intentionally compact — a summary signal, not the full story.
- After 2.5, the remaining Tier 2 features are 2.1 (OpenTelemetry) and 2.4 (Helm chart production-ready). Both are infra/DevOps focused.
- Future Tier 3 work on JWS signature verification will build on the parsing infrastructure added here.
- The `SecurityStorePlugin` is designed to be protocol-agnostic. When MCP defines its own security model, the same plugin and transport types can be reused — just a different parser populates them.

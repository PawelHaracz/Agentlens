# Feature 2.3 — Capability-based Discovery

**Tier:** 2 — SHOULD HAVE
**Effort:** S (2-3 days)
**Date:** 2026-04-09

---

## Goal

Give platform engineers a dedicated way to answer the question: "which agent in my catalog can do X?" — where X is a capability name, tag, or description phrase. Instead of browsing agents and opening each one to check their capabilities, a user lands on a Capabilities tab, searches for `translate`, and sees every agent that offers a translation capability, visually grouped by capability name.

This feature covers **discoverable** capability kinds — initially A2A skills, MCP tools, MCP resources, MCP prompts. Technical kinds (`a2a.extension`, `a2a.security_scheme`, `a2a.interface`, `a2a.signature`) are excluded — they are configuration details, not discoverable capabilities. Which kinds are discoverable is determined by metadata on the capability registry, not a hardcoded whitelist. Security schemes are handled separately in feature 2.5.

## Why

Capability-centric discovery is the actual mental model of people integrating AI agents. They don't start from "I want to use agent Foo" — they start from "I need summarization in Polish". Today that question is unanswerable in AgentLens without opening each card. Competitors (a2aregistry.org, Apicurio) don't expose this either, so it's a cheap way to be visibly ahead.

Second reason: the Product Archetype refactor (feature 1.4) created the polymorphic `Capability` interface and the relational `capabilities` table specifically to enable cross-protocol capability queries. Without 2.3, that investment looks like unused infrastructure.

---

## Decisions Made During Design

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Capability kinds in scope | Only discoverable kinds (initially 4: `a2a.skill`, `mcp.tool`, `mcp.resource`, `mcp.prompt`) | Technical kinds are excluded. Which kinds are discoverable is determined by the capability registry metadata, not a hardcoded whitelist. |
| Discoverable flag | **Extend `RegisterCapability` with `Discoverable bool`** | The existing `capabilityRegistry` is the single source of truth for capability kinds. Adding a `Discoverable` field to the registration metadata means new kinds declare their discoverability at registration time. No second source of truth (no DB table, no separate slice). Store queries filter by discoverable kinds via a helper function. |
| Kind labels in UI | **Frontend maps kind strings to display labels** | Registry holds only `Factory` + `Discoverable`. Frontend maps `"a2a.skill"` -> `"A2A Skill"` etc. Less Go code change, and frontend needs kind-specific rendering logic anyway (icons, colors). |
| Backend data model | **Flat list** — no server-side aggregation | Two agents with `a2a.skill` named "search" can do completely different things. Server-side GROUP BY `(kind, name)` creates false equivalence. Backend returns each capability instance with its parent agent. Frontend groups visually. |
| Frontend grouping | **Accordion-style** grouping by `(kind, name)` with agent count in header | Preserves the capability-first mental model ("find translate -> see 4 agents") without backend making assumptions about capability identity. Groups are computed client-side from the flat list. |
| Detail view | **Separate page** at `/catalog/capabilities/{key}` | Shareable URL for "all agents offering this capability". Needed for cross-linking from agent detail page. |
| Detail URL routing | **Single path param** with `::` separator: `/capabilities/{key}` where `key = kind::name` | Chi router can't reliably split two path params when `kind` contains dots and `name` may contain slashes. Single param with `::` separator is unambiguous — capability names don't contain `::`. |
| Breaking change `/skills` | **Remove immediately** | AgentLens is pre-1.0, API is not stable. Document in CHANGELOG. |
| Paginacja | **Flat pagination** (limit/offset on capability instances) | Groups may be split across pages. This is acceptable — frontend shows "continued from previous page" indicator. Simple REST pagination. |
| Offline agents | **Show in detail with badge**, exclude from index counts | Users want reachability counts in index. Detail view shows full picture including offline/deprecated agents with status badges. |
| Query implementation | SQL JOINs on the relational `capabilities` table | Capabilities are NOT in a JSON column. They live in a `capabilities` table with `(agent_type_id, kind, name)` unique index. Standard SQL JOINs apply. |
| Dialect handling | GORM-portable queries, no dialect branching needed | Flat list query needs no `GROUP_CONCAT`/`STRING_AGG`. Simple SELECT + JOIN + WHERE + ORDER BY — fully portable. |
| `examples` field | Not included | `A2ASkill` has no `examples` field. The struct has: `Name`, `Description`, `Tags`, `InputModes`, `OutputModes`. Nothing else. |
| Handler placement | New `CapabilityHandler` struct in `internal/api/` | Follows the pattern of `HealthHandler`, `UserHandler`, `RoleHandler`. |
| Tags | **Shown in index view** | Tags help quickly assess what a capability does without clicking. Requires parsing `properties` JSON for `a2a.skill` rows, but `rowToCapability()` already does this. |
| Case sensitivity | **Case-insensitive** search using `LOWER()` | The existing `SearchCapabilities` is case-sensitive (inconsistent with `List`). New endpoint fixes this. |

---

## Scope (in)

- New route `/catalog/capabilities` — dedicated "Capabilities" tab in the main navigation
- Capability index view: flat list of capability instances, visually grouped by `(kind, name)` in accordion-style layout, with agent count in each group header
- Capability search box filtering by name, tags (A2A skills only), description — case-insensitive, 250ms debounce
- Kind filter: All + one option per discoverable kind (initially: A2A Skill, MCP Tool, MCP Resource, MCP Prompt)
- Capability detail page: all agents offering a specific capability, with health, status, provider, and agent-specific capability snippet
- Cross-link from agent detail page: clicking a capability navigates to the capability detail page
- Backend: flat list endpoint + detail endpoint
- URL state: `?q=translate&kind=a2a.skill`, shareable

## Scope (out)

- Technical capability kinds (`a2a.extension`, `a2a.security_scheme`, `a2a.interface`, `a2a.signature`) — not discoverable
- Capability versioning / compatibility ranges — backlog
- Capability "similarity" or semantic matching — feature 3.3
- Capability-level health (only agent-level health matters today)
- Capability ratings, reviews, usage stats — backlog
- Per-capability security requirements display — feature 2.5 handles security
- Server-side aggregation / GROUP BY — frontend groups, not backend
- Dedicated capabilities aggregation table — capabilities stay in the existing `capabilities` table
- Capability kind registry in database — kinds are registered in Go code via `RegisterCapability`, not in a DB table

---

## Backend Changes

### 1. Extend capability registry with discoverability metadata

File: `internal/model/capability.go`

The existing registry maps kind strings to factory functions. Extend it to carry metadata:

```go
// CapabilityMeta holds registration metadata for a capability kind.
type CapabilityMeta struct {
    Factory      func() Capability
    Discoverable bool // true = user-facing, shown in capability discovery UI
}

var capabilityRegistry = map[string]CapabilityMeta{}

func RegisterCapability(kind string, factory func() Capability, discoverable bool) {
    capabilityRegistry[kind] = CapabilityMeta{
        Factory:      factory,
        Discoverable: discoverable,
    }
}

// GetCapabilityFactory returns the factory for a given kind (unchanged behavior).
func GetCapabilityFactory(kind string) (func() Capability, bool) {
    m, ok := capabilityRegistry[kind]
    return m.Factory, ok
}

// DiscoverableKinds returns the kind strings where Discoverable == true.
func DiscoverableKinds() []string {
    var kinds []string
    for kind, meta := range capabilityRegistry {
        if meta.Discoverable {
            kinds = append(kinds, kind)
        }
    }
    sort.Strings(kinds) // deterministic order
    return kinds
}
```

Update all `RegisterCapability` call sites with the `discoverable` parameter:

File: `internal/model/a2a_capabilities.go` — `init()`:
```go
RegisterCapability("a2a.skill", func() Capability { return &A2ASkill{} }, true)
RegisterCapability("a2a.interface", func() Capability { return &A2AInterface{} }, false)
RegisterCapability("a2a.security_scheme", func() Capability { return &A2ASecurityScheme{} }, false)
RegisterCapability("a2a.extension", func() Capability { return &A2AExtension{} }, false)
RegisterCapability("a2a.signature", func() Capability { return &A2ASignature{} }, false)
```

File: `internal/model/mcp_capabilities.go` — `init()`:
```go
RegisterCapability("mcp.tool", func() Capability { return &MCPTool{} }, true)
RegisterCapability("mcp.resource", func() Capability { return &MCPResource{} }, true)
RegisterCapability("mcp.prompt", func() Capability { return &MCPPrompt{} }, true)
```

**Why this approach:**
- Adding a new discoverable capability kind = add struct + `RegisterCapability("new.kind", factory, true)` in `init()`. One place, one line change.
- The store query calls `model.DiscoverableKinds()` to get the WHERE IN list — no hardcoded kind strings in the store layer.
- No database migration needed. No second source of truth.
- The existing `GetCapabilityFactory` behavior is unchanged — deserialization still works.
- `DiscoverableKinds()` returns a sorted slice for deterministic SQL generation.

### 2. New model types

File: `internal/model/capability_instance.go`

```go
// CapabilityInstance represents a single capability from a single agent,
// enriched with agent metadata for the discovery view.
type CapabilityInstance struct {
    // Capability fields
    Kind        string   `json:"kind"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Tags        []string `json:"tags,omitempty"`
    InputModes  []string `json:"input_modes,omitempty"`
    OutputModes []string `json:"output_modes,omitempty"`

    // Parent agent fields (subset — not the full CatalogEntry)
    AgentID     string         `json:"agent_id"`
    AgentName   string         `json:"agent_name"`
    Protocol    Protocol       `json:"protocol"`
    Status      LifecycleState `json:"status"`
    SpecVersion string         `json:"spec_version,omitempty"`

    // Provider (flattened)
    ProviderOrg string `json:"provider_org,omitempty"`
    ProviderURL string `json:"provider_url,omitempty"`

    // Health (subset)
    HealthState LifecycleState `json:"health_state"`
    LatencyMs   int64          `json:"latency_ms"`
}

// CapabilityListResult wraps a paginated list of capability instances.
type CapabilityListResult struct {
    Total int                   `json:"total"`
    Items []CapabilityInstance  `json:"items"`
}
```

Notes:
- `CapabilityInstance` is a **flat DTO** combining capability fields with agent metadata. It is not a GORM model — it is built in the store layer from JOIN query results.
- `Tags`, `InputModes`, `OutputModes` are populated only for `a2a.skill` kind by parsing the `properties` JSON column. For MCP kinds, these are nil.
- Agent fields are a curated subset — not the full `CatalogEntry` with all its JSON columns. This keeps response sizes small.
- `Protocol` uses `model.Protocol`, `Status` and `HealthState` use `model.LifecycleState` — reuse existing domain types instead of raw strings. This ensures compile-time safety and consistency with the rest of the codebase where `CatalogEntry.Status`, `Health.State`, `AgentType.Protocol`, `catalogEntryJSON.Status`, and `catalogEntryJSON.Protocol` all use their typed aliases.

### 3. New endpoint — GET /api/v1/capabilities

Replaces the existing `GET /api/v1/skills` endpoint.

Query params:

| Param | Type | Default | Behavior |
|-------|------|---------|----------|
| `q` | string | empty | Case-insensitive substring match against `capabilities.name`, `capabilities.description`, and `capabilities.properties` (for tags in A2A skills). Uses `LOWER()` on both sides. |
| `kind` | string | empty | Filter by capability kind. Allowed values: any discoverable kind (validated against `model.DiscoverableKinds()`), or empty (all discoverable kinds). Unknown values return 400. |
| `limit` | int | 50 | Max items returned |
| `offset` | int | 0 | Pagination offset |
| `sort` | enum | `name_asc` | Allowed: `name_asc`, `agentName_asc` |

Response shape:

```json
{
  "total": 342,
  "items": [
    {
      "kind": "a2a.skill",
      "name": "Translate EN-DE",
      "description": "Bidirectional translation between English and German",
      "tags": ["translation", "german", "english"],
      "input_modes": ["text/plain"],
      "output_modes": ["text/plain"],
      "agent_id": "entry-uuid-1",
      "agent_name": "Translation Agent",
      "protocol": "a2a",
      "status": "active",
      "spec_version": "1.0",
      "provider_org": "Acme",
      "provider_url": "https://acme.io",
      "health_state": "active",
      "latency_ms": 142
    },
    {
      "kind": "a2a.skill",
      "name": "Translate EN-DE",
      "description": "Translation between English and German with context",
      "tags": ["translation", "german"],
      "input_modes": ["text/plain", "application/json"],
      "output_modes": ["text/plain"],
      "agent_id": "entry-uuid-2",
      "agent_name": "Polyglot Agent",
      "protocol": "a2a",
      "status": "active",
      "spec_version": "1.1",
      "provider_org": "LangCorp",
      "provider_url": null,
      "health_state": "active",
      "latency_ms": 89
    },
    {
      "kind": "mcp.tool",
      "name": "search_documents",
      "description": "Full-text search across indexed documents",
      "tags": null,
      "input_modes": null,
      "output_modes": null,
      "agent_id": "entry-uuid-3",
      "agent_name": "DocSearch Server",
      "protocol": "mcp",
      "status": "active",
      "spec_version": "",
      "provider_org": "SearchInc",
      "provider_url": "https://searchinc.dev",
      "health_state": "active",
      "latency_ms": 200
    }
  ]
}
```

Notes:
- Each item is one capability from one agent — **no aggregation**. Two agents offering `a2a.skill` "Translate EN-DE" produce two separate items.
- The `sort=name_asc` default groups items by capability name naturally, making frontend grouping straightforward.
- Only capabilities from **active + degraded** agents are returned. Offline and deprecated agents are excluded from the list endpoint. (The detail endpoint shows all lifecycle states.)
- Only capabilities with discoverable kinds (per `model.DiscoverableKinds()`) are returned. Technical kinds are filtered out at the store level via a `WHERE kind IN (...)` clause using the registry.
- `total` is the count of matching capability instances (not unique capability names).

### 4. New endpoint — GET /api/v1/capabilities/{key}

Returns all agents offering a specific capability. The `key` path parameter uses the format `kind::name` (URL-encoded). Examples:
- `/api/v1/capabilities/a2a.skill::Translate%20EN-DE`
- `/api/v1/capabilities/mcp.tool::search_documents`

The backend splits `key` on the first `::` occurrence to extract `kind` and `name`. If the key does not contain `::`, return 400.

Response:

```json
{
  "capability": {
    "kind": "a2a.skill",
    "name": "Translate EN-DE"
  },
  "agents": [
    {
      "id": "entry-uuid-1",
      "display_name": "Translation Agent",
      "protocol": "a2a",
      "provider": { "organization": "Acme", "url": "https://acme.io" },
      "health": { "state": "active", "latencyMs": 142 },
      "spec_version": "1.0",
      "status": "active",
      "capability_snippet": {
        "kind": "a2a.skill",
        "name": "Translate EN-DE",
        "description": "Bidirectional translation between English and German",
        "tags": ["translation", "german", "english"],
        "inputModes": ["text/plain"],
        "outputModes": ["text/plain"]
      }
    },
    {
      "id": "entry-uuid-4",
      "display_name": "Legacy Translator",
      "protocol": "a2a",
      "provider": null,
      "health": { "state": "offline", "latencyMs": 0 },
      "spec_version": "0.9",
      "status": "offline",
      "capability_snippet": {
        "kind": "a2a.skill",
        "name": "Translate EN-DE",
        "description": "EN/DE translation (legacy)",
        "tags": ["translation"],
        "inputModes": ["text/plain"],
        "outputModes": ["text/plain"]
      }
    }
  ]
}
```

Notes:
- `capability_snippet` contains the **full capability instance** for that specific agent, because two agents may offer the same capability name with different descriptions, inputModes, or outputModes. The snippet is built by finding the matching `(kind, name)` capability in the agent's `AgentType.Capabilities` slice and marshaling it via the existing `MarshalCapabilitiesJSON` pattern (which injects the `kind` field).
- The response includes agents in **all lifecycle states** (active, degraded, offline, deprecated). Offline/deprecated agents are shown with their status so users can see "this capability used to be offered by Foo, but Foo is currently down".
- If no agents offer the requested capability, return **404** via `ErrorResponse` — the resource does not exist. Not 200 with empty array.
- The `health` sub-object reuses the `healthJSON` shape from `CatalogEntry.MarshalJSON()` (camelCase keys: `latencyMs`, `lastProbedAt`, etc.).

Permission: any authenticated user with `catalog:read` permission (viewer and up).

### 5. Store layer

No migration needed. Both queries are answered from the existing `capabilities` table joined to `agent_types` and `catalog_entries`.

Add two methods and one filter type to the Store interface:

File: `internal/store/store.go`

```go
// CapabilityFilter holds filtering parameters for listing capability instances.
type CapabilityFilter struct {
    Query  string // case-insensitive substring match against name, description, properties
    Kind   string // filter by capability kind (e.g., "a2a.skill", "mcp.tool")
    Limit  int
    Offset int
    Sort   string // "name_asc" (default) | "agentName_asc"
}
```

Add to `Store` interface:

```go
type Store interface {
    // ...existing methods...

    // ListCapabilities returns a flat list of capability instances (one per
    // agent per capability) with agent metadata. Only active + degraded entries
    // are included. Only user-facing capability kinds are returned.
    ListCapabilities(ctx context.Context, filter CapabilityFilter) (*model.CapabilityListResult, error)

    // ListAgentsByCapability returns catalog entries offering a specific
    // capability identified by (kind, name). Returns all lifecycle states.
    ListAgentsByCapability(ctx context.Context, kind, name string) ([]model.CatalogEntry, error)
}
```

Remove existing method:

```go
// SearchCapabilities is REMOVED — replaced by ListCapabilities.
// SearchCapabilities(ctx context.Context, query string) ([]model.CatalogEntry, error)
```

### 6. Store implementation

File: `internal/store/sql_store_query.go` (extend existing file)

**`ListCapabilities` implementation:**

The query is a flat SELECT + JOIN — no GROUP BY, no dialect branching needed:

```sql
SELECT
    c.kind, c.name, c.description, c.properties,
    ce.id AS agent_id, ce.display_name AS agent_name,
    at.protocol, at.spec_version,
    ce.status,
    p.organization AS provider_org, p.url AS provider_url,
    ce.health_latency_ms, ce.status AS health_state
FROM capabilities c
JOIN agent_types at ON c.agent_type_id = at.id
JOIN catalog_entries ce ON ce.agent_type_id = at.id
LEFT JOIN providers p ON at.provider_id = p.id
WHERE c.kind IN ?  -- model.DiscoverableKinds()
  AND ce.status IN ('active', 'degraded')
  [AND LOWER(c.name) LIKE ? OR LOWER(c.description) LIKE ? OR LOWER(c.properties) LIKE ?]
  [AND c.kind = ?]
ORDER BY c.name ASC, ce.display_name ASC
LIMIT ? OFFSET ?
```

Implementation notes:
- Use GORM query builder, not raw SQL. The pseudo-SQL above illustrates the intent.
- The `properties` column is searched via `LOWER(c.properties) LIKE ?` — this catches tags stored as JSON (e.g., `{"tags":["translation","german"]}`). Not ideal but sufficient for MVP volumes.
- After the query, parse `properties` in Go to extract `tags`, `inputModes`, `outputModes` for `a2a.skill` rows. Use `json.Unmarshal` into a `map[string]any` and extract the fields. For non-`a2a.skill` kinds, leave these fields nil.
- Total count: run a separate `COUNT(*)` query with the same WHERE filters (without LIMIT/OFFSET), following the `Stats()` pattern.
- The query result is scanned into a struct that maps to `CapabilityInstance`. This avoids loading full `CatalogEntry` objects with all their JSON columns.

Define a scan struct in `sql_store_query.go`:

```go
type capabilityInstanceRow struct {
    Kind        string
    Name        string
    Description string
    Properties  string
    AgentID     string
    AgentName   string
    Protocol    string
    SpecVersion string
    Status      string
    ProviderOrg *string
    ProviderURL *string
    LatencyMs   int64
    HealthState string
}
```

Convert to `model.CapabilityInstance` after scanning, extracting tags/inputModes/outputModes from `properties` for `a2a.skill` rows.

**`ListAgentsByCapability` implementation:**

1. SELECT `catalog_entries.*` joined through `agent_types` and `capabilities`
2. WHERE `capabilities.kind = ? AND capabilities.name = ?`
3. Preload `AgentType`, `AgentType.Provider`
4. Load capabilities via `loadCapabilities()` for each entry
5. Call `SyncFromDB()` for each entry
6. Return all lifecycle states — the handler does not filter

### 7. Handler layer

File: `internal/api/capability_handlers.go`

New handler struct following the project pattern:

```go
type CapabilityHandler struct {
    store store.Store
}

func NewCapabilityHandler(s store.Store) *CapabilityHandler {
    return &CapabilityHandler{store: s}
}
```

Two handler methods:

**`ListCapabilities(w, r)`** — handles `GET /api/v1/capabilities`:
- Parses `q`, `kind`, `limit`, `offset`, `sort` from query params
- Validates `kind` against `model.DiscoverableKinds()` — if non-empty and not in the discoverable set, returns 400
- Validates `sort` against a whitelist: `name_asc`, `agentName_asc`
- Returns 400 on unknown values
- Calls `h.store.ListCapabilities(ctx, filter)`
- Returns `model.CapabilityListResult` via `JSONResponse`

**`GetCapabilityAgents(w, r)`** — handles `GET /api/v1/capabilities/{key}`:
- Extracts `key` from URL path via `chi.URLParam(r, "key")`
- URL-decodes `key`, splits on first `::` to get `kind` and `name`
- If `key` does not contain `::`, returns 400
- Calls `h.store.ListAgentsByCapability(ctx, kind, name)`
- If no entries found, returns 404 via `ErrorResponse`
- Builds the response DTO:
  - `capability`: `{kind, name}` summary
  - `agents`: for each `CatalogEntry`, build an unexported `capabilityAgentDTO` struct with the curated fields + `capability_snippet`
  - The `capability_snippet` is extracted by iterating the agent's `AgentType.Capabilities` slice, finding the one where `Kind() == kind && name matches`, and marshaling it to `json.RawMessage` via the existing marshal pattern

Response DTO structs (unexported, in `capability_handlers.go`):

```go
type capabilityDetailResponse struct {
    Capability capabilitySummary   `json:"capability"`
    Agents     []capabilityAgentDTO `json:"agents"`
}

type capabilitySummary struct {
    Kind string `json:"kind"`
    Name string `json:"name"`
}

type capabilityAgentDTO struct {
    ID               string           `json:"id"`
    DisplayName      string           `json:"display_name"`
    Protocol         string           `json:"protocol"`
    Provider         *model.Provider  `json:"provider,omitempty"`
    Health           healthJSON       `json:"health"`
    SpecVersion      string           `json:"spec_version,omitempty"`
    Status           string           `json:"status"`
    CapabilitySnippet json.RawMessage `json:"capability_snippet"`
}
```

Note: `healthJSON` is already defined in `internal/model/agent.go` (lines 176-183) but is unexported. Either export it (rename to `HealthJSON`) or duplicate the struct in the handler file. Recommendation: export it in the model package since `health_handlers.go` already has a similar `healthToDTO()` function — consolidating avoids drift.

### 8. Router wiring

File: `internal/api/router.go`

In `registerCatalogRoutes()`, replace the existing skills route and add the new capability routes:

```go
// Remove:
r.With(RequirePermission(auth.PermCatalogRead)).Get("/skills", h.SearchCapabilities)

// Add:
capHandler := NewCapabilityHandler(deps.Kernel.Store())
r.With(RequirePermission(auth.PermCatalogRead)).Get("/capabilities", capHandler.ListCapabilities)
r.With(RequirePermission(auth.PermCatalogRead)).Get("/capabilities/{key}", capHandler.GetCapabilityAgents)
```

Same changes in `registerUnauthenticatedCatalogRoutes()` for the unauthenticated mode.

Remove the `SearchCapabilities` method from `handlers.go` (lines 320-332) and `SearchCapabilities` from `sql_store_query.go` (lines 92-116).

---

## Frontend Changes

### Navigation

Add a new top-level tab in the main nav (`Layout.tsx`), next to the existing "Catalog" tab: "Capabilities". Icon: `lucide-react` `Sparkles` or `Zap` (pick whichever matches the existing visual language — do not introduce a third icon pack).

### Route layout

```
web/src/routes/capabilities/
  CapabilityListPage.tsx          (capability index — grouped list + search)
  CapabilityDetailPage.tsx        (capability detail — list of agents)
  components/
    CapabilityGroup.tsx           (accordion group header + expanded agent list)
    CapabilityInstanceCard.tsx    (one capability instance within a group)
    KindFilter.tsx                (toggle group filter for capability kinds)
    AgentForCapabilityRow.tsx     (table row in the detail view)
web/src/hooks/
  useCapabilitiesQuery.ts         (React Query hook with URL sync)
```

API client: extend `web/src/api.ts` with new functions:
- `listCapabilities(filter)` — `GET /api/v1/capabilities?...`
- `getCapabilityAgents(key)` — `GET /api/v1/capabilities/{key}`

Reuse `UnifiedSearchBox` from feature 2.2 — pass different placeholder text and `onChange`. If it is too coupled to the catalog, extract a base `DebouncedSearchBox` component and have both views use it. Do not duplicate the debounce + URL-sync logic.

### TypeScript types

Add to `web/src/types.ts`:

```typescript
interface CapabilityInstance {
  kind: string
  name: string
  description: string
  tags: string[] | null
  input_modes: string[] | null
  output_modes: string[] | null
  agent_id: string
  agent_name: string
  protocol: string
  status: string
  spec_version: string
  provider_org: string | null
  provider_url: string | null
  health_state: string
  latency_ms: number
}

interface CapabilityListResult {
  total: number
  items: CapabilityInstance[]
}

interface CapabilityDetailResponse {
  capability: { kind: string; name: string }
  agents: CapabilityAgentDTO[]
}

interface CapabilityAgentDTO {
  id: string
  display_name: string
  protocol: string
  provider: { organization: string; url: string } | null
  health: { state: string; latencyMs: number; [key: string]: unknown }
  spec_version: string
  status: string
  capability_snippet: Record<string, unknown>
}
```

### Capability index view (`/catalog/capabilities`)

Header: title "Capabilities" + subtitle "Discover agents by capability". Search box below, filter bar with `KindFilter` (All / A2A Skill / MCP Tool / MCP Resource / MCP Prompt). Uses shadcn `ToggleGroup` like `ProtocolFilter`.

**Data flow:**
1. `useCapabilitiesQuery` hook fetches flat list from backend via React Query
2. Frontend groups items by `(kind, name)` using a simple `reduce()` into a `Map<string, CapabilityInstance[]>`
3. Groups are sorted by name (matching backend sort order)
4. Each group is rendered as a `CapabilityGroup` accordion item

**`CapabilityGroup` component (accordion):**
- Header bar shows:
  - Kind badge (e.g., "A2A Skill", "MCP Tool") using shadcn `Badge`
  - Capability name as headline
  - First line of description from the first instance (truncated)
  - Tag pills (max 5 visible, "+N more") using shadcn `Badge` — only for `a2a.skill`
  - Agent count: `N agents` on the right
- Click header to expand/collapse (shadcn `Collapsible` or custom accordion)
- Expanded state shows the list of agents offering this capability:
  - Each agent: name, protocol badge (reuse `ProtocolBadge`), status badge (reuse `StatusBadge`), provider org, latency
  - Click agent name -> navigate to `/catalog/{agent_id}`
  - Click "View all" link -> navigate to detail page `/catalog/capabilities/{encodeURIComponent(kind + '::' + name)}`

Empty / loading / error states mirror 2.2: skeleton groups while loading, "No capabilities found" with clear filters button if filters exclude all, "No capabilities published yet" if the catalog has no user-facing capabilities.

Pagination: "Load more" button at the bottom (offset-based). Loads next page and appends. If a group is split across pages, the new items merge into the existing group.

### Capability detail view (`/catalog/capabilities/:key`)

Route: `/catalog/capabilities/:key` where `key` is `kind::name` (URL-encoded).

Breadcrumb: Catalog > Capabilities > `<capability name>`. Back button.

Top section:
- Kind badge
- Capability name as h1
- Agent count (total agents, including offline/deprecated since this is detail view)

Bottom section: table of agents offering this capability. Columns:
1. Protocol — `ProtocolBadge`
2. Agent name — clickable, navigates to `/catalog/:id`
3. Provider — organization name
4. Status — `StatusBadge` (shows offline/deprecated badges clearly)
5. Spec version
6. This capability's details — agent-specific `capability_snippet.description` truncated to 100 chars with tooltip for full text
7. Latency — from health data

Clicking a row navigates to the agent detail page.

### Cross-linking from agent detail

On the existing agent detail page (`CatalogDetailPage.tsx`), capabilities are listed in a section titled "Capabilities (N)". Each capability name should now become a clickable link that navigates to `/catalog/capabilities/${encodeURIComponent(kind + '::' + name)}`. Use shadcn `Button variant="link"` or a plain anchor. Only link user-facing capability kinds — technical kinds remain plain text.

### URL state

Same contract as 2.2: URL is the source of truth. `?q=`, `?kind=`, `?sort=` all sync to `useSearchParams`. Shareable link is an acceptance test.

### Route registration

File: `web/src/App.tsx`

Add two new routes inside the protected layout:

```tsx
<Route path="/catalog/capabilities" element={<CapabilityListPage />} />
<Route path="/catalog/capabilities/:key" element={<CapabilityDetailPage />} />
```

---

## Tests

### Backend

**Registry tests** (in `internal/model/`):
- `DiscoverableKinds()` returns exactly the 4 discoverable kinds (sorted)
- `DiscoverableKinds()` does not include technical kinds
- `GetCapabilityFactory` still works for all 8 kinds (backward compatibility)

**Store tests** (using `store.NewSQLiteStore(":memory:")`):
- Two entries each with `a2a.skill` named "Translate" -> `ListCapabilities` returns two items (one per agent), not one aggregate
- Same capability name but different kind (e.g., `a2a.skill` "search" vs `mcp.tool` "search") -> both appear as separate items
- `q="translate"` matches name, description, and properties (tags) — case-insensitive — separately verified
- `kind="a2a.skill"` filters out MCP tools
- Only active + degraded entries appear in `ListCapabilities` results — seed one active and one offline, verify offline absent
- Technical capability kinds (registered with `discoverable=false`) never appear in `ListCapabilities` results
- `ListAgentsByCapability` returns only entries with the exact `(kind, name)` match
- `ListAgentsByCapability` includes all lifecycle states (including offline/deprecated)
- `ListAgentsByCapability` for non-existent capability returns empty slice
- `sort=name_asc` and `sort=agentName_asc` both produce correct ordering
- Pagination: `limit=2&offset=2` returns the correct window
- Total count in `CapabilityListResult` reflects filters correctly
- Tags, inputModes, outputModes populated for `a2a.skill` items, nil for MCP items

**Handler tests** (following existing handler test patterns):
- `GET /api/v1/capabilities` with no params returns items sorted by name
- Unknown `kind` -> 400
- Unknown `sort` -> 400
- `GET /api/v1/capabilities/{key}` with non-existent capability -> 404
- `GET /api/v1/capabilities/{key}` with malformed key (no `::`) -> 400
- Viewer role can list and read
- Response includes correct `capability_snippet` per agent in detail endpoint
- Detail endpoint includes offline agents (with offline status)

### Frontend

**Unit (Vitest):**
- `useCapabilitiesQuery` hook: URL round-trip for `q`, `kind`, `sort`
- `CapabilityGroup`: renders correct agent count in header, expands on click
- `KindFilter`: updates URL on toggle
- Grouping logic: correctly groups flat items by `(kind, name)`, handles split groups from pagination

**Playwright E2E:**
1. Seed 3 A2A entries and 1 MCP entry, each with overlapping capabilities (at least one `a2a.skill` shared across three agents, at least one `mcp.tool` shared across two)
2. Navigate to `/catalog/capabilities` -> see accordion groups with correct agent counts
3. Type `translate` in search -> groups filter in real time
4. Click kind filter "A2A Skill" -> MCP tools disappear
5. Clear filters -> all capabilities return
6. Expand an accordion group -> see agents listed
7. Click "View all" -> detail view opens with correct URL
8. Verify the agent list on detail shows agents offering that capability (including offline if seeded)
9. Click an agent row -> navigates to that agent's detail page
10. From the agent detail page, click a capability name -> navigates to `/catalog/capabilities/{key}`
11. Reload the detail page -> state persists via URL

---

## Acceptance Criteria

- "Capabilities" tab visible in main navigation
- `/catalog/capabilities` shows capability instances grouped by `(kind, name)` with agent counts in accordion headers
- Search box filters by name, tags, and description in real time (case-insensitive)
- Kind filter works, state survives reload via URL
- Capability detail page shows all agents offering the capability with health, status, provider, and agent-specific snippet
- Agent-specific `capability_snippet` shown per row (not a merged/aggregated description)
- Capability names on the agent detail page are clickable and navigate to capability detail
- Offline and deprecated agents excluded from index view, included in detail view with status badge
- Technical capability kinds (registered with `discoverable=false`) excluded from discovery
- `RegisterCapability` accepts `discoverable` parameter; `DiscoverableKinds()` returns only discoverable kinds
- Adding a new discoverable kind requires only a `RegisterCapability` call change + frontend label mapping — no store/handler changes
- Shareable URL `?q=translate&kind=a2a.skill` reproduces the exact view
- SQLite store tests pass (Postgres tests deferred until enterprise plugin is active)
- Playwright E2E green on CI
- Empty states present for: no capabilities at all, no capabilities matching filter, no agents offering a specific capability (404)
- `docs/api.md` updated for new endpoints
- `docs/end-user-guide.md` updated for the new Capabilities tab
- `GET /api/v1/skills` endpoint removed

---

## Known Traps

- **Do NOT add a separate capabilities aggregation table.** The flat list is queried from the existing `capabilities` table. No new tables.
- **Do NOT aggregate on the backend.** GROUP BY `(kind, name)` creates false equivalence — two agents with skill "search" can do completely different things. Backend returns flat list, frontend groups visually.
- **Do NOT rename `CatalogEntry` to `Agent`.** This rename trap has been avoided six features in a row. Keep the streak.
- **Do NOT merge `capability_snippet` into an aggregated capability.** Each agent's instance is distinct. Show them separately.
- **Do NOT include technical capability kinds** in discovery results. Use `model.DiscoverableKinds()` to filter — do not hardcode kind strings in the store or handler layer.
- **Do NOT hardcode kind strings in the store query.** Use `model.DiscoverableKinds()` which reads from the capability registry. Adding a new discoverable kind should require only a `RegisterCapability` call change, not store/handler code changes. They are configuration details.
- **Do NOT include offline agents in the index view.** Only active + degraded. Detail view shows all.
- **Do NOT fetch all capabilities on every keystroke.** Reuse the 250ms debounce pattern from 2.2's search box.
- **Do NOT duplicate the `UnifiedSearchBox` component.** Extract a base version if needed.
- **Do NOT introduce a new frontend library** (e.g., fuse.js for fuzzy search). Server-side `LIKE` is enough, same as 2.2.
- **Do NOT embed full `CatalogEntry` objects in the flat list response.** Use the curated `CapabilityInstance` DTO with minimal agent fields. Large responses kill performance.
- **Do NOT gate 2.3 behind the enterprise license.** It is OSS Core.
- **Do NOT add capability-level health.** Health lives on agents. This is an explicit scope boundary.
- **Do NOT use `json_each` or `jsonb_array_elements`.** Capabilities are in a relational table. Standard SQL JOINs.
- **Do NOT put capabilities on `CatalogEntry`.** Capabilities belong to `AgentType` (Product Archetype). Join path: `catalog_entries -> agent_types -> capabilities`.
- **Do NOT reference an `examples` field on `A2ASkill`.** It does not exist.
- **Do NOT violate arch-go function limits:** max 80 lines per function, 5 params, 3 return values, 10 public functions per file.
- **Do NOT use case-sensitive search.** The existing `SearchCapabilities` is case-sensitive (bug). New endpoint must use `LOWER()` on both sides, matching the `List` query pattern.
- **Do NOT use two path params for the detail URL.** Chi can't reliably split `{kind}/{name}` when kind contains dots. Use single `{key}` with `::` separator.

---

## Execution Order (suggested commit sequence)

1. **Extend capability registry:** Change `RegisterCapability` signature to accept `discoverable bool`. Add `CapabilityMeta` struct and `DiscoverableKinds()` helper to `internal/model/capability.go`. Update all `init()` call sites in `a2a_capabilities.go` and `mcp_capabilities.go`. Update `archetype_test.go`. All existing tests pass.
2. **Model types:** Add `CapabilityInstance` and `CapabilityListResult` to `internal/model/capability_instance.go`.
3. **Store interface extension:** Add `CapabilityFilter`, `ListCapabilities`, `ListAgentsByCapability` to `Store` interface. Remove `SearchCapabilities`. Implement in `sql_store_query.go`. Store tests pass.
4. **Handler:** Add `CapabilityHandler` struct in `internal/api/capability_handlers.go` with `ListCapabilities` and `GetCapabilityAgents` methods + unexported DTO structs. Handler tests pass.
5. **Router wiring:** Replace `/skills` route with `/capabilities` routes in `router.go`. Remove `SearchCapabilities` handler method from `handlers.go`.
6. **Frontend API client + types:** Add `listCapabilities()` and `getCapabilityAgents()` to `web/src/api.ts`. Add TS types to `types.ts`. Add `useCapabilitiesQuery` hook with URL sync + hook tests.
7. **Frontend index view:** Route, `CapabilityGroup` accordion, `KindFilter`, search, filter, grouping logic, empty states.
8. **Frontend detail view:** Route, agent list table, `AgentForCapabilityRow`, capability snippet column.
9. **Cross-linking:** Make discoverable capability names on the agent detail page clickable.
10. **Navigation:** Add "Capabilities" tab to main nav in `Layout.tsx`.
11. **E2E tests** extension in Playwright.
12. **Docs:** Update `docs/api.md` for new/removed endpoints. Update `docs/end-user-guide.md` for the new Capabilities tab. Add entry to CHANGELOG for `/skills` removal.

Each step is its own commit.

---

## Architectural Notes

### Product Archetype alignment

The capability discovery feature follows the Product Archetype separation established in feature 1.4:

- `AgentType` (= ProductType) owns the capabilities. Capabilities are loaded separately via `loadCapabilities()` and attached to `AgentType.Capabilities []Capability`.
- `CatalogEntry` wraps the `AgentType` with catalog concerns (display name, status, health, source). It never directly holds capabilities.
- The `capabilities` table has a FK to `agent_types.id`, not to `catalog_entries.id`.
- The join path for capability-to-catalog queries is: `capabilities.agent_type_id -> agent_types.id -> catalog_entries.agent_type_id`.

### User-facing vs technical capability kinds

Discoverability is declared at registration time via `RegisterCapability(kind, factory, discoverable)`:

| Kind | Struct | Discoverable | Rationale |
|------|--------|--------------|-----------|
| `a2a.skill` | `A2ASkill` | `true` | "What the agent can do" |
| `mcp.tool` | `MCPTool` | `true` | "What the server offers" |
| `mcp.resource` | `MCPResource` | `true` | "What data the server exposes" |
| `mcp.prompt` | `MCPPrompt` | `true` | "What prompts the server provides" |
| `a2a.extension` | `A2AExtension` | `false` | Protocol extension (configuration) |
| `a2a.security_scheme` | `A2ASecurityScheme` | `false` | Auth configuration (feature 2.5) |
| `a2a.interface` | `A2AInterface` | `false` | Binding configuration |
| `a2a.signature` | `A2ASignature` | `false` | Crypto configuration |

Adding a new discoverable capability kind requires:
1. Define a new struct implementing `Capability` interface
2. Call `RegisterCapability("new.kind", factory, true)` in an `init()` function
3. Add parser logic to extract the capability from raw cards
4. Add kind label mapping in the frontend `KindFilter` component

No store, handler, or router changes needed — `DiscoverableKinds()` picks up the new kind automatically.

### Microkernel boundaries

The handler accesses the store via `kernel.Kernel.Store()`, following the existing pattern. No plugin interaction is needed — capability discovery is a pure store query feature.

Per arch-go rules, the new code lives in:
- `internal/model/` — Foundation layer, no internal deps
- `internal/store/` — Infrastructure layer, depends on `model` only
- `internal/api/` — API layer, depends on `store` and `model`

---

## Out-of-band Notes

This feature is small on purpose. After 2.2 (unified catalog view) landed, most of the plumbing already exists. The real work is the flat list query, the frontend grouping logic, and the accordion UI. Resist the temptation to add ratings, reviews, or version ranges — those are separate features for a reason.

The flat list + frontend grouping approach is intentionally simple. If MVP usage shows that server-side aggregation is needed (e.g., catalog grows beyond 10k capabilities and the flat list is too large), that's a follow-up optimization, not a v1 requirement.

After 2.3 lands, 2.5 (A2A Security Schemes) becomes the next logical step: users discovering agents by capability will immediately ask "how do I authenticate to this one?". 2.5 answers that.

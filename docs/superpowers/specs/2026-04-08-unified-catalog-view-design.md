# Feature 2.2 — Unified Catalog View (A2A + MCP in one screen)

**Tier:** 2 — SHOULD HAVE, first post-launch feature
**Effort:** M-L (5-7 days, increased from original 3-5 due to RawCard plugin extraction)
**Date:** 2026-04-08

---

## Goal

Turn the catalog screen into a single unified table listing every discovered resource — A2A agents, MCP servers, and (future) A2UI surfaces — side by side, with protocol filtering, unified search, and a drill-down with the raw card JSON.

## Why

Multi-protocol registry in a single UI is AgentLens's strongest differentiator. No existing tool shows "all your AI agents and MCP servers in one place, filter by protocol, search across all of them." The backend already stores both protocols in the same `catalog_entries` table — the UI just does not yet show them unified.

---

## Decisions Made During Design

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Raw card storage | New `CardStorePlugin` in microkernel | Respects Product Archetype — `RawDefinition` is being extracted from `AgentType` into a dedicated plugin to encapsulate growing complexity (content type, fetch time, truncation, future: history, diffing) |
| List endpoint | Extend existing `GET /api/v1/catalog` | Already supports `protocol` and `q` params. Add `sort`, expand search scope. No new endpoint. |
| Detail page structure | Restructure into Tabs (Overview + Raw Card) | Current detail is a single scrollable card. Tabs provide clean separation for the new Raw Card view. |
| Data fetching | Add `@tanstack/react-query` | App currently uses plain fetch. react-query provides caching, dedup, background refetch, and cleaner hook patterns. |
| Catalog UI approach | Clean rewrite | New route-based file structure under `routes/catalog/`. Old `CatalogList.tsx` and `EntryDetail.tsx` replaced. |
| Syntax highlighting | `prismjs` | Lightweight (~15KB), supports JSON well, good enough for read-only display. No monaco-editor. |

---

## Scope (in)

1. Unified catalog table showing A2A + MCP entries together, sorted by default by `lastSuccessAt DESC`
2. Protocol filter bar: `All` (default), `A2A`, `MCP` — single-select
3. Full-text search across `displayName`, `description`, `capabilities.name`, `capabilities.description`, `categories`, `provider.organization`
4. Per-row display: protocol badge, provider, skill count, health badge, spec version badge, last seen relative time
5. Drill-down detail view: tabbed layout with Overview + Raw Card tab (pretty-printed JSON)
6. Search and filter state synced to URL query params (`?protocol=a2a&q=translate`)
7. Empty states and skeleton loaders
8. RawCard plugin extracting raw definition storage from AgentType

## Scope (out)

- A2UI protocol rendering (no parser yet)
- Semantic/vector search (feature 3.3)
- Saved searches/bookmarks
- Server-side pagination redesign — keep existing limit/offset
- Bulk actions (delete, deprecate many)
- Column customization/show-hide/reordering
- CSV/JSON export
- FTS5 or Postgres `tsvector` — `LIKE`/`ILIKE` is enough for MVP volumes

---

## Backend Changes

### 1. RawCard Plugin (CardStorePlugin)

#### New kernel interface

File: `internal/kernel/plugin.go`

```go
type CardStorePlugin interface {
    Plugin
    StoreCard(ctx context.Context, agentTypeID string, data []byte, contentType string) error
    GetCard(ctx context.Context, agentTypeID string) (*model.RawCard, error)
}
```

New plugin type constant: `PluginTypeCardStore = "cardstore"`

#### New model

File: `internal/model/raw_card.go`

```go
type RawCard struct {
    AgentTypeID string    `json:"agent_type_id"`
    Data        []byte    `json:"data"`
    ContentType string    `json:"content_type"`
    FetchedAt   time.Time `json:"fetched_at"`
    Truncated   bool      `json:"truncated"`
}
```

This is NOT a GORM model — it's a plain struct. The plugin owns the DB schema.

#### Plugin implementation

Location: `plugins/cardstore/plugin.go`

- Plugin name: `"card-store"`
- Plugin type: `PluginTypeCardStore`
- DB table: `raw_cards`
  - `agent_type_id TEXT PRIMARY KEY` (FK to agent_types.id)
  - `data BLOB NOT NULL`
  - `content_type TEXT NOT NULL DEFAULT 'application/json'`
  - `fetched_at TIMESTAMP NOT NULL`
  - `truncated BOOLEAN NOT NULL DEFAULT FALSE`
- `Init`: uses GORM AutoMigrate (idempotent — migration 006 creates the table first, AutoMigrate is a no-op safety net)
- `StoreCard`: enforces 256 KiB cap (truncates and sets `truncated=true`), upserts by `agent_type_id`
- `GetCard`: returns `*model.RawCard` or error if not found

#### Kernel extension

- `Core` gains `cardStore CardStorePlugin` field
- `Core.CardStore() CardStorePlugin` method added to `Kernel` interface
- `PluginManager.InitAll()` detects `CardStorePlugin` and registers via `core.RegisterCardStore()`
- New method `Core.RegisterCardStore(p CardStorePlugin)` in `core_registry.go`

#### Migration 006: Extract raw_definition from agent_types

- **Step 1**: CREATE `raw_cards` table (the migration creates it, not the plugin — the plugin's AutoMigrate is idempotent and serves as a safety net)
- **Step 2**: Copy existing `agent_types.raw_definition` data into `raw_cards` table (INSERT ... SELECT)
- **Step 3**: Drop `raw_definition` column from `agent_types` (dialect-aware: SQLite 3.35+ supports DROP COLUMN, older SQLite requires table rebuild)
- Update `AgentType` model: remove `RawDefinition []byte` field and `RawDefJSON json.RawMessage` field
- Update `catalogEntryJSON`: remove `RawDef json.RawMessage` field (raw card is only served via `/catalog/{id}/card`, never embedded in list responses)
- Remove `SyncRawDefForJSON()` method from `AgentType`
- Update archetype test `TestArchetype_AgentTypeHasRequiredFields`: remove `RawDefinition` from required fields
- Update archetype test `TestArchetype_CatalogEntryHasNoProductFields`: remove `RawDefinition` from product-only fields (it no longer exists on either)

#### Ingestion path changes

- **Discovery** (`discovery/manager.go` `upsert()`): after parsing, call `kernel.CardStore().StoreCard(ctx, at.ID, rawBytes, "application/json")` with the raw bytes that were passed to the parser
- **API** (`api/catalog_helpers.go` `registerAgentType()`): same — store card via plugin after creating the entry
- **Parser** (`Parse(raw []byte) (*AgentType, error)`): signature unchanged. Parsers no longer set `RawDefinition` on the returned `AgentType`. The raw bytes are stored by the caller via the card store plugin.

#### API endpoint changes

`GET /api/v1/catalog/{id}/card` (existing in `handlers.go:232`):
- Change from reading `entry.AgentType.RawDefinition` to calling `kernel.CardStore().GetCard(ctx, entry.AgentTypeID)`
- Set `Content-Type` from `card.ContentType` (was hardcoded `application/json`)
- Add `X-Raw-Card-Fetched-At` response header with ISO 8601 timestamp
- Add `If-None-Match` / weak ETag support using `fetched_at`
- Return 404 with `{"error": "no raw card stored"}` if card not found (legacy entries or plugin not loaded)

### 2. List Endpoint Extension

Extend `GET /api/v1/catalog` handler (`handlers.go:42`):

**New `sort` parameter** added to `ListFilter`:

```go
type ListFilter struct {
    // ...existing fields...
    Sort string // "lastSuccessAt_desc" (default), "displayName_asc", "createdAt_desc"
}
```

Handler validates `sort` values — unknown values return `400 Bad Request`.

**Expanded `q` search** in `sql_store_query.go`:
- Current: `display_name LIKE ? OR description LIKE ?`
- New: also searches `capabilities.name`, `capabilities.description`, `categories` JSON, `providers.organization`
- Implementation: LEFT JOIN capabilities + LEFT JOIN providers, DISTINCT to avoid duplicates
- Case-insensitive via `LOWER()` on both sides (works on SQLite and Postgres)

**Sort implementation** in `sql_store_query.go`:
- `lastSuccessAt_desc`: `ORDER BY health_last_success_at DESC NULLS LAST`
- `displayName_asc`: `ORDER BY display_name ASC` (current default)
- `createdAt_desc`: `ORDER BY created_at DESC`
- Default (empty): `lastSuccessAt_desc`

**Search highlighting** (optional, low effort):
When `q` is non-empty, compute `searchMatch` for each returned entry:
```json
{
  "searchMatch": {
    "field": "capabilities.name",
    "snippet": "...supports translate between en and de..."
  }
}
```
Priority order: displayName > description > capabilities.name > capabilities.description > categories > provider.organization. Computed server-side after the query, not in SQL.

### 3. Handler requires CardStore

`Handler` struct gains access to the card store:
```go
type Handler struct {
    store       store.Store
    parsers     kernel.Kernel
    cardFetcher service.Fetcher
    // kernel reference for CardStore() access
}
```

The handler already holds a `kernel.Kernel` reference via `parsers`. `CardStore()` is accessed via `h.parsers.CardStore()`.

---

## Frontend Changes

### File Layout

```
web/src/
  routes/catalog/
    CatalogListPage.tsx        (new — replaces components/CatalogList.tsx)
    CatalogDetailPage.tsx      (new — replaces components/EntryDetail.tsx)
    components/
      ProtocolFilter.tsx       (new)
      UnifiedSearchBox.tsx     (new)
      CatalogRow.tsx           (new)
      RawCardTab.tsx           (new)
      SearchHighlight.tsx      (new)
      SpecVersionBadge.tsx     (new — extracted reusable component)
  hooks/
    useCatalogQuery.ts         (new — react-query hook with URL sync)
  lib/
    catalogApi.ts              (new — extended API client for catalog)
```

**Old files removed**: `components/CatalogList.tsx`, `components/EntryDetail.tsx`

### New Dependencies

- `@tanstack/react-query` — data fetching, caching, background refetch
- `@radix-ui/react-toggle-group` — for shadcn ToggleGroup component
- `@radix-ui/react-hover-card` — for shadcn HoverCard component
- `prismjs` + `@types/prismjs` — JSON syntax highlighting

### react-query Setup

- Add `QueryClientProvider` wrapping `<App>` in `main.tsx` (or `App.tsx`)
- Default stale time: 30 seconds
- `useCatalogQuery` hook:
  - Reads URL params via `useSearchParams` (react-router-dom v6)
  - Builds `ListFilter` from URL params
  - Calls `listCatalog(filter)` via react-query `useQuery`
  - Query key: `['catalog', { protocol, q, sort, state, limit, offset }]`
  - Exposes `setProtocol()`, `setQuery()`, `setSort()` functions that update URL params
  - URL changes trigger re-render → new query key → auto refetch

### URL State Contract

Single source of truth: the URL. Params:
- `?protocol=a2a` | `?protocol=mcp` | absent (= all)
- `?q=translate`
- `?sort=lastSuccessAt_desc` | `?sort=displayName_asc` | `?sort=createdAt_desc`
- `?state=active,degraded` (existing)
- `?limit=` / `?offset=` (existing)

### Components

**ProtocolFilter**:
- shadcn `ToggleGroup` (single-select)
- Options: `All` (Boxes icon), `A2A` (Bot icon), `MCP` (Plug icon)
- On change: updates `?protocol=` URL param

**UnifiedSearchBox**:
- shadcn `Input` with `Search` lucide icon
- 250ms debounce before URL push
- Clear button (`X`) when non-empty
- Enter bypasses debounce
- Placeholder: `Search across A2A and MCP — name, description, skills, tags, provider...`

**CatalogRow**:
- shadcn `TableRow` + `TableCell`
- Columns:
  1. Protocol — `ProtocolBadge` (existing component, blue=A2A, green=MCP, purple=A2UI)
  2. Name — `displayName` primary, truncated `description` (80 chars) muted underneath, tooltip for full
  3. Provider — `provider.organization` (fallback `—`), link icon if `provider.url`
  4. Skills — pill with count (`12 skills`), `HoverCard` showing first 8 names
  5. Status — `StatusBadge` (existing component, reused)
  6. Spec — `SpecVersionBadge` (new extracted component)
  7. Last seen — relative time from `health.lastSuccessAt`, tooltip with absolute UTC
- Row clickable → navigates to `/catalog/:id`

**SpecVersionBadge**:
- Extracted from inline `<Badge variant="outline">Spec v{entry.spec_version}</Badge>` in current EntryDetail
- Renders nothing if `spec_version` is empty

**SearchHighlight**:
- When `searchMatch.snippet` present, renders below description
- Matched text wrapped in `<mark>` with `bg-yellow-200/40 dark:bg-yellow-500/20 rounded-sm`

**CatalogDetailPage**:
- Restructured into shadcn `Tabs`:
  - **Overview** tab: header (title, description, badges, actions), fields grid, categories, capabilities, health — all current content minus raw definition
  - **Raw Card** tab: `RawCardTab` component

**RawCardTab**:
- Calls `GET /catalog/{id}/card`
- Renders with prismjs syntax-highlighted JSON
- Copy-to-clipboard button in tab header
- Download button → browser download as `<displayName>.json`
- Shows "Fetched at: <relative time>" from `X-Raw-Card-Fetched-At` header
- If truncated (metadata flag): shadcn `Alert` warning

### Empty / Loading / Error States

- **Skeleton**: 8 shadcn `Skeleton` rows during initial load
- **Empty (no data)**: shadcn `Card` with "No catalog entries yet" + `curl` command + copy button
- **Empty (filters exclude all)**: shadcn `Alert` with "No entries match your filters" + Clear filters button
- **Error**: shadcn `Alert variant="destructive"` with retry button

### Keyboard Shortcuts

- `/` — focus search box
- `Up/Down` — navigate rows
- `Enter` — open selected row detail
- `Escape` — leave search focus

Implemented via `useEffect` + `keydown` listener on the list page.

---

## Route Changes

**`App.tsx`** route updates:
```tsx
<Route path="/" element={<CatalogListPage />} />
<Route path="/catalog/:id" element={<CatalogDetailPage />} />
```

### API Client Extension

File: `web/src/lib/catalogApi.ts` (new, or extend existing `api.ts`)

```typescript
interface ListFilter {
  protocol?: string    // 'a2a' | 'mcp' | undefined (= all)
  q?: string
  sort?: string        // 'lastSuccessAt_desc' | 'displayName_asc' | 'createdAt_desc'
  state?: string       // comma-separated lifecycle states
  limit?: number
  offset?: number
}

function listCatalog(filter: ListFilter): Promise<CatalogEntry[]>
function getEntry(id: string): Promise<CatalogEntry>

// getRawCard reads the raw response body as text + extracts Content-Type
// and X-Raw-Card-Fetched-At from response headers (the /card endpoint
// returns raw bytes, not JSON — the frontend reads body + headers separately)
function getRawCard(id: string): Promise<{ data: string; contentType: string; fetchedAt: string }>
```

---

## Testing

### Backend Tests

**Card store plugin tests** (both SQLite and Postgres):
- Store card → GetCard returns identical bytes
- GetCard for nonexistent entry → error
- Card > 256 KiB → stored truncated, `Truncated == true`

**Store query tests**:
- List with `protocol=A2A` returns only A2A entries
- List with `q=translate` matches across all six text fields
- List with `protocol=A2A&q=mcp-thing` returns empty
- Sort `lastSuccessAt_desc`: entries with NULL lastSuccessAt at bottom

**Handler tests**:
- Unknown `protocol` → 400
- Unknown `sort` → 400
- `GET /catalog/{id}/card` → 200 with correct Content-Type
- Same endpoint → 404 for entry without card
- `If-None-Match` with matching ETag → 304
- Viewer role can list, search, filter, and read card

**Parser tests**:
- A2A/MCP parsers return valid `*AgentType` without `RawDefinition` field

**Migration test**:
- Existing entries with raw_definition are migrated to raw_cards table
- AgentType no longer has RawDefinition column

### Frontend Tests (Vitest)

- `useCatalogQuery`: URL ↔ state round-trip
- Each empty state snapshot test
- `ProtocolFilter`: updates URL on toggle
- `UnifiedSearchBox`: 250ms debounce behavior
- `RawCardTab`: renders JSON, copy/download work
- `SpecVersionBadge`: renders for known versions, renders nothing for empty

### E2E Tests (Playwright)

1. Seed 2 A2A + 1 MCP entries
2. Open `/` → see all 3 rows
3. Click A2A filter → 2 rows, URL updates to `?protocol=a2a`
4. Reload → filter persists
5. Type `translate` → matching rows only, `<mark>` visible
6. Click row → detail page
7. Click Raw Card tab → JSON visible, copy works
8. Click All → 3 rows back
9. Keyboard: `/` → search focused, `Escape` → leaves, `Up/Down/Enter` → navigation

---

## Known Traps

1. Do NOT re-serialize the raw card. Store verbatim bytes. Reviewers must see exactly what the agent published.
2. Do NOT rename `CatalogEntry` to `Agent`. The catalog is not just about agents.
3. Do NOT build two separate list endpoints for A2A and MCP. One endpoint, one table.
4. Do NOT use `DisallowUnknownFields` in parsers or DTOs.
5. Do NOT add FTS5 or Postgres `tsvector`. `LIKE`/`ILIKE` for MVP.
6. Do NOT embed raw card bytes in list responses. Only via `/catalog/{id}/card`.
7. Do NOT pull in `monaco-editor`. prismjs is sufficient.
8. Do NOT introduce a new router or state manager. Use `react-router-dom` v6 + react-query.
9. Do NOT bypass existing `StatusBadge` / `ProtocolBadge` components. Import and reuse.
10. Do NOT gate 2.2 behind enterprise license. Unified view is OSS Core.
11. Do NOT store raw card in `Metadata map[string]string`. Card store plugin owns this.
12. Do NOT block list queries on heavy JOINs. If skill search is slow, add denormalized `search_text` in follow-up.
13. Do NOT add `RawDefinition` or equivalent to `CatalogEntry`. Archetype test enforces separation.

---

## Execution Order (suggested commit sequence)

1. `RawCard` model struct + `CardStorePlugin` interface in kernel
2. Card store plugin implementation in `plugins/cardstore/`
3. Migration 006: create `raw_cards` table, copy data, remove `raw_definition` from `agent_types`
4. Update `AgentType` model: remove `RawDefinition`, `RawDefJSON`, `SyncRawDefForJSON()`
5. Update `catalogEntryJSON`: remove `RawDef` field
6. Update archetype tests
7. Update parsers: stop setting `RawDefinition`
8. Update ingestion paths: store card via plugin after parsing
9. Update `GET /catalog/{id}/card` to read from card store plugin
10. Add `sort` param to `ListFilter` and store query
11. Expand `q` search to include capabilities, categories, provider
12. Add search highlighting metadata
13. Install frontend dependencies (react-query, shadcn components, prismjs)
14. Create `useCatalogQuery` hook with URL sync
15. Build `ProtocolFilter` and `UnifiedSearchBox`
16. Build `CatalogRow` and `SpecVersionBadge`
17. Build `CatalogListPage` (clean rewrite)
18. Build empty/loading/error states
19. Build `RawCardTab`
20. Build `CatalogDetailPage` with Tabs (clean rewrite)
21. Add keyboard shortcuts
22. Wire up routes in `App.tsx`, remove old components
23. Frontend unit tests
24. E2E test extension in Playwright
25. Wire card store plugin in `main.go`

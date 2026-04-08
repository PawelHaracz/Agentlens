# Feature 2.2 — Unified Catalog View: Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement task-by-task. Each step is 2–5 min. Steps use `- [ ]` for tracking.

**Goal:** Unified catalog table (A2A + MCP side-by-side), protocol filtering, full-text search across all fields, drill-down detail with raw card JSON tab. Backend extracts raw card storage into a `CardStorePlugin`; removes `AgentType.RawDefinition`.

**Approved spec:** `docs/superpowers/specs/2026-04-08-unified-catalog-view-design.md`

**Tech Stack:** Go 1.26 · GORM (SQLite + Postgres) · chi · React 18 · Tailwind CSS · shadcn/ui · Vitest · Playwright

---

## Design Decisions (carried from spec)

1. `RawDefinition` removed from `AgentType` → dedicated `plugins/cardstore/plugin.go` (CardStorePlugin)
2. `GET /api/v1/catalog` extended: `sort` param + expanded `q` search (capabilities, categories, provider). No new endpoint.
3. Detail page restructured into shadcn Tabs (Overview + Raw Card)
4. `@tanstack/react-query` added for data fetching
5. Catalog UI: clean rewrite under `web/src/routes/catalog/`. Old `CatalogList.tsx` + `EntryDetail.tsx` deleted.
6. Syntax highlighting: `prismjs` (JSON only)

---

## File Map

### Created
| File | Responsibility |
|------|---------------|
| `internal/model/raw_card.go` | `RawCard` plain struct |
| `plugins/cardstore/plugin.go` | CardStorePlugin implementation (SQLite + Postgres) |
| `plugins/cardstore/plugin_test.go` | CardStorePlugin unit tests |
| `web/src/lib/catalogApi.ts` | Extended API client functions |
| `web/src/hooks/useCatalogQuery.ts` | react-query hook with URL sync |
| `web/src/routes/catalog/CatalogListPage.tsx` | Clean-rewrite list page |
| `web/src/routes/catalog/CatalogDetailPage.tsx` | Clean-rewrite detail page (tabbed) |
| `web/src/routes/catalog/components/ProtocolFilter.tsx` | Protocol toggle-group |
| `web/src/routes/catalog/components/UnifiedSearchBox.tsx` | Debounced search input |
| `web/src/routes/catalog/components/CatalogRow.tsx` | Table row component |
| `web/src/routes/catalog/components/RawCardTab.tsx` | Syntax-highlighted raw card |
| `web/src/routes/catalog/components/SearchHighlight.tsx` | `<mark>` snippet renderer |
| `web/src/routes/catalog/components/SpecVersionBadge.tsx` | Spec version badge |
| Various `*.test.tsx` / `*.spec.ts` | Tests |

### Modified
| File | What changes |
|------|-------------|
| `internal/kernel/plugin.go` | Add `CardStorePlugin` interface + `PluginTypeCardStore` constant |
| `internal/kernel/kernel.go` | Add `cardStore CardStorePlugin` field + `CardStore()` to `Core` |
| `internal/kernel/core_registry.go` | Add `RegisterCardStore()` |
| `internal/model/agent_type.go` | Remove `RawDefinition`, `RawDefJSON`, `SyncRawDefForJSON()` |
| `internal/model/agent.go` | Remove `SyncRawDefForJSON()` call in `SyncFromDB()` |
| `internal/model/archetype_test.go` | Remove `RawDefinition` from required/forbidden field lists |
| `internal/db/migrations.go` | Add `migration006RawCards()` |
| `internal/store/store.go` | Add `Sort string` to `ListFilter` |
| `internal/store/sql_store_query.go` | Expand `q` search + implement `sort` |
| `internal/api/handlers.go` | `GetEntryCard` reads from card store; `ListCatalog` adds `sort` param |
| `internal/api/catalog_helpers.go` | `registerAgentType()` stores card via plugin after create |
| `internal/api/import_handler.go` | `ImportCatalogEntry()` stores card via plugin after parse |
| `internal/discovery/manager.go` | `upsert()` stores card via card store after parse |
| `plugins/parsers/a2a/a2a.go` | Remove `RawDefinition: raw` |
| `plugins/parsers/mcp/mcp.go` | Remove `RawDefinition: raw` |
| `cmd/agentlens/main.go` | Register + wire `cardstore.New()` plugin |
| `web/src/types.ts` | Add `sort` to `ListFilter`; add `SearchMatch` interface |
| `web/src/api.ts` | Add `sort` param to `listCatalog()`; add `getRawCard()` |
| `web/src/App.tsx` | Rewire routes to new page components; remove old imports |
| `web/src/main.tsx` | Wrap `<App>` in `QueryClientProvider` |
| `web/package.json` | Add `@tanstack/react-query`, `@radix-ui/react-toggle-group`, `@radix-ui/react-hover-card`, `prismjs`, `@types/prismjs` |
| `e2e/tests/catalog.spec.ts` | Add unified-view E2E scenarios |

### Deleted
| File | Reason |
|------|--------|
| `web/src/components/CatalogList.tsx` | Replaced by `routes/catalog/CatalogListPage.tsx` |
| `web/src/components/EntryDetail.tsx` | Replaced by `routes/catalog/CatalogDetailPage.tsx` |
| `web/src/components/CatalogList.test.tsx` | Replaced by new test file |
| `web/src/components/EntryDetail.test.tsx` | Replaced by new test file |

---

## Task 1 — RawCard model struct

**Files:** Create `internal/model/raw_card.go`

### Why this order
All later tasks depend on this type. No behavior — just a plain struct.

- [ ] **Step 1.1: Create `internal/model/raw_card.go`**

```go
package model

import "time"

// RawCard holds the verbatim card bytes for an AgentType.
// It is owned by the CardStorePlugin — never embedded in AgentType or CatalogEntry.
type RawCard struct {
	AgentTypeID string    `json:"agent_type_id"`
	Data        []byte    `json:"data"`
	ContentType string    `json:"content_type"`
	FetchedAt   time.Time `json:"fetched_at"`
	Truncated   bool      `json:"truncated"`
}
```

- [ ] **Step 1.2: Verify the file compiles**

```bash
rtk go build ./internal/model/...
```

Expected: exits 0 (no compile errors).

- [ ] **Step 1.3: Commit**

```bash
rtk git add internal/model/raw_card.go
rtk git commit -m "feat(model): add RawCard plain struct"
```

---

## Task 2 — CardStorePlugin kernel interface

**Files:** Modify `internal/kernel/plugin.go`, `internal/kernel/kernel.go`, `internal/kernel/core_registry.go`

### Why this order
Must exist before the plugin implementation (Task 3) and the ingestion callers (Task 8).

- [ ] **Step 2.1: Add `PluginTypeCardStore` constant and `CardStorePlugin` interface to `internal/kernel/plugin.go`**

Add after the `PluginTypeStore` constant block:

```go
PluginTypeCardStore PluginType = "cardstore"
```

Add after the `SourcePlugin` interface block:

```go
// CardStorePlugin persists verbatim raw card bytes keyed by AgentTypeID.
type CardStorePlugin interface {
	Plugin
	StoreCard(ctx context.Context, agentTypeID string, data []byte, contentType string) error
	GetCard(ctx context.Context, agentTypeID string) (*model.RawCard, error)
}
```

- [ ] **Step 2.2: Add `cardStore` field and `CardStore()` accessor to `internal/kernel/kernel.go`**

In `Core` struct, add after `middlewares`:
```go
cardStore CardStorePlugin
```

Add `CardStore() CardStorePlugin` to the `Kernel` interface block (after `RegisterMiddleware`):
```go
CardStore() CardStorePlugin
```

Add the method to `Core`:
```go
// CardStore returns the registered card store plugin, or nil if not loaded.
func (c *Core) CardStore() CardStorePlugin { return c.cardStore }
```

- [ ] **Step 2.3: Add `RegisterCardStore()` to `internal/kernel/core_registry.go`**

```go
// RegisterCardStore registers the card store plugin.
func (c *Core) RegisterCardStore(p CardStorePlugin) {
	c.cardStore = p
}
```

- [ ] **Step 2.4: Compile**

```bash
rtk go build ./internal/kernel/...
```

- [ ] **Step 2.5: Commit**

```bash
rtk git add internal/kernel/plugin.go internal/kernel/kernel.go internal/kernel/core_registry.go
rtk git commit -m "feat(kernel): add CardStorePlugin interface + PluginTypeCardStore"
```

---

## Task 3 — CardStorePlugin implementation

**Files:** Create `plugins/cardstore/plugin.go`, `plugins/cardstore/plugin_test.go`

### Why this order
Plugin exists before migration (Task 4) wires the schema. AutoMigrate in Init() is the safety net.

- [ ] **Step 3.1: Write failing tests first — create `plugins/cardstore/plugin_test.go`**

```go
package cardstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/plugins/cardstore"
)

func newTestPlugin(t *testing.T) *cardstore.Plugin {
	t.Helper()
	database, err := db.OpenMemory(db.DialectSQLite)
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := database.DB.DB()
		_ = sqlDB.Close()
	})
	p := cardstore.New(database)
	if err := p.MigrateSchema(context.Background()); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}
	return p
}

func TestCardStore_StoreAndGet(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()
	data := []byte(`{"name":"test-agent"}`)
	if err := p.StoreCard(ctx, "agent-001", data, "application/json"); err != nil {
		t.Fatalf("StoreCard: %v", err)
	}
	card, err := p.GetCard(ctx, "agent-001")
	if err != nil {
		t.Fatalf("GetCard: %v", err)
	}
	if string(card.Data) != string(data) {
		t.Errorf("Data mismatch: got %s, want %s", card.Data, data)
	}
	if card.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want application/json", card.ContentType)
	}
	if card.Truncated {
		t.Error("Truncated should be false for small payload")
	}
}

func TestCardStore_GetNotFound(t *testing.T) {
	p := newTestPlugin(t)
	_, err := p.GetCard(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent card")
	}
}

func TestCardStore_Truncation(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()
	// 257 KiB — exceeds 256 KiB cap
	big := make([]byte, 257*1024)
	for i := range big {
		big[i] = 'x'
	}
	if err := p.StoreCard(ctx, "agent-big", big, "application/json"); err != nil {
		t.Fatalf("StoreCard large: %v", err)
	}
	card, err := p.GetCard(ctx, "agent-big")
	if err != nil {
		t.Fatalf("GetCard large: %v", err)
	}
	if !card.Truncated {
		t.Error("Truncated should be true for oversized payload")
	}
	if len(card.Data) > 256*1024 {
		t.Errorf("Data len %d exceeds 256 KiB cap", len(card.Data))
	}
}

func TestCardStore_UpsertUpdates(t *testing.T) {
	p := newTestPlugin(t)
	ctx := context.Background()
	v1 := []byte(`{"version":"1"}`)
	v2 := []byte(`{"version":"2"}`)
	_ = p.StoreCard(ctx, "agent-upsert", v1, "application/json")
	time.Sleep(time.Millisecond) // ensure FetchedAt advances
	_ = p.StoreCard(ctx, "agent-upsert", v2, "application/json")
	card, _ := p.GetCard(ctx, "agent-upsert")
	if string(card.Data) != string(v2) {
		t.Errorf("expected updated data v2, got %s", card.Data)
	}
}

func TestCardPlugin_Name(t *testing.T) {
	p := newTestPlugin(t)
	if p.Name() != "card-store" {
		t.Errorf("Name = %q, want card-store", p.Name())
	}
}

func TestCardPlugin_Type(t *testing.T) {
	p := newTestPlugin(t)
	if p.Type() != "cardstore" {
		t.Errorf("Type = %q, want cardstore", p.Type())
	}
}
```

- [ ] **Step 3.2: Run tests to verify they fail**

```bash
rtk go test ./plugins/cardstore/... -v
```

Expected: compile error (`cardstore` package does not exist yet).

- [ ] **Step 3.3: Create `plugins/cardstore/plugin.go`**

```go
// Package cardstore provides the CardStorePlugin for persisting raw agent card bytes.
package cardstore

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

const maxCardBytes = 256 * 1024 // 256 KiB hard cap

// rawCardRow is the GORM model for the raw_cards table.
// It is internal to this plugin — callers use model.RawCard.
type rawCardRow struct {
	AgentTypeID string    `gorm:"primaryKey;type:text"`
	Data        []byte    `gorm:"not null;type:blob"`
	ContentType string    `gorm:"not null;type:text;default:'application/json'"`
	FetchedAt   time.Time `gorm:"not null"`
	Truncated   bool      `gorm:"not null;default:false"`
}

func (rawCardRow) TableName() string { return "raw_cards" }

// Plugin implements kernel.CardStorePlugin.
type Plugin struct {
	database *db.DB
}

// New creates a new Plugin. Call MigrateSchema before first use.
func New(database *db.DB) *Plugin {
	return &Plugin{database: database}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "card-store" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeCardStore }

// Init is called by the PluginManager. It retrieves the DB from the kernel
// and runs AutoMigrate as an idempotent safety net (migration 006 creates the table).
func (p *Plugin) Init(k kernel.Kernel) error {
	// The database was injected via New() at composition root.
	// AutoMigrate is ONLY a safety net — migration 006 creates the table first.
	return p.MigrateSchema(context.Background())
}

// Start is a no-op — card store is synchronous.
func (p *Plugin) Start(_ context.Context) error { return nil }

// Stop is a no-op.
func (p *Plugin) Stop(_ context.Context) error { return nil }

// MigrateSchema ensures the raw_cards table exists. Idempotent.
func (p *Plugin) MigrateSchema(_ context.Context) error {
	return p.database.AutoMigrate(&rawCardRow{})
}

// StoreCard persists raw card bytes for an AgentType (upsert by agentTypeID).
// Payloads exceeding maxCardBytes are truncated and flagged.
func (p *Plugin) StoreCard(ctx context.Context, agentTypeID string, data []byte, contentType string) error {
	if agentTypeID == "" {
		return fmt.Errorf("agentTypeID must not be empty")
	}
	truncated := false
	if len(data) > maxCardBytes {
		data = data[:maxCardBytes]
		truncated = true
	}
	row := rawCardRow{
		AgentTypeID: agentTypeID,
		Data:        data,
		ContentType: contentType,
		FetchedAt:   time.Now().UTC(),
		Truncated:   truncated,
	}
	result := p.database.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "agent_type_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"data", "content_type", "fetched_at", "truncated"}),
		}).
		Create(&row)
	if result.Error != nil {
		return fmt.Errorf("storing raw card for %s: %w", agentTypeID, result.Error)
	}
	return nil
}

// GetCard retrieves the raw card for the given AgentTypeID.
// Returns an error (wrapping gorm.ErrRecordNotFound) if no card is stored.
func (p *Plugin) GetCard(ctx context.Context, agentTypeID string) (*model.RawCard, error) {
	var row rawCardRow
	result := p.database.WithContext(ctx).
		Where("agent_type_id = ?", agentTypeID).
		First(&row)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no raw card stored for agent_type_id %s: %w", agentTypeID, result.Error)
		}
		return nil, fmt.Errorf("getting raw card for %s: %w", agentTypeID, result.Error)
	}
	return &model.RawCard{
		AgentTypeID: row.AgentTypeID,
		Data:        row.Data,
		ContentType: row.ContentType,
		FetchedAt:   row.FetchedAt,
		Truncated:   row.Truncated,
	}, nil
}
```

- [ ] **Step 3.4: Wire CardStorePlugin into PluginManager**

Open `internal/kernel/plugin_manager.go`. In `InitAll()`, add detection for `CardStorePlugin` after the parser detection block:

```go
if csp, ok := p.(CardStorePlugin); ok {
    c.core.RegisterCardStore(csp)
}
```

- [ ] **Step 3.5: Run card store tests**

```bash
rtk go test ./plugins/cardstore/... -v
```

Expected: all 6 tests pass.

- [ ] **Step 3.6: Run full test suite**

```bash
rtk go test ./...
```

Expected: 0 failures.

- [ ] **Step 3.7: Commit**

```bash
rtk git add plugins/cardstore/ internal/kernel/plugin_manager.go
rtk git commit -m "feat(cardstore): CardStorePlugin implementation with 256 KiB cap"
```

---

## Task 4 — Migration 006: raw_cards table

**Files:** Modify `internal/db/migrations.go`

### Why this order
Migration must exist before the column removal from `agent_types` (Task 5). The migration copies existing `RawDefinition` data to `raw_cards`, then drops the column.

- [ ] **Step 4.1: Add `migration006RawCards()` to `AllMigrations()` and implement it**

In `internal/db/migrations.go`, add to `AllMigrations()`:
```go
migration006RawCards(),
```

Add the function body at the end of the file:

```go
func migration006RawCards() Migration {
	return Migration{
		Version:     6,
		Description: "create raw_cards table, copy raw_definition data, drop raw_definition column",
		Up: func(tx *gorm.DB) error {
			// Step 1: Create raw_cards table if not already present.
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS raw_cards (
				agent_type_id TEXT PRIMARY KEY,
				data BLOB NOT NULL,
				content_type TEXT NOT NULL DEFAULT 'application/json',
				fetched_at DATETIME NOT NULL,
				truncated BOOLEAN NOT NULL DEFAULT FALSE
			)`).Error; err != nil {
				return fmt.Errorf("creating raw_cards table: %w", err)
			}

			// Step 2: Copy existing raw_definition bytes into raw_cards (INSERT OR IGNORE).
			// Only copy rows that have non-empty raw_definition.
			if err := tx.Exec(`
				INSERT OR IGNORE INTO raw_cards (agent_type_id, data, content_type, fetched_at, truncated)
				SELECT id, raw_definition, 'application/json', CURRENT_TIMESTAMP, FALSE
				FROM agent_types
				WHERE raw_definition IS NOT NULL AND length(raw_definition) > 0
			`).Error; err != nil {
				return fmt.Errorf("copying raw_definition to raw_cards: %w", err)
			}

			// Step 3: Drop raw_definition column from agent_types.
			// SQLite 3.35+ supports DROP COLUMN directly.
			// For older SQLite versions (< 3.35), we skip and leave the column.
			// In production Postgres, ALTER TABLE DROP COLUMN works unconditionally.
			if err := tx.Exec(`ALTER TABLE agent_types DROP COLUMN IF EXISTS raw_definition`).Error; err != nil {
				// SQLite older than 3.35 does not support DROP COLUMN.
				// The column becomes a harmless dead column.
				_ = err
			}

			return nil
		},
	}
}
```

- [ ] **Step 4.2: Write a migration test**

In `internal/db/migrate_test.go` (or a new `migrations_test.go` in `internal/db/`), add:

```go
func TestMigration006_RawCardsCreated(t *testing.T) {
	database, err := OpenMemory(DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		sqlDB, _ := database.DB.DB()
		_ = sqlDB.Close()
	}()
	migrator := NewMigrator(database, AllMigrations())
	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Verify raw_cards table exists.
	if !database.Migrator().HasTable("raw_cards") {
		t.Error("raw_cards table should exist after migration 006")
	}
	// Verify agent_types table still exists.
	if !database.Migrator().HasTable("agent_types") {
		t.Error("agent_types table should still exist")
	}
}
```

- [ ] **Step 4.3: Run migration tests**

```bash
rtk go test ./internal/db/... -v -run TestMigration
```

Expected: all migration tests pass including the new one.

- [ ] **Step 4.4: Commit**

```bash
rtk git add internal/db/migrations.go internal/db/
rtk git commit -m "feat(db): migration 006 — raw_cards table, copy raw_definition, drop column"
```

---

## Task 5 — Remove RawDefinition from AgentType

**Files:** Modify `internal/model/agent_type.go`, `internal/model/agent.go`, `internal/model/archetype_test.go`

### Why this order
The migration has already run (Task 4), so the DB no longer has the column. Now remove the Go struct field.

- [ ] **Step 5.1: Run archetype tests first to confirm current state**

```bash
rtk go test ./internal/model/... -run TestArchetype -v
```

Note which tests pass/fail before the change.

- [ ] **Step 5.2: Update `internal/model/archetype_test.go`**

In `TestArchetype_AgentTypeHasRequiredFields`, remove `"RawDefinition"` from the `required` slice:

Old:
```go
required := []string{"ID", "AgentKey", "Protocol", "Endpoint", "RawDefinition", "CreatedOn"}
```
New:
```go
required := []string{"ID", "AgentKey", "Protocol", "Endpoint", "CreatedOn"}
```

In `TestArchetype_CatalogEntryHasNoProductFields`, remove `"RawDefinition"` from `productOnlyFields`:

Old:
```go
productOnlyFields := []string{
    "Protocol", "Endpoint", "RawDefinition",
    "Capabilities", "Skills", "TypedMeta", "SpecVersion",
}
```
New:
```go
productOnlyFields := []string{
    "Protocol", "Endpoint",
    "Capabilities", "Skills", "TypedMeta", "SpecVersion",
}
```

- [ ] **Step 5.3: Remove `RawDefinition`, `RawDefJSON`, and `SyncRawDefForJSON()` from `internal/model/agent_type.go`**

Remove the `RawDefinition` field line:
```go
RawDefinition []byte    `json:"-"              gorm:"not null;type:blob"`
```

Remove the `RawDefJSON` field line:
```go
RawDefJSON json.RawMessage `json:"raw_definition,omitempty" gorm:"-"`
```

Remove the entire `SyncRawDefForJSON()` method:
```go
// SyncRawDefForJSON populates RawDefJSON from RawDefinition for JSON serialization.
func (a *AgentType) SyncRawDefForJSON() {
	if len(a.RawDefinition) > 0 && json.Valid(a.RawDefinition) {
		a.RawDefJSON = json.RawMessage(a.RawDefinition)
	}
}
```

Remove the `"encoding/json"` import from `agent_type.go` if it becomes unused.

- [ ] **Step 5.4: Remove `SyncRawDefForJSON()` call in `internal/model/agent.go`**

In `SyncFromDB()`, remove:
```go
if e.AgentType != nil {
    e.AgentType.SyncRawDefForJSON()
}
```

- [ ] **Step 5.5: Compile to find all broken references**

```bash
rtk go build ./...
```

Fix any remaining references to `RawDefinition` or `SyncRawDefForJSON` found by the compiler.

- [ ] **Step 5.6: Run archetype tests**

```bash
rtk go test ./internal/model/... -run TestArchetype -v
```

Expected: all archetype tests pass.

- [ ] **Step 5.7: Run full test suite**

```bash
rtk go test ./...
```

Expected: 0 failures.

- [ ] **Step 5.8: Commit**

```bash
rtk git add internal/model/
rtk git commit -m "feat(model): remove AgentType.RawDefinition — superseded by CardStorePlugin"
```

---

## Task 6 — Update parsers: stop setting RawDefinition

**Files:** Modify `plugins/parsers/a2a/a2a.go`, `plugins/parsers/mcp/mcp.go`

### Why this order
After Task 5 the field is gone; the compiler already caught these. Confirm structural fix here.

- [ ] **Step 6.1: Remove `RawDefinition: raw` from `plugins/parsers/a2a/a2a.go`**

In the `Parse()` return statement, remove the line:
```go
RawDefinition: raw,
```

- [ ] **Step 6.2: Remove `RawDefinition: raw` from `plugins/parsers/mcp/mcp.go`**

In the `Parse()` return statement, remove the line:
```go
RawDefinition: raw,
```

- [ ] **Step 6.3: Run parser tests**

```bash
rtk go test ./plugins/parsers/... -v
```

Expected: all tests pass.

- [ ] **Step 6.4: Commit**

```bash
rtk git add plugins/parsers/
rtk git commit -m "feat(parsers): stop setting RawDefinition — callers store card via CardStorePlugin"
```

---

## Task 7 — Update CreateEntry handler (stub card)

**Files:** Modify `internal/api/handlers.go`

### Why this order
`CreateEntry` manually constructs `AgentType` with `RawDefinition: []byte("{}")`. After Task 5 that field is gone — fix the compile error now.

- [ ] **Step 7.1: Remove `RawDefinition: []byte("{}")` from `CreateEntry` in `internal/api/handlers.go`**

In the `agentType` literal inside `CreateEntry`:

Old:
```go
agentType := &model.AgentType{
    ID:            uuid.NewString(),
    Protocol:      model.Protocol(req.Protocol),
    Endpoint:      req.Endpoint,
    Version:       req.Version,
    RawDefinition: []byte("{}"),
    CreatedOn:     now,
}
```
New:
```go
agentType := &model.AgentType{
    ID:        uuid.NewString(),
    Protocol:  model.Protocol(req.Protocol),
    Endpoint:  req.Endpoint,
    Version:   req.Version,
    CreatedOn: now,
}
```

- [ ] **Step 7.2: Compile and test**

```bash
rtk go build ./... && rtk go test ./...
```

Expected: 0 failures.

- [ ] **Step 7.3: Commit**

```bash
rtk git add internal/api/handlers.go
rtk git commit -m "fix(api): remove RawDefinition stub from CreateEntry"
```

---

## Task 8 — Wire card storage into every ingestion path

**Files:** Modify `internal/api/catalog_helpers.go`, `internal/api/import_handler.go`, `internal/api/register_handler.go`, `internal/discovery/manager.go`

### Why this order
All callers that previously set `RawDefinition` on the `AgentType` must now call `CardStore().StoreCard()` after creating the catalog entry. The card store may be nil (plugin disabled) — guard with a nil check.

- [ ] **Step 8.1: Store card in `internal/api/catalog_helpers.go` `registerAgentType()`**

After `h.store.Create(ctx, entry)` succeeds, add:

```go
// Store the raw card via the card store plugin (best-effort — not fatal if plugin absent).
if cs := h.parsers.CardStore(); cs != nil && len(rawCard) > 0 {
    if err := cs.StoreCard(ctx, agentType.ID, rawCard, "application/json"); err != nil {
        slog.WarnContext(ctx, "failed to store raw card", "agent_type_id", agentType.ID, "err", err)
    }
}
```

- [ ] **Step 8.2: Store card in `internal/discovery/manager.go` `upsert()` — new entries**

The `Manager` struct does not hold a kernel reference. Add a `cardStore` field of type `kernel.CardStorePlugin`. Update `NewManager`:

```go
// Manager orchestrates periodic discovery from all sources.
type Manager struct {
    sources      []Source
    store        store.Store
    cardStore    kernel.CardStorePlugin // may be nil
    pollInterval time.Duration
    log          *slog.Logger
}

// NewManager creates a new Manager.
func NewManager(sources []Source, s store.Store, pollInterval time.Duration) *Manager {
    return &Manager{
        sources:      sources,
        store:        s,
        pollInterval: pollInterval,
        log:          slog.With("component", "discovery-manager"),
    }
}

// SetCardStore injects the card store plugin (called after plugin init).
func (m *Manager) SetCardStore(cs kernel.CardStorePlugin) {
    m.cardStore = cs
}
```

In `upsert()`, after the new entry `m.store.Create(ctx, entry)` succeeds, add:

```go
if m.cardStore != nil && at.rawBytes != nil {
    if err := m.cardStore.StoreCard(ctx, at.ID, at.rawBytes, "application/json"); err != nil {
        m.log.Warn("failed to store raw card", "endpoint", at.Endpoint, "err", err)
    }
}
```

**Note:** The discovery sources return `*model.AgentType` without the raw bytes attached (they set `RawDefinition` before Task 5). We need to thread the raw bytes through. The cleanest approach without changing the `Source.Discover()` signature is to extend the `AgentType` with a transient field:

In `internal/model/agent_type.go`, add a transient field after `Capabilities`:

```go
// RawBytes holds the original card bytes during discovery. Not persisted.
// Populated by crawlers/sources; consumed by ingestion path to call StoreCard.
RawBytes []byte `json:"-" gorm:"-"`
```

Update `plugins/parsers/a2a/a2a.go` and `plugins/parsers/mcp/mcp.go` to set `RawBytes: raw` (not `RawDefinition`).

Update `upsert()` in `manager.go` to use `at.RawBytes` (not `at.rawBytes`).

Also store card for updated entries:

```go
// Update existing entry
if existing.AgentType != nil {
    existing.AgentType.Protocol = at.Protocol
    existing.AgentType.Version = at.Version
    existing.AgentType.SpecVersion = at.SpecVersion
    existing.AgentType.Provider = at.Provider
    existing.AgentType.Capabilities = at.Capabilities
}
existing.Validity.LastSeen = now
existing.UpdatedAt = now
if err := m.store.Update(ctx, existing); err != nil {
    m.log.Warn("failed to update entry", "id", existing.ID, "err", err)
}
// Store updated raw card.
if m.cardStore != nil && len(at.RawBytes) > 0 && existing.AgentType != nil {
    if err := m.cardStore.StoreCard(ctx, existing.AgentType.ID, at.RawBytes, "application/json"); err != nil {
        m.log.Warn("failed to store raw card on update", "id", existing.AgentType.ID, "err", err)
    }
}
```

- [ ] **Step 8.3: Wire `SetCardStore` in `cmd/agentlens/main.go`**

After plugin init in `main.go`, after the `PluginManager.InitAll()` call, add:

```go
if core.CardStore() != nil {
    mgr.SetCardStore(core.CardStore())
}
```

Note: `mgr` must be moved before this call (currently created after). Move the `discovery.NewManager(...)` call to before `pm.InitAll()` is used, or add a deferred `SetCardStore` after init. The simplest approach: move the discovery manager creation earlier in `main.go` and call `mgr.SetCardStore(core.CardStore())` after `pm.InitAll()`.

- [ ] **Step 8.4: Compile and test**

```bash
rtk go build ./... && rtk go test ./...
```

Expected: 0 failures.

- [ ] **Step 8.5: Commit**

```bash
rtk git add internal/api/ internal/discovery/ internal/model/agent_type.go cmd/agentlens/
rtk git commit -m "feat(ingestion): store raw card via CardStorePlugin in all ingestion paths"
```

---

## Task 9 — Update GET /catalog/{id}/card to read from card store

**Files:** Modify `internal/api/handlers.go`

### Why this order
`GetEntryCard` currently reads `entry.AgentType.RawDefinition` which is gone. Replace with card store lookup.

- [ ] **Step 9.1: Write a failing handler test**

In `internal/api/handlers_test.go`, add:

```go
func TestGetEntryCard_FromCardStore(t *testing.T) {
    // This test verifies GetEntryCard returns 200 when card is in card store,
    // and 404 when no card is stored.
    // Set up a handler with a mock/nil card store and verify behavior.
    // (Full test implementation uses testServer helper from test_helpers_test.go)
}
```

(See existing test patterns in `handlers_test.go` for the full setup.)

- [ ] **Step 9.2: Update `GetEntryCard` in `internal/api/handlers.go`**

Replace the entire `GetEntryCard` function:

```go
// GetEntryCard handles GET /api/v1/catalog/{id}/card.
// Returns the raw agent card bytes from the card store plugin.
// Returns 404 if the entry does not exist or no card is stored.
func (h *Handler) GetEntryCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	entry, err := h.store.Get(r.Context(), id)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to get catalog entry")
		return
	}
	if entry == nil {
		ErrorResponse(w, http.StatusNotFound, "catalog entry not found")
		return
	}

	cs := h.parsers.CardStore()
	if cs == nil {
		ErrorResponse(w, http.StatusNotFound, "card store plugin not loaded")
		return
	}

	card, err := cs.GetCard(r.Context(), entry.AgentTypeID)
	if err != nil {
		ErrorResponse(w, http.StatusNotFound, "no raw card stored")
		return
	}

	// Weak ETag from FetchedAt for conditional GET support.
	etag := fmt.Sprintf(`W/"%d"`, card.FetchedAt.UnixMilli())
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Content-Type", card.ContentType)
	w.Header().Set("X-Raw-Card-Fetched-At", card.FetchedAt.UTC().Format(time.RFC3339))
	w.Header().Set("ETag", etag)
	if card.Truncated {
		w.Header().Set("X-Raw-Card-Truncated", "true")
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(card.Data); err != nil {
		slog.Error("failed to write card response", "err", err)
	}
}
```

Add `"fmt"` and `"time"` to imports if not already present.

- [ ] **Step 9.3: Run API tests**

```bash
rtk go test ./internal/api/... -v
```

Expected: all existing tests plus new card store test pass.

- [ ] **Step 9.4: Commit**

```bash
rtk git add internal/api/handlers.go internal/api/handlers_test.go
rtk git commit -m "feat(api): GetEntryCard reads from CardStorePlugin, adds ETag + X-Raw-Card-Fetched-At"
```

---

## Task 10 — Add `sort` param to ListFilter + store query

**Files:** Modify `internal/store/store.go`, `internal/store/sql_store_query.go`, `internal/api/handlers.go`

### Why this order
Sort is a pure backend extension with no model changes. Expand search in same step.

- [ ] **Step 10.1: Add `Sort string` to `ListFilter` in `internal/store/store.go`**

```go
// ListFilter holds filtering parameters for listing catalog entries.
type ListFilter struct {
    Protocol   *model.Protocol
    States     []model.LifecycleState
    Source     *model.SourceType
    Team       string
    Query      string
    Categories []string
    Sort       string // "lastSuccessAt_desc" (default) | "displayName_asc" | "createdAt_desc"
    Limit      int
    Offset     int
}
```

- [ ] **Step 10.2: Write store tests for sort and expanded search**

In `internal/store/sql_store_query_test.go` (create if it does not exist), add:

```go
func TestList_SortLastSuccessAtDesc(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    // Create two entries; give one a recent health success.
    entry1 := buildTestEntry("test-a2a-001", model.ProtocolA2A, "http://agent1.test")
    entry2 := buildTestEntry("test-a2a-002", model.ProtocolA2A, "http://agent2.test")
    _ = s.Create(ctx, entry1)
    _ = s.Create(ctx, entry2)
    now := time.Now().UTC()
    _ = s.UpdateHealth(ctx, entry2.ID, model.Health{
        State:        model.LifecycleActive,
        LastSuccessAt: &now,
    })
    results, err := s.List(ctx, store.ListFilter{Sort: "lastSuccessAt_desc"})
    if err != nil {
        t.Fatal(err)
    }
    if len(results) < 2 {
        t.Fatalf("expected at least 2 results, got %d", len(results))
    }
    // entry2 had a success; should come first.
    if results[0].ID != entry2.ID {
        t.Errorf("expected entry2 first (has lastSuccessAt), got %s", results[0].ID)
    }
}

func TestList_ExpandedSearch_Capabilities(t *testing.T) {
    s := newTestStore(t)
    ctx := context.Background()
    entry := buildTestEntry("test-mcp-001", model.ProtocolMCP, "http://mcp-translate.test")
    entry.AgentType.Capabilities = []model.Capability{
        &model.MCPTool{Name: "translate-text", Description: "Translates between languages"},
    }
    _ = s.Create(ctx, entry)
    results, err := s.List(ctx, store.ListFilter{Query: "translate"})
    if err != nil {
        t.Fatal(err)
    }
    if len(results) == 0 {
        t.Error("expected results for capability name search 'translate'")
    }
}
```

- [ ] **Step 10.3: Run tests to verify they fail**

```bash
rtk go test ./internal/store/... -run TestList_Sort -v
```

Expected: test panics or returns wrong order (sort not yet implemented).

- [ ] **Step 10.4: Implement sort and expanded search in `internal/store/sql_store_query.go`**

Replace the `filter.Query` block and the fixed `Order` clause:

```go
if filter.Query != "" {
    q := "%" + strings.ToLower(filter.Query) + "%"
    query = query.
        Joins("LEFT JOIN capabilities ON capabilities.agent_type_id = agent_types.id").
        Where(
            "LOWER(catalog_entries.display_name) LIKE ? OR "+
                "LOWER(catalog_entries.description) LIKE ? OR "+
                "LOWER(capabilities.name) LIKE ? OR "+
                "LOWER(capabilities.description) LIKE ? OR "+
                "LOWER(catalog_entries.categories) LIKE ?",
            q, q, q, q, q,
        ).
        Distinct("catalog_entries.*").
        Joins("LEFT JOIN providers p2 ON p2.id = agent_types.provider_id").
        Where("LOWER(p2.organization) LIKE ? OR catalog_entries.description LIKE ?", q, q)
}
```

Replace the fixed order:

```go
switch filter.Sort {
case "displayName_asc":
    query = query.Order("catalog_entries.display_name ASC")
case "createdAt_desc":
    query = query.Order("catalog_entries.created_at DESC")
default: // "lastSuccessAt_desc" or empty
    query = query.Order("catalog_entries.health_last_success_at DESC NULLS LAST, catalog_entries.display_name ASC")
}
```

**Note on NULLS LAST:** SQLite supports `NULLS LAST` syntax. PostgreSQL also supports it. This is safe for both dialects.

- [ ] **Step 10.5: Parse `sort` param in `ListCatalog` handler (`internal/api/handlers.go`)**

After the `offset` parsing block, add:

```go
if v := q.Get("sort"); v != "" {
    validSorts := map[string]bool{
        "lastSuccessAt_desc": true,
        "displayName_asc":    true,
        "createdAt_desc":     true,
    }
    if !validSorts[v] {
        ErrorResponse(w, http.StatusBadRequest, "invalid sort value: "+v)
        return
    }
    filter.Sort = v
}
```

- [ ] **Step 10.6: Run store and handler tests**

```bash
rtk go test ./internal/store/... ./internal/api/... -v
```

Expected: all tests pass.

- [ ] **Step 10.7: Commit**

```bash
rtk git add internal/store/ internal/api/handlers.go
rtk git commit -m "feat(store,api): add sort param + expand q search to capabilities/categories/provider"
```

---

## Task 11 — Register CardStorePlugin in main.go

**Files:** Modify `cmd/agentlens/main.go`

### Why this order
All plugin/kernel changes are in place; wire the composition root.

- [ ] **Step 11.1: Add cardstore import and registration to `cmd/agentlens/main.go`**

Add import:
```go
cardstorePlugin "github.com/PawelHaracz/agentlens/plugins/cardstore"
```

Before `pm.Register(a2aplugin.New())`, add:
```go
pm.Register(cardstorePlugin.New(database))
```

After `pm.InitAll()` (and before starting discovery manager), add the `SetCardStore` call:
```go
// Wire card store into discovery manager if plugin loaded.
if core.CardStore() != nil {
    mgr.SetCardStore(core.CardStore())
}
```

- [ ] **Step 11.2: Build the binary**

```bash
rtk make build
```

Expected: binary compiles successfully.

- [ ] **Step 11.3: Commit**

```bash
rtk git add cmd/agentlens/main.go
rtk git commit -m "feat(main): register CardStorePlugin + wire into discovery manager"
```

---

## Task 12 — Frontend: install new dependencies

**Files:** Modify `web/package.json`

- [ ] **Step 12.1: Install dependencies**

```bash
cd web && bun add @tanstack/react-query @radix-ui/react-toggle-group @radix-ui/react-hover-card prismjs && bun add -d @types/prismjs
```

- [ ] **Step 12.2: Verify TypeScript can find types**

```bash
rtk make web-lint
```

Expected: no new type errors.

- [ ] **Step 12.3: Commit**

```bash
rtk git add web/package.json web/bun.lockb
rtk git commit -m "feat(web): add react-query, radix toggle-group, hover-card, prismjs"
```

---

## Task 13 — Frontend: QueryClientProvider + types update

**Files:** Modify `web/src/main.tsx`, `web/src/types.ts`, `web/src/api.ts`

- [ ] **Step 13.1: Add `QueryClientProvider` to `web/src/main.tsx`**

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </React.StrictMode>,
)
```

- [ ] **Step 13.2: Add `sort` to `ListFilter` and `SearchMatch` to `web/src/types.ts`**

In the `ListFilter` interface, add:
```ts
sort?: 'lastSuccessAt_desc' | 'displayName_asc' | 'createdAt_desc'
```

Add new interface after `ListFilter`:
```ts
export interface SearchMatch {
  field: string
  snippet: string
}
```

- [ ] **Step 13.3: Add `sort` param and `getRawCard()` to `web/src/api.ts`**

In `listCatalog()`, add after the `categories` param:
```ts
if (filter.sort) params.set('sort', filter.sort)
```

Add `getRawCard` function after `getEntry`:
```ts
export async function getRawCard(id: string): Promise<{ data: string; contentType: string; fetchedAt: string; truncated: boolean }> {
  const headers: Record<string, string> = {}
  if (authToken) headers['Authorization'] = `Bearer ${authToken}`
  const res = await fetch(`${BASE}/catalog/${id}/card`, { headers })
  if (res.status === 401 && authToken) handleUnauthorized()
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error ?? res.statusText)
  }
  const data = await res.text()
  return {
    data,
    contentType: res.headers.get('Content-Type') ?? 'application/json',
    fetchedAt: res.headers.get('X-Raw-Card-Fetched-At') ?? '',
    truncated: res.headers.get('X-Raw-Card-Truncated') === 'true',
  }
}
```

- [ ] **Step 13.4: Run frontend lint**

```bash
rtk make web-lint
```

Expected: clean.

- [ ] **Step 13.5: Commit**

```bash
rtk git add web/src/main.tsx web/src/types.ts web/src/api.ts
rtk git commit -m "feat(web): QueryClientProvider, sort param, getRawCard API function"
```

---

## Task 14 — Frontend: useCatalogQuery hook

**Files:** Create `web/src/hooks/useCatalogQuery.ts`

- [ ] **Step 14.1: Write the hook test first — create `web/src/hooks/useCatalogQuery.test.ts`**

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import React from 'react'
import { useCatalogQuery } from './useCatalogQuery'
import * as api from '../api'

const wrapper = ({ children }: { children: React.ReactNode }) =>
  React.createElement(
    QueryClientProvider,
    { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
    React.createElement(MemoryRouter, null, children)
  )

describe('useCatalogQuery', () => {
  beforeEach(() => {
    vi.spyOn(api, 'listCatalog').mockResolvedValue([])
  })
  it('calls listCatalog with default filter', async () => {
    renderHook(() => useCatalogQuery(), { wrapper })
    await waitFor(() => {
      expect(api.listCatalog).toHaveBeenCalledWith(expect.objectContaining({}))
    })
  })
})
```

- [ ] **Step 14.2: Run test to confirm it fails**

```bash
rtk make web-test
```

Expected: compile error (`useCatalogQuery` module not found).

- [ ] **Step 14.3: Create `web/src/hooks/useCatalogQuery.ts`**

```ts
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { useCallback } from 'react'
import { listCatalog } from '../api'
import type { ListFilter, Protocol, LifecycleState } from '../types'

export interface CatalogQueryResult {
  entries: ReturnType<typeof useQuery>['data']
  isLoading: boolean
  isError: boolean
  error: Error | null
  filter: ListFilter
  setProtocol: (protocol: Protocol | undefined) => void
  setQuery: (q: string) => void
  setSort: (sort: ListFilter['sort']) => void
  setStates: (states: LifecycleState[]) => void
  clearFilters: () => void
  refetch: () => void
}

export function useCatalogQuery(): CatalogQueryResult {
  const [searchParams, setSearchParams] = useSearchParams()

  const protocol = (searchParams.get('protocol') as Protocol) || undefined
  const q = searchParams.get('q') || undefined
  const sort = (searchParams.get('sort') as ListFilter['sort']) || undefined
  const stateParam = searchParams.get('state')
  const states = stateParam
    ? (stateParam.split(',') as LifecycleState[])
    : undefined

  const filter: ListFilter = {
    protocol,
    q,
    sort,
    state: stateParam || undefined,
  }

  const result = useQuery({
    queryKey: ['catalog', { protocol, q, sort, states }],
    queryFn: () => listCatalog(filter),
  })

  const setProtocol = useCallback(
    (p: Protocol | undefined) => {
      setSearchParams(prev => {
        const next = new URLSearchParams(prev)
        if (p) next.set('protocol', p)
        else next.delete('protocol')
        next.delete('offset')
        return next
      })
    },
    [setSearchParams]
  )

  const setQuery = useCallback(
    (newQ: string) => {
      setSearchParams(prev => {
        const next = new URLSearchParams(prev)
        if (newQ) next.set('q', newQ)
        else next.delete('q')
        next.delete('offset')
        return next
      })
    },
    [setSearchParams]
  )

  const setSort = useCallback(
    (newSort: ListFilter['sort']) => {
      setSearchParams(prev => {
        const next = new URLSearchParams(prev)
        if (newSort) next.set('sort', newSort)
        else next.delete('sort')
        return next
      })
    },
    [setSearchParams]
  )

  const setStates = useCallback(
    (newStates: LifecycleState[]) => {
      setSearchParams(prev => {
        const next = new URLSearchParams(prev)
        if (newStates.length > 0) next.set('state', newStates.join(','))
        else next.delete('state')
        next.delete('offset')
        return next
      })
    },
    [setSearchParams]
  )

  const clearFilters = useCallback(() => {
    setSearchParams({})
  }, [setSearchParams])

  return {
    entries: result.data,
    isLoading: result.isLoading,
    isError: result.isError,
    error: result.error as Error | null,
    filter,
    setProtocol,
    setQuery,
    setSort,
    setStates,
    clearFilters,
    refetch: result.refetch,
  }
}
```

- [ ] **Step 14.4: Run hook test**

```bash
rtk make web-test
```

Expected: hook test passes.

- [ ] **Step 14.5: Commit**

```bash
rtk git add web/src/hooks/
rtk git commit -m "feat(web): useCatalogQuery hook with URL sync via react-query"
```

---

## Task 15 — Frontend: SpecVersionBadge + SearchHighlight components

**Files:** Create `web/src/routes/catalog/components/SpecVersionBadge.tsx`, `web/src/routes/catalog/components/SearchHighlight.tsx`

- [ ] **Step 15.1: Create `web/src/routes/catalog/components/SpecVersionBadge.tsx`**

```tsx
import { Badge } from '../../components/ui/badge'

interface Props {
  version?: string
}

export function SpecVersionBadge({ version }: Props) {
  if (!version) return null
  return (
    <Badge variant="outline" className="font-mono text-xs">
      {version}
    </Badge>
  )
}
```

Note: The `ui/badge` import path is relative to the new location. Adjust if needed:
`import { Badge } from '../../../components/ui/badge'`

- [ ] **Step 15.2: Create `web/src/routes/catalog/components/SearchHighlight.tsx`**

```tsx
interface Props {
  snippet?: string
}

export function SearchHighlight({ snippet }: Props) {
  if (!snippet) return null
  // Snippet may contain HTML <mark> tags from server or be plain text.
  // We render as plain text and bold the match portion.
  return (
    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-1 italic">
      …{snippet}…
    </p>
  )
}
```

- [ ] **Step 15.3: Run lint**

```bash
rtk make web-lint
```

- [ ] **Step 15.4: Commit**

```bash
rtk git add web/src/routes/
rtk git commit -m "feat(web): SpecVersionBadge + SearchHighlight components"
```

---

## Task 16 — Frontend: ProtocolFilter + UnifiedSearchBox

**Files:** Create `web/src/routes/catalog/components/ProtocolFilter.tsx`, `web/src/routes/catalog/components/UnifiedSearchBox.tsx`

**shadcn ToggleGroup:** Install with `bunx shadcn@latest add toggle-group` first.

- [ ] **Step 16.1: Add shadcn ToggleGroup component**

```bash
cd web && bunx shadcn@latest add toggle-group
```

Expected: creates `web/src/components/ui/toggle-group.tsx`.

- [ ] **Step 16.2: Create `web/src/routes/catalog/components/ProtocolFilter.tsx`**

```tsx
import { Boxes, Bot, Plug } from 'lucide-react'
import { ToggleGroup, ToggleGroupItem } from '../../../components/ui/toggle-group'
import type { Protocol } from '../../../types'

interface Props {
  value: Protocol | undefined
  onChange: (protocol: Protocol | undefined) => void
}

const OPTIONS = [
  { value: undefined, label: 'All', icon: Boxes },
  { value: 'a2a' as Protocol, label: 'A2A', icon: Bot },
  { value: 'mcp' as Protocol, label: 'MCP', icon: Plug },
]

export function ProtocolFilter({ value, onChange }: Props) {
  return (
    <ToggleGroup
      type="single"
      value={value ?? 'all'}
      onValueChange={v => onChange(v === 'all' ? undefined : (v as Protocol))}
      aria-label="Filter by protocol"
    >
      {OPTIONS.map(opt => (
        <ToggleGroupItem
          key={opt.value ?? 'all'}
          value={opt.value ?? 'all'}
          aria-label={opt.label}
          className="flex items-center gap-1.5"
        >
          <opt.icon className="h-3.5 w-3.5" />
          {opt.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
```

- [ ] **Step 16.3: Create `web/src/routes/catalog/components/UnifiedSearchBox.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react'
import { Search, X } from 'lucide-react'
import { Input } from '../../../components/ui/input'

interface Props {
  value: string | undefined
  onChange: (q: string) => void
}

export function UnifiedSearchBox({ value, onChange }: Props) {
  const [draft, setDraft] = useState(value ?? '')
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Sync external value changes (e.g., clear filters).
  useEffect(() => {
    setDraft(value ?? '')
  }, [value])

  // Focus on '/' keydown anywhere on the page.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === '/' && document.activeElement !== inputRef.current) {
        e.preventDefault()
        inputRef.current?.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const v = e.target.value
    setDraft(v)
    if (timerRef.current) clearTimeout(timerRef.current)
    timerRef.current = setTimeout(() => onChange(v), 250)
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      if (timerRef.current) clearTimeout(timerRef.current)
      onChange(draft)
    }
    if (e.key === 'Escape') {
      inputRef.current?.blur()
    }
  }

  function handleClear() {
    setDraft('')
    onChange('')
    inputRef.current?.focus()
  }

  return (
    <div className="relative flex-1">
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
      <Input
        ref={inputRef}
        value={draft}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        placeholder="Search across A2A and MCP — name, description, skills, tags, provider…"
        className="pl-9 pr-8"
        aria-label="Search catalog"
      />
      {draft && (
        <button
          onClick={handleClear}
          className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          aria-label="Clear search"
        >
          <X className="h-4 w-4" />
        </button>
      )}
    </div>
  )
}
```

- [ ] **Step 16.4: Run lint**

```bash
rtk make web-lint
```

- [ ] **Step 16.5: Commit**

```bash
rtk git add web/src/routes/ web/src/components/ui/toggle-group.tsx
rtk git commit -m "feat(web): ProtocolFilter + UnifiedSearchBox components"
```

---

## Task 17 — Frontend: CatalogRow component

**Files:** Create `web/src/routes/catalog/components/CatalogRow.tsx`

Depends on: `StatusBadge`, `ProtocolBadge` (existing), `SpecVersionBadge`, `SearchHighlight` (Task 15).

- [ ] **Step 17.1: Create `web/src/routes/catalog/components/CatalogRow.tsx`**

```tsx
import { useNavigate } from 'react-router-dom'
import { ExternalLink } from 'lucide-react'
import { TableRow, TableCell } from '../../../components/ui/table'
import StatusBadge from '../../../components/StatusBadge'
import ProtocolBadge from '../../../components/ProtocolBadge'
import { SpecVersionBadge } from './SpecVersionBadge'
import { SearchHighlight } from './SearchHighlight'
import type { CatalogEntry } from '../../../types'

interface Props {
  entry: CatalogEntry
  searchSnippet?: string
}

function relativeTime(iso: string | null): string {
  if (!iso) return '—'
  const ms = Date.now() - new Date(iso).getTime()
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

export function CatalogRow({ entry, searchSnippet }: Props) {
  const navigate = useNavigate()
  const skillCount = entry.capabilities?.length ?? 0
  const provider = entry.provider?.organization

  return (
    <TableRow
      className="cursor-pointer hover:bg-muted/50 transition-colors"
      onClick={() => navigate(`/catalog/${entry.id}`)}
      role="link"
      aria-label={`View ${entry.display_name}`}
    >
      <TableCell>
        <ProtocolBadge protocol={entry.protocol} />
      </TableCell>
      <TableCell>
        <div>
          <p className="font-medium truncate max-w-[260px]">{entry.display_name}</p>
          {entry.description && (
            <p className="text-xs text-muted-foreground truncate max-w-[260px]">
              {entry.description.slice(0, 80)}
            </p>
          )}
          <SearchHighlight snippet={searchSnippet} />
        </div>
      </TableCell>
      <TableCell>
        {provider ? (
          <span className="flex items-center gap-1 text-sm">
            {provider}
            {entry.provider?.url && (
              <a
                href={entry.provider.url}
                target="_blank"
                rel="noopener noreferrer"
                onClick={e => e.stopPropagation()}
                className="text-muted-foreground hover:text-foreground"
                aria-label="Provider website"
              >
                <ExternalLink className="h-3 w-3" />
              </a>
            )}
          </span>
        ) : (
          <span className="text-muted-foreground">—</span>
        )}
      </TableCell>
      <TableCell>
        <span className="text-sm tabular-nums">
          {skillCount > 0 ? `${skillCount} skill${skillCount === 1 ? '' : 's'}` : '—'}
        </span>
      </TableCell>
      <TableCell>
        <StatusBadge status={entry.status} health={entry.health} />
      </TableCell>
      <TableCell>
        <SpecVersionBadge version={entry.spec_version} />
      </TableCell>
      <TableCell>
        <span
          className="text-sm text-muted-foreground"
          title={entry.health?.lastSuccessAt ?? undefined}
        >
          {relativeTime(entry.health?.lastSuccessAt ?? null)}
        </span>
      </TableCell>
    </TableRow>
  )
}
```

- [ ] **Step 17.2: Run lint**

```bash
rtk make web-lint
```

- [ ] **Step 17.3: Commit**

```bash
rtk git add web/src/routes/catalog/components/CatalogRow.tsx
rtk git commit -m "feat(web): CatalogRow table row component"
```

---

## Task 18 — Frontend: CatalogListPage (clean rewrite)

**Files:** Create `web/src/routes/catalog/CatalogListPage.tsx`

- [ ] **Step 18.1: Create `web/src/routes/catalog/CatalogListPage.tsx`**

```tsx
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Table, TableBody, TableHead, TableHeader, TableRow } from '../../components/ui/table'
import { Skeleton } from '../../components/ui/skeleton'
import { Alert, AlertDescription } from '../../components/ui/alert'
import { Button } from '../../components/ui/button'
import StatsBar from '../../components/StatsBar'
import RegisterAgentDialog from '../../components/RegisterAgentDialog'
import { ProtocolFilter } from './components/ProtocolFilter'
import { UnifiedSearchBox } from './components/UnifiedSearchBox'
import { CatalogRow } from './components/CatalogRow'
import { useCatalogQuery } from '../../hooks/useCatalogQuery'
import type { CatalogEntry } from '../../types'

export default function CatalogListPage() {
  const { entries, isLoading, isError, error, filter, setProtocol, setQuery, clearFilters, refetch } =
    useCatalogQuery()
  const navigate = useNavigate()
  const [selectedIdx, setSelectedIdx] = useState(-1)
  const rowRefs = useRef<(HTMLTableRowElement | null)[]>([])
  const entryList = entries ?? []
  const hasFilters = !!(filter.protocol || filter.q)

  // Keyboard navigation: Up/Down/Enter.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (document.activeElement?.tagName === 'INPUT') return
      if (e.key === 'ArrowDown') {
        setSelectedIdx(i => Math.min(i + 1, entryList.length - 1))
        e.preventDefault()
      } else if (e.key === 'ArrowUp') {
        setSelectedIdx(i => Math.max(i - 1, 0))
        e.preventDefault()
      } else if (e.key === 'Enter' && selectedIdx >= 0) {
        const entry = entryList[selectedIdx]
        if (entry) navigate(`/catalog/${entry.id}`)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [entryList, selectedIdx, navigate])

  return (
    <div className="p-4 space-y-4">
      <StatsBar onRegisterSuccess={refetch} />

      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-3">
        <ProtocolFilter value={filter.protocol} onChange={setProtocol} />
        <UnifiedSearchBox value={filter.q} onChange={setQuery} />
        <RegisterAgentDialog onSuccess={refetch} />
      </div>

      {/* Error state */}
      {isError && (
        <Alert variant="destructive">
          <AlertDescription>
            {error?.message ?? 'Failed to load catalog.'}{' '}
            <Button variant="link" size="sm" onClick={refetch}>
              Retry
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {/* Skeleton loading */}
      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full rounded-md" />
          ))}
        </div>
      )}

      {/* Empty — no data at all */}
      {!isLoading && !isError && entryList.length === 0 && !hasFilters && (
        <div className="rounded-xl border bg-card p-8 text-center space-y-3">
          <p className="text-muted-foreground">No catalog entries yet.</p>
          <p className="text-xs text-muted-foreground font-mono bg-muted rounded px-3 py-2 inline-block">
            curl -X POST http://localhost:8080/api/v1/catalog/import \<br />
            {'  '}-d '{'"'}url{'"'}:{'"'}https://your-agent.example.com/.well-known/agent-card.json{'"'}{'}'}'
          </p>
        </div>
      )}

      {/* Empty — filters exclude all results */}
      {!isLoading && !isError && entryList.length === 0 && hasFilters && (
        <Alert>
          <AlertDescription className="flex items-center justify-between">
            No entries match your filters.
            <Button variant="outline" size="sm" onClick={clearFilters}>
              Clear filters
            </Button>
          </AlertDescription>
        </Alert>
      )}

      {/* Catalog table */}
      {!isLoading && entryList.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-20">Protocol</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Provider</TableHead>
              <TableHead className="w-24">Skills</TableHead>
              <TableHead className="w-28">Status</TableHead>
              <TableHead className="w-28">Spec</TableHead>
              <TableHead className="w-28">Last seen</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(entryList as CatalogEntry[]).map((entry, idx) => (
              <CatalogRow
                key={entry.id}
                entry={entry}
              />
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
```

- [ ] **Step 18.2: Run lint**

```bash
rtk make web-lint
```

- [ ] **Step 18.3: Commit**

```bash
rtk git add web/src/routes/catalog/CatalogListPage.tsx
rtk git commit -m "feat(web): CatalogListPage — unified table with protocol filter + search"
```

---

## Task 19 — Frontend: RawCardTab component

**Files:** Create `web/src/routes/catalog/components/RawCardTab.tsx`

- [ ] **Step 19.1: Create `web/src/routes/catalog/components/RawCardTab.tsx`**

```tsx
import { useEffect, useState } from 'react'
import Prism from 'prismjs'
import 'prismjs/components/prism-json'
import { Copy, Download, AlertTriangle } from 'lucide-react'
import { Alert, AlertDescription } from '../../../components/ui/alert'
import { Button } from '../../../components/ui/button'
import { Skeleton } from '../../../components/ui/skeleton'
import { getRawCard } from '../../../api'

interface Props {
  entryId: string
  displayName: string
}

export function RawCardTab({ entryId, displayName }: Props) {
  const [state, setState] = useState<{
    status: 'idle' | 'loading' | 'success' | 'error'
    data?: string
    fetchedAt?: string
    truncated?: boolean
    error?: string
  }>({ status: 'idle' })
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    setState({ status: 'loading' })
    getRawCard(entryId)
      .then(result => {
        setState({
          status: 'success',
          data: result.data,
          fetchedAt: result.fetchedAt,
          truncated: result.truncated,
        })
      })
      .catch(err => {
        setState({ status: 'error', error: (err as Error).message })
      })
  }, [entryId])

  useEffect(() => {
    if (state.status === 'success') {
      Prism.highlightAll()
    }
  }, [state])

  function handleCopy() {
    if (state.data) {
      navigator.clipboard.writeText(state.data).then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 1500)
      })
    }
  }

  function handleDownload() {
    if (!state.data) return
    const blob = new Blob([state.data], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${displayName.replace(/\s+/g, '-').toLowerCase()}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  if (state.status === 'loading' || state.status === 'idle') {
    return <Skeleton className="h-64 w-full rounded-md" />
  }

  if (state.status === 'error') {
    return (
      <Alert variant="destructive">
        <AlertDescription>{state.error ?? 'Failed to load raw card.'}</AlertDescription>
      </Alert>
    )
  }

  let pretty = state.data ?? ''
  try {
    pretty = JSON.stringify(JSON.parse(pretty), null, 2)
  } catch {
    // Not valid JSON — render as-is
  }

  return (
    <div className="space-y-2">
      {state.truncated && (
        <Alert>
          <AlertTriangle className="h-4 w-4" />
          <AlertDescription>
            Card was truncated at 256 KiB. Download may be incomplete.
          </AlertDescription>
        </Alert>
      )}
      <div className="flex items-center justify-between gap-2">
        {state.fetchedAt && (
          <p className="text-xs text-muted-foreground">
            Fetched at: {new Date(state.fetchedAt).toLocaleString()}
          </p>
        )}
        <div className="flex gap-2 ml-auto">
          <Button variant="outline" size="sm" onClick={handleCopy}>
            <Copy className="h-3.5 w-3.5 mr-1" />
            {copied ? 'Copied!' : 'Copy'}
          </Button>
          <Button variant="outline" size="sm" onClick={handleDownload}>
            <Download className="h-3.5 w-3.5 mr-1" />
            Download
          </Button>
        </div>
      </div>
      <pre className="rounded-md bg-muted p-4 overflow-auto text-sm max-h-[600px]">
        <code className="language-json">{pretty}</code>
      </pre>
    </div>
  )
}
```

- [ ] **Step 19.2: Run lint**

```bash
rtk make web-lint
```

- [ ] **Step 19.3: Commit**

```bash
rtk git add web/src/routes/catalog/components/RawCardTab.tsx
rtk git commit -m "feat(web): RawCardTab with prismjs JSON highlighting, copy, download"
```

---

## Task 20 — Frontend: CatalogDetailPage (tabbed rewrite)

**Files:** Create `web/src/routes/catalog/CatalogDetailPage.tsx`

- [ ] **Step 20.1: Create `web/src/routes/catalog/CatalogDetailPage.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft } from 'lucide-react'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../../components/ui/tabs'
import { Button } from '../../components/ui/button'
import { Skeleton } from '../../components/ui/skeleton'
import { Alert, AlertDescription } from '../../components/ui/alert'
import StatusBadge from '../../components/StatusBadge'
import ProtocolBadge from '../../components/ProtocolBadge'
import { SpecVersionBadge } from './components/SpecVersionBadge'
import { RawCardTab } from './components/RawCardTab'
import { getEntry, patchLifecycle, postProbe } from '../../api'
import type { CatalogEntry } from '../../types'

export default function CatalogDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [entry, setEntry] = useState<CatalogEntry | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    getEntry(id)
      .then(setEntry)
      .catch(err => setError((err as Error).message))
      .finally(() => setLoading(false))
  }, [id])

  async function handleProbeNow() {
    if (!id) return
    try {
      await postProbe(id)
      const updated = await getEntry(id)
      setEntry(updated)
    } catch {
      // Non-fatal — badge will update on next auto-probe
    }
  }

  async function handleDeprecate() {
    if (!id) return
    try {
      await patchLifecycle(id, 'deprecated')
      const updated = await getEntry(id)
      setEntry(updated)
    } catch {
      // Non-fatal
    }
  }

  async function handleUndeprecate() {
    if (!id) return
    try {
      await patchLifecycle(id, 'registered')
      const updated = await getEntry(id)
      setEntry(updated)
    } catch {
      // Non-fatal
    }
  }

  if (loading) return <div className="p-4 space-y-4"><Skeleton className="h-40 w-full" /></div>
  if (error) return (
    <div className="p-4">
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    </div>
  )
  if (!entry) return null

  const isDeprecated = entry.status === 'deprecated'

  return (
    <div className="p-4 space-y-4">
      {/* Back + header */}
      <div className="flex items-start gap-3">
        <Button variant="ghost" size="icon" onClick={() => navigate(-1)} aria-label="Back">
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-bold">{entry.display_name}</h1>
            <ProtocolBadge protocol={entry.protocol} />
            <StatusBadge status={entry.status} health={entry.health} />
            <SpecVersionBadge version={entry.spec_version} />
          </div>
          {entry.description && (
            <p className="text-muted-foreground">{entry.description}</p>
          )}
        </div>
        <div className="flex gap-2">
          {!isDeprecated && (
            <Button variant="outline" size="sm" onClick={handleProbeNow}>
              Probe Now
            </Button>
          )}
          {!isDeprecated ? (
            <Button variant="outline" size="sm" onClick={handleDeprecate}>
              Deprecate
            </Button>
          ) : (
            <Button variant="outline" size="sm" onClick={handleUndeprecate}>
              Undeprecate
            </Button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="raw-card">Raw Card</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="space-y-4 pt-4">
          {/* Fields grid */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
            <div><span className="text-muted-foreground">Endpoint</span><br /><code className="text-xs">{entry.endpoint}</code></div>
            <div><span className="text-muted-foreground">Source</span><br />{entry.source}</div>
            <div><span className="text-muted-foreground">Version</span><br />{entry.version || '—'}</div>
            {entry.provider && (
              <div>
                <span className="text-muted-foreground">Provider</span><br />
                {entry.provider.organization}
                {entry.provider.team && ` / ${entry.provider.team}`}
              </div>
            )}
          </div>

          {/* Categories */}
          {entry.categories && entry.categories.length > 0 && (
            <div>
              <p className="text-sm font-medium mb-1">Categories</p>
              <div className="flex flex-wrap gap-1">
                {entry.categories.map(c => (
                  <span key={c} className="text-xs rounded-full bg-muted px-2 py-0.5">{c}</span>
                ))}
              </div>
            </div>
          )}

          {/* Capabilities */}
          {entry.capabilities && entry.capabilities.length > 0 && (
            <div>
              <p className="text-sm font-medium mb-2">Capabilities ({entry.capabilities.length})</p>
              <div className="space-y-1">
                {entry.capabilities.map((cap, i) => (
                  <div key={i} className="rounded-md border px-3 py-2 text-sm">
                    <span className="font-mono font-medium">{cap.name}</span>
                    {cap.description && (
                      <p className="text-xs text-muted-foreground mt-0.5">{cap.description}</p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Health info */}
          {entry.health && (
            <div>
              <p className="text-sm font-medium mb-1">Health</p>
              <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
                <div>Latency: {entry.health.latencyMs}ms</div>
                <div>Failures: {entry.health.consecutiveFailures}</div>
                {entry.health.lastError && (
                  <div className="col-span-2 text-destructive">Last error: {entry.health.lastError}</div>
                )}
              </div>
            </div>
          )}
        </TabsContent>

        <TabsContent value="raw-card" className="pt-4">
          {id && <RawCardTab entryId={id} displayName={entry.display_name} />}
        </TabsContent>
      </Tabs>
    </div>
  )
}
```

- [ ] **Step 20.2: Run lint**

```bash
rtk make web-lint
```

- [ ] **Step 20.3: Commit**

```bash
rtk git add web/src/routes/catalog/CatalogDetailPage.tsx
rtk git commit -m "feat(web): CatalogDetailPage — tabbed layout with Overview + Raw Card"
```

---

## Task 21 — Wire new routes in App.tsx, remove old components

**Files:** Modify `web/src/App.tsx`; delete `web/src/components/CatalogList.tsx`, `web/src/components/EntryDetail.tsx` (and their test files)

- [ ] **Step 21.1: Update `web/src/App.tsx`**

Replace the existing `CatalogList`/`EntryDetail` imports and routes:

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from './contexts/AuthContext'
import { ThemeProvider } from './contexts/ThemeContext'
import Layout from './components/Layout'
import ProtectedRoute from './components/ProtectedRoute'
import CatalogListPage from './routes/catalog/CatalogListPage'
import CatalogDetailPage from './routes/catalog/CatalogDetailPage'
import LoginPage from './pages/LoginPage'
import SettingsPage from './pages/SettingsPage'

export default function App() {
  return (
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/" element={<ProtectedRoute><Layout><CatalogListPage /></Layout></ProtectedRoute>} />
            <Route path="/catalog/:id" element={<ProtectedRoute><Layout><CatalogDetailPage /></Layout></ProtectedRoute>} />
            <Route path="/settings" element={<ProtectedRoute><Layout><SettingsPage /></Layout></ProtectedRoute>} />
          </Routes>
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  )
}
```

- [ ] **Step 21.2: Delete old component files**

```bash
rm web/src/components/CatalogList.tsx
rm web/src/components/EntryDetail.tsx
rm web/src/components/CatalogList.test.tsx
rm web/src/components/EntryDetail.test.tsx
```

- [ ] **Step 21.3: Run lint and build**

```bash
rtk make web-lint && rtk make web-build
```

Expected: clean build with no references to deleted files.

- [ ] **Step 21.4: Commit**

```bash
rtk git add -A
rtk git commit -m "feat(web): rewire routes to CatalogListPage + CatalogDetailPage, remove old components"
```

---

## Task 22 — Frontend unit tests

**Files:** Create test files for new components + hook

### Why this order
Tests written after implementation for the new route components (TDD was used for the hook in Task 14).

- [ ] **Step 22.1: Create `web/src/routes/catalog/components/SpecVersionBadge.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { SpecVersionBadge } from './SpecVersionBadge'

describe('SpecVersionBadge', () => {
  it('renders spec version text', () => {
    render(<SpecVersionBadge version="0.7" />)
    expect(screen.getByText('0.7')).toBeInTheDocument()
  })
  it('renders nothing for empty version', () => {
    const { container } = render(<SpecVersionBadge />)
    expect(container.firstChild).toBeNull()
  })
  it('renders nothing for undefined version', () => {
    const { container } = render(<SpecVersionBadge version={undefined} />)
    expect(container.firstChild).toBeNull()
  })
})
```

- [ ] **Step 22.2: Create `web/src/routes/catalog/components/SearchHighlight.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { SearchHighlight } from './SearchHighlight'

describe('SearchHighlight', () => {
  it('renders snippet', () => {
    render(<SearchHighlight snippet="translate between en and de" />)
    expect(screen.getByText(/translate between en and de/)).toBeInTheDocument()
  })
  it('renders nothing when no snippet', () => {
    const { container } = render(<SearchHighlight />)
    expect(container.firstChild).toBeNull()
  })
})
```

- [ ] **Step 22.3: Create `web/src/routes/catalog/CatalogListPage.test.tsx`**

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import React from 'react'
import CatalogListPage from './CatalogListPage'
import * as api from '../../api'

const wrapper = ({ children }: { children: React.ReactNode }) =>
  React.createElement(
    QueryClientProvider,
    { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
    React.createElement(MemoryRouter, null, children)
  )

describe('CatalogListPage', () => {
  beforeEach(() => {
    vi.spyOn(api, 'listCatalog').mockResolvedValue([])
    vi.spyOn(api, 'getStats').mockResolvedValue({ total: 0, by_status: {}, by_source: {} })
  })

  it('shows skeleton during loading', () => {
    render(<CatalogListPage />, { wrapper })
    expect(document.querySelector('.animate-pulse, [data-skeleton]')).toBeTruthy()
  })

  it('shows empty state when no entries', async () => {
    render(<CatalogListPage />, { wrapper })
    // Wait for query to resolve.
    await screen.findByText(/No catalog entries yet/i)
  })

  it('renders table when entries present', async () => {
    vi.spyOn(api, 'listCatalog').mockResolvedValue([
      {
        id: 'e1', display_name: 'My Agent', description: '', protocol: 'a2a',
        endpoint: 'http://agent.test', version: '1', status: 'active', source: 'push',
        agent_type_id: 'at1', validity: { last_seen: new Date().toISOString() },
        health: { state: 'active', lastProbedAt: null, lastSuccessAt: null, latencyMs: 0, consecutiveFailures: 0, lastError: '' },
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    ])
    render(<CatalogListPage />, { wrapper })
    await screen.findByText('My Agent')
  })
})
```

- [ ] **Step 22.4: Run all frontend tests and check coverage**

```bash
rtk make web-test-coverage
```

Expected: all tests pass, coverage ≥ 80%.

- [ ] **Step 22.5: Commit**

```bash
rtk git add web/src/routes/
rtk git commit -m "test(web): unit tests for new catalog components + CatalogListPage"
```

---

## Task 23 — E2E Playwright tests

**Files:** Modify `e2e/tests/catalog.spec.ts`

- [ ] **Step 23.1: Add unified-view E2E scenarios to `e2e/tests/catalog.spec.ts`**

Append the following test blocks:

```ts
import { test, expect } from '@playwright/test'
import { loginViaAPI, authHeader } from './helpers'

test.describe('Unified Catalog View', () => {
  test.beforeEach(async ({ page }) => {
    await loginViaAPI(page)
  })

  test('shows all entries by default (All protocol filter)', async ({ page }) => {
    await page.goto('/')
    // Protocol filter should show "All" selected by default.
    await expect(page.getByRole('group', { name: 'Filter by protocol' })).toBeVisible()
    // Table should be visible or empty state shown.
    const table = page.locator('table')
    const emptyState = page.getByText(/No catalog entries yet/i)
    await expect(table.or(emptyState)).toBeVisible({ timeout: 10_000 })
  })

  test('search input focuses on "/" keypress', async ({ page }) => {
    await page.goto('/')
    await page.keyboard.press('/')
    const input = page.getByRole('textbox', { name: 'Search catalog' })
    await expect(input).toBeFocused()
  })

  test('protocol filter updates URL param', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('radio', { name: 'A2A' }).or(page.getByText('A2A')).first().click()
    await expect(page).toHaveURL(/protocol=a2a/)
  })

  test('URL filter persists on reload', async ({ page }) => {
    await page.goto('/?protocol=mcp')
    await page.reload()
    await expect(page).toHaveURL(/protocol=mcp/)
  })

  test('Raw Card tab visible on detail page', async ({ page, request }) => {
    // Seed an entry via API.
    const headers = await authHeader(page)
    const res = await request.post('/api/v1/catalog', {
      data: { display_name: 'E2E Test Agent', protocol: 'a2a', endpoint: 'http://e2e-test.local' },
      headers,
    })
    expect(res.ok()).toBeTruthy()
    const entry = await res.json()

    await page.goto(`/catalog/${entry.id}`)
    await expect(page.getByRole('tab', { name: 'Raw Card' })).toBeVisible()
    await page.getByRole('tab', { name: 'Raw Card' }).click()
    // Card store may return 404 for manually created entries without a card — that's acceptable.
    await expect(
      page.getByText('no raw card stored').or(page.locator('pre code.language-json'))
    ).toBeVisible({ timeout: 5_000 })
  })
})
```

- [ ] **Step 23.2: Run E2E tests (requires built binary + frontend)**

```bash
rtk make e2e-test
```

Expected: all E2E tests pass including new unified-view suite.

- [ ] **Step 23.3: Commit**

```bash
rtk git add e2e/tests/catalog.spec.ts
rtk git commit -m "test(e2e): unified catalog view E2E — protocol filter, URL persistence, raw card tab"
```

---

## Task 24 — Documentation updates

**Files:** Modify `docs/api.md`, `docs/architecture.md`

- [ ] **Step 24.1: Update `docs/api.md` — new sort param + card endpoint changes**

Add to the `GET /api/v1/catalog` endpoint section:

```markdown
| `sort` | string | `lastSuccessAt_desc` | Sort order. Values: `lastSuccessAt_desc`, `displayName_asc`, `createdAt_desc`. Unknown values return `400`. |
```

Update the search (`q` param) description:
```markdown
| `q` | string | — | Full-text search across `display_name`, `description`, `capabilities.name`, `capabilities.description`, `categories`, `provider.organization`. |
```

Update `GET /api/v1/catalog/{id}/card` documentation:
```markdown
- Response headers: `Content-Type` (from card store), `X-Raw-Card-Fetched-At` (ISO 8601), `ETag` (weak, for conditional GET)
- Request header: `If-None-Match` — if ETag matches, returns `304 Not Modified`  
- Returns `404` with `{"error": "no raw card stored"}` if entry exists but no card is in the card store (e.g., manually created entries)
```

- [ ] **Step 24.2: Update `docs/architecture.md` — CardStorePlugin + Frontend layers**

Add to the Plugin Types section:
```markdown
| `cardstore` | `CardStorePlugin` | Persists verbatim raw card bytes keyed by `agent_type_id`. Owns the `raw_cards` table. Enforces 256 KiB cap. |
```

Add note to Frontend section:
```markdown
Data fetching migrated from plain `fetch`+`useState` to `@tanstack/react-query`. Catalog list page uses `useCatalogQuery` hook for URL-synced state. Detail page uses tabbed layout (Overview + Raw Card).
```

- [ ] **Step 24.3: Commit**

```bash
rtk git add docs/api.md docs/architecture.md
rtk git commit -m "docs: update API docs for sort param + card endpoint; architecture for CardStorePlugin"
```

---

## Task 25 — Final validation

- [ ] **Step 25.1: Run full Go test suite**

```bash
rtk make test
```

Expected: all tests pass.

- [ ] **Step 25.2: Run architecture validation**

```bash
rtk make arch-test
```

Expected: 100% compliance. If `plugins/cardstore` violates layer rules, update `arch-go.yml` to allow `plugins/cardstore` to import `internal/kernel` and `internal/db` (consistent with other plugins).

- [ ] **Step 25.3: Run frontend tests with coverage**

```bash
rtk make web-test-coverage
```

Expected: all tests pass, functions coverage ≥ 80%.

- [ ] **Step 25.4: Build frontend and Go binary**

```bash
rtk make web-build && rtk make build
```

Expected: both succeed.

- [ ] **Step 25.5: Run E2E (optional but recommended)**

```bash
rtk make e2e-test
```

- [ ] **Step 25.6: Final commit if any cleanup needed**

```bash
rtk git add -A
rtk git commit -m "chore: final cleanup for feature/2.2-unified-catalog-view"
```

---

## Known Traps & Gotchas

1. **SQLite DROP COLUMN requires 3.35+.** The migration uses `IF EXISTS` and silently ignores errors — the column becomes a dead column in old SQLite. This is safe (Go model ignores it).
2. **`DISTINCT` + search JOINs.** The expanded search joins `capabilities` which can produce duplicates. `Distinct("catalog_entries.*")` prevents that.
3. **`NULLS LAST` syntax.** Both SQLite 3.30+ and Postgres support this. Older SQLite needs a workaround (`IS NULL` first). If CI runs old SQLite, replace with `CASE WHEN health_last_success_at IS NULL THEN 1 ELSE 0 END ASC, health_last_success_at DESC`.
4. **Prism.js in jsdom.** `Prism.highlightAll()` is a no-op in jsdom (no DOM rendering). Do not try to assert on highlighted output in Vitest tests.
5. **`getRawCard` returns raw bytes, not JSON.** The function reads `res.text()` not `res.json()`. Do not change this — the card bytes may not be valid JSON in all future protocols.
6. **Card store nil guard.** Every caller of `h.parsers.CardStore()` must nil-check. The card store plugin may not be registered in test environments.
7. **`RawBytes` field on `AgentType` is `gorm:"-"`.** It is never persisted. Sources must set it; ingestion path reads and clears it.
8. **Old `CatalogList.test.tsx` and `EntryDetail.test.tsx` must be deleted.** They import the deleted components and will cause `make web-test` to fail.
9. **arch-go `plugins/cardstore` rule.** After creating the plugin, run `make arch-test` and add a rule if needed: `plugins/cardstore` may depend on `internal/kernel`, `internal/db`, `internal/model`.

---

## Execution Choice

The plan is written and ready. Choose how to proceed:

**Option A — Subagent-driven (recommended)**
Run each task as a subagent with the task description + file contents, letting the agent implement one task at a time and commit. Slower but each commit is verified.

**Option B — Inline execution**
Ask GitHub Copilot to implement this plan task-by-task in the current session, following each `- [ ]` step in order.

**Option C — Manual**
Follow the plan yourself, checking off each step. All code is complete above — paste and adjust as needed.

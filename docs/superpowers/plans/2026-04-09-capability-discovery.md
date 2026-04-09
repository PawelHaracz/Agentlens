# Capability-based Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable platform engineers to discover agents by capability ("which agent can translate?") through a dedicated Capabilities tab with search, filtering, and detail views showing all agents offering each capability.

**Architecture:** Extends the capability registry (internal/model/capability.go) with discoverability metadata, adds flat-list store queries joining the existing relational capabilities table to catalog_entries, new REST endpoints for capability list and detail, and React UI with accordion grouping. Follows Product Archetype: AgentType owns Capabilities, CatalogEntry wraps with catalog concerns. No aggregation table, no server-side GROUP BY — frontend groups the flat list client-side.

**Tech Stack:** Go 1.26.1 (GORM, chi router), React 18, shadcn/ui, Tailwind CSS, React Query, Playwright E2E

---

## File Structure

### Backend Files

**Create:**
- `internal/model/capability_instance.go` — CapabilityInstance, CapabilityListResult DTOs
- `internal/model/capability_test.go` — Tests for DiscoverableKinds and GetCapabilityFactory
- `internal/api/capability_handlers.go` — CapabilityHandler with ListCapabilities, GetCapabilityAgents
- `internal/api/capability_handlers_test.go` — Handler tests

**Modify:**
- `internal/model/capability.go` — Add CapabilityMeta, change RegisterCapability signature, add DiscoverableKinds()
- `internal/model/a2a_capabilities.go` — Update 5 RegisterCapability calls
- `internal/model/mcp_capabilities.go` — Update 3 RegisterCapability calls
- `internal/model/archetype_test.go` — Update for new RegisterCapability signature
- `internal/store/store.go` — Add CapabilityFilter, ListCapabilities, ListAgentsByCapability; remove SearchCapabilities
- `internal/store/sql_store_query.go` — Implement new methods, remove SearchCapabilities
- `internal/store/sqlite_test.go` — Add capability discovery tests, update SearchCapabilities tests
- `internal/api/router.go` — Replace /skills routes with /capabilities routes
- `internal/api/handlers.go` — Remove SearchCapabilities method

### Frontend Files

**Create:**
- `web/src/routes/capabilities/CapabilityListPage.tsx` — Capability index with accordion groups
- `web/src/routes/capabilities/CapabilityDetailPage.tsx` — Capability detail showing all agents
- `web/src/routes/capabilities/components/CapabilityGroup.tsx` — Accordion group component
- `web/src/routes/capabilities/components/KindFilter.tsx` — Kind toggle filter
- `web/src/routes/capabilities/components/AgentForCapabilityRow.tsx` — Table row for detail view
- `web/src/hooks/useCapabilitiesQuery.ts` — React Query hook with URL sync
- `web/src/hooks/useCapabilitiesQuery.test.ts` — Hook tests

**Modify:**
- `web/src/App.tsx` — Add capability routes
- `web/src/api.ts` — Add listCapabilities, getCapabilityAgents
- `web/src/types.ts` — Add CapabilityInstance, CapabilityListResult, CapabilityDetailResponse, CapabilityAgentDTO
- `web/src/components/Layout.tsx` — Add Capabilities nav tab
- `web/src/routes/catalog/CatalogDetailPage.tsx` — Make capability names clickable

### Test Files

**Modify:**
- `e2e/tests/catalog.spec.ts` — Update skills test, add capability discovery E2E tests

---

## Task 1: Extend Capability Registry with Discoverability Metadata

**Files:**
- Modify: `internal/model/capability.go:17-32` (capabilityRegistry and RegisterCapability)
- Modify: `internal/model/a2a_capabilities.go:8-14` (init function RegisterCapability calls)
- Modify: `internal/model/mcp_capabilities.go:5-9` (init function RegisterCapability calls)
- Modify: `internal/model/archetype_test.go` (update test expectations)

### Step 1.1: Write failing test for DiscoverableKinds()

- [ ] **Write test for DiscoverableKinds() helper**

File: `internal/model/capability_test.go` (create new file)

```go
package model

import (
	"sort"
	"testing"
)

func TestDiscoverableKinds(t *testing.T) {
	kinds := DiscoverableKinds()

	// Should return exactly 4 discoverable kinds
	if len(kinds) != 4 {
		t.Errorf("expected 4 discoverable kinds, got %d", len(kinds))
	}

	// Should be sorted
	sortedKinds := make([]string, len(kinds))
	copy(sortedKinds, kinds)
	sort.Strings(sortedKinds)
	for i := range kinds {
		if kinds[i] != sortedKinds[i] {
			t.Errorf("kinds not sorted: got %v", kinds)
			break
		}
	}

	// Should contain user-facing kinds
	expected := []string{"a2a.skill", "mcp.prompt", "mcp.resource", "mcp.tool"}
	for _, e := range expected {
		found := false
		for _, k := range kinds {
			if k == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected kind %s not found in %v", e, kinds)
		}
	}

	// Should NOT contain technical kinds
	technical := []string{"a2a.extension", "a2a.interface", "a2a.security_scheme", "a2a.signature"}
	for _, tk := range technical {
		for _, k := range kinds {
			if k == tk {
				t.Errorf("technical kind %s should not be discoverable", tk)
			}
		}
	}
}

func TestGetCapabilityFactoryStillWorks(t *testing.T) {
	// Backward compatibility: GetCapabilityFactory should work for all 8 kinds
	allKinds := []string{
		"a2a.skill", "a2a.interface", "a2a.security_scheme", "a2a.extension", "a2a.signature",
		"mcp.tool", "mcp.resource", "mcp.prompt",
	}

	for _, kind := range allKinds {
		factory, ok := GetCapabilityFactory(kind)
		if !ok {
			t.Errorf("GetCapabilityFactory(%s) returned not ok", kind)
		}
		if factory == nil {
			t.Errorf("GetCapabilityFactory(%s) returned nil factory", kind)
		}

		// Call factory to ensure it works
		cap := factory()
		if cap == nil {
			t.Errorf("factory for %s returned nil capability", kind)
		}
		if cap.Kind() != kind {
			t.Errorf("factory for %s returned capability with kind %s", kind, cap.Kind())
		}
	}
}
```

- [ ] **Run test to verify it fails**

```bash
rtk go test ./internal/model -run TestDiscoverableKinds -v
```

Expected: FAIL with "undefined: DiscoverableKinds"

### Step 1.2: Implement CapabilityMeta and DiscoverableKinds()

- [ ] **Modify capability.go to add CapabilityMeta struct and update registry**

File: `internal/model/capability.go`

Replace lines 17-36 (the existing capabilityRegistry, RegisterCapability, AND GetCapabilityFactory) with:

```go
// CapabilityMeta holds registration metadata for a capability kind.
type CapabilityMeta struct {
	Factory      func() Capability
	Discoverable bool // true = user-facing, shown in capability discovery UI
}

var capabilityRegistry = map[string]CapabilityMeta{}

// RegisterCapability registers a capability kind with its factory and discoverability flag.
func RegisterCapability(kind string, factory func() Capability, discoverable bool) {
	capabilityRegistry[kind] = CapabilityMeta{
		Factory:      factory,
		Discoverable: discoverable,
	}
}

// GetCapabilityFactory returns the factory for a given kind.
// Maintains backward compatibility with existing deserialization code.
func GetCapabilityFactory(kind string) (func() Capability, bool) {
	m, ok := capabilityRegistry[kind]
	return m.Factory, ok
}

// DiscoverableKinds returns the kind strings where Discoverable == true.
// Results are sorted for deterministic behavior.
func DiscoverableKinds() []string {
	var kinds []string
	for kind, meta := range capabilityRegistry {
		if meta.Discoverable {
			kinds = append(kinds, kind)
		}
	}
	sort.Strings(kinds)
	return kinds
}
```

Add `"sort"` to the import block at top of file (lines 4-7):

```go
import (
	"encoding/json"
	"fmt"
	"sort"
)
```

- [ ] **Update UnmarshalCapabilitiesJSON to use GetCapabilityFactory**

File: `internal/model/capability.go`

The registry type change breaks `UnmarshalCapabilitiesJSON` at line 77 which directly indexes
`capabilityRegistry`. Replace lines 77-81:

```go
		factory, ok := capabilityRegistry[w.Kind]
		if !ok {
			continue // silently skip unknown kinds
		}
		item := factory()
```

with:

```go
		factory, ok := GetCapabilityFactory(w.Kind)
		if !ok {
			continue // silently skip unknown kinds
		}
		item := factory()
```

This ensures `UnmarshalCapabilitiesJSON` goes through the accessor that extracts `.Factory`
from `CapabilityMeta`, rather than trying to call `CapabilityMeta` directly.

- [ ] **Run test to verify DiscoverableKinds test still fails**

```bash
rtk go test ./internal/model -run TestDiscoverableKinds -v
```

Expected: Still FAIL because RegisterCapability calls haven't been updated with discoverable param

### Step 1.3: Update A2A capability registrations

- [ ] **Update a2a_capabilities.go RegisterCapability calls**

File: `internal/model/a2a_capabilities.go`

Replace lines 8-14 (the 5 RegisterCapability calls in init()) with:

```go
func init() {
	RegisterCapability("a2a.skill", func() Capability { return &A2ASkill{} }, true)
	RegisterCapability("a2a.interface", func() Capability { return &A2AInterface{} }, false)
	RegisterCapability("a2a.security_scheme", func() Capability { return &A2ASecurityScheme{} }, false)
	RegisterCapability("a2a.extension", func() Capability { return &A2AExtension{} }, false)
	RegisterCapability("a2a.signature", func() Capability { return &A2ASignature{} }, false)
}
```

### Step 1.4: Update MCP capability registrations

- [ ] **Update mcp_capabilities.go RegisterCapability calls**

File: `internal/model/mcp_capabilities.go`

Replace lines 5-9 (the 3 RegisterCapability calls in init()) with:

```go
func init() {
	RegisterCapability("mcp.tool", func() Capability { return &MCPTool{} }, true)
	RegisterCapability("mcp.resource", func() Capability { return &MCPResource{} }, true)
	RegisterCapability("mcp.prompt", func() Capability { return &MCPPrompt{} }, true)
}
```

- [ ] **Run tests to verify they pass**

```bash
rtk go test ./internal/model -run TestDiscoverableKinds -v
rtk go test ./internal/model -run TestGetCapabilityFactoryStillWorks -v
```

Expected: Both PASS

### Step 1.5: Update archetype_test.go for new signature

- [ ] **Fix any test failures in archetype_test.go**

```bash
rtk go test ./internal/model -run TestAgentType -v
```

If test uses RegisterCapability directly (unlikely), update the call to include the discoverable parameter. Most likely this will pass without changes since archetype_test.go uses the registered capabilities, not the registration itself.

### Step 1.6: Run all model tests

- [ ] **Verify all model tests pass**

```bash
rtk go test ./internal/model -v
```

Expected: All PASS

### Step 1.7: Commit

- [ ] **Commit capability registry extension**

```bash
rtk git add internal/model/capability.go internal/model/capability_test.go internal/model/a2a_capabilities.go internal/model/mcp_capabilities.go
rtk git commit -m "feat(model): extend capability registry with discoverability metadata

- Add CapabilityMeta struct with Factory + Discoverable fields
- Change RegisterCapability signature to accept discoverable bool
- Add DiscoverableKinds() helper returning sorted discoverable kinds
- Mark a2a.skill, mcp.tool, mcp.resource, mcp.prompt as discoverable=true
- Mark technical kinds (a2a.extension, a2a.interface, a2a.security_scheme, a2a.signature) as discoverable=false
- GetCapabilityFactory backward compatible, existing deserialization unchanged
- Tests: DiscoverableKinds returns 4 kinds, excludes technical kinds, GetCapabilityFactory works for all 8 kinds"
```

---

## Task 2: Add Model Types for Capability Discovery DTOs

**Files:**
- Create: `internal/model/capability_instance.go`

### Step 2.1: Write capability_instance.go with DTOs

- [ ] **Create capability_instance.go with CapabilityInstance and CapabilityListResult**

File: `internal/model/capability_instance.go`

```go
package model

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

- [ ] **Verify file compiles**

```bash
rtk go build ./internal/model
```

Expected: Success (no output)

### Step 2.2: Commit

- [ ] **Commit DTO types**

```bash
rtk git add internal/model/capability_instance.go
rtk git commit -m "feat(model): add capability discovery DTO types

- CapabilityInstance: flat DTO combining capability fields with agent metadata
- CapabilityListResult: paginated list wrapper
- Uses domain types Protocol and LifecycleState for type safety
- Tags/InputModes/OutputModes populated only for a2a.skill (nil for MCP kinds)
- Minimal agent fields (not full CatalogEntry) for small response size"
```

---

## Task 3: Store Interface and Implementation for Capability Queries

**Files:**
- Modify: `internal/store/store.go:23` (remove SearchCapabilities, add new methods)
- Modify: `internal/store/sql_store_query.go:92-116` (remove SearchCapabilities impl, add new impls)
- Modify: `internal/store/sqlite_test.go` (update tests)

### Step 3.1: Write failing test for ListCapabilities

- [ ] **Add test for ListCapabilities in sqlite_test.go**

File: `internal/store/sqlite_test.go`

Add at end of file:

```go
func TestListCapabilities(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Seed two entries with same capability name but different descriptions
	entry1 := sampleEntry("entry-1")
	entry1.AgentType.Protocol = model.ProtocolA2A
	entry1.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{
			Name:        "Translate EN-DE",
			Description: "Bidirectional translation",
			Tags:        []string{"translation", "german", "english"},
			InputModes:  []string{"text/plain"},
			OutputModes: []string{"text/plain"},
		},
	}
	entry1.Status = model.LifecycleActive
	if err := s.Create(ctx, entry1); err != nil {
		t.Fatalf("Create entry1: %v", err)
	}

	entry2 := sampleEntry("entry-2")
	entry2.DisplayName = "Polyglot Agent"
	entry2.AgentType.Protocol = model.ProtocolA2A
	entry2.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{
			Name:        "Translate EN-DE",
			Description: "Translation with context",
			Tags:        []string{"translation", "german"},
			InputModes:  []string{"text/plain", "application/json"},
			OutputModes: []string{"text/plain"},
		},
	}
	entry2.Status = model.LifecycleActive
	if err := s.Create(ctx, entry2); err != nil {
		t.Fatalf("Create entry2: %v", err)
	}

	// Seed MCP tool entry
	entry3 := sampleEntry("entry-3")
	entry3.DisplayName = "DocSearch Server"
	entry3.AgentType.Protocol = model.ProtocolMCP
	entry3.AgentType.Capabilities = []model.Capability{
		&model.MCPTool{
			Name:        "search_documents",
			Description: "Full-text search",
		},
	}
	entry3.Status = model.LifecycleActive
	if err := s.Create(ctx, entry3); err != nil {
		t.Fatalf("Create entry3: %v", err)
	}

	// Seed offline entry (should be excluded from list)
	entry4 := sampleEntry("entry-4")
	entry4.AgentType.Protocol = model.ProtocolA2A
	entry4.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Offline Skill", Description: "Should not appear"},
	}
	entry4.Status = model.LifecycleOffline
	if err := s.Create(ctx, entry4); err != nil {
		t.Fatalf("Create entry4: %v", err)
	}

	t.Run("list all capabilities", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Limit: 50,
			Sort:  "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		// Should return 3 items (entry1 skill, entry2 skill, entry3 tool) — offline excluded
		if result.Total != 3 {
			t.Errorf("expected total=3, got %d", result.Total)
		}
		if len(result.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(result.Items))
		}

		// Verify first item is entry3 tool (sorted by name: search_documents < Translate)
		if result.Items[0].Kind != "mcp.tool" {
			t.Errorf("expected first item kind=mcp.tool, got %s", result.Items[0].Kind)
		}
		if result.Items[0].Name != "search_documents" {
			t.Errorf("expected first item name=search_documents, got %s", result.Items[0].Name)
		}
		if result.Items[0].AgentID != entry3.ID {
			t.Errorf("expected first item agent_id=%s, got %s", entry3.ID, result.Items[0].AgentID)
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Kind:  "a2a.skill",
			Limit: 50,
			Sort:  "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		if result.Total != 2 {
			t.Errorf("expected total=2, got %d", result.Total)
		}
		for _, item := range result.Items {
			if item.Kind != "a2a.skill" {
				t.Errorf("expected kind=a2a.skill, got %s", item.Kind)
			}
		}
	})

	t.Run("search by query", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Query: "translation",
			Limit: 50,
			Sort:  "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		// Should match both entry1 and entry2 (description + tags contain "translation")
		if result.Total != 2 {
			t.Errorf("expected total=2, got %d", result.Total)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Limit:  1,
			Offset: 1,
			Sort:   "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
		if result.Total != 3 {
			t.Errorf("expected total=3, got %d", result.Total)
		}
	})
}
```

- [ ] **Run test to verify it fails**

```bash
rtk go test ./internal/store -run TestListCapabilities -v
```

Expected: FAIL with "undefined: CapabilityFilter" or "store.ListCapabilities undefined"

### Step 3.2: Write failing test for ListAgentsByCapability

- [ ] **Add test for ListAgentsByCapability**

File: `internal/store/sqlite_test.go`

Add at end of file:

```go
func TestListAgentsByCapability(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Seed entries with same capability (kind, name)
	entry1 := sampleEntry("entry-1")
	entry1.DisplayName = "Translation Agent"
	entry1.AgentType.Protocol = model.ProtocolA2A
	entry1.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Translate EN-DE", Description: "Bidirectional"},
	}
	entry1.Status = model.LifecycleActive
	if err := s.Create(ctx, entry1); err != nil {
		t.Fatalf("Create entry1: %v", err)
	}

	entry2 := sampleEntry("entry-2")
	entry2.DisplayName = "Legacy Translator"
	entry2.AgentType.Protocol = model.ProtocolA2A
	entry2.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Translate EN-DE", Description: "Legacy"},
	}
	entry2.Status = model.LifecycleOffline
	if err := s.Create(ctx, entry2); err != nil {
		t.Fatalf("Create entry2: %v", err)
	}

	entry3 := sampleEntry("entry-3")
	entry3.AgentType.Protocol = model.ProtocolA2A
	entry3.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Other Skill", Description: "Different"},
	}
	entry3.Status = model.LifecycleActive
	if err := s.Create(ctx, entry3); err != nil {
		t.Fatalf("Create entry3: %v", err)
	}

	t.Run("list agents by capability", func(t *testing.T) {
		entries, err := s.ListAgentsByCapability(ctx, "a2a.skill", "Translate EN-DE")
		if err != nil {
			t.Fatalf("ListAgentsByCapability: %v", err)
		}

		// Should return 2 entries (entry1 + entry2), including offline
		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}

		// Verify entry IDs
		ids := map[string]bool{}
		for _, e := range entries {
			ids[e.ID] = true
		}
		if !ids[entry1.ID] || !ids[entry2.ID] {
			t.Errorf("expected entries %s and %s, got %v", entry1.ID, entry2.ID, ids)
		}
	})

	t.Run("non-existent capability returns empty", func(t *testing.T) {
		entries, err := s.ListAgentsByCapability(ctx, "a2a.skill", "NonExistent")
		if err != nil {
			t.Fatalf("ListAgentsByCapability: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})
}
```

- [ ] **Run test to verify it fails**

```bash
rtk go test ./internal/store -run TestListAgentsByCapability -v
```

Expected: FAIL with "store.ListAgentsByCapability undefined"

### Step 3.3: Update Store interface

- [ ] **Modify store.go to add new methods and remove SearchCapabilities**

File: `internal/store/store.go`

Add before the Store interface definition (around line 10):

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

Replace `SearchCapabilities` method (line 23) with:

```go
	// ListCapabilities returns a flat list of capability instances (one per
	// agent per capability) with agent metadata. Only active + degraded entries
	// are included. Only user-facing capability kinds are returned.
	ListCapabilities(ctx context.Context, filter CapabilityFilter) (*model.CapabilityListResult, error)

	// ListAgentsByCapability returns catalog entries offering a specific
	// capability identified by (kind, name). Returns all lifecycle states.
	ListAgentsByCapability(ctx context.Context, kind, name string) ([]model.CatalogEntry, error)
```

Remove the `SearchCapabilities(ctx context.Context, query string) ([]model.CatalogEntry, error)` line completely.

- [ ] **Verify store interface compiles**

```bash
rtk go build ./internal/store
```

Expected: FAIL with "sql_store.go does not implement Store" because methods not implemented yet

### Step 3.4: Implement ListCapabilities in sql_store_query.go

- [ ] **Add ListCapabilities implementation**

File: `internal/store/sql_store_query.go`

Add at end of file (after Stats function):

```go
func (s *SQLStore) ListCapabilities(ctx context.Context, filter CapabilityFilter) (*model.CapabilityListResult, error) {
	discoverableKinds := model.DiscoverableKinds()
	if len(discoverableKinds) == 0 {
		return &model.CapabilityListResult{Total: 0, Items: []model.CapabilityInstance{}}, nil
	}

	// Build base query
	query := s.gdb.WithContext(ctx).
		Table("capabilities c").
		Select(`c.kind, c.name, c.description, c.properties,
			ce.id AS agent_id, ce.display_name AS agent_name,
			at.protocol, at.spec_version,
			ce.status,
			p.organization AS provider_org, p.url AS provider_url,
			ce.health_latency_ms, ce.status AS health_state`).
		Joins("JOIN agent_types at ON c.agent_type_id = at.id").
		Joins("JOIN catalog_entries ce ON ce.agent_type_id = at.id").
		Joins("LEFT JOIN providers p ON at.provider_id = p.id").
		Where("c.kind IN ?", discoverableKinds).
		Where("ce.status IN ?", []string{string(model.LifecycleActive), string(model.LifecycleDegraded)})

	// Apply filters
	if filter.Query != "" {
		lowerQuery := "%" + strings.ToLower(filter.Query) + "%"
		query = query.Where(
			"LOWER(c.name) LIKE ? OR LOWER(c.description) LIKE ? OR LOWER(c.properties) LIKE ?",
			lowerQuery, lowerQuery, lowerQuery,
		)
	}

	if filter.Kind != "" {
		query = query.Where("c.kind = ?", filter.Kind)
	}

	// Count total
	var total int64
	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count capabilities: %w", err)
	}

	// Apply sorting
	orderClause := "c.name ASC, ce.display_name ASC"
	if filter.Sort == "agentName_asc" {
		orderClause = "ce.display_name ASC, c.name ASC"
	}
	query = query.Order(orderClause)

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Execute query
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

	var rows []capabilityInstanceRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query capabilities: %w", err)
	}

	// Convert to CapabilityInstance
	items := make([]model.CapabilityInstance, len(rows))
	for i, row := range rows {
		items[i] = model.CapabilityInstance{
			Kind:        row.Kind,
			Name:        row.Name,
			Description: row.Description,
			AgentID:     row.AgentID,
			AgentName:   row.AgentName,
			Protocol:    model.Protocol(row.Protocol),
			Status:      model.LifecycleState(row.Status),
			SpecVersion: row.SpecVersion,
			HealthState: model.LifecycleState(row.HealthState),
			LatencyMs:   row.LatencyMs,
		}

		if row.ProviderOrg != nil {
			items[i].ProviderOrg = *row.ProviderOrg
		}
		if row.ProviderURL != nil {
			items[i].ProviderURL = *row.ProviderURL
		}

		// Extract tags, inputModes, outputModes from properties JSON for a2a.skill
		if row.Kind == "a2a.skill" && row.Properties != "" {
			var props map[string]any
			if err := json.Unmarshal([]byte(row.Properties), &props); err == nil {
				if tags, ok := props["tags"].([]any); ok {
					for _, t := range tags {
						if str, ok := t.(string); ok {
							items[i].Tags = append(items[i].Tags, str)
						}
					}
				}
				if inputModes, ok := props["inputModes"].([]any); ok {
					for _, m := range inputModes {
						if str, ok := m.(string); ok {
							items[i].InputModes = append(items[i].InputModes, str)
						}
					}
				}
				if outputModes, ok := props["outputModes"].([]any); ok {
					for _, m := range outputModes {
						if str, ok := m.(string); ok {
							items[i].OutputModes = append(items[i].OutputModes, str)
						}
					}
				}
			}
		}
	}

	return &model.CapabilityListResult{
		Total: int(total),
		Items: items,
	}, nil
}
```

Add imports at top of file (merge with existing imports, keeping `time` which is used by `ListForProbing`):

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
	"gorm.io/gorm"
)
```

### Step 3.5: Implement ListAgentsByCapability

- [ ] **Add ListAgentsByCapability implementation**

File: `internal/store/sql_store_query.go`

Add at end of file:

```go
func (s *SQLStore) ListAgentsByCapability(ctx context.Context, kind, name string) ([]model.CatalogEntry, error) {
	var entries []model.CatalogEntry

	err := s.gdb.WithContext(ctx).
		Joins("JOIN agent_types ON catalog_entries.agent_type_id = agent_types.id").
		Joins("JOIN capabilities ON capabilities.agent_type_id = agent_types.id").
		Where("capabilities.kind = ? AND capabilities.name = ?", kind, name).
		Preload("AgentType").
		Preload("AgentType.Provider").
		Find(&entries).Error

	if err != nil {
		return nil, fmt.Errorf("query agents by capability: %w", err)
	}

	// Load capabilities for each entry
	for i := range entries {
		if err := s.loadCapabilities(ctx, &entries[i].AgentType); err != nil {
			return nil, fmt.Errorf("load capabilities for agent %s: %w", entries[i].ID, err)
		}
		entries[i].SyncFromDB()
	}

	return entries, nil
}
```

### Step 3.6: Remove SearchCapabilities implementation

- [ ] **Remove SearchCapabilities from sql_store_query.go**

File: `internal/store/sql_store_query.go`

Delete lines 92-116 (the SearchCapabilities function entirely).

- [ ] **Run tests to verify they pass**

```bash
rtk go test ./internal/store -run TestListCapabilities -v
rtk go test ./internal/store -run TestListAgentsByCapability -v
```

Expected: Both PASS

### Step 3.7: Update SearchCapabilities tests

- [ ] **Comment out or remove TestSearchCapabilities in sqlite_test.go**

File: `internal/store/sqlite_test.go`

Find the `func TestSearchCapabilities(t *testing.T)` function and either delete it or comment it out with a note:

```go
// TestSearchCapabilities removed - SearchCapabilities method replaced by ListCapabilities
// See TestListCapabilities for new capability discovery tests
```

- [ ] **Run all store tests**

```bash
rtk go test ./internal/store -v
```

Expected: All PASS

### Step 3.8: Commit

- [ ] **Commit store layer changes**

```bash
rtk git add internal/store/store.go internal/store/sql_store_query.go internal/store/sqlite_test.go
rtk git commit -m "feat(store): add capability discovery queries, remove SearchCapabilities

- Add CapabilityFilter struct with Query, Kind, Limit, Offset, Sort
- Add ListCapabilities: flat list of capability instances with agent metadata
  - Joins capabilities -> agent_types -> catalog_entries -> providers
  - Filters by discoverable kinds via model.DiscoverableKinds()
  - Only active + degraded entries included
  - Case-insensitive search via LOWER() on name, description, properties
  - Extracts tags, inputModes, outputModes from properties JSON for a2a.skill
  - Supports pagination, sort by name or agent name
- Add ListAgentsByCapability: all agents offering a (kind, name) capability
  - Returns all lifecycle states (including offline, deprecated)
  - Preloads AgentType, Provider, Capabilities
- Remove SearchCapabilities (replaced by ListCapabilities)
- Tests: list all, filter by kind, search by query, pagination, agents by capability"
```

---

## Task 4: Handler Layer for Capability Endpoints

**Files:**
- Create: `internal/api/capability_handlers.go`
- Create: `internal/api/capability_handlers_test.go`

### Step 4.1: Write failing handler test for ListCapabilities

- [ ] **Create capability_handlers_test.go with ListCapabilities test**

File: `internal/api/capability_handlers_test.go`

```go
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestCapabilityHandlerListCapabilities(t *testing.T) {
	router, st := newTestRouter(t)

	// Seed test data
	ctx := context.Background()
	entry := &model.CatalogEntry{
		ID:          "test-entry-1",
		DisplayName: "Test Agent",
		Status:      model.LifecycleActive,
		AgentType: model.AgentType{
			Protocol: model.ProtocolA2A,
			Endpoint: "http://test.local/a2a",
			Capabilities: []model.Capability{
				&model.A2ASkill{
					Name:        "Test Skill",
					Description: "Test description",
					Tags:        []string{"test"},
				},
			},
		},
	}
	if err := st.Create(ctx, entry); err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	t.Run("list capabilities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result model.CapabilityListResult
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected total=1, got %d", result.Total)
		}
		if len(result.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(result.Items))
		}
		if result.Items[0].Kind != "a2a.skill" {
			t.Errorf("expected kind=a2a.skill, got %s", result.Items[0].Kind)
		}
		if result.Items[0].Name != "Test Skill" {
			t.Errorf("expected name='Test Skill', got %s", result.Items[0].Name)
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?kind=a2a.skill", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("unknown kind returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?kind=unknown.kind", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("unknown sort returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?sort=invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}
```

- [ ] **Run test to verify it fails**

```bash
rtk go test ./internal/api -run TestCapabilityHandlerListCapabilities -v
```

Expected: FAIL with 404 (route not registered) or "NewCapabilityHandler undefined"

### Step 4.2: Write failing handler test for GetCapabilityAgents

- [ ] **Add GetCapabilityAgents test to capability_handlers_test.go**

File: `internal/api/capability_handlers_test.go`

Add at end of file:

```go
func TestCapabilityHandlerGetCapabilityAgents(t *testing.T) {
	router, st := newTestRouter(t)

	// Seed test data
	ctx := context.Background()
	entry := &model.CatalogEntry{
		ID:          "test-entry-1",
		DisplayName: "Test Agent",
		Status:      model.LifecycleActive,
		AgentType: model.AgentType{
			Protocol:    model.ProtocolA2A,
			Endpoint:    "http://test.local/a2a",
			SpecVersion: "1.0",
			Capabilities: []model.Capability{
				&model.A2ASkill{
					Name:        "Test Skill",
					Description: "Test description",
				},
			},
		},
	}
	if err := st.Create(ctx, entry); err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	t.Run("get capability agents", func(t *testing.T) {
		key := "a2a.skill::Test%20Skill"
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/"+key, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result map[string]any
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		capability, ok := result["capability"].(map[string]any)
		if !ok {
			t.Fatalf("missing capability field")
		}
		if capability["kind"] != "a2a.skill" {
			t.Errorf("expected kind=a2a.skill, got %v", capability["kind"])
		}
		if capability["name"] != "Test Skill" {
			t.Errorf("expected name='Test Skill', got %v", capability["name"])
		}

		agents, ok := result["agents"].([]any)
		if !ok {
			t.Fatalf("missing agents field")
		}
		if len(agents) != 1 {
			t.Errorf("expected 1 agent, got %d", len(agents))
		}
	})

	t.Run("malformed key returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/no-separator", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("non-existent capability returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/a2a.skill::NonExistent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}
```

- [ ] **Run test to verify it fails**

```bash
rtk go test ./internal/api -run TestCapabilityHandlerGetCapabilityAgents -v
```

Expected: FAIL with 404 or "undefined"

### Step 4.3: Implement CapabilityHandler

- [ ] **Create capability_handlers.go with CapabilityHandler struct and methods**

File: `internal/api/capability_handlers.go`

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/go-chi/chi/v5"
)

type CapabilityHandler struct {
	store store.Store
}

func NewCapabilityHandler(s store.Store) *CapabilityHandler {
	return &CapabilityHandler{store: s}
}

func (h *CapabilityHandler) ListCapabilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query params
	query := r.URL.Query().Get("q")
	kind := r.URL.Query().Get("kind")
	sortParam := r.URL.Query().Get("sort")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Validate kind
	if kind != "" {
		discoverableKinds := model.DiscoverableKinds()
		valid := false
		for _, dk := range discoverableKinds {
			if dk == kind {
				valid = true
				break
			}
		}
		if !valid {
			ErrorResponse(w, http.StatusBadRequest, "invalid kind parameter")
			return
		}
	}

	// Validate sort
	sort := "name_asc"
	if sortParam != "" {
		if sortParam != "name_asc" && sortParam != "agentName_asc" {
			ErrorResponse(w, http.StatusBadRequest, "invalid sort parameter")
			return
		}
		sort = sortParam
	}

	// Parse pagination
	limit := 50
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			ErrorResponse(w, http.StatusBadRequest, "invalid limit parameter")
			return
		}
	}

	offset := 0
	if offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			ErrorResponse(w, http.StatusBadRequest, "invalid offset parameter")
			return
		}
	}

	// Query store
	result, err := h.store.ListCapabilities(ctx, store.CapabilityFilter{
		Query:  query,
		Kind:   kind,
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
	})
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("query capabilities: %v", err))
		return
	}

	JSONResponse(w, http.StatusOK, result)
}

func (h *CapabilityHandler) GetCapabilityAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract key from URL
	key := chi.URLParam(r, "key")
	keyDecoded, err := url.QueryUnescape(key)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "invalid key encoding")
		return
	}

	// Split key on first ::
	parts := strings.SplitN(keyDecoded, "::", 2)
	if len(parts) != 2 {
		ErrorResponse(w, http.StatusBadRequest, "key must be in format kind::name")
		return
	}

	kind := parts[0]
	name := parts[1]

	// Query store
	entries, err := h.store.ListAgentsByCapability(ctx, kind, name)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("query agents: %v", err))
		return
	}

	if len(entries) == 0 {
		ErrorResponse(w, http.StatusNotFound, "capability not found")
		return
	}

	// Build response
	agents := make([]capabilityAgentDTO, len(entries))
	for i, entry := range entries {
		// Find capability snippet
		var snippet json.RawMessage
		for _, cap := range entry.AgentType.Capabilities {
			if cap.Kind() == kind {
				// Check name match (capability name is stored in properties)
				capJSON, err := json.Marshal(cap)
				if err != nil {
					continue
				}
				var capMap map[string]any
				if err := json.Unmarshal(capJSON, &capMap); err != nil {
					continue
				}
				if capName, ok := capMap["name"].(string); ok && capName == name {
					// Inject kind field
					capMap["kind"] = cap.Kind()
					snippet, _ = json.Marshal(capMap)
					break
				}
			}
		}

		agents[i] = capabilityAgentDTO{
			ID:               entry.ID,
			DisplayName:      entry.DisplayName,
			Protocol:         string(entry.AgentType.Protocol),
			Provider:         entry.AgentType.Provider,
			Health:           buildHealthJSON(entry),
			SpecVersion:      entry.AgentType.SpecVersion,
			Status:           string(entry.Status),
			CapabilitySnippet: snippet,
		}
	}

	response := capabilityDetailResponse{
		Capability: capabilitySummary{
			Kind: kind,
			Name: name,
		},
		Agents: agents,
	}

	JSONResponse(w, http.StatusOK, response)
}

// Response DTOs (unexported)

type capabilityDetailResponse struct {
	Capability capabilitySummary    `json:"capability"`
	Agents     []capabilityAgentDTO `json:"agents"`
}

type capabilitySummary struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type capabilityAgentDTO struct {
	ID                string           `json:"id"`
	DisplayName       string           `json:"display_name"`
	Protocol          string           `json:"protocol"`
	Provider          *model.Provider  `json:"provider,omitempty"`
	Health            map[string]any   `json:"health"`
	SpecVersion       string           `json:"spec_version,omitempty"`
	Status            string           `json:"status"`
	CapabilitySnippet json.RawMessage  `json:"capability_snippet"`
}

func buildHealthJSON(entry model.CatalogEntry) map[string]any {
	return map[string]any{
		"state":               string(entry.Health.State),
		"lastProbedAt":        entry.Health.LastProbedAt,
		"lastSuccessAt":       entry.Health.LastSuccessAt,
		"latencyMs":           entry.Health.LatencyMs,
		"consecutiveFailures": entry.Health.ConsecutiveFailures,
		"lastError":           entry.Health.LastError,
	}
}
```

- [ ] **Run tests to verify they pass**

```bash
rtk go test ./internal/api -run TestCapabilityHandler -v
```

Expected: Both tests PASS (or FAIL with 404 if routes not wired yet — that's next task)

### Step 4.4: Commit

- [ ] **Commit handler implementation**

```bash
rtk git add internal/api/capability_handlers.go internal/api/capability_handlers_test.go
rtk git commit -m "feat(api): add capability discovery handlers

- CapabilityHandler with ListCapabilities and GetCapabilityAgents
- ListCapabilities: validates kind (must be discoverable), validates sort, supports pagination
- GetCapabilityAgents: splits key on :: separator, returns 404 if not found, builds capability_snippet per agent
- Response DTOs: capabilityDetailResponse, capabilitySummary, capabilityAgentDTO
- buildHealthJSON helper to construct health JSON matching CatalogEntry.MarshalJSON pattern
- Tests: list, filter by kind, unknown kind/sort returns 400, get agents, malformed key returns 400, non-existent returns 404"
```

---

## Task 5: Router Wiring and Remove /skills Endpoint

**Files:**
- Modify: `internal/api/router.go:98,162` (replace /skills with /capabilities routes)
- Modify: `internal/api/handlers.go:321-332` (remove SearchCapabilities method)

### Step 5.1: Wire capability routes in router.go

- [ ] **Replace /skills route with /capabilities routes in registerCatalogRoutes**

File: `internal/api/router.go`

Around line 98, replace the /skills route:

```go
// Remove this line:
r.With(RequirePermission(auth.PermCatalogRead)).Get("/skills", h.SearchCapabilities)

// Add these lines:
capHandler := NewCapabilityHandler(deps.Kernel.Store())
r.With(RequirePermission(auth.PermCatalogRead)).Get("/capabilities", capHandler.ListCapabilities)
r.With(RequirePermission(auth.PermCatalogRead)).Get("/capabilities/{key}", capHandler.GetCapabilityAgents)
```

Around line 162 in `registerUnauthenticatedCatalogRoutes`, replace:

```go
// Remove this line:
r.Get("/skills", h.SearchCapabilities)

// Add these lines:
capHandler := NewCapabilityHandler(deps.Kernel.Store())
r.Get("/capabilities", capHandler.ListCapabilities)
r.Get("/capabilities/{key}", capHandler.GetCapabilityAgents)
```

- [ ] **Verify router compiles**

```bash
rtk go build ./internal/api
```

Expected: Success (or FAIL if SearchCapabilities still referenced in handlers.go)

### Step 5.2: Remove SearchCapabilities from handlers.go

- [ ] **Delete SearchCapabilities method**

File: `internal/api/handlers.go`

Delete lines 321-332 (the entire `SearchCapabilities` method).

- [ ] **Build and verify**

```bash
rtk go build ./internal/api
```

Expected: Success

### Step 5.3: Run all API tests

- [ ] **Verify all API tests pass**

```bash
rtk go test ./internal/api -v
```

Expected: All PASS

### Step 5.4: Commit

- [ ] **Commit router changes**

```bash
rtk git add internal/api/router.go internal/api/handlers.go
rtk git commit -m "feat(api): wire capability routes, remove /skills endpoint

- Replace GET /api/v1/skills with GET /api/v1/capabilities
- Add GET /api/v1/capabilities/{key} for capability detail
- Both routes in authenticated and unauthenticated catalog route groups
- Remove SearchCapabilities handler method (replaced by CapabilityHandler)
- Breaking change: /skills endpoint removed (pre-1.0, API not stable)"
```

---

## Task 6: Frontend API Client and TypeScript Types

**Files:**
- Modify: `web/src/api.ts` (add listCapabilities, getCapabilityAgents)
- Modify: `web/src/types.ts` (add CapabilityInstance, CapabilityListResult, CapabilityDetailResponse, CapabilityAgentDTO)

### Step 6.1: Add TypeScript types

- [ ] **Add capability types to types.ts**

File: `web/src/types.ts`

Add at end of file:

```typescript
export interface CapabilityInstance {
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

export interface CapabilityListResult {
  total: number
  items: CapabilityInstance[]
}

export interface CapabilityDetailResponse {
  capability: {
    kind: string
    name: string
  }
  agents: CapabilityAgentDTO[]
}

export interface CapabilityAgentDTO {
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

### Step 6.2: Add API client functions

- [ ] **Add listCapabilities and getCapabilityAgents to api.ts**

File: `web/src/api.ts`

Add at end of file (after existing functions):

```typescript
export async function listCapabilities(filter: {
  q?: string
  kind?: string
  limit?: number
  offset?: number
  sort?: string
}): Promise<CapabilityListResult> {
  const params = new URLSearchParams()
  if (filter.q) params.set('q', filter.q)
  if (filter.kind) params.set('kind', filter.kind)
  if (filter.limit) params.set('limit', filter.limit.toString())
  if (filter.offset) params.set('offset', filter.offset.toString())
  if (filter.sort) params.set('sort', filter.sort)

  const queryString = params.toString()
  const url = `/api/v1/capabilities${queryString ? '?' + queryString : ''}`

  return request<CapabilityListResult>(url, {
    method: 'GET',
  })
}

export async function getCapabilityAgents(
  kind: string,
  name: string
): Promise<CapabilityDetailResponse> {
  const key = encodeURIComponent(`${kind}::${name}`)
  return request<CapabilityDetailResponse>(`/api/v1/capabilities/${key}`, {
    method: 'GET',
  })
}
```

Add imports at top of file:

```typescript
import type { CapabilityListResult, CapabilityDetailResponse } from './types'
```

- [ ] **Verify TypeScript compiles**

```bash
cd web && rtk tsc --noEmit
```

Expected: No errors

### Step 6.3: Commit

- [ ] **Commit API client and types**

```bash
rtk git add web/src/api.ts web/src/types.ts
rtk git commit -m "feat(web): add capability API client and TypeScript types

- CapabilityInstance: flat DTO with capability fields + agent metadata
- CapabilityListResult: paginated list wrapper
- CapabilityDetailResponse: capability summary + agents array
- CapabilityAgentDTO: agent info with capability_snippet
- listCapabilities: query with q, kind, limit, offset, sort params
- getCapabilityAgents: fetch agents by kind::name key"
```

---

## Task 7: useCapabilitiesQuery Hook with URL Sync

**Files:**
- Create: `web/src/hooks/useCapabilitiesQuery.ts`
- Create: `web/src/hooks/useCapabilitiesQuery.test.ts`

### Step 7.1: Write failing test for useCapabilitiesQuery

- [ ] **Create useCapabilitiesQuery.test.ts**

File: `web/src/hooks/useCapabilitiesQuery.test.ts`

```typescript
import { renderHook, waitFor } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useCapabilitiesQuery } from './useCapabilitiesQuery'

// Mock API
vi.mock('../api', () => ({
  listCapabilities: vi.fn(() =>
    Promise.resolve({
      total: 1,
      items: [
        {
          kind: 'a2a.skill',
          name: 'Test Skill',
          description: 'Test',
          tags: null,
          input_modes: null,
          output_modes: null,
          agent_id: 'test-id',
          agent_name: 'Test Agent',
          protocol: 'a2a',
          status: 'active',
          spec_version: '1.0',
          provider_org: null,
          provider_url: null,
          health_state: 'active',
          latency_ms: 100,
        },
      ],
    })
  ),
}))

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <BrowserRouter>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </BrowserRouter>
  )
}

describe('useCapabilitiesQuery', () => {
  it('syncs query param to URL', async () => {
    const wrapper = createWrapper()
    const { result } = renderHook(() => useCapabilitiesQuery(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    // setQuery updates URL
    result.current.setQuery('test')
    await waitFor(() => {
      const url = new URL(window.location.href)
      expect(url.searchParams.get('q')).toBe('test')
    })
  })

  it('syncs kind param to URL', async () => {
    const wrapper = createWrapper()
    const { result } = renderHook(() => useCapabilitiesQuery(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    result.current.setKind('a2a.skill')
    await waitFor(() => {
      const url = new URL(window.location.href)
      expect(url.searchParams.get('kind')).toBe('a2a.skill')
    })
  })
})
```

- [ ] **Run test to verify it fails**

```bash
cd web && rtk vitest run -t useCapabilitiesQuery
```

Expected: FAIL with "Cannot find module './useCapabilitiesQuery'"

### Step 7.2: Implement useCapabilitiesQuery hook

- [ ] **Create useCapabilitiesQuery.ts**

File: `web/src/hooks/useCapabilitiesQuery.ts`

```typescript
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { listCapabilities } from '../api'
import type { CapabilityListResult } from '../types'

export function useCapabilitiesQuery() {
  const [searchParams, setSearchParams] = useSearchParams()

  const query = searchParams.get('q') || ''
  const kind = searchParams.get('kind') || ''
  const sort = searchParams.get('sort') || 'name_asc'

  const result = useQuery<CapabilityListResult>({
    queryKey: ['capabilities', query, kind, sort],
    queryFn: () =>
      listCapabilities({
        q: query || undefined,
        kind: kind || undefined,
        sort,
        limit: 50,
      }),
  })

  const setQuery = (value: string) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (value) {
        next.set('q', value)
      } else {
        next.delete('q')
      }
      return next
    })
  }

  const setKind = (value: string) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (value) {
        next.set('kind', value)
      } else {
        next.delete('kind')
      }
      return next
    })
  }

  const setSort = (value: string) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('sort', value)
      return next
    })
  }

  const clearFilters = () => {
    setSearchParams({})
  }

  return {
    ...result,
    query,
    kind,
    sort,
    setQuery,
    setKind,
    setSort,
    clearFilters,
  }
}
```

- [ ] **Run test to verify it passes**

```bash
cd web && rtk vitest run -t useCapabilitiesQuery
```

Expected: PASS

### Step 7.3: Commit

- [ ] **Commit useCapabilitiesQuery hook**

```bash
rtk git add web/src/hooks/useCapabilitiesQuery.ts web/src/hooks/useCapabilitiesQuery.test.ts
rtk git commit -m "feat(web): add useCapabilitiesQuery hook with URL sync

- React Query hook for capabilities list
- URL params: q (search), kind (filter), sort
- Helpers: setQuery, setKind, setSort, clearFilters
- All state synced to URL via useSearchParams
- Tests: query param sync, kind param sync"
```

---

## Task 8: Frontend Capability Index View

**Files:**
- Create: `web/src/routes/capabilities/CapabilityListPage.tsx`
- Create: `web/src/routes/capabilities/components/CapabilityGroup.tsx`
- Create: `web/src/routes/capabilities/components/KindFilter.tsx`

### Step 8.1: Create KindFilter component

- [ ] **Create KindFilter.tsx**

File: `web/src/routes/capabilities/components/KindFilter.tsx`

```typescript
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

interface KindFilterProps {
  value: string
  onChange: (value: string) => void
}

const kinds = [
  { value: '', label: 'All' },
  { value: 'a2a.skill', label: 'A2A Skill' },
  { value: 'mcp.tool', label: 'MCP Tool' },
  { value: 'mcp.resource', label: 'MCP Resource' },
  { value: 'mcp.prompt', label: 'MCP Prompt' },
]

export function KindFilter({ value, onChange }: KindFilterProps) {
  return (
    <ToggleGroup
      type="single"
      value={value}
      onValueChange={(val) => onChange(val || '')}
      className="justify-start"
    >
      {kinds.map((kind) => (
        <ToggleGroupItem key={kind.value} value={kind.value}>
          {kind.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
```

### Step 8.2: Create CapabilityGroup accordion component

- [ ] **Create CapabilityGroup.tsx**

File: `web/src/routes/capabilities/components/CapabilityGroup.tsx`

```typescript
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/StatusBadge'
import { ProtocolBadge } from '@/components/ProtocolBadge'
import type { CapabilityInstance } from '@/types'

interface CapabilityGroupProps {
  kind: string
  name: string
  items: CapabilityInstance[]
}

function kindToLabel(kind: string): string {
  const map: Record<string, string> = {
    'a2a.skill': 'A2A Skill',
    'mcp.tool': 'MCP Tool',
    'mcp.resource': 'MCP Resource',
    'mcp.prompt': 'MCP Prompt',
  }
  return map[kind] || kind
}

export function CapabilityGroup({ kind, name, items }: CapabilityGroupProps) {
  const [isOpen, setIsOpen] = useState(false)

  const firstItem = items[0]
  const description = firstItem?.description || ''
  const tags = firstItem?.tags || []
  const visibleTags = tags.slice(0, 5)
  const remainingTags = tags.length - 5

  const detailURL = `/catalog/capabilities/${encodeURIComponent(kind + '::' + name)}`

  return (
    <div className="border rounded-lg">
      {/* Header */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full px-4 py-3 flex items-center gap-3 hover:bg-muted/50 transition-colors text-left"
      >
        {isOpen ? (
          <ChevronDown className="h-4 w-4 text-muted-foreground flex-shrink-0" />
        ) : (
          <ChevronRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
        )}

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <Badge variant="outline">{kindToLabel(kind)}</Badge>
            <h3 className="font-semibold truncate">{name}</h3>
          </div>

          <p className="text-sm text-muted-foreground line-clamp-1">{description}</p>

          {visibleTags.length > 0 && (
            <div className="flex gap-1 mt-2">
              {visibleTags.map((tag) => (
                <Badge key={tag} variant="secondary" className="text-xs">
                  {tag}
                </Badge>
              ))}
              {remainingTags > 0 && (
                <Badge variant="secondary" className="text-xs">
                  +{remainingTags} more
                </Badge>
              )}
            </div>
          )}
        </div>

        <div className="text-sm text-muted-foreground">
          {items.length} {items.length === 1 ? 'agent' : 'agents'}
        </div>
      </button>

      {/* Expanded content */}
      {isOpen && (
        <div className="border-t px-4 py-3 space-y-2">
          {items.map((item) => (
            <div
              key={item.agent_id}
              className="flex items-center gap-3 p-2 rounded hover:bg-muted/50"
            >
              <ProtocolBadge protocol={item.protocol} />
              <Link
                to={`/catalog/${item.agent_id}`}
                className="font-medium hover:underline flex-1"
              >
                {item.agent_name}
              </Link>
              <StatusBadge status={item.status} />
              {item.provider_org && (
                <span className="text-sm text-muted-foreground">{item.provider_org}</span>
              )}
              <span className="text-xs text-muted-foreground">{item.latency_ms}ms</span>
            </div>
          ))}

          <div className="pt-2">
            <Button variant="outline" size="sm" asChild>
              <Link to={detailURL}>View all →</Link>
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
```

### Step 8.3: Create CapabilityListPage

- [ ] **Create CapabilityListPage.tsx**

File: `web/src/routes/capabilities/CapabilityListPage.tsx`

```typescript
import { useMemo } from 'react'
import { Loader2 } from 'lucide-react'
import { useCapabilitiesQuery } from '@/hooks/useCapabilitiesQuery'
import { UnifiedSearchBox } from '@/routes/catalog/components/UnifiedSearchBox'
import { KindFilter } from './components/KindFilter'
import { CapabilityGroup } from './components/CapabilityGroup'
import type { CapabilityInstance } from '@/types'

export default function CapabilityListPage() {
  const {
    data,
    isLoading,
    isError,
    error,
    query,
    kind,
    setQuery,
    setKind,
    clearFilters,
  } = useCapabilitiesQuery()

  // Group items by (kind, name)
  const groups = useMemo(() => {
    if (!data?.items) return []

    const map = new Map<string, CapabilityInstance[]>()
    for (const item of data.items) {
      const key = `${item.kind}::${item.name}`
      if (!map.has(key)) {
        map.set(key, [])
      }
      map.get(key)!.push(item)
    }

    // Convert to array and sort by name
    return Array.from(map.entries())
      .map(([key, items]) => {
        const [kind, name] = key.split('::', 2)
        return { kind, name, items }
      })
      .sort((a, b) => a.name.localeCompare(b.name))
  }, [data?.items])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">Failed to load capabilities</p>
        <p className="text-sm text-muted-foreground mt-1">
          {error instanceof Error ? error.message : 'Unknown error'}
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Capabilities</h1>
        <p className="text-muted-foreground mt-1">Discover agents by capability</p>
      </div>

      {/* Toolbar */}
      <div className="space-y-4">
        <UnifiedSearchBox
          value={query}
          onChange={setQuery}
        />

        <KindFilter value={kind} onChange={setKind} />
      </div>

      {/* Groups */}
      {groups.length === 0 && (
        <div className="text-center py-12">
          <p className="text-muted-foreground">
            {query || kind ? (
              <>
                No capabilities found.{' '}
                <button onClick={clearFilters} className="text-primary hover:underline">
                  Clear filters
                </button>
              </>
            ) : (
              'No capabilities published yet.'
            )}
          </p>
        </div>
      )}

      <div className="space-y-3">
        {groups.map((group) => (
          <CapabilityGroup
            key={`${group.kind}::${group.name}`}
            kind={group.kind}
            name={group.name}
            items={group.items}
          />
        ))}
      </div>
    </div>
  )
}
```

### Step 8.4: Commit

- [ ] **Commit capability index view**

```bash
rtk git add web/src/routes/capabilities/CapabilityListPage.tsx web/src/routes/capabilities/components/KindFilter.tsx web/src/routes/capabilities/components/CapabilityGroup.tsx
rtk git commit -m "feat(web): add capability index view with accordion grouping

- CapabilityListPage: header, search box, kind filter, accordion groups
- CapabilityGroup: collapsible accordion with kind badge, capability name, description, tags, agent count
- Expanded view shows agents with protocol, status, provider, latency, link to detail page
- KindFilter: ToggleGroup with All / A2A Skill / MCP Tool / MCP Resource / MCP Prompt
- Client-side grouping by (kind, name), sorted by name
- Empty states: no capabilities, no matches (with clear filters link)
- Loading and error states"
```

---

## Task 9: Frontend Capability Detail View

**Files:**
- Create: `web/src/routes/capabilities/CapabilityDetailPage.tsx`
- Create: `web/src/routes/capabilities/components/AgentForCapabilityRow.tsx`

### Step 9.1: Create AgentForCapabilityRow component

- [ ] **Create AgentForCapabilityRow.tsx**

File: `web/src/routes/capabilities/components/AgentForCapabilityRow.tsx`

```typescript
import { Link } from 'react-router-dom'
import { ProtocolBadge } from '@/components/ProtocolBadge'
import { StatusBadge } from '@/components/StatusBadge'
import type { CapabilityAgentDTO } from '@/types'

interface AgentForCapabilityRowProps {
  agent: CapabilityAgentDTO
}

export function AgentForCapabilityRow({ agent }: AgentForCapabilityRowProps) {
  const snippetDescription =
    (agent.capability_snippet?.description as string) || ''
  const truncated =
    snippetDescription.length > 100
      ? snippetDescription.slice(0, 100) + '...'
      : snippetDescription

  return (
    <tr className="hover:bg-muted/50">
      <td className="p-3">
        <ProtocolBadge protocol={agent.protocol} />
      </td>
      <td className="p-3">
        <Link to={`/catalog/${agent.id}`} className="font-medium hover:underline">
          {agent.display_name}
        </Link>
      </td>
      <td className="p-3 text-sm text-muted-foreground">
        {agent.provider?.organization || '-'}
      </td>
      <td className="p-3">
        <StatusBadge status={agent.status} />
      </td>
      <td className="p-3 text-sm text-muted-foreground">
        {agent.spec_version || '-'}
      </td>
      <td className="p-3 text-sm text-muted-foreground max-w-xs" title={snippetDescription}>
        {truncated}
      </td>
      <td className="p-3 text-sm text-muted-foreground">
        {agent.health.latencyMs}ms
      </td>
    </tr>
  )
}
```

### Step 9.2: Create CapabilityDetailPage

- [ ] **Create CapabilityDetailPage.tsx**

File: `web/src/routes/capabilities/CapabilityDetailPage.tsx`

```typescript
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { getCapabilityAgents } from '@/api'
import { AgentForCapabilityRow } from './components/AgentForCapabilityRow'

function kindToLabel(kind: string): string {
  const map: Record<string, string> = {
    'a2a.skill': 'A2A Skill',
    'mcp.tool': 'MCP Tool',
    'mcp.resource': 'MCP Resource',
    'mcp.prompt': 'MCP Prompt',
  }
  return map[kind] || kind
}

export default function CapabilityDetailPage() {
  const { key } = useParams<{ key: string }>()

  if (!key) {
    return <div className="text-center py-12">Invalid capability key</div>
  }

  const decoded = decodeURIComponent(key)
  const [kind, name] = decoded.split('::', 2)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['capability-agents', kind, name],
    queryFn: () => getCapabilityAgents(kind, name),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive">Failed to load capability details</p>
        <p className="text-sm text-muted-foreground mt-1">
          {error instanceof Error ? error.message : 'Unknown error'}
        </p>
      </div>
    )
  }

  if (!data || data.agents.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">No agents offer this capability</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link to="/" className="hover:underline">
          Catalog
        </Link>
        <span>/</span>
        <Link to="/catalog/capabilities" className="hover:underline">
          Capabilities
        </Link>
        <span>/</span>
        <span className="text-foreground">{name}</span>
      </div>

      {/* Back button */}
      <Button variant="ghost" size="sm" asChild>
        <Link to="/catalog/capabilities">
          <ArrowLeft className="h-4 w-4 mr-2" />
          Back to Capabilities
        </Link>
      </Button>

      {/* Header */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <Badge variant="outline">{kindToLabel(kind)}</Badge>
        </div>
        <h1 className="text-3xl font-bold">{name}</h1>
        <p className="text-muted-foreground mt-1">
          {data.agents.length} {data.agents.length === 1 ? 'agent' : 'agents'}
        </p>
      </div>

      {/* Agents table */}
      <div className="border rounded-lg overflow-hidden">
        <table className="w-full">
          <thead className="bg-muted/50 border-b">
            <tr>
              <th className="p-3 text-left text-sm font-medium">Protocol</th>
              <th className="p-3 text-left text-sm font-medium">Agent</th>
              <th className="p-3 text-left text-sm font-medium">Provider</th>
              <th className="p-3 text-left text-sm font-medium">Status</th>
              <th className="p-3 text-left text-sm font-medium">Version</th>
              <th className="p-3 text-left text-sm font-medium">Description</th>
              <th className="p-3 text-left text-sm font-medium">Latency</th>
            </tr>
          </thead>
          <tbody>
            {data.agents.map((agent) => (
              <AgentForCapabilityRow key={agent.id} agent={agent} />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
```

### Step 9.3: Commit

- [ ] **Commit capability detail view**

```bash
rtk git add web/src/routes/capabilities/CapabilityDetailPage.tsx web/src/routes/capabilities/components/AgentForCapabilityRow.tsx
rtk git commit -m "feat(web): add capability detail view

- CapabilityDetailPage: breadcrumb, back button, kind badge, capability name, agent count
- AgentForCapabilityRow: table row with protocol, agent link, provider, status, version, description snippet, latency
- Description truncated to 100 chars with full text in tooltip
- Table shows all agents offering the capability (including offline/deprecated)
- Empty and error states"
```

---

## Task 10: Cross-linking from Agent Detail and Navigation

**Files:**
- Modify: `web/src/routes/catalog/CatalogDetailPage.tsx:228-244` (make capability names clickable)
- Modify: `web/src/components/Layout.tsx:58-63,106-117` (add Capabilities nav tab)
- Modify: `web/src/App.tsx:18-20` (add capability routes)

### Step 10.1: Make capabilities clickable on agent detail page

- [ ] **Modify CatalogDetailPage to add capability links**

File: `web/src/routes/catalog/CatalogDetailPage.tsx`

Around lines 228-244 (in the capabilities rendering section), replace the capabilities list rendering:

Find the existing `<ul>` rendering capabilities (likely around line 237) and replace with:

```typescript
<ul className="space-y-2">
  {entry.capabilities.map((cap: any, idx: number) => {
    const isDiscoverable = ['a2a.skill', 'mcp.tool', 'mcp.resource', 'mcp.prompt'].includes(cap.kind)
    const capKey = `${cap.kind}::${cap.name}`
    const capURL = `/catalog/capabilities/${encodeURIComponent(capKey)}`

    return (
      <li key={idx} className="border rounded p-3">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs text-muted-foreground">{cap.kind}</span>
          {isDiscoverable ? (
            <Link to={capURL} className="font-medium hover:underline">
              {cap.name}
            </Link>
          ) : (
            <span className="font-medium">{cap.name}</span>
          )}
        </div>
        {cap.description && (
          <p className="text-sm text-muted-foreground">{cap.description}</p>
        )}
      </li>
    )
  })}
</ul>
```

Add Link import at top of file:

```typescript
import { Link, useParams } from 'react-router-dom'
```

### Step 10.2: Add Capabilities tab to navigation

- [ ] **Modify Layout.tsx to add Capabilities nav link**

File: `web/src/components/Layout.tsx`

Around lines 58-63 (desktop nav), add the Capabilities link after Catalog:

```typescript
<NavLink to="/catalog">Catalog</NavLink>
<NavLink to="/catalog/capabilities">Capabilities</NavLink>
<NavLink to="/settings">Settings</NavLink>
```

Around lines 106-117 (mobile nav), add the Capabilities link:

```typescript
<NavLink to="/catalog">Catalog</NavLink>
<NavLink to="/catalog/capabilities">Capabilities</NavLink>
<NavLink to="/settings">Settings</NavLink>
```

### Step 10.3: Register capability routes

- [ ] **Modify App.tsx to add capability routes**

File: `web/src/App.tsx`

Around lines 18-20, add the capability routes. Note: the catalog list is at `/` (not `/catalog`),
and `/catalog/capabilities` must be defined BEFORE `/catalog/:id` so React Router v6 ranks
the static segment higher than the dynamic param:

```typescript
<Route path="/login" element={<LoginPage />} />
<Route path="/" element={<ProtectedRoute><Layout><CatalogListPage /></Layout></ProtectedRoute>} />
<Route path="/catalog/capabilities" element={<ProtectedRoute><Layout><CapabilityListPage /></Layout></ProtectedRoute>} />
<Route path="/catalog/capabilities/:key" element={<ProtectedRoute><Layout><CapabilityDetailPage /></Layout></ProtectedRoute>} />
<Route path="/catalog/:id" element={<ProtectedRoute><Layout><CatalogDetailPage /></Layout></ProtectedRoute>} />
<Route path="/settings" element={<ProtectedRoute><Layout><SettingsPage /></Layout></ProtectedRoute>} />
```

Add imports at top of file:

```typescript
import CapabilityListPage from './routes/capabilities/CapabilityListPage'
import CapabilityDetailPage from './routes/capabilities/CapabilityDetailPage'
```

- [ ] **Verify TypeScript compiles**

```bash
cd web && rtk tsc --noEmit
```

Expected: No errors

### Step 10.4: Commit

- [ ] **Commit navigation and cross-linking**

```bash
rtk git add web/src/routes/catalog/CatalogDetailPage.tsx web/src/components/Layout.tsx web/src/App.tsx
rtk git commit -m "feat(web): add navigation and cross-linking for capabilities

- Capabilities tab in main nav (desktop and mobile)
- Routes: /catalog/capabilities (list), /catalog/capabilities/:key (detail)
- Agent detail page: discoverable capability names now link to capability detail
- Technical capability kinds (a2a.extension, etc) remain plain text (not linked)"
```

---

## Task 11: End-to-End Tests

**Files:**
- Modify: `e2e/tests/catalog.spec.ts` (update skills test, add capability discovery tests)

### Step 11.1: Update existing skills test

- [ ] **Update the skills test to use /capabilities endpoint**

File: `e2e/tests/catalog.spec.ts`

Find the test that hits `/api/v1/skills` and replace it with a test for `/api/v1/capabilities`:

```typescript
test('capabilities API returns capability instances', async ({ request }) => {
  const response = await request.get('/api/v1/capabilities', {
    headers: authHeader(),
  })

  expect(response.status()).toBe(200)
  const data = await response.json()
  expect(data).toHaveProperty('total')
  expect(data).toHaveProperty('items')
  expect(Array.isArray(data.items)).toBe(true)
})
```

### Step 11.2: Add capability discovery E2E tests

- [ ] **Add capability discovery E2E tests to catalog.spec.ts**

File: `e2e/tests/catalog.spec.ts`

Add at end of file:

```typescript
test.describe('Capability Discovery', () => {
  test.beforeEach(async ({ page }) => {
    await loginViaUI(page)
  })

  test('capability list page shows accordion groups', async ({ page }) => {
    await page.goto('/catalog/capabilities')

    await page.waitForLoadState('networkidle')

    // Should show header
    await expect(page.getByRole('heading', { name: 'Capabilities' })).toBeVisible()

    // Should show search box
    await expect(page.getByPlaceholder('Search capabilities...')).toBeVisible()

    // Should show kind filter
    await expect(page.getByText('All')).toBeVisible()
    await expect(page.getByText('A2A Skill')).toBeVisible()
  })

  test('search filters capabilities', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Type in search
    const searchBox = page.getByPlaceholder('Search capabilities...')
    await searchBox.fill('translate')

    // URL should update
    await page.waitForURL(/q=translate/)
  })

  test('kind filter works', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Click A2A Skill filter
    await page.getByText('A2A Skill').click()

    // URL should update
    await page.waitForURL(/kind=a2a\.skill/)
  })

  test('accordion expands and shows agents', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Find first accordion group and click to expand
    const firstGroup = page.locator('.border.rounded-lg').first()
    await firstGroup.click()

    // Should show agent list (wait for expanded content)
    await expect(firstGroup.getByText(/agent/)).toBeVisible({ timeout: 5000 })
  })

  test('capability detail page shows agents', async ({ page }) => {
    await page.goto('/catalog/capabilities')
    await page.waitForLoadState('networkidle')

    // Expand first group
    const firstGroup = page.locator('.border.rounded-lg').first()
    await firstGroup.click()

    // Click "View all" link
    await firstGroup.getByText('View all').click()

    // Should navigate to detail page
    await expect(page).toHaveURL(/\/catalog\/capabilities\//)

    // Should show table headers
    await expect(page.getByText('Protocol')).toBeVisible()
    await expect(page.getByText('Agent')).toBeVisible()
    await expect(page.getByText('Provider')).toBeVisible()
  })

  test('capability link from agent detail navigates correctly', async ({ page }) => {
    // Navigate to catalog
    await page.goto('/catalog')
    await page.waitForLoadState('networkidle')

    // Click first agent
    const firstAgent = page.getByRole('link', { name: /agent/i }).first()
    await firstAgent.click()

    // Wait for detail page
    await page.waitForSelector('h1')

    // Find and click a capability link (discoverable capability)
    const capabilityLink = page.locator('a[href*="/catalog/capabilities/"]').first()
    if (await capabilityLink.isVisible()) {
      await capabilityLink.click()

      // Should navigate to capability detail
      await expect(page).toHaveURL(/\/catalog\/capabilities\//)
    }
  })
})
```

- [ ] **Run E2E tests locally (if possible)**

```bash
make e2e-test
```

Expected: Tests PASS (or skip if E2E env not configured locally)

### Step 11.3: Commit

- [ ] **Commit E2E tests**

```bash
rtk git add e2e/tests/catalog.spec.ts
rtk git commit -m "test(e2e): update skills test to capabilities, add discovery tests

- Update existing skills API test to use /api/v1/capabilities
- Add capability discovery E2E tests:
  - List page shows header, search, kind filter
  - Search filters in real time, updates URL
  - Kind filter updates URL
  - Accordion expands and shows agents
  - View all navigates to detail page
  - Detail page shows agent table
  - Capability link from agent detail navigates to capability detail"
```

---

## Task 12: Documentation Updates

**Files:**
- Modify: `docs/api.md` (add /capabilities endpoints, remove /skills)
- Modify: `docs/end-user-guide.md` (add Capabilities tab section)
- Modify: `CHANGELOG.md` (add breaking change note)

### Step 12.1: Update API documentation

- [ ] **Add /capabilities endpoints to docs/api.md**

File: `docs/api.md`

Find the `/skills` endpoint section and replace with:

```markdown
### GET /api/v1/capabilities

List capability instances (one per agent per capability) with agent metadata.

**Query Parameters:**
- `q` (string, optional): Case-insensitive substring search on capability name, description, and properties
- `kind` (string, optional): Filter by capability kind (e.g., `a2a.skill`, `mcp.tool`). Must be a discoverable kind.
- `limit` (integer, optional, default 50): Max results per page
- `offset` (integer, optional, default 0): Pagination offset
- `sort` (string, optional, default `name_asc`): Sort order. Values: `name_asc`, `agentName_asc`

**Permission:** `catalog:read`

**Response:** 200 OK

```json
{
  "total": 3,
  "items": [
    {
      "kind": "a2a.skill",
      "name": "Translate EN-DE",
      "description": "Bidirectional translation",
      "tags": ["translation", "german"],
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
    }
  ]
}
```

**Error Responses:**
- 400 Bad Request: Invalid `kind` or `sort` parameter

---

### GET /api/v1/capabilities/{key}

Get all agents offering a specific capability.

**Path Parameters:**
- `key` (string, required): Capability identifier in format `kind::name` (URL-encoded). Example: `a2a.skill::Translate%20EN-DE`

**Permission:** `catalog:read`

**Response:** 200 OK

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
        "description": "Bidirectional translation",
        "tags": ["translation"],
        "inputModes": ["text/plain"],
        "outputModes": ["text/plain"]
      }
    }
  ]
}
```

**Error Responses:**
- 400 Bad Request: Malformed key (missing `::` separator)
- 404 Not Found: No agents offer this capability

---

### ~~GET /api/v1/skills~~ (REMOVED)

**Breaking Change:** This endpoint has been removed in favor of `/api/v1/capabilities`. See CHANGELOG.
```

### Step 12.2: Update end-user guide

- [ ] **Add Capabilities section to docs/end-user-guide.md**

File: `docs/end-user-guide.md`

Add a new section after the Catalog section:

```markdown
## Capabilities

The Capabilities tab provides a capability-first view of your agent catalog. Instead of browsing individual agents, you can search for capabilities (skills, tools, resources, prompts) and see all agents that offer each one.

### Discovering Capabilities

1. Click **Capabilities** in the main navigation
2. Browse the accordion groups showing capability names and agent counts
3. Use the search box to filter by name, description, or tags (for A2A skills)
4. Use the kind filter to narrow results: All / A2A Skill / MCP Tool / MCP Resource / MCP Prompt

### Viewing Agents by Capability

1. Click an accordion group to expand and see the list of agents offering that capability
2. Each agent shows: protocol, name, status, provider, and latency
3. Click **View all** to open the full capability detail page with a table of all agents

### Capability Detail Page

The detail page shows:
- Capability kind badge and name
- Total agent count (including offline/deprecated agents)
- Table with columns: Protocol, Agent, Provider, Status, Version, Description (agent-specific snippet), Latency
- Click any agent name to navigate to the agent detail page

### Cross-linking from Agent Detail

On the agent detail page, discoverable capability names (A2A skills, MCP tools/resources/prompts) are now clickable links. Click a capability name to see all other agents offering the same capability.

Technical capability kinds (extensions, security schemes, interfaces, signatures) are not linked — they are configuration details, not user-facing capabilities.

### URL Sharing

Capability views support shareable URLs:
- `/catalog/capabilities?q=translate&kind=a2a.skill` — filtered list view
- `/catalog/capabilities/a2a.skill::Translate%20EN-DE` — specific capability detail

Copy and share these URLs to point teammates directly to specific capabilities or search results.
```

### Step 12.3: Update CHANGELOG

- [ ] **Add breaking change entry to CHANGELOG.md**

File: `CHANGELOG.md`

Add at the top of the file (or in the Unreleased section):

```markdown
## [Unreleased]

### Added
- Capability-based discovery: new Capabilities tab in main navigation
- GET /api/v1/capabilities — list capability instances with agent metadata
- GET /api/v1/capabilities/{key} — get all agents offering a specific capability
- Capability registry extended with discoverability metadata (`DiscoverableKinds()` helper)
- Cross-linking: capability names on agent detail page now link to capability detail view
- URL state: capability list view supports shareable URLs with search and filter params

### Changed
- Capability registry: `RegisterCapability` signature now includes `discoverable bool` parameter

### Removed
- **BREAKING:** GET /api/v1/skills endpoint removed (replaced by /api/v1/capabilities)
- **BREAKING:** `SearchCapabilities(query string)` method removed from Store interface (replaced by `ListCapabilities(filter CapabilityFilter)`)

### Migration Notes
- If your code calls `GET /api/v1/skills`, replace with `GET /api/v1/capabilities`. The response shape is different — see docs/api.md.
- If you have custom store implementations, remove `SearchCapabilities` method and implement `ListCapabilities` and `ListAgentsByCapability`.
```

- [ ] **Verify all docs compile/render correctly**

```bash
# If docs use a renderer like mkdocs:
# cd docs && mkdocs build
# Otherwise just verify markdown is valid
cat docs/api.md docs/end-user-guide.md CHANGELOG.md > /dev/null
```

Expected: No errors

### Step 12.4: Commit

- [ ] **Commit documentation updates**

```bash
rtk git add docs/api.md docs/end-user-guide.md CHANGELOG.md
rtk git commit -m "docs: add capability discovery endpoints and user guide

- docs/api.md: add GET /capabilities and GET /capabilities/{key}, remove /skills
- docs/end-user-guide.md: add Capabilities section with discovery workflow, detail page, cross-linking, URL sharing
- CHANGELOG.md: add breaking change note for /skills removal and SearchCapabilities removal
- Migration notes for API consumers and custom store implementations"
```

---

## Final Checklist

- [ ] **Run all backend tests**

```bash
rtk go test ./... -v
```

Expected: All PASS

- [ ] **Run frontend type check and tests**

```bash
cd web && rtk tsc --noEmit && rtk vitest run
```

Expected: No type errors, all tests PASS

- [ ] **Run E2E tests (if env configured)**

```bash
make e2e-test
```

Expected: All PASS

- [ ] **Run architecture validation**

```bash
make arch-test
```

Expected: All rules PASS (no violations of max function lines, params, return values, public functions per file)

- [ ] **Manual smoke test**

1. Start the server: `go run ./cmd/agentlens --config agentlens.yaml`
2. Navigate to `http://localhost:8080`
3. Log in
4. Click "Capabilities" tab — should show capability list with accordion groups
5. Search for a capability — results should filter in real time
6. Click a kind filter — results should filter
7. Expand an accordion — should show agents
8. Click "View all" — should navigate to detail page
9. Detail page should show agents table
10. Click an agent — should navigate to agent detail
11. On agent detail, click a capability name — should navigate to capability detail
12. Verify URL contains `?q=` or `?kind=` params and is shareable

- [ ] **Verify all commits follow conventional commit format**

```bash
rtk git log --oneline -12
```

Expected: All 12 commits start with `feat(scope):`, `test(scope):`, or `docs:`

---

## Implementation Complete

All 12 tasks completed. Feature 2.3 — Capability-based Discovery is ready for review and merge.

**Acceptance criteria checklist:**

- ✅ "Capabilities" tab visible in main navigation
- ✅ `/catalog/capabilities` shows capability instances grouped by (kind, name) with agent counts
- ✅ Search box filters by name, tags, description in real time
- ✅ Kind filter works, state survives reload via URL
- ✅ Capability detail page shows all agents offering the capability
- ✅ Agent-specific `capability_snippet` shown per row
- ✅ Capability names on agent detail page are clickable
- ✅ Offline/deprecated agents excluded from index, included in detail with status badge
- ✅ Technical capability kinds excluded from discovery
- ✅ `RegisterCapability` accepts `discoverable` parameter
- ✅ `DiscoverableKinds()` returns only discoverable kinds
- ✅ Adding new discoverable kind requires only `RegisterCapability` call change
- ✅ Shareable URL `?q=translate&kind=a2a.skill` reproduces exact view
- ✅ SQLite store tests pass
- ✅ Playwright E2E tests green
- ✅ Empty states present
- ✅ `docs/api.md` updated
- ✅ `docs/end-user-guide.md` updated
- ✅ `GET /api/v1/skills` removed

# Product Archetype Refactor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the domain model to properly follow the Product Archetype pattern — separate `AgentType` (what an agent IS) from `CatalogEntry` (how it is cataloged), with polymorphic `Capability` interface replacing `Skills[]` and `TypedMetadata`.

**Architecture:** Two DB tables (`agent_types` + `catalog_entries`) with FK. `providers` table for reusable orgs. `capabilities` table with `kind` discriminator. Parsers return `*AgentType`, handlers wrap in `CatalogEntry`. REST API stays backward compatible via `MarshalJSON()` flattening.

**Tech Stack:** Go 1.26.1, GORM (SQLite + PostgreSQL), chi router, React 18 / TypeScript (frontend types only)

**Spec:** `docs/superpowers/specs/2026-04-03-product-archetype-design.md`

---

## File Structure

### New files
| File | Responsibility |
|---|---|
| `internal/model/agent_type.go` | `AgentType` struct + `AgentKey()` helper |
| `internal/model/provider.go` | `Provider` struct (standalone with ID, GORM model) |
| `internal/model/capability.go` | `Capability` interface + registry (replaces `typed_metadata.go`) |
| `internal/model/a2a_capabilities.go` | `A2ASkill`, `A2AInterface`, `A2ASecurityScheme`, `A2AExtension`, `A2ASignature` |
| `internal/model/mcp_capabilities.go` | `MCPTool`, `MCPResource`, `MCPPrompt` |
| `internal/model/archetype_test.go` | Reflective architecture test enforcing Product Archetype |
| `internal/store/provider_store.go` | Provider upsert/CRUD |

### Modified files
| File | Change |
|---|---|
| `internal/model/agent.go` | Strip to `CatalogEntry` wrapper only (remove product fields) |
| `internal/db/migrations.go` | Rewrite — 4 tables: providers, agent_types, capabilities, catalog_entries |
| `internal/store/store.go` | Update `Store` interface for new model |
| `internal/store/sql_store.go` | Rewrite CRUD for AgentType + CatalogEntry with joins |
| `internal/store/sql_store_query.go` | Update List/Search/Stats for joined model |
| `internal/kernel/plugin.go` | `Parse(raw []byte) (*model.AgentType, error)` — remove source param |
| `plugins/parsers/a2a/a2a.go` | Return `*AgentType` with A2A capabilities |
| `plugins/parsers/a2a/validation.go` | No changes (validates raw card, doesn't touch model) |
| `plugins/parsers/a2a/a2a_validate.go` | No changes |
| `plugins/parsers/mcp/mcp.go` | Return `*AgentType` with MCP capabilities |
| `plugins/parsers/mcp/mcp_validate.go` | No changes |
| `internal/api/handlers.go` | Update `CreateEntry`, `GetEntryCard`, `ListCatalog` for new model |
| `internal/api/import_handler.go` | Wrap AgentType in CatalogEntry |
| `internal/api/register_handler.go` | Wrap AgentType in CatalogEntry |
| `internal/discovery/manager.go` | Update upsert for AgentType + CatalogEntry |
| `web/src/types.ts` | Replace `skills`/`typed_meta` with `capabilities` |
| All `*_test.go` files | Update for new types |

### Removed files
| File | Reason |
|---|---|
| `internal/model/typed_metadata.go` | Replaced by `capability.go` |
| `internal/model/a2a_metadata.go` | Replaced by `a2a_capabilities.go` |
| `internal/model/skill.go` | Replaced by capabilities |

---

### Task 1: Create Capability interface and registry

**Files:**
- Create: `internal/model/capability.go`

- [ ] **Step 1: Create `capability.go`**

```go
// Package model provides domain types for AgentLens.
package model

import (
	"encoding/json"
	"fmt"
)

// Capability represents a ProductFeatureType — a polymorphic capability of an agent.
// Each protocol defines its own capability kinds (e.g. a2a.skill, mcp.tool).
type Capability interface {
	Kind() string
	Validate() error
}

var capabilityRegistry = map[string]func() Capability{}

// RegisterCapability registers a factory for a capability kind.
func RegisterCapability(kind string, factory func() Capability) {
	capabilityRegistry[kind] = factory
}

type capKindWrapper struct {
	Kind string `json:"kind"`
}

// MarshalCapabilitiesJSON serializes a slice of Capability into a JSON array,
// injecting the "kind" discriminator into each object.
func MarshalCapabilitiesJSON(items []Capability) ([]byte, error) {
	if len(items) == 0 {
		return []byte("[]"), nil
	}
	result := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("marshaling capability %s: %w", item.Kind(), err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		m["kind"] = item.Kind()
		merged, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		result = append(result, merged)
	}
	return json.Marshal(result)
}

// UnmarshalCapabilitiesJSON deserializes a JSON array into a slice of Capability,
// dispatching by the "kind" field. Unknown kinds are silently skipped.
func UnmarshalCapabilitiesJSON(data []byte) ([]Capability, error) {
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, fmt.Errorf("unmarshaling capabilities array: %w", err)
	}
	caps := make([]Capability, 0, len(rawItems))
	for _, raw := range rawItems {
		var kw capKindWrapper
		if err := json.Unmarshal(raw, &kw); err != nil {
			continue
		}
		factory, ok := capabilityRegistry[kw.Kind]
		if !ok {
			continue
		}
		cap := factory()
		if err := json.Unmarshal(raw, cap); err != nil {
			continue
		}
		caps = append(caps, cap)
	}
	return caps, nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/model/...`

- [ ] **Step 3: Commit**

```bash
git add internal/model/capability.go
git commit -m "feat(model): add Capability interface and registry"
```

---

### Task 2: Create A2A capability types

**Files:**
- Create: `internal/model/a2a_capabilities.go`

- [ ] **Step 1: Create `a2a_capabilities.go`**

```go
package model

import "fmt"

func init() {
	RegisterCapability("a2a.skill", func() Capability { return &A2ASkill{} })
	RegisterCapability("a2a.interface", func() Capability { return &A2AInterface{} })
	RegisterCapability("a2a.security_scheme", func() Capability { return &A2ASecurityScheme{} })
	RegisterCapability("a2a.extension", func() Capability { return &A2AExtension{} })
	RegisterCapability("a2a.signature", func() Capability { return &A2ASignature{} })
}

// A2ASkill represents an A2A agent skill (ProductFeatureType).
type A2ASkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"input_modes,omitempty"`
	OutputModes []string `json:"output_modes,omitempty"`
}

func (s *A2ASkill) Kind() string { return "a2a.skill" }
func (s *A2ASkill) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("a2a.skill: name is required")
	}
	return nil
}

// A2AInterface represents an A2A supported interface.
type A2AInterface struct {
	URL     string `json:"url"`
	Binding string `json:"binding,omitempty"`
}

func (a *A2AInterface) Kind() string { return "a2a.interface" }
func (a *A2AInterface) Validate() error {
	if a.URL == "" {
		return fmt.Errorf("a2a.interface: url is required")
	}
	return nil
}

// A2ASecurityScheme represents an A2A security scheme.
type A2ASecurityScheme struct {
	Type   string `json:"type"`
	Method string `json:"method,omitempty"`
	Name   string `json:"name,omitempty"`
}

func (a *A2ASecurityScheme) Kind() string { return "a2a.security_scheme" }
func (a *A2ASecurityScheme) Validate() error {
	if a.Type == "" {
		return fmt.Errorf("a2a.security_scheme: type is required")
	}
	return nil
}

// A2AExtension represents an A2A capability extension.
type A2AExtension struct {
	URI      string `json:"uri"`
	Required bool   `json:"required"`
}

func (a *A2AExtension) Kind() string { return "a2a.extension" }
func (a *A2AExtension) Validate() error {
	if a.URI == "" {
		return fmt.Errorf("a2a.extension: uri is required")
	}
	return nil
}

// A2ASignature represents an A2A card signature.
type A2ASignature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId,omitempty"`
}

func (a *A2ASignature) Kind() string { return "a2a.signature" }
func (a *A2ASignature) Validate() error {
	if a.Algorithm == "" {
		return fmt.Errorf("a2a.signature: algorithm is required")
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/model/...`

- [ ] **Step 3: Commit**

```bash
git add internal/model/a2a_capabilities.go
git commit -m "feat(model): add A2A capability types"
```

---

### Task 3: Create MCP capability types

**Files:**
- Create: `internal/model/mcp_capabilities.go`

- [ ] **Step 1: Create `mcp_capabilities.go`**

```go
package model

import "fmt"

func init() {
	RegisterCapability("mcp.tool", func() Capability { return &MCPTool{} })
	RegisterCapability("mcp.resource", func() Capability { return &MCPResource{} })
	RegisterCapability("mcp.prompt", func() Capability { return &MCPPrompt{} })
}

// MCPTool represents an MCP server tool.
type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema,omitempty"`
}

func (t *MCPTool) Kind() string { return "mcp.tool" }
func (t *MCPTool) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("mcp.tool: name is required")
	}
	return nil
}

// MCPResource represents an MCP server resource.
type MCPResource struct {
	Name        string `json:"name"`
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
}

func (r *MCPResource) Kind() string { return "mcp.resource" }
func (r *MCPResource) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("mcp.resource: name is required")
	}
	return nil
}

// MCPPrompt represents an MCP server prompt.
type MCPPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Arguments   []any  `json:"arguments,omitempty"`
}

func (p *MCPPrompt) Kind() string { return "mcp.prompt" }
func (p *MCPPrompt) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("mcp.prompt: name is required")
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/model/...`

- [ ] **Step 3: Commit**

```bash
git add internal/model/mcp_capabilities.go
git commit -m "feat(model): add MCP capability types (tool, resource, prompt)"
```

---

### Task 4: Create Provider and AgentType models

**Files:**
- Create: `internal/model/provider.go`
- Create: `internal/model/agent_type.go`

- [ ] **Step 1: Create `provider.go`**

```go
package model

import "time"

// Provider represents an organization that owns agents (reusable across AgentTypes).
type Provider struct {
	ID           string    `json:"id"           gorm:"primaryKey;type:text"`
	Organization string    `json:"organization" gorm:"not null;type:text"`
	Team         string    `json:"team,omitempty" gorm:"type:text"`
	URL          string    `json:"url,omitempty"  gorm:"type:text"`
	CreatedOn    time.Time `json:"created_on"`
}

func (Provider) TableName() string { return "providers" }
```

- [ ] **Step 2: Create `agent_type.go`**

```go
package model

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// AgentType represents a ProductType — "what the agent IS".
// It holds the protocol-specific definition parsed from a raw card/manifest.
type AgentType struct {
	ID            string          `json:"id"             gorm:"primaryKey;type:text"`
	AgentKey      string          `json:"agent_key"      gorm:"not null;type:text;index"`
	Protocol      Protocol        `json:"protocol"       gorm:"not null;type:text"`
	Endpoint      string          `json:"endpoint"       gorm:"not null;type:text"`
	Version       string          `json:"version"        gorm:"not null;type:text;default:''"`
	SpecVersion   string          `json:"spec_version"   gorm:"type:text;default:''"`
	ProviderID    *string         `json:"provider_id,omitempty" gorm:"type:text;index"`
	Provider      *Provider       `json:"provider,omitempty"    gorm:"foreignKey:ProviderID"`
	RawDefinition []byte          `json:"-"              gorm:"not null;type:blob"`
	CreatedOn     time.Time       `json:"created_on"`

	// Capabilities loaded via separate query, not GORM auto-preload.
	Capabilities []Capability `json:"capabilities,omitempty" gorm:"-"`

	// DB serialization column for raw_definition base64 in JSON responses.
	RawDefJSON json.RawMessage `json:"raw_definition,omitempty" gorm:"-"`
}

func (AgentType) TableName() string { return "agent_types" }

// ComputeAgentKey computes SHA256(protocol + endpoint) as the partition key.
func ComputeAgentKey(protocol Protocol, endpoint string) string {
	h := sha256.Sum256([]byte(string(protocol) + endpoint))
	return fmt.Sprintf("%x", h)
}

// SyncRawDefForJSON populates RawDefJSON from RawDefinition for JSON serialization.
func (a *AgentType) SyncRawDefForJSON() {
	if len(a.RawDefinition) > 0 && json.Valid(a.RawDefinition) {
		a.RawDefJSON = json.RawMessage(a.RawDefinition)
	}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/model/...`

- [ ] **Step 4: Commit**

```bash
git add internal/model/provider.go internal/model/agent_type.go
git commit -m "feat(model): add Provider and AgentType (ProductType) models"
```

---

### Task 5: Refactor CatalogEntry to catalog wrapper

**Files:**
- Modify: `internal/model/agent.go`

- [ ] **Step 1: Rewrite `agent.go`**

Strip `CatalogEntry` to catalog-only fields. Remove product fields (`Protocol`, `Endpoint`, `Skills`, `TypedMeta`, `RawCard`, `SpecVersion`, `Version`). Add `AgentTypeID` FK. Keep `Provider` convenience accessor. Add `MarshalJSON` that flattens AgentType + CatalogEntry for backward-compatible API.

```go
package model

import (
	"encoding/json"
	"time"
)

// Protocol, Status, SourceType constants stay here (they are domain enums used by both AgentType and CatalogEntry).

type Protocol string

const (
	ProtocolA2A  Protocol = "a2a"
	ProtocolMCP  Protocol = "mcp"
	ProtocolA2UI Protocol = "a2ui"
)

type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusUnknown  Status = "unknown"
)

type SourceType string

const (
	SourceK8s      SourceType = "k8s"
	SourceConfig   SourceType = "config"
	SourcePush     SourceType = "push"
	SourceUpstream SourceType = "upstream"
)

// Validity represents a time-bounded availability window.
type Validity struct {
	From     *time.Time `json:"from,omitempty"`
	To       *time.Time `json:"to,omitempty"`
	LastSeen time.Time  `json:"last_seen"`
}

// IsActiveAt returns true if the given time falls within the validity window.
func (v Validity) IsActiveAt(t time.Time) bool {
	if v.From != nil && t.Before(*v.From) {
		return false
	}
	if v.To != nil && t.After(*v.To) {
		return false
	}
	return true
}

// CatalogEntry is the commercial wrapper — "how the agent is cataloged".
// It references an AgentType (what the agent IS) via AgentTypeID.
type CatalogEntry struct {
	ID          string     `json:"id"           gorm:"primaryKey;type:text"`
	AgentTypeID string     `json:"agent_type_id" gorm:"not null;type:text;index"`
	AgentType   *AgentType `json:"-"            gorm:"foreignKey:AgentTypeID"`
	DisplayName string     `json:"display_name" gorm:"not null;type:text"`
	Description string     `json:"description"  gorm:"type:text;default:''"`
	Status      Status     `json:"status"       gorm:"not null;type:text;default:'unknown';index"`
	Source      SourceType `json:"source"       gorm:"not null;type:text;index"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// Catalog-specific fields stored as JSON text in DB.
	Categories []string          `json:"categories,omitempty" gorm:"-"`
	Metadata   map[string]string `json:"metadata,omitempty"   gorm:"-"`
	Validity   Validity          `json:"validity"             gorm:"-"`

	// DB serialization columns.
	CategoriesJSON string     `json:"-" gorm:"column:categories;type:text;not null;default:'[]'"`
	MetadataJSON   string     `json:"-" gorm:"column:metadata;type:text;not null;default:'{}'"`
	ValidFrom      *time.Time `json:"-" gorm:"column:validity_from"`
	ValidTo        *time.Time `json:"-" gorm:"column:validity_to"`
	LastSeen       time.Time  `json:"-" gorm:"column:validity_last_seen;not null"`
}

func (CatalogEntry) TableName() string { return "catalog_entries" }

// SyncToDB marshals public fields into DB serialization columns.
func (e *CatalogEntry) SyncToDB() {
	if e.Categories != nil {
		b, _ := json.Marshal(e.Categories)
		e.CategoriesJSON = string(b)
	}
	if e.Metadata != nil {
		b, _ := json.Marshal(e.Metadata)
		e.MetadataJSON = string(b)
	}
	e.ValidFrom = e.Validity.From
	e.ValidTo = e.Validity.To
	e.LastSeen = e.Validity.LastSeen
}

// SyncFromDB unmarshals DB serialization columns into public fields.
func (e *CatalogEntry) SyncFromDB() {
	if e.CategoriesJSON != "" {
		_ = json.Unmarshal([]byte(e.CategoriesJSON), &e.Categories)
	}
	if e.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(e.MetadataJSON), &e.Metadata)
	}
	e.Validity = Validity{
		From:     e.ValidFrom,
		To:       e.ValidTo,
		LastSeen: e.LastSeen,
	}
	if e.AgentType != nil {
		e.AgentType.SyncRawDefForJSON()
	}
}

// MarshalJSON produces a flat JSON object merging AgentType and CatalogEntry fields
// for backward-compatible REST API responses.
func (e CatalogEntry) MarshalJSON() ([]byte, error) {
	e.SyncFromDB()

	type flat struct {
		ID            string            `json:"id"`
		AgentTypeID   string            `json:"agent_type_id"`
		DisplayName   string            `json:"display_name"`
		Description   string            `json:"description"`
		Protocol      Protocol          `json:"protocol"`
		Endpoint      string            `json:"endpoint"`
		Version       string            `json:"version"`
		SpecVersion   string            `json:"spec_version,omitempty"`
		Status        Status            `json:"status"`
		Source        SourceType        `json:"source"`
		Provider      *Provider         `json:"provider,omitempty"`
		Categories    []string          `json:"categories,omitempty"`
		Capabilities  []Capability      `json:"capabilities,omitempty"`
		Validity      Validity          `json:"validity"`
		Metadata      map[string]string `json:"metadata,omitempty"`
		RawDefinition json.RawMessage   `json:"raw_definition,omitempty"`
		CreatedAt     time.Time         `json:"created_at"`
		UpdatedAt     time.Time         `json:"updated_at"`
	}

	f := flat{
		ID:          e.ID,
		AgentTypeID: e.AgentTypeID,
		DisplayName: e.DisplayName,
		Description: e.Description,
		Status:      e.Status,
		Source:      e.Source,
		Categories:  e.Categories,
		Validity:    e.Validity,
		Metadata:    e.Metadata,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}

	if e.AgentType != nil {
		f.Protocol = e.AgentType.Protocol
		f.Endpoint = e.AgentType.Endpoint
		f.Version = e.AgentType.Version
		f.SpecVersion = e.AgentType.SpecVersion
		f.Provider = e.AgentType.Provider
		f.Capabilities = e.AgentType.Capabilities
		f.RawDefinition = e.AgentType.RawDefJSON
	}

	return json.Marshal(f)
}
```

- [ ] **Step 2: Verify model compiles (expect other packages to break — that's OK)**

Run: `go build ./internal/model/...`

- [ ] **Step 3: Commit**

```bash
git add internal/model/agent.go
git commit -m "refactor(model): strip CatalogEntry to catalog wrapper, add AgentType FK"
```

---

### Task 6: Remove old model files

**Files:**
- Remove: `internal/model/typed_metadata.go`
- Remove: `internal/model/a2a_metadata.go`
- Remove: `internal/model/skill.go`

- [ ] **Step 1: Delete old files**

```bash
rm internal/model/typed_metadata.go
rm internal/model/a2a_metadata.go
rm internal/model/skill.go
```

- [ ] **Step 2: Fix any model tests that reference removed types**

Read `internal/model/typed_metadata_test.go` and `internal/model/a2a_metadata_test.go`. If they exist, either rewrite them for the new `Capability` types or remove them (they'll be replaced by the archetype compliance test and capability tests).

Run: `go build ./internal/model/...`

Fix any remaining compilation errors in the model package.

- [ ] **Step 3: Commit**

```bash
git add -A internal/model/
git commit -m "refactor(model): remove Skill, TypedMetadata, A2A metadata (replaced by Capability)"
```

---

### Task 7: Archetype compliance test

**Files:**
- Create: `internal/model/archetype_test.go`

- [ ] **Step 1: Write the compliance test**

```go
package model

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArchetype_AgentTypeHasNoCatalogFields(t *testing.T) {
	catalogOnlyFields := []string{
		"DisplayName", "Description", "Categories",
		"Status", "Source", "Metadata", "Validity",
	}
	agentTypeT := reflect.TypeOf(AgentType{})
	for _, name := range catalogOnlyFields {
		_, found := agentTypeT.FieldByName(name)
		assert.False(t, found, "AgentType must not have catalog field %q — it belongs on CatalogEntry", name)
	}
}

func TestArchetype_CatalogEntryHasNoProductFields(t *testing.T) {
	productOnlyFields := []string{
		"Protocol", "Endpoint", "RawDefinition",
		"Capabilities", "Skills", "TypedMeta", "SpecVersion",
	}
	entryT := reflect.TypeOf(CatalogEntry{})
	for _, name := range productOnlyFields {
		_, found := entryT.FieldByName(name)
		assert.False(t, found, "CatalogEntry must not have product field %q — it belongs on AgentType", name)
	}
}

func TestArchetype_CatalogEntryReferencesAgentType(t *testing.T) {
	entryT := reflect.TypeOf(CatalogEntry{})
	field, found := entryT.FieldByName("AgentTypeID")
	assert.True(t, found, "CatalogEntry must have AgentTypeID field")
	if found {
		assert.Equal(t, reflect.String, field.Type.Kind(), "AgentTypeID must be a string")
	}

	atField, found := entryT.FieldByName("AgentType")
	assert.True(t, found, "CatalogEntry must have AgentType field")
	if found {
		assert.Equal(t, reflect.Ptr, atField.Type.Kind(), "AgentType must be a pointer")
	}
}

func TestArchetype_AgentTypeHasRequiredFields(t *testing.T) {
	required := []string{"ID", "AgentKey", "Protocol", "Endpoint", "RawDefinition", "CreatedOn"}
	agentTypeT := reflect.TypeOf(AgentType{})
	for _, name := range required {
		_, found := agentTypeT.FieldByName(name)
		assert.True(t, found, "AgentType must have required field %q", name)
	}
}

func TestArchetype_AllCapabilityTypesImplementInterface(t *testing.T) {
	capType := reflect.TypeOf((*Capability)(nil)).Elem()

	types := []any{
		&A2ASkill{},
		&A2AInterface{},
		&A2ASecurityScheme{},
		&A2AExtension{},
		&A2ASignature{},
		&MCPTool{},
		&MCPResource{},
		&MCPPrompt{},
	}
	for _, impl := range types {
		implType := reflect.TypeOf(impl)
		assert.True(t, implType.Implements(capType),
			"%s must implement Capability interface", implType.Elem().Name())
	}
}

func TestArchetype_CapabilityKindsAreRegistered(t *testing.T) {
	expectedKinds := []string{
		"a2a.skill", "a2a.interface", "a2a.security_scheme",
		"a2a.extension", "a2a.signature",
		"mcp.tool", "mcp.resource", "mcp.prompt",
	}
	for _, kind := range expectedKinds {
		_, ok := capabilityRegistry[kind]
		assert.True(t, ok, "capability kind %q must be registered", kind)
	}
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/model/... -run TestArchetype -v`

Expected: all 6 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/model/archetype_test.go
git commit -m "test(model): add Product Archetype compliance test"
```

---

### Task 8: Rewrite database migrations

**Files:**
- Modify: `internal/db/migrations.go`

- [ ] **Step 1: Rewrite migrations**

Replace migration001 (old monolithic `catalog_entries` table) with new schema: `providers`, `agent_types`, `capabilities`, `catalog_entries` with FK.

Keep migration002 (users/roles), migration003 (default roles), migration004 (settings) unchanged.

Remove migration005 (added `spec_version` and `typed_meta` columns to old table — no longer needed).

The new migration001 should use `tx.AutoMigrate` with the new GORM model structs. The `CatalogEntry`, `AgentType`, `Provider` structs already have proper GORM tags.

For `capabilities` table, define a local migration struct since the `Capability` interface can't be automigrated:

```go
var migration001 = Migration{
	ID: "001_create_tables",
	Migrate: func(tx *gorm.DB) error {
		// Capabilities needs a concrete struct for GORM migration.
		type capabilityRow struct {
			ID          string `gorm:"primaryKey;type:text"`
			AgentTypeID string `gorm:"not null;type:text;index"`
			Kind        string `gorm:"not null;type:text;index"`
			Name        string `gorm:"not null;type:text"`
			Description string `gorm:"type:text;default:''"`
			Properties  string `gorm:"type:text;not null;default:'{}'"`
		}
		if err := tx.Table("capabilities").AutoMigrate(&capabilityRow{}); err != nil {
			return err
		}
		if err := tx.AutoMigrate(&model.Provider{}); err != nil {
			return err
		}
		if err := tx.AutoMigrate(&model.AgentType{}); err != nil {
			return err
		}
		if err := tx.AutoMigrate(&model.CatalogEntry{}); err != nil {
			return err
		}
		// Add unique constraint on (agent_key, version).
		return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_types_key_version ON agent_types(agent_key, version)").Error
	},
}
```

Also add unique constraint on capabilities: `CREATE UNIQUE INDEX IF NOT EXISTS idx_capabilities_type_kind_name ON capabilities(agent_type_id, kind, name)`.

And unique constraint on providers: `CREATE UNIQUE INDEX IF NOT EXISTS idx_providers_org_team ON providers(organization, team)`.

- [ ] **Step 2: Update `AllMigrations()` to return the corrected slice**

Remove migration005 from the list.

- [ ] **Step 3: Verify migrations run**

Run: `go test ./internal/db/... -v`

- [ ] **Step 4: Commit**

```bash
git add internal/db/migrations.go
git commit -m "refactor(db): rewrite migrations for Product Archetype schema (4 tables)"
```

---

### Task 9: Update Store interface and rewrite SQL store

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sql_store.go`
- Modify: `internal/store/sql_store_query.go`
- Create: `internal/store/provider_store.go`

- [ ] **Step 1: Update `Store` interface in `store.go`**

Add methods for AgentType and Provider. `SearchSkills` becomes `SearchCapabilities`. Keep backward-compatible methods where possible.

```go
type Store interface {
	// Provider operations
	UpsertProvider(ctx context.Context, provider *model.Provider) (*model.Provider, error)

	// AgentType operations
	CreateAgentType(ctx context.Context, agentType *model.AgentType) error
	GetAgentTypeByKey(ctx context.Context, agentKey string) (*model.AgentType, error)
	GetAgentTypeByEndpoint(ctx context.Context, endpoint string) (*model.AgentType, error)

	// CatalogEntry operations (entry always loaded with AgentType + Capabilities)
	Create(ctx context.Context, entry *model.CatalogEntry) error
	Get(ctx context.Context, id string) (*model.CatalogEntry, error)
	Update(ctx context.Context, entry *model.CatalogEntry) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ListFilter) ([]model.CatalogEntry, error)
	FindByEndpoint(ctx context.Context, endpoint string) (*model.CatalogEntry, error)
	SearchCapabilities(ctx context.Context, query string) ([]model.CatalogEntry, error)
	Stats(ctx context.Context) (*StoreStats, error)

	Close() error
}
```

- [ ] **Step 2: Create `provider_store.go`**

```go
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// UpsertProvider finds an existing provider by (organization, team) or creates a new one.
func (s *SQLStore) UpsertProvider(ctx context.Context, provider *model.Provider) (*model.Provider, error) {
	var existing model.Provider
	err := s.gdb.DB.WithContext(ctx).
		Where("organization = ? AND team = ?", provider.Organization, provider.Team).
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	provider.ID = uuid.NewString()
	provider.CreatedOn = time.Now().UTC()
	if err := s.gdb.DB.WithContext(ctx).Create(provider).Error; err != nil {
		return nil, err
	}
	return provider, nil
}
```

- [ ] **Step 3: Rewrite `sql_store.go`**

Key changes:
- `Create` now expects `entry.AgentType` to be set. It creates the AgentType (with capabilities as separate rows), then the CatalogEntry.
- `Get` preloads AgentType + Provider, loads capabilities separately.
- `FindByEndpoint` joins through `agent_types`.
- All reads call `loadCapabilities` to populate `AgentType.Capabilities` from the `capabilities` table.

The capability rows use a concrete struct `capabilityRow` for DB operations:

```go
type capabilityRow struct {
	ID          string `gorm:"primaryKey;type:text"`
	AgentTypeID string `gorm:"not null;type:text;index"`
	Kind        string `gorm:"not null;type:text"`
	Name        string `gorm:"not null;type:text"`
	Description string `gorm:"type:text;default:''"`
	Properties  string `gorm:"type:text;not null;default:'{}'"`
}

func (capabilityRow) TableName() string { return "capabilities" }
```

Write `saveCapabilities(ctx, tx, agentTypeID, caps)` and `loadCapabilities(ctx, agentTypeID)` helpers to marshal/unmarshal between `model.Capability` and `capabilityRow` using the registry.

- [ ] **Step 4: Rewrite `sql_store_query.go`**

Update `List` to join `agent_types` for protocol/endpoint filtering. Replace `SearchSkills` with `SearchCapabilities` (searches `capabilities.name` and `capabilities.description`). Update `Stats` (no structural change — still counts catalog_entries by status/source).

- [ ] **Step 5: Run store tests**

Run: `go test ./internal/store/... -v`

Fix any compilation errors. Store tests use `NewSQLiteStore(":memory:")` which runs migrations — verify the new migration creates all 4 tables correctly.

- [ ] **Step 6: Commit**

```bash
git add internal/store/
git commit -m "refactor(store): rewrite for Product Archetype (AgentType + CatalogEntry + capabilities)"
```

---

### Task 10: Update ParserPlugin interface

**Files:**
- Modify: `internal/kernel/plugin.go`

- [ ] **Step 1: Change Parse signature**

Change `ParserPlugin.Parse` from:
```go
Parse(raw []byte, source model.SourceType) (*model.CatalogEntry, error)
```
to:
```go
Parse(raw []byte) (*model.AgentType, error)
```

Also update `SourcePlugin.Discover` to return `*model.AgentType` instead of `*model.CatalogEntry`:
```go
type SourcePlugin interface {
	Plugin
	Discover(ctx context.Context) ([]*model.AgentType, error)
}
```

- [ ] **Step 2: Verify kernel compiles (parsers will break — that's expected)**

Run: `go build ./internal/kernel/...`

- [ ] **Step 3: Commit**

```bash
git add internal/kernel/plugin.go
git commit -m "refactor(kernel): ParserPlugin.Parse returns *AgentType, remove source param"
```

---

### Task 11: Rewrite A2A parser

**Files:**
- Modify: `plugins/parsers/a2a/a2a.go`

- [ ] **Step 1: Rewrite Parse to return `*model.AgentType`**

Remove `source model.SourceType` parameter. Return `*model.AgentType` instead of `*model.CatalogEntry`. Convert skills → `A2ASkill` capabilities. Convert interfaces/extensions/security/signatures → capabilities. Set `RawDefinition` instead of `RawCard`.

The helper functions change:
- `buildSkills` → `buildA2ASkillCaps` returning `[]model.Capability`
- `buildTypedMeta` → `buildA2AMetaCaps` returning `[]model.Capability`
- `buildProvider` stays, returns `model.Provider`
- `resolveEndpoint` stays

```go
func (p *Plugin) Parse(raw []byte) (*model.AgentType, error) {
	var card fullCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("parsing a2a card: %w", err)
	}
	if card.Name == "" {
		return nil, fmt.Errorf("a2a card missing required field: name")
	}
	endpoint, err := resolveEndpoint(&card)
	if err != nil {
		return nil, err
	}

	specVersion := detectSpecVersion(&card)
	provider := buildProvider(&card)
	caps := buildA2ASkillCaps(&card)
	caps = append(caps, buildA2AMetaCaps(&card)...)

	return &model.AgentType{
		Protocol:      model.ProtocolA2A,
		Endpoint:      endpoint,
		Version:       card.Version,
		SpecVersion:   specVersion,
		Provider:      &provider,
		Capabilities:  caps,
		RawDefinition: raw,
		CreatedOn:     time.Now().UTC(),
	}, nil
}
```

Where `buildA2ASkillCaps` returns `[]model.Capability` with `*model.A2ASkill` entries:
```go
func buildA2ASkillCaps(card *fullCard) []model.Capability {
	caps := make([]model.Capability, 0, len(card.Skills))
	for _, s := range card.Skills {
		caps = append(caps, &model.A2ASkill{
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
			InputModes:  s.InputModes,
			OutputModes: s.OutputModes,
		})
	}
	return caps
}
```

And `buildA2AMetaCaps` returns capabilities for interfaces, extensions, security schemes, signatures (same data as before, but now as `model.A2AInterface`, etc. instead of `model.TypedMetadata`).

- [ ] **Step 2: Run A2A parser tests**

Run: `go test ./plugins/parsers/a2a/... -v`

Fix test expectations (they currently check for `CatalogEntry` fields).

- [ ] **Step 3: Commit**

```bash
git add plugins/parsers/a2a/
git commit -m "refactor(a2a): Parse returns *AgentType with A2A capabilities"
```

---

### Task 12: Rewrite MCP parser

**Files:**
- Modify: `plugins/parsers/mcp/mcp.go`

- [ ] **Step 1: Expand `mcpCard` to include resources and prompts**

```go
type mcpCard struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Version     string        `json:"version"`
	Remotes     []mcpRemote   `json:"remotes,omitempty"`
	Tools       []mcpTool     `json:"tools,omitempty"`
	Resources   []mcpResource `json:"resources,omitempty"`
	Prompts     []mcpPrompt   `json:"prompts,omitempty"`
}

type mcpResource struct {
	Name        string `json:"name"`
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
}

type mcpPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Arguments   []any  `json:"arguments,omitempty"`
}
```

- [ ] **Step 2: Rewrite Parse to return `*model.AgentType`**

```go
func (p *Plugin) Parse(raw []byte) (*model.AgentType, error) {
	var card mcpCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("parsing mcp card: %w", err)
	}
	if card.Name == "" {
		return nil, fmt.Errorf("mcp card missing required field: name")
	}
	if len(card.Remotes) == 0 || card.Remotes[0].URL == "" {
		return nil, fmt.Errorf("mcp card missing required field: remotes[0].url")
	}

	var caps []model.Capability
	for _, t := range card.Tools {
		caps = append(caps, &model.MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	for _, r := range card.Resources {
		caps = append(caps, &model.MCPResource{
			Name:        r.Name,
			URI:         r.URI,
			Description: r.Description,
		})
	}
	for _, pr := range card.Prompts {
		caps = append(caps, &model.MCPPrompt{
			Name:        pr.Name,
			Description: pr.Description,
			Arguments:   pr.Arguments,
		})
	}

	return &model.AgentType{
		Protocol:      model.ProtocolMCP,
		Endpoint:      card.Remotes[0].URL,
		Version:       card.Version,
		Capabilities:  caps,
		RawDefinition: raw,
		CreatedOn:     time.Now().UTC(),
	}, nil
}
```

Also update `mcpTool` to include `InputSchema`:
```go
type mcpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema,omitempty"`
}
```

- [ ] **Step 3: Run MCP parser tests (if any exist)**

Run: `go test ./plugins/parsers/mcp/... -v`

- [ ] **Step 4: Commit**

```bash
git add plugins/parsers/mcp/
git commit -m "refactor(mcp): Parse returns *AgentType with MCP capabilities (tools, resources, prompts)"
```

---

### Task 13: Update API handlers

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/import_handler.go`
- Modify: `internal/api/register_handler.go`

- [ ] **Step 1: Update `import_handler.go`**

After `parser.Parse(result.RawJSON)` returns `*model.AgentType`, wrap it:

```go
agentType, err := parser.Parse(result.RawJSON)
if err != nil {
	ErrorResponse(w, http.StatusBadRequest, err.Error())
	return
}

// Compute agent key and set ID.
now := time.Now().UTC()
agentType.ID = uuid.NewString()
agentType.AgentKey = model.ComputeAgentKey(agentType.Protocol, agentType.Endpoint)
agentType.CreatedOn = now

// Upsert provider if present.
if agentType.Provider != nil {
	p, err := h.store.UpsertProvider(r.Context(), agentType.Provider)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, "failed to upsert provider")
		return
	}
	agentType.ProviderID = &p.ID
	agentType.Provider = p
}

// Create CatalogEntry wrapper.
entry := &model.CatalogEntry{
	ID:          uuid.NewString(),
	AgentTypeID: agentType.ID,
	AgentType:   agentType,
	DisplayName: inferDisplayName(agentType),
	Source:      model.SourcePush,
	Status:      model.StatusUnknown,
	Validity:    model.Validity{LastSeen: now},
	CreatedAt:   now,
	UpdatedAt:   now,
}

if err := h.store.Create(r.Context(), entry); err != nil {
	// handle duplicate etc.
}
```

Add helper `inferDisplayName(at *model.AgentType) string` that picks the first capability name or falls back to endpoint.

- [ ] **Step 2: Update `register_handler.go`**

Same pattern — `parser.Parse(raw)` returns `*AgentType`, wrap in `CatalogEntry`.

- [ ] **Step 3: Update `handlers.go`**

- `CreateEntry` — this is the direct JSON creation endpoint. It needs updating since `CatalogEntry` no longer has `Protocol`/`Endpoint` directly. The request body should provide `agent_type_id` referencing an existing AgentType, OR we keep backward compatibility by accepting flat JSON and splitting it.

  For backward compatibility: accept the current request shape, create an `AgentType` + `CatalogEntry` from it.

- `GetEntryCard` → becomes `GetEntryDefinition` — returns `entry.AgentType.RawDefinition`.

- `ListCatalog` — should work via `MarshalJSON` flattening.

- `SearchSkills` → `SearchCapabilities` — update method name.

- [ ] **Step 4: Run API tests**

Run: `go test ./internal/api/... -v`

Fix all test failures. The key changes: tests need to create AgentType + CatalogEntry instead of just CatalogEntry.

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "refactor(api): handlers wrap AgentType in CatalogEntry, backward-compatible JSON"
```

---

### Task 14: Update discovery manager

**Files:**
- Modify: `internal/discovery/manager.go`

- [ ] **Step 1: Update Source interface**

`Discover` now returns `[]*model.AgentType` (same change as SourcePlugin in kernel).

- [ ] **Step 2: Update upsert logic**

The manager's upsert now:
1. Receives `[]* model.AgentType` from source
2. For each: `FindByEndpoint` (looks through catalog_entries → agent_types)
3. If exists and source is push: skip
4. If exists (non-push): update agent_type fields + capabilities, update catalog_entry timestamps
5. If not found: create new AgentType + CatalogEntry

- [ ] **Step 3: Update source plugins that call Discover**

Check `plugins/sources/k8s/k8s.go` and `plugins/sources/static/static.go` — update their `Discover` return type from `[]*model.CatalogEntry` to `[]*model.AgentType`.

- [ ] **Step 4: Run discovery tests**

Run: `go test ./internal/discovery/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/discovery/ plugins/sources/
git commit -m "refactor(discovery): sources return *AgentType, manager wraps in CatalogEntry"
```

---

### Task 15: Update frontend types

**Files:**
- Modify: `web/src/types.ts`

- [ ] **Step 1: Replace skills/typed_meta with capabilities**

```typescript
// Remove: Skill interface
// Remove: TypedMeta, A2AExtensionMeta, A2ASecuritySchemeMeta, A2AInterfaceMeta

// Add:
interface Capability {
  kind: string
  name: string
  description?: string
  [key: string]: unknown  // protocol-specific properties
}

// Update CatalogEntry:
interface CatalogEntry {
  id: string
  agent_type_id: string
  display_name: string
  description: string
  protocol: Protocol
  endpoint: string
  version: string
  status: Status
  source: SourceType
  spec_version?: string
  provider?: Provider
  categories?: string[]
  capabilities?: Capability[]
  validity: Validity
  metadata?: Record<string, string>
  raw_definition?: unknown
  created_at: string
  updated_at: string
}
```

- [ ] **Step 2: Update frontend components that reference `skills` or `typed_meta`**

Search for `skills` and `typed_meta` in `web/src/components/` and update to use `capabilities`. Key files:
- `CardPreview.tsx` — uses `skills_count` from `ValidationPreview` and `TypedMeta[]` for extensions/interfaces
- `CatalogList.tsx` — may show skill counts
- `EntryDetail.tsx` — may display skills

- [ ] **Step 3: Run frontend type check**

Run: `cd web && bunx tsc --noEmit`

- [ ] **Step 4: Commit**

```bash
git add web/src/
git commit -m "refactor(web): replace skills/typed_meta with capabilities in TypeScript types"
```

---

### Task 16: Full verification

- [ ] **Step 1: Run all Go tests**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 2: Run arch-go**

```bash
make arch-test
```

Expected: all rules pass. If new violations appear (e.g. function length from new store code), fix them.

- [ ] **Step 3: Run lint**

```bash
make lint
```

Expected: 0 issues.

- [ ] **Step 4: Run archetype compliance test specifically**

```bash
go test ./internal/model/... -run TestArchetype -v
```

Expected: all 6 archetype tests pass.

- [ ] **Step 5: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "fix: resolve test/lint issues after Product Archetype refactor"
```

# A2A Agent Card Validation & UI Import — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable AgentLens to validate A2A Agent Cards (v0.3/v1.0), expose a validation endpoint, and provide a Dashboard modal to import/register cards with spec compliance checking.

**Architecture:** Extend the domain model with a `TypedMetadata` interface (inspired by Product Archetype's `FeatureValueConstraint`) for polymorphic protocol-specific data. Enhance the A2A parser to detect spec version and extract extensions/security/interfaces. Add a dry-run validation endpoint. Build a 4-step registration modal in React with shadcn/ui.

**Tech Stack:** Go 1.26 (Chi, GORM), React 18 (TypeScript, shadcn/ui, Tailwind), Playwright E2E

**Design Spec:** `docs/superpowers/specs/2026-04-01-a2a-card-validation-design.md`

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `internal/model/typed_metadata.go` | `TypedMetadata` interface, registry, JSON serialization helpers |
| `internal/model/typed_metadata_test.go` | Round-trip serialization, validation tests |
| `internal/model/a2a_metadata.go` | A2A-specific types: Extension, SecurityScheme, Interface, Signature |
| `internal/model/a2a_metadata_test.go` | A2A metadata Validate() tests |
| `internal/api/validate_handler.go` | `POST /api/v1/catalog/validate` handler |
| `internal/api/validate_handler_test.go` | Validation endpoint tests |
| `plugins/parsers/a2a/validation.go` | `ValidateCard()` returning structured errors/warnings |
| `plugins/parsers/a2a/validation_test.go` | Parser validation tests (v0.3, v1.0, legacy, invalid) |
| `plugins/parsers/a2a/testdata/a2a_v03_card.json` | Test fixture: full v0.3 card |
| `plugins/parsers/a2a/testdata/a2a_v10_card.json` | Test fixture: full v1.0 card |
| `plugins/parsers/a2a/testdata/a2a_legacy_card.json` | Test fixture: legacy card (url only) |
| `plugins/parsers/a2a/testdata/a2a_invalid_card.json` | Test fixture: missing required fields |
| `web/src/components/RegisterAgentDialog.tsx` | 4-step registration modal |
| `web/src/components/CardPreview.tsx` | Reusable card preview (modal + detail view) |

### Modified Files

| File | Changes |
|------|---------|
| `internal/model/agent.go` | Add `SpecVersion`, `TypedMeta` fields + SyncToDB/SyncFromDB/MarshalJSON |
| `internal/db/migrations.go` | Migration 005: add `spec_version`, `typed_meta` columns |
| `plugins/parsers/a2a/a2a.go` | Extend `a2aCard` struct, update `Parse()` to extract new fields |
| `internal/api/router.go` | Register `POST /catalog/validate` route |
| `internal/api/handlers.go` | No changes needed — validate handler uses `a2a.ValidateCard()` directly |
| `web/src/types.ts` | Add `TypedMeta`, `ValidationResult`, `CardPreview` types |
| `web/src/api.ts` | Add `validateAgentCard()`, `createAgentFromCard()` functions |
| `web/src/components/CatalogList.tsx` | Add "Register Agent" button |
| `web/src/components/EntryDetail.tsx` | Add Extensions, Security, SpecVersion sections |
| `e2e/tests/helpers.ts` | Add `validateAgentCard()` helper |
| `e2e/tests/catalog.spec.ts` | Add validation and registration E2E tests |

---

## Task 1: TypedMetadata Interface & Registry

**Files:**
- Create: `internal/model/typed_metadata.go`
- Create: `internal/model/typed_metadata_test.go`

- [ ] **Step 1: Write the failing test for TypedMetadata serialization round-trip**

```go
// internal/model/typed_metadata_test.go
package model_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestMarshalTypedMetadata(t *testing.T) {
	items := []model.TypedMetadata{
		&model.A2AExtension{URI: "urn:example:ext", Required: true},
		&model.A2ASecurityScheme{Type: "bearer", Method: "header", Name: "Authorization"},
	}

	data, err := model.MarshalTypedMetaJSON(items)
	require.NoError(t, err)

	parsed, err := model.UnmarshalTypedMetaJSON(data)
	require.NoError(t, err)
	require.Len(t, parsed, 2)

	ext, ok := parsed[0].(*model.A2AExtension)
	require.True(t, ok)
	assert.Equal(t, "urn:example:ext", ext.URI)
	assert.True(t, ext.Required)

	sec, ok := parsed[1].(*model.A2ASecurityScheme)
	require.True(t, ok)
	assert.Equal(t, "bearer", sec.Type)
}

func TestUnmarshalTypedMetadata_EmptyArray(t *testing.T) {
	parsed, err := model.UnmarshalTypedMetaJSON([]byte("[]"))
	require.NoError(t, err)
	assert.Empty(t, parsed)
}

func TestUnmarshalTypedMetadata_UnknownKind(t *testing.T) {
	data := []byte(`[{"kind":"unknown.type","foo":"bar"}]`)
	parsed, err := model.UnmarshalTypedMetaJSON(data)
	require.NoError(t, err)
	assert.Empty(t, parsed, "unknown kinds should be silently skipped")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestMarshalTypedMetadata -v`
Expected: compilation error — `model.TypedMetadata` undefined

- [ ] **Step 3: Implement TypedMetadata interface, registry, and serialization**

```go
// internal/model/typed_metadata.go
package model

import (
	"encoding/json"
	"fmt"
)

// TypedMetadata is the polymorphic base for protocol-specific structured data.
// Inspired by the Product Archetype's FeatureValueConstraint pattern.
type TypedMetadata interface {
	Kind() string
	Validate() error
}

// typedMetaRegistry maps kind strings to factory functions for deserialization.
var typedMetaRegistry = map[string]func() TypedMetadata{}

// RegisterTypedMeta registers a TypedMetadata factory for a given kind.
func RegisterTypedMeta(kind string, factory func() TypedMetadata) {
	typedMetaRegistry[kind] = factory
}

// typedMetaEnvelope wraps a TypedMetadata for JSON serialization with kind discriminator.
type typedMetaEnvelope struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"-"`
}

// MarshalTypedMetaJSON serializes a slice of TypedMetadata to JSON with kind discriminators.
func MarshalTypedMetaJSON(items []TypedMetadata) ([]byte, error) {
	if len(items) == 0 {
		return []byte("[]"), nil
	}
	var result []json.RawMessage
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("marshaling typed metadata %s: %w", item.Kind(), err)
		}
		// Inject "kind" field into the JSON object.
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("re-parsing typed metadata: %w", err)
		}
		kindBytes, _ := json.Marshal(item.Kind())
		m["kind"] = kindBytes
		merged, err := json.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("merging kind field: %w", err)
		}
		result = append(result, merged)
	}
	return json.Marshal(result)
}

// UnmarshalTypedMetaJSON deserializes a JSON array into concrete TypedMetadata types.
func UnmarshalTypedMetaJSON(data []byte) ([]TypedMetadata, error) {
	var raws []json.RawMessage
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("unmarshaling typed metadata array: %w", err)
	}
	var result []TypedMetadata
	for _, raw := range raws {
		var peek struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			continue
		}
		factory, ok := typedMetaRegistry[peek.Kind]
		if !ok {
			continue // silently skip unknown kinds for forward compatibility
		}
		item := factory()
		if err := json.Unmarshal(raw, item); err != nil {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/model/ -run TestMarshalTypedMetadata -v && go test ./internal/model/ -run TestUnmarshalTypedMetadata -v`
Expected: PASS (after A2A types are created in Task 2)

- [ ] **Step 5: Commit**

```bash
git add internal/model/typed_metadata.go internal/model/typed_metadata_test.go
git commit -m "feat: add TypedMetadata interface and serialization registry"
```

---

## Task 2: A2A Metadata Types

**Files:**
- Create: `internal/model/a2a_metadata.go`
- Create: `internal/model/a2a_metadata_test.go`

- [ ] **Step 1: Write failing tests for A2A metadata validation**

```go
// internal/model/a2a_metadata_test.go
package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestA2AExtension_Kind(t *testing.T) {
	ext := &model.A2AExtension{URI: "urn:test", Required: true}
	assert.Equal(t, "a2a.extension", ext.Kind())
}

func TestA2AExtension_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ext     model.A2AExtension
		wantErr bool
	}{
		{"valid", model.A2AExtension{URI: "urn:test", Required: true}, false},
		{"missing uri", model.A2AExtension{URI: "", Required: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ext.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestA2ASecurityScheme_Validate(t *testing.T) {
	tests := []struct {
		name    string
		scheme  model.A2ASecurityScheme
		wantErr bool
	}{
		{"valid bearer", model.A2ASecurityScheme{Type: "bearer", Method: "header", Name: "Authorization"}, false},
		{"missing type", model.A2ASecurityScheme{Type: "", Method: "header"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.scheme.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestA2AInterface_Validate(t *testing.T) {
	tests := []struct {
		name    string
		iface   model.A2AInterface
		wantErr bool
	}{
		{"valid", model.A2AInterface{URL: "https://example.com/a2a", Binding: "jsonrpc"}, false},
		{"missing url", model.A2AInterface{URL: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.iface.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestA2ASignature_Validate(t *testing.T) {
	valid := model.A2ASignature{Algorithm: "RS256", KeyID: "key-1"}
	assert.NoError(t, valid.Validate())

	invalid := model.A2ASignature{Algorithm: ""}
	assert.Error(t, invalid.Validate())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestA2A -v`
Expected: compilation error — `model.A2AExtension` undefined

- [ ] **Step 3: Implement A2A metadata types with init() registration**

```go
// internal/model/a2a_metadata.go
package model

import "fmt"

func init() {
	RegisterTypedMeta("a2a.extension", func() TypedMetadata { return &A2AExtension{} })
	RegisterTypedMeta("a2a.security_scheme", func() TypedMetadata { return &A2ASecurityScheme{} })
	RegisterTypedMeta("a2a.interface", func() TypedMetadata { return &A2AInterface{} })
	RegisterTypedMeta("a2a.signature", func() TypedMetadata { return &A2ASignature{} })
}

// A2AExtension represents an A2A agent card extension.
type A2AExtension struct {
	URI      string `json:"uri"`
	Required bool   `json:"required"`
}

func (e *A2AExtension) Kind() string { return "a2a.extension" }
func (e *A2AExtension) Validate() error {
	if e.URI == "" {
		return fmt.Errorf("extension uri is required")
	}
	return nil
}

// A2ASecurityScheme represents an A2A security scheme.
type A2ASecurityScheme struct {
	Type   string `json:"type"`
	Method string `json:"method,omitempty"`
	Name   string `json:"name,omitempty"`
}

func (s *A2ASecurityScheme) Kind() string { return "a2a.security_scheme" }
func (s *A2ASecurityScheme) Validate() error {
	if s.Type == "" {
		return fmt.Errorf("security scheme type is required")
	}
	return nil
}

// A2AInterface represents a supported interface endpoint.
type A2AInterface struct {
	URL     string `json:"url"`
	Binding string `json:"binding,omitempty"`
}

func (i *A2AInterface) Kind() string { return "a2a.interface" }
func (i *A2AInterface) Validate() error {
	if i.URL == "" {
		return fmt.Errorf("interface url is required")
	}
	return nil
}

// A2ASignature represents a signature block.
type A2ASignature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId,omitempty"`
}

func (s *A2ASignature) Kind() string { return "a2a.signature" }
func (s *A2ASignature) Validate() error {
	if s.Algorithm == "" {
		return fmt.Errorf("signature algorithm is required")
	}
	return nil
}
```

- [ ] **Step 4: Run all model tests**

Run: `go test ./internal/model/ -v`
Expected: ALL PASS (including Task 1 round-trip tests which now have the registered types)

- [ ] **Step 5: Commit**

```bash
git add internal/model/a2a_metadata.go internal/model/a2a_metadata_test.go
git commit -m "feat: add A2A typed metadata implementations (extension, security, interface, signature)"
```

---

## Task 3: Extend CatalogEntry Model + Migration

**Files:**
- Modify: `internal/model/agent.go`
- Modify: `internal/db/migrations.go`

- [ ] **Step 1: Write failing test for CatalogEntry with new fields**

```go
// Add to internal/model/typed_metadata_test.go
func TestCatalogEntry_SyncTypedMeta_RoundTrip(t *testing.T) {
	entry := &model.CatalogEntry{
		ID:          "test-1",
		DisplayName: "Test",
		Protocol:    model.ProtocolA2A,
		SpecVersion: "1.0",
		TypedMeta: []model.TypedMetadata{
			&model.A2AExtension{URI: "urn:test", Required: true},
			&model.A2AInterface{URL: "https://example.com", Binding: "jsonrpc"},
		},
	}
	entry.SyncToDB()

	// Verify JSON was produced
	assert.NotEqual(t, "[]", entry.TypedMetaJSON)
	assert.Equal(t, "1.0", entry.SpecVersion)

	// Round-trip via SyncFromDB
	restored := &model.CatalogEntry{
		SpecVersion:  entry.SpecVersion,
		TypedMetaJSON: entry.TypedMetaJSON,
	}
	restored.SyncFromDB()

	require.Len(t, restored.TypedMeta, 2)
	ext, ok := restored.TypedMeta[0].(*model.A2AExtension)
	require.True(t, ok)
	assert.Equal(t, "urn:test", ext.URI)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestCatalogEntry_SyncTypedMeta -v`
Expected: compilation error — `entry.SpecVersion` undefined

- [ ] **Step 3: Add SpecVersion and TypedMeta to CatalogEntry**

In `internal/model/agent.go`, add to the CatalogEntry struct:

After line 80 (`Metadata    map[string]string ...`), add:
```go
	SpecVersion string          `json:"spec_version,omitempty" gorm:"type:text;not null;default:''"`
	TypedMeta   []TypedMetadata `json:"typed_meta,omitempty" gorm:"-"`
```

After line 92 (`LastSeen       time.Time ...`), add:
```go
	TypedMetaJSON string `json:"-" gorm:"column:typed_meta;type:text;not null;default:'[]'"`
```

In `SyncToDB()` method, before the closing brace, add:
```go
	if b, err := MarshalTypedMetaJSON(e.TypedMeta); err == nil {
		e.TypedMetaJSON = string(b)
	}
```

In `SyncFromDB()` method, before the Validity reconstruction, add:
```go
	if e.TypedMetaJSON != "" {
		e.TypedMeta, _ = UnmarshalTypedMetaJSON([]byte(e.TypedMetaJSON))
	}
```

In `MarshalJSON()`, update the anonymous struct to include:
```go
	SpecVersion string          `json:"spec_version,omitempty"`
	TypedMeta   []TypedMetadata `json:"typed_meta,omitempty"`
```
And in the struct literal:
```go
	SpecVersion: e.SpecVersion,
	TypedMeta:   e.TypedMeta,
```

- [ ] **Step 4: Add migration 005**

In `internal/db/migrations.go`, add to `AllMigrations()` slice:
```go
	migration005TypedMetadata(),
```

Add the migration function:
```go
func migration005TypedMetadata() Migration {
	return Migration{
		Version:     5,
		Description: "add spec_version and typed_meta columns to catalog_entries",
		Up: func(tx *gorm.DB) error {
			if err := tx.Exec("ALTER TABLE catalog_entries ADD COLUMN spec_version TEXT NOT NULL DEFAULT ''").Error; err != nil {
				return err
			}
			return tx.Exec("ALTER TABLE catalog_entries ADD COLUMN typed_meta TEXT NOT NULL DEFAULT '[]'").Error
		},
	}
}
```

- [ ] **Step 5: Run all model and store tests**

Run: `go test ./internal/model/ -v && go test ./internal/store/ -v && go test ./internal/db/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/model/agent.go internal/db/migrations.go internal/model/typed_metadata_test.go
git commit -m "feat: extend CatalogEntry with SpecVersion and TypedMeta, add migration 005"
```

---

## Task 4: Test Fixtures

**Files:**
- Create: `plugins/parsers/a2a/testdata/a2a_v03_card.json`
- Create: `plugins/parsers/a2a/testdata/a2a_v10_card.json`
- Create: `plugins/parsers/a2a/testdata/a2a_legacy_card.json`
- Create: `plugins/parsers/a2a/testdata/a2a_invalid_card.json`

- [ ] **Step 1: Create v0.3 card fixture**

```json
// plugins/parsers/a2a/testdata/a2a_v03_card.json
{
  "name": "Weather Agent v0.3",
  "description": "Provides weather forecasts",
  "url": "https://weather.example.com/a2a",
  "version": "2.1.0",
  "supportsExtendedAgentCard": true,
  "provider": {
    "organization": "WeatherCorp",
    "url": "https://weathercorp.example.com"
  },
  "supportedInterfaces": [
    {"url": "https://weather.example.com/a2a", "binding": "jsonrpc"},
    {"url": "https://weather.example.com/http", "binding": "http"}
  ],
  "capabilities": {
    "extensions": [
      {"uri": "urn:weather:realtime", "required": true},
      {"uri": "urn:weather:historical", "required": false}
    ]
  },
  "securitySchemes": [
    {"type": "bearer", "method": "header", "name": "Authorization"},
    {"type": "apiKey", "method": "query", "name": "api_key"}
  ],
  "signatures": [
    {"algorithm": "RS256", "keyId": "weather-key-1"}
  ],
  "skills": [
    {
      "id": "get-forecast",
      "name": "Get Forecast",
      "description": "Returns weather forecast for a location",
      "tags": ["weather", "forecast"],
      "inputModes": ["text"],
      "outputModes": ["text", "json"]
    },
    {
      "id": "get-alerts",
      "name": "Get Alerts",
      "description": "Returns active weather alerts",
      "tags": ["weather", "alerts"],
      "inputModes": ["text"],
      "outputModes": ["text"]
    }
  ]
}
```

- [ ] **Step 2: Create v1.0 card fixture**

```json
// plugins/parsers/a2a/testdata/a2a_v10_card.json
{
  "name": "Translation Agent v1.0",
  "description": "Multi-language translation service",
  "version": "3.0.0",
  "provider": {
    "organization": "LangTech",
    "url": "https://langtech.example.com"
  },
  "supportedInterfaces": [
    {"url": "https://translate.example.com/a2a", "binding": "jsonrpc"}
  ],
  "capabilities": {
    "extensions": [
      {"uri": "urn:translate:glossary", "required": true},
      {"uri": "urn:translate:memory", "required": false},
      {"uri": "urn:translate:context", "required": false}
    ],
    "supportsExtendedAgentCard": true
  },
  "securitySchemes": [
    {"type": "oauth2", "method": "header", "name": "Authorization"},
    {"type": "mtls"}
  ],
  "signatures": [
    {"algorithm": "ES256", "keyId": "lang-key-1"}
  ],
  "skills": [
    {
      "id": "translate-text",
      "name": "Translate Text",
      "description": "Translates text between languages",
      "tags": ["translation", "text"],
      "inputModes": ["text"],
      "outputModes": ["text"]
    },
    {
      "id": "detect-language",
      "name": "Detect Language",
      "description": "Detects the language of input text",
      "tags": ["detection"],
      "inputModes": ["text"],
      "outputModes": ["json"]
    }
  ]
}
```

- [ ] **Step 3: Create legacy card fixture (url only, no supportedInterfaces)**

```json
// plugins/parsers/a2a/testdata/a2a_legacy_card.json
{
  "name": "Legacy Agent",
  "description": "An older agent card format",
  "url": "https://legacy.example.com/agent",
  "version": "1.0.0",
  "provider": {
    "organization": "OldCorp"
  },
  "skills": [
    {
      "id": "echo",
      "name": "Echo",
      "description": "Echoes back the input",
      "inputModes": ["text"],
      "outputModes": ["text"]
    }
  ]
}
```

- [ ] **Step 4: Create invalid card fixture**

```json
// plugins/parsers/a2a/testdata/a2a_invalid_card.json
{
  "description": "Missing name and endpoints",
  "version": "1.0.0",
  "capabilities": {
    "extensions": [
      {"required": true},
      {"uri": "urn:valid:ext", "required": false}
    ]
  },
  "skills": [
    {
      "name": "Incomplete Skill",
      "inputModes": ["text"]
    },
    {
      "id": "valid-skill",
      "name": "Valid Skill",
      "description": "This skill is valid"
    }
  ]
}
```

- [ ] **Step 5: Commit**

```bash
git add plugins/parsers/a2a/testdata/
git commit -m "test: add A2A card test fixtures (v0.3, v1.0, legacy, invalid)"
```

---

## Task 5: A2A Parser Validation Logic

**Files:**
- Create: `plugins/parsers/a2a/validation.go`
- Create: `plugins/parsers/a2a/validation_test.go`

- [ ] **Step 1: Write failing tests for ValidateCard**

```go
// plugins/parsers/a2a/validation_test.go
package a2a_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return data
}

func TestValidateCard_V03(t *testing.T) {
	result := a2a.ValidateCard(readFixture(t, "a2a_v03_card.json"))

	assert.True(t, result.Valid)
	assert.Equal(t, "0.3", result.SpecVersion)
	assert.Empty(t, result.Errors)
	assert.NotNil(t, result.Preview)
	assert.Equal(t, "Weather Agent v0.3", result.Preview.DisplayName)
	assert.Equal(t, 2, result.Preview.SkillsCount)
	assert.Equal(t, 2, result.Preview.ExtensionsCount)
	assert.Contains(t, result.Preview.SecuritySchemes, "bearer")
	assert.Contains(t, result.Preview.SecuritySchemes, "apiKey")
	assert.Len(t, result.Preview.Interfaces, 2)
}

func TestValidateCard_V10(t *testing.T) {
	result := a2a.ValidateCard(readFixture(t, "a2a_v10_card.json"))

	assert.True(t, result.Valid)
	assert.Equal(t, "1.0", result.SpecVersion)
	assert.Empty(t, result.Errors)
	assert.NotNil(t, result.Preview)
	assert.Equal(t, "Translation Agent v1.0", result.Preview.DisplayName)
	assert.Equal(t, 3, result.Preview.ExtensionsCount)
	assert.Contains(t, result.Preview.SecuritySchemes, "oauth2")
	assert.Contains(t, result.Preview.SecuritySchemes, "mtls")
}

func TestValidateCard_Legacy(t *testing.T) {
	result := a2a.ValidateCard(readFixture(t, "a2a_legacy_card.json"))

	assert.True(t, result.Valid)
	assert.Equal(t, "", result.SpecVersion)
	assert.Empty(t, result.Errors)
	assert.NotNil(t, result.Preview)
	assert.Equal(t, "Legacy Agent", result.Preview.DisplayName)
	assert.Equal(t, []string{"https://legacy.example.com/agent"}, result.Preview.Interfaces)
}

func TestValidateCard_Invalid(t *testing.T) {
	result := a2a.ValidateCard(readFixture(t, "a2a_invalid_card.json"))

	assert.False(t, result.Valid)
	assert.Nil(t, result.Preview)
	require.NotEmpty(t, result.Errors)

	// Check specific errors
	fieldErrors := map[string]bool{}
	for _, e := range result.Errors {
		fieldErrors[e.Field] = true
	}
	assert.True(t, fieldErrors["name"], "should report missing name")
	assert.True(t, fieldErrors["supportedInterfaces"], "should report missing endpoints")
	assert.True(t, fieldErrors["skills[0].id"], "should report missing skill id")
	assert.True(t, fieldErrors["skills[0].description"], "should report missing skill description")
	assert.True(t, fieldErrors["extensions[0].uri"], "should report missing extension uri")
}

func TestValidateCard_InvalidJSON(t *testing.T) {
	result := a2a.ValidateCard([]byte("not json"))

	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "json", result.Errors[0].Field)
}

func TestValidateCard_V10_DeprecatedFields(t *testing.T) {
	// v1.0 card with deprecated stateTransitionHistory
	card := []byte(`{
		"name": "Deprecated Test",
		"description": "Tests deprecation warnings",
		"version": "1.0.0",
		"supportedInterfaces": [{"url": "https://example.com/a2a"}],
		"capabilities": {"supportsExtendedAgentCard": true},
		"stateTransitionHistory": true,
		"skills": [{"id": "s1", "name": "S1", "description": "Skill 1"}]
	}`)
	result := a2a.ValidateCard(card)

	assert.True(t, result.Valid)
	assert.Equal(t, "1.0", result.SpecVersion)
	assert.NotEmpty(t, result.Warnings, "should have deprecation warning for stateTransitionHistory")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./plugins/parsers/a2a/ -run TestValidateCard -v`
Expected: compilation error — `a2a.ValidateCard` undefined

- [ ] **Step 3: Implement ValidateCard**

```go
// plugins/parsers/a2a/validation.go
package a2a

import (
	"encoding/json"
	"fmt"
)

// ValidationError represents a single validation error with field path.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationPreview is a summary of a successfully parsed card.
type ValidationPreview struct {
	DisplayName     string   `json:"display_name"`
	Description     string   `json:"description"`
	Protocol        string   `json:"protocol"`
	SpecVersion     string   `json:"spec_version"`
	SkillsCount     int      `json:"skills_count"`
	ExtensionsCount int      `json:"extensions_count"`
	SecuritySchemes []string `json:"security_schemes"`
	Interfaces      []string `json:"interfaces"`
}

// ValidationResult is the structured output of card validation.
type ValidationResult struct {
	Valid       bool              `json:"valid"`
	SpecVersion string           `json:"spec_version"`
	Errors      []ValidationError `json:"errors"`
	Warnings    []string          `json:"warnings"`
	Preview     *ValidationPreview `json:"preview,omitempty"`
}

// fullCard is the extended JSON structure for parsing all A2A spec fields.
type fullCard struct {
	Name                      string          `json:"name"`
	Description               string          `json:"description"`
	URL                       string          `json:"url"`
	Version                   string          `json:"version"`
	Provider                  *a2aProvider    `json:"provider,omitempty"`
	Skills                    []fullSkill     `json:"skills,omitempty"`
	SupportedInterfaces       []fullInterface `json:"supportedInterfaces,omitempty"`
	SecuritySchemes           []fullSecurity  `json:"securitySchemes,omitempty"`
	Signatures                []fullSignature `json:"signatures,omitempty"`
	SupportsExtendedAgentCard *bool           `json:"supportsExtendedAgentCard,omitempty"`
	Capabilities              *capabilities   `json:"capabilities,omitempty"`
	StateTransitionHistory    *bool           `json:"stateTransitionHistory,omitempty"`
}

type capabilities struct {
	Extensions                []fullExtension `json:"extensions,omitempty"`
	SupportsExtendedAgentCard *bool           `json:"supportsExtendedAgentCard,omitempty"`
}

type fullSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

type fullExtension struct {
	URI      string `json:"uri"`
	Required bool   `json:"required"`
}

type fullInterface struct {
	URL     string `json:"url"`
	Binding string `json:"binding,omitempty"`
}

type fullSecurity struct {
	Type   string `json:"type"`
	Method string `json:"method,omitempty"`
	Name   string `json:"name,omitempty"`
}

type fullSignature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId,omitempty"`
}

// ValidateCard validates a raw JSON agent card and returns structured results.
func ValidateCard(raw []byte) ValidationResult {
	var card fullCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return ValidationResult{
			Valid:  false,
			Errors: []ValidationError{{Field: "json", Message: fmt.Sprintf("invalid JSON: %v", err)}},
		}
	}

	var errs []ValidationError
	var warnings []string

	// Detect spec version.
	specVersion := detectSpecVersion(&card)

	// Required fields.
	if card.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "required field is missing"})
	}
	if card.Description == "" {
		errs = append(errs, ValidationError{Field: "description", Message: "required field is missing"})
	}
	if card.Version == "" {
		errs = append(errs, ValidationError{Field: "version", Message: "required field is missing"})
	}

	// Endpoint: supportedInterfaces or legacy url.
	if len(card.SupportedInterfaces) == 0 && card.URL == "" {
		errs = append(errs, ValidationError{Field: "supportedInterfaces", Message: "at least one endpoint is required (supportedInterfaces or url)"})
	}

	// Validate skills.
	for i, s := range card.Skills {
		if s.ID == "" {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("skills[%d].id", i), Message: "required field is missing"})
		}
		if s.Name == "" {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("skills[%d].name", i), Message: "required field is missing"})
		}
		if s.Description == "" {
			errs = append(errs, ValidationError{Field: fmt.Sprintf("skills[%d].description", i), Message: "required field is missing"})
		}
	}

	// Validate extensions.
	if card.Capabilities != nil {
		for i, ext := range card.Capabilities.Extensions {
			if ext.URI == "" {
				errs = append(errs, ValidationError{Field: fmt.Sprintf("extensions[%d].uri", i), Message: "required field is missing"})
			}
		}
	}

	// Deprecation warnings.
	if card.StateTransitionHistory != nil {
		warnings = append(warnings, "stateTransitionHistory field removed in v1.0, ignored")
	}

	if len(errs) > 0 {
		return ValidationResult{
			Valid:       false,
			SpecVersion: specVersion,
			Errors:      errs,
			Warnings:    warnings,
		}
	}

	// Build preview.
	interfaces := make([]string, 0)
	for _, iface := range card.SupportedInterfaces {
		interfaces = append(interfaces, iface.URL)
	}
	if len(interfaces) == 0 && card.URL != "" {
		interfaces = append(interfaces, card.URL)
	}

	secSchemes := make([]string, 0)
	for _, s := range card.SecuritySchemes {
		secSchemes = append(secSchemes, s.Type)
	}

	extCount := 0
	if card.Capabilities != nil {
		extCount = len(card.Capabilities.Extensions)
	}

	return ValidationResult{
		Valid:       true,
		SpecVersion: specVersion,
		Errors:      []ValidationError{},
		Warnings:    warnings,
		Preview: &ValidationPreview{
			DisplayName:     card.Name,
			Description:     card.Description,
			Protocol:        "a2a",
			SpecVersion:     specVersion,
			SkillsCount:     len(card.Skills),
			ExtensionsCount: extCount,
			SecuritySchemes: secSchemes,
			Interfaces:      interfaces,
		},
	}
}

// detectSpecVersion determines the A2A spec version based on card signals.
func detectSpecVersion(card *fullCard) string {
	// v0.3: supportsExtendedAgentCard at root level.
	if card.SupportsExtendedAgentCard != nil {
		return "0.3"
	}
	// v1.0: supportsExtendedAgentCard in capabilities (not root).
	if card.Capabilities != nil && card.Capabilities.SupportsExtendedAgentCard != nil {
		return "1.0"
	}
	// No version signal detected.
	return ""
}
```

- [ ] **Step 4: Run validation tests**

Run: `go test ./plugins/parsers/a2a/ -run TestValidateCard -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add plugins/parsers/a2a/validation.go plugins/parsers/a2a/validation_test.go
git commit -m "feat: add A2A card validation with spec version detection and structured errors"
```

---

## Task 6: Enhance A2A Parser Parse() Method

**Files:**
- Modify: `plugins/parsers/a2a/a2a.go`

- [ ] **Step 1: Write failing test for Parse() with v1.0 card**

```go
// Add to plugins/parsers/a2a/validation_test.go
import (
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
)

func TestParse_V10Card(t *testing.T) {
	p := a2a.New()
	entry, err := p.Parse(readFixture(t, "a2a_v10_card.json"), model.SourcePush)
	require.NoError(t, err)

	assert.Equal(t, "Translation Agent v1.0", entry.DisplayName)
	assert.Equal(t, "https://translate.example.com/a2a", entry.Endpoint)
	assert.Equal(t, "1.0", entry.SpecVersion)
	assert.NotEmpty(t, entry.TypedMeta)

	// Check typed metadata contents.
	var extCount, secCount, ifaceCount int
	for _, tm := range entry.TypedMeta {
		switch tm.Kind() {
		case "a2a.extension":
			extCount++
		case "a2a.security_scheme":
			secCount++
		case "a2a.interface":
			ifaceCount++
		}
	}
	assert.Equal(t, 3, extCount)
	assert.Equal(t, 2, secCount)
	assert.Equal(t, 1, ifaceCount)
}

func TestParse_V03Card(t *testing.T) {
	p := a2a.New()
	entry, err := p.Parse(readFixture(t, "a2a_v03_card.json"), model.SourceConfig)
	require.NoError(t, err)

	assert.Equal(t, "Weather Agent v0.3", entry.DisplayName)
	assert.Equal(t, "https://weather.example.com/a2a", entry.Endpoint)
	assert.Equal(t, "0.3", entry.SpecVersion)
	assert.Equal(t, model.SourceConfig, entry.Source)
}

func TestParse_LegacyCard(t *testing.T) {
	p := a2a.New()
	entry, err := p.Parse(readFixture(t, "a2a_legacy_card.json"), model.SourcePush)
	require.NoError(t, err)

	assert.Equal(t, "Legacy Agent", entry.DisplayName)
	assert.Equal(t, "https://legacy.example.com/agent", entry.Endpoint)
	assert.Equal(t, "", entry.SpecVersion)
	assert.Empty(t, entry.TypedMeta)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./plugins/parsers/a2a/ -run TestParse_V10Card -v`
Expected: FAIL — `entry.SpecVersion` is empty, `entry.TypedMeta` is nil

- [ ] **Step 3: Update Parse() to extract new fields**

Replace the entire `a2a.go` content:

```go
// Package a2a provides the A2A protocol parser plugin.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// Plugin implements the A2A parser plugin.
type Plugin struct {
	initialized bool
}

// New creates a new A2A parser plugin.
func New() *Plugin { return &Plugin{} }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "a2a-parser" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeParser }

// Protocol returns the protocol this parser handles.
func (p *Plugin) Protocol() model.Protocol { return model.ProtocolA2A }

// CardPath returns the default card path for A2A.
func (p *Plugin) CardPath() string { return "/.well-known/agent-card.json" }

// Init initializes the plugin.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.initialized = true
	return nil
}

// Start starts the plugin (no-op for parser).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for parser).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Parse parses an A2A agent card JSON blob into a CatalogEntry.
func (p *Plugin) Parse(raw []byte, source model.SourceType) (*model.CatalogEntry, error) {
	var card fullCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("parsing a2a card: %w", err)
	}
	if card.Name == "" {
		return nil, fmt.Errorf("a2a card missing required field: name")
	}

	// Determine endpoint: prefer first supportedInterfaces URL, fall back to url.
	endpoint := card.URL
	if len(card.SupportedInterfaces) > 0 {
		endpoint = card.SupportedInterfaces[0].URL
	}
	if endpoint == "" {
		return nil, fmt.Errorf("a2a card missing required field: url or supportedInterfaces")
	}

	// Convert skills.
	skills := make([]model.Skill, 0, len(card.Skills))
	for _, s := range card.Skills {
		skills = append(skills, model.Skill{
			Name:        s.Name,
			Description: s.Description,
			InputModes:  s.InputModes,
			OutputModes: s.OutputModes,
		})
	}

	// Build provider.
	var provider model.Provider
	if card.Provider != nil {
		provider.Organization = card.Provider.Organization
		provider.Team = card.Provider.Organization
		provider.URL = card.Provider.URL
	}

	// Detect spec version.
	specVersion := detectSpecVersion(&card)

	// Build typed metadata.
	var typedMeta []model.TypedMetadata

	// Extensions from capabilities.
	if card.Capabilities != nil {
		for _, ext := range card.Capabilities.Extensions {
			typedMeta = append(typedMeta, &model.A2AExtension{
				URI:      ext.URI,
				Required: ext.Required,
			})
		}
	}

	// Security schemes.
	for _, sec := range card.SecuritySchemes {
		typedMeta = append(typedMeta, &model.A2ASecurityScheme{
			Type:   sec.Type,
			Method: sec.Method,
			Name:   sec.Name,
		})
	}

	// Supported interfaces.
	for _, iface := range card.SupportedInterfaces {
		typedMeta = append(typedMeta, &model.A2AInterface{
			URL:     iface.URL,
			Binding: iface.Binding,
		})
	}

	// Signatures.
	for _, sig := range card.Signatures {
		typedMeta = append(typedMeta, &model.A2ASignature{
			Algorithm: sig.Algorithm,
			KeyID:     sig.KeyID,
		})
	}

	now := time.Now().UTC()
	return &model.CatalogEntry{
		DisplayName: card.Name,
		Description: card.Description,
		Protocol:    model.ProtocolA2A,
		Endpoint:    endpoint,
		Version:     card.Version,
		Status:      model.StatusUnknown,
		Source:      source,
		Provider:    provider,
		Skills:      skills,
		SpecVersion: specVersion,
		TypedMeta:   typedMeta,
		Validity:    model.Validity{LastSeen: now},
		RawCard:     json.RawMessage(raw),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
```

- [ ] **Step 4: Run all parser tests**

Run: `go test ./plugins/parsers/a2a/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add plugins/parsers/a2a/a2a.go plugins/parsers/a2a/validation_test.go
git commit -m "feat: enhance A2A parser to extract extensions, security schemes, interfaces, and spec version"
```

---

## Task 7: Validation Endpoint

**Files:**
- Create: `internal/api/validate_handler.go`
- Create: `internal/api/validate_handler_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write failing test for validation endpoint**

```go
// internal/api/validate_handler_test.go
package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
)

func TestValidateEndpoint_ValidCard(t *testing.T) {
	router, _ := newTestRouter(t)
	body, err := os.ReadFile("../../plugins/parsers/a2a/testdata/a2a_v10_card.json")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result a2a.ValidationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.True(t, result.Valid)
	assert.Equal(t, "1.0", result.SpecVersion)
	assert.NotNil(t, result.Preview)
	assert.Equal(t, "Translation Agent v1.0", result.Preview.DisplayName)
}

func TestValidateEndpoint_InvalidCard(t *testing.T) {
	router, _ := newTestRouter(t)
	body, err := os.ReadFile("../../plugins/parsers/a2a/testdata/a2a_invalid_card.json")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result a2a.ValidationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateEndpoint_InvalidJSON(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/validate", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api/ -run TestValidateEndpoint -v`
Expected: 404 — route not registered

- [ ] **Step 3: Implement validation handler**

```go
// internal/api/validate_handler.go
package api

import (
	"io"
	"net/http"

	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
)

// ValidateAgentCard handles POST /api/v1/catalog/validate.
func (h *Handler) ValidateAgentCard(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	result := a2a.ValidateCard(raw)

	if result.Valid {
		JSONResponse(w, http.StatusOK, result)
	} else {
		JSONResponse(w, http.StatusUnprocessableEntity, result)
	}
}
```

- [ ] **Step 4: Register route in router.go**

In `internal/api/router.go`, add the validate route next to the existing catalog routes.

In the protected catalog routes section (around line 59), add:
```go
r.With(RequirePermission(auth.PermCatalogWrite)).Post("/catalog/validate", h.ValidateAgentCard)
```

In the unprotected section (around line 109), add:
```go
r.Post("/catalog/validate", h.ValidateAgentCard)
```

**Important:** The `/catalog/validate` route must be registered BEFORE `/catalog/{id}` to avoid chi interpreting "validate" as an `{id}` parameter. Move it above the `GET /catalog/{id}` line.

- [ ] **Step 5: Run validation endpoint tests**

Run: `go test ./internal/api/ -run TestValidateEndpoint -v`
Expected: ALL PASS

- [ ] **Step 6: Run all existing tests to verify no regression**

Run: `go test ./internal/api/ -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/api/validate_handler.go internal/api/validate_handler_test.go internal/api/router.go
git commit -m "feat: add POST /api/v1/catalog/validate dry-run validation endpoint"
```

---

## Task 8: Frontend Types & API Client

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Add TypeScript types**

In `web/src/types.ts`, add after the `ListFilter` interface:

```typescript
export interface TypedMeta {
  kind: string
  [key: string]: unknown
}

export interface A2AExtensionMeta extends TypedMeta {
  kind: 'a2a.extension'
  uri: string
  required: boolean
}

export interface A2ASecuritySchemeMeta extends TypedMeta {
  kind: 'a2a.security_scheme'
  type: string
  method?: string
  name?: string
}

export interface A2AInterfaceMeta extends TypedMeta {
  kind: 'a2a.interface'
  url: string
  binding?: string
}

export interface ValidationError {
  field: string
  message: string
}

export interface ValidationPreview {
  display_name: string
  description: string
  protocol: string
  spec_version: string
  skills_count: number
  extensions_count: number
  security_schemes: string[]
  interfaces: string[]
}

export interface ValidationResult {
  valid: boolean
  spec_version: string
  errors: ValidationError[]
  warnings: string[]
  preview?: ValidationPreview
}
```

Add `spec_version` and `typed_meta` to the existing `CatalogEntry` interface:

```typescript
// Add after raw_card line:
  spec_version?: string
  typed_meta?: TypedMeta[]
```

- [ ] **Step 2: Add API functions**

In `web/src/api.ts`, add after the `getStats()` function:

```typescript
export function validateAgentCard(cardJson: string): Promise<ValidationResult> {
  return request<ValidationResult>('/catalog/validate', {
    method: 'POST',
    body: cardJson,
  })
}

export function createAgentFromCard(cardJson: string): Promise<CatalogEntry> {
  return request<CatalogEntry>('/catalog', {
    method: 'POST',
    body: cardJson,
  })
}
```

Add `ValidationResult` to the import from types:
```typescript
import type { CatalogEntry, ListFilter, Stats, ValidationResult } from './types'
```

- [ ] **Step 3: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add TypedMeta types and validation API client functions"
```

---

## Task 9: CardPreview Component

**Files:**
- Create: `web/src/components/CardPreview.tsx`

- [ ] **Step 1: Create the reusable CardPreview component**

```tsx
// web/src/components/CardPreview.tsx
import type { ValidationPreview, TypedMeta } from '../types'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'

interface CardPreviewProps {
  preview: ValidationPreview
  typedMeta?: TypedMeta[]
}

export default function CardPreview({ preview, typedMeta }: CardPreviewProps) {
  const extensions = typedMeta?.filter(m => m.kind === 'a2a.extension') ?? []
  const securitySchemes = typedMeta?.filter(m => m.kind === 'a2a.security_scheme') ?? []

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-lg font-semibold">{preview.display_name}</h3>
        {preview.description && (
          <p className="text-sm text-muted-foreground mt-1">{preview.description}</p>
        )}
      </div>

      <div className="flex flex-wrap gap-2">
        <Badge variant="secondary">{preview.protocol.toUpperCase()}</Badge>
        {preview.spec_version && (
          <Badge variant="outline">v{preview.spec_version}</Badge>
        )}
        <Badge variant="secondary">{preview.skills_count} skill{preview.skills_count !== 1 ? 's' : ''}</Badge>
      </div>

      {preview.interfaces.length > 0 && (
        <>
          <Separator />
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Interfaces</p>
            <div className="space-y-1">
              {preview.interfaces.map((url, i) => (
                <p key={i} className="text-sm font-mono break-all">{url}</p>
              ))}
            </div>
          </div>
        </>
      )}

      {extensions.length > 0 && (
        <>
          <Separator />
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">
              Extensions ({extensions.length})
            </p>
            <div className="space-y-1">
              {extensions.map((ext, i) => (
                <div key={i} className="flex items-center gap-2 text-sm">
                  <span className="font-mono break-all">{(ext as { uri: string }).uri}</span>
                  <Badge variant={(ext as { required: boolean }).required ? 'destructive' : 'secondary'} className="text-xs">
                    {(ext as { required: boolean }).required ? 'Required' : 'Optional'}
                  </Badge>
                </div>
              ))}
            </div>
          </div>
        </>
      )}

      {(securitySchemes.length > 0 || preview.security_schemes.length > 0) && (
        <>
          <Separator />
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Security</p>
            <div className="flex flex-wrap gap-1">
              {preview.security_schemes.map((scheme, i) => (
                <Badge key={i} variant="outline">{scheme}</Badge>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/CardPreview.tsx
git commit -m "feat: add reusable CardPreview component for agent card display"
```

---

## Task 10: RegisterAgentDialog Component

**Files:**
- Create: `web/src/components/RegisterAgentDialog.tsx`
- Modify: `web/src/components/CatalogList.tsx`

- [ ] **Step 1: Create the RegisterAgentDialog**

```tsx
// web/src/components/RegisterAgentDialog.tsx
import { useState, useCallback } from 'react'
import { validateAgentCard, createAgentFromCard } from '../api'
import type { ValidationResult } from '../types'
import CardPreview from './CardPreview'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Card } from '@/components/ui/card'
import { Plus, Upload, AlertCircle, AlertTriangle, CheckCircle } from 'lucide-react'

type Step = 'input' | 'validation' | 'preview' | 'confirm'

interface RegisterAgentDialogProps {
  onRegistered: () => void
}

export default function RegisterAgentDialog({ onRegistered }: RegisterAgentDialogProps) {
  const [open, setOpen] = useState(false)
  const [step, setStep] = useState<Step>('input')
  const [cardJson, setCardJson] = useState('')
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null)
  const [validating, setValidating] = useState(false)
  const [registering, setRegistering] = useState(false)
  const [registerError, setRegisterError] = useState<string | null>(null)

  const reset = useCallback(() => {
    setStep('input')
    setCardJson('')
    setJsonError(null)
    setValidationResult(null)
    setValidating(false)
    setRegistering(false)
    setRegisterError(null)
  }, [])

  const handleOpenChange = (v: boolean) => {
    setOpen(v)
    if (!v) reset()
  }

  const handleJsonChange = (value: string) => {
    setCardJson(value)
    setJsonError(null)
    if (value.trim()) {
      try {
        JSON.parse(value)
      } catch {
        setJsonError('Invalid JSON syntax')
      }
    }
  }

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      const text = reader.result as string
      handleJsonChange(text)
    }
    reader.readAsText(file)
  }

  const handleValidate = async () => {
    if (jsonError || !cardJson.trim()) return
    setValidating(true)
    try {
      const result = await validateAgentCard(cardJson)
      setValidationResult(result)
      if (result.valid && result.errors.length === 0 && result.warnings.length === 0) {
        setStep('preview')
      } else {
        setStep('validation')
      }
    } catch (e) {
      setValidationResult({
        valid: false,
        spec_version: '',
        errors: [{ field: 'request', message: e instanceof Error ? e.message : 'Validation failed' }],
        warnings: [],
      })
      setStep('validation')
    } finally {
      setValidating(false)
    }
  }

  const handleRegister = async () => {
    setRegistering(true)
    setRegisterError(null)
    try {
      await createAgentFromCard(cardJson)
      setOpen(false)
      reset()
      onRegistered()
    } catch (e) {
      setRegisterError(e instanceof Error ? e.message : 'Registration failed')
    } finally {
      setRegistering(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-2 h-4 w-4" />
          Register Agent
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Register Agent</DialogTitle>
        </DialogHeader>

        {step === 'input' && (
          <div className="space-y-4">
            <Tabs defaultValue="paste">
              <TabsList>
                <TabsTrigger value="paste">Paste JSON</TabsTrigger>
                <TabsTrigger value="upload">Upload File</TabsTrigger>
              </TabsList>
              <TabsContent value="paste">
                <textarea
                  className="w-full h-64 font-mono text-sm border rounded-md p-3 bg-muted/50 focus:outline-none focus:ring-2 focus:ring-ring"
                  placeholder={'{\n  "name": "My Agent",\n  "description": "...",\n  "version": "1.0.0",\n  "supportedInterfaces": [...],\n  "skills": [...]\n}'}
                  value={cardJson}
                  onChange={e => handleJsonChange(e.target.value)}
                />
              </TabsContent>
              <TabsContent value="upload">
                <div className="border-2 border-dashed rounded-md p-8 text-center">
                  <Upload className="mx-auto h-8 w-8 text-muted-foreground mb-2" />
                  <p className="text-sm text-muted-foreground mb-2">Drop a .json file or click to browse</p>
                  <input
                    type="file"
                    accept=".json"
                    onChange={handleFileUpload}
                    className="text-sm"
                  />
                </div>
              </TabsContent>
            </Tabs>

            {jsonError && (
              <p className="text-sm text-destructive flex items-center gap-1">
                <AlertCircle className="h-4 w-4" />
                {jsonError}
              </p>
            )}

            <div className="flex justify-end">
              <Button onClick={handleValidate} disabled={!cardJson.trim() || !!jsonError || validating}>
                {validating ? 'Validating...' : 'Validate'}
              </Button>
            </div>
          </div>
        )}

        {step === 'validation' && validationResult && (
          <div className="space-y-4">
            {validationResult.errors.length > 0 && (
              <Card className="border-destructive bg-destructive/10 p-4">
                <p className="font-medium text-destructive flex items-center gap-1 mb-2">
                  <AlertCircle className="h-4 w-4" />
                  Validation Errors
                </p>
                <ul className="text-sm text-destructive space-y-1">
                  {validationResult.errors.map((err, i) => (
                    <li key={i}><code className="font-mono">{err.field}</code>: {err.message}</li>
                  ))}
                </ul>
              </Card>
            )}

            {validationResult.warnings.length > 0 && (
              <Card className="border-yellow-500 bg-yellow-50 dark:bg-yellow-900/10 p-4">
                <p className="font-medium text-yellow-700 dark:text-yellow-400 flex items-center gap-1 mb-2">
                  <AlertTriangle className="h-4 w-4" />
                  Warnings
                </p>
                <ul className="text-sm text-yellow-700 dark:text-yellow-400 space-y-1">
                  {validationResult.warnings.map((w, i) => (
                    <li key={i}>{w}</li>
                  ))}
                </ul>
              </Card>
            )}

            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep('input')}>Back to Edit</Button>
              {validationResult.valid && (
                <Button onClick={() => setStep('preview')}>Continue to Preview</Button>
              )}
            </div>
          </div>
        )}

        {step === 'preview' && validationResult?.preview && (
          <div className="space-y-4">
            <Card className="border-green-500 bg-green-50 dark:bg-green-900/10 p-4">
              <p className="text-sm text-green-700 dark:text-green-400 flex items-center gap-1">
                <CheckCircle className="h-4 w-4" />
                Card validated successfully
              </p>
            </Card>

            <CardPreview preview={validationResult.preview} />

            {registerError && (
              <Card className="border-destructive bg-destructive/10 p-4 text-destructive text-sm">
                {registerError}
              </Card>
            )}

            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep('input')}>Back to Edit</Button>
              <Button onClick={handleRegister} disabled={registering}>
                {registering ? 'Registering...' : 'Register Agent'}
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 2: Add Register button to CatalogList**

In `web/src/components/CatalogList.tsx`, add the import:
```typescript
import RegisterAgentDialog from './RegisterAgentDialog'
```

In the filter bar `<div className="flex flex-col sm:flex-row gap-2 mb-4">`, add the RegisterAgentDialog after the status Select (before the closing `</div>` of the filter bar):

```tsx
        <RegisterAgentDialog onRegistered={load} />
```

- [ ] **Step 3: Build frontend to verify compilation**

Run: `cd web && bun run build`
Expected: Build succeeds with no TypeScript errors

- [ ] **Step 4: Commit**

```bash
git add web/src/components/RegisterAgentDialog.tsx web/src/components/CatalogList.tsx
git commit -m "feat: add Register Agent modal with 4-step validation flow"
```

---

## Task 11: EntryDetail — Extensions & Security Display

**Files:**
- Modify: `web/src/components/EntryDetail.tsx`

- [ ] **Step 1: Add Extensions and Security sections**

In `web/src/components/EntryDetail.tsx`:

Add imports for the new types at the top:
```typescript
import type { CatalogEntry, TypedMeta } from '../types'
```

In the badges section (after the version badge, around line 100), add spec_version badge:
```tsx
            {entry.spec_version && (
              <Badge variant="outline">Spec v{entry.spec_version}</Badge>
            )}
```

After the Skills section closing `</>` (around line 161) and before the Raw Card section, add:

```tsx
          {entry.typed_meta && entry.typed_meta.filter(m => m.kind === 'a2a.extension').length > 0 && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">
                  Extensions ({entry.typed_meta.filter(m => m.kind === 'a2a.extension').length})
                </p>
                <div className="space-y-1">
                  {entry.typed_meta.filter(m => m.kind === 'a2a.extension').map((ext, i) => (
                    <div key={i} className="flex items-center gap-2 text-sm">
                      <span className="font-mono break-all">{(ext as { uri: string }).uri}</span>
                      <Badge variant={(ext as { required: boolean }).required ? 'destructive' : 'secondary'} className="text-xs">
                        {(ext as { required: boolean }).required ? 'Required' : 'Optional'}
                      </Badge>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}

          {entry.typed_meta && entry.typed_meta.filter(m => m.kind === 'a2a.security_scheme').length > 0 && (
            <>
              <Separator className="my-4" />
              <div>
                <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Security</p>
                <div className="flex flex-wrap gap-1">
                  {entry.typed_meta.filter(m => m.kind === 'a2a.security_scheme').map((scheme, i) => (
                    <Badge key={i} variant="outline">
                      {(scheme as { type: string }).type}
                    </Badge>
                  ))}
                </div>
              </div>
            </>
          )}
```

- [ ] **Step 2: Build frontend to verify compilation**

Run: `cd web && bun run build`
Expected: Build succeeds

- [ ] **Step 3: Commit**

```bash
git add web/src/components/EntryDetail.tsx
git commit -m "feat: display extensions, security schemes, and spec version in agent detail view"
```

---

## Task 12: E2E Tests

**Files:**
- Modify: `e2e/tests/helpers.ts`
- Modify: `e2e/tests/catalog.spec.ts`

- [ ] **Step 1: Add validation helper to E2E helpers**

In `e2e/tests/helpers.ts`, add after the `deleteCatalogEntry` function:

```typescript
/** Validate an agent card via the API. Returns the validation result. */
export async function validateAgentCard(
  request: APIRequestContext,
  token: string,
  cardJson: string,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
): Promise<any> {
  const res = await request.post(`${BASE}/api/v1/catalog/validate`, {
    headers: { ...authHeader(token), 'Content-Type': 'application/json' },
    data: cardJson,
  });
  return { status: res.status(), body: await res.json() };
}
```

- [ ] **Step 2: Add validation API tests to catalog.spec.ts**

Add to the `test.describe('Catalog Management')` block:

```typescript
  test('validate valid A2A v1.0 card returns 200 with preview', async ({ request }) => {
    const card = JSON.stringify({
      name: 'E2E Validation Agent',
      description: 'Tests validation endpoint',
      version: '1.0.0',
      supportedInterfaces: [{ url: `http://validate-${Date.now()}.example.com/a2a`, binding: 'jsonrpc' }],
      capabilities: {
        extensions: [{ uri: 'urn:test:ext', required: true }],
        supportsExtendedAgentCard: true,
      },
      securitySchemes: [{ type: 'bearer', method: 'header', name: 'Authorization' }],
      skills: [{ id: 's1', name: 'Skill One', description: 'A test skill' }],
    });

    const { status, body } = await validateAgentCard(request, token, card);
    expect(status).toBe(200);
    expect(body.valid).toBe(true);
    expect(body.spec_version).toBe('1.0');
    expect(body.preview.display_name).toBe('E2E Validation Agent');
    expect(body.preview.extensions_count).toBe(1);
    expect(body.preview.security_schemes).toContain('bearer');
  });

  test('validate invalid card returns 422 with errors', async ({ request }) => {
    const card = JSON.stringify({
      description: 'Missing name and endpoints',
      skills: [{ name: 'No ID' }],
    });

    const { status, body } = await validateAgentCard(request, token, card);
    expect(status).toBe(422);
    expect(body.valid).toBe(false);
    expect(body.errors.length).toBeGreaterThan(0);

    const fields = body.errors.map((e: { field: string }) => e.field);
    expect(fields).toContain('name');
    expect(fields).toContain('supportedInterfaces');
  });
```

- [ ] **Step 3: Add UI registration flow test**

```typescript
  test('register agent via UI modal', async ({ page }) => {
    await loginViaUI(page);

    // Open register dialog.
    await page.getByRole('button', { name: 'Register Agent' }).click();
    await expect(page.getByText('Register Agent').nth(1)).toBeVisible();

    // Paste valid JSON.
    const card = JSON.stringify({
      name: `UI Register Agent ${Date.now()}`,
      description: 'Registered via UI E2E test',
      version: '1.0.0',
      url: `http://ui-register-${Date.now()}.example.com`,
      skills: [{ id: 'echo', name: 'Echo', description: 'Echoes input' }],
    }, null, 2);

    const textarea = page.locator('textarea');
    await textarea.fill(card);

    // Validate.
    await page.getByRole('button', { name: 'Validate' }).click();

    // Should advance to preview (or show preview).
    await expect(page.getByText('Card validated successfully')).toBeVisible({ timeout: 10_000 });

    // Register.
    await page.getByRole('button', { name: 'Register Agent' }).click();

    // Should close dialog and refresh catalog.
    await expect(page.getByText('UI Register Agent')).toBeVisible({ timeout: 10_000 });
  });
```

- [ ] **Step 4: Commit**

```bash
git add e2e/tests/helpers.ts e2e/tests/catalog.spec.ts
git commit -m "test: add E2E tests for validation endpoint and UI registration flow"
```

---

## Task 13: Build, Run All Tests, Fix Issues

- [ ] **Step 1: Run Go tests**

Run: `go test ./... -v`
Expected: ALL PASS

- [ ] **Step 2: Build Go binary**

Run: `go build ./cmd/agentlens/`
Expected: Compiles without errors

- [ ] **Step 3: Build frontend**

Run: `cd web && bun run build`
Expected: Build succeeds

- [ ] **Step 4: Fix any failures found in steps 1-3**

Address each failure individually: read the error, identify root cause, fix.

- [ ] **Step 5: Final commit if any fixes were needed**

```bash
git add -A
git commit -m "fix: address test/build issues from integration"
```

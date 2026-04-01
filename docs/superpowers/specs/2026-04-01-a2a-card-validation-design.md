# A2A Agent Card Validation & UI Import — Design Spec

**Date:** 2026-04-01
**Feature:** 1.6 — A2A Agent Card Validation
**Status:** Approved

---

## Summary

Enable AgentLens to validate A2A Agent Cards (v0.3 and v1.0) during registration, and provide a Dashboard modal to manually import/register cards with full spec compliance checking.

### Key Decisions

| Decision | Choice |
|----------|--------|
| Reachability check | Two-phase: schema validated synchronously, reachability checked asynchronously via health plugin |
| Data model | Typed metadata (`TypedMetadata` interface) inspired by Product Archetype, `SpecVersion` as first-class field |
| UI registration | Modal (shadcn/ui Dialog) with 4-step flow |
| Real-time validation | Hybrid: JSON syntax check client-side, full schema validation server-side via API |

---

## 1. Domain Model — Typed Metadata

### New Fields on CatalogEntry

```go
type CatalogEntry struct {
    // ...existing fields unchanged...
    SpecVersion  string          // "0.3", "1.0", "" (unknown)
    TypedMeta    []TypedMetadata // structured, protocol-specific metadata
}
```

### TypedMetadata Interface

Polymorphic base for protocol-specific structured data. Inspired by `FeatureValueConstraint` from the Product Archetype — each implementation self-validates and carries a `Kind()` discriminator for serialization.

```go
type TypedMetadata interface {
    Kind() string      // discriminator: "a2a.extension", "a2a.security_scheme", etc.
    Validate() error   // self-validation
}
```

### A2A Implementations

```go
type A2AExtension struct {
    URI      string `json:"uri"`
    Required bool   `json:"required"`
}
// Kind() = "a2a.extension"
// Validate() = URI required

type A2ASecurityScheme struct {
    Type   string `json:"type"`   // bearer, apiKey, oauth2, oidc, mtls
    Method string `json:"method"` // header, query, cookie
    Name   string `json:"name"`   // header/param name
}
// Kind() = "a2a.security_scheme"
// Validate() = Type required

type A2AInterface struct {
    URL     string `json:"url"`
    Binding string `json:"binding"` // jsonrpc, http, grpc
}
// Kind() = "a2a.interface"
// Validate() = URL required

type A2ASignature struct {
    Algorithm string `json:"algorithm"`
    KeyID     string `json:"keyId"`
}
// Kind() = "a2a.signature"
// Validate() = Algorithm required
```

### Extensibility

New protocols (MCP, a2ui) can add their own `TypedMetadata` implementations without changing the core model. Registration via `map[string]func() TypedMetadata` registry for deserialization.

---

## 2. A2A Parser Enhancement

### Extended Card Structure

```go
type a2aCard struct {
    // existing
    Name        string
    Description string
    URL         string              // legacy (v0.3 fallback)
    Version     string
    Provider    *a2aProvider
    Skills      []a2aSkill

    // new
    SupportedInterfaces []a2aInterface
    Extensions          []a2aExtension
    SecuritySchemes     []a2aSecurityScheme
    Signatures          []a2aSignature
    SupportsExtendedAgentCard *bool   // v0.3: root level
}
```

### Spec Version Auto-Detection

| Signal | Detected Version |
|--------|-----------------|
| `supportedInterfaces` present | >= v0.3 |
| `supportsExtendedAgentCard` at root | v0.3 |
| `supportsExtendedAgentCard` absent from root, present in capabilities | v1.0 |
| `stateTransitionHistory` present | v0.3 (deprecated in v1.0, log warning) |
| OAuth Implicit/Password flows present | v0.3 (removed in v1.0, log warning) |
| None of the above signals | unknown ("") |

### Parser Output Mapping

- `CatalogEntry.Endpoint` <- first URL from `supportedInterfaces`, fallback to `url` for legacy
- `CatalogEntry.SpecVersion` <- detected version string
- `CatalogEntry.TypedMeta` <- extensions, security schemes, interfaces, signatures as typed metadata
- `CatalogEntry.Skills` <- unchanged (existing logic)

### Validation Rules (Required Fields)

- `name` — required
- `description` — required
- `version` — required
- `supportedInterfaces` OR `url` — at least one endpoint
- Each `skill`: `id`, `name`, `description` required
- Each `extension`: `uri` + `required` required

Errors collected into a list (not fail-fast), returned as structured validation result.

### Spec Compatibility Matrix

| Feature | v0.3 | v1.0 | Parser Action |
|---------|------|------|---------------|
| supportedInterfaces[] | Y | Y | Primary endpoint |
| capabilities.extensions[] | Y | Y | Parse & store as TypedMeta |
| signatures[] | Y | Y | Store as TypedMeta |
| supportsExtendedAgentCard | Y (root) | N (moved) | Check both locations |
| OAuth Implicit/Password | Y (deprecated) | N (removed) | Parse, log warning |
| OAuth DeviceCode | N | Y | Parse when present |
| stateTransitionHistory | Y | N (removed) | Best-effort parse, log warning |

---

## 3. Validation Endpoint

### Endpoint

```
POST /api/v1/catalog/validate
```

Dry-run validation. No data persisted, no reachability check.

**Permission:** `catalog:write`

### Success Response (200)

```json
{
  "valid": true,
  "spec_version": "1.0",
  "warnings": [
    "OAuth Implicit flow is deprecated in v1.0",
    "stateTransitionHistory field removed in v1.0, ignored"
  ],
  "preview": {
    "display_name": "My Agent",
    "description": "...",
    "protocol": "a2a",
    "spec_version": "1.0",
    "skills_count": 3,
    "extensions_count": 2,
    "security_schemes": ["bearer", "oauth2"],
    "interfaces": ["https://agent.example.com/a2a"]
  }
}
```

### Validation Error Response (422)

```json
{
  "valid": false,
  "spec_version": "0.3",
  "errors": [
    {"field": "name", "message": "required field is missing"},
    {"field": "skills[1].id", "message": "required field is missing"},
    {"field": "extensions[0].uri", "message": "required field is missing"}
  ],
  "warnings": []
}
```

### Processing Flow

1. Parse JSON (syntax check)
2. Detect spec version
3. Validate required fields -> collect errors
4. Validate deprecations -> collect warnings
5. If no errors -> build preview from parsed data
6. Return structured result

---

## 4. Dashboard UI — Register Agent Modal

### Trigger

"Register Agent" button in `CatalogList.tsx` header (next to filters).

### Modal Flow (shadcn/ui Dialog + Tabs)

#### Step 1: Input
- Two tabs: "Paste JSON" | "Upload File"
- Paste: `<Textarea>` with monospace font, placeholder with example A2A card
- Upload: drag & drop zone, accepts `.json`
- Client-side: JSON syntax validation (instant). Inline error under textarea if invalid
- "Validate" button -> calls `POST /api/v1/catalog/validate`

#### Step 2: Validation Results
- **Errors present:** red alert with field-level error list. "Back to Edit" button
- **Warnings only:** yellow alerts with warning list. "Continue to Preview" button
- **Clean:** auto-advance to Preview

#### Step 3: Preview
- Agent name + description
- Protocol version badge (`v0.3` / `v1.0`)
- Supported interfaces (URLs + binding type)
- Extensions list with `Required` / `Optional` badges
- Security schemes (type badges)
- Skills count with collapsible list
- Data sourced from `preview` object in `/validate` response

#### Step 4: Confirm

- "Register Agent" button -> `POST /api/v1/catalog` (existing endpoint, enhanced parser handles new fields) with original JSON
- Loading state on button
- Success -> toast notification + close modal + refresh catalog
- Error (e.g., 409 duplicate endpoint) -> inline error

### New Components

- `RegisterAgentDialog.tsx` — main modal with step logic
- `CardPreview.tsx` — reusable card preview (also used in EntryDetail)

### Changes to Existing Files

- `CatalogList.tsx` — add "Register Agent" button
- `api.ts` — new `validateAgentCard(json)` function
- `types.ts` — `ValidationResult`, `CardPreview` types

---

## 5. Entry Detail — Extensions & Security Display

### New Sections in EntryDetail.tsx

Added below Skills section, above Raw Card display.

#### Extensions Section
- Header: "Extensions" with count badge
- Each extension: row with `uri` + `Required` (red badge) / `Optional` (gray badge)
- Hidden when agent has no extensions

#### Security Schemes Section
- Header: "Security"
- Badge per scheme type: `Bearer`, `API Key`, `OAuth2`, `OIDC`, `mTLS`
- Tooltip with method/name details where available

#### Spec Version Badge
- Badge next to protocol badge in detail header: `v0.3` / `v1.0`
- Reuses `ProtocolBadge.tsx` pattern

### Data Source

`TypedMeta` from API response, filtered by `kind`:
- `a2a.extension` -> Extensions section
- `a2a.security_scheme` -> Security section
- `a2a.interface` -> Interfaces section

### Component Reuse

`CardPreview.tsx` from registration modal reused in read-only mode for the detail view.

---

## 6. Database Migration & Serialization

### Migration 005

```sql
ALTER TABLE catalog_entries ADD COLUMN spec_version TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_entries ADD COLUMN typed_meta TEXT NOT NULL DEFAULT '[]';
```

### DB Storage Format

JSON array with `kind` discriminator:

```json
[
  {"kind": "a2a.extension", "uri": "urn:example:ext", "required": true},
  {"kind": "a2a.security_scheme", "type": "bearer", "method": "header", "name": "Authorization"},
  {"kind": "a2a.interface", "url": "https://agent.example.com/a2a", "binding": "jsonrpc"}
]
```

### Serialization

`SyncToDB()` / `SyncFromDB()` extended:
- `SyncToDB`: serializes `[]TypedMetadata` -> JSON string with `kind` discriminator per entry
- `SyncFromDB`: deserializes JSON -> concrete types via registry pattern (`map[string]func() TypedMetadata`)

### Backward Compatibility

- Existing entries get `spec_version=""` and `typed_meta="[]"` — zero impact
- Parser repopulates data on next crawl cycle
- API response includes `typed_meta` only when non-empty (`omitempty`)

---

## 7. Testing Strategy

### Go Unit Tests

- **Parser tests** — table-driven: v0.3 card, v1.0 card, legacy card (only `url`), card with missing fields, card with deprecated fields. Verify: correct version detection, typed metadata extraction, error collection, warning generation.
- **Validation endpoint tests** — valid card -> 200, invalid card -> 422 with error list, mixed -> 200 with warnings. Tested via httptest (existing handler test pattern).
- **TypedMetadata serialization** — round-trip: struct -> JSON -> struct. Registry deserialization with `kind` discriminator.
- **Migration test** — existing entries work after migration (backward compat).

### E2E Playwright Tests

- **Full registration flow** — paste JSON -> validate -> preview -> confirm -> agent visible in catalog
- **Validation errors** — paste broken JSON -> errors visible -> fix -> validate -> success
- **File upload** — upload .json file -> validate -> preview
- **Extensions display** — register agent with extensions -> open detail -> extensions visible with badges
- **Security schemes display** — same pattern for security schemes

### Test Fixtures

- `testdata/a2a_v03_card.json` — full v0.3 card
- `testdata/a2a_v10_card.json` — full v1.0 card
- `testdata/a2a_legacy_card.json` — card with only `url` (no `supportedInterfaces`)
- `testdata/a2a_invalid_card.json` — missing required fields

---

## Scope Boundaries

### In Scope (Tier 1 MVP)
- A2A parser enhancement (v0.3 + v1.0)
- Typed metadata model + DB migration
- Validation endpoint (dry-run)
- Register Agent modal (paste + upload + preview + confirm)
- Extensions & security display in Entry Detail
- Unit + E2E tests

### Out of Scope
- MCP/a2ui typed metadata implementations (future: same pattern)
- Reachability check implementation (existing health plugin handles this)
- Agent card editing/updating via UI
- Batch import of multiple cards
- Signature verification (store only, display in Tier 2)

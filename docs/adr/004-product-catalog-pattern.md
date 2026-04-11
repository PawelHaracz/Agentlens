# ADR-004: Product Catalog Pattern for Agent Domain Modeling

Date: 2026-04-11
Status: Accepted
Related: [ADR-001](001-product-archetype-principles.md) (technical mapping), [ADR-003](003-microkernel-plugin-architecture.md) (complementary pattern)

## Context

AgentLens catalogs AI agents across multiple protocols (A2A, MCP, future protocols). Each protocol defines different capabilities: A2A agents have skills, security schemes, interfaces, extensions, and signatures; MCP agents have tools, resources, and prompts. The domain model must:

1. **Represent heterogeneous agents uniformly** — a single catalog view over agents with fundamentally different capability sets.
2. **Extend to new protocols without schema changes** — adding MCP prompts or a future protocol's capabilities must not require database migrations or changes to the API response format.
3. **Support structured querying** — "find all agents with an `mcp.tool` named `search`" or "which agents require OAuth 2.0" must be efficient queries, not full-table JSON scans.
4. **Separate identity from presentation** — the same agent type (protocol + endpoint + capabilities) may appear in multiple catalog views with different display names, categories, or validity windows.

A naive approach (one table per protocol, or capabilities as a JSON blob on the agent row) breaks under these requirements.

## Decision

Model the agent catalog using the **Product Archetype pattern** from [softwarearchetypes/product](https://github.com/softwarearchetypes), translated to Go idioms.

### Core separation

- **AgentType** (= Product Type) — what the agent IS: protocol, endpoint, version, capabilities. Identity is `AgentKey = SHA256(protocol + endpoint)`.
- **CatalogEntry** (= Catalog Entry) — how the agent is offered: display name, description, categories, lifecycle state, health, validity window. 1:1 FK to AgentType.
- **Capability** (= Product Feature) — polymorphic interface with `Kind()` discriminator. Each protocol defines its own capability kinds. All stored in a single `capabilities` table with promoted `name`/`description` columns + `properties` JSON for kind-specific fields.

### How the two architectural patterns complement each other

The **microkernel** (ADR-003) and **product catalog** patterns address orthogonal concerns:

| Concern | Pattern | Role |
|---------|---------|------|
| **How new capabilities enter the system** | Microkernel (plugins) | Parser plugins convert protocol-specific cards into `AgentType` + `[]Capability` |
| **How capabilities are structured, stored, and queried** | Product Catalog (archetype) | Polymorphic `Capability` interface + normalized storage + registry-based deserialization |

A parser plugin (microkernel extension point) produces domain objects (product catalog entities). The capability registry (product catalog infrastructure) validates and deserializes what plugins produce. Neither pattern works alone:

- Without the microkernel: adding a new protocol means modifying the core parsing logic.
- Without the product catalog: each plugin would define its own storage schema, breaking uniform querying and API responses.

## Use Cases

### 1. Multi-protocol catalog view

An operator lists all agents: A2A agents show skills and security schemes, MCP agents show tools and resources. The API returns a uniform JSON array where each entry has a `capabilities` array with `kind`-discriminated objects. The frontend filters by `kind` prefix (`a2a.*` vs `mcp.*`) — no protocol-specific API endpoints needed.

### 2. Capability-based discovery

A developer searches "which agents offer an MCP tool named `code-review`?" This is a query on the `capabilities` table: `WHERE kind = 'mcp.tool' AND name = 'code-review'`, joined back to `catalog_entries` for the full listing. The promoted `name` column enables indexed lookups without JSON parsing.

### 3. Adding a new capability kind

MCP adds a "sampling" capability in a future spec version. Implementation: define `MCPSampling` struct implementing `Capability`, register it with `RegisterCapability("mcp.sampling", ...)` in an `init()` function, update the MCP parser plugin to extract it. No database migration — the `capabilities` table already handles arbitrary kinds via the `kind`+`properties` columns.

### 4. Security scheme modeling without parallel storage

Security schemes are complex (OAuth 2.0 flows with scopes, mutual TLS, API keys in different locations). Rather than a separate `security_details` table, they live in the existing `capabilities` table as `a2a.security_scheme` kind entries. The `A2ASecurityScheme` struct uses a union pattern with a `Type` discriminator for sub-variants (oauth2, apiKey, http, openIdConnect, mutualTls). Nested structures like `OAuthFlows` serialize into the `properties` JSON column. This keeps all capability data in one table with one query path.

### 5. Catalog presentation independence

An internal catalog entry shows an agent as "Translation Service (APAC)" while an external catalog entry for the same agent shows "AI Translator". Both reference the same `AgentType` with the same capabilities. `CatalogEntry` handles display concerns; `AgentType` handles identity and capabilities. This separation is enforced by reflection-based tests (`archetype_test.go`) that verify field ownership between the two structs.

### 6. Full-text search across capabilities

An operator searches "authentication" across the catalog. The `description` column on the `capabilities` table participates in search queries directly, finding agents whose security schemes, skills, or tools mention authentication — without deserializing JSON blobs.

## Consequences

### Positive

- Single `capabilities` table serves all protocols and all capability kinds — no N+1 tables problem.
- New capability kinds require zero database migrations.
- Promoted `name` and `description` columns enable indexed queries and full-text search.
- CatalogEntry `MarshalJSON()` flattens AgentType fields into a clean REST response — API consumers see a simple flat object.
- The registry + factory pattern gives type-safe deserialization with graceful handling of unknown kinds (skip, don't crash).

### Negative / Trade-offs

- The `properties` JSON column means some capability fields are not individually indexable. If a future query pattern requires filtering by a field inside `properties` (e.g., "all agents using OAuth 2.0 authorization code flow"), it requires either promoting that field to a column (migration) or scanning JSON.
- Union structs (like `A2ASecurityScheme` with all sub-variant fields) trade Go type safety for serialization simplicity. A capability can have `APIKeyLocation` set even when `Type` is `oauth2` — runtime validation (`Validate()`) catches this, not the compiler.
- Full-replacement updates (delete-all + re-insert capabilities) are simple but generate more write I/O than diff-based updates. Acceptable at current scale.

### Neutral

- The pattern originates from retail/product management domains. The mapping to AI agent catalogs is natural but not obvious — new contributors need to understand the archetype to understand why capabilities are on `AgentType` and not `CatalogEntry`.
- ADR-001 documents the detailed Go translation of each archetype principle. This ADR documents only the rationale for choosing the pattern and how it interacts with the microkernel.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **One table per protocol** (`a2a_skills`, `mcp_tools`, etc.) | Breaks uniform querying; adding a protocol requires migrations; N+1 tables for N capability kinds |
| **Capabilities as JSON blob on AgentType row** | Cannot index by kind or name; full-text search requires JSON parsing; no unique constraints on (agent, kind, name) |
| **Separate domain models per protocol** | Duplicates CRUD logic, API endpoints, and storage code for each protocol; discovery/listing requires protocol-aware union queries |
| **EAV (Entity-Attribute-Value) model** | Flexible but query-hostile; reconstructing a capability requires N rows; no type safety on attribute names |
| **GraphQL schema-per-protocol** | Shifts complexity to the API layer; still needs a unified storage model underneath; premature for current scale |

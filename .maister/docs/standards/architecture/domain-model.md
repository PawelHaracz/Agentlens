# Architecture: Domain Model (Product Archetype)

AgentLens models the agent catalog using the Product Archetype pattern. `AgentType` = what an agent *is* (protocol + endpoint identity + capabilities). `CatalogEntry` = how it is *offered*. `Capability` is polymorphic, discriminated by `kind`.

### Capabilities Belong to AgentType

All protocol-specific features are **capabilities** — skills, tools, security schemes, extensions, signatures, resources, prompts. Capabilities attach to `AgentType`, not `CatalogEntry`. They are stored in the single `capabilities` table (no parallel per-kind tables).

New capability kinds are added by:

1. Defining a struct that implements the `Capability` interface.
2. Registering the kind in `init()`.

Sub-variants within a kind use the **union-struct pattern** discriminated by a `Type` field, not separate Go types/tables.

Sources: ADR-001, ADR-004.

### Computed View Fields Are Not Stored

Derived/summary fields used in API responses (for example `auth_summary`, `security_detail`) are computed at serialization time from the underlying capabilities. They are not stored in parallel columns or tables — the capabilities are the single source of truth.

Sources: ADR-001, `docs/architecture.md`.

### Full Replacement on Capability Update

When capabilities change for an agent, **delete all existing capability rows** for that agent and **insert all new ones** — never merge or diff. This matches the archetype's immutable-aggregate pattern and keeps discovery idempotent.

Sources: ADR-001, ADR-008.

### Discovery Upsert by Endpoint

Discovery upserts entries keyed by `endpoint`, which carries a UNIQUE constraint. The identity hash is:

```
AgentKey = SHA256(protocol + endpoint)
```

Sources: `CLAUDE.md`, ADR-008.

### SourcePush Entries Are Protected From Discovery

Entries with `SourcePush` (created via REST API) must **never** be overwritten by discovery cycles. Discovery skips them entirely, preserving externally managed entries against clobber from k8s/static sources.

Source: ADR-008.

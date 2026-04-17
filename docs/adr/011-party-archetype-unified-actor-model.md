# ADR-011: Party Archetype as Unified Actor Model

Date: 2026-04-14
Status: Accepted

## Context

AgentLens requires a multi-party access control model: users must be groupable into
hierarchical groups, catalog entries must be scopeable to projects, and both users and
groups must be assignable to projects with specific roles.

A naïve approach would introduce separate `Group`, `Project`, and `User` tables with
independent join tables connecting them. This works for the initial feature set but
fragments as new actor types are added — each new type requires new tables, new
permission resolution paths, and new API handlers.

The Java [party archetype](~/src/private/archetypes/party) provides a proven pattern for
this class of problem: a unified `Party` abstraction (persons and organisations) connected
by named, directed `PartyRelationship` edges. This creates a typed graph of actors and
their relationships instead of a proliferation of per-type join tables.

## Decision

Model all actors in AgentLens (users, groups, projects, and any future types) as `Party`
entities stored in a single `parties` table, discriminated by a `kind` field. Actor
relationships — group membership, project membership — are stored as directed named edges
in a `party_relationships` table, following the party archetype pattern.

**Key mapping from Java archetype to Go:**

- `Person` → `Party{Kind: "person"}`, linked 1:1 to existing `User` via `user_id` FK
- `OrganizationUnit` → `Party{Kind: "group"}`, hierarchical via `group_member` relationships
- `Company` → `Party{Kind: "project"}`, logical namespace scoping catalog entries
- `PartyRelationship(from: PartyRole, to: PartyRole, name)` → `PartyRelationship` struct

**Catalog scoping:** `CatalogEntry` relates to `Party{Kind: "project"}` via a
`catalog_project_memberships` join table (many-to-many). New entries auto-assign to the
system `default` project.

**Extensibility:** New party kinds (e.g. `organization`) require only a new `PartyKind`
constant, an entry in `model.ContainmentRelationships` for hierarchical kinds, an entry in
`model.ValidPartyKinds`, and a `PartyKindConfig` registration for the generic handler —
no schema changes, no new handler code, no new store methods.

**Existing `User`, `Role`, and global permission system are unchanged.** The party layer is
additive — `User.RoleID` continues to carry global permissions; `Party` carries
project-scoped relationships on top.

## Consequences

### Positive

- Single `parties` table and single `party_relationships` table serve all actor types and
  relationships — no table proliferation as new kinds are introduced
- Generic `PartyKindConfig`-driven API handler: adding a new party kind costs ~10 lines of
  config, zero handler code
- Full ArgoCD-style project scoping: catalog entries scoped per project, per-project RBAC,
  global role bypass for admins
- Hierarchical groups supported natively through the relationship graph
- Backward compatible: all existing endpoints and auth flows unchanged

### Negative / Trade-offs

- Graph model adds conceptual overhead compared to simple join tables — developers must
  understand `PartyRelationship` and the `ContainmentRelationships` registry
- Person parties must be kept in sync with `User` records (created together, never orphaned)
- `party_group_closures` must be rebuilt on every group membership change (mitigated by ADR-012)

### Neutral

- Go lacks sealed types; the `kind` discriminator replaces Java's sealed class hierarchy
- `PartyIdentifier` (RegisteredIdentifier mapping) schema is added in v1 but not populated
  until v2 — extends the archetype without blocking the initial feature

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| Separate `Group` + `Project` tables with per-type join tables (Option C) | Fragments permission resolution across multiple code paths; each new actor type requires schema + handler + store changes |
| Pragmatic hybrid: Party for actors, Project as separate entity (Option B) | Cleaner separation but loses the graph's ability to relate any two parties; extensibility requires structural changes |
| Full graph with Projects as Party kind (Option A — chosen) | Maximum flexibility; single resolution algorithm; extensible by config |

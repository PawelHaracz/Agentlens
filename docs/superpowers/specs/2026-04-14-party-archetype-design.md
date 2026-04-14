# Party Archetype — Design Spec

**Date:** 2026-04-14
**Status:** Approved
**Scope:** Users, groups, projects, project-scoped permissions, catalog entry project membership

---

## 1. Problem Statement

AgentLens currently has a flat, global permission model: every user has one global role
(`admin`, `editor`, `viewer`) and all catalog entries are visible to all authenticated users.
There is no concept of projects, group membership, or per-project permissions.

The goals of this feature:

- Introduce **projects** as logical namespaces (à la ArgoCD AppProject) that scope catalog entries
- Introduce **groups** as hierarchical collections of users that can carry project roles
- Allow **catalog entries** to belong to multiple projects simultaneously
- Enforce **project-scoped RBAC** in addition to existing global roles
- Model all actors (users, groups, future org types) using the **Party archetype** as a unified abstraction
- Keep the system **open to new party kinds** (e.g. organization) with zero handler code changes

---

## 2. Source Archetype — Java Mapping

The [party archetype](~/src/private/archetypes/party) defines:

| Java concept | AgentLens Go mapping |
|---|---|
| `Party` (sealed abstract) | `Party` struct + `PartyKind` discriminator |
| `Person` | `Party{Kind: "person"}` linked to existing `User` (auth lives in `User`) |
| `Organization` / `OrganizationUnit` | `Party{Kind: "group"}` — nested via relationships |
| `Company` | `Party{Kind: "project"}` — logical namespace |
| `Role` (value object) | plain `string` — no separate table |
| `PartyRole` | `(PartyID, RoleName)` pair — embedded in relationship edges |
| `PartyRelationship` | `PartyRelationship` struct — directed named graph edge |
| `RegisteredIdentifier` | `PartyIdentifier` struct — external IDs (email, username) |

Go does not have sealed types. The discriminator pattern (`kind` field + validation in store
layer) replaces Java's sealed class hierarchy.

---

## 3. Core Domain Model

### 3.1 Party

```go
type PartyKind string

const (
    PartyKindPerson  PartyKind = "person"
    PartyKindGroup   PartyKind = "group"
    PartyKindProject PartyKind = "project"
    // Future: PartyKindOrganization PartyKind = "organization"
)

type Party struct {
    ID        string    `gorm:"primaryKey"`
    Kind      PartyKind `gorm:"not null"`
    Name      string    `gorm:"not null"`
    Version   int       `gorm:"not null;default:0"`
    UserID    *string   `gorm:"uniqueIndex"` // non-null for persons only, FK→users.id
    IsSystem  bool      `gorm:"default:false"` // true for default project / system groups
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- **Person** parties are created automatically when a `User` is created (migration + bootstrap).
- **Group** parties are user-managed. Groups can contain persons or other groups (hierarchical).
- **Project** parties are user-managed namespaces. One system project (`is_system=true`,
  `name="default"`) is seeded at migration time.

### 3.2 PartyRelationship (graph edge)

```go
type PartyRelationship struct {
    ID               string `gorm:"primaryKey"`
    FromPartyID      string `gorm:"not null;index"`
    FromRole         string `gorm:"not null"`
    ToPartyID        string `gorm:"not null;index"`
    ToRole           string `gorm:"not null"`
    RelationshipName string `gorm:"not null"`
    CreatedAt        time.Time
    // UNIQUE(FromPartyID, FromRole, ToPartyID, ToRole, RelationshipName)
}
```

**Relationship taxonomy:**

| `RelationshipName` | `FromRole` | `ToRole` | Meaning |
|---|---|---|---|
| `group_member` | `member` | `group` | Person or group is a member of a group |
| `project_member` | `project:owner` \| `project:developer` \| `project:viewer` | `project` | Party has a role within a project |

New party kinds add a new row to this taxonomy. No schema change required.

### 3.3 Party Group Closure (materialized)

```go
type PartyGroupClosure struct {
    MemberPartyID   string `gorm:"primaryKey"`
    AncestorPartyID string `gorm:"primaryKey"`
}
```

Pre-computed transitive closure of **all containment relationships** (see §3.4).
Rebuilt synchronously (in the same transaction) whenever a containment relationship
is added or removed. Enables O(1) indexed auth checks — no recursive CTE at request time.

### 3.4 Containment Relationship Registry

```go
// ContainmentRelationships is the set of relationship names that form
// hierarchical containment and trigger closure rebuilds.
// Adding a new hierarchical party kind costs exactly one entry here.
var ContainmentRelationships = map[string]bool{
    "group_member": true,
    // "org_member": true  ← future: adding "organization" kind costs one line
}
```

The closure rebuild fires for any relationship whose name is in `ContainmentRelationships`.
`PartyGroupClosure` stores `(member_party_id, ancestor_party_id)` regardless of kind —
the permission resolution algorithm queries it without filtering by kind.

### 3.5 Global Party Roles (groups only)

```go
type GlobalPartyRole struct {
    PartyID string `gorm:"primaryKey"` // kind=group (or future hierarchical kinds)
    RoleID  string `gorm:"primaryKey"` // FK→roles.id
}
```

Person parties derive their global role from `users.role_id` (unchanged). Groups can
additionally carry global roles (e.g. a `platform-admins` group with `global:admin`).

### 3.6 Catalog Project Membership

```go
type CatalogProjectMembership struct {
    CatalogEntryID string `gorm:"primaryKey"` // FK→catalog_entries.id
    ProjectPartyID string `gorm:"primaryKey"` // FK→parties.id, kind=project
    CreatedAt      time.Time
}
```

Many-to-many. Every new `CatalogEntry` is automatically inserted into the `default` project
at creation time. Additional project assignments are explicit.

### 3.7 Party Identifier (v2)

```go
type PartyIdentifier struct {
    ID        string `gorm:"primaryKey"`
    PartyID   string `gorm:"not null;index"`
    Kind      string `gorm:"not null"` // "email", "username", "external_id"
    Value     string `gorm:"not null"`
    CreatedAt time.Time
    // UNIQUE(Kind, Value)
}
```

Schema added in v1 migration, not populated until v2. Included for archetype completeness.

---

## 4. Permission Resolution

### 4.1 Project-scoped permission matrix

| Project role | Permissions granted within project |
|---|---|
| `project:owner` | `catalog:read`, `catalog:write`, `catalog:delete` |
| `project:developer` | `catalog:read`, `catalog:write` |
| `project:viewer` | `catalog:read` |

Static in-memory map — no DB table. New project roles = one map entry in `auth/party_permissions.go`.

### 4.2 Resolution algorithm

```text
HasProjectPermission(ctx, userID, permission, projectPartyID) bool:

  Step 1 — Global bypass (existing system, unchanged)
    user.Role.Permissions ∋ permission → ALLOW

  Step 2 — Group global roles
    SELECT r.permissions FROM global_party_roles gpr
    JOIN roles r ON r.id = gpr.role_id
    WHERE gpr.party_id IN (
      SELECT ancestor_party_id FROM party_group_closure
      WHERE member_party_id = $userPartyID
    )
    any permissions set ∋ permission → ALLOW

  Step 3 — Project-scoped roles (user direct + via group ancestry)
    SELECT from_role FROM party_relationships
    WHERE relationship_name = 'project_member'
      AND to_party_id = $projectPartyID
      AND from_party_id IN (
            $userPartyID,
            SELECT ancestor_party_id FROM party_group_closure
            WHERE member_party_id = $userPartyID
          )
    map from_role → permission set (static in-memory table)
    any mapped permissions ∋ permission → ALLOW

  Step 4 — DENY
```

All lookups use indexed columns. No recursive CTE at auth time (closure is pre-materialized).

### 4.3 Catalog entry access

```text
HasCatalogPermission(ctx, userID, permission, catalogEntryID) bool:
  projectIDs = SELECT project_party_id FROM catalog_project_memberships
               WHERE catalog_entry_id = $catalogEntryID
  return ANY projectID: HasProjectPermission(ctx, userID, permission, projectID)
```

### 4.4 Per-request context cache

`RequireProjectPermission` middleware caches the user's ancestor party IDs (from
`party_group_closure`) in `r.Context()` on the first call per request. Subsequent
middleware calls on the same request reuse the cached set — single DB round-trip
per request for group ancestry resolution.

---

## 5. API Design

### 5.1 Generic handler architecture

All party kinds share a single handler implementation, parameterized by `PartyKindConfig`:

```go
// PartyKindConfig drives route registration and handler behaviour for one party kind.
// Adding a new party kind = add a new PartyKindConfig. Zero handler code changes.
type PartyKindConfig struct {
    Kind               PartyKind   // "group", "project", "organization"
    URLPrefix          string      // "groups", "projects", "orgs"
    MemberRelationship string      // relationship_name for members: "group_member", "project_member"
    ValidMemberRoles   []string    // roles a member may hold in this kind
    CreatePermission   string      // global permission required to create
    ManagePermission   string      // global permission required to delete / modify
    CanContainKinds    []PartyKind // which party kinds may be added as members
}
```

Route registration in `router.go`:

```go
RegisterPartyKindRoutes(r, PartyKindConfig{
    Kind: PartyKindGroup, URLPrefix: "groups",
    MemberRelationship: "group_member",
    ValidMemberRoles:   []string{"member"},
    CreatePermission:   "users:write", ManagePermission: "users:write",
    CanContainKinds:    []PartyKind{PartyKindPerson, PartyKindGroup},
})
RegisterPartyKindRoutes(r, PartyKindConfig{
    Kind: PartyKindProject, URLPrefix: "projects",
    MemberRelationship: "project_member",
    ValidMemberRoles:   []string{"project:owner", "project:developer", "project:viewer"},
    CreatePermission:   "catalog:write", ManagePermission: "catalog:write",
    CanContainKinds:    []PartyKind{PartyKindPerson, PartyKindGroup},
})
// Future: RegisterPartyKindRoutes(r, PartyKindConfig{Kind: PartyKindOrganization, ...})
```

### 5.2 Generic endpoints (registered per kind)

```text
GET    /api/v1/{prefix}                         ListParties(cfg)
POST   /api/v1/{prefix}                         CreateParty(cfg)
GET    /api/v1/{prefix}/{id}                    GetParty(cfg)
DELETE /api/v1/{prefix}/{id}                    DeleteParty(cfg)   [non-system only]
POST   /api/v1/{prefix}/{id}/members            AddMember(cfg)
DELETE /api/v1/{prefix}/{id}/members/{memberID} RemoveMember(cfg)
GET    /api/v1/{prefix}/{id}/members            ListMembers(cfg)
```

### 5.3 Catalog scoping endpoints (catalog-specific, not generic)

```text
POST   /api/v1/catalog/{id}/projects             assign entry to additional project
DELETE /api/v1/catalog/{id}/projects/{projectID} remove from project
GET    /api/v1/catalog?project={projectID}        filter catalog by project
```

### 5.4 New middleware

```go
// RequireProjectPermission is project-aware auth middleware.
// Reads projectID from chi URL param, resolves permission via closure,
// caches ancestor parties in context for the request lifetime.
func RequireProjectPermission(projectIDParam, permission string) func(http.Handler) http.Handler
```

Existing `RequirePermission` middleware (global) is unchanged and continues to guard all
existing endpoints.

---

## 6. Layer Boundaries (arch-go compliant)

```text
internal/model/
  party.go              Party, PartyKind, PartyRelationship, PartyGroupClosure,
                        GlobalPartyRole, CatalogProjectMembership, PartyIdentifier

internal/store/
  party_store.go        PartyStore — generic CRUD for parties, relationships,
                        closure rebuild (same tx as relationship changes)
                        Methods: CreateParty, AddMember, RemoveMember, ListMembers,
                                 GetParty, DeleteParty, ListParties

internal/auth/
  party_permissions.go  HasProjectPermission, HasCatalogPermission,
                        ContainmentRelationships (registry),
                        projectRolePermissions (static in-memory map)

internal/api/
  party_handlers.go     Generic CRUD + member handlers (parameterized by PartyKindConfig)
  party_kind_configs.go PartyKindConfig definitions for group and project
  party_middleware.go   RequireProjectPermission + context cache
  catalog_project.go    Catalog↔project scoping handlers (POST/DELETE/GET)

internal/db/
  migrations.go         Migration 007: all party tables + seed data
```

No new arch-go dependency rules required. All placements fit existing layer contracts.

---

## 7. Extensibility — Adding a New Party Kind

Adding `organization` (or any future kind) costs:

| Change | Location | Cost |
|---|---|---|
| `PartyKindOrganization` constant | `model/party.go` | 1 line |
| `"org_member"` in `ContainmentRelationships` | `auth/party_permissions.go` | 1 line |
| Kind validation entry | `store/party_store.go` | 1 line |
| `PartyKindConfig{...}` registration | `api/party_kind_configs.go` | ~10 lines |
| Migration | none — `parties` table already has `kind` discriminator | 0 lines |

Permission resolution, closure rebuild, store methods, and all handlers — **unchanged**.

---

## 8. Migration Strategy

**Migration 007** — next in sequence after `migration006RawCards` (idempotent):

```text
1. CREATE TABLE parties (id, kind, name, version, user_id, is_system, ...)
2. CREATE TABLE party_relationships (id, from_party_id, from_role, to_party_id, to_role, relationship_name)
3. CREATE TABLE party_group_closure (member_party_id, ancestor_party_id)
4. CREATE TABLE global_party_roles (party_id, role_id)
5. CREATE TABLE catalog_project_memberships (catalog_entry_id, project_party_id)
6. CREATE TABLE party_identifiers (id, party_id, kind, value)   [schema only, not populated]
7. INSERT INTO parties: default project (kind='project', is_system=true, name='default')
8. INSERT INTO parties: one Person party per existing User (kind='person', user_id=u.id)
9. INSERT INTO catalog_project_memberships: all existing CatalogEntries → default project
```

**Backward compatibility:**

- All existing API endpoints work unchanged after migration
- `?project=` query param on `GET /api/v1/catalog` is opt-in; omitting it returns all entries
  the caller has global access to (existing behaviour preserved)
- `RequireProjectPermission` only guards new endpoints; existing endpoints keep `RequirePermission`
- Bootstrap admin creation gains a corresponding Person party automatically

---

## 9. Out of Scope (v1)

- Party addresses (from archetype) — not relevant to AgentLens
- `PartyIdentifier` population — schema added, populated in v2
- Invitation / approval flows for project membership
- Project-level source/destination restrictions
- Audit log for membership changes (enterprise plugin concern)
- UI for project/group management (backend API only in v1)

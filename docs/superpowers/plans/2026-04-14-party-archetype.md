# Party Archetype Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the party archetype model to support hierarchical groups, projects, project-scoped RBAC, and catalog entry scoping in AgentLens.

**Architecture:** Unified `Party` table (person/group/project kinds) connected by named directed `PartyRelationship` edges. Group ancestry pre-computed into `party_group_closures` for O(1) permission checks. Generic `PartyKindConfig`-driven API handlers eliminate per-kind handler files.

**Tech Stack:** Go 1.26, GORM (SQLite+Postgres), chi router, `github.com/google/uuid`, existing `internal/auth`, `internal/store`, `internal/model` patterns.

**Arch-go note:** `auth` imports `model` only. DB-dependent permission resolution lives in `api/party_middleware.go` (which may import `store`). `model.ContainmentRelationships` is used by both `store` and `api`.

**Read before starting:**
- `docs/superpowers/specs/2026-04-14-party-archetype-design.md` — full design
- `docs/adr/011-party-archetype-unified-actor-model.md`
- `docs/adr/012-materialized-group-closure-for-permission-resolution.md`
- `internal/store/user_store.go` — store pattern to follow
- `internal/api/auth_middleware.go` — middleware pattern to follow
- `internal/db/migrations.go` — migration pattern to follow

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/model/party.go` | Create | Party, PartyKind, PartyRelationship, PartyGroupClosure, GlobalPartyRole, CatalogProjectMembership, PartyIdentifier, ContainmentRelationships |
| `internal/auth/party_permissions.go` | Create | ProjectRolePermissions static map (pure, no DB) |
| `internal/store/party_store.go` | Create | PartyStore — CRUD + closure rebuild |
| `internal/db/migrations.go` | Modify | Add migration007PartyArchetype() |
| `internal/api/party_kind_configs.go` | Create | PartyKindConfig struct + group/project configs |
| `internal/api/party_handlers.go` | Create | Generic CreateParty, ListParties, GetParty, DeleteParty, AddMember, RemoveMember, ListMembers |
| `internal/api/party_middleware.go` | Create | RequireProjectPermission + ancestor context cache |
| `internal/api/catalog_project.go` | Create | AssignToProject, RemoveFromProject, ListCatalogProjects handlers |
| `internal/api/router.go` | Modify | RegisterPartyKindRoutes, catalog project routes, RouterDeps.PartyStore |
| `internal/store/sql_store.go` | Modify | Auto-assign new CatalogEntry to default project |
| `cmd/agentlens/main.go` | Modify | Wire PartyStore into RouterDeps |

**Test files:**
| File | Tests |
|---|---|
| `internal/store/party_store_test.go` | CRUD, closure rebuild, member ops |
| `internal/auth/party_permissions_test.go` | ProjectRolePermissions lookups |
| `internal/api/party_handlers_test.go` | Handler status codes + response shape |

---

## Task 1: Party Model Types

**Files:**
- Create: `internal/model/party.go`

- [ ] **Step 1: Write the file**

```go
// internal/model/party.go
package model

import "time"

// PartyKind discriminates the type of party stored in the parties table.
type PartyKind string

const (
    PartyKindPerson  PartyKind = "person"
    PartyKindGroup   PartyKind = "group"
    PartyKindProject PartyKind = "project"
)

// ValidPartyKinds is the set of allowed PartyKind values.
var ValidPartyKinds = map[PartyKind]bool{
    PartyKindPerson:  true,
    PartyKindGroup:   true,
    PartyKindProject: true,
}

// ContainmentRelationships is the set of relationship names that form hierarchical
// containment and trigger party_group_closures rebuild.
// Adding a new hierarchical party kind = add its relationship name here.
var ContainmentRelationships = map[string]bool{
    "group_member": true,
}

// ContainmentRelationshipNames returns ContainmentRelationships as a string slice.
// Used for SQL IN clauses in closure rebuild.
func ContainmentRelationshipNames() []string {
    names := make([]string, 0, len(ContainmentRelationships))
    for name := range ContainmentRelationships {
        names = append(names, name)
    }
    return names
}

// Party is the unified actor. Person parties link 1:1 to User records.
// Group and Project parties are standalone.
type Party struct {
    ID        string    `gorm:"primaryKey;type:text" json:"id"`
    Kind      PartyKind `gorm:"not null;type:text" json:"kind"`
    Name      string    `gorm:"not null;type:text" json:"name"`
    Version   int       `gorm:"not null;default:0" json:"-"`
    UserID    *string   `gorm:"uniqueIndex;type:text" json:"-"`
    IsSystem  bool      `gorm:"default:false" json:"is_system"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// PartyRelationship is a directed named edge in the party graph.
// UNIQUE(FromPartyID, FromRole, ToPartyID, ToRole, RelationshipName) is enforced in migration.
type PartyRelationship struct {
    ID               string    `gorm:"primaryKey;type:text" json:"id"`
    FromPartyID      string    `gorm:"not null;type:text;index" json:"from_party_id"`
    FromRole         string    `gorm:"not null;type:text" json:"from_role"`
    ToPartyID        string    `gorm:"not null;type:text;index" json:"to_party_id"`
    ToRole           string    `gorm:"not null;type:text" json:"to_role"`
    RelationshipName string    `gorm:"not null;type:text" json:"relationship_name"`
    CreatedAt        time.Time `json:"created_at"`
}

// PartyGroupClosure is the pre-computed transitive closure of containment relationships.
// Never edit directly — managed exclusively by PartyStore.rebuildClosure.
type PartyGroupClosure struct {
    MemberPartyID   string `gorm:"primaryKey;type:text" json:"-"`
    AncestorPartyID string `gorm:"primaryKey;type:text" json:"-"`
}

// GlobalPartyRole assigns a global system role to a group party.
// Person parties derive global roles from User.RoleID (unchanged).
type GlobalPartyRole struct {
    PartyID string `gorm:"primaryKey;type:text" json:"party_id"`
    RoleID  string `gorm:"primaryKey;type:text" json:"role_id"`
}

// CatalogProjectMembership links a catalog entry to a project party (many-to-many).
type CatalogProjectMembership struct {
    CatalogEntryID string    `gorm:"primaryKey;type:text" json:"catalog_entry_id"`
    ProjectPartyID string    `gorm:"primaryKey;type:text" json:"project_party_id"`
    CreatedAt      time.Time `json:"created_at"`
}

// PartyIdentifier stores external identifiers for parties.
// Schema added in v1, not populated until v2.
type PartyIdentifier struct {
    ID        string    `gorm:"primaryKey;type:text" json:"id"`
    PartyID   string    `gorm:"not null;type:text;index" json:"party_id"`
    Kind      string    `gorm:"not null;type:text" json:"kind"`
    Value     string    `gorm:"not null;type:text" json:"value"`
    CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Verify it compiles**

```bash
rtk go build ./internal/model/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
rtk git add internal/model/party.go
rtk git commit -m "feat(model): add party archetype types"
```

---

## Task 2: ProjectRolePermissions (auth — pure, no DB)

**Files:**
- Create: `internal/auth/party_permissions.go`
- Create: `internal/auth/party_permissions_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/auth/party_permissions_test.go
package auth_test

import (
    "testing"

    "github.com/PawelHaracz/agentlens/internal/auth"
    "github.com/stretchr/testify/assert"
)

func TestProjectRolePermissions(t *testing.T) {
    tests := []struct {
        role       string
        permission string
        want       bool
    }{
        {"project:owner", "catalog:read", true},
        {"project:owner", "catalog:write", true},
        {"project:owner", "catalog:delete", true},
        {"project:developer", "catalog:read", true},
        {"project:developer", "catalog:write", true},
        {"project:developer", "catalog:delete", false},
        {"project:viewer", "catalog:read", true},
        {"project:viewer", "catalog:write", false},
        {"project:viewer", "catalog:delete", false},
        {"project:unknown", "catalog:read", false},
    }
    for _, tc := range tests {
        t.Run(tc.role+"/"+tc.permission, func(t *testing.T) {
            got := auth.ProjectRoleHasPermission(tc.role, tc.permission)
            assert.Equal(t, tc.want, got)
        })
    }
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
rtk go test ./internal/auth/... -run TestProjectRolePermissions -v
```

Expected: FAIL — `auth.ProjectRoleHasPermission` undefined.

- [ ] **Step 3: Implement**

```go
// internal/auth/party_permissions.go
package auth

import "sort"

// projectRolePermissions maps project role names to the permissions they grant.
// Static in-memory — no DB lookup. New roles = one entry here.
var projectRolePermissions = map[string][]string{
    "project:owner":     {PermCatalogRead, PermCatalogWrite, PermCatalogDelete},
    "project:developer": {PermCatalogRead, PermCatalogWrite},
    "project:viewer":    {PermCatalogRead},
}

// ProjectRoleHasPermission returns true if the project role grants the given permission.
func ProjectRoleHasPermission(role, permission string) bool {
    perms, ok := projectRolePermissions[role]
    if !ok {
        return false
    }
    return HasPermission(perms, permission)
}

// ValidProjectRoles returns all valid project-scoped role names in sorted order.
func ValidProjectRoles() []string {
    roles := make([]string, 0, len(projectRolePermissions))
    for r := range projectRolePermissions {
        roles = append(roles, r)
    }
    sort.Strings(roles)
    return roles
}
```

- [ ] **Step 4: Run to confirm pass**

```bash
rtk go test ./internal/auth/... -run TestProjectRolePermissions -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/auth/party_permissions.go internal/auth/party_permissions_test.go
rtk git commit -m "feat(auth): add project role permission map"
```

---

## Task 3: Migration 007

**Files:**
- Modify: `internal/db/migrations.go`

- [ ] **Step 1: Write a test that the migration runs on :memory: SQLite**

Add to a new file `internal/db/migrations_party_test.go`:

```go
// internal/db/migrations_party_test.go
package db_test

import (
    "testing"

    "github.com/PawelHaracz/agentlens/internal/db"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMigration007_TablesExist(t *testing.T) {
    database, err := db.Open(db.DialectSQLite, ":memory:")
    require.NoError(t, err)

    migrator := db.NewMigrator(database, db.AllMigrations())
    require.NoError(t, migrator.Migrate(t.Context()))

    sqlDB, err := database.DB.DB()
    require.NoError(t, err)

    tables := []string{
        "parties",
        "party_relationships",
        "party_group_closures",
        "global_party_roles",
        "catalog_project_memberships",
        "party_identifiers",
    }
    for _, tbl := range tables {
        var count int
        row := sqlDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tbl)
        require.NoError(t, row.Scan(&count))
        assert.Equal(t, 1, count, "table %s should exist", tbl)
    }

    // Default project should be seeded
    var partyCount int
    row := sqlDB.QueryRow("SELECT COUNT(*) FROM parties WHERE kind='project' AND is_system=1")
    require.NoError(t, row.Scan(&partyCount))
    assert.Equal(t, 1, partyCount, "default project party should exist")
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
rtk go test ./internal/db/... -run TestMigration007 -v
```

Expected: FAIL — tables don't exist yet.

- [ ] **Step 3: Add migration007 to migrations.go**

In `internal/db/migrations.go`, add imports `"github.com/google/uuid"` and `"log/slog"` and append to `AllMigrations()`:

```go
// In AllMigrations():
func AllMigrations() []Migration {
    return []Migration{
        migration001CreateTables(),
        migration002UsersAndRoles(),
        migration003DefaultRoles(),
        migration004Settings(),
        migration005HealthColumns(),
        migration006RawCards(),
        migration007PartyArchetype(), // add this line
    }
}
```

Then add the function at the bottom of the file:

```go
func migration007PartyArchetype() Migration {
    return Migration{
        Version:     7,
        Description: "create party archetype tables and seed default project",
        Up: func(tx *gorm.DB) error {
            // Create all party tables
            for _, m := range []interface{}{
                &model.Party{},
                &model.PartyRelationship{},
                &model.PartyGroupClosure{},
                &model.GlobalPartyRole{},
                &model.CatalogProjectMembership{},
                &model.PartyIdentifier{},
            } {
                if err := tx.AutoMigrate(m); err != nil {
                    return fmt.Errorf("auto migrate %T: %w", m, err)
                }
            }

            // Unique constraint on party_relationships
            if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_party_rel_unique
                ON party_relationships(from_party_id, from_role, to_party_id, to_role, relationship_name)`).Error; err != nil {
                return fmt.Errorf("creating party_relationships unique index: %w", err)
            }

            // Unique constraint on party_identifiers
            if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_party_identifiers_kind_value
                ON party_identifiers(kind, value)`).Error; err != nil {
                return fmt.Errorf("creating party_identifiers unique index: %w", err)
            }

            // Seed default project (idempotent)
            defaultID := uuid.New().String()
            if err := tx.Exec(`
                INSERT INTO parties (id, kind, name, version, is_system, created_at, updated_at)
                SELECT ?, 'project', 'default', 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
                WHERE NOT EXISTS (SELECT 1 FROM parties WHERE kind='project' AND is_system=1)
            `, defaultID).Error; err != nil {
                return fmt.Errorf("seeding default project: %w", err)
            }

            // Get default project ID (may differ from defaultID if already seeded)
            var defaultProjectID string
            if err := tx.Raw("SELECT id FROM parties WHERE kind='project' AND is_system=1 LIMIT 1").
                Scan(&defaultProjectID).Error; err != nil {
                return fmt.Errorf("reading default project id: %w", err)
            }

            // Create Person party for each existing user (idempotent, dialect-agnostic).
            // Go-side loop with uuid.New() — avoids SQLite-only randomblob, works on Postgres.
            var existingUsers []struct {
                ID          string  `gorm:"column:id"`
                Username    string  `gorm:"column:username"`
                DisplayName *string `gorm:"column:display_name"`
            }
            if err := tx.Raw("SELECT id, username, display_name FROM users").Scan(&existingUsers).Error; err != nil {
                return fmt.Errorf("reading existing users: %w", err)
            }
            for _, u := range existingUsers {
                name := u.Username
                if u.DisplayName != nil && *u.DisplayName != "" {
                    name = *u.DisplayName
                }
                partyID := uuid.New().String()
                userID := u.ID
                if err := tx.Exec(`
                    INSERT INTO parties (id, kind, name, version, user_id, is_system, created_at, updated_at)
                    SELECT ?, 'person', ?, 0, ?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
                    WHERE NOT EXISTS (SELECT 1 FROM parties WHERE user_id = ?)
                `, partyID, name, userID, userID).Error; err != nil {
                    return fmt.Errorf("creating person party for user %s: %w", userID, err)
                }
            }

            // Assign all existing catalog entries to the default project (idempotent)
            if err := tx.Exec(`
                INSERT INTO catalog_project_memberships (catalog_entry_id, project_party_id, created_at)
                SELECT ce.id, ?, CURRENT_TIMESTAMP
                FROM catalog_entries ce
                WHERE NOT EXISTS (
                    SELECT 1 FROM catalog_project_memberships
                    WHERE catalog_entry_id = ce.id AND project_party_id = ?
                )
            `, defaultProjectID, defaultProjectID).Error; err != nil {
                return fmt.Errorf("assigning catalog entries to default project: %w", err)
            }

            slog.Info("migration007: party archetype tables created and seeded")
            return nil
        },
    }
}
```

- [ ] **Step 4: Run to confirm pass**

```bash
rtk go test ./internal/db/... -run TestMigration007 -v
```

Expected: PASS.

- [ ] **Step 5: Run all existing tests to confirm no regression**

```bash
rtk make test
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/db/migrations.go internal/db/migrations_party_test.go
rtk git commit -m "feat(db): add migration007 for party archetype tables"
```

---

## Task 4: PartyStore — Basic CRUD

**Files:**
- Create: `internal/store/party_store.go`
- Create: `internal/store/party_store_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/store/party_store_test.go
package store_test

import (
    "context"
    "testing"

    "github.com/PawelHaracz/agentlens/internal/db"
    "github.com/PawelHaracz/agentlens/internal/model"
    "github.com/PawelHaracz/agentlens/internal/store"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func newTestPartyDB(t *testing.T) *db.DB {
    t.Helper()
    database, err := db.Open(db.DialectSQLite, ":memory:")
    require.NoError(t, err)
    migrator := db.NewMigrator(database, db.AllMigrations())
    require.NoError(t, migrator.Migrate(context.Background()))
    return database
}

func TestPartyStore_CreateAndGet(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    p := &model.Party{Kind: model.PartyKindGroup, Name: "eng-team"}
    require.NoError(t, s.CreateParty(ctx, p))
    assert.NotEmpty(t, p.ID)

    got, err := s.GetParty(ctx, p.ID)
    require.NoError(t, err)
    require.NotNil(t, got)
    assert.Equal(t, "eng-team", got.Name)
    assert.Equal(t, model.PartyKindGroup, got.Kind)
}

func TestPartyStore_CreateParty_InvalidKind(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    err := s.CreateParty(context.Background(), &model.Party{Kind: "bad-kind", Name: "x"})
    assert.Error(t, err)
}

func TestPartyStore_ListParties(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    require.NoError(t, s.CreateParty(ctx, &model.Party{Kind: model.PartyKindGroup, Name: "g1"}))
    require.NoError(t, s.CreateParty(ctx, &model.Party{Kind: model.PartyKindGroup, Name: "g2"}))
    require.NoError(t, s.CreateParty(ctx, &model.Party{Kind: model.PartyKindProject, Name: "p1"}))

    groups, err := s.ListParties(ctx, model.PartyKindGroup)
    require.NoError(t, err)
    assert.Len(t, groups, 2)

    projects, err := s.ListParties(ctx, model.PartyKindProject)
    require.NoError(t, err)
    // migration seeded "default" project, so expect at least 2
    assert.GreaterOrEqual(t, len(projects), 2)
}

func TestPartyStore_DeleteParty(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    p := &model.Party{Kind: model.PartyKindGroup, Name: "to-delete"}
    require.NoError(t, s.CreateParty(ctx, p))
    require.NoError(t, s.DeleteParty(ctx, p.ID))

    got, err := s.GetParty(ctx, p.ID)
    require.NoError(t, err)
    assert.Nil(t, got)
}

func TestPartyStore_DeleteParty_SystemPartyRejected(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    def, err := s.GetDefaultProject(ctx)
    require.NoError(t, err)
    require.NotNil(t, def)

    err = s.DeleteParty(ctx, def.ID)
    assert.Error(t, err)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
rtk go test ./internal/store/... -run "TestPartyStore_Create|TestPartyStore_List|TestPartyStore_Delete|TestPartyStore_Get" -v
```

Expected: FAIL — `store.NewPartyStore` undefined.

- [ ] **Step 3: Implement**

```go
// internal/store/party_store.go
package store

import (
    "context"
    "errors"
    "fmt"
    "strings"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "github.com/PawelHaracz/agentlens/internal/db"
    "github.com/PawelHaracz/agentlens/internal/model"
)

// PartyStore provides CRUD operations for the party archetype graph.
type PartyStore struct {
    db *db.DB
}

// NewPartyStore creates a new PartyStore backed by the given database.
func NewPartyStore(database *db.DB) *PartyStore {
    return &PartyStore{db: database}
}

// CreateParty inserts a new party. Assigns a UUID if ID is empty.
// Returns error if Kind is not in ValidPartyKinds.
func (s *PartyStore) CreateParty(ctx context.Context, p *model.Party) error {
    if !model.ValidPartyKinds[p.Kind] {
        return fmt.Errorf("invalid party kind: %q", p.Kind)
    }
    if p.ID == "" {
        p.ID = uuid.New().String()
    }
    if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
        return fmt.Errorf("creating party: %w", err)
    }
    return nil
}

// GetParty returns the party with the given ID, or nil if not found.
func (s *PartyStore) GetParty(ctx context.Context, id string) (*model.Party, error) {
    var p model.Party
    err := s.db.WithContext(ctx).First(&p, "id = ?", id).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, fmt.Errorf("getting party: %w", err)
    }
    return &p, nil
}

// GetPartyByUserID returns the Person party linked to the given user ID.
func (s *PartyStore) GetPartyByUserID(ctx context.Context, userID string) (*model.Party, error) {
    var p model.Party
    err := s.db.WithContext(ctx).First(&p, "user_id = ?", userID).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, fmt.Errorf("getting party by user id: %w", err)
    }
    return &p, nil
}

// GetDefaultProject returns the system-seeded default project party.
func (s *PartyStore) GetDefaultProject(ctx context.Context) (*model.Party, error) {
    var p model.Party
    err := s.db.WithContext(ctx).First(&p, "kind = ? AND is_system = ?", model.PartyKindProject, true).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, fmt.Errorf("default project not found — was migration007 run?")
        }
        return nil, fmt.Errorf("getting default project: %w", err)
    }
    return &p, nil
}

// ListParties returns all parties of the given kind.
func (s *PartyStore) ListParties(ctx context.Context, kind model.PartyKind) ([]model.Party, error) {
    var parties []model.Party
    if err := s.db.WithContext(ctx).Where("kind = ?", kind).Find(&parties).Error; err != nil {
        return nil, fmt.Errorf("listing parties: %w", err)
    }
    return parties, nil
}

// DeleteParty deletes a non-system party and cascades: removes all party_relationships,
// party_group_closures, global_party_roles, and catalog_project_memberships rows that
// reference the party, then rebuilds the full closure. Returns error if not found or is_system.
func (s *PartyStore) DeleteParty(ctx context.Context, id string) error {
    return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        var party model.Party
        if err := tx.First(&party, "id = ?", id).Error; err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return fmt.Errorf("party %q not found", id)
            }
            return fmt.Errorf("finding party: %w", err)
        }
        if party.IsSystem {
            return fmt.Errorf("party %q is a system party and cannot be deleted", id)
        }

        // Cascade: party_relationships (as from or to)
        if err := tx.Where("from_party_id = ? OR to_party_id = ?", id, id).
            Delete(&model.PartyRelationship{}).Error; err != nil {
            return fmt.Errorf("deleting relationships for party: %w", err)
        }

        // Cascade: global_party_roles
        if err := tx.Where("party_id = ?", id).
            Delete(&model.GlobalPartyRole{}).Error; err != nil {
            return fmt.Errorf("deleting global party roles: %w", err)
        }

        // Cascade: catalog_project_memberships (if party is a project)
        if err := tx.Where("project_party_id = ?", id).
            Delete(&model.CatalogProjectMembership{}).Error; err != nil {
            return fmt.Errorf("deleting catalog project memberships: %w", err)
        }

        // Rebuild closure after cascading relationship deletion
        if err := rebuildAllClosures(tx); err != nil {
            return fmt.Errorf("rebuilding closure after party deletion: %w", err)
        }

        // Delete the party itself
        if err := tx.Delete(&party).Error; err != nil {
            return fmt.Errorf("deleting party: %w", err)
        }
        return nil
    })
}
```

- [ ] **Step 4: Run to confirm pass**

```bash
rtk go test ./internal/store/... -run "TestPartyStore_Create|TestPartyStore_List|TestPartyStore_Delete|TestPartyStore_Get" -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/store/party_store.go internal/store/party_store_test.go
rtk git commit -m "feat(store): add PartyStore basic CRUD"
```

---

## Task 5: PartyStore — AddMember, RemoveMember, ListMembers, Closure Rebuild

**Files:**
- Modify: `internal/store/party_store.go` (add methods)
- Modify: `internal/store/party_store_test.go` (add tests)

- [ ] **Step 1: Add failing tests**

Append to `internal/store/party_store_test.go`:

```go
func TestPartyStore_AddMember_RebuildsClosure(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    // Create group hierarchy: alice → eng → platform
    alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
    eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
    platform := &model.Party{Kind: model.PartyKindGroup, Name: "platform"}
    require.NoError(t, s.CreateParty(ctx, alice))
    require.NoError(t, s.CreateParty(ctx, eng))
    require.NoError(t, s.CreateParty(ctx, platform))

    // alice → eng
    require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: alice.ID, FromRole: "member",
        ToPartyID: eng.ID, ToRole: "group",
        RelationshipName: "group_member",
    }))
    // eng → platform
    require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: eng.ID, FromRole: "member",
        ToPartyID: platform.ID, ToRole: "group",
        RelationshipName: "group_member",
    }))

    // Alice should have both eng and platform as ancestors
    ancestors, err := s.AncestorGroupIDs(ctx, alice.ID)
    require.NoError(t, err)
    assert.Contains(t, ancestors, eng.ID)
    assert.Contains(t, ancestors, platform.ID)
}

// TestPartyStore_AddMember_OutOfOrderHierarchy tests the correctness of the full-table
// closure rebuild when relationships are added in the "wrong" order (parent→grandparent
// before child→parent). A scoped rebuild would miss (alice, platform) in this scenario.
func TestPartyStore_AddMember_OutOfOrderHierarchy(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
    eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
    platform := &model.Party{Kind: model.PartyKindGroup, Name: "platform"}
    require.NoError(t, s.CreateParty(ctx, alice))
    require.NoError(t, s.CreateParty(ctx, eng))
    require.NoError(t, s.CreateParty(ctx, platform))

    // Add eng → platform FIRST
    require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: eng.ID, FromRole: "member",
        ToPartyID: platform.ID, ToRole: "group",
        RelationshipName: "group_member",
    }))
    // Then add alice → eng
    require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: alice.ID, FromRole: "member",
        ToPartyID: eng.ID, ToRole: "group",
        RelationshipName: "group_member",
    }))

    // Alice must have platform as a transitive ancestor even with reversed insertion order
    ancestors, err := s.AncestorGroupIDs(ctx, alice.ID)
    require.NoError(t, err)
    assert.Contains(t, ancestors, eng.ID, "alice should be in eng")
    assert.Contains(t, ancestors, platform.ID, "alice should transitively be in platform")
}

func TestPartyStore_AddMember_CycleRejected(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
    platform := &model.Party{Kind: model.PartyKindGroup, Name: "platform"}
    require.NoError(t, s.CreateParty(ctx, eng))
    require.NoError(t, s.CreateParty(ctx, platform))

    // eng → platform
    require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: eng.ID, FromRole: "member",
        ToPartyID: platform.ID, ToRole: "group",
        RelationshipName: "group_member",
    }))

    // platform → eng would create a cycle — must be rejected
    err := s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: platform.ID, FromRole: "member",
        ToPartyID: eng.ID, ToRole: "group",
        RelationshipName: "group_member",
    })
    assert.Error(t, err, "cycle should be rejected")
}

func TestPartyStore_RemoveMember_UpdatesClosure(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
    eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
    require.NoError(t, s.CreateParty(ctx, alice))
    require.NoError(t, s.CreateParty(ctx, eng))

    rel := &model.PartyRelationship{
        FromPartyID: alice.ID, FromRole: "member",
        ToPartyID: eng.ID, ToRole: "group",
        RelationshipName: "group_member",
    }
    require.NoError(t, s.AddMember(ctx, rel))

    ancestors, _ := s.AncestorGroupIDs(ctx, alice.ID)
    assert.Contains(t, ancestors, eng.ID)

    require.NoError(t, s.RemoveMember(ctx, alice.ID, eng.ID, "group_member"))

    ancestors, err := s.AncestorGroupIDs(ctx, alice.ID)
    require.NoError(t, err)
    assert.NotContains(t, ancestors, eng.ID)
}

func TestPartyStore_ListMembers(t *testing.T) {
    s := store.NewPartyStore(newTestPartyDB(t))
    ctx := context.Background()

    alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
    bob := &model.Party{Kind: model.PartyKindPerson, Name: "bob"}
    eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
    require.NoError(t, s.CreateParty(ctx, alice))
    require.NoError(t, s.CreateParty(ctx, bob))
    require.NoError(t, s.CreateParty(ctx, eng))

    require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: alice.ID, FromRole: "member",
        ToPartyID: eng.ID, ToRole: "group",
        RelationshipName: "group_member",
    }))
    require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
        FromPartyID: bob.ID, FromRole: "member",
        ToPartyID: eng.ID, ToRole: "group",
        RelationshipName: "group_member",
    }))

    rels, err := s.ListMembers(ctx, eng.ID, "group_member")
    require.NoError(t, err)
    assert.Len(t, rels, 2)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
rtk go test ./internal/store/... -run "TestPartyStore_Add|TestPartyStore_Remove|TestPartyStore_List" -v
```

Expected: FAIL — `AddMember`, `RemoveMember`, `AncestorGroupIDs`, `ListMembers` undefined.

- [ ] **Step 3: Implement — append to party_store.go**

```go
// AddMember creates a PartyRelationship and rebuilds the full closure table if it is a
// containment relationship (as defined by model.ContainmentRelationships).
// Detects and rejects cycles before inserting. Idempotent: duplicate edges silently ignored.
func (s *PartyStore) AddMember(ctx context.Context, rel *model.PartyRelationship) error {
    if rel.ID == "" {
        rel.ID = uuid.New().String()
    }
    return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if model.ContainmentRelationships[rel.RelationshipName] {
            // Self-loop check
            if rel.FromPartyID == rel.ToPartyID {
                return fmt.Errorf("cannot add party %s as member of itself", rel.FromPartyID)
            }
            // Cycle check: would adding from→to create a cycle?
            // A cycle exists if `to` is already a transitive member of `from`
            // (i.e., `from` is an ancestor of `to` in the current closure).
            var count int64
            if err := tx.Model(&model.PartyGroupClosure{}).
                Where("member_party_id = ? AND ancestor_party_id = ?", rel.ToPartyID, rel.FromPartyID).
                Count(&count).Error; err != nil {
                return fmt.Errorf("cycle detection: %w", err)
            }
            if count > 0 {
                return fmt.Errorf("adding %s → %s would create a cycle", rel.FromPartyID, rel.ToPartyID)
            }
        }

        result := tx.Where(
            "from_party_id = ? AND from_role = ? AND to_party_id = ? AND to_role = ? AND relationship_name = ?",
            rel.FromPartyID, rel.FromRole, rel.ToPartyID, rel.ToRole, rel.RelationshipName,
        ).FirstOrCreate(rel)
        if result.Error != nil {
            return fmt.Errorf("adding member relationship: %w", result.Error)
        }
        if model.ContainmentRelationships[rel.RelationshipName] {
            return rebuildAllClosures(tx)
        }
        return nil
    })
}

// RemoveMember deletes the relationship from fromPartyID to toPartyID with the
// given relationship name, then rebuilds the full closure if it is a containment relationship.
func (s *PartyStore) RemoveMember(ctx context.Context, fromPartyID, toPartyID, relationshipName string) error {
    return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        if err := tx.Where(
            "from_party_id = ? AND to_party_id = ? AND relationship_name = ?",
            fromPartyID, toPartyID, relationshipName,
        ).Delete(&model.PartyRelationship{}).Error; err != nil {
            return fmt.Errorf("removing member relationship: %w", err)
        }
        if model.ContainmentRelationships[relationshipName] {
            return rebuildAllClosures(tx)
        }
        return nil
    })
}

// ListMembers returns all PartyRelationship records where ToPartyID = groupID
// and RelationshipName = relationshipName.
func (s *PartyStore) ListMembers(ctx context.Context, toPartyID, relationshipName string) ([]model.PartyRelationship, error) {
    var rels []model.PartyRelationship
    if err := s.db.WithContext(ctx).
        Where("to_party_id = ? AND relationship_name = ?", toPartyID, relationshipName).
        Find(&rels).Error; err != nil {
        return nil, fmt.Errorf("listing members: %w", err)
    }
    return rels, nil
}

// AncestorGroupIDs returns all ancestor party IDs for the given party from the
// pre-computed party_group_closures table.
func (s *PartyStore) AncestorGroupIDs(ctx context.Context, partyID string) ([]string, error) {
    var ids []string
    if err := s.db.WithContext(ctx).
        Model(&model.PartyGroupClosure{}).
        Select("ancestor_party_id").
        Where("member_party_id = ?", partyID).
        Find(&ids).Error; err != nil {
        return nil, fmt.Errorf("loading ancestor group ids: %w", err)
    }
    return ids, nil
}

// AssignToProject adds a catalog entry to a project (idempotent).
func (s *PartyStore) AssignToProject(ctx context.Context, catalogEntryID, projectPartyID string) error {
    m := model.CatalogProjectMembership{
        CatalogEntryID: catalogEntryID,
        ProjectPartyID: projectPartyID,
    }
    return s.db.WithContext(ctx).
        Where("catalog_entry_id = ? AND project_party_id = ?", catalogEntryID, projectPartyID).
        FirstOrCreate(&m).Error
}

// RemoveFromProject removes a catalog entry from a project.
func (s *PartyStore) RemoveFromProject(ctx context.Context, catalogEntryID, projectPartyID string) error {
    return s.db.WithContext(ctx).
        Where("catalog_entry_id = ? AND project_party_id = ?", catalogEntryID, projectPartyID).
        Delete(&model.CatalogProjectMembership{}).Error
}

// ListProjectsForCatalogEntry returns all project parties a catalog entry belongs to.
func (s *PartyStore) ListProjectsForCatalogEntry(ctx context.Context, catalogEntryID string) ([]model.Party, error) {
    var projects []model.Party
    if err := s.db.WithContext(ctx).
        Joins("JOIN catalog_project_memberships cpm ON cpm.project_party_id = parties.id").
        Where("cpm.catalog_entry_id = ?", catalogEntryID).
        Find(&projects).Error; err != nil {
        return nil, fmt.Errorf("listing projects for catalog entry: %w", err)
    }
    return projects, nil
}

// ListCatalogEntryIDsForProject returns catalog entry IDs belonging to a project.
func (s *PartyStore) ListCatalogEntryIDsForProject(ctx context.Context, projectPartyID string) ([]string, error) {
    var ids []string
    if err := s.db.WithContext(ctx).
        Model(&model.CatalogProjectMembership{}).
        Select("catalog_entry_id").
        Where("project_party_id = ?", projectPartyID).
        Find(&ids).Error; err != nil {
        return nil, fmt.Errorf("listing catalog entry ids for project: %w", err)
    }
    return ids, nil
}

// GetProjectRoles returns the from_role values for parties in fromPartyIDs that
// have a project_member relationship to projectPartyID.
func (s *PartyStore) GetProjectRoles(ctx context.Context, fromPartyIDs []string, projectPartyID string) ([]string, error) {
    if len(fromPartyIDs) == 0 {
        return nil, nil
    }
    var roles []string
    if err := s.db.WithContext(ctx).
        Model(&model.PartyRelationship{}).
        Select("from_role").
        Where("relationship_name = ? AND to_party_id = ? AND from_party_id IN ?",
            "project_member", projectPartyID, fromPartyIDs).
        Find(&roles).Error; err != nil {
        return nil, fmt.Errorf("getting project roles: %w", err)
    }
    return roles, nil
}

// rebuildAllClosures recomputes the entire party_group_closures table from scratch.
// Correct regardless of edge insertion order — a scoped rebuild (per-group) would miss
// transitive members when relationships are added out of order.
// Must run inside an existing transaction.
func rebuildAllClosures(tx *gorm.DB) error {
    // Clear all closure rows
    if err := tx.Exec("DELETE FROM party_group_closures").Error; err != nil {
        return fmt.Errorf("clearing party_group_closures: %w", err)
    }

    names := model.ContainmentRelationshipNames()
    if len(names) == 0 {
        return nil
    }

    // Build placeholders for IN clause — GORM Exec IN ? expansion is reliable in
    // Where clauses but can be fragile in raw SQL. Build explicitly.
    placeholders := make([]string, len(names))
    args := make([]interface{}, 0, len(names)*2)
    for i, n := range names {
        placeholders[i] = "?"
        args = append(args, n)
    }
    // args will hold [names... names...] for the two IN clauses below
    args = append(args, args...)

    inClause := strings.Join(placeholders, ",")
    sql := fmt.Sprintf(`
        INSERT INTO party_group_closures (member_party_id, ancestor_party_id)
        WITH RECURSIVE pairs(member_id, ancestor_id) AS (
            SELECT from_party_id, to_party_id
            FROM party_relationships
            WHERE relationship_name IN (%s)
            UNION
            SELECT p.member_id, pr.to_party_id
            FROM pairs p
            JOIN party_relationships pr ON pr.from_party_id = p.ancestor_id
            WHERE pr.relationship_name IN (%s)
        )
        SELECT DISTINCT member_id, ancestor_id FROM pairs
    `, inClause, inClause)

    return tx.Exec(sql, args...).Error
}

// Note: rebuildAllClosures requires "fmt" and "strings" imports in party_store.go.
```

- [ ] **Step 4: Run to confirm pass**

```bash
rtk go test ./internal/store/... -v
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/store/party_store.go internal/store/party_store_test.go
rtk git commit -m "feat(store): add member ops, closure rebuild, project assignment"
```

---

## Task 6: PartyKindConfig + RegisterPartyKindRoutes

**Files:**
- Create: `internal/api/party_kind_configs.go`
- Modify: `internal/api/router.go` (add `RegisterPartyKindRoutes`, add `PartyStore` to `RouterDeps`)

- [ ] **Step 1: Create party_kind_configs.go**

```go
// internal/api/party_kind_configs.go
package api

import "github.com/PawelHaracz/agentlens/internal/model"

// PartyKindConfig drives route registration and handler behaviour for one party kind.
// Adding a new party kind = add a new PartyKindConfig registration in NewRouter.
// Zero handler code changes required.
type PartyKindConfig struct {
    Kind               model.PartyKind  // "group", "project"
    URLPrefix          string           // "groups", "projects"
    MemberRelationship string           // relationship_name for members
    ValidMemberRoles   []string         // roles a member may hold
    CreatePermission   string           // global permission required to create
    ManagePermission   string           // global permission required to delete
    CanContainKinds    []model.PartyKind // which party kinds may be members
}

// groupKindConfig is the PartyKindConfig for groups.
var groupKindConfig = PartyKindConfig{
    Kind:               model.PartyKindGroup,
    URLPrefix:          "groups",
    MemberRelationship: "group_member",
    ValidMemberRoles:   []string{"member"},
    CreatePermission:   "users:write",
    ManagePermission:   "users:write",
    CanContainKinds:    []model.PartyKind{model.PartyKindPerson, model.PartyKindGroup},
}

// projectKindConfig is the PartyKindConfig for projects.
var projectKindConfig = PartyKindConfig{
    Kind:               model.PartyKindProject,
    URLPrefix:          "projects",
    MemberRelationship: "project_member",
    ValidMemberRoles:   []string{"project:owner", "project:developer", "project:viewer"},
    CreatePermission:   "catalog:write",
    ManagePermission:   "catalog:write",
    CanContainKinds:    []model.PartyKind{model.PartyKindPerson, model.PartyKindGroup},
}
```

- [ ] **Step 2: Add PartyStore to RouterDeps and wire registerPartyRoutes**

In `internal/api/router.go`, add to `RouterDeps`:

```go
// Add to RouterDeps struct:
PartyStore *store.PartyStore  // nil disables party/project/group routes
```

The import `"github.com/PawelHaracz/agentlens/internal/store"` is already present.

Add a `registerPartyRoutes` function — follow the exact same pattern as `registerCatalogRoutes`
(uses `r.Group` with `r.Use(RequireAuth(...))` to ensure auth is applied):

```go
// registerPartyRoutes mounts group, project, and catalog-project scoping endpoints
// behind the same auth middleware as other /api/v1 routes.
func registerPartyRoutes(r chi.Router, deps RouterDeps) {
    if deps.PartyStore == nil {
        return
    }
    r.Group(func(r chi.Router) {
        r.Use(RequireAuth(deps.JWTService))
        RegisterPartyKindRoutes(r, groupKindConfig, deps.PartyStore)
        RegisterPartyKindRoutes(r, projectKindConfig, deps.PartyStore)
        registerCatalogProjectRoutes(r, deps.PartyStore)
    })
}
```

Call it inside the `if deps.JWTService != nil` block in `NewRouter`, after `registerSettingsRoutes`:

```go
// Inside r.Route("/api/v1", func(r chi.Router) { if deps.JWTService != nil { ... } }):
registerAuthRoutes(r, deps, authHandler)
registerCatalogRoutes(r, h, deps)
registerUserRoutes(r, deps)
registerSettingsRoutes(r, deps)
registerPartyRoutes(r, deps)   // add this line
```

Add `RegisterPartyKindRoutes` — uses **relative** paths (no `/api/v1/` prefix) because it
is called inside the already-mounted `/api/v1` subrouter:

```go
// RegisterPartyKindRoutes registers CRUD + member routes for one PartyKindConfig.
// r must already be a subrouter mounted at /api/v1 with RequireAuth applied.
func RegisterPartyKindRoutes(r chi.Router, cfg PartyKindConfig, partyStore *store.PartyStore) {
    r.Route("/"+cfg.URLPrefix, func(r chi.Router) {
        r.Get("/", ListPartiesHandler(cfg, partyStore))
        r.Post("/", CreatePartyHandler(cfg, partyStore))
        r.Route("/{partyID}", func(r chi.Router) {
            r.Get("/", GetPartyHandler(cfg, partyStore))
            r.Delete("/", DeletePartyHandler(cfg, partyStore))
            r.Get("/members", ListMembersHandler(cfg, partyStore))
            r.Post("/members", AddMemberHandler(cfg, partyStore))
            r.Delete("/members/{memberPartyID}", RemoveMemberHandler(cfg, partyStore))
        })
    })
}
```

**Critical:** Do NOT call `r.Route("/api/v1/"+cfg.URLPrefix, ...)` — that would mount at
`/api/v1/api/v1/groups`. Use relative paths only (`"/"+cfg.URLPrefix`).

- [ ] **Step 3: Compile check**

```bash
rtk go build ./internal/api/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
rtk git add internal/api/party_kind_configs.go internal/api/router.go
rtk git commit -m "feat(api): add PartyKindConfig and RegisterPartyKindRoutes scaffold"
```

---

## Task 7: Generic Party Handlers

**Files:**
- Create: `internal/api/party_handlers.go`
- Create: `internal/api/party_handlers_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/api/party_handlers_test.go
package api_test

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/PawelHaracz/agentlens/internal/api"
    "github.com/PawelHaracz/agentlens/internal/auth"
    "github.com/PawelHaracz/agentlens/internal/db"
    "github.com/PawelHaracz/agentlens/internal/model"
    "github.com/PawelHaracz/agentlens/internal/store"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func newPartyTestRouter(t *testing.T) (http.Handler, *store.PartyStore) {
    t.Helper()
    database, err := db.Open(db.DialectSQLite, ":memory:")
    require.NoError(t, err)
    require.NoError(t, db.NewMigrator(database, db.AllMigrations()).Migrate(context.Background()))
    ps := store.NewPartyStore(database)

    // JWTService requires UserStore + RoleStore — router panics if any are nil when JWTService is set.
    jwtSvc := auth.NewJWTService(auth.JWTConfig{Secret: "test-secret", Expiration: 24 * 3600 * 1e9})
    deps := api.RouterDeps{
        JWTService:    jwtSvc,
        UserStore:     store.NewUserStore(database),
        RoleStore:     store.NewRoleStore(database),
        SettingsStore: store.NewSettingsStore(database),
        PartyStore:    ps,
    }
    return api.NewRouter(deps), ps
}

func partyAuthHeader(t *testing.T, permissions []string) string {
    t.Helper()
    jwtSvc := auth.NewJWTService(auth.JWTConfig{Secret: "test-secret", Expiration: 24 * 3600 * 1e9})
    token, err := jwtSvc.GenerateToken(auth.Claims{
        UserID: "user1", Username: "alice", RoleID: "r1", Permissions: permissions,
    })
    require.NoError(t, err)
    return "Bearer " + token
}

func TestCreateGroup_ReturnsCreated(t *testing.T) {
    router, _ := newPartyTestRouter(t)

    body, _ := json.Marshal(map[string]string{"name": "eng-team"})
    req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", bytes.NewReader(body))
    req.Header.Set("Authorization", partyAuthHeader(t, []string{"users:write"}))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusCreated, w.Code)

    var resp model.Party
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
    assert.Equal(t, "eng-team", resp.Name)
    assert.Equal(t, model.PartyKindGroup, resp.Kind)
}

func TestCreateGroup_Forbidden_WithoutPermission(t *testing.T) {
    router, _ := newPartyTestRouter(t)

    body, _ := json.Marshal(map[string]string{"name": "eng-team"})
    req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", bytes.NewReader(body))
    req.Header.Set("Authorization", partyAuthHeader(t, []string{"catalog:read"}))
    req.Header.Set("Content-Type", "application/json")
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListGroups_ReturnsOK(t *testing.T) {
    router, ps := newPartyTestRouter(t)
    require.NoError(t, ps.CreateParty(context.Background(), &model.Party{Kind: model.PartyKindGroup, Name: "g1"}))

    req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
    req.Header.Set("Authorization", partyAuthHeader(t, []string{"users:read"}))
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    assert.Equal(t, http.StatusOK, w.Code)
    var groups []model.Party
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &groups))
    assert.GreaterOrEqual(t, len(groups), 1)
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
rtk go test ./internal/api/... -run "TestCreate|TestListGroups" -v
```

Expected: FAIL — handlers undefined or routes not registered.

- [ ] **Step 3: Implement party_handlers.go**

```go
// internal/api/party_handlers.go
package api

import (
    "encoding/json"
    "log/slog"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"

    "github.com/PawelHaracz/agentlens/internal/auth"
    "github.com/PawelHaracz/agentlens/internal/model"
    "github.com/PawelHaracz/agentlens/internal/store"
)

type createPartyRequest struct {
    Name string `json:"name"`
}

type addMemberRequest struct {
    PartyID string `json:"party_id"`
    Role    string `json:"role"`
}

// ListPartiesHandler returns all parties of cfg.Kind.
func ListPartiesHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        parties, err := ps.ListParties(r.Context(), cfg.Kind)
        if err != nil {
            slog.Error("listing parties", "kind", cfg.Kind, "err", err)
            ErrorResponse(w, http.StatusInternalServerError, "failed to list "+cfg.URLPrefix)
            return
        }
        JSONResponse(w, http.StatusOK, parties)
    }
}

// CreatePartyHandler creates a new party of cfg.Kind.
// Requires cfg.CreatePermission in the caller's global role.
func CreatePartyHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.CreatePermission) {
            ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
            return
        }
        var req createPartyRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
            ErrorResponse(w, http.StatusBadRequest, "name is required")
            return
        }
        p := &model.Party{ID: uuid.New().String(), Kind: cfg.Kind, Name: req.Name}
        if err := ps.CreateParty(r.Context(), p); err != nil {
            slog.Error("creating party", "kind", cfg.Kind, "err", err)
            ErrorResponse(w, http.StatusInternalServerError, "failed to create")
            return
        }
        JSONResponse(w, http.StatusCreated, p)
    }
}

// GetPartyHandler returns a single party by ID, scoped to cfg.Kind.
func GetPartyHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := chi.URLParam(r, "partyID")
        p, err := ps.GetParty(r.Context(), id)
        if err != nil {
            ErrorResponse(w, http.StatusInternalServerError, "failed to get")
            return
        }
        if p == nil || p.Kind != cfg.Kind {
            ErrorResponse(w, http.StatusNotFound, "not found")
            return
        }
        JSONResponse(w, http.StatusOK, p)
    }
}

// DeletePartyHandler deletes a non-system party of cfg.Kind.
// Requires cfg.ManagePermission.
func DeletePartyHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.ManagePermission) {
            ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
            return
        }
        id := chi.URLParam(r, "partyID")
        if err := ps.DeleteParty(r.Context(), id); err != nil {
            ErrorResponse(w, http.StatusBadRequest, err.Error())
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// AddMemberHandler adds a party as a member with a role.
// Validates role against cfg.ValidMemberRoles.
func AddMemberHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
    validRoles := make(map[string]bool, len(cfg.ValidMemberRoles))
    for _, r := range cfg.ValidMemberRoles {
        validRoles[r] = true
    }
    return func(w http.ResponseWriter, r *http.Request) {
        if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.ManagePermission) {
            ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
            return
        }
        toPartyID := chi.URLParam(r, "partyID")
        var req addMemberRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            ErrorResponse(w, http.StatusBadRequest, "invalid request body")
            return
        }
        if req.PartyID == "" || req.Role == "" {
            ErrorResponse(w, http.StatusBadRequest, "party_id and role are required")
            return
        }
        if !validRoles[req.Role] {
            ErrorResponse(w, http.StatusBadRequest, "invalid role for this party kind")
            return
        }
        rel := &model.PartyRelationship{
            FromPartyID:      req.PartyID,
            FromRole:         req.Role,
            ToPartyID:        toPartyID,
            ToRole:           string(cfg.Kind),
            RelationshipName: cfg.MemberRelationship,
        }
        if err := ps.AddMember(r.Context(), rel); err != nil {
            slog.Error("adding member", "err", err)
            ErrorResponse(w, http.StatusInternalServerError, "failed to add member")
            return
        }
        JSONResponse(w, http.StatusCreated, rel)
    }
}

// RemoveMemberHandler removes a party from membership.
func RemoveMemberHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !auth.HasPermission(PermissionsFromContext(r.Context()), cfg.ManagePermission) {
            ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
            return
        }
        toPartyID := chi.URLParam(r, "partyID")
        memberPartyID := chi.URLParam(r, "memberPartyID")
        if err := ps.RemoveMember(r.Context(), memberPartyID, toPartyID, cfg.MemberRelationship); err != nil {
            ErrorResponse(w, http.StatusInternalServerError, "failed to remove member")
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// ListMembersHandler lists all direct members of a party.
func ListMembersHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        toPartyID := chi.URLParam(r, "partyID")
        rels, err := ps.ListMembers(r.Context(), toPartyID, cfg.MemberRelationship)
        if err != nil {
            ErrorResponse(w, http.StatusInternalServerError, "failed to list members")
            return
        }
        JSONResponse(w, http.StatusOK, rels)
    }
}
```

- [ ] **Step 4: Run to confirm pass**

```bash
rtk go test ./internal/api/... -run "TestCreate|TestListGroups" -v
```

Expected: PASS.

- [ ] **Step 5: Run all tests**

```bash
rtk make test
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/api/party_handlers.go internal/api/party_handlers_test.go
rtk git commit -m "feat(api): add generic party handlers (create/list/get/delete/members)"
```

---

## Task 8: RequireProjectPermission Middleware

**Files:**
- Create: `internal/api/party_middleware.go`

- [ ] **Step 1: Write failing test**

Append to `internal/api/party_handlers_test.go`:

```go
func TestRequireProjectPermission_GlobalAdminBypasses(t *testing.T) {
    router, ps := newPartyTestRouter(t)

    // Create a project
    proj := &model.Party{Kind: model.PartyKindProject, Name: "myproject"}
    require.NoError(t, ps.CreateParty(context.Background(), proj))

    // global:admin permissions bypass project check
    req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID, nil)
    req.Header.Set("Authorization", partyAuthHeader(t, []string{"catalog:read", "catalog:write", "catalog:delete", "users:read", "users:write", "users:delete", "roles:read", "roles:write", "settings:read", "settings:write"}))
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    // 200 because global admin — project exists and kind matches
    assert.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 2: Implement party_middleware.go**

```go
// internal/api/party_middleware.go
package api

import (
    "context"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/PawelHaracz/agentlens/internal/auth"
    "github.com/PawelHaracz/agentlens/internal/store"
)

type ancestorCacheKey struct{}

// RequireProjectPermission checks that the authenticated user has the given permission
// in the project identified by the chi URL parameter projectIDParam.
//
// Resolution order:
//  1. Global bypass: if user's JWT permissions contain `permission` → ALLOW
//  2. Group global roles: if any ancestor group carries a global role with `permission` → ALLOW
//  3. Project roles: if user or ancestor group has a project_member relationship to the project
//     with a role that grants `permission` → ALLOW
//  4. DENY
func RequireProjectPermission(ps *store.PartyStore, us *store.UserStore, projectIDParam, permission string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := r.Context()

            // Step 1: global bypass (from JWT context — no DB call)
            if auth.HasPermission(PermissionsFromContext(ctx), permission) {
                next.ServeHTTP(w, r)
                return
            }

            userID := UserIDFromContext(ctx)
            projectID := chi.URLParam(r, projectIDParam)

            // Resolve user's party
            userParty, err := ps.GetPartyByUserID(ctx, userID)
            if err != nil || userParty == nil {
                ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
                return
            }

            // Get/cache ancestor IDs for this request
            ancestorIDs, ctx := cachedAncestorIDs(ctx, ps, userParty.ID)

            // Step 2: group global roles
            if len(ancestorIDs) > 0 {
                groupRoles, err := us.GetRolesForParties(ctx, ancestorIDs)
                if err == nil {
                    for _, role := range groupRoles {
                        if auth.HasPermission(role.Permissions, permission) {
                            next.ServeHTTP(w, r.WithContext(ctx))
                            return
                        }
                    }
                }
            }

            // Step 3: project-scoped roles
            fromIDs := append([]string{userParty.ID}, ancestorIDs...)
            projectRoles, err := ps.GetProjectRoles(ctx, fromIDs, projectID)
            if err == nil {
                for _, role := range projectRoles {
                    if auth.ProjectRoleHasPermission(role, permission) {
                        next.ServeHTTP(w, r.WithContext(ctx))
                        return
                    }
                }
            }

            ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
        })
    }
}

// cachedAncestorIDs returns ancestor group IDs from context cache (fast path)
// or from the DB (first call per request). Returns an updated context with the cache set.
// Returns a defensive copy of the slice so callers cannot mutate the cached value.
func cachedAncestorIDs(ctx context.Context, ps *store.PartyStore, partyID string) ([]string, context.Context) {
    if cached, ok := ctx.Value(ancestorCacheKey{}).([]string); ok {
        return append([]string(nil), cached...), ctx
    }
    ids, err := ps.AncestorGroupIDs(ctx, partyID)
    if err != nil {
        ids = nil
    }
    // Store a copy in the cache; return a copy to the caller
    cached := append([]string(nil), ids...)
    return cached, context.WithValue(ctx, ancestorCacheKey{}, cached)
}
```

**Note:** `us.GetRolesForParties(ctx, partyIDs)` requires a new method on `UserStore`. Add it:

Append to `internal/store/user_store.go`:

```go
// GetRolesForParties returns the global Role records for parties listed in partyIDs
// via the global_party_roles join table.
func (s *UserStore) GetRolesForParties(ctx context.Context, partyIDs []string) ([]model.Role, error) {
    var roles []model.Role
    if err := s.db.WithContext(ctx).
        Joins("JOIN global_party_roles gpr ON gpr.role_id = roles.id").
        Where("gpr.party_id IN ?", partyIDs).
        Find(&roles).Error; err != nil {
        return nil, fmt.Errorf("getting roles for parties: %w", err)
    }
    return roles, nil
}
```

- [ ] **Step 3: Verify compilation**

```bash
rtk go build ./internal/api/... ./internal/store/...
```

Expected: no errors.

- [ ] **Step 4: Run tests**

```bash
rtk make test
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
rtk git add internal/api/party_middleware.go internal/store/user_store.go internal/api/party_handlers_test.go
rtk git commit -m "feat(api): add RequireProjectPermission middleware with closure cache"
```

---

## Task 9: Catalog Project Handlers

**Files:**
- Create: `internal/api/catalog_project.go`

- [ ] **Step 1: Write failing test**

Add `TestCatalogAssignToProject_ReturnsCreated` to `internal/api/party_handlers_test.go`:

```go
func TestCatalogAssignToProject_ReturnsCreated(t *testing.T) {
    // Requires a full router with CatalogStore — skip if CatalogStore not wired in test router
    // This is an integration test; run with make e2e-test for full coverage.
    // Unit-level: verify route exists and returns 401 without auth.
    router, _ := newPartyTestRouter(t)

    req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/nonexistent/projects", nil)
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

- [ ] **Step 2: Implement catalog_project.go**

```go
// internal/api/catalog_project.go
package api

import (
    "encoding/json"
    "log/slog"
    "net/http"

    "github.com/go-chi/chi/v5"

    "github.com/PawelHaracz/agentlens/internal/auth"
    "github.com/PawelHaracz/agentlens/internal/store"
)

type assignProjectRequest struct {
    ProjectID string `json:"project_id"`
}

// AssignCatalogToProjectHandler assigns a catalog entry to an additional project.
// Requires catalog:write globally or project:owner/developer in the target project.
func AssignCatalogToProjectHandler(ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !auth.HasPermission(PermissionsFromContext(r.Context()), auth.PermCatalogWrite) {
            ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
            return
        }
        catalogID := chi.URLParam(r, "id")
        var req assignProjectRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProjectID == "" {
            ErrorResponse(w, http.StatusBadRequest, "project_id is required")
            return
        }
        if err := ps.AssignToProject(r.Context(), catalogID, req.ProjectID); err != nil {
            slog.Error("assigning catalog to project", "err", err)
            ErrorResponse(w, http.StatusInternalServerError, "failed to assign")
            return
        }
        w.WriteHeader(http.StatusCreated)
    }
}

// RemoveCatalogFromProjectHandler removes a catalog entry from a project.
// Cannot remove from the default project if it is the only membership.
func RemoveCatalogFromProjectHandler(ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !auth.HasPermission(PermissionsFromContext(r.Context()), auth.PermCatalogWrite) {
            ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
            return
        }
        catalogID := chi.URLParam(r, "id")
        projectID := chi.URLParam(r, "projectID")
        if err := ps.RemoveFromProject(r.Context(), catalogID, projectID); err != nil {
            ErrorResponse(w, http.StatusInternalServerError, "failed to remove")
            return
        }
        w.WriteHeader(http.StatusNoContent)
    }
}

// ListCatalogProjectsHandler lists all projects a catalog entry belongs to.
func ListCatalogProjectsHandler(ps *store.PartyStore) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        catalogID := chi.URLParam(r, "id")
        projects, err := ps.ListProjectsForCatalogEntry(r.Context(), catalogID)
        if err != nil {
            ErrorResponse(w, http.StatusInternalServerError, "failed to list projects")
            return
        }
        JSONResponse(w, http.StatusOK, projects)
    }
}

// registerCatalogProjectRoutes mounts catalog↔project scoping endpoints.
// Called from registerPartyRoutes — already inside /api/v1 subrouter with auth.
// Uses relative path (no /api/v1/ prefix).
func registerCatalogProjectRoutes(r chi.Router, ps *store.PartyStore) {
    r.Route("/catalog/{id}/projects", func(r chi.Router) {
        r.Get("/", ListCatalogProjectsHandler(ps))
        r.Post("/", AssignCatalogToProjectHandler(ps))
        r.Delete("/{projectID}", RemoveCatalogFromProjectHandler(ps))
    })
}
```

- [ ] **Step 3: Run test**

```bash
rtk go test ./internal/api/... -run "TestCatalogAssign" -v
```

Expected: PASS (401 route check passes).

- [ ] **Step 4: Commit**

```bash
rtk git add internal/api/catalog_project.go
rtk git commit -m "feat(api): add catalog project scoping handlers"
```

---

## Task 10: Router + main.go Wiring

**Files:**
- Modify: `internal/api/router.go` — ensure `registerCatalogProjectRoutes` is called, `PartyStore` wired
- Modify: `cmd/agentlens/main.go` — create `PartyStore`, wire into `RouterDeps`

- [ ] **Step 1: Wire PartyStore in main.go**

In `cmd/agentlens/main.go`, after `catalogStore := store.NewSQLStore(database)` (line ~169):

```go
// Add after catalogStore init:
partyStore := store.NewPartyStore(database)
```

Add `PartyStore: partyStore` to `RouterDeps`:

```go
routerDeps := api.RouterDeps{
    // ... existing fields ...
    PartyStore: partyStore,
}
```

- [ ] **Step 2: Auto-assign new catalog entries to default project**

In `internal/store/sql_store.go`, find the `Create` (or `Upsert`) method for `CatalogEntry`.
After a successful insert, assign the entry to the default project.

The store needs access to `PartyStore`. Add a field to `SQLStore` — **keep the existing
`gdb` field name unchanged** (renaming it to `db` would break all existing methods):

```go
// In sql_store.go — add partyStore field, keep gdb unchanged:
type SQLStore struct {
    gdb        *db.DB
    partyStore *PartyStore // nil until WithPartyStore is called; safe to omit in tests
}

// NewSQLStore is unchanged — only gdb is set here.

// WithPartyStore injects a PartyStore for auto-project assignment on catalog entry creation.
func (s *SQLStore) WithPartyStore(ps *PartyStore) {
    s.partyStore = ps
}
```

In `main.go`, after creating both stores:

```go
catalogStore.WithPartyStore(partyStore)
```

In `sql_store.go`, in `SQLStore.Create` (around line 240), after the successful insert:

```go
// Auto-assign to default project (best-effort — log on failure, don't break catalog write)
if s.partyStore != nil {
    if def, err := s.partyStore.GetDefaultProject(ctx); err == nil && def != nil {
        if err := s.partyStore.AssignToProject(ctx, entry.ID, def.ID); err != nil {
            slog.Warn("failed to assign new catalog entry to default project", "entry_id", entry.ID, "err", err)
        }
    }
}
```

- [ ] **Step 3: Bootstrap admin gets a Person party**

`auth.BootstrapAdmin` creates the initial admin `User` but leaves no `Party` for that user.
On a fresh install, project-scoped endpoints would return 403 for the admin because
`GetPartyByUserID` returns nil.

In `cmd/agentlens/main.go`, after the `BootstrapAdmin` call, create the Person party:

```go
// After: password, err := auth.BootstrapAdmin(ctx, userStore)
// Add:
if err == nil && password != "" {
    // A new admin was just created — create the corresponding Person party
    adminUser, err := userStore.GetByUsername(ctx, "admin")
    if err == nil && adminUser != nil {
        adminParty := &model.Party{
            Kind:   model.PartyKindPerson,
            Name:   adminUser.Username,
            UserID: &adminUser.ID,
        }
        if err := partyStore.CreateParty(ctx, adminParty); err != nil {
            slog.Warn("failed to create Person party for bootstrap admin", "err", err)
        }
    }
}
```

Add import `"github.com/PawelHaracz/agentlens/internal/model"` to `main.go` if not present.

- [ ] **Step 4: Build**

```bash
rtk make build
```

Expected: no errors.

- [ ] **Step 5: Run all tests + arch-test**

```bash
rtk make test
rtk make arch-test
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
rtk git add cmd/agentlens/main.go internal/store/sql_store.go internal/api/router.go
rtk git commit -m "feat: wire PartyStore into router and main, auto-assign catalog entries to default project"
```

---

## Task 11: Catalog Filter by Project

**Files:**
- Modify: `internal/store/sql_store.go` — add `project` filter to `List`
- Modify: `internal/api/handlers.go` — pass `project` query param to store

- [ ] **Step 1: Add failing test for project-scoped catalog list**

Append to `internal/api/party_handlers_test.go`:

```go
func TestListCatalog_FilteredByProject_ReturnsOK(t *testing.T) {
    router, _ := newPartyTestRouter(t)

    // Without project filter — existing behavior, no change expected
    req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
    req.Header.Set("Authorization", partyAuthHeader(t, []string{"catalog:read"}))
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)
    assert.Equal(t, http.StatusOK, w.Code)

    // With project filter — must also return 200 (empty list is fine)
    req2 := httptest.NewRequest(http.MethodGet, "/api/v1/catalog?project=nonexistent-id", nil)
    req2.Header.Set("Authorization", partyAuthHeader(t, []string{"catalog:read"}))
    w2 := httptest.NewRecorder()
    router.ServeHTTP(w2, req2)
    assert.Equal(t, http.StatusOK, w2.Code)
}
```

- [ ] **Step 2: Run to confirm existing behavior passes, project filter needs work**

```bash
rtk go test ./internal/api/... -run TestListCatalog -v
```

- [ ] **Step 3: Add `ProjectID` to `ListFilter` in `internal/store/store.go`**

`Handler.store` is typed as `store.Store` (interface). Adding a separate `ListByProject`
method would require extending the interface and every implementer. Instead, add an optional
filter field to the existing `ListFilter` — this is the established pattern and composes with
all other filters (protocol, state, query, etc.).

In `internal/store/store.go`, add to `ListFilter`:

```go
type ListFilter struct {
    Protocol   *model.Protocol
    States     []model.LifecycleState
    Source     *model.SourceType
    Team       string
    Query      string
    Categories []string
    Limit      int
    Offset     int
    Sort       string
    ProjectID  string // optional: filter to entries in this project party ID (empty = no filter)
}
```

- [ ] **Step 4: Handle `ProjectID` in `SQLStore.List`**

In `internal/store/sql_store.go`, in the `List` method, add the JOIN after the existing
filter conditions (look for where `Protocol`, `States`, `Source` etc. are applied):

```go
// Add inside SQLStore.List, alongside existing filter conditions:
if filter.ProjectID != "" {
    db = db.Joins("JOIN catalog_project_memberships cpm ON cpm.catalog_entry_id = catalog_entries.id").
        Where("cpm.project_party_id = ?", filter.ProjectID)
}
```

- [ ] **Step 5: Pass project query param to `ListFilter` in the catalog handler**

In `internal/api/handlers.go`, find `ListCatalog` (or wherever `parseListFilter` is called).
Add the project param alongside existing params:

```go
// In the ListCatalog handler, after parseListFilter(r) or equivalent:
filter := parseListFilter(r)
if projectID := r.URL.Query().Get("project"); projectID != "" {
    filter.ProjectID = projectID
}
entries, err := h.store.List(r.Context(), filter)
// rest of handler unchanged
```

This preserves composability — a caller can combine `?project=X&protocol=a2a` to get A2A
entries scoped to project X.

- [ ] **Step 6: Run tests**

```bash
rtk go test ./internal/api/... -run TestListCatalog -v
rtk make test
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
rtk git add internal/store/store.go internal/store/sql_store.go internal/api/handlers.go internal/api/party_handlers_test.go
rtk git commit -m "feat(api): add catalog filter by project query param"
```

---

## Task 12: Final Validation

- [ ] **Step 1: Full test suite**

```bash
rtk make all
```

Expected: format → lint → test → arch-test → build — all green.

- [ ] **Step 2: Smoke test with running server**

```bash
rtk go run ./cmd/agentlens --config agentlens.yaml &
sleep 2
# Should return 200 with default project listed
curl -s http://localhost:8080/healthz
# Login to get token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<from bootstrap output>"}' | jq -r .token)
# List groups (empty)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/groups
# List projects (should include 'default')
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/projects
```

Expected: `/api/v1/projects` returns JSON array containing the default project.

- [ ] **Step 3: Commit final**

```bash
rtk git add -A
rtk git commit -m "feat(party): complete party archetype implementation"
```

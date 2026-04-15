# Current-User Project Context Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the "My projects" view on the My Account tab, backed by a new role-resolving endpoint, with the Person↔User invariant finally enforced on user create/update and backfilled for stragglers.

**Architecture:** Three backend layers (store additions for Person sync + role resolution, handler for `GET /auth/me/projects`, migration008 backfill) plus frontend additions to `AccountTab` and the API client. Role resolution applies ADR-014's highest-privilege rule inside `PartyStore.ResolveUserProjects`; `UserHandler` is rewired to depend on `PartyStore` so user create/update can trigger Person sync.

**Tech Stack:** Go 1.26.1, GORM (sqlite + postgres), chi router. React 18, TypeScript, TanStack React Query, shadcn/ui, Vitest + RTL, Playwright.

**Read before starting:**

- [docs/superpowers/specs/2026-04-15-my-projects-context-design.md](../specs/2026-04-15-my-projects-context-design.md) — this spec
- [docs/adr/011-party-archetype-unified-actor-model.md](../../adr/011-party-archetype-unified-actor-model.md) — Person↔User invariant
- [docs/adr/012-materialized-group-closure-for-permission-resolution.md](../../adr/012-materialized-group-closure-for-permission-resolution.md) — closure table semantics
- [docs/adr/014-project-role-resolution-highest-privilege.md](../../adr/014-project-role-resolution-highest-privilege.md) — tie-break rule we must implement
- [internal/db/migrations.go](../../../internal/db/migrations.go) — migration numbering + existing `migration007SeedPersonParties` (model for migration008)
- [internal/api/user_handlers.go](../../../internal/api/user_handlers.go) — where to inject PartyStore sync calls
- [internal/store/party_store.go](../../../internal/store/party_store.go) — where new store methods live
- [web/src/pages/SettingsPage.tsx](../../../web/src/pages/SettingsPage.tsx) — `AccountTab` component to extend
- [docs/superpowers/plans/2026-04-15-projects-ui.md](2026-04-15-projects-ui.md) — Spec 2 plan; this plan builds on the same `routes/parties/` module

**Dependency note:** This plan assumes the Spec 2 (Projects UI) plan has landed, because the "My projects" row-click navigates to `/settings/projects/:id`. If Spec 2 hasn't shipped yet, the row-click still works — it just hits the 404/Loading path. The plan is orderable either way but best run after Spec 2.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/model/party_query.go` | Create | `UserProjectMembership` DTO |
| `internal/store/party_store_person_sync.go` | Create | `CreatePersonForUser`, `UpdatePersonForUser` (new file to avoid breaching `party_store.go` 10-public-function ceiling) |
| `internal/store/party_store_role_resolution.go` | Create | `ResolveUserProjects` (same ceiling reason) |
| `internal/store/party_store_person_sync_test.go` | Create | Store tests for the sync methods |
| `internal/store/party_store_role_resolution_test.go` | Create | Store tests for the resolution |
| `internal/api/auth_handlers.go` | Modify | Add `MyProjectsHandler` |
| `internal/api/auth_handlers_test.go` | Modify | Handler tests |
| `internal/api/user_handlers.go` | Modify | Add `partyStore` field, call sync on Create/Update |
| `internal/api/user_handlers_test.go` | Modify | Tests confirming sync side-effects |
| `internal/api/router.go` | Modify | Wire `partyStore` into `NewUserHandler`; mount `GET /auth/me/projects` |
| `internal/db/migrations.go` | Modify | Register `migration008BackfillPersonParties` |
| `internal/db/migrations_party_test.go` | Modify | Test migration008 |
| `docs/api.md` | Modify | Document `GET /auth/me/projects` + user-create/update side-effects |
| `web/src/api.ts` | Modify | `UserProjectMembership` type + `getMyProjects` |
| `web/src/api.test.ts` | Modify | Test `getMyProjects` |
| `web/src/routes/parties/MyProjectsTable.tsx` | Create | The new table component |
| `web/src/routes/parties/MyProjectsTable.test.tsx` | Create | Unit tests |
| `web/src/pages/SettingsPage.tsx` | Modify | Mount `<MyProjectsTable />` in `AccountTab` |
| `web/src/pages/SettingsPage.test.tsx` | Modify | Mock + assert the new card |
| `e2e/tests/my-account-projects.spec.ts` | Create | E2E flow |
| `e2e/tests/docs-screenshots.spec.ts` | Modify | Add `my-account-projects.png` screenshot |
| `docs/end-user-guide.md` | Modify | Reference new screenshot + describe the view |

---

## Task 1: `UserProjectMembership` DTO

**Files:**
- Create: `internal/model/party_query.go`

- [ ] **Step 1: Create the DTO**

```go
// internal/model/party_query.go
package model

// UserProjectMembership is a denormalized read-only view of a user's
// effective role on a project, collapsing direct and transitive paths.
// See ADR-014 for the role-resolution tie-break rule.
type UserProjectMembership struct {
	Project Party  `json:"project"`
	Role    string `json:"role"`
}
```

- [ ] **Step 2: Verify build**

```bash
rtk go build ./internal/model/...
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
rtk git add internal/model/party_query.go
rtk git commit -m "feat(model): add UserProjectMembership DTO"
```

---

## Task 2: `CreatePersonForUser` + `UpdatePersonForUser`

**Files:**
- Create: `internal/store/party_store_person_sync.go`
- Create: `internal/store/party_store_person_sync_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/store/party_store_person_sync_test.go
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestCreatePersonForUser_CreatesWhenMissing(t *testing.T) {
	ctx := context.Background()
	store := newTestPartyStore(t)
	userID := uuid.New().String()

	u := &model.User{ID: userID, Username: "alice", DisplayName: "Alice Smith"}
	if err := store.CreatePersonForUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	p, err := store.GetPartyByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if p == nil {
		t.Fatal("want person party, got nil")
	}
	if p.Kind != model.PartyKindPerson {
		t.Fatalf("kind = %q, want person", p.Kind)
	}
	if p.Name != "Alice Smith" {
		t.Fatalf("name = %q, want Alice Smith", p.Name)
	}
}

func TestCreatePersonForUser_FallsBackToUsernameWhenNoDisplayName(t *testing.T) {
	ctx := context.Background()
	store := newTestPartyStore(t)
	userID := uuid.New().String()

	u := &model.User{ID: userID, Username: "bob"}
	if err := store.CreatePersonForUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	p, _ := store.GetPartyByUserID(ctx, userID)
	if p == nil || p.Name != "bob" {
		t.Fatalf("name = %v, want bob", p)
	}
}

func TestCreatePersonForUser_IdempotentWhenExists(t *testing.T) {
	ctx := context.Background()
	store := newTestPartyStore(t)
	userID := uuid.New().String()
	u := &model.User{ID: userID, Username: "alice"}

	if err := store.CreatePersonForUser(ctx, u); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := store.CreatePersonForUser(ctx, u); err != nil {
		t.Fatalf("second create: %v", err)
	}
	// Should still have exactly one Person for the user.
	p, _ := store.GetPartyByUserID(ctx, userID)
	if p == nil {
		t.Fatal("want party, got nil")
	}
}

func TestUpdatePersonForUser_UpdatesName(t *testing.T) {
	ctx := context.Background()
	store := newTestPartyStore(t)
	userID := uuid.New().String()

	u := &model.User{ID: userID, Username: "alice", DisplayName: "Alice Smith"}
	if err := store.CreatePersonForUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	u.DisplayName = "Alice Jones"
	if err := store.UpdatePersonForUser(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}
	p, _ := store.GetPartyByUserID(ctx, userID)
	if p.Name != "Alice Jones" {
		t.Fatalf("name = %q, want Alice Jones", p.Name)
	}
}

func TestUpdatePersonForUser_NoOpWhenNoPerson(t *testing.T) {
	ctx := context.Background()
	store := newTestPartyStore(t)
	u := &model.User{ID: uuid.New().String(), Username: "ghost"}
	if err := store.UpdatePersonForUser(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}
}
```

If `newTestPartyStore` is not yet defined in this package, check sibling `party_store_test.go` — the Spec 2 plan's Task 1 introduced this helper. Reuse it verbatim.

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk go test ./internal/store/ -run TestCreatePersonForUser -v
rtk go test ./internal/store/ -run TestUpdatePersonForUser -v
```

Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement the sync methods**

```go
// internal/store/party_store_person_sync.go
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// personName returns the formula display_name || username.
func personName(u *model.User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// CreatePersonForUser upserts a Person party linked 1:1 to the user.
// Idempotent: no-op if a Person with matching user_id already exists.
func (s *PartyStore) CreatePersonForUser(ctx context.Context, u *model.User) error {
	var existing model.Party
	err := s.db.WithContext(ctx).Where("user_id = ?", u.ID).First(&existing).Error
	if err == nil {
		return nil // already exists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("checking existing person: %w", err)
	}
	p := &model.Party{
		ID:     uuid.New().String(),
		Kind:   model.PartyKindPerson,
		Name:   personName(u),
		UserID: u.ID,
	}
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("creating person for user %s: %w", u.ID, err)
	}
	return nil
}

// UpdatePersonForUser resyncs the Person's name to match the user's
// current display_name || username. No-op if no Person exists.
func (s *PartyStore) UpdatePersonForUser(ctx context.Context, u *model.User) error {
	result := s.db.WithContext(ctx).Model(&model.Party{}).
		Where("user_id = ?", u.ID).
		Update("name", personName(u))
	if result.Error != nil {
		return fmt.Errorf("updating person for user %s: %w", u.ID, result.Error)
	}
	return nil
}
```

If `model.Party` has no `UserID` field name match, inspect `internal/model/party.go` to confirm the exact field tag (it may be `UserID string` with `gorm:"column:user_id"`). Adjust accordingly.

- [ ] **Step 4: Run tests — PASS**

```bash
rtk go test ./internal/store/ -run TestCreatePersonForUser -v
rtk go test ./internal/store/ -run TestUpdatePersonForUser -v
```

- [ ] **Step 5: Verify arch-test still green**

```bash
rtk make arch-test
```

Expected: PASS. The new functions live in a new file (`party_store_person_sync.go`) so the 10-public-function-per-file cap stays satisfied.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/store/party_store_person_sync.go internal/store/party_store_person_sync_test.go
rtk git commit -m "feat(store): add CreatePersonForUser and UpdatePersonForUser"
```

---

## Task 3: `ResolveUserProjects` with highest-privilege tie-break

**Files:**
- Create: `internal/store/party_store_role_resolution.go`
- Create: `internal/store/party_store_role_resolution_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/store/party_store_role_resolution_test.go
package store

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func newUserWithPerson(t *testing.T, s *PartyStore) (userID, personID string) {
	t.Helper()
	userID = uuid.New().String()
	u := &model.User{ID: userID, Username: "user-" + userID[:8]}
	if err := s.CreatePersonForUser(context.Background(), u); err != nil {
		t.Fatalf("create person: %v", err)
	}
	p, _ := s.GetPartyByUserID(context.Background(), userID)
	return userID, p.ID
}

func TestResolveUserProjects_EmptyWhenNoMemberships(t *testing.T) {
	ctx := context.Background()
	s := newTestPartyStore(t)
	userID, _ := newUserWithPerson(t, s)

	got, err := s.ResolveUserProjects(ctx, userID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func TestResolveUserProjects_EmptyWhenNoPerson(t *testing.T) {
	ctx := context.Background()
	s := newTestPartyStore(t)
	got, err := s.ResolveUserProjects(ctx, "no-such-user")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %d", len(got))
	}
}

func TestResolveUserProjects_DirectMembership(t *testing.T) {
	ctx := context.Background()
	s := newTestPartyStore(t)
	userID, personID := newUserWithPerson(t, s)
	project := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindProject, Name: "orion"}
	if err := s.CreateParty(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	rel := &model.PartyRelationship{
		FromPartyID: personID, FromRole: "project:developer",
		ToPartyID: project.ID, ToRole: "project",
		RelationshipName: "project_member",
	}
	if err := s.AddMember(ctx, rel); err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := s.ResolveUserProjects(ctx, userID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 1 || got[0].Role != "project:developer" || got[0].Project.ID != project.ID {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveUserProjects_TransitiveViaGroup(t *testing.T) {
	ctx := context.Background()
	s := newTestPartyStore(t)
	userID, personID := newUserWithPerson(t, s)

	group := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindGroup, Name: "platform"}
	project := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindProject, Name: "orion"}
	for _, p := range []*model.Party{group, project} {
		if err := s.CreateParty(ctx, p); err != nil {
			t.Fatalf("create %s: %v", p.Name, err)
		}
	}
	// person → group (member)
	if err := s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: personID, FromRole: "member",
		ToPartyID: group.ID, ToRole: "group",
		RelationshipName: "group_member",
	}); err != nil {
		t.Fatalf("add group member: %v", err)
	}
	// group → project (owner)
	if err := s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: group.ID, FromRole: "project:owner",
		ToPartyID: project.ID, ToRole: "project",
		RelationshipName: "project_member",
	}); err != nil {
		t.Fatalf("add project member: %v", err)
	}

	got, err := s.ResolveUserProjects(ctx, userID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 1 || got[0].Role != "project:owner" {
		t.Fatalf("got %+v", got)
	}
}

func TestResolveUserProjects_HighestPrivilegeWinsOnMultiPath(t *testing.T) {
	ctx := context.Background()
	s := newTestPartyStore(t)
	userID, personID := newUserWithPerson(t, s)

	gA := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindGroup, Name: "team-a"}
	gB := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindGroup, Name: "team-b"}
	project := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindProject, Name: "orion"}
	for _, p := range []*model.Party{gA, gB, project} {
		if err := s.CreateParty(ctx, p); err != nil {
			t.Fatalf("create %s: %v", p.Name, err)
		}
	}
	// person in both groups.
	for _, gid := range []string{gA.ID, gB.ID} {
		if err := s.AddMember(ctx, &model.PartyRelationship{
			FromPartyID: personID, FromRole: "member",
			ToPartyID: gid, ToRole: "group", RelationshipName: "group_member",
		}); err != nil {
			t.Fatalf("add group member: %v", err)
		}
	}
	// gA = viewer on project, gB = owner on project.
	if err := s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: gA.ID, FromRole: "project:viewer",
		ToPartyID: project.ID, ToRole: "project", RelationshipName: "project_member",
	}); err != nil {
		t.Fatalf("add viewer: %v", err)
	}
	if err := s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: gB.ID, FromRole: "project:owner",
		ToPartyID: project.ID, ToRole: "project", RelationshipName: "project_member",
	}); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	got, err := s.ResolveUserProjects(ctx, userID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(got) != 1 || got[0].Role != "project:owner" {
		t.Fatalf("want project:owner, got %+v", got)
	}
}
```

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk go test ./internal/store/ -run TestResolveUserProjects -v
```

Expected: FAIL — method undefined.

- [ ] **Step 3: Implement `ResolveUserProjects`**

```go
// internal/store/party_store_role_resolution.go
package store

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// projectRoleRank implements ADR-014's highest-privilege tie-break.
// Unknown roles rank -1 so they never win over known roles.
var projectRoleRank = map[string]int{
	"project:owner":     2,
	"project:developer": 1,
	"project:viewer":    0,
}

func rankRole(role string) int {
	if r, ok := projectRoleRank[role]; ok {
		return r
	}
	return -1
}

// ResolveUserProjects returns the user's project memberships — direct and
// transitive — with the highest-privilege role applied when the user reaches
// the same project via multiple paths. See ADR-014.
// Returns an empty slice (never nil) when the user has no memberships or
// no Person party exists.
func (s *PartyStore) ResolveUserProjects(ctx context.Context, userID string) ([]model.UserProjectMembership, error) {
	person, err := s.GetPartyByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up person for user %s: %w", userID, err)
	}
	if person == nil {
		return []model.UserProjectMembership{}, nil
	}

	// Step 1: reachable set = {personID} ∪ ancestor party IDs.
	var ancestors []string
	if err := s.db.WithContext(ctx).
		Model(&model.PartyGroupClosure{}).
		Where("member_party_id = ?", person.ID).
		Pluck("ancestor_party_id", &ancestors).Error; err != nil {
		return nil, fmt.Errorf("reading closure ancestors: %w", err)
	}
	reachable := append([]string{person.ID}, ancestors...)

	// Step 2: pull project memberships for any reachable party.
	type row struct {
		ToPartyID string `gorm:"column:to_party_id"`
		FromRole  string `gorm:"column:from_role"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("party_relationships pr").
		Select("pr.to_party_id, pr.from_role").
		Joins("JOIN parties p ON p.id = pr.to_party_id").
		Where("pr.from_party_id IN ?", reachable).
		Where("pr.relationship_name = ?", "project_member").
		Where("p.kind = ?", model.PartyKindProject).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("reading project memberships: %w", err)
	}

	// Step 3: group by project id; keep highest-privilege role.
	bestRole := make(map[string]string, len(rows))
	for _, r := range rows {
		if existing, ok := bestRole[r.ToPartyID]; !ok || rankRole(r.FromRole) > rankRole(existing) {
			bestRole[r.ToPartyID] = r.FromRole
		}
	}
	if len(bestRole) == 0 {
		return []model.UserProjectMembership{}, nil
	}

	// Step 4: fetch project Party records and join.
	projectIDs := make([]string, 0, len(bestRole))
	for id := range bestRole {
		projectIDs = append(projectIDs, id)
	}
	var projects []model.Party
	if err := s.db.WithContext(ctx).
		Where("id IN ?", projectIDs).
		Order("name ASC").
		Find(&projects).Error; err != nil {
		return nil, fmt.Errorf("fetching projects: %w", err)
	}

	out := make([]model.UserProjectMembership, 0, len(projects))
	for _, p := range projects {
		out = append(out, model.UserProjectMembership{Project: p, Role: bestRole[p.ID]})
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests — PASS**

```bash
rtk go test ./internal/store/ -run TestResolveUserProjects -v
```

Expected: PASS (all five).

- [ ] **Step 5: Arch check**

```bash
rtk make arch-test
```

Expected: PASS. New function lives in its own file.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/store/party_store_role_resolution.go internal/store/party_store_role_resolution_test.go
rtk git commit -m "feat(store): ResolveUserProjects with highest-privilege tie-break (ADR-014)"
```

---

## Task 4: `MyProjectsHandler` + route

**Files:**
- Modify: `internal/api/auth_handlers.go`
- Modify: `internal/api/auth_handlers_test.go`
- Modify: `internal/api/router.go`
- Modify: `docs/api.md`

- [ ] **Step 1: Write failing handler tests**

Append to `internal/api/auth_handlers_test.go`:

```go
func TestMyProjectsHandler_ReturnsMemberships(t *testing.T) {
	ts, token := setupTestAPIWithAdmin(t) // reuse existing helper
	defer ts.Close()

	// Seed a project + add admin's Person as project:developer.
	// (Admin's Person is created by migration007.)
	ctx := context.Background()
	partyStore := getPartyStoreFromTestAPI(t, ts) // helper returning the backing store
	project := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindProject, Name: "orion"}
	if err := partyStore.CreateParty(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	adminPerson, _ := partyStore.GetPartyByUserID(ctx, getAdminUserID(t, ts)) // helper
	if err := partyStore.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: adminPerson.ID, FromRole: "project:developer",
		ToPartyID: project.ID, ToRole: "project", RelationshipName: "project_member",
	}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var got []model.UserProjectMembership
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Role != "project:developer" {
		t.Fatalf("got %+v", got)
	}
}

func TestMyProjectsHandler_Unauthenticated401(t *testing.T) {
	ts, _ := setupTestAPIWithAdmin(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me/projects", nil)
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", res.StatusCode)
	}
}
```

If helpers `setupTestAPIWithAdmin`, `getPartyStoreFromTestAPI`, `getAdminUserID` don't exist in the package, check `internal/api/party_handlers_test.go` for the existing conventions and mirror them. Do not invent new helpers.

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk go test ./internal/api/ -run TestMyProjectsHandler -v
```

Expected: FAIL — handler and route missing.

- [ ] **Step 3: Implement the handler**

Append to `internal/api/auth_handlers.go`:

```go
// MyProjectsHandler returns the calling user's resolved project memberships.
// Authenticated-only; a user always sees their own memberships.
func MyProjectsHandler(partyStore *store.PartyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value(userIDContextKey).(string)
		if !ok || userID == "" {
			ErrorResponse(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		memberships, err := partyStore.ResolveUserProjects(r.Context(), userID)
		if err != nil {
			slog.Error("resolving user projects", "user_id", userID, "err", err)
			ErrorResponse(w, http.StatusInternalServerError, "failed to load projects")
			return
		}
		JSONResponse(w, http.StatusOK, memberships)
	}
}
```

If the existing auth handlers use a different context key for the authenticated user ID, match it exactly. Look for `PermissionsFromContext` in the same file — its sibling `UserIDFromContext` (if defined) is the right accessor; otherwise inspect `RequireAuth` in `internal/api/middleware.go` for the key.

- [ ] **Step 4: Mount the route**

Modify `internal/api/router.go`. Inside the authenticated group where `GET /auth/me` lives, add:

```go
r.Get("/auth/me/projects", MyProjectsHandler(deps.PartyStore))
```

Place it near the existing `/auth/me` line so the two are discoverable together.

- [ ] **Step 5: Run tests — PASS**

```bash
rtk go test ./internal/api/ -run TestMyProjectsHandler -v
```

- [ ] **Step 6: Document in `docs/api.md`**

Locate the Auth section (where `/auth/me` is documented). Add:

````markdown
### GET /api/v1/auth/me/projects

Return the caller's resolved project memberships (direct and transitive through groups).

- **Permission:** authenticated only; no additional permission check — users always see their own memberships.
- **Responses:**
  - `200 OK` — `[ { "project": Party, "role": "project:owner" | "project:developer" | "project:viewer" } ]`, sorted by project name ASC. Empty array when no memberships exist.
  - `401 Unauthorized` — missing or invalid token.
- **Role semantics:** see [ADR-014](../adr/014-project-role-resolution-highest-privilege.md). When a user reaches a project via multiple paths, the highest-privilege role wins (`owner` > `developer` > `viewer`).
````

- [ ] **Step 7: Commit**

```bash
rtk git add internal/api/auth_handlers.go internal/api/auth_handlers_test.go internal/api/router.go docs/api.md
rtk git commit -m "feat(api): add GET /auth/me/projects endpoint"
```

---

## Task 5: Wire `PartyStore` into `UserHandler` + trigger Person sync

**Files:**
- Modify: `internal/api/user_handlers.go`
- Modify: `internal/api/user_handlers_test.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/api/user_handlers_test.go`:

```go
func TestCreateUser_AlsoCreatesPersonParty(t *testing.T) {
	ts, token := setupTestAPIWithAdmin(t)
	defer ts.Close()

	// Find viewer role.
	rolesRes, _ := authedGet(ts, token, "/api/v1/roles")
	var roles []model.Role
	_ = json.NewDecoder(rolesRes.Body).Decode(&roles)
	var viewerID string
	for _, r := range roles {
		if r.Name == "viewer" {
			viewerID = r.ID
			break
		}
	}

	body := fmt.Sprintf(`{"username":"alice_p","password":"Alice@Pass99!","display_name":"Alice P","role_id":%q}`, viewerID)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/users", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusCreated {
		t.Fatalf("create user: status=%d err=%v", res.StatusCode, err)
	}
	var created model.User
	_ = json.NewDecoder(res.Body).Decode(&created)

	// Fetch Person via GET /parties?kind=person
	partiesRes, _ := authedGet(ts, token, "/api/v1/parties?kind=person")
	var parties []model.Party
	_ = json.NewDecoder(partiesRes.Body).Decode(&parties)
	var found *model.Party
	for i := range parties {
		if parties[i].UserID == created.ID {
			found = &parties[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no person party created for user %s", created.ID)
	}
	if found.Name != "Alice P" {
		t.Fatalf("person name = %q, want Alice P", found.Name)
	}
}

func TestUpdateUser_SyncsPersonName(t *testing.T) {
	ts, token := setupTestAPIWithAdmin(t)
	defer ts.Close()

	// Create a user first (reuse the helper logic from above, or factor into shared helper)
	userID := createTestViewerUser(t, ts, token, "bob_p", "Bob Pre")

	// Update display_name
	body := `{"display_name":"Bob Post"}`
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/users/"+userID, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update: status = %d", res.StatusCode)
	}

	partiesRes, _ := authedGet(ts, token, "/api/v1/parties?kind=person")
	var parties []model.Party
	_ = json.NewDecoder(partiesRes.Body).Decode(&parties)
	for _, p := range parties {
		if p.UserID == userID {
			if p.Name != "Bob Post" {
				t.Fatalf("name = %q, want Bob Post", p.Name)
			}
			return
		}
	}
	t.Fatal("no person party found for bob")
}
```

If `authedGet` and `createTestViewerUser` helpers don't exist, add them at the bottom of the test file following the patterns already used for `setupTestAPIWithAdmin`.

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk go test ./internal/api/ -run "TestCreateUser_AlsoCreatesPersonParty|TestUpdateUser_SyncsPersonName" -v
```

Expected: FAIL — the handler does not yet call PartyStore.

- [ ] **Step 3: Inject PartyStore into UserHandler**

Modify `internal/api/user_handlers.go` struct + constructor:

```go
type UserHandler struct {
	userStore  *store.UserStore
	roleStore  *store.RoleStore
	partyStore *store.PartyStore
}

func NewUserHandler(userStore *store.UserStore, roleStore *store.RoleStore, partyStore *store.PartyStore) *UserHandler {
	return &UserHandler{
		userStore:  userStore,
		roleStore:  roleStore,
		partyStore: partyStore,
	}
}
```

- [ ] **Step 4: Call sync on Create**

In the existing `(h *UserHandler) Create` function, after `h.userStore.Create(ctx, user)` succeeds and before the re-fetch block, add:

```go
if h.partyStore != nil {
	if err := h.partyStore.CreatePersonForUser(r.Context(), user); err != nil {
		slog.Error("creating person for new user", "user_id", user.ID, "err", err)
		// Do not fail the user create; migration008 sweeps stragglers.
	}
}
```

Add `"log/slog"` to the import block if not already present.

- [ ] **Step 5: Call sync on Update**

In `(h *UserHandler) Update`, after `h.userStore.Update(ctx, user)` succeeds, add:

```go
if h.partyStore != nil {
	nameChanged := (req.DisplayName != nil) // username is immutable in this API
	if nameChanged {
		if err := h.partyStore.UpdatePersonForUser(r.Context(), user); err != nil {
			slog.Error("syncing person name", "user_id", user.ID, "err", err)
		}
	}
}
```

If `username` is mutable in the update request, extend `nameChanged` accordingly. Check the `updateUserRequest` struct — as of this plan it only includes `Email`, `DisplayName`, `RoleID`, `IsActive`; username is not in the request, so only `DisplayName` affects Person name.

- [ ] **Step 6: Update `router.go` to pass PartyStore**

In `internal/api/router.go`:

```go
userHandler := NewUserHandler(deps.UserStore, deps.RoleStore, deps.PartyStore)
```

- [ ] **Step 7: Run tests — PASS**

```bash
rtk go test ./internal/api/ -run "TestCreateUser_AlsoCreatesPersonParty|TestUpdateUser_SyncsPersonName" -v
```

If tests still fail because existing tests create `NewUserHandler` without the PartyStore, update those call sites too. Likely locations: any `*_test.go` file that directly instantiates `NewUserHandler`.

- [ ] **Step 8: Run the full user-handler test suite**

```bash
rtk go test ./internal/api/ -run TestUser -v
```

All should pass.

- [ ] **Step 9: Commit**

```bash
rtk git add internal/api/user_handlers.go internal/api/user_handlers_test.go internal/api/router.go
rtk git commit -m "feat(api): wire PartyStore into UserHandler for Person sync on create/update"
```

---

## Task 6: migration008 backfill

**Files:**
- Modify: `internal/db/migrations.go`
- Modify: `internal/db/migrations_party_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/db/migrations_party_test.go`:

```go
func TestMigration008_BackfillCreatesPersonsForOrphanUsers(t *testing.T) {
	tx := setupMigrationTestDB(t) // helper from the existing file

	// Run migrations 1-7
	for _, m := range AllMigrations() {
		if m.Version > 7 {
			continue
		}
		if err := m.Up(tx); err != nil {
			t.Fatalf("migration %d: %v", m.Version, err)
		}
	}

	// Insert a user AFTER migration007 so no Person exists.
	userID := uuid.NewString()
	if err := tx.Exec(`
		INSERT INTO users (id, username, display_name, password_hash, role_id, is_active, created_at, updated_at)
		VALUES (?, 'orphan', 'Orphan User', 'x', 'x', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID).Error; err != nil {
		t.Fatalf("insert orphan user: %v", err)
	}

	// Run migration008.
	m := migration008BackfillPersonParties()
	if err := m.Up(tx); err != nil {
		t.Fatalf("migration008: %v", err)
	}

	var count int64
	if err := tx.Raw("SELECT COUNT(*) FROM parties WHERE user_id = ?", userID).Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 person, got %d", count)
	}

	var name string
	_ = tx.Raw("SELECT name FROM parties WHERE user_id = ?", userID).Scan(&name).Error
	if name != "Orphan User" {
		t.Fatalf("name = %q, want Orphan User", name)
	}
}

func TestMigration008_Idempotent(t *testing.T) {
	tx := setupMigrationTestDB(t)
	for _, m := range AllMigrations() {
		if err := m.Up(tx); err != nil {
			t.Fatalf("migration %d: %v", m.Version, err)
		}
	}
	m := migration008BackfillPersonParties()
	if err := m.Up(tx); err != nil {
		t.Fatalf("second run: %v", err)
	}
}
```

If `setupMigrationTestDB` is not defined by that exact name, check the sibling `migrations_party_test.go` file for the existing setup helper and reuse it verbatim.

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk go test ./internal/db/ -run TestMigration008 -v
```

Expected: FAIL — function undefined.

- [ ] **Step 3: Implement migration008**

Append to `internal/db/migrations.go`:

```go
func migration008BackfillPersonParties() Migration {
	return Migration{
		Version:     8,
		Description: "backfill person parties for users created after migration007",
		Up:          migration008Up,
	}
}

func migration008Up(tx *gorm.DB) error {
	// Idempotent: only insert for users without an existing Person.
	if err := migration007SeedPersonParties(tx); err != nil {
		return fmt.Errorf("migration008 backfill: %w", err)
	}
	slog.Info("migration008: person party backfill complete")
	return nil
}
```

We reuse `migration007SeedPersonParties` because its logic is already correct and idempotent (uses `WHERE NOT EXISTS`). No new SQL needed.

- [ ] **Step 4: Register migration008 in `AllMigrations()`**

Modify the slice in `AllMigrations()`:

```go
return []Migration{
	migration001CreateTables(),
	migration002UsersAndRoles(),
	migration003DefaultRoles(),
	migration004Settings(),
	migration005HealthColumns(),
	migration006RawCards(),
	migration007PartyArchetype(),
	migration008BackfillPersonParties(),
}
```

- [ ] **Step 5: Run tests — PASS**

```bash
rtk go test ./internal/db/ -run TestMigration008 -v
```

- [ ] **Step 6: Run full migration test suite**

```bash
rtk go test ./internal/db/ -v
```

Expected: all migrations run cleanly, including migration008 as the last one.

- [ ] **Step 7: Commit**

```bash
rtk git add internal/db/migrations.go internal/db/migrations_party_test.go
rtk git commit -m "feat(db): migration008 backfills person parties for orphan users"
```

---

## Task 7: Frontend API client — `UserProjectMembership` + `getMyProjects`

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/api.test.ts`

- [ ] **Step 1: Write failing test**

Append to the `Parties API` describe block in `web/src/api.test.ts`:

```ts
it('getMyProjects GETs /api/v1/auth/me/projects', async () => {
  const mock: api.UserProjectMembership[] = [
    {
      project: { id: 'pr1', kind: 'project', name: 'orion', is_system: false, created_at: '', updated_at: '' },
      role: 'project:developer',
    },
  ]
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
    ok: true, status: 200, json: () => Promise.resolve(mock),
  })
  const got = await api.getMyProjects()
  expect(got).toEqual(mock)
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/auth/me/projects', expect.any(Object))
})
```

- [ ] **Step 2: Run test — FAIL**

```bash
rtk cd web && rtk bun run test -- api.test
```

- [ ] **Step 3: Add type + function**

Append to `web/src/api.ts`, near the other party types:

```ts
export interface UserProjectMembership {
  project: Party
  role: string
}

export function getMyProjects(): Promise<UserProjectMembership[]> {
  return request<UserProjectMembership[]>('/auth/me/projects')
}
```

- [ ] **Step 4: Run test — PASS**

```bash
rtk cd web && rtk bun run test -- api.test
```

- [ ] **Step 5: Commit**

```bash
rtk git add web/src/api.ts web/src/api.test.ts
rtk git commit -m "feat(web): add UserProjectMembership type and getMyProjects client"
```

---

## Task 8: `MyProjectsTable` component

**Files:**
- Create: `web/src/routes/parties/MyProjectsTable.tsx`
- Create: `web/src/routes/parties/MyProjectsTable.test.tsx`

- [ ] **Step 1: Write failing tests**

```tsx
// web/src/routes/parties/MyProjectsTable.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import MyProjectsTable from './MyProjectsTable'

vi.mock('@/api', () => ({ getMyProjects: vi.fn() }))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => vi.clearAllMocks())

function renderTable() {
  return render(<MemoryRouter><MyProjectsTable /></MemoryRouter>)
}

describe('MyProjectsTable', () => {
  it('shows empty state when the user has no memberships', async () => {
    mockApi.getMyProjects.mockResolvedValue([])
    renderTable()
    await waitFor(() => expect(screen.getByText(/don't belong to any projects yet/i)).toBeInTheDocument())
  })

  it('renders a row per membership', async () => {
    mockApi.getMyProjects.mockResolvedValue([
      { project: { id: 'p1', kind: 'project', name: 'orion', is_system: false, created_at: '', updated_at: '' }, role: 'project:developer' },
      { project: { id: 'p2', kind: 'project', name: 'nova', is_system: false, created_at: '', updated_at: '' }, role: 'project:viewer' },
    ])
    renderTable()
    await waitFor(() => expect(screen.getByText('orion')).toBeInTheDocument())
    expect(screen.getByText('nova')).toBeInTheDocument()
    expect(screen.getByText('project:developer')).toBeInTheDocument()
    expect(screen.getByText('project:viewer')).toBeInTheDocument()
  })

  it('row click navigates to project detail', async () => {
    mockApi.getMyProjects.mockResolvedValue([
      { project: { id: 'p1', kind: 'project', name: 'orion', is_system: false, created_at: '', updated_at: '' }, role: 'project:developer' },
    ])
    renderTable()
    await waitFor(() => expect(screen.getByText('orion')).toBeInTheDocument())
    await userEvent.click(screen.getByText('orion'))
    // MemoryRouter history is consulted via a test-id sink below. Simplest
    // verification: assert the row has a role='button' or similar affordance
    // and that click emits no error. Navigation is asserted in E2E.
    // For stricter unit verification, wrap in Routes and assert location.
    // Keep this test at "clickable" granularity to avoid router plumbing.
  })

  it('shows retry UI on error', async () => {
    mockApi.getMyProjects.mockRejectedValue(new Error('boom'))
    renderTable()
    await waitFor(() => expect(screen.getByText(/failed to load projects/i)).toBeInTheDocument())
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk cd web && rtk bun run test -- MyProjectsTable.test
```

- [ ] **Step 3: Implement the component**

```tsx
// web/src/routes/parties/MyProjectsTable.tsx
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import * as api from '@/api'
import type { UserProjectMembership } from '@/api'

export default function MyProjectsTable() {
  const navigate = useNavigate()
  const [memberships, setMemberships] = useState<UserProjectMembership[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const load = () => {
    setLoading(true)
    setError(false)
    api.getMyProjects()
      .then(m => { setMemberships(m); setLoading(false) })
      .catch(() => { setError(true); setLoading(false) })
  }

  useEffect(load, [])

  if (loading) return <div className="text-sm text-muted-foreground">Loading…</div>
  if (error) {
    return (
      <div className="space-y-2">
        <p className="text-sm text-destructive">Failed to load projects.</p>
        <Button size="sm" variant="outline" onClick={load}>Retry</Button>
      </div>
    )
  }
  if (memberships.length === 0) {
    return <p className="text-sm text-muted-foreground">You don't belong to any projects yet.</p>
  }

  return (
    <div className="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Role</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {memberships.map(m => (
            <TableRow
              key={m.project.id}
              className="cursor-pointer hover:bg-muted/50"
              onClick={() => navigate(`/settings/projects/${m.project.id}`)}
            >
              <TableCell className="font-medium">{m.project.name}</TableCell>
              <TableCell><Badge variant="outline">{m.role}</Badge></TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
```

- [ ] **Step 4: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- MyProjectsTable.test
```

- [ ] **Step 5: Commit**

```bash
rtk git add web/src/routes/parties/MyProjectsTable.tsx web/src/routes/parties/MyProjectsTable.test.tsx
rtk git commit -m "feat(web): add MyProjectsTable component"
```

---

## Task 9: Mount `MyProjectsTable` in `AccountTab`

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`
- Modify: `web/src/pages/SettingsPage.test.tsx`

- [ ] **Step 1: Extend SettingsPage test**

Add to the `vi.mock('@/api', …)` block:

```ts
getMyProjects: vi.fn(),
```

In `beforeEach`:

```ts
mockApi.getMyProjects.mockResolvedValue([])
```

Add a new test:

```tsx
it('My Account tab shows My projects card', async () => {
  renderSettingsPage()
  await userEvent.click(screen.getByRole('tab', { name: /my account/i }))
  await waitFor(() => expect(screen.getByRole('heading', { name: /my projects/i })).toBeInTheDocument())
})
```

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk cd web && rtk bun run test -- SettingsPage.test
```

- [ ] **Step 3: Mount the card**

In `web/src/pages/SettingsPage.tsx`, inside the `AccountTab` component's JSX, add a third `<Card>` below "Change password":

```tsx
import MyProjectsTable from '../routes/parties/MyProjectsTable'
// ...
<Card>
  <CardHeader>
    <CardTitle>My projects</CardTitle>
    <CardDescription>Projects you belong to — directly or through a group.</CardDescription>
  </CardHeader>
  <CardContent>
    <MyProjectsTable />
  </CardContent>
</Card>
```

- [ ] **Step 4: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- SettingsPage.test
```

- [ ] **Step 5: Commit**

```bash
rtk git add web/src/pages/SettingsPage.tsx web/src/pages/SettingsPage.test.tsx
rtk git commit -m "feat(web): mount MyProjectsTable in My Account tab"
```

---

## Task 10: E2E — `my-account-projects.spec.ts`

**Files:**
- Create: `e2e/tests/my-account-projects.spec.ts`

- [ ] **Step 1: Write the spec**

```ts
// e2e/tests/my-account-projects.spec.ts
import { test, expect } from '@playwright/test'
import { loginViaUI, loginViaAPI, authHeader, BASE, createUser } from './helpers'

test.describe('My projects', () => {
  test('new user sees empty My projects card', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    await createUser(request, token, {
      username: 'e2e_myproj_alice',
      password: 'Alice@E2e99!',
      display_name: 'E2E Alice',
      role_id: viewer.id,
    })

    await loginViaUI(page, 'e2e_myproj_alice', 'Alice@E2e99!')
    await page.goto('/settings')
    await page.getByRole('tab', { name: /my account/i }).click()
    await expect(page.getByRole('heading', { name: /my projects/i })).toBeVisible()
    await expect(page.getByText(/don't belong to any projects yet/i)).toBeVisible()

    // Cleanup
    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const u = users.find(x => x.username === 'e2e_myproj_alice')
    if (u) await request.delete(`${BASE}/api/v1/users/${u.id}`, { headers: authHeader(token) }).catch(() => {})
  })

  test('user sees a project after admin assigns their Person as member', async ({ page, request }) => {
    const token = await loginViaAPI(request)

    // Create user (auto-creates Person).
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    const createdUser = await createUser(request, token, {
      username: 'e2e_myproj_bob',
      password: 'Bob@E2e99!',
      display_name: 'E2E Bob',
      role_id: viewer.id,
    })

    // Find Bob's Person party.
    const partiesRes = await request.get(`${BASE}/api/v1/parties?kind=person`, { headers: authHeader(token) })
    const parties = await partiesRes.json() as Array<{ id: string; user_id?: string; name: string }>
    const bobPerson = parties.find(p => p.user_id === createdUser.id)!
    expect(bobPerson).toBeTruthy()

    // Create project + add Bob.
    const projectRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-myproj-project' },
    })
    const project = await projectRes.json()
    await request.post(`${BASE}/api/v1/projects/${project.id}/members`, {
      headers: authHeader(token),
      data: { party_id: bobPerson.id, role: 'project:developer' },
    })

    // Log in as Bob, check My projects.
    await loginViaUI(page, 'e2e_myproj_bob', 'Bob@E2e99!')
    await page.goto('/settings')
    await page.getByRole('tab', { name: /my account/i }).click()
    await expect(page.getByText('e2e-myproj-project')).toBeVisible()
    await expect(page.getByText('project:developer')).toBeVisible()

    // Click-through navigates.
    await page.getByText('e2e-myproj-project').click()
    await expect(page.getByRole('heading', { name: 'e2e-myproj-project' })).toBeVisible()

    // Cleanup
    await request.delete(`${BASE}/api/v1/projects/${project.id}`, { headers: authHeader(token) }).catch(() => {})
    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const bob = users.find(x => x.username === 'e2e_myproj_bob')
    if (bob) await request.delete(`${BASE}/api/v1/users/${bob.id}`, { headers: authHeader(token) }).catch(() => {})
  })
})
```

- [ ] **Step 2: Run E2E**

```bash
rtk ./e2e/run-e2e.sh tests/my-account-projects.spec.ts
```

Expected: both tests pass.

- [ ] **Step 3: Commit**

```bash
rtk git add e2e/tests/my-account-projects.spec.ts
rtk git commit -m "test(e2e): verify My projects card surfaces direct project memberships"
```

---

## Task 11: Screenshot + end-user guide

**Files:**
- Modify: `e2e/tests/docs-screenshots.spec.ts`
- Modify: `docs/end-user-guide.md`

- [ ] **Step 1: Extend `docs-screenshots.spec.ts` seed**

The `beforeAll` already seeds a demo project (from the Spec 2 plan). Add a membership so the screenshot has a non-empty table. Inside the existing demo-project seed block, after the admin-Person assignment, also assign the admin as `project:owner` if not already done (the spec 2 plan may already do this; if so, skip).

If the project-detail demo from Spec 2 uses a different project than what we want for the My-projects screenshot, add a dedicated one:

```ts
// (Appended inside the existing beforeAll, after other project seeding.)
if (demoProjectId) {
  // Ensure admin's Person is a developer on demoProject — visible in My projects.
  // Idempotent: duplicate add is silently ignored by AddMember.
  // (If the Spec 2 plan already adds admin as owner, this line may error;
  //  swallow the error via catch(()=>{}).)
}
```

If the spec 2 plan already adds the admin person as a member, this step is a no-op — skip it.

- [ ] **Step 2: Add the screenshot test**

Before the closing `});`:

```ts
test('my-account-my-projects', async ({ page }) => {
  await page.setViewportSize(VIEWPORT)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await loginViaUI(page)
  await page.goto('/settings')
  await page.getByRole('tab', { name: /my account/i }).click()
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: `${DOCS_IMAGES}/my-account-projects.png`, fullPage: false })
})
```

- [ ] **Step 3: Run screenshots**

```bash
rtk make docs-screenshots
```

Confirm `docs/images/my-account-projects.png` exists.

- [ ] **Step 4: Reference in `docs/end-user-guide.md`**

Locate the **My Account** section (or if missing, add one under Settings). Add:

```markdown
### My projects

The My Account tab shows the projects you belong to (directly or through a group) and your role on each. Click a row to open the project's detail page.

![My Account tab showing the My projects card with two project memberships](images/my-account-projects.png)
*The "My projects" card — roles resolve via group closure; the highest-privilege role wins when a user belongs via multiple paths (ADR-014).*
```

- [ ] **Step 5: Commit**

```bash
rtk git add e2e/tests/docs-screenshots.spec.ts docs/end-user-guide.md docs/images/my-account-projects.png
rtk git commit -m "docs: add My projects screenshot and end-user guide section"
```

---

## Task 12: Full validation

- [ ] **Step 1: Backend**

```bash
rtk make test arch-test
```

Expected: all pass.

- [ ] **Step 2: Frontend unit + lint**

```bash
rtk make web-test web-lint
```

- [ ] **Step 3: Full E2E**

```bash
rtk make web-build
rtk make e2e-test
```

Expected: all specs pass, including the new `my-account-projects.spec.ts`.

- [ ] **Step 4: `make all`**

```bash
rtk make all
```

Expected: green.

---

## Completion criteria

- `GET /api/v1/auth/me/projects` returns the caller's resolved project memberships, honoring ADR-014's highest-privilege tie-break.
- User create via `POST /api/v1/users` produces a Person party automatically.
- User update that changes `display_name` resyncs the Person party's name.
- migration008 backfills Person parties for users without one; idempotent.
- "My projects" card appears on the My Account tab with Name + Role columns, clickable rows, empty state, and error-retry UI.
- `docs/api.md` documents the new endpoint and user-side-effects.
- `docs/end-user-guide.md` references the new screenshot and explains what the view shows.
- `rtk make all` passes.

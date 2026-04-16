# Projects UI + Catalog Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Projects management UI (mirroring Groups) and a catalog project filter. Generalize Groups components into a kind-parameterized `parties/` module so both kinds share one implementation.

**Architecture:** Rename `web/src/routes/groups/` → `web/src/routes/parties/`, parameterize all components by a `PartyUIConfig` map, then mount two configs: `groupUIConfig` (unchanged behavior) and `projectUIConfig` (new tab + detail page + member roles + entries panel). Add one backend endpoint: `PATCH /api/v1/projects/:id/members/:memberID` for in-place role updates. Extend `listCatalog` with `project` filter and add a Project Select to the catalog page.

**Tech Stack:** React 18, react-router-dom v6, TypeScript, Vite, Tailwind, shadcn/ui, TanStack React Query, Vitest + React Testing Library, Playwright. Backend Go 1.26.1 + GORM.

**Read before starting:**

- [docs/superpowers/specs/2026-04-15-projects-ui-design.md](../specs/2026-04-15-projects-ui-design.md) — this spec
- [docs/superpowers/plans/2026-04-15-groups-ui.md](2026-04-15-groups-ui.md) — predecessor plan; this one mirrors its shape
- [internal/api/party_kind_configs.go](../../../internal/api/party_kind_configs.go) — backend kind config (mirror on frontend)
- [internal/api/party_handlers.go](../../../internal/api/party_handlers.go) — where PATCH handler goes
- [internal/store/party_store.go](../../../internal/store/party_store.go) — where UpdateMemberRole goes
- [web/src/routes/groups/](../../../web/src/routes/groups/) — source for rename
- [web/src/hooks/useCatalogQuery.ts](../../../web/src/hooks/useCatalogQuery.ts) — filter pattern to extend
- [web/src/types.ts](../../../web/src/types.ts) — `ListFilter` interface to extend
- [docs/adr/011-party-archetype-unified-actor-model.md](../../adr/011-party-archetype-unified-actor-model.md) — compliance gate
- [docs/adr/012-materialized-closure.md](../../adr/012-materialized-closure.md) — explains why in-place role update beats remove+add

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/store/party_store.go` | Modify | Add `UpdateMemberRole(ctx, fromPartyID, toPartyID, relName, newRole)` |
| `internal/store/party_store_test.go` | Modify | Test `UpdateMemberRole` |
| `internal/api/party_handlers.go` | Modify | Add `UpdateMemberRoleHandler` |
| `internal/api/router.go` | Modify | Mount `PATCH /projects/:id/members/:memberID` |
| `internal/api/party_handlers_test.go` | Modify | Test PATCH handler |
| `docs/api.md` | Modify | Document PATCH route |
| `web/src/routes/groups/` | Rename via `git mv` | Source tree renamed to `web/src/routes/parties/` |
| `web/src/routes/parties/partyUIConfig.ts` | Create | `PartyUIConfig` type + `groupUIConfig`, `projectUIConfig` |
| `web/src/routes/parties/PartyTab.tsx` | Rename/Modify | Parameterized list+create+delete (from `GroupsTab.tsx`) |
| `web/src/routes/parties/PartyTab.test.tsx` | Rename/Modify | Parametric tests |
| `web/src/routes/parties/CreatePartyDialog.tsx` | Rename/Modify | Parameterized create dialog (from `CreateGroupDialog.tsx`) |
| `web/src/routes/parties/CreatePartyDialog.test.tsx` | Rename/Modify | |
| `web/src/routes/parties/PartyDetailPage.tsx` | Rename/Modify | Parameterized detail page (from `GroupDetailPage.tsx`) + role column + entries panel gates |
| `web/src/routes/parties/PartyDetailPage.test.tsx` | Rename/Modify | |
| `web/src/routes/parties/AddMemberDialog.tsx` | Rename/Modify | Optional `roleOptions` prop |
| `web/src/routes/parties/AddMemberDialog.test.tsx` | Rename/Modify | |
| `web/src/routes/parties/EditMemberRoleDialog.tsx` | Create | Edit role dialog (project-only) |
| `web/src/routes/parties/EditMemberRoleDialog.test.tsx` | Create | |
| `web/src/routes/parties/ProjectEntriesPanel.tsx` | Create | Assigned catalog entries panel |
| `web/src/routes/parties/ProjectEntriesPanel.test.tsx` | Create | |
| `web/src/routes/parties/AssignEntryDialog.tsx` | Create | Search-then-pick dialog |
| `web/src/routes/parties/AssignEntryDialog.test.tsx` | Create | |
| `web/src/api.ts` | Modify | Project CRUD + member PATCH + entry assign/unassign; extend `listCatalog({project})` |
| `web/src/api.test.ts` | Modify | Tests for the new API functions |
| `web/src/types.ts` | Modify | Add `project?: string` to `ListFilter` |
| `web/src/hooks/useCatalogQuery.ts` | Modify | Read/write `?project=` URL param + include in `listCatalog` call |
| `web/src/hooks/useCatalogQuery.test.tsx` | Modify | Test new `setProject` behavior |
| `web/src/pages/SettingsPage.tsx` | Modify | Mount `<PartyTab config={projectUIConfig}/>` as new "Projects" tab |
| `web/src/pages/SettingsPage.test.tsx` | Modify | Mock + assert Projects tab |
| `web/src/App.tsx` | Modify | Add `/settings/projects/:id` route |
| `web/src/routes/catalog/CatalogListPage.tsx` | Modify | Add Project Select to filter bar |
| `web/src/routes/catalog/CatalogListPage.test.tsx` | Modify | Test Project filter |
| `e2e/tests/projects.spec.ts` | Create | End-to-end Projects scenarios |
| `e2e/tests/catalog.spec.ts` | Modify | E2E catalog-project filter |
| `e2e/tests/docs-screenshots.spec.ts` | Modify | `projects-tab.png`, `project-detail.png` screenshots |
| `docs/end-user-guide.md` | Modify | Reference new screenshots + document Projects UI |

---

## Task 1: Backend — `UpdateMemberRole` store method

**Files:**
- Modify: `internal/store/party_store.go`
- Modify: `internal/store/party_store_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/store/party_store_test.go`:

```go
func TestPartyStore_UpdateMemberRole(t *testing.T) {
	ctx := context.Background()
	store := newTestPartyStore(t)

	// Seed a project + a person, add as member with role project:viewer.
	project := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindProject, Name: "p1"}
	if err := store.CreateParty(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	person := &model.Party{ID: uuid.New().String(), Kind: model.PartyKindPerson, Name: "alice"}
	if err := store.CreateParty(ctx, person); err != nil {
		t.Fatalf("create person: %v", err)
	}
	rel := &model.PartyRelationship{
		FromPartyID:      person.ID,
		FromRole:         "project:viewer",
		ToPartyID:        project.ID,
		ToRole:           "project",
		RelationshipName: "project_member",
	}
	if err := store.AddMember(ctx, rel); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Update to project:developer.
	if err := store.UpdateMemberRole(ctx, person.ID, project.ID, "project_member", "project:developer"); err != nil {
		t.Fatalf("update: %v", err)
	}

	rels, err := store.ListMembers(ctx, project.ID, "project_member")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("want 1 rel, got %d", len(rels))
	}
	if rels[0].FromRole != "project:developer" {
		t.Fatalf("want role project:developer, got %q", rels[0].FromRole)
	}
}

func TestPartyStore_UpdateMemberRole_NotFound(t *testing.T) {
	ctx := context.Background()
	store := newTestPartyStore(t)

	err := store.UpdateMemberRole(ctx, "missing", "also-missing", "project_member", "project:viewer")
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
```

If `newTestPartyStore` does not yet exist, inspect the existing tests in the file and reuse whatever helper they use (likely `store.NewSQLiteStore(":memory:")`).

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk go test ./internal/store/ -run TestPartyStore_UpdateMemberRole -v
```

Expected: FAIL — `UpdateMemberRole` undefined.

- [ ] **Step 3: Implement `UpdateMemberRole`**

Add to `internal/store/party_store.go` below `RemoveMember`:

```go
// UpdateMemberRole changes the FromRole of an existing PartyRelationship in place.
// No closure rebuild is performed — the containment graph is unchanged.
// Returns an error if no matching relationship exists.
func (s *PartyStore) UpdateMemberRole(ctx context.Context, fromPartyID, toPartyID, relationshipName, newRole string) error {
	result := s.db.WithContext(ctx).Model(&model.PartyRelationship{}).
		Where("from_party_id = ? AND to_party_id = ? AND relationship_name = ?", fromPartyID, toPartyID, relationshipName).
		Update("from_role", newRole)
	if result.Error != nil {
		return fmt.Errorf("updating member role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("no member relationship found for from=%s to=%s name=%s", fromPartyID, toPartyID, relationshipName)
	}
	return nil
}
```

- [ ] **Step 4: Run tests — PASS**

```bash
rtk go test ./internal/store/ -run TestPartyStore_UpdateMemberRole -v
```

Expected: PASS.

- [ ] **Step 5: Arch check still green (we added 1 public method — back to 11 which violates max 10)**

If `rtk make arch-test` fails with `has too many public functions`, move one of the existing public helpers into an unexported helper OR split `party_store.go` by creating `internal/store/party_store_members.go` and move `AddMember`, `RemoveMember`, `ListMembers`, `UpdateMemberRole`, and `AncestorGroupIDs` there. Verify with:

```bash
rtk make arch-test
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add internal/store/party_store.go internal/store/party_store_test.go
rtk git commit -m "feat(store): add UpdateMemberRole for in-place role edits"
```

---

## Task 2: Backend — `PATCH /api/v1/projects/:id/members/:memberID` handler

**Files:**
- Modify: `internal/api/party_handlers.go`
- Modify: `internal/api/router.go`
- Modify: `internal/api/party_handlers_test.go`
- Modify: `docs/api.md`

- [ ] **Step 1: Write failing handler test**

Append to `internal/api/party_handlers_test.go` (adapt `newTestAPI` helper name to whatever is already used in that file):

```go
func TestUpdateMemberRoleHandler_Success(t *testing.T) {
	ts, token := setupTestAPIWithAdmin(t) // or equivalent helper already in this file
	defer ts.Close()

	projectID := createTestParty(t, ts, token, "project", "p1")
	memberID := createTestParty(t, ts, token, "person", "alice")
	addMember(t, ts, token, "projects", projectID, memberID, "project:viewer")

	req, _ := http.NewRequest(http.MethodPatch,
		ts.URL+"/api/v1/projects/"+projectID+"/members/"+memberID,
		strings.NewReader(`{"role":"project:developer"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", res.StatusCode)
	}
}

func TestUpdateMemberRoleHandler_InvalidRole(t *testing.T) {
	ts, token := setupTestAPIWithAdmin(t)
	defer ts.Close()

	projectID := createTestParty(t, ts, token, "project", "p1")
	memberID := createTestParty(t, ts, token, "person", "alice")
	addMember(t, ts, token, "projects", projectID, memberID, "project:viewer")

	req, _ := http.NewRequest(http.MethodPatch,
		ts.URL+"/api/v1/projects/"+projectID+"/members/"+memberID,
		strings.NewReader(`{"role":"bogus"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", res.StatusCode)
	}
}

func TestUpdateMemberRoleHandler_PermissionDenied(t *testing.T) {
	ts, _ := setupTestAPIWithAdmin(t)
	defer ts.Close()
	viewerToken := makeViewerToken(t, ts) // or equivalent helper

	req, _ := http.NewRequest(http.MethodPatch,
		ts.URL+"/api/v1/projects/any/members/any",
		strings.NewReader(`{"role":"project:viewer"}`))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	req.Header.Set("Content-Type", "application/json")

	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", res.StatusCode)
	}
}
```

If test helpers `setupTestAPIWithAdmin`, `createTestParty`, `addMember`, `makeViewerToken` do not exist in this file, look at `internal/api/party_handlers_test.go` for existing pattern and follow it. Do not re-invent helpers.

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk go test ./internal/api/ -run TestUpdateMemberRoleHandler -v
```

Expected: FAIL — handler and route both missing.

- [ ] **Step 3: Add handler**

Append to `internal/api/party_handlers.go`:

```go
type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

// UpdateMemberRoleHandler changes a member's role on a party in place.
func UpdateMemberRoleHandler(cfg PartyKindConfig, ps *store.PartyStore) http.HandlerFunc {
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
		memberPartyID := chi.URLParam(r, "memberPartyID")
		var req updateMemberRoleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			ErrorResponse(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Role == "" || !validRoles[req.Role] {
			ErrorResponse(w, http.StatusBadRequest, "invalid role for this party kind")
			return
		}
		if err := ps.UpdateMemberRole(r.Context(), memberPartyID, toPartyID, cfg.MemberRelationship, req.Role); err != nil {
			slog.Error("updating member role", "err", err)
			ErrorResponse(w, http.StatusNotFound, "member not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
```

- [ ] **Step 4: Register the route**

Modify `internal/api/router.go` inside `RegisterPartyKindRoutes`:

```go
r.Route("/{partyID}", func(r chi.Router) {
    r.Get("/", GetPartyHandler(cfg, partyStore))
    r.Delete("/", DeletePartyHandler(cfg, partyStore))
    r.Get("/members", ListMembersHandler(cfg, partyStore))
    r.Post("/members", AddMemberHandler(cfg, partyStore))
    r.Delete("/members/{memberPartyID}", RemoveMemberHandler(cfg, partyStore))
    r.Patch("/members/{memberPartyID}", UpdateMemberRoleHandler(cfg, partyStore))
})
```

- [ ] **Step 5: Run tests — PASS**

```bash
rtk go test ./internal/api/ -run TestUpdateMemberRoleHandler -v
```

Expected: PASS (all three).

- [ ] **Step 6: Document the route in `docs/api.md`**

Locate the Party Archetype section. Add under Project members:

````markdown
### PATCH /api/v1/projects/{projectID}/members/{memberID}

Update a member's role on a project in place.

- **Permission:** `catalog:write`
- **Body:** `{"role": "project:owner" | "project:developer" | "project:viewer"}`
- **Responses:**
  - `204 No Content` — role updated
  - `400 Bad Request` — missing or invalid role
  - `403 Forbidden` — caller lacks `catalog:write`
  - `404 Not Found` — no such membership
````

- [ ] **Step 7: Commit**

```bash
rtk git add internal/api/party_handlers.go internal/api/router.go internal/api/party_handlers_test.go docs/api.md
rtk git commit -m "feat(api): add PATCH /projects/:id/members/:memberID for in-place role edit"
```

---

## Task 3: Rename `routes/groups/` → `routes/parties/` (no behavior change)

**Files:** all of `web/src/routes/groups/*`, plus imports in `web/src/App.tsx`, `web/src/pages/SettingsPage.tsx`.

- [ ] **Step 1: Git-move the directory**

```bash
rtk git mv web/src/routes/groups web/src/routes/parties
```

- [ ] **Step 2: Update imports**

In `web/src/App.tsx`:

```tsx
import GroupDetailPage from './routes/parties/GroupDetailPage'
```

In `web/src/pages/SettingsPage.tsx`:

```tsx
import GroupsTab from '../routes/parties/GroupsTab'
```

- [ ] **Step 3: Run tests — PASS (no logic change)**

```bash
rtk cd web && rtk bun run test -- parties
rtk make web-lint
```

Expected: all existing Group tests still pass under the new path.

- [ ] **Step 4: Commit**

```bash
rtk git add -A
rtk git commit -m "refactor(web): rename routes/groups to routes/parties"
```

---

## Task 4: `partyUIConfig.ts` + rename components to `Party*`

**Files:**
- Create: `web/src/routes/parties/partyUIConfig.ts`
- Rename (via `git mv`): `GroupsTab.tsx` → `PartyTab.tsx`, `GroupsTab.test.tsx` → `PartyTab.test.tsx`, `CreateGroupDialog.tsx` → `CreatePartyDialog.tsx`, `CreateGroupDialog.test.tsx` → `CreatePartyDialog.test.tsx`, `GroupDetailPage.tsx` → `PartyDetailPage.tsx`, `GroupDetailPage.test.tsx` → `PartyDetailPage.test.tsx`

- [ ] **Step 1: Create `partyUIConfig.ts`**

```bash
rtk cat > web/src/routes/parties/partyUIConfig.ts <<'EOF'
export interface PartyUIConfig {
  kind: 'group' | 'project'
  urlPrefix: 'groups' | 'projects'
  detailPath: (id: string) => string
  labels: { single: string; plural: string }
  writePermission: string
  memberRoleOptions: string[]
  defaultMemberRole: string
  showMemberRoleColumn: boolean
  showEntriesPanel: boolean
  cycleErrorMessage: string
}

export const groupUIConfig: PartyUIConfig = {
  kind: 'group',
  urlPrefix: 'groups',
  detailPath: (id) => `/settings/groups/${id}`,
  labels: { single: 'Group', plural: 'Groups' },
  writePermission: 'users:write',
  memberRoleOptions: ['member'],
  defaultMemberRole: 'member',
  showMemberRoleColumn: false,
  showEntriesPanel: false,
  cycleErrorMessage: "This member is already in the group's ancestry — adding them would create a cycle.",
}

export const projectUIConfig: PartyUIConfig = {
  kind: 'project',
  urlPrefix: 'projects',
  detailPath: (id) => `/settings/projects/${id}`,
  labels: { single: 'Project', plural: 'Projects' },
  writePermission: 'catalog:write',
  memberRoleOptions: ['project:owner', 'project:developer', 'project:viewer'],
  defaultMemberRole: 'project:viewer',
  showMemberRoleColumn: true,
  showEntriesPanel: true,
  cycleErrorMessage: "This member is already in the project's ancestry — adding them would create a cycle.",
}
EOF
```

- [ ] **Step 2: Rename component files**

```bash
rtk git mv web/src/routes/parties/GroupsTab.tsx web/src/routes/parties/PartyTab.tsx
rtk git mv web/src/routes/parties/GroupsTab.test.tsx web/src/routes/parties/PartyTab.test.tsx
rtk git mv web/src/routes/parties/CreateGroupDialog.tsx web/src/routes/parties/CreatePartyDialog.tsx
rtk git mv web/src/routes/parties/CreateGroupDialog.test.tsx web/src/routes/parties/CreatePartyDialog.test.tsx
rtk git mv web/src/routes/parties/GroupDetailPage.tsx web/src/routes/parties/PartyDetailPage.tsx
rtk git mv web/src/routes/parties/GroupDetailPage.test.tsx web/src/routes/parties/PartyDetailPage.test.tsx
```

Do NOT edit the component internals in this task — just update:
- Exported component names: `GroupsTab` → `PartyTab`, `CreateGroupDialog` → `CreatePartyDialog`, `GroupDetailPage` → `PartyDetailPage`.
- Import names in tests to match.
- In `web/src/App.tsx`: `import PartyDetailPage from './routes/parties/PartyDetailPage'` and update the route element.
- In `web/src/pages/SettingsPage.tsx`: `import PartyTab from '../routes/parties/PartyTab'` and update the JSX.

- [ ] **Step 3: Run tests — PASS (no behavior change)**

```bash
rtk cd web && rtk bun run test -- parties
```

Expected: all 19 existing tests pass with new component names.

- [ ] **Step 4: Commit**

```bash
rtk git add -A
rtk git commit -m "refactor(web): rename Group components to Party* and add PartyUIConfig"
```

---

## Task 5: Parameterize `PartyTab` with `config` prop

**Files:**
- Modify: `web/src/routes/parties/PartyTab.tsx`
- Modify: `web/src/routes/parties/PartyTab.test.tsx`
- Modify: `web/src/pages/SettingsPage.tsx`

- [ ] **Step 1: Extend tests to cover both configs**

Replace the contents of `web/src/routes/parties/PartyTab.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import PartyTab from './PartyTab'
import { groupUIConfig, projectUIConfig, type PartyUIConfig } from './partyUIConfig'

vi.mock('../../contexts/AuthContext', () => ({ useAuth: vi.fn() }))
vi.mock('@/api', () => ({
  listGroups: vi.fn(),
  createGroup: vi.fn(),
  deleteGroup: vi.fn(),
  listProjects: vi.fn(),
  createProject: vi.fn(),
  deleteProject: vi.fn(),
}))

import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'

const mockUseAuth = useAuth as ReturnType<typeof vi.fn>
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

function makeRender(config: PartyUIConfig) {
  return () => render(<MemoryRouter><PartyTab config={config} /></MemoryRouter>)
}

describe.each([
  ['groups', groupUIConfig, 'listGroups', 'deleteGroup', 'Group'],
  ['projects', projectUIConfig, 'listProjects', 'deleteProject', 'Project'],
])('PartyTab (%s)', (_name, config, listFn, deleteFn, label) => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseAuth.mockReturnValue({ hasPermission: (p: string) => p === config.writePermission })
    mockApi[listFn].mockResolvedValue([])
  })

  it('shows empty state when none exist', async () => {
    makeRender(config)()
    await waitFor(() => expect(screen.getByText(new RegExp(`no ${label.toLowerCase()}s yet`, 'i'))).toBeInTheDocument())
  })

  it('renders rows', async () => {
    mockApi[listFn].mockResolvedValue([
      { id: 'a', kind: config.kind, name: 'alpha', is_system: false, created_at: '2026-01-01T00:00:00Z', updated_at: '' },
    ])
    makeRender(config)()
    await waitFor(() => expect(screen.getByText('alpha')).toBeInTheDocument())
  })

  it('opens create dialog', async () => {
    makeRender(config)()
    await userEvent.click(screen.getByRole('button', { name: new RegExp(`create ${label.toLowerCase()}`, 'i') }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('hides create button without write permission', async () => {
    mockUseAuth.mockReturnValue({ hasPermission: () => false })
    makeRender(config)()
    await waitFor(() => expect(screen.queryByRole('button', { name: new RegExp(`create ${label.toLowerCase()}`, 'i') })).not.toBeInTheDocument())
  })

  it('deletes on confirmed delete', async () => {
    mockApi[listFn]
      .mockResolvedValueOnce([{ id: 'a', kind: config.kind, name: 'alpha', is_system: false, created_at: '', updated_at: '' }])
      .mockResolvedValueOnce([])
    mockApi[deleteFn].mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    makeRender(config)()
    await waitFor(() => expect(screen.getByText('alpha')).toBeInTheDocument())
    await userEvent.click(screen.getByTitle('Delete'))
    await waitFor(() => expect(mockApi[deleteFn]).toHaveBeenCalledWith('a'))
    confirmSpy.mockRestore()
  })
})
```

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk cd web && rtk bun run test -- PartyTab.test
```

Expected: FAIL — PartyTab does not yet accept `config`, and project API functions do not exist.

- [ ] **Step 3: Parameterize `PartyTab.tsx`**

Replace `web/src/routes/parties/PartyTab.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Plus, Trash2 } from 'lucide-react'
import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'
import type { Party } from '@/api'
import CreatePartyDialog from './CreatePartyDialog'
import type { PartyUIConfig } from './partyUIConfig'

interface Props {
  config: PartyUIConfig
}

export default function PartyTab({ config }: Props) {
  const { hasPermission } = useAuth()
  const navigate = useNavigate()
  const [parties, setParties] = useState<Party[]>([])
  const [createOpen, setCreateOpen] = useState(false)
  const canWrite = hasPermission(config.writePermission)

  const listFn = config.kind === 'group' ? api.listGroups : api.listProjects
  const deleteFn = config.kind === 'group' ? api.deleteGroup : api.deleteProject

  const load = () => { listFn().then(setParties).catch(() => {}) }
  useEffect(load, [listFn])

  const handleDelete = async (p: Party) => {
    if (!confirm(`Delete ${config.labels.single.toLowerCase()} "${p.name}"?`)) return
    try { await deleteFn(p.id); load() } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">{config.labels.plural}</h3>
        {canWrite && (
          <Button size="sm" className="gap-1" onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> Create {config.labels.single.toLowerCase()}
          </Button>
        )}
      </div>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Created</TableHead>
              {canWrite && <TableHead className="w-24">Actions</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {parties.map(p => (
              <TableRow
                key={p.id}
                className="cursor-pointer hover:bg-muted/50"
                onClick={() => navigate(config.detailPath(p.id))}
              >
                <TableCell className="font-medium">{p.name}</TableCell>
                <TableCell>{new Date(p.created_at).toLocaleDateString()}</TableCell>
                {canWrite && (
                  <TableCell onClick={e => e.stopPropagation()}>
                    <Button variant="ghost" size="icon" onClick={() => handleDelete(p)} title="Delete">
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {parties.length === 0 && (
              <TableRow><TableCell colSpan={3} className="text-center text-muted-foreground py-8">
                No {config.labels.plural.toLowerCase()} yet. Click <strong>Create {config.labels.single.toLowerCase()}</strong> to get started.
              </TableCell></TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <CreatePartyDialog
        config={config}
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={load}
      />
    </div>
  )
}
```

- [ ] **Step 4: Update `SettingsPage.tsx` groups mount to pass config**

```tsx
import PartyTab from '../routes/parties/PartyTab'
import { groupUIConfig } from '../routes/parties/partyUIConfig'
// ...
<TabsContent value="groups" className="mt-6"><PartyTab config={groupUIConfig} /></TabsContent>
```

- [ ] **Step 5: Project API functions do not exist yet — add stubs to silence typecheck**

Append to `web/src/api.ts` (real implementations come in Task 8; these stubs let this task compile and tests fail for the right reason):

```ts
export function listProjects(): Promise<Party[]> {
  throw new Error('listProjects not yet implemented')
}
export function deleteProject(_id: string): Promise<void> {
  throw new Error('deleteProject not yet implemented')
}
```

Remove these stubs in Task 8 when the real functions land.

- [ ] **Step 6: Run tests — PASS for groups config; project tests still fail (expected — no projectApi yet)**

```bash
rtk cd web && rtk bun run test -- PartyTab.test
```

If project tests fail because `listProjects` throws, that's fine — they'll pass after Task 8. Skip with `.skip` for now and add them back in Task 8.

Simpler alternative: make the project portion of the `describe.each` array conditionally skipped via an env flag — but the cleanest approach is to defer the project-config tests to Task 8. Replace the `describe.each` back to the group-only form for now, and re-add the project row in Task 8.

- [ ] **Step 7: Commit**

```bash
rtk git add web/src/routes/parties/ web/src/pages/SettingsPage.tsx web/src/api.ts
rtk git commit -m "refactor(web): parameterize PartyTab by PartyUIConfig"
```

---

## Task 6: Parameterize `CreatePartyDialog`

**Files:**
- Modify: `web/src/routes/parties/CreatePartyDialog.tsx`
- Modify: `web/src/routes/parties/CreatePartyDialog.test.tsx`

- [ ] **Step 1: Add config prop**

Replace `web/src/routes/parties/CreatePartyDialog.tsx`:

```tsx
import { useState, type FormEvent } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import * as api from '@/api'
import type { PartyUIConfig } from './partyUIConfig'

interface Props {
  config: PartyUIConfig
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}

export default function CreatePartyDialog({ config, open, onOpenChange, onCreated }: Props) {
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const createFn = config.kind === 'group' ? api.createGroup : api.createProject

  const reset = () => { setName(''); setError(''); setSaving(false) }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setSaving(true)
    try {
      await createFn({ name: name.trim() })
      onCreated()
      reset()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to create ${config.labels.single.toLowerCase()}`)
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create {config.labels.single.toLowerCase()}</DialogTitle>
          <DialogDescription>
            {config.kind === 'group'
              ? 'Groups organize people and nested groups. Add members after creation.'
              : 'Projects scope catalog entries and group access by role. Add members after creation.'}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <div className="space-y-2">
            <label htmlFor="party-name" className="text-sm font-medium">Name</label>
            <Input id="party-name" value={name} onChange={e => setName(e.target.value)} maxLength={128} required autoFocus />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={saving || name.trim().length === 0}>
              {saving ? 'Creating…' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

Also add a stub for `createProject` to `web/src/api.ts`:

```ts
export function createProject(_data: { name: string }): Promise<Party> {
  throw new Error('createProject not yet implemented')
}
```

- [ ] **Step 2: Update existing tests to pass the config prop**

Edit `web/src/routes/parties/CreatePartyDialog.test.tsx` — replace all `<CreateGroupDialog ...>` with:

```tsx
import { groupUIConfig } from './partyUIConfig'
// ...
render(<CreatePartyDialog config={groupUIConfig} open={true} onOpenChange={onOpenChange} onCreated={onCreated} />)
```

The rest of the tests continue to mock `createGroup` — that's fine, the dialog selects `createGroup` from the group config.

- [ ] **Step 3: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- CreatePartyDialog.test
```

- [ ] **Step 4: Commit**

```bash
rtk git add web/src/routes/parties/CreatePartyDialog.tsx web/src/routes/parties/CreatePartyDialog.test.tsx web/src/api.ts
rtk git commit -m "refactor(web): parameterize CreatePartyDialog by PartyUIConfig"
```

---

## Task 7: Parameterize `AddMemberDialog` with `roleOptions`

**Files:**
- Modify: `web/src/routes/parties/AddMemberDialog.tsx`
- Modify: `web/src/routes/parties/AddMemberDialog.test.tsx`

- [ ] **Step 1: Add role select when `roleOptions.length > 1`**

Replace `web/src/routes/parties/AddMemberDialog.tsx`:

```tsx
import { useState } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import * as api from '@/api'
import type { Party } from '@/api'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded: () => void
  groupId: string
  parties: Party[]
  excludedIds: Set<string>
  existingMemberIds: Set<string>
  kind: 'group' | 'project'
  roleOptions?: string[]
  defaultRole?: string
  cycleErrorMessage?: string
}

function translateError(msg: string, fallback: string): string {
  if (msg.toLowerCase().includes('cycle')) return fallback
  return msg
}

export default function AddMemberDialog({
  open, onOpenChange, onAdded, groupId, parties, excludedIds, existingMemberIds,
  kind, roleOptions = ['member'], defaultRole = 'member',
  cycleErrorMessage = "This member is already in the ancestry — adding them would create a cycle.",
}: Props) {
  const [selectedId, setSelectedId] = useState('')
  const [role, setRole] = useState(defaultRole)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const selectable = parties.filter(
    p => !excludedIds.has(p.id) && !existingMemberIds.has(p.id) && p.kind !== 'project'
  )

  const reset = () => { setSelectedId(''); setRole(defaultRole); setError(''); setSaving(false) }

  const handleSubmit = async () => {
    if (!selectedId) return
    setError('')
    setSaving(true)
    try {
      if (kind === 'group') {
        await api.addGroupMember(groupId, selectedId)
      } else {
        await api.addProjectMember(groupId, selectedId, role)
      }
      onAdded()
      reset()
      onOpenChange(false)
    } catch (err) {
      setError(translateError(err instanceof Error ? err.message : 'Failed to add member', cycleErrorMessage))
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Add member</DialogTitle>
          <DialogDescription>Pick a person or another group to add as a member.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <div className="space-y-2">
            <label htmlFor="member-select" className="text-sm font-medium">Member</label>
            <select
              id="member-select"
              role="combobox"
              aria-label="Member"
              value={selectedId}
              onChange={e => setSelectedId(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
            >
              <option value="" disabled>Select a party…</option>
              {selectable.map(p => (
                <option key={p.id} value={p.id}>
                  {p.name} ({p.kind === 'person' ? 'Person' : 'Group'})
                </option>
              ))}
            </select>
          </div>
          {roleOptions.length > 1 && (
            <div className="space-y-2">
              <label htmlFor="role-select" className="text-sm font-medium">Role</label>
              <select
                id="role-select"
                aria-label="Role"
                value={role}
                onChange={e => setRole(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
              >
                {roleOptions.map(r => <option key={r} value={r}>{r}</option>)}
              </select>
            </div>
          )}
          {selectedId && (
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              Selected: <Badge variant="secondary">
                {selectable.find(p => p.id === selectedId)?.kind === 'person' ? 'Person' : 'Group'}
              </Badge>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button onClick={handleSubmit} disabled={saving || !selectedId}>
            {saving ? 'Adding…' : 'Add'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

Add a stub to `web/src/api.ts` (real impl in Task 8):

```ts
export function addProjectMember(_projectId: string, _partyId: string, _role: string): Promise<void> {
  throw new Error('addProjectMember not yet implemented')
}
```

- [ ] **Step 2: Update existing `AddMemberDialog.test.tsx` tests to pass `kind="group"` and extend with a project test**

Modify three call sites of `renderDialog` to pass `kind="group"`. Add a new test at the bottom:

```tsx
it('renders role select and posts with selected role for project kind', async () => {
  mockApi.addProjectMember = vi.fn().mockResolvedValue(undefined)
  const onAdded = vi.fn()
  render(<AddMemberDialog
    open={true}
    onOpenChange={vi.fn()}
    onAdded={onAdded}
    groupId="proj1"
    parties={parties}
    excludedIds={new Set()}
    existingMemberIds={new Set()}
    kind="project"
    roleOptions={['project:owner', 'project:developer', 'project:viewer']}
    defaultRole="project:viewer"
  />)
  await userEvent.selectOptions(screen.getByLabelText('Member'), 'p2')
  await userEvent.selectOptions(screen.getByLabelText('Role'), 'project:developer')
  await userEvent.click(screen.getByRole('button', { name: /^add$/i }))
  await waitFor(() => expect(mockApi.addProjectMember).toHaveBeenCalledWith('proj1', 'p2', 'project:developer'))
})
```

Add `addProjectMember: vi.fn()` to the `vi.mock('@/api', …)` block.

- [ ] **Step 3: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- AddMemberDialog.test
```

- [ ] **Step 4: Commit**

```bash
rtk git add web/src/routes/parties/AddMemberDialog.tsx web/src/routes/parties/AddMemberDialog.test.tsx web/src/api.ts
rtk git commit -m "refactor(web): add kind + roleOptions to AddMemberDialog"
```

---

## Task 8: API client — projects + catalog assignment + `listCatalog({project})`

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/api.test.ts`
- Modify: `web/src/types.ts`

- [ ] **Step 1: Extend `ListFilter`**

In `web/src/types.ts`, add to `ListFilter`:

```ts
export interface ListFilter {
  // …existing fields…
  project?: string
  // …rest…
}
```

- [ ] **Step 2: Write failing tests for all new API functions**

Append to the `Parties API` describe block in `web/src/api.test.ts`:

```ts
it('listProjects GETs /api/v1/projects', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
    ok: true, status: 200, json: () => Promise.resolve([]),
  })
  await api.listProjects()
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects', expect.any(Object))
})

it('createProject POSTs to /api/v1/projects', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
    ok: true, status: 201, json: () => Promise.resolve({ id: 'p1' }),
  })
  await api.createProject({ name: 'proj' })
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects', expect.objectContaining({
    method: 'POST', body: JSON.stringify({ name: 'proj' }),
  }))
})

it('deleteProject sends DELETE', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: true, status: 204 })
  await api.deleteProject('p1')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects/p1', expect.objectContaining({ method: 'DELETE' }))
})

it('getProject GETs /api/v1/projects/:id', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
    ok: true, status: 200, json: () => Promise.resolve({ id: 'p1' }),
  })
  await api.getProject('p1')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects/p1', expect.any(Object))
})

it('listProjectMembers GETs /api/v1/projects/:id/members', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
    ok: true, status: 200, json: () => Promise.resolve([]),
  })
  await api.listProjectMembers('p1')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects/p1/members', expect.any(Object))
})

it('addProjectMember POSTs with party_id + role', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: true, status: 204 })
  await api.addProjectMember('p1', 'm1', 'project:developer')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects/p1/members', expect.objectContaining({
    method: 'POST',
    body: JSON.stringify({ party_id: 'm1', role: 'project:developer' }),
  }))
})

it('removeProjectMember DELETEs', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: true, status: 204 })
  await api.removeProjectMember('p1', 'm1')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects/p1/members/m1', expect.objectContaining({ method: 'DELETE' }))
})

it('updateProjectMemberRole PATCHes with role', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: true, status: 204 })
  await api.updateProjectMemberRole('p1', 'm1', 'project:owner')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/projects/p1/members/m1', expect.objectContaining({
    method: 'PATCH', body: JSON.stringify({ role: 'project:owner' }),
  }))
})

it('assignEntryToProject POSTs to /api/v1/catalog/:entryId/projects', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: true, status: 201 })
  await api.assignEntryToProject('e1', 'p1')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/catalog/e1/projects', expect.objectContaining({
    method: 'POST', body: JSON.stringify({ project_party_id: 'p1' }),
  }))
})

it('removeEntryFromProject DELETEs', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({ ok: true, status: 204 })
  await api.removeEntryFromProject('e1', 'p1')
  expect(globalThis.fetch).toHaveBeenCalledWith('/api/v1/catalog/e1/projects/p1', expect.objectContaining({ method: 'DELETE' }))
})

it('listCatalog sends project filter', async () => {
  ;(globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
    ok: true, status: 200, json: () => Promise.resolve([]),
  })
  await api.listCatalog({ project: 'p1' })
  expect(globalThis.fetch).toHaveBeenCalledWith(
    expect.stringContaining('project=p1'),
    expect.any(Object)
  )
})
```

- [ ] **Step 3: Run tests — FAIL**

```bash
rtk cd web && rtk bun run test -- api.test
```

Expected: multiple failures — functions undefined / stubs throwing.

- [ ] **Step 4: Replace stubs with real implementations + extend `listCatalog`**

In `web/src/api.ts`, replace the Task 5/6/7 stubs and add new functions. Section after the Groups API:

```ts
/* ─── Projects API ─── */

export function listProjects(): Promise<Party[]> {
  return request<Party[]>('/projects')
}

export function getProject(id: string): Promise<Party> {
  return request<Party>(`/projects/${id}`)
}

export function createProject(data: { name: string }): Promise<Party> {
  return request<Party>('/projects', { method: 'POST', body: JSON.stringify(data) })
}

export function deleteProject(id: string): Promise<void> {
  return request<void>(`/projects/${id}`, { method: 'DELETE' })
}

export function listProjectMembers(id: string): Promise<PartyRelationship[]> {
  return request<PartyRelationship[]>(`/projects/${id}/members`)
}

export function addProjectMember(projectId: string, partyId: string, role: string): Promise<void> {
  return request<void>(`/projects/${projectId}/members`, {
    method: 'POST',
    body: JSON.stringify({ party_id: partyId, role }),
  })
}

export function removeProjectMember(projectId: string, memberPartyId: string): Promise<void> {
  return request<void>(`/projects/${projectId}/members/${memberPartyId}`, { method: 'DELETE' })
}

export function updateProjectMemberRole(projectId: string, memberPartyId: string, role: string): Promise<void> {
  return request<void>(`/projects/${projectId}/members/${memberPartyId}`, {
    method: 'PATCH',
    body: JSON.stringify({ role }),
  })
}

/* ─── Catalog × Project assignment ─── */

export function assignEntryToProject(entryId: string, projectId: string): Promise<void> {
  return request<void>(`/catalog/${entryId}/projects`, {
    method: 'POST',
    body: JSON.stringify({ project_party_id: projectId }),
  })
}

export function removeEntryFromProject(entryId: string, projectId: string): Promise<void> {
  return request<void>(`/catalog/${entryId}/projects/${projectId}`, { method: 'DELETE' })
}
```

Extend `listCatalog` in the same file — locate the existing function and add the project param line:

```ts
if (filter.project) params.set('project', filter.project)
```

Place this line in the same block as the other `params.set` calls.

- [ ] **Step 5: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- api.test
```

- [ ] **Step 6: Re-enable project-config tests in `PartyTab.test.tsx` (from Task 5 Step 6)**

Restore the full `describe.each` array with both configs. All should now pass.

```bash
rtk cd web && rtk bun run test -- PartyTab.test
```

- [ ] **Step 7: Commit**

```bash
rtk git add web/src/api.ts web/src/api.test.ts web/src/types.ts web/src/routes/parties/PartyTab.test.tsx
rtk git commit -m "feat(web): add Projects API client and extend listCatalog project filter"
```

---

## Task 9: `EditMemberRoleDialog` component

**Files:**
- Create: `web/src/routes/parties/EditMemberRoleDialog.tsx`
- Create: `web/src/routes/parties/EditMemberRoleDialog.test.tsx`

- [ ] **Step 1: Write failing tests**

```tsx
// web/src/routes/parties/EditMemberRoleDialog.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import EditMemberRoleDialog from './EditMemberRoleDialog'

vi.mock('@/api', () => ({ updateProjectMemberRole: vi.fn() }))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => vi.clearAllMocks())

describe('EditMemberRoleDialog', () => {
  it('pre-selects the current role', () => {
    render(<EditMemberRoleDialog
      open={true} onOpenChange={vi.fn()} onSaved={vi.fn()}
      projectId="p1" memberPartyId="m1" memberName="alice"
      currentRole="project:viewer"
      roleOptions={['project:owner', 'project:developer', 'project:viewer']}
    />)
    expect((screen.getByLabelText(/role/i) as HTMLSelectElement).value).toBe('project:viewer')
  })

  it('calls updateProjectMemberRole on save', async () => {
    mockApi.updateProjectMemberRole.mockResolvedValue(undefined)
    const onSaved = vi.fn()
    render(<EditMemberRoleDialog
      open={true} onOpenChange={vi.fn()} onSaved={onSaved}
      projectId="p1" memberPartyId="m1" memberName="alice"
      currentRole="project:viewer"
      roleOptions={['project:owner', 'project:developer', 'project:viewer']}
    />)
    await userEvent.selectOptions(screen.getByLabelText(/role/i), 'project:developer')
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))
    await waitFor(() => expect(mockApi.updateProjectMemberRole).toHaveBeenCalledWith('p1', 'm1', 'project:developer'))
    expect(onSaved).toHaveBeenCalled()
  })

  it('displays backend error', async () => {
    mockApi.updateProjectMemberRole.mockRejectedValue(new Error('forbidden'))
    render(<EditMemberRoleDialog
      open={true} onOpenChange={vi.fn()} onSaved={vi.fn()}
      projectId="p1" memberPartyId="m1" memberName="alice"
      currentRole="project:viewer"
      roleOptions={['project:owner', 'project:developer', 'project:viewer']}
    />)
    await userEvent.click(screen.getByRole('button', { name: /^save$/i }))
    await waitFor(() => expect(screen.getByText(/forbidden/i)).toBeInTheDocument())
  })
})
```

- [ ] **Step 2: Run tests — FAIL (module missing)**

```bash
rtk cd web && rtk bun run test -- EditMemberRoleDialog.test
```

- [ ] **Step 3: Implement the dialog**

```tsx
// web/src/routes/parties/EditMemberRoleDialog.tsx
import { useState } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import * as api from '@/api'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => void
  projectId: string
  memberPartyId: string
  memberName: string
  currentRole: string
  roleOptions: string[]
}

export default function EditMemberRoleDialog({
  open, onOpenChange, onSaved, projectId, memberPartyId, memberName, currentRole, roleOptions,
}: Props) {
  const [role, setRole] = useState(currentRole)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const reset = () => { setRole(currentRole); setError(''); setSaving(false) }

  const handleSubmit = async () => {
    setError('')
    setSaving(true)
    try {
      await api.updateProjectMemberRole(projectId, memberPartyId, role)
      onSaved()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update role')
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit role</DialogTitle>
          <DialogDescription>Change {memberName}'s role on this project.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <div className="space-y-2">
            <label htmlFor="edit-role-select" className="text-sm font-medium">Role</label>
            <select
              id="edit-role-select"
              aria-label="Role"
              value={role}
              onChange={e => setRole(e.target.value)}
              className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm"
            >
              {roleOptions.map(r => <option key={r} value={r}>{r}</option>)}
            </select>
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleSubmit} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 4: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- EditMemberRoleDialog.test
```

- [ ] **Step 5: Commit**

```bash
rtk git add web/src/routes/parties/EditMemberRoleDialog.tsx web/src/routes/parties/EditMemberRoleDialog.test.tsx
rtk git commit -m "feat(web): add EditMemberRoleDialog for project member role edits"
```

---

## Task 10: Parameterize `PartyDetailPage` + role column + entries panel gates

**Files:**
- Modify: `web/src/routes/parties/PartyDetailPage.tsx`
- Modify: `web/src/routes/parties/PartyDetailPage.test.tsx`

- [ ] **Step 1: Extend tests (add tests for project config before implementation)**

Keep existing group tests unchanged. Add a new `describe('PartyDetailPage — project config', () => {...})` block after them with tests mirroring the group set using `projectUIConfig`, plus:

```tsx
it('renders a Role column when showMemberRoleColumn is true', async () => {
  mockApi.listProjectMembers.mockResolvedValue([
    { id: 'r1', from_party_id: 'p1', from_role: 'project:developer', to_party_id: 'proj1', to_role: 'project', relationship_name: 'project_member' },
  ])
  mockApi.listParties.mockResolvedValue([
    { id: 'p1', kind: 'person', name: 'alice', is_system: false, created_at: '', updated_at: '' },
  ])
  renderDetail({ config: projectUIConfig, id: 'proj1' })
  await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
  expect(screen.getByText('project:developer')).toBeInTheDocument()
})

it('opens edit-role dialog when role badge is clicked', async () => {
  mockApi.listProjectMembers.mockResolvedValue([
    { id: 'r1', from_party_id: 'p1', from_role: 'project:developer', to_party_id: 'proj1', to_role: 'project', relationship_name: 'project_member' },
  ])
  mockApi.listParties.mockResolvedValue([
    { id: 'p1', kind: 'person', name: 'alice', is_system: false, created_at: '', updated_at: '' },
  ])
  renderDetail({ config: projectUIConfig, id: 'proj1' })
  await waitFor(() => expect(screen.getByText('alice')).toBeInTheDocument())
  await userEvent.click(screen.getByText('project:developer'))
  expect(screen.getByRole('dialog', { name: /edit role/i })).toBeInTheDocument()
})

it('renders ProjectEntriesPanel when showEntriesPanel is true', async () => {
  renderDetail({ config: projectUIConfig, id: 'proj1' })
  await waitFor(() => expect(screen.getByRole('heading', { name: /assigned catalog entries/i })).toBeInTheDocument())
})
```

Extend the shared `renderDetail` helper signature to take `{config, id}`:

```tsx
function renderDetail({ config = groupUIConfig, id = 'g1' } = {}) {
  const route = config.detailPath(id)
  return render(
    <MemoryRouter initialEntries={[route]}>
      <Routes>
        <Route path={`/settings/${config.urlPrefix}/:id`} element={<PartyDetailPage config={config} />} />
      </Routes>
    </MemoryRouter>
  )
}
```

Add mocks for project API functions to the top-level `vi.mock('@/api', …)` block:

```ts
vi.mock('@/api', () => ({
  getGroup: vi.fn(),
  listGroupMembers: vi.fn(),
  getProject: vi.fn(),
  listProjectMembers: vi.fn(),
  addProjectMember: vi.fn(),
  removeProjectMember: vi.fn(),
  updateProjectMemberRole: vi.fn(),
  listParties: vi.fn(),
  addGroupMember: vi.fn(),
  removeGroupMember: vi.fn(),
  listCatalog: vi.fn().mockResolvedValue([]),
  assignEntryToProject: vi.fn(),
  removeEntryFromProject: vi.fn(),
}))
```

Set `mockApi.getProject.mockResolvedValue({ id: 'proj1', kind: 'project', name: 'demo-project', ... })` and `mockApi.listProjectMembers.mockResolvedValue([])` in `beforeEach`.

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk cd web && rtk bun run test -- PartyDetailPage.test
```

- [ ] **Step 3: Parameterize `PartyDetailPage.tsx`**

Replace the component to accept `config: PartyUIConfig` as a prop. Select kind-specific functions by `config.kind`:

```tsx
import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ChevronLeft, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { useAuth } from '../../contexts/AuthContext'
import * as api from '@/api'
import type { Party, PartyRelationship } from '@/api'
import AddMemberDialog from './AddMemberDialog'
import EditMemberRoleDialog from './EditMemberRoleDialog'
import ProjectEntriesPanel from './ProjectEntriesPanel'
import type { PartyUIConfig } from './partyUIConfig'

interface Props {
  config: PartyUIConfig
}

export default function PartyDetailPage({ config }: Props) {
  const { id } = useParams<{ id: string }>()
  const { hasPermission } = useAuth()
  const canWrite = hasPermission(config.writePermission)
  const [party, setParty] = useState<Party | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [members, setMembers] = useState<PartyRelationship[]>([])
  const [parties, setParties] = useState<Party[]>([])
  const [addOpen, setAddOpen] = useState(false)
  const [editRoleState, setEditRoleState] = useState<{ memberId: string; memberName: string; currentRole: string } | null>(null)

  const getFn = config.kind === 'group' ? api.getGroup : api.getProject
  const listMembersFn = config.kind === 'group' ? api.listGroupMembers : api.listProjectMembers
  const removeFn = config.kind === 'group' ? api.removeGroupMember : api.removeProjectMember

  const partyById = useMemo(() => {
    const m = new Map<string, Party>()
    for (const p of parties) m.set(p.id, p)
    return m
  }, [parties])

  const reloadMembers = () => {
    if (!id) return
    listMembersFn(id).then(setMembers).catch(() => {})
  }

  useEffect(() => {
    if (!id) return
    setNotFound(false)
    getFn(id).then(setParty).catch(() => setNotFound(true))
    listMembersFn(id).then(setMembers).catch(() => {})
    api.listParties().then(setParties).catch(() => {})
  }, [id, getFn, listMembersFn])

  const handleRemove = async (partyId: string, name: string) => {
    if (!id) return
    if (!confirm(`Remove "${name}" from "${party?.name ?? ''}"?`)) return
    try { await removeFn(id, partyId); reloadMembers() } catch { /* ignore */ }
  }

  const excludedIds = useMemo(() => new Set<string>([id ?? '']), [id])
  const existingMemberIds = useMemo(() => {
    const set = new Set<string>()
    for (const r of members) set.add(r.from_party_id)
    return set
  }, [members])

  if (notFound) {
    return (
      <div className="space-y-4">
        <Link to={`/settings?tab=${config.urlPrefix}`} className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ChevronLeft className="h-4 w-4" /> Back to Settings
        </Link>
        <p className="text-muted-foreground">{config.labels.single} not found.</p>
      </div>
    )
  }

  if (!party) return <div className="text-muted-foreground">Loading…</div>

  const memberRows = members
    .map(r => {
      const p = partyById.get(r.from_party_id)
      return p ? { party: p, rel: r } : null
    })
    .filter(Boolean) as { party: Party; rel: PartyRelationship }[]

  return (
    <div className="space-y-6">
      <Link to={`/settings?tab=${config.urlPrefix}`} className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ChevronLeft className="h-4 w-4" /> Back to Settings
      </Link>
      <div>
        <h2 className="text-2xl font-bold tracking-tight">{party.name}</h2>
        <p className="text-muted-foreground">Created {new Date(party.created_at).toLocaleDateString()}</p>
      </div>
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-medium">Members</h3>
          {canWrite && (
            <Button size="sm" className="gap-1" onClick={() => setAddOpen(true)}>
              <Plus className="h-4 w-4" /> Add member
            </Button>
          )}
        </div>
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Kind</TableHead>
                {config.showMemberRoleColumn && <TableHead>Role</TableHead>}
                {canWrite && <TableHead className="w-24">Actions</TableHead>}
              </TableRow>
            </TableHeader>
            <TableBody>
              {memberRows.map(({ party: p, rel }) => (
                <TableRow key={p.id}>
                  <TableCell className="font-medium">{p.name}</TableCell>
                  <TableCell>
                    <Badge variant="secondary">{p.kind === 'person' ? 'Person' : p.kind === 'group' ? 'Group' : 'Project'}</Badge>
                  </TableCell>
                  {config.showMemberRoleColumn && (
                    <TableCell>
                      {canWrite ? (
                        <button
                          type="button"
                          className="inline-flex"
                          onClick={() => setEditRoleState({ memberId: p.id, memberName: p.name, currentRole: rel.from_role })}
                        >
                          <Badge variant="outline">{rel.from_role}</Badge>
                        </button>
                      ) : (
                        <Badge variant="outline">{rel.from_role}</Badge>
                      )}
                    </TableCell>
                  )}
                  {canWrite && (
                    <TableCell>
                      <Button variant="ghost" size="icon" onClick={() => handleRemove(p.id, p.name)} title="Remove">
                        <Trash2 className="h-3.5 w-3.5 text-destructive" />
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
              {memberRows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={config.showMemberRoleColumn ? 4 : 3} className="text-center text-muted-foreground py-8">
                    No members yet. Click <strong>Add member</strong> to add the first person or group.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </div>
      {config.showEntriesPanel && id && <ProjectEntriesPanel projectId={id} canWrite={canWrite} />}
      <AddMemberDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        onAdded={reloadMembers}
        groupId={id ?? ''}
        parties={parties}
        excludedIds={excludedIds}
        existingMemberIds={existingMemberIds}
        kind={config.kind}
        roleOptions={config.memberRoleOptions}
        defaultRole={config.defaultMemberRole}
        cycleErrorMessage={config.cycleErrorMessage}
      />
      {editRoleState && id && (
        <EditMemberRoleDialog
          open={true}
          onOpenChange={(o) => { if (!o) setEditRoleState(null) }}
          onSaved={reloadMembers}
          projectId={id}
          memberPartyId={editRoleState.memberId}
          memberName={editRoleState.memberName}
          currentRole={editRoleState.currentRole}
          roleOptions={config.memberRoleOptions}
        />
      )}
    </div>
  )
}
```

Update `web/src/App.tsx` to pass the config prop:

```tsx
<Route path="/settings/groups/:id" element={<ProtectedRoute><Layout><PartyDetailPage config={groupUIConfig} /></Layout></ProtectedRoute>} />
<Route path="/settings/projects/:id" element={<ProtectedRoute><Layout><PartyDetailPage config={projectUIConfig} /></Layout></ProtectedRoute>} />
```

Add imports for both configs.

- [ ] **Step 4: Run tests — PASS (except ProjectEntriesPanel test, which needs the component — Task 11)**

```bash
rtk cd web && rtk bun run test -- PartyDetailPage.test
```

The `renders ProjectEntriesPanel` test will fail until Task 11 — skip it temporarily with `.skip` and re-enable after Task 11.

- [ ] **Step 5: Commit**

```bash
rtk git add web/src/routes/parties/PartyDetailPage.tsx web/src/routes/parties/PartyDetailPage.test.tsx web/src/App.tsx
rtk git commit -m "feat(web): parameterize PartyDetailPage with role column and entries gate"
```

---

## Task 11: `ProjectEntriesPanel` + `AssignEntryDialog`

**Files:**
- Create: `web/src/routes/parties/ProjectEntriesPanel.tsx`
- Create: `web/src/routes/parties/ProjectEntriesPanel.test.tsx`
- Create: `web/src/routes/parties/AssignEntryDialog.tsx`
- Create: `web/src/routes/parties/AssignEntryDialog.test.tsx`

- [ ] **Step 1: Write `ProjectEntriesPanel` failing tests**

```tsx
// web/src/routes/parties/ProjectEntriesPanel.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import ProjectEntriesPanel from './ProjectEntriesPanel'

vi.mock('@/api', () => ({
  listCatalog: vi.fn(),
  removeEntryFromProject: vi.fn(),
}))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => {
  vi.clearAllMocks()
  mockApi.listCatalog.mockResolvedValue([])
})

describe('ProjectEntriesPanel', () => {
  it('renders heading and empty state', async () => {
    render(<ProjectEntriesPanel projectId="p1" canWrite={true} />)
    expect(screen.getByRole('heading', { name: /assigned catalog entries/i })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText(/no entries assigned/i)).toBeInTheDocument())
  })

  it('renders rows for each assigned entry', async () => {
    mockApi.listCatalog.mockResolvedValue([
      { id: 'e1', display_name: 'Alpha', protocol: 'a2a', endpoint: 'https://a.example', version: '1.0', status: 'active', source: 'push', agent_type_id: 't', validity: { last_seen: '' }, health: {}, created_at: '', updated_at: '' },
    ])
    render(<ProjectEntriesPanel projectId="p1" canWrite={true} />)
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
  })

  it('hides Assign button without canWrite', async () => {
    render(<ProjectEntriesPanel projectId="p1" canWrite={false} />)
    expect(screen.queryByRole('button', { name: /assign entry/i })).not.toBeInTheDocument()
  })

  it('removes assigned entry on confirm', async () => {
    mockApi.listCatalog
      .mockResolvedValueOnce([
        { id: 'e1', display_name: 'Alpha', protocol: 'a2a', endpoint: 'https://a.example', version: '1.0', status: 'active', source: 'push', agent_type_id: 't', validity: { last_seen: '' }, health: {}, created_at: '', updated_at: '' },
      ])
      .mockResolvedValueOnce([])
    mockApi.removeEntryFromProject.mockResolvedValue(undefined)
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    render(<ProjectEntriesPanel projectId="p1" canWrite={true} />)
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument())
    await userEvent.click(screen.getByTitle('Remove'))
    await waitFor(() => expect(mockApi.removeEntryFromProject).toHaveBeenCalledWith('e1', 'p1'))
    confirmSpy.mockRestore()
  })
})
```

- [ ] **Step 2: Write `AssignEntryDialog` failing tests**

```tsx
// web/src/routes/parties/AssignEntryDialog.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import AssignEntryDialog from './AssignEntryDialog'

vi.mock('@/api', () => ({
  listCatalog: vi.fn(),
  assignEntryToProject: vi.fn(),
}))
import * as api from '@/api'
const mockApi = api as unknown as Record<string, ReturnType<typeof vi.fn>>

beforeEach(() => {
  vi.clearAllMocks()
  mockApi.listCatalog.mockResolvedValue([])
})

describe('AssignEntryDialog', () => {
  it('searches and shows results', async () => {
    mockApi.listCatalog.mockImplementation(async ({ q }: { q?: string }) => {
      if (!q) return []
      return [{ id: 'e1', display_name: `Found ${q}`, protocol: 'a2a', endpoint: 'https://ex', version: '1.0', status: 'active', source: 'push', agent_type_id: 't', validity: { last_seen: '' }, health: {}, created_at: '', updated_at: '' }]
    })
    render(<AssignEntryDialog open={true} onOpenChange={vi.fn()} onAssigned={vi.fn()} projectId="p1" alreadyAssignedIds={new Set()} />)
    await userEvent.type(screen.getByPlaceholderText(/search catalog/i), 'trans')
    await waitFor(() => expect(screen.getByText('Found trans')).toBeInTheDocument(), { timeout: 1000 })
  })

  it('filters out already-assigned entries', async () => {
    mockApi.listCatalog.mockResolvedValue([
      { id: 'e1', display_name: 'Alpha', protocol: 'a2a', endpoint: 'https://a', version: '1.0', status: 'active', source: 'push', agent_type_id: 't', validity: { last_seen: '' }, health: {}, created_at: '', updated_at: '' },
      { id: 'e2', display_name: 'Bravo', protocol: 'a2a', endpoint: 'https://b', version: '1.0', status: 'active', source: 'push', agent_type_id: 't', validity: { last_seen: '' }, health: {}, created_at: '', updated_at: '' },
    ])
    render(<AssignEntryDialog open={true} onOpenChange={vi.fn()} onAssigned={vi.fn()} projectId="p1" alreadyAssignedIds={new Set(['e1'])} />)
    await userEvent.type(screen.getByPlaceholderText(/search catalog/i), 'a')
    await waitFor(() => expect(screen.getByText('Bravo')).toBeInTheDocument(), { timeout: 1000 })
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument()
  })

  it('assigns on confirm', async () => {
    mockApi.listCatalog.mockResolvedValue([
      { id: 'e1', display_name: 'Alpha', protocol: 'a2a', endpoint: 'https://a', version: '1.0', status: 'active', source: 'push', agent_type_id: 't', validity: { last_seen: '' }, health: {}, created_at: '', updated_at: '' },
    ])
    mockApi.assignEntryToProject.mockResolvedValue(undefined)
    const onAssigned = vi.fn()
    render(<AssignEntryDialog open={true} onOpenChange={vi.fn()} onAssigned={onAssigned} projectId="p1" alreadyAssignedIds={new Set()} />)
    await userEvent.type(screen.getByPlaceholderText(/search catalog/i), 'a')
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument(), { timeout: 1000 })
    await userEvent.click(screen.getByText('Alpha'))
    await userEvent.click(screen.getByRole('button', { name: /^assign$/i }))
    await waitFor(() => expect(mockApi.assignEntryToProject).toHaveBeenCalledWith('e1', 'p1'))
    expect(onAssigned).toHaveBeenCalled()
  })
})
```

- [ ] **Step 3: Run tests — FAIL (modules missing)**

```bash
rtk cd web && rtk bun run test -- ProjectEntriesPanel.test AssignEntryDialog.test
```

- [ ] **Step 4: Implement `AssignEntryDialog`**

```tsx
// web/src/routes/parties/AssignEntryDialog.tsx
import { useEffect, useMemo, useState } from 'react'
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import * as api from '@/api'
import type { CatalogEntry } from '@/types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  onAssigned: () => void
  projectId: string
  alreadyAssignedIds: Set<string>
}

export default function AssignEntryDialog({
  open, onOpenChange, onAssigned, projectId, alreadyAssignedIds,
}: Props) {
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const [results, setResults] = useState<CatalogEntry[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQuery(query), 250)
    return () => clearTimeout(t)
  }, [query])

  useEffect(() => {
    if (!debouncedQuery) { setResults([]); return }
    api.listCatalog({ q: debouncedQuery, limit: 20 })
      .then(setResults)
      .catch(() => setResults([]))
  }, [debouncedQuery])

  const visible = useMemo(
    () => results.filter(e => !alreadyAssignedIds.has(e.id)),
    [results, alreadyAssignedIds]
  )

  const reset = () => { setQuery(''); setDebouncedQuery(''); setResults([]); setSelectedId(''); setError(''); setSaving(false) }

  const handleSubmit = async () => {
    if (!selectedId) return
    setSaving(true)
    try {
      await api.assignEntryToProject(selectedId, projectId)
      onAssigned()
      reset()
      onOpenChange(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to assign entry')
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) reset(); onOpenChange(o) }}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Assign catalog entry</DialogTitle>
          <DialogDescription>Search the catalog, pick an entry, and assign it to this project.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          {error && <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>}
          <Input
            placeholder="Search catalog by name…"
            value={query}
            onChange={e => setQuery(e.target.value)}
            autoFocus
          />
          <div className="max-h-80 overflow-y-auto rounded-md border">
            {visible.length === 0 && debouncedQuery && (
              <div className="p-4 text-sm text-muted-foreground">No matching entries.</div>
            )}
            {visible.map(e => (
              <button
                key={e.id}
                type="button"
                onClick={() => setSelectedId(e.id)}
                className={`w-full text-left p-3 border-b last:border-b-0 hover:bg-muted/50 ${selectedId === e.id ? 'bg-muted' : ''}`}
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium">{e.display_name}</span>
                  <Badge variant="outline">{e.protocol}</Badge>
                </div>
                <div className="text-xs text-muted-foreground">{e.endpoint} · v{e.version}</div>
              </button>
            ))}
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleSubmit} disabled={saving || !selectedId}>
            {saving ? 'Assigning…' : 'Assign'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 5: Implement `ProjectEntriesPanel`**

```tsx
// web/src/routes/parties/ProjectEntriesPanel.tsx
import { useEffect, useMemo, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import * as api from '@/api'
import type { CatalogEntry } from '@/types'
import AssignEntryDialog from './AssignEntryDialog'

interface Props {
  projectId: string
  canWrite: boolean
}

export default function ProjectEntriesPanel({ projectId, canWrite }: Props) {
  const [entries, setEntries] = useState<CatalogEntry[]>([])
  const [assignOpen, setAssignOpen] = useState(false)

  const load = () => {
    api.listCatalog({ project: projectId }).then(setEntries).catch(() => {})
  }
  useEffect(load, [projectId])

  const assignedIds = useMemo(() => new Set(entries.map(e => e.id)), [entries])

  const handleRemove = async (e: CatalogEntry) => {
    if (!confirm(`Remove "${e.display_name}" from this project?`)) return
    try { await api.removeEntryFromProject(e.id, projectId); load() } catch { /* ignore */ }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-medium">Assigned catalog entries</h3>
        {canWrite && (
          <Button size="sm" className="gap-1" onClick={() => setAssignOpen(true)}>
            <Plus className="h-4 w-4" /> Assign entry
          </Button>
        )}
      </div>
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Protocol</TableHead>
              <TableHead>Endpoint</TableHead>
              {canWrite && <TableHead className="w-24">Actions</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map(e => (
              <TableRow key={e.id}>
                <TableCell className="font-medium">{e.display_name}</TableCell>
                <TableCell><Badge variant="outline">{e.protocol}</Badge></TableCell>
                <TableCell className="text-xs text-muted-foreground">{e.endpoint}</TableCell>
                {canWrite && (
                  <TableCell>
                    <Button variant="ghost" size="icon" onClick={() => handleRemove(e)} title="Remove">
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </TableCell>
                )}
              </TableRow>
            ))}
            {entries.length === 0 && (
              <TableRow>
                <TableCell colSpan={canWrite ? 4 : 3} className="text-center text-muted-foreground py-8">
                  No entries assigned yet.{canWrite && <> Click <strong>Assign entry</strong>.</>}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <AssignEntryDialog
        open={assignOpen}
        onOpenChange={setAssignOpen}
        onAssigned={load}
        projectId={projectId}
        alreadyAssignedIds={assignedIds}
      />
    </div>
  )
}
```

- [ ] **Step 6: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- ProjectEntriesPanel.test AssignEntryDialog.test PartyDetailPage.test
```

Re-enable the previously skipped `renders ProjectEntriesPanel` test from Task 10 — it should now pass.

- [ ] **Step 7: Commit**

```bash
rtk git add web/src/routes/parties/ProjectEntriesPanel.tsx web/src/routes/parties/ProjectEntriesPanel.test.tsx web/src/routes/parties/AssignEntryDialog.tsx web/src/routes/parties/AssignEntryDialog.test.tsx web/src/routes/parties/PartyDetailPage.test.tsx
rtk git commit -m "feat(web): add ProjectEntriesPanel and AssignEntryDialog"
```

---

## Task 12: Mount Projects tab in `SettingsPage`

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`
- Modify: `web/src/pages/SettingsPage.test.tsx`

- [ ] **Step 1: Extend SettingsPage test**

Add to the `vi.mock('@/api', () => ({ … }))` block in `web/src/pages/SettingsPage.test.tsx`:

```ts
listProjects: vi.fn(),
listProjectMembers: vi.fn(),
createProject: vi.fn(),
deleteProject: vi.fn(),
```

In `beforeEach`:

```ts
mockApi.listProjects.mockResolvedValue([])
```

Add a new test:

```tsx
it('renders Projects tab for any authenticated user', async () => {
  renderSettingsPage()
  await waitFor(() => expect(screen.getByRole('tab', { name: /projects/i })).toBeInTheDocument())
})
```

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk cd web && rtk bun run test -- SettingsPage.test
```

- [ ] **Step 3: Mount the tab**

Edit `web/src/pages/SettingsPage.tsx`:

```tsx
import { groupUIConfig, projectUIConfig } from '../routes/parties/partyUIConfig'
// ...
<Tabs defaultValue="general">
  <TabsList>
    <TabsTrigger value="general">General</TabsTrigger>
    {showUsers && <TabsTrigger value="users">Users</TabsTrigger>}
    {showRoles && <TabsTrigger value="roles">Roles</TabsTrigger>}
    <TabsTrigger value="groups">Groups</TabsTrigger>
    <TabsTrigger value="projects">Projects</TabsTrigger>
    <TabsTrigger value="account">My Account</TabsTrigger>
  </TabsList>
  <TabsContent value="general" className="mt-6"><GeneralTab /></TabsContent>
  {showUsers && <TabsContent value="users" className="mt-6"><UsersTab /></TabsContent>}
  {showRoles && <TabsContent value="roles" className="mt-6"><RolesTab /></TabsContent>}
  <TabsContent value="groups" className="mt-6"><PartyTab config={groupUIConfig} /></TabsContent>
  <TabsContent value="projects" className="mt-6"><PartyTab config={projectUIConfig} /></TabsContent>
  <TabsContent value="account" className="mt-6"><AccountTab /></TabsContent>
</Tabs>
```

- [ ] **Step 4: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- SettingsPage.test
```

- [ ] **Step 5: Commit**

```bash
rtk git add web/src/pages/SettingsPage.tsx web/src/pages/SettingsPage.test.tsx
rtk git commit -m "feat(web): mount Projects tab in SettingsPage"
```

---

## Task 13: Catalog project filter

**Files:**
- Modify: `web/src/hooks/useCatalogQuery.ts`
- Modify: `web/src/hooks/useCatalogQuery.test.tsx`
- Modify: `web/src/routes/catalog/CatalogListPage.tsx`
- Modify: `web/src/routes/catalog/CatalogListPage.test.tsx` (create if absent)

- [ ] **Step 1: Extend `useCatalogQuery` tests**

Append to `web/src/hooks/useCatalogQuery.test.tsx` patterns already in the file:

```tsx
it('setProject updates URL param', async () => {
  // test harness mirrors existing setProtocol test
  // ...
  await user.click(screen.getByText('set-project'))
  expect(window.location.search).toContain('project=p1')
})
```

Mirror the existing `setProtocol` test exactly — add `setProject` button wired to `() => setProject('p1')`.

- [ ] **Step 2: Run tests — FAIL**

```bash
rtk cd web && rtk bun run test -- useCatalogQuery
```

- [ ] **Step 3: Extend `useCatalogQuery`**

Modify `web/src/hooks/useCatalogQuery.ts`:

```ts
const project = searchParams.get('project') || undefined

const filter: ListFilter = {
  protocol,
  q,
  sort,
  project,
}

// ... inside useQuery queryKey: ['catalog', { protocol, q, sort, project }]

const setProject = useCallback(
  (p: string | undefined) => {
    setSearchParams(prev => {
      const next = new URLSearchParams(prev)
      if (p) next.set('project', p)
      else next.delete('project')
      return next
    })
  },
  [setSearchParams]
)

return {
  // ... existing returns
  setProject,
}
```

- [ ] **Step 4: Add Project Select to `CatalogListPage`**

Import `Select` components and `useQuery` for projects:

```tsx
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useQuery } from '@tanstack/react-query'
import * as api from '@/api'

// Inside component:
const { entries, isLoading, isError, error, filter, setProtocol, setQuery, setProject, clearFilters, refetch } = useCatalogQuery()
const { data: projects = [] } = useQuery({
  queryKey: ['projects-for-filter'],
  queryFn: () => api.listParties('project'),
  staleTime: 60_000,
})

// Inside the filter bar JSX:
<Select value={filter.project ?? '__all__'} onValueChange={v => setProject(v === '__all__' ? undefined : v)}>
  <SelectTrigger className="w-[200px]"><SelectValue placeholder="All projects" /></SelectTrigger>
  <SelectContent>
    <SelectItem value="__all__">All projects</SelectItem>
    {projects.map(p => <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}
  </SelectContent>
</Select>
```

Update `hasActiveFilters`:

```tsx
const hasActiveFilters = Boolean(filter.protocol || filter.q || filter.project)
```

- [ ] **Step 5: Add a CatalogListPage test**

If `web/src/routes/catalog/CatalogListPage.test.tsx` exists, extend it; otherwise create it with:

```tsx
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import CatalogListPage from './CatalogListPage'

vi.mock('@/api', () => ({
  listCatalog: vi.fn().mockResolvedValue([]),
  listParties: vi.fn().mockResolvedValue([
    { id: 'p1', kind: 'project', name: 'alpha', is_system: false, created_at: '', updated_at: '' },
    { id: 'p2', kind: 'project', name: 'bravo', is_system: false, created_at: '', updated_at: '' },
  ]),
}))

function renderPage(initial = '/') {
  const qc = new QueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initial]}>
        <CatalogListPage />
      </MemoryRouter>
    </QueryClientProvider>
  )
}

beforeEach(() => vi.clearAllMocks())

describe('CatalogListPage project filter', () => {
  it('populates Project select with projects', async () => {
    renderPage()
    await waitFor(() => expect(screen.getByText(/all projects/i)).toBeInTheDocument())
  })

  it('selecting a project updates URL and refetches', async () => {
    renderPage()
    await userEvent.click(screen.getByRole('combobox', { name: /project/i }))
    // Actual interaction depends on shadcn Select internals — use `selectOptions` or click the option.
  })
})
```

Note: the shadcn `Select` is a Radix component, not a native element. The test may need `getAllByRole('option')` and click. If this proves brittle, defer this test to E2E only and keep the hook-level test.

- [ ] **Step 6: Run tests — PASS**

```bash
rtk cd web && rtk bun run test -- CatalogListPage.test useCatalogQuery
```

- [ ] **Step 7: Commit**

```bash
rtk git add web/src/hooks/useCatalogQuery.ts web/src/hooks/useCatalogQuery.test.tsx web/src/routes/catalog/CatalogListPage.tsx web/src/routes/catalog/CatalogListPage.test.tsx
rtk git commit -m "feat(web): add project filter dropdown to catalog list"
```

---

## Task 14: E2E — `projects.spec.ts`

**Files:**
- Create: `e2e/tests/projects.spec.ts`

- [ ] **Step 1: Write the spec**

```ts
// e2e/tests/projects.spec.ts
import { test, expect } from '@playwright/test'
import { loginViaUI, loginViaAPI, authHeader, BASE, createUser } from './helpers'

test.describe('Projects management', () => {
  test.afterEach(async ({ request }) => {
    const token = await loginViaAPI(request)
    const listRes = await request.get(`${BASE}/api/v1/projects`, { headers: authHeader(token) })
    if (listRes.ok()) {
      const projects: Array<{ id: string; name: string }> = await listRes.json()
      for (const p of projects.filter(p => p.name.startsWith('e2e-'))) {
        await request.delete(`${BASE}/api/v1/projects/${p.id}`, { headers: authHeader(token) }).catch(() => {})
      }
    }
  })

  test('admin creates, views, deletes a project', async ({ page }) => {
    await loginViaUI(page)
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^projects$/i }).click()
    await page.getByRole('button', { name: /create project/i }).click()
    await page.getByLabel(/name/i).fill('e2e-demo-project')
    await page.getByRole('button', { name: /^create$/i }).click()
    await expect(page.getByText('e2e-demo-project')).toBeVisible()

    await page.getByText('e2e-demo-project').click()
    await expect(page.getByRole('heading', { name: 'e2e-demo-project' })).toBeVisible()
    await expect(page.getByRole('heading', { name: /assigned catalog entries/i })).toBeVisible()

    await page.getByRole('link', { name: /back to settings/i }).click()
    await page.getByRole('tab', { name: /^projects$/i }).click()
    page.once('dialog', d => d.accept())
    await page.getByTitle('Delete').first().click()
    await expect(page.getByText('e2e-demo-project')).not.toBeVisible()
  })

  test('admin adds a person member with role project:developer', async ({ page }) => {
    await loginViaUI(page)
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^projects$/i }).click()
    await page.getByRole('button', { name: /create project/i }).click()
    await page.getByLabel(/name/i).fill('e2e-member-project')
    await page.getByRole('button', { name: /^create$/i }).click()

    await page.getByText('e2e-member-project').click()
    await page.getByRole('button', { name: /add member/i }).click()
    await page.getByLabel('Member').selectOption({ label: 'admin (Person)' })
    await page.getByLabel('Role').selectOption('project:developer')
    await page.getByRole('button', { name: /^add$/i }).click()
    await expect(page.getByRole('cell', { name: 'admin' })).toBeVisible()
    await expect(page.getByText('project:developer')).toBeVisible()
  })

  test('admin changes a member role via edit dialog', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const projectRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-role-project' },
    })
    const project = await projectRes.json()
    const partiesRes = await request.get(`${BASE}/api/v1/parties?kind=person`, { headers: authHeader(token) })
    const persons = await partiesRes.json() as Array<{ id: string; name: string }>
    const adminPerson = persons[0]
    await request.post(`${BASE}/api/v1/projects/${project.id}/members`, {
      headers: authHeader(token),
      data: { party_id: adminPerson.id, role: 'project:viewer' },
    })

    await loginViaUI(page)
    await page.goto(`/settings/projects/${project.id}`)
    await page.getByText('project:viewer').click()
    await page.getByLabel('Role').selectOption('project:owner')
    await page.getByRole('button', { name: /^save$/i }).click()
    await expect(page.getByText('project:owner')).toBeVisible()
  })

  test('admin assigns a catalog entry to a project', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    const projectRes = await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-assign-project' },
    })
    const project = await projectRes.json()
    const entryRes = await request.post(`${BASE}/api/v1/catalog`, {
      headers: authHeader(token),
      data: {
        display_name: 'E2E Assigned Agent',
        description: 'assign test',
        protocol: 'a2a',
        endpoint: `http://e2e-assign-${Date.now()}.example.com`,
        version: '1.0.0',
      },
    })
    const entry = await entryRes.json()

    await loginViaUI(page)
    await page.goto(`/settings/projects/${project.id}`)
    await page.getByRole('button', { name: /assign entry/i }).click()
    await page.getByPlaceholder(/search catalog/i).fill('E2E Assigned')
    await page.getByText('E2E Assigned Agent').click()
    await page.getByRole('button', { name: /^assign$/i }).click()
    await expect(page.getByText('E2E Assigned Agent')).toBeVisible()

    await request.delete(`${BASE}/api/v1/catalog/${entry.id}`, { headers: authHeader(token) }).catch(() => {})
  })

  test('viewer sees list but no mutate buttons', async ({ page, request }) => {
    const token = await loginViaAPI(request)
    await request.post(`${BASE}/api/v1/projects`, {
      headers: authHeader(token),
      data: { name: 'e2e-read-only-project' },
    })
    const rolesRes = await request.get(`${BASE}/api/v1/roles`, { headers: authHeader(token) })
    const roles = await rolesRes.json() as Array<{ id: string; name: string }>
    const viewer = roles.find(r => r.name === 'viewer')!
    await createUser(request, token, {
      username: 'e2e_proj_viewer', password: 'Viewer@E2e99!', display_name: 'Project Viewer', role_id: viewer.id,
    })

    await loginViaUI(page, 'e2e_proj_viewer', 'Viewer@E2e99!')
    await page.goto('/settings')
    await page.getByRole('tab', { name: /^projects$/i }).click()
    await expect(page.getByText('e2e-read-only-project')).toBeVisible()
    await expect(page.getByRole('button', { name: /create project/i })).not.toBeVisible()

    const usersRes = await request.get(`${BASE}/api/v1/users`, { headers: authHeader(token) })
    const users = await usersRes.json() as Array<{ id: string; username: string }>
    const vu = users.find(u => u.username === 'e2e_proj_viewer')
    if (vu) await request.delete(`${BASE}/api/v1/users/${vu.id}`, { headers: authHeader(token) }).catch(() => {})
  })
})
```

- [ ] **Step 2: Run E2E**

```bash
rtk ./e2e/run-e2e.sh tests/projects.spec.ts
```

Expected: all tests pass.

- [ ] **Step 3: Commit**

```bash
rtk git add e2e/tests/projects.spec.ts
rtk git commit -m "test(e2e): add projects management spec"
```

---

## Task 15: E2E — catalog project filter

**Files:**
- Modify: `e2e/tests/catalog.spec.ts` (add one test) — OR create `e2e/tests/catalog-project-filter.spec.ts` if catalog.spec is already large.

- [ ] **Step 1: Write the test**

Append to `e2e/tests/catalog.spec.ts` inside the main describe block:

```ts
test('UI: project filter scopes catalog list', async ({ page, request }) => {
  const token = await loginViaAPI(request)
  const projectRes = await request.post(`${BASE}/api/v1/projects`, {
    headers: authHeader(token),
    data: { name: 'e2e-filter-project' },
  })
  const project = await projectRes.json()

  const scopedRes = await request.post(`${BASE}/api/v1/catalog`, {
    headers: authHeader(token),
    data: {
      display_name: 'E2E Scoped Agent',
      description: 'scoped',
      protocol: 'a2a',
      endpoint: `http://e2e-scoped-${Date.now()}.example.com`,
      version: '1.0.0',
    },
  })
  const scoped = await scopedRes.json()
  await request.post(`${BASE}/api/v1/catalog/${scoped.id}/projects`, {
    headers: authHeader(token),
    data: { project_party_id: project.id },
  })

  await loginViaUI(page)
  await page.goto(`/?project=${project.id}`)
  await expect(page.getByText('E2E Scoped Agent')).toBeVisible()

  // Cleanup
  await request.delete(`${BASE}/api/v1/catalog/${scoped.id}`, { headers: authHeader(token) }).catch(() => {})
  await request.delete(`${BASE}/api/v1/projects/${project.id}`, { headers: authHeader(token) }).catch(() => {})
})
```

- [ ] **Step 2: Run E2E**

```bash
rtk ./e2e/run-e2e.sh tests/catalog.spec.ts
```

- [ ] **Step 3: Commit**

```bash
rtk git add e2e/tests/catalog.spec.ts
rtk git commit -m "test(e2e): verify catalog project filter scopes results"
```

---

## Task 16: Screenshot tests + end-user guide

**Files:**
- Modify: `e2e/tests/docs-screenshots.spec.ts`
- Modify: `docs/end-user-guide.md`

- [ ] **Step 1: Extend `docs-screenshots.spec.ts` seed block**

In the `beforeAll`, after the `demoGroupId` seeding, add project seeding:

```ts
let demoProjectId: string

// At top with other let's:
// let demoProjectId: string

// Inside beforeAll:
const projSeedRes = await request.post(`${BASE}/api/v1/projects`, {
  headers: authHeader(token),
  data: { name: 'docs-demo-project-detail' },
})
if (projSeedRes.ok()) {
  demoProjectId = (await projSeedRes.json()).id as string
  const partiesRes = await request.get(`${BASE}/api/v1/parties?kind=person`, { headers: authHeader(token) })
  if (partiesRes.ok()) {
    const parties = (await partiesRes.json()) as Array<{ id: string }>
    if (parties[0]) {
      await request.post(`${BASE}/api/v1/projects/${demoProjectId}/members`, {
        headers: authHeader(token),
        data: { party_id: parties[0].id, role: 'project:owner' },
      }).catch(() => {})
    }
  }
  // Assign the a2a entry to the project so the entries panel has a row
  await request.post(`${BASE}/api/v1/catalog/${a2aEntryId}/projects`, {
    headers: authHeader(token),
    data: { project_party_id: demoProjectId },
  }).catch(() => {})
}
```

- [ ] **Step 2: Extend `afterAll`**

```ts
const projListRes = await request.get(`${BASE}/api/v1/projects`, { headers: authHeader(token) })
if (projListRes.ok()) {
  const projects = (await projListRes.json()) as Array<{ id: string; name: string }>
  for (const p of projects.filter(p => p.name.startsWith('docs-demo-project-detail'))) {
    await request.delete(`${BASE}/api/v1/projects/${p.id}`, { headers: authHeader(token) })
      .catch(ignoreCleanupError(`delete ${p.name}`))
  }
}
```

- [ ] **Step 3: Add two screenshot tests before the closing `});`**

```ts
test('projects-tab', async ({ page }) => {
  await page.setViewportSize(VIEWPORT)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await loginViaUI(page)
  await page.goto('/settings')
  await page.getByRole('tab', { name: /^projects$/i }).click()
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: `${DOCS_IMAGES}/projects-tab.png`, fullPage: false })
})

test('project-detail', async ({ page }) => {
  await page.setViewportSize(VIEWPORT)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await loginViaUI(page)
  await page.goto(`/settings/projects/${demoProjectId}`)
  await page.waitForLoadState('networkidle')
  await page.screenshot({ path: `${DOCS_IMAGES}/project-detail.png`, fullPage: false })
})
```

- [ ] **Step 4: Run screenshots**

```bash
rtk make docs-screenshots
```

(If the task fails on an unrelated transient seed, retry only the new tests via `rtk ./e2e/run-e2e.sh tests/docs-screenshots.spec.ts --grep "projects-tab|project-detail"`.)

- [ ] **Step 5: Reference screenshots in `docs/end-user-guide.md`**

Locate the **Groups and Projects** → **Managing Projects** section. After the first `curl` (create project), add:

```markdown
Projects can be managed from the **Settings → Projects** tab in the web UI.

![Settings page showing the Projects tab populated with demo projects](images/projects-tab.png)
*The Projects tab inside Settings — admins can create and delete projects here.*
```

After the second `curl` (assign member), add:

```markdown
![Project detail page showing members with roles and an assigned catalog entry](images/project-detail.png)
*Project detail page — manage members (with roles) and assigned catalog entries.*
```

- [ ] **Step 6: Commit**

```bash
rtk git add e2e/tests/docs-screenshots.spec.ts docs/end-user-guide.md docs/images/projects-tab.png docs/images/project-detail.png
rtk git commit -m "docs: add projects UI screenshots and wire into end-user guide"
```

---

## Task 17: Full validation

- [ ] **Step 1: Unit + lint**

```bash
rtk make web-test web-lint
```

Expected: all pass.

- [ ] **Step 2: Backend + arch**

```bash
rtk make test arch-test
```

Expected: all pass. If arch-test fails on `party_store.go` public-function count, apply the split suggested in Task 1 Step 5.

- [ ] **Step 3: Full E2E**

```bash
rtk make web-build
rtk make e2e-test
```

Expected: all specs pass including new `projects.spec.ts` and the catalog project-filter test.

- [ ] **Step 4: `make all`**

```bash
rtk make all
```

Expected: green.

---

## Completion criteria

- Groups continue to work exactly as before the refactor (test + screenshot equivalence).
- Projects tab visible to all authenticated users; admins can create/delete/add-member/edit-role/assign-entry.
- Member role column displays on project detail page; clicking opens edit dialog.
- Assigned catalog entries panel shows entries; assign dialog searches catalog and filters out already-assigned.
- Catalog list page has a "Project" filter dropdown persisting via `?project=<id>`.
- Cycle detection still works on projects (reuses groups' translation logic).
- Unit tests green; E2E green; screenshots generated and wired into end-user guide.
- `rtk make all` passes.
- `docs/api.md` documents the new PATCH route.

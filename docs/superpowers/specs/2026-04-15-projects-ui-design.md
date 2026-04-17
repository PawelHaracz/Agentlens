# Projects UI + Catalog Integration — Design Spec

**Date:** 2026-04-15
**Status:** Draft
**Predecessor specs:** [2026-04-15-groups-ui-design.md](2026-04-15-groups-ui-design.md), [2026-04-14-party-archetype-design.md](2026-04-14-party-archetype-design.md)
**Relevant ADRs:** [011-party-archetype-unified-actor-model.md](../../adr/011-party-archetype-unified-actor-model.md), [012-materialized-group-closure-for-permission-resolution.md](../../adr/012-materialized-group-closure-for-permission-resolution.md), [013-frontend-party-ui-kind-config.md](../../adr/013-frontend-party-ui-kind-config.md), [007-embedded-spa-frontend.md](../../adr/007-embedded-spa-frontend.md)

## Goal

Add a Projects management UI alongside the just-shipped Groups UI, plus a project filter on the catalog list page. Generalize the Groups components so Groups and Projects share one parameterized implementation.

## Non-goals

- Bulk catalog-entry assignment from the catalog list (multi-select rows). Deferred — single-entry assignment from the project detail page is sufficient for v1.
- Project-role-aware UI permission gating (e.g. project `owner` sees mutate buttons even without global `catalog:write`). Backend currently only checks the global permission; honoring project-roles client-side without a backend change would mislead users with 403s.
- Auto-creating a Person party on user creation. This is a known gap from the party archetype rollout (see "Known limitations"). Not introduced by this spec; not fixed here either.
- A global "current project" header switcher. That belongs to spec (3) — Current-user project context.

## Background

The party archetype backend ([party-archetype-design.md](2026-04-14-party-archetype-design.md)) ships projects as `Party{Kind: "project"}` with a `catalog_project_memberships` join table. Existing endpoints:

- `GET/POST /api/v1/projects`, `GET/DELETE /api/v1/projects/{id}` — CRUD
- `GET/POST /api/v1/projects/{id}/members`, `DELETE /api/v1/projects/{id}/members/{memberID}` — membership
- `POST /api/v1/catalog/{entryID}/projects`, `DELETE /api/v1/catalog/{entryID}/projects/{projectID}` — entry assignment
- `GET /api/v1/catalog?project=<id>` — catalog filter

Project member roles are `project:owner`, `project:developer`, `project:viewer` (per `projectKindConfig.ValidMemberRoles` in `internal/api/party_kind_configs.go`).

The Groups UI ([groups-ui-design.md](2026-04-15-groups-ui-design.md)) shipped in `web/src/routes/groups/` with `GroupsTab`, `GroupDetailPage`, `CreateGroupDialog`, `AddMemberDialog`. This spec generalizes those components.

## Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Refactor Groups into a kind-parameterized `PartyTab` / `PartyDetailPage` / dialogs in `web/src/routes/parties/`. Mount twice in `SettingsPage` with `groupUIConfig` and `projectUIConfig`. | Avoids ~70% duplicated code that would diverge over time. Frontend `PartyUIConfig` mirrors backend `PartyKindConfig` — same extensibility story per ADR-011. |
| D2 | Project detail page adds two project-only sections gated by `config.showMemberRoleColumn` and `config.showEntriesPanel`: a Role column on the members table and an "Assigned catalog entries" panel below members. | Keeps page layout consistent with groups while surfacing project-specific data. Conditional rendering driven by config, not kind checks scattered through the component. |
| D3 | Catalog list adds a Project filter Select alongside protocol/source/state. URL-driven via `?project=<id>`. | Symmetric with existing filter UX. Avoids implying a "global project mode" (that's spec 3's scope). |
| D4 | Assign-entry uses a search-then-pick dialog (debounced search, results show display_name + protocol + endpoint + version). | Catalogs realistically grow past dropdown size and entries need disambiguation. Combobox would degrade. |
| D5 | Edit-member-role uses a small dialog (mirrors AddMemberDialog shape) instead of inline editing. | Consistent with the rest of the app (no inline-edit pattern exists today). Role changes are infrequent — the extra click is fine. |
| D6 | Permission gating mirrors groups: anyone authenticated sees the Projects tab and read-only data; only `catalog:write` shows Create/Delete/Add-member/Assign-entry/Edit-role buttons. | Backend currently enforces `catalog:write` (per `projectKindConfig.ManagePermission`). Showing buttons that would 403 misleads users. |
| D7 | Drop the planned `GET /api/v1/projects/{id}/entries` endpoint. Reuse `GET /api/v1/catalog?project=<id>`. | The data is the same — `catalog_project_memberships` rows joined to catalog entries. No need for a parallel endpoint. |
| D8 | Add `PATCH /api/v1/projects/{id}/members/{memberID}` to update an existing membership's `from_role` in place. | Remove+add would trigger a closure rebuild (ADR-012) and lose the edge transiently. The role is just a field on the existing `PartyRelationship`; updating it in place avoids both costs. |

## Architecture

### Component layout

```
web/src/routes/parties/                       (renamed from web/src/routes/groups/)
├── partyUIConfig.ts                          ← config map: groupUIConfig + projectUIConfig
├── PartyTab.tsx + .test.tsx                  ← list + create + delete (was GroupsTab)
├── CreatePartyDialog.tsx + .test.tsx         ← name input (was CreateGroupDialog)
├── PartyDetailPage.tsx + .test.tsx           ← drill-down: members table + optional sections
├── AddMemberDialog.tsx + .test.tsx           ← party picker; gains optional roleOptions prop
├── EditMemberRoleDialog.tsx + .test.tsx      ← NEW: change a project member's role
├── ProjectEntriesPanel.tsx + .test.tsx       ← NEW: assigned catalog entries (project-only)
└── AssignEntryDialog.tsx + .test.tsx         ← NEW: search-then-pick catalog assignment

web/src/pages/SettingsPage.tsx                ← mount <PartyTab config={projectUIConfig}/> as a new "Projects" tab
web/src/App.tsx                               ← add /settings/projects/:id route
web/src/routes/catalog/CatalogListPage.tsx    ← add Project filter Select to existing filter bar
web/src/api.ts                                ← project API client + catalog-project assign/unassign + member-role PATCH
```

### `PartyUIConfig` shape

```ts
export interface PartyUIConfig {
  kind: 'group' | 'project'
  urlPrefix: 'groups' | 'projects'                    // /api/v1/<urlPrefix>
  detailPath: (id: string) => string                  // /settings/<urlPrefix>/:id
  labels: { single: string; plural: string }          // 'Project' / 'Projects'
  writePermission: string                             // 'users:write' | 'catalog:write'
  memberRoleOptions: string[]                         // ['member'] | ['project:owner', ...]
  defaultMemberRole: string                           // 'member' | 'project:viewer'
  showMemberRoleColumn: boolean                       // false | true
  showEntriesPanel: boolean                           // false | true
  cycleErrorMessage: string                           // friendly translation
}
```

`groupUIConfig` exists in renamed form (no behavior change). `projectUIConfig` is new.

### Backend changes

| Route | Method | Purpose | Notes |
|---|---|---|---|
| `/api/v1/projects/{id}/members/{memberID}` | PATCH | Update a member's role on a project | New handler. Body: `{role: string}`. Validates role against `projectKindConfig.ValidMemberRoles`. Updates `PartyRelationship.from_role` in place — no closure rebuild required since containment is unchanged. Requires `catalog:write`. |

No new tables. No schema changes. No changes to existing routes.

### Data flow — Project detail page

1. `PartyDetailPage` reads `:id` from route, mounts with `projectUIConfig`.
2. Parallel fetches: `getProject(id)`, `listProjectMembers(id)`, `listParties()` (for the AddMember picker), `listCatalog({project: id})` (for ProjectEntriesPanel — only when `config.showEntriesPanel`).
3. Members table renders with extra Role column when `config.showMemberRoleColumn`. Role badge cell triggers `EditMemberRoleDialog`.
4. **Add member** → `AddMemberDialog` with `roleOptions={config.memberRoleOptions}` and `defaultRole={config.defaultMemberRole}` → `api.addProjectMember(projectId, partyId, role)` → reload members.
5. **Remove member** → `confirm(...)` → `api.removeProjectMember(projectId, partyId)` → reload.
6. **Edit role** → `EditMemberRoleDialog` → `api.updateProjectMemberRole(projectId, partyId, role)` → reload.
7. **ProjectEntriesPanel** → "Assign entry" → `AssignEntryDialog` (debounced search, calls `api.listCatalog({q, limit: 20})`, filters out already-assigned entries) → `api.assignEntryToProject(entryId, projectId)` → reload entries.
8. **Unassign entry** → confirm → `api.removeEntryFromProject(entryId, projectId)` → reload.

### Data flow — Catalog project filter

1. `CatalogListPage` adds a Select with options "All projects" + each project (fetched via `listParties('project')`, cached via React Query for the session).
2. Selecting a project sets `?project=<id>` and triggers `listCatalog({project: id})`.
3. "All projects" clears the param.
4. URL is the source of truth (matches existing protocol/state/source filter pattern in `useCatalogFilters`).

## API surface (frontend)

```ts
// projects (kind-scoped wrappers; share request/response shape with groups)
listProjects(): Promise<Party[]>
getProject(id: string): Promise<Party>
createProject(data: { name: string }): Promise<Party>
deleteProject(id: string): Promise<void>
listProjectMembers(id: string): Promise<PartyRelationship[]>
addProjectMember(projectId: string, partyId: string, role: string): Promise<void>
removeProjectMember(projectId: string, memberPartyId: string): Promise<void>
updateProjectMemberRole(projectId: string, memberPartyId: string, role: string): Promise<void>

// catalog-project assignment
assignEntryToProject(entryId: string, projectId: string): Promise<void>
removeEntryFromProject(entryId: string, projectId: string): Promise<void>

// listCatalog already supports `project` filter — extend ListFilter type if missing
```

## Error handling

- Network errors → inline error message in dialogs (mirrors groups pattern).
- 409 Conflict on AddMember (cycle detection) → translated by existing `translateError` in `AddMemberDialog`. Project nesting cycles use the same backend code path as groups.
- 403 Forbidden (write actions without `catalog:write`) → caller already hides the button; if the request still fires, surface raw error message.
- 404 on detail page → show "Project not found" with back link (mirrors groups).

## Testing

**Unit (Vitest + RTL):**
- `partyUIConfig.ts` — type-only file, no test.
- `PartyTab.test.tsx` — empty state, list render, create dialog open, hide create without write perm, delete confirm. Run with both configs (parametric describe block).
- `CreatePartyDialog.test.tsx` — create + error + disable on empty.
- `PartyDetailPage.test.tsx` — header, back link, 404, member rows, empty state, hide add without write, open add dialog, remove member confirm. Project-specific: role column visible, edit-role dialog opens, entries panel renders.
- `AddMemberDialog.test.tsx` — filter excluded/existing/wrong-kind parties; submit calls correct API with role; cycle error translated.
- `EditMemberRoleDialog.test.tsx` — current role pre-selected; submit calls update; error displayed.
- `ProjectEntriesPanel.test.tsx` — empty state, list render, hide assign without write, remove confirm.
- `AssignEntryDialog.test.tsx` — debounced search fires `listCatalog`, results render, already-assigned filtered, submit assigns.
- `CatalogListPage.test.tsx` — add: Project select changes URL, "All projects" clears param.

**E2E (Playwright):**
- `e2e/tests/projects.spec.ts` — admin CRUD, add member with role, edit role, assign entry, unassign entry, viewer read-only visibility.
- Extend `e2e/tests/catalog.spec.ts` (or new spec) — filter catalog by project via the Select.
- `e2e/tests/docs-screenshots.spec.ts` — two new screenshots: `projects-tab.png`, `project-detail.png`. Reuse `docs-demo` project already seeded for catalog-project-filter.

**Backend (Go):**
- `internal/api/party_handlers_test.go` — `PATCH /projects/:id/members/:memberID` happy path, invalid role, missing membership, permission denied.
- `internal/store/party_store_test.go` — `UpdateMemberRole` updates `from_role` and does not rebuild closure.

## Migration & rollout

- File rename `web/src/routes/groups/` → `web/src/routes/parties/` is a `git mv`, preserving history. Imports in `SettingsPage.tsx` and `App.tsx` update accordingly. The plan's first task does this rename atomically with no behavior change so the tests stay green.
- Component rename `Group*` → `Party*` (`GroupsTab` → `PartyTab` etc.) inside the same task as the rename.
- Public route `/settings/groups/:id` is preserved for the groups link; new `/settings/projects/:id` added.

## Known limitations (carried over)

- New users created via `/api/v1/users` do not get an auto-created Person party. The AddMember picker only lists Persons that exist (today: only the bootstrap admin). Closing this gap requires a separate change to user creation; it is not in scope here. Documented in [groups-ui-design.md](2026-04-15-groups-ui-design.md) as well.

## Open questions

None at draft time. Plan author should confirm the backend `PATCH` route lands cleanly (D8) before frontend tasks rely on it.

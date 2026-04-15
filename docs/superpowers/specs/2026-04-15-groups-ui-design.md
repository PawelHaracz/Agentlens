# Groups Management UI — Design Spec

**Date:** 2026-04-15
**Status:** Approved
**Scope:** Spec 1 of 3 in the party archetype UI decomposition.
**Out of scope:** Projects UI (spec 2), catalog project filter (spec 2), current user's project memberships (spec 3).

---

## Goal

Expose the existing party archetype **groups** backend API through the AgentLens web UI. Give administrators a place to create groups, view their membership, add and remove members (persons or nested groups), and delete groups.

## Non-goals

- Projects UI (lists, detail, catalog membership)
- Assigning global system roles to groups (`GlobalPartyRole`) — no UI in v1
- Editing a group's name after creation — deferred
- Showing transitive/inherited members in a consolidated view — direct members only

## Context & Constraints

- Backend endpoints are live: `GET/POST /api/v1/groups`, `GET/DELETE /api/v1/groups/{id}`, `GET/POST /api/v1/groups/{id}/members`, `DELETE /api/v1/groups/{id}/members/{memberPartyID}` (see [docs/api.md](../../api.md)).
- Permission check on the backend is `party:write` for all mutating endpoints; read endpoints are auth-gated only.
- ADR-007 locks us to React 18 + React Router v6 + shadcn/ui + TanStack React Query + Vite + TypeScript.
- ADR-011 defines `Party`, `PartyRelationship`, and the party kinds. The UI model mirrors these directly.
- Backend cycle detection rejects edges that would create loops; the UI pre-empts this client-side where feasible but also surfaces backend errors gracefully.

---

## Architecture

### Routes

| Route | Description |
|---|---|
| `/settings` (existing) | Adds a new **Groups** tab alongside Users / Roles / My Account. |
| `/settings/groups/:id` (new) | Full-page drill-down for a single group's members. |

The detail route is a sibling of `/settings` — not nested — because `SettingsPage` owns its tab switching and nesting would fight that pattern. The detail page renders inside `<Layout>` and shows a `← Back to Settings` link.

### Component tree

```
SettingsPage (existing)
├── GeneralTab (existing)
├── UsersTab (existing)
├── RolesTab (existing)
├── GroupsTab (new)
│   ├── GroupsTable
│   ├── CreateGroupDialog
│   └── DeleteGroupConfirmDialog (inline shadcn AlertDialog)
└── MyAccountTab (existing)

GroupDetailPage (new, standalone route wrapped by Layout)
├── BackToSettingsLink
├── GroupHeader (name, created date)
├── MembersTable
├── AddMemberDialog
│   └── PartyCombobox (shadcn Command)
└── RemoveMemberConfirmDialog
```

### Data flow

All reads go through TanStack React Query. Keys:

- `['groups']` — list of groups.
- `['group', id, 'members']` — direct members of a group (returns `PartyRelationship[]`).
- `['parties']` — all parties, used to resolve party names/kinds from member relationship rows and to populate the Add Member picker.

Mutations invalidate the affected keys:

- `createGroup` → invalidate `['groups']` and `['parties']`
- `deleteGroup` → invalidate `['groups']` and `['parties']`
- `addGroupMember(groupId, partyId)` → invalidate `['group', groupId, 'members']`
- `removeGroupMember(groupId, partyId)` → invalidate `['group', groupId, 'members']`

### Client-side join

`listGroupMembers` returns `PartyRelationship[]`. To render a member row (name + kind + remove button) we join against `['parties']` client-side rather than expanding the backend response. This avoids an N+1 and keeps the server API simple.

---

## API Client Additions

Add to [web/src/api.ts](../../../web/src/api.ts):

```ts
export interface Party {
  id: string
  kind: 'person' | 'group' | 'project'
  name: string
  user_id?: string
  is_system: boolean
  created_at: string
  updated_at: string
}

export interface PartyRelationship {
  id: string
  from_party_id: string
  from_role: string
  to_party_id: string
  to_role: string
  relationship_name: string
}

export function listGroups(): Promise<Party[]>
export function createGroup(data: { name: string }): Promise<Party>
export function getGroup(id: string): Promise<Party>
export function deleteGroup(id: string): Promise<void>
export function listGroupMembers(id: string): Promise<PartyRelationship[]>
export function addGroupMember(groupId: string, partyId: string): Promise<void>
export function removeGroupMember(groupId: string, memberPartyId: string): Promise<void>
export function listParties(kind?: 'person' | 'group' | 'project'): Promise<Party[]>
```

`listParties` is added now (not in spec 2 or 3) because the Add Member picker needs it. Kind filter is optional so spec 2/3 can reuse it.

---

## UI Details

### GroupsTab (inside `SettingsPage`)

- shadcn `<Table>` with columns: **Name**, **Direct members** (count), **Created**, **Actions**.
- Top-right **Create group** button. Hidden if the current user lacks `party:write`.
- Row click navigates to `/settings/groups/<id>`.
- Per-row **Delete** icon button (trash icon, `variant="ghost"`). Hidden without `party:write`. Click opens shadcn `<AlertDialog>` with confirm/cancel.
- Loading: shadcn `<Skeleton>` rows (3 rows) inside the table body.
- Empty state: centered message *"No groups yet. Click **Create group** to get started."* (or *"No groups yet."* for users without `party:write`).

### CreateGroupDialog

- shadcn `<Dialog>` with a single **Name** input (required, 1–128 chars — matches backend validation).
- Submit button labeled **Create**; disabled until name is non-empty; shows spinner during mutation.
- On success: invalidate `['groups']` and `['parties']`, close dialog, reset form.
- On error: inline `<Alert variant="destructive">` under the input with the error message.

### GroupDetailPage

- Route: `/settings/groups/:id`.
- Header:
  - `← Back to Settings` link (routes back to `/settings` with the Groups tab active — preserve the tab via query string, e.g. `/settings?tab=groups`).
  - Group name (from `useQuery(['parties'])` lookup by `id`, with fallback to `getGroup(id)` if not found in the parties cache).
  - Created date.
- **Members** section with an **Add member** button (hidden without `party:write`).
- Members table columns: **Name**, **Kind** (shadcn `<Badge>`: *Person* or *Group*), **Actions** (remove icon, hidden without `party:write`).
- 404 on fetch → inline empty state *"Group not found."* + back link.
- Empty members state: *"No members yet. Click **Add member** to add the first person or group."*

### AddMemberDialog

- shadcn `<Dialog>` with a **party picker** — shadcn `<Command>` / `<Combobox>` pattern — listing all parties from `listParties()`.
- Search matches case-insensitive substring against `party.name`.
- Each row renders: `<Badge>` (Person or Group) + name.
- Filter out, client-side:
  - the current group itself
  - all ancestors of the current group (resolved from `listGroupMembers` recursively — kept simple since the data is already cached)
  - parties that are already direct members
- Submit → `addGroupMember(groupId, partyId)` → invalidate `['group', id, 'members']` → close.
- Backend cycle error (matched by message substring "cycle") displays as *"This member is already in the group's ancestry — adding them would create a cycle."* instead of the raw backend string.

### RemoveMemberConfirmDialog

- shadcn `<AlertDialog>`. Confirm text: *"Remove **{memberName}** from **{groupName}**?"*
- On confirm → `removeGroupMember(groupId, partyId)` → invalidate members.

---

## Permissions

| Action | Required permission | UI treatment |
|---|---|---|
| View groups list | auth | always visible |
| View group detail | auth | always visible |
| Create group | `party:write` | button hidden |
| Delete group | `party:write` | button hidden |
| Add member | `party:write` | button hidden |
| Remove member | `party:write` | button hidden |

Client-side gating is informative only; the backend remains authoritative. We add a `hasPermission(perm: string): boolean` helper on `AuthContext` if one doesn't already exist (check during implementation — if present, reuse).

A 403 response from any mutating call surfaces as an inline `<Alert variant="destructive">`: *"You don't have permission to perform this action."* — no redirect.

---

## Error Handling

- All API errors surface via shadcn `<Alert variant="destructive">` inside the relevant panel (dialog body or page header). No toasts.
- Loading states: `<Skeleton>` rows during fetch; mutation buttons disable with a spinner.
- 404 on detail page: empty state with back link; no hard redirect.
- Network errors (fetch rejection): *"Network error. Please try again."*

---

## Testing

### Unit tests (Vitest + React Testing Library)

Co-located `.test.tsx` files for every new component:

| File | Scenarios |
|---|---|
| `GroupsTab.test.tsx` | empty state, list render, create button hidden without permission, delete confirm flow |
| `CreateGroupDialog.test.tsx` | required-field validation, successful create, error inline, spinner during submit |
| `GroupDetailPage.test.tsx` | member table render, 404 empty state, add/remove actions hidden without permission, cycle-error message |
| `AddMemberDialog.test.tsx` | search filtering, ancestor exclusion, existing-member exclusion, kind badge render, cycle-error translation |

All tests use `vi.mock('@/api')` following the existing SettingsPage test patterns (verified: no msw in use today).

### E2E test (Playwright) — `e2e/tests/groups.spec.ts`

Scenarios (reuse helpers from `e2e/tests/helpers.ts`):

1. **admin** — create a group, see it in the list, navigate to detail, delete, verify removed
2. **admin** — add a person to a group
3. **admin** — remove a member
4. **admin** — nest group B inside group A; verify B appears as a member of A
5. **admin** — attempt to add group A as a member of group B (where B ⊂ A) → inline cycle error
6. **viewer** — sees the groups list and detail but no create/delete/add/remove controls

Run via `rtk make e2e-test`.

### Documentation screenshots — extend `e2e/tests/docs-screenshots.spec.ts`

Two new tests append to the existing `describe` block, reusing the `token` + seeded entries from `beforeAll`:

- `groups-tab.png` — Settings page with Groups tab active, showing 2–3 seeded groups.
- `group-detail.png` — group detail page showing a mix of person and nested-group members.

Cleanup in `afterAll` deletes any groups created for the screenshots.

Both images are referenced inline in the Groups and Projects section of [docs/end-user-guide.md](../../end-user-guide.md) once implementation lands.

---

## Non-functional

- **No new dependencies** — everything uses React 18, React Router v6, React Query, shadcn/ui components already in the tree.
- **Bundle impact:** expected < 20 kB gzip — four new components + one dialog, no heavy libs.
- **Accessibility:** shadcn primitives are ARIA-correct; we verify focus management in the dialogs via the unit tests.
- **i18n:** out of scope — strings are hard-coded English, matching the rest of the app.

---

## Open questions / risks

None. All backend API contracts are already shipped; frontend-only changes; no schema migrations required.

---

## Success criteria

- Admin can create, view, drill into, delete groups via the UI.
- Admin can add and remove members (persons and nested groups).
- Cycle attempts are rejected with a clear message.
- Non-admin users see the list/detail but cannot mutate.
- All unit tests pass (`rtk make web-test`).
- E2E spec passes (`rtk make e2e-test`).
- Screenshots regenerate cleanly.
- `rtk make web-lint` and `rtk make arch-test` pass.

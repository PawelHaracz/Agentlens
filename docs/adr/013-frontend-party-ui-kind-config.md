# ADR-013: Frontend Party UI Parameterized by Kind Config

Date: 2026-04-15
Status: Accepted

## Context

ADR-011 established a unified party archetype on the backend: all actor types (persons,
groups, projects, and future kinds) share a single `parties` table discriminated by `kind`,
and route registration is driven by a `PartyKindConfig` table in `internal/api/party_kind_configs.go`.
Adding a new party kind on the backend costs ~10 lines of config with zero handler code.

The frontend, however, shipped Groups and Projects as two parallel implementations during
the party archetype rollout: `web/src/routes/groups/*` and `web/src/routes/projects/*` — two
`Tab` components, two detail pages, two dialog sets. Both implement ~70% identical behaviour
(list + create + delete + member CRUD) and diverge only in a few details (URL prefix, labels,
write-permission name, member-role options, whether an "Assigned catalog entries" panel is
rendered, cycle-error message wording).

This parallel structure has two problems:

- **Drift.** The two trees inevitably grow apart — a bug fix or UX polish applied to one side
  doesn't propagate to the other. We already hit this during the Projects rollout: the groups
  cycle-error translation, keyboard handling in dialogs, and "click-row-to-drill-down" pattern
  all had to be re-applied manually.
- **Asymmetric extensibility.** The backend's key win from ADR-011 is that adding a new kind
  is a config change. The frontend as shipped would require duplicating a page tree for every
  new kind — a structural change, not a config one. The two tiers' extensibility stories
  don't match.

## Decision

Parameterize the frontend party UI by a **`PartyUIConfig`** record — the frontend mirror of
the backend's `PartyKindConfig`. A single set of components in `web/src/routes/parties/` is
mounted once per kind with a different config. Kind-specific sections (member-role column,
entries panel, etc.) are gated by boolean flags on the config, not by `kind` checks scattered
through the components.

**Shape:**

```ts
export interface PartyUIConfig {
  kind: 'group' | 'project'
  urlPrefix: 'groups' | 'projects'
  detailPath: (id: string) => string
  labels: { single: string; plural: string }
  writePermission: string              // global permission that gates mutation UI
  memberRoleOptions: string[]          // ['member'] | ['project:owner', 'project:developer', 'project:viewer']
  defaultMemberRole: string
  showMemberRoleColumn: boolean
  showEntriesPanel: boolean
  cycleErrorMessage: string
}
```

**Components** (all in `web/src/routes/parties/`):

| Component | Responsibility | Kind-aware via |
|-----------|---------------|----------------|
| `PartyTab` | List + create + delete | `config.urlPrefix`, `config.writePermission`, `config.labels` |
| `CreatePartyDialog` | Name input + create | `config.labels`, kind-appropriate description text |
| `PartyDetailPage` | Members table + conditional sections | `config.showMemberRoleColumn`, `config.showEntriesPanel` |
| `AddMemberDialog` | Party picker + optional role select | `config.memberRoleOptions`, `config.defaultMemberRole` |
| `EditMemberRoleDialog` | Change member role (projects only) | Mounted only when `showMemberRoleColumn` |
| `ProjectEntriesPanel` | Assigned catalog entries (projects only) | Mounted only when `showEntriesPanel` |
| `AssignEntryDialog` | Search-then-pick catalog entry | — |

Each component takes `config: PartyUIConfig` as a prop. Kind-specific API functions
(`listGroups` vs `listProjects`, etc.) are selected inside the component via a small switch
on `config.kind` — the frontend does not unify the API client across kinds, because the
backend route trees are already kind-scoped and the generated request shapes match that
scoping.

**Mounting:** `SettingsPage.tsx` mounts `<PartyTab config={groupUIConfig} />` and
`<PartyTab config={projectUIConfig} />` as two tabs. `App.tsx` registers two detail routes
(`/settings/groups/:id` and `/settings/projects/:id`) pointing at the same
`PartyDetailPage` with different config props.

## Consequences

### Positive

- **Single source of truth.** A polish or bug fix to member-row rendering, cycle-error
  translation, keyboard handling, or Add-member filter logic ships to both kinds at once.
- **Symmetric extensibility with the backend.** Adding a third kind (e.g. an `organization`)
  is a new `PartyUIConfig` entry plus one tab mount — no page-tree duplication. The
  frontend extensibility story now matches ADR-011's backend story.
- **Config as documentation.** `partyUIConfig.ts` is a readable map of what differs between
  kinds — instead of that knowledge being spread across two page trees.
- **Test reuse.** Parametric `describe.each` over both configs runs the same suite for
  every kind, catching behaviour drift the moment it appears.

### Negative / Trade-offs

- **Project-specific sections live in the shared tree.** `EditMemberRoleDialog` and
  `ProjectEntriesPanel` are mounted conditionally from `PartyDetailPage` based on config
  flags. They still live in `routes/parties/` alongside kind-agnostic components. A reader
  has to look at the config to know which kinds use them. Mitigated by filename prefix
  (`Project*` for project-only components) and a comment at the top of each project-only
  component naming the flag that mounts it.
- **Small indirection cost.** A developer debugging `PartyTab` must hop to `partyUIConfig.ts`
  to see why, say, the Create button's permission check is `catalog:write` in one mount and
  `users:write` in another. Acceptable — the same indirection exists on the backend with
  `PartyKindConfig`, and matches the ADR-011 mental model.
- **Not all divergence fits a flag.** Future features that aren't "add/remove a section"
  may not map cleanly onto boolean config flags. When that happens, the config gains a
  named hook (e.g. a `renderExtraDetailSection?: () => ReactNode`) rather than a `switch(kind)`
  in the component. We accept this as a case-by-case judgment call.

### Neutral

- Per-kind API functions (`listGroups`, `listProjects`, etc.) are kept in `web/src/api.ts`
  rather than unified behind a single `listParties(kind)` call. The backend already scopes
  routes by kind; mirroring that in the client keeps the types precise and avoids runtime
  kind dispatch.
- The rename from `routes/groups/` to `routes/parties/` is a one-time cost; git history
  follows via `git mv`.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| Keep Groups and Projects as parallel page trees | Drift between near-identical code paths (~70% shared); extensibility asymmetric with backend; manual re-application of shared fixes. |
| Unify everything into a single `<PartyTab kind="..." />` with `switch(kind)` branches throughout | Scatters kind knowledge through the code; config-driven approach makes the per-kind differences a single readable table. |
| Generate per-kind pages from the config at build time | Over-engineered; React's runtime prop-driven composition already does this cleanly without codegen. |
| Unify the API client behind `listParties(kind)` | Loses per-kind type precision (e.g. project members always carry a role, group members don't); runtime kind dispatch for no gain since the backend routes are already kind-scoped. |

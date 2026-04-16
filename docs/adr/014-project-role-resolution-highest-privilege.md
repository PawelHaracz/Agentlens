# ADR-014: Project Role Resolution — Highest-Privilege Wins Across Multi-Path Membership

Date: 2026-04-15
Status: Accepted

## Context

ADR-011 (party archetype) and ADR-012 (materialized closure) together describe how a user's
project-scoped role is looked up: walk the `party_group_closures` table from the user's
Person party to collect all ancestor group IDs, then find `project_member` relationships
from any of those parties to the target project.

What neither ADR specifies is the **tie-break rule** when a user reaches the same project
via multiple paths with different roles. Example:

- Alice is a `member` of group `platform`.
- Alice is a `member` of group `sre`.
- Group `platform` is a `project:developer` on project `orion`.
- Group `sre` is a `project:viewer` on project `orion`.

The closure resolution returns two rows for alice → orion: one with role `project:developer`
and one with role `project:viewer`. What is alice's effective role on orion?

This question comes up in two places:

1. **The `GET /api/v1/auth/me/projects` endpoint** introduced in
   [my-projects-context-design.md](../superpowers/specs/2026-04-15-my-projects-context-design.md)
   must return exactly one role per project for the "My projects" view.
2. **Future project-scoped permission checks** (deferred from Spec 2) will need the same
   semantic — a permission check must answer "does alice have enough role on orion?" with a
   single effective role, not a set.

Without an explicit rule, every callsite invents its own — guaranteed drift.

## Decision

When a user reaches a project via multiple paths with different roles, the user's
**effective role is the highest-privilege role** across all paths. Rank:

```
project:owner     = 2
project:developer = 1
project:viewer    = 0
(unknown role)    = -1
```

Ties at the same rank collapse to a single value (they're equal). Unknown roles never win
over known roles; they are logged at warn level so operators can spot misconfigured
relationships.

**Implementation location:** A single function in the store layer,
`PartyStore.ResolveUserProjects`, applies the rank map. Future permission-check code paths
MUST reuse this function (or a shared helper that applies the same rank map) rather than
re-implementing the tie-break.

**Direct-vs-transitive does not affect the rule.** A direct `project_member` relationship is
treated identically to a transitive one — if alice is personally `project:viewer` on orion
*and* her group is `project:owner` on orion, she is `project:owner` on orion. Direct
membership is not privileged over transitive membership; the rank alone decides.

## Consequences

### Positive

- **No silent downgrade.** A user never sees the UI offer them less access than the
  permission system would grant on a real request. "Highest wins" is the only tie-break
  choice that keeps the displayed role and the effective permission in sync.
- **Standard RBAC semantics.** Unix file permissions, AWS IAM, Kubernetes RBAC, and most
  enterprise systems resolve multi-path access by taking the most permissive result.
  Matches user expectation.
- **One function, one rule.** Future permission-check code can't accidentally disagree with
  the "My projects" view — both paths go through the same rank map.
- **Safe for nested groups.** Adding a user to a broad-scope group that has a high role on
  a project grants them that access immediately; removing them revokes it immediately
  (closure rebuild, ADR-012). No surprise where the "direct" role lingers with a different
  value.

### Negative / Trade-offs

- **A user cannot be intentionally restricted on a project via a narrow-scope relationship
  if a broader-scope one grants more.** Example: if you want alice to be *only* a viewer on
  orion despite her group being an owner, you cannot achieve that by adding a direct
  `project:viewer` edge — the group-derived `owner` still wins. Workaround: remove alice
  from the broader group, or use project-scoped deny lists (not currently modelled). We
  accept this limitation — additive/permissive models are the industry norm and inverting
  to "most restrictive wins" has its own surprising failure modes.
- **Unknown roles are silently never-wins.** A typo like `project:deveoper` will log a warn
  but the user may see no effective role for that project while a clean fix-forward goes
  unnoticed. Mitigated by backend validation on the AddMember and PATCH member-role
  handlers — unknown roles are rejected before insertion, so the `(unknown role)` branch
  exists only as defense-in-depth for historical data.

### Neutral

- **Role rank is hard-coded in Go, not DB-driven.** Moving ranks to a table would add
  runtime configurability we don't need — the three roles are fixed by the party archetype
  and change only with schema migration. Hard-coded is simpler and typo-proof.
- **Sort stability.** Returned memberships are sorted by project name ASC so the same
  inputs produce the same output — important for snapshot tests and for any future
  response-ETag caching.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| Most-restrictive wins (`viewer` beats `owner` on tie) | Surprising failure mode: adding a user to a low-role group silently downgrades their effective access across all projects that group touches. Not how operators expect groups to behave. |
| Direct membership beats transitive | Two sources of truth for "effective role" — the direct edge when present, the closure rank otherwise. Every callsite would have to re-implement the two-tier check. Opens the door to "did we remember to check direct first?" bugs. |
| Return all paths and their roles; let the caller decide | Punts the problem — we'd just get per-callsite tie-break logic, which is exactly the drift this ADR is preventing. |
| Forbid multi-path memberships at write time | Inverts the party graph's value: flexible relationships are the whole point. Many legitimate scenarios involve a user in multiple overlapping groups. |
| First-found wins (arbitrary order from the DB) | Non-deterministic; snapshot tests would flake; auditability suffers. |

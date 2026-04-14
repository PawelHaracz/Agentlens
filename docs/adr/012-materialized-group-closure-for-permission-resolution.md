# ADR-012: Materialized Group Closure for Permission Resolution

Date: 2026-04-14
Status: Accepted

## Context

The party archetype (ADR-011) introduces hierarchical group membership: a user can be a
member of a group, which can itself be a member of another group. Permission resolution
must traverse this hierarchy to determine a user's effective project roles — a user inherits
the project roles of all their ancestor groups.

The straightforward implementation uses a recursive CTE at auth time:

```sql
WITH RECURSIVE ancestors AS (
  SELECT to_party_id FROM party_relationships
  WHERE from_party_id = $userPartyID AND relationship_name = 'group_member'
  UNION
  SELECT pr.to_party_id FROM party_relationships pr
  JOIN ancestors a ON pr.from_party_id = a.to_party_id
  WHERE pr.relationship_name = 'group_member'
)
SELECT from_role FROM party_relationships
WHERE relationship_name = 'project_member'
  AND from_party_id IN (SELECT * FROM ancestors)
  AND to_party_id = $projectID
```

This runs on every authenticated API request. At low scale (< 500 parties, depth < 5) it
is fast. Under load or with deep hierarchies it degrades: O(depth × edges) per request,
with SQLite's query planner adding contention under concurrent reads.

## Decision

Pre-compute the transitive closure of all containment relationships into a
`party_group_closure(member_party_id, ancestor_party_id)` table. Rebuild the closure
synchronously within the same database transaction whenever a containment relationship
(any `relationship_name` in `ContainmentRelationships`) is added or removed.

Permission resolution steps 2 and 3 (from the spec) become indexed point lookups:

```sql
-- Step 2: group global roles
SELECT r.permissions FROM global_party_roles gpr
JOIN roles r ON r.id = gpr.role_id
WHERE gpr.party_id IN (
  SELECT ancestor_party_id FROM party_group_closures
  WHERE member_party_id = $userPartyID
)

-- Step 3: project-scoped roles
SELECT from_role FROM party_relationships
WHERE relationship_name = 'project_member'
  AND to_party_id = $projectPartyID
  AND from_party_id IN (
    $userPartyID,
    SELECT ancestor_party_id FROM party_group_closures WHERE member_party_id = $userPartyID
  )
```

No recursive CTE executes at auth time. All three steps use indexed columns.

**Per-request context cache:** `RequireProjectPermission` middleware caches the ancestor
set from `party_group_closures` in `r.Context()` on the first resolution per request.
Subsequent permission checks within the same request reuse the cached set — one DB
round-trip per request regardless of how many middleware checks fire.

**Rebuild scope:** The closure rebuild is a **full-table rebuild** — all rows in
`party_group_closures` are deleted and re-inserted from a single recursive CTE on every
containment relationship change. This is correct regardless of edge insertion order.

A scoped rebuild (only rows for the affected group) is insufficient: adding edge A→B must
also update the closure for every ancestor of B, since A's transitive descendants become
reachable from those ancestors. The full-table rebuild avoids this bookkeeping and remains
under 10ms at AgentLens scale (< 10k parties).

## Consequences

### Positive

- Auth check complexity drops from O(depth × edges) recursive CTE to O(1) indexed lookups
- Works efficiently on both SQLite and Postgres without query planner differences
- Rebuild is transactional — closure is never partially updated; no stale reads
- Per-request cache eliminates repeated DB calls within a single request

### Negative / Trade-offs

- Write operations that change group membership pay the cost of a full-table closure rebuild
  (synchronous, same transaction). At AgentLens scale this is under 10ms. Acceptable since
  membership changes are infrequent compared to reads
- `party_group_closures` is a derived table; it must never be edited directly
- Circular group membership (A member of B, B member of A) is rejected by the store layer
  before the relationship is inserted (cycle detection via a closure lookup)

### Neutral

- Closure rebuild uses a recursive CTE internally, but only on membership write, not on
  every auth read — the expensive operation is amortized
- Adding a new containment relationship kind requires adding it to
  `model.ContainmentRelationships` (`internal/model/party.go`) to trigger rebuilds;
  forgetting to do so causes stale closure data (silent bug)
- The full-table rebuild is simpler to reason about than scoped rebuilds and eliminates
  the class of correctness bugs caused by out-of-order edge insertion

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| On-demand recursive CTE at auth time | O(depth × edges) per request; degrades under load; SQLite contention |
| JWT-embedded project roles | Roles go stale until re-login; unacceptable when membership changes must take immediate effect |
| Depth limit + bounded BFS | Still O(depth × edges); adds artificial constraint on group hierarchy |
| Async closure rebuild (eventual consistency) | Auth checks could use stale data mid-rebuild; consistency complexity not worth it at current scale |

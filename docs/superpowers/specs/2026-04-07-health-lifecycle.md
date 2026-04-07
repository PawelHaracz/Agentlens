# Feature 1.2 — Health Check & Lifecycle State on Dashboard

> Ready-to-use instruction file for GitHub Copilot Agent / Claude Code.
> Project: **AgentLens** (`github.com/PawelHaracz/Agentlens`)
> Tier: **1 — MUST HAVE for launch**
> Effort: M (3–5 days)

---

## Goal

Transform AgentLens from a "glorified JSON store" into a live registry that shows the real-time state of every catalog entry. Platform engineers must be able to glance at the dashboard and immediately see: which agents are alive, which are degraded, which are gone, and how fast they respond.

## Why this matters

89% of organizations running AI agents in production have already deployed observability for them. A registry that doesn't reflect runtime state of its entries is a non-starter for those teams. Competitors (AGNTCY, Apicurio, Nacos) all surface health state. Without this feature the launch on HN/Reddit will be torn apart with "looks nice but how do I know if anything actually works".

---

## Scope (in)

1. Lifecycle state machine on `CatalogEntry`
2. Periodic health probe worker (already partially configurable — extend it)
3. State transitions persisted in storage (both SQLite and PostgreSQL)
4. `status`, `lastSeen`, `latencyMs` fields exposed via REST API
5. Dashboard: colored badges, last-seen relative timestamp, latency display
6. Manual "probe now" action from the dashboard (admin/editor only)

## Scope (out — explicitly NOT this feature)

- Alerting / notifications (email, Slack, webhooks) — backlog
- Historical health charts / time-series — backlog
- SLA tracking, uptime % — backlog
- Custom probe scripts per agent — backlog
- Probing the inside of A2A skills (only the transport endpoint is probed)

---

## Domain model changes

### Lifecycle states

```
registered → active → degraded → offline → deprecated
```

| State        | Meaning                                                      | Trigger                                                      |
| ------------ | ------------------------------------------------------------ | ------------------------------------------------------------ |
| `registered` | Entry created but not yet probed                             | Initial state on POST `/api/v1/catalog`                      |
| `active`     | Last probe succeeded within `healthyThreshold`               | Probe → 2xx within timeout                                   |
| `degraded`   | Last probe slow (latency > `degradedLatencyMs`) OR 1 failure within window | Probe → 2xx but slow, OR single failure                      |
| `offline`    | `failureThreshold` consecutive failures                      | Probe → error / 5xx / timeout `failureThreshold` times in a row |
| `deprecated` | Manually set by admin via API/UI                             | PATCH `/api/v1/catalog/{id}/lifecycle`                       |

`deprecated` is a **terminal manual state** — the probe worker must skip deprecated entries entirely.

### `CatalogEntry` additions

Add to `internal/model/catalog_entry.go` (do **not** rename to `Agent` — see Known Traps):

```go
type LifecycleState string

const (
    LifecycleRegistered LifecycleState = "registered"
    LifecycleActive     LifecycleState = "active"
    LifecycleDegraded   LifecycleState = "degraded"
    LifecycleOffline    LifecycleState = "offline"
    LifecycleDeprecated LifecycleState = "deprecated"
)

type Health struct {
    State              LifecycleState
    LastProbedAt       *time.Time
    LastSuccessAt      *time.Time
    LastError          string        // last non-empty error message (truncated to 512 chars)
    LatencyMs          int64         // latency of the last successful probe
    ConsecutiveFailures int
}
```

The existing `Validity` struct (with `From`, `To`, `LastSeen`, `IsActiveAt()`) stays untouched. `Health.LastSuccessAt` mirrors into `Validity.LastSeen` on successful probe so `IsActiveAt()` keeps working — but the source of truth for "is it up right now" is `Health.State`.

---

## Storage layer

### Migration

Create a new versioned migration: `internal/store/migrations/0NN_health_state.sql` (use the next free sequence number — check existing migrations folder).

```sql
-- +migrate Up
ALTER TABLE catalog_entries ADD COLUMN health_state TEXT NOT NULL DEFAULT 'registered';
ALTER TABLE catalog_entries ADD COLUMN health_last_probed_at TIMESTAMP NULL;
ALTER TABLE catalog_entries ADD COLUMN health_last_success_at TIMESTAMP NULL;
ALTER TABLE catalog_entries ADD COLUMN health_last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE catalog_entries ADD COLUMN health_latency_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE catalog_entries ADD COLUMN health_consecutive_failures INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_catalog_entries_health_state ON catalog_entries(health_state);
CREATE INDEX idx_catalog_entries_health_last_probed_at ON catalog_entries(health_last_probed_at);

-- +migrate Down
DROP INDEX IF EXISTS idx_catalog_entries_health_last_probed_at;
DROP INDEX IF EXISTS idx_catalog_entries_health_state;
ALTER TABLE catalog_entries DROP COLUMN health_consecutive_failures;
ALTER TABLE catalog_entries DROP COLUMN health_latency_ms;
ALTER TABLE catalog_entries DROP COLUMN health_last_error;
ALTER TABLE catalog_entries DROP COLUMN health_last_success_at;
ALTER TABLE catalog_entries DROP COLUMN health_last_probed_at;
ALTER TABLE catalog_entries DROP COLUMN health_state;
```

**Both dialects** must compile and run cleanly. PostgreSQL accepts the same DDL above; if anything diverges (e.g. `BOOLEAN` vs `INTEGER` truthiness), gate it via the existing dialect helper used by the project.

### `Store` interface additions

```go
// internal/store/store.go
type Store interface {
    // ...existing...
    UpdateHealth(ctx context.Context, entryID string, h model.Health) error
    ListForProbing(ctx context.Context, olderThan time.Time, limit int) ([]model.CatalogEntry, error)
    SetLifecycle(ctx context.Context, entryID string, state model.LifecycleState) error
}
```

`ListForProbing` returns entries whose `health_last_probed_at IS NULL OR health_last_probed_at < olderThan` AND `health_state != 'deprecated'`, ordered by `health_last_probed_at NULLS FIRST`, capped by `limit`. This gives the worker a fair, deterministic batch.

Implement for both SQLite and PostgreSQL backends.

---

## Health probe worker

Create `internal/service/health/prober.go`:

```go
type Prober struct {
    store          store.Store
    httpClient     *http.Client
    interval       time.Duration   // default 30s
    timeout        time.Duration   // default 5s
    concurrency    int             // default 8
    degradedLatency time.Duration  // default 1500ms
    failureThreshold int           // default 3
    logger         *slog.Logger
}

func (p *Prober) Run(ctx context.Context) error
func (p *Prober) ProbeOnce(ctx context.Context, entry model.CatalogEntry) (model.Health, error)
```

### Probe algorithm (per entry)

1. Resolve probe URL:
   - For A2A entries: `supportedInterfaces[0].url` if present, otherwise legacy `url` field
   - For MCP entries: the entry's primary endpoint (existing field)
   - If no URL → set state to `offline`, error `"no probeable endpoint"`, do not perform HTTP
2. `GET <probeURL>` with `p.timeout`. Respect `https_skip_verify` if env says so (existing config).
3. Measure wall-clock latency.
4. Decide new state from `(httpResult, latency, currentHealth)`:
   - HTTP error / non-2xx → `failures++`. If `failures >= failureThreshold` → `offline`. Else → keep `degraded`.
   - 2xx and `latency > degradedLatency` → `degraded`, `failures = 0`.
   - 2xx and `latency <= degradedLatency` → `active`, `failures = 0`.
5. Always update `LastProbedAt = now`. On success update `LastSuccessAt = now` and write through to `Validity.LastSeen`.
6. Persist via `store.UpdateHealth`.

### Worker loop

- Tick every `interval`. On each tick fetch `ListForProbing(now - interval, concurrency * 4)`.
- Run probes in a worker pool with `concurrency` goroutines.
- Worker loop must respect `ctx.Done()` for clean shutdown (the microkernel `Stop()` lifecycle).
- Register the prober as a microkernel plugin in `internal/plugins/health/plugin.go` so it follows `Register → Init → Start → Stop`.

### Configuration

Reuse existing config struct `internal/config/config.go`. Add:

```go
type HealthConfig struct {
    Enabled          bool          `yaml:"enabled"          env:"AGENTLENS_HEALTH_ENABLED"           default:"true"`
    Interval         time.Duration `yaml:"interval"         env:"AGENTLENS_HEALTH_INTERVAL"          default:"30s"`
    Timeout          time.Duration `yaml:"timeout"          env:"AGENTLENS_HEALTH_TIMEOUT"           default:"5s"`
    Concurrency      int           `yaml:"concurrency"      env:"AGENTLENS_HEALTH_CONCURRENCY"       default:"8"`
    DegradedLatency  time.Duration `yaml:"degradedLatency"  env:"AGENTLENS_HEALTH_DEGRADED_LATENCY"  default:"1500ms"`
    FailureThreshold int           `yaml:"failureThreshold" env:"AGENTLENS_HEALTH_FAILURE_THRESHOLD" default:"3"`
}
```

---

## REST API

### `GET /api/v1/catalog` and `GET /api/v1/catalog/{id}`

Extend the response DTO (do **not** leak the storage struct directly):

```json
{
  "id": "...",
  "displayName": "...",
  "categories": ["A2A"],
  "health": {
    "state": "active",
    "lastProbedAt": "2026-04-07T11:42:13Z",
    "lastSuccessAt": "2026-04-07T11:42:13Z",
    "latencyMs": 142,
    "consecutiveFailures": 0,
    "lastError": ""
  }
}
```

For backward compatibility also expose a flat `status` field (string, same value as `health.state`) — the feature list explicitly requires this.

### `PATCH /api/v1/catalog/{id}/lifecycle`

Body: `{ "state": "deprecated" }`.

- Allowed values: `deprecated`, `active` (un-deprecate).
- Permission: `editor` or `admin` (existing RBAC). `viewer` → 403.
- Returns updated entry DTO.
- Audit log entry written via existing audit hook.

### `POST /api/v1/catalog/{id}/probe`

Triggers an immediate single-shot probe for one entry, bypasses the worker queue.

- Permission: `editor` or `admin`.
- Returns the resulting `health` object.
- Rate limit: max 1 call per entry per 5s (in-memory token bucket is fine).

### Filtering

`GET /api/v1/catalog?state=active,degraded` — comma-separated allow-list. Unknown states → 400.

---

## Frontend (React 18 + Tailwind + shadcn/ui)

> **Hard rule:** raw HTML elements styled with Tailwind are not acceptable for new components. Use shadcn/ui primitives (`Badge`, `Button`, `Tooltip`, `DropdownMenu`, `Alert`). See Known Traps.

### Catalog list view (`web/src/routes/catalog/list.tsx`)

Add a column **Status**, between Provider and Categories.

Use a shadcn `Badge` with semantic color via `variant`:

| State      | Variant                             | Color suggestion | Label        |
| ---------- | ----------------------------------- | ---------------- | ------------ |
| active     | `default` (or `success` if defined) | green            | "Active"     |
| degraded   | `warning`                           | amber            | "Degraded"   |
| offline    | `destructive`                       | red              | "Offline"    |
| registered | `secondary`                         | gray             | "Pending"    |
| deprecated | `outline`                           | slate            | "Deprecated" |

Next to the badge:

- Latency: small muted text `142 ms` if `state ∈ {active, degraded}`
- Last seen: relative time tooltip — `5s ago`, `2m ago`, `3h ago`. On hover show absolute UTC.

Add a filter bar above the table — multi-select shadcn `DropdownMenu` with the five states. Selected states are pushed to the URL as `?state=...` and used in the API query.

### Detail drawer / view

New section **"Health"** with a small grid:

```
State            Active
Last probed      2s ago  (2026-04-07 11:42:15 UTC)
Last successful  2s ago
Latency          142 ms
Failures (run)   0
Last error       —
```

Action buttons in the section header (admin/editor only — gate via existing `useCurrentUser().permissions`):

- **Probe now** → `POST /api/v1/catalog/{id}/probe`, optimistic loading state, show toast on result
- **Deprecate** / **Un-deprecate** → `PATCH /api/v1/catalog/{id}/lifecycle`, confirmation dialog before deprecating

### Empty / loading states

- Loading: skeleton bar in the badge column
- No probe yet: badge "Pending" with tooltip "Will be probed within next interval"
- All filter values exclude every entry: empty state message "No entries match the selected status filter — clear filters" with a Clear button

### Localization

The frontend uses no i18n yet. Keep all strings in English as in the rest of the codebase. Do not introduce a translation library.

---

## Tests

### Backend

- **Unit:** `prober_test.go` covering all state-transition cases. Use `httptest.Server` for the probed agent. Test cases (minimum):
  - Fresh entry → 200 fast → `active`
  - Active → 200 slow → `degraded`
  - Active → 500 once → `degraded`, failures=1
  - Degraded → 500 twice more → `offline`, failures=3
  - Offline → 200 fast → `active`, failures=0
  - Deprecated entry passed in → returns early, no HTTP call (use a recording transport that fails the test if invoked)
  - No URL → `offline`, no HTTP call
  - Probe times out → counted as failure
- **Store:** for both SQLite and PostgreSQL backends, test `UpdateHealth` and `ListForProbing` — including the `NULLS FIRST` ordering and the `deprecated` exclusion. Use existing `storetest` harness if it exists; otherwise write a small one.
- **API:** handler tests for `PATCH /lifecycle`, `POST /probe`, the new `state` filter, and 403 for viewers.

### E2E (Playwright)

Extend an existing spec (do not create a new file just for this):

1. Seed a catalog entry pointing to a stub server controlled by the test.
2. Wait for the badge to flip from "Pending" → "Active".
3. Stop the stub server, wait for the badge to flip to "Offline" (use a short interval override via env in the test runner).
4. Click "Probe now" while still offline → toast appears with the failure reason.
5. Click "Deprecate" → confirm dialog → badge becomes "Deprecated", probe worker stops touching the entry (verify by checking that `lastProbedAt` does not advance over the next interval).

---

## Acceptance criteria

A reviewer must be able to verify each of these without reading source code.

1. ✅ Fresh `docker compose up` shows demo agents flipping from "Pending" to "Active" within 30s
2. ✅ `curl -s http://localhost:8080/api/v1/catalog | jq '.[0].health.state'` returns one of the 5 lifecycle values
3. ✅ Killing a demo agent flips it to "Degraded" within one interval and to "Offline" after 3 intervals
4. ✅ Bringing it back flips it to "Active" on the next probe
5. ✅ `PATCH /api/v1/catalog/{id}/lifecycle` with `{"state":"deprecated"}` works for admin, returns 403 for viewer
6. ✅ Deprecated entries are visibly distinct in the dashboard and are not probed
7. ✅ `?state=active,degraded` filter works in both API and dashboard URL
8. ✅ Both SQLite and PostgreSQL backends pass the same store-level test suite
9. ✅ Worker shuts down cleanly on `SIGTERM` (no goroutine leaks — verify with `goleak` if available)
10. ✅ All new strings on the dashboard come from shadcn/ui components, not raw `<div className="bg-red-500">`

---

## Known traps (read before writing code)

These are the recurring AI-coding-agent mistakes in this codebase. **Do not commit any code that violates them.**

1. ❌ **Do not rename `CatalogEntry` to `Agent`.** This rename has been undone three times. The catalog stores A2A agents, MCP servers, and (future) A2UI surfaces — `Agent` is wrong for two of those three.
2. ❌ **Do not add a top-level `LastSeen` field to `CatalogEntry`.** It already lives inside `Validity`. The new `Health.LastSuccessAt` mirrors into `Validity.LastSeen` on success.
3. ❌ **Do not add `Tags`, `Team`, `Namespace` fields.** These were superseded by the archetype model (`categories`, `Provider`, `Metadata map[string]string`).
4. ❌ **Do not store JWTs / session tokens in `localStorage`.** Use the existing httpOnly cookie auth flow.
5. ❌ **Do not write raw `<button>`, `<input>`, `<table>` styled with Tailwind.** Use shadcn/ui primitives.
6. ❌ **Do not break the dual-dialect contract.** Every migration and every `Store` query must run on both SQLite and PostgreSQL. Run the full test suite against both before opening the PR.
7. ❌ **Do not gate this feature behind the enterprise license.** Health monitoring is OSS Core. The license-gated features remain SSO, RBAC federation, audit log federation, multi-region.
8. ❌ **Do not bypass the microkernel lifecycle.** The prober is a plugin: `Register → Init → Start → Stop`. Do not start a goroutine from `init()` or from `main`.
9. ❌ **Do not block the request handler on a probe.** The `POST /probe` endpoint runs the probe synchronously but with the `p.timeout` ceiling, returning a structured error if exceeded — never an open-ended wait.
10. ❌ **Do not invent a new `pkg/healthcheck` package outside `internal/`.** The project keeps everything under `internal/` for now to avoid premature API commitments.

---

## Execution order (suggested commit sequence)

1. Migration + model fields (compiles, no behavior)
2. `Store` interface methods + SQLite impl + PostgreSQL impl + store tests
3. Prober service + unit tests (no plugin wiring yet)
4. Microkernel plugin wiring + config + integration into `cmd/agentlens`
5. REST API: response DTO extension + `state` filter
6. REST API: `PATCH /lifecycle` + `POST /probe` + audit hooks
7. Frontend: badge + column + filter
8. Frontend: detail view "Health" section + actions
9. E2E test extension
10. README snippet update — one paragraph on health monitoring under Features

Each step should be a separate commit so the PR is reviewable.

---

## Out-of-band notes

- `degraded` is intentionally a "soft" state that flips on a single failure. This gives operators a fast warning signal. `offline` requires `failureThreshold` failures to avoid badge flapping during transient network blips.
- The `failureThreshold` default of 3 with a 30s interval means an entry takes ~90s to be marked offline. This is intentionally conservative — the launch story is "trustworthy state", not "instant alarm".
- `lastError` is truncated to 512 chars to keep the row small in SQLite. Full error details are not persisted on purpose; that is observability's job, not the registry's.
- Future Tier 2 work will add OpenTelemetry spans for each probe (feature 2.1) — keep the prober's hot path easy to instrument by passing `ctx` through everything.
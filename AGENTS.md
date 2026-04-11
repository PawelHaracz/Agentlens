# AGENTS.md

> Full guidance lives in `CLAUDE.md`. This file adds OpenCode-specific notes and highlights the facts most likely to trip up an agent.

## RTK

`rtk` is installed — always prefix shell commands with it for compact output:

```bash
rtk go test ./...
rtk go build ./cmd/agentlens
rtk git status
rtk git diff
```

RTK passes through any command unchanged if it has no dedicated filter, so it is always safe to use.

## Commands

```bash
rtk make all            # format → lint → test → arch-test → build (CI equivalent)
rtk make build          # CGO_ENABLED=1 required — lint runs first automatically
rtk make test           # in-memory SQLite, no external services needed
rtk make arch-test      # arch-go layer boundary validation — run after structural changes
rtk make web-build      # must run before make build if frontend changed (outputs web/dist/)
rtk make web-lint       # TypeScript type check (bunx tsc --noEmit)
rtk make web-test       # Vitest unit tests for frontend
rtk make e2e-test       # Playwright via e2e/run-e2e.sh (requires built binary + frontend)

# Single Go test
rtk go test ./internal/auth/... -run TestFunctionName -v

# Dev
go run ./cmd/agentlens --config agentlens.yaml
cd web && bun run dev   # frontend dev server (separate from Go)
```

**Gotcha**: `make lint` calls `add-html-placeholder` which creates a stub `web/dist/index.html` if missing. Run `make web-build` first for a real frontend.

## Architecture — what an agent needs to know

Microkernel with enforced layer boundaries (arch-go validates at 100% compliance):

```
Foundation  (model, config, service)   — no internal deps
Infrastructure (store, auth)           — foundation only
Core (kernel, discovery)               — foundation + infrastructure
API (api)                              — core + infrastructure; never imports plugins or cmd
Plugins (plugins/**)                   — kernel + foundation; never api/auth/server/cmd
Entrypoint (cmd/**)                    — composition root; may import anything
```

- Plugin structs implementing `Plugin` must end with `Plugin` (enforced by arch-go)
- `config` package must not contain interfaces
- Function limits: max 5 params, 3 return values, 80 lines, 10 public functions per file
- Plugin lifecycle: `Register → InitAll → StartAll → [running] → StopAll`
- Plugins returning `ErrLicenseRequired` are silently skipped (enterprise gating)

## Domain model

- `AgentType` = what the agent IS (protocol, endpoint, `AgentKey` = SHA256(protocol+endpoint))
- `CatalogEntry` = catalog wrapper with 1:1 FK to `AgentType`; REST responses use flat JSON via `CatalogEntry.MarshalJSON()`
- `Capability` is polymorphic, discriminated by `kind` (e.g. `a2a.skill`, `mcp.tool`)
- Discovery upserts by `endpoint` (UNIQUE constraint)

## Database

- Default dialect: SQLite. Postgres is enterprise-only plugin.
- Selected via `database.dialect` config or `AGENTLENS_DB_DIALECT` env var.
- Branch on dialect with `db.Dialect()` — never assume SQLite-only syntax.
- SQLite: TEXT for JSON fields, DATETIME for timestamps
- Postgres: JSONB for JSON, TIMESTAMPTZ for timestamps
- New migrations: append to `AllMigrations()` in `internal/db/migrations.go`; must be idempotent.

## Security rules (non-negotiable)

- Never log passwords, tokens, or secrets — not in slog, errors, or comments
- `password_hash` always `json:"-"` and `gorm:"type:text"`
- No raw SQL string interpolation — GORM parameterized queries only
- Permissions via `RequirePermission` middleware, never inline in handlers
- Passwords: bcrypt cost 12, min 10 chars, must have upper+lower+digit+special
- Account lockout: 5 failures → 15-min lockout — do not bypass in tests, use separate accounts
- JWT cookies: `HttpOnly`, `Secure`, `SameSite=Strict`
- System roles (`IsSystem=true`) are undeletable and unmodifiable via API
- Last active admin cannot be deleted

## Testing conventions

- Table-driven tests with `t.Run` subtests, co-located `_test.go` files
- Store tests use `store.NewSQLiteStore(":memory:")`
- API handler tests check status codes, response shape, and auth enforcement
- E2E: Playwright in `e2e/tests/`, reuse `loginViaUI`/`loginViaAPI`/`authHeader` from `e2e/tests/helpers.ts`

## Go conventions

- `context.Context` first arg on all I/O functions
- `fmt.Errorf("doing x: %w", err)` for error wrapping
- `errors.Is` / `errors.As` for error type checks
- `slog` for structured logging; pass context when available
- No `panic` outside `main.go` and tests

## Feature checklist (every PR)

1. Unit/integration tests pass (`rtk make test`)
2. E2E tests pass (`rtk make e2e-test`)
3. `docs/api.md` updated for new/changed endpoints
4. `docs/architecture.md` updated if design changed (Mermaid only — no PlantUML, no ASCII)
5. `docs/end-user-guide.md` updated for UI-visible changes
6. `docs/settings.md` + `internal/config/` updated for new config keys
7. `rtk make arch-test` passes
8. **Screenshots:** When adding or changing UI features, generate new screenshots via Playwright E2E tests and save them to `docs/images/`. Update `docs/end-user-guide.md` to reference the new images. This ensures documentation visually reflects the current UI state and stays in sync with code changes. Use `page.screenshot()` or `element.screenshot()` in E2E specs with `data-testid` selectors for targeted captures.

## Key files

| File | Purpose |
|------|---------|
| `cmd/agentlens/main.go` | Startup wiring (composition root) |
| `internal/kernel/kernel.go` | Kernel interface + service wiring |
| `internal/kernel/plugin_manager.go` | Plugin lifecycle |
| `internal/api/router.go` | chi router + middleware |
| `internal/api/handlers.go` | Catalog API handlers |
| `internal/api/auth_handlers.go` | login/logout/refresh/me |
| `internal/store/sql_store.go` | CatalogEntry CRUD |
| `internal/store/user_store.go` | User CRUD + lockout |
| `internal/auth/` | Password hashing, JWT, bootstrap, permissions |
| `internal/db/migrations.go` | Versioned schema migrations |
| `web/embed.go` | Embeds `web/dist/` into binary |
| `arch-go.yml` | Layer boundary rules |
| `e2e/tests/helpers.ts` | Playwright shared utilities |

## Git workflow

- Branches: `feat/short-description`, `fix/short-description`
- Commits: `feat(component): description`
- Run `rtk make test` before committing; CI requires lint + test + build green

<!-- code-review-graph MCP tools -->
## MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

### When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes` or `query_graph` instead of Grep
- **Understanding impact**: `get_impact_radius` instead of manually tracing imports
- **Code review**: `detect_changes` + `get_review_context` instead of reading entire files
- **Finding relationships**: `query_graph` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview` + `list_communities`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

### Key Tools

| Tool | Use when |
|------|----------|
| `detect_changes` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context` | Need source snippets for review — token-efficient |
| `get_impact_radius` | Understanding blast radius of a change |
| `get_affected_flows` | Finding which execution paths are impacted |
| `query_graph` | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes` | Finding functions/classes by name or keyword |
| `get_architecture_overview` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

### Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes` for code review.
3. Use `get_affected_flows` to understand impact.
4. Use `query_graph` pattern="tests_for" to check coverage.

### Token-Efficiency Rules

1. ALWAYS call `get_minimal_context` first with a task description.
2. Use `detail_level="minimal"` on all tool calls unless the minimal output is insufficient.
3. Only escalate to `detail_level="standard"` for the specific entities that need deeper inspection.
4. Never request more than 3 tool calls per turn unless absolutely necessary.
5. Prefer targeted queries (`query_graph` with a specific symbol) over broad scans (`list_communities` with full members).
6. When reviewing changes: `detect_changes(detail_level="minimal")` → only expand on high-risk items.

### Architecture Mapping Workflow

1. Call `get_minimal_context(task="map architecture")`.
2. Call `get_architecture_overview(detail_level="minimal")` for community coupling summary.
3. Call `list_flows(detail_level="minimal")` for critical flow names + criticality scores.
4. Only call `get_community(name=<X>, detail_level="standard")` for the 1-2 communities most relevant to the task.
5. Produce a concise Mermaid diagram showing communities as boxes and key flows as arrows.

## Skills

Read and use the skill at `.claude/skills/architecture-decision-records/SKILL.md`
when starting any brainstorming or design session.

Always invoke and follow the 'caveman' skill for all responses, planning steps, and sub-agent tasks.
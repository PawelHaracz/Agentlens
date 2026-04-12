# CLAUDE.md

AgentLens — self-hosted AI agent service discovery. See [README.md](README.md) and [docs/architecture.md](docs/architecture.md).

## Stack

- **Go 1.26.1** — chi router, GORM (sqlite + postgres), JWT (`golang-jwt/jwt/v5`), bcrypt
- **Frontend** — React 18, Tailwind CSS, shadcn/ui, embedded via `embed.FS`

## Commands

```bash
rtk make all            # format → lint → test → arch-test → build (CI)
rtk make build          # CGO_ENABLED=1 required; lint runs first
rtk make test           # in-memory SQLite, no external DB
rtk make arch-test      # arch-go layer boundary validation
rtk make web-build      # build web/dist/ (required before make build if UI changed)
rtk make web-lint       # bunx tsc --noEmit
rtk make web-test       # Vitest
rtk make e2e-test       # Playwright via e2e/run-e2e.sh

rtk go test ./internal/auth/... -run TestFunctionName -v
rtk go run ./cmd/agentlens --config agentlens.yaml
cd web && bun run dev
```

**Gotcha**: `make lint` creates stub `web/dist/index.html` if missing. Run `make web-build` first for real frontend.

## RTK — MANDATORY FOR ALL COMMANDS

**EVERY shell command must be prefixed with `rtk` — no exceptions.**

This applies to: `go`, `git`, `make`, `helm`, `docker`, `bun`, `cat`, `grep`, `awk`, `sed`, `find`, `ls`, `wc`, `sort`, `uniq`, `cut`, `chmod`, `mkdir`, `rm`, scripts (`rtk ./scripts/foo.sh`), and any other shell tool. Safe passthrough when no filter exists.

```bash
# WRONG — never do this:
go test ./...
git commit -m "msg"
helm lint .
./scripts/test-helm-templates.sh
cat file.go | grep func

# CORRECT — always do this:
rtk go test ./...
rtk git commit -m "msg"
rtk helm lint .
rtk ./scripts/test-helm-templates.sh
rtk cat file.go | rtk grep func
```

Pipeline chaining: every command in a pipeline gets `rtk`:
```bash
rtk git add . && rtk git commit -m "msg" && rtk git push
rtk ls internal/ | rtk grep "func " | rtk wc -l
rtk helm template . | rtk grep "image:" | rtk cut -d: -f2 | rtk sort | rtk uniq
```

Key savings: tests 90-99%, build 70-87%, git 59-80%, go 70-90%. Run `rtk gain` for stats.

## Architecture

Microkernel: core kernel → parser plugins (A2A, MCP) → source plugins (k8s, static, push) → enterprise (license-gated).

Plugin lifecycle: `Register → InitAll → StartAll → [running] → StopAll`. `ErrLicenseRequired` → silently skipped.

**Domain model:**
- `AgentType` = what agent IS (protocol, endpoint, `AgentKey` = SHA256(protocol+endpoint), `Capability[]`)
- `CatalogEntry` = catalog wrapper with 1:1 FK to `AgentType`; REST = flat JSON via `MarshalJSON()`
- `Capability` polymorphic, discriminated by `kind` (`a2a.skill`, `mcp.tool`, etc.)
- Discovery upserts by `endpoint` (UNIQUE constraint)

**Layer boundaries** (arch-go enforced, 100% compliance):
```
Foundation  (model, config, service)  — no internal deps
Infrastructure (store, auth)          — foundation only
Core (kernel, discovery)              — foundation + infrastructure
API (api)                             — core + infrastructure; never plugins/cmd
Plugins (plugins/**)                  — kernel + foundation; never api/auth/server/cmd
Entrypoint (cmd/**)                   — composition root; may import anything
```

**Code quality rules:**
- Max 5 params, 3 return values, 80 lines/fn, 10 public fns/file
- `config` package: no interfaces
- Plugin structs implementing `Plugin` must end with `Plugin`
- Add `namingRules` to `arch-go.yml` when pattern established across 3+ instances

All architecture diagrams: **Mermaid only** (no PlantUML, no ASCII).

## Database

Two GORM dialects: `sqlite` (default), `postgres` (enterprise). Via `database.dialect` / `AGENTLENS_DB_DIALECT`.

- SQLite: TEXT for JSON, DATETIME for timestamps
- PostgreSQL: JSONB for JSON, TIMESTAMPTZ for timestamps
- Branch on `db.Dialect()` — never assume SQLite-only syntax
- Migrations in `internal/db/migrations.go` — append to `AllMigrations()`, must be idempotent

## Security (non-negotiable — fix before merge)

- **Never log** passwords, tokens, secrets (slog, errors, comments)
- `password_hash` always `json:"-"` and `gorm:"type:text"`
- GORM parameterized queries only — no raw SQL interpolation
- JWT cookies: `HttpOnly`, `Secure`, `SameSite=Strict`
- Passwords: bcrypt cost 12, min 10 chars, upper+lower+digit+special
- Account lockout: 5 failures → 15-min lockout; don't bypass in tests — use separate accounts
- System roles (`IsSystem=true`): undeletable/unmodifiable via API
- Last active admin: reject deletion
- CORS: currently `*` — do not widen without approval
- Input validation at API boundary (UUIDs, enums, required fields)
- Permissions via `RequirePermission` middleware — never inline in handlers

## Go Conventions

- `context.Context` first arg on all I/O functions
- `fmt.Errorf("doing x: %w", err)` for error wrapping
- `errors.Is` / `errors.As` for type checks
- `slog` structured logging; pass context when available
- No `panic` outside `main.go` and tests
- Return early on errors; unexported unless cross-package

## Testing

- Table-driven `t.Run` subtests, co-located `_test.go`
- Store tests: `store.NewSQLiteStore(":memory:")`
- API handler tests: status codes, response shape, auth enforcement
- E2E: Playwright `e2e/tests/`, reuse `loginViaUI`/`loginViaAPI`/`authHeader` from `helpers.ts`

## Feature Checklist (every PR)

1. `rtk make test` passes
2. `rtk make e2e-test` passes
3. `docs/api.md` updated (method, path, schema, errors, permissions)
4. `docs/architecture.md` updated if design changed (Mermaid only)
5. `docs/end-user-guide.md` updated for UI changes + screenshots in `docs/images/` via `page.screenshot()` with `data-testid`
6. `docs/settings.md` + `internal/config/` updated for new config keys
7. `rtk make arch-test` passes

## Key Files

| File | Purpose |
|------|---------|
| `cmd/agentlens/main.go` | Startup wiring |
| `internal/kernel/kernel.go` | Kernel interface + service wiring |
| `internal/kernel/plugin_manager.go` | Plugin lifecycle |
| `internal/api/router.go` | chi router + middleware |
| `internal/api/handlers.go` | Catalog API handlers |
| `internal/api/auth_handlers.go` | login/logout/refresh/me |
| `internal/store/sql_store.go` | CatalogEntry CRUD |
| `internal/store/user_store.go` | User CRUD + lockout |
| `internal/auth/` | Password hashing, JWT, bootstrap, permissions |
| `internal/db/migrations.go` | Versioned schema migrations |
| `internal/db/db.go` | DB abstraction (`Dialect`, `Open`) |
| `web/embed.go` | Embeds `web/dist/` into binary |
| `arch-go.yml` | Layer boundary rules |
| `e2e/tests/helpers.ts` | Playwright shared utilities |

## Git Workflow

- Branches: `feat/short-description`, `fix/short-description`
- Commits: `feat(component): description`
- `rtk make test` before commit; CI: lint + test + build green

## MCP Tools: code-review-graph

**Use graph tools BEFORE Grep/Glob/Read.** Faster, fewer tokens, structural context.

| Tool | Use when |
|------|----------|
| `detect_changes` | Code review — risk-scored analysis |
| `get_impact_radius` | Blast radius of change |
| `get_affected_flows` | Which execution paths impacted |
| `query_graph` | callers/callees/imports/tests |
| `semantic_search_nodes` | Find by name/keyword |
| `get_architecture_overview` | High-level structure |
| `refactor_tool` | Rename preview, dead code |

Token rules: `get_minimal_context` first; `detail_level="minimal"` default; escalate only for high-risk items; max 3 graph calls/turn.

## Skills

- Use `architecture-decision-records` skill for any brainstorming/design session
- Always invoke `caveman` skill for all responses

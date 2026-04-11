# AGENTS.md

> Full guidance in `CLAUDE.md`. This file: OpenCode-specific notes + agent gotchas.

## RTK

Always prefix commands with `rtk`. Safe passthrough if no filter exists.

```bash
rtk make all            # CI equivalent: format → lint → test → arch-test → build
rtk make build          # CGO_ENABLED=1 required
rtk make test           # in-memory SQLite
rtk make arch-test      # after structural changes
rtk make web-build      # before make build if frontend changed
rtk make e2e-test       # requires built binary + frontend
rtk go test ./internal/auth/... -run TestFunctionName -v
```

**Gotcha**: `make lint` creates stub `web/dist/index.html`. Run `make web-build` first for real frontend.

## Architecture

Microkernel, arch-go enforced (100% compliance):

```
Foundation  (model, config, service)  — no internal deps
Infrastructure (store, auth)          — foundation only
Core (kernel, discovery)              — foundation + infrastructure
API (api)                             — core + infrastructure; never plugins/cmd
Plugins (plugins/**)                  — kernel + foundation; never api/auth/server/cmd
Entrypoint (cmd/**)                   — composition root
```

- Plugin structs → must end with `Plugin`
- `config` package → no interfaces
- Fn limits: max 5 params, 3 returns, 80 lines, 10 public fns/file
- Plugin lifecycle: `Register → InitAll → StartAll → [running] → StopAll`
- `ErrLicenseRequired` → silently skipped

## Domain model

- `AgentType` = protocol + endpoint + `AgentKey` (SHA256)
- `CatalogEntry` = catalog wrapper, 1:1 FK → `AgentType`; REST = flat JSON via `MarshalJSON()`
- `Capability` polymorphic, discriminated by `kind` (`a2a.skill`, `mcp.tool`)
- Discovery upserts by `endpoint` (UNIQUE)

## Database

- SQLite default; Postgres enterprise-only
- `database.dialect` config or `AGENTLENS_DB_DIALECT` env
- Branch on `db.Dialect()` — never SQLite-only syntax
- New migrations: append to `AllMigrations()`, must be idempotent

## Security (non-negotiable)

- Never log passwords/tokens/secrets
- `password_hash`: `json:"-"`, `gorm:"type:text"`
- GORM parameterized queries only
- Permissions via `RequirePermission` middleware
- Passwords: bcrypt cost 12, min 10 chars, upper+lower+digit+special
- Lockout: 5 failures → 15-min; don't bypass in tests
- JWT: `HttpOnly`, `Secure`, `SameSite=Strict`
- System roles: undeletable/unmodifiable; last admin: reject deletion

## Testing

- Table-driven `t.Run`, co-located `_test.go`
- Store: `store.NewSQLiteStore(":memory:")`
- API: status codes, response shape, auth enforcement
- E2E: Playwright `e2e/tests/`, reuse helpers from `e2e/tests/helpers.ts`

## Go conventions

- `context.Context` first arg
- `fmt.Errorf("doing x: %w", err)`
- `errors.Is` / `errors.As`
- `slog`; pass context when available
- No `panic` outside `main.go` and tests

## Feature checklist (every PR)

1. `rtk make test` passes
2. `rtk make e2e-test` passes
3. `docs/api.md` updated for new/changed endpoints
4. `docs/architecture.md` updated if design changed (Mermaid only)
5. `docs/end-user-guide.md` updated for UI changes
6. `docs/settings.md` + `internal/config/` updated for new config keys
7. `rtk make arch-test` passes
8. Screenshots via `page.screenshot()` with `data-testid` → `docs/images/`

## Key files

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
| `web/embed.go` | Embeds `web/dist/` into binary |
| `arch-go.yml` | Layer boundary rules |
| `e2e/tests/helpers.ts` | Playwright shared utilities |

## Git workflow

- Branches: `feat/short-description`, `fix/short-description`
- Commits: `feat(component): description`
- `rtk make test` before commit; CI: lint + test + build green

## MCP Tools: code-review-graph

Use graph tools **before** Grep/Glob/Read.

| Tool | Use when |
|------|----------|
| `detect_changes` | Code review |
| `get_impact_radius` | Blast radius |
| `get_affected_flows` | Execution paths |
| `query_graph` | callers/callees/imports/tests |
| `semantic_search_nodes` | Find by name/keyword |
| `get_architecture_overview` | High-level structure |

Token rules: `get_minimal_context` first; `detail_level="minimal"` default; max 3 calls/turn.

## Skills

- Use `architecture-decision-records` skill for brainstorming/design
- Always invoke `caveman` skill for all responses

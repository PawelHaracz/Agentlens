# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

AgentLens is a self-hosted service discovery platform for AI agents. See [README.md](README.md) and [docs/architecture.md](docs/architecture.md).

## Stack

- **Go 1.26.1** — chi router, GORM (sqlite + postgres drivers), JWT (`golang-jwt/jwt/v5`), bcrypt (`golang.org/x/crypto`)
- **Frontend** — React 18, Tailwind CSS, shadcn/ui, embedded in the binary via `embed.FS`

## Commands

```bash
make build          # Compile Go binary (CGO_ENABLED=1 required for SQLite)
make test           # Run Go tests (in-memory SQLite, no external DB needed)
make lint           # golangci-lint
make format         # gofmt + go fmt
make arch-test      # Run arch-go architecture rules validation
make all            # format → lint → test → arch-test → build

# Single test
go test ./internal/auth/... -run TestFunctionName -v

# Frontend (uses bun)
cd web && bun run dev     # Dev server
make web-build            # Build to web/dist/ (required before make build if UI changed)
make web-lint             # TypeScript type check

# Docker / Helm
make docker-build         # agentlens:local
make helm-lint
make e2e-test             # Playwright (via e2e/run-e2e.sh)

# Run locally
go run ./cmd/agentlens --config agentlens.yaml
```

## Architecture

Microkernel: core kernel → parser plugins (A2A, MCP) → source plugins (k8s, static, push) → enterprise plugins (license-gated). See [docs/architecture.md](docs/architecture.md).

Plugin lifecycle: `Register → InitAll → StartAll → [running] → StopAll`. Plugins returning `ErrLicenseRequired` during init are silently skipped.

Domain model (Product Archetype): `AgentType` represents what the agent IS (protocol, endpoint, version, `AgentKey` = SHA256(protocol+endpoint), `Capability[]`, `Provider`, raw definition). `CatalogEntry` is the catalog wrapper (display name, status, source, validity, categories, metadata) with a 1:1 FK to `AgentType`. `Capability` is polymorphic, discriminated by `kind` (`a2a.skill`, `a2a.interface`, `a2a.security_scheme`, `a2a.extension`, `a2a.signature`, `mcp.tool`, `mcp.resource`, `mcp.prompt`). REST responses are backward-compatible flat JSON (via `CatalogEntry.MarshalJSON()`).

All architecture diagrams must be written in **Mermaid** (not PlantUML, not ASCII art).

## Database

Two GORM dialects always compiled: `sqlite` (default) and `postgres` (enterprise plugin). Selected via `database.dialect` config / `AGENTLENS_DB_DIALECT`.

- SQLite: TEXT for JSON fields, DATETIME for timestamps
- PostgreSQL: JSONB for JSON fields, TIMESTAMPTZ for timestamps
- Use `db.Dialect()` to branch on dialect; GORM handles query placeholders automatically
- Migrations in `internal/db/migrations.go`, versioned, auto-run on startup
- `endpoint` column has a UNIQUE constraint — discovery upserts by endpoint
- New migrations: add to `AllMigrations()` slice, design for idempotency (FirstOrCreate, ON CONFLICT DO NOTHING)

## Security by Design

These rules are non-negotiable — violations must be fixed before merge:

- **NEVER log passwords, tokens, or secrets** — not in slog, errors, or debug output
- `password_hash` fields always tagged `json:"-"` and `gorm:"type:text"`
- All database queries go through GORM parameterized statements — no raw string interpolation in SQL
- JWT cookie attributes: `HttpOnly`, `Secure`, `SameSite=Strict`
- Passwords: bcrypt cost 12, minimum 10 chars, must contain uppercase + lowercase + digit + special
- Account lockout: 5 failed attempts → 15-minute lockout (do not bypass in tests — use separate test accounts)
- System roles (`IsSystem=true`) must not be deletable or modifiable by API
- Protect last admin: API must reject deletion of the last active admin user
- CORS: current config is permissive (`*`) — do not widen further without explicit approval
- Input validation at API boundary — reject malformed UUIDs, unknown enum values, empty required fields early
- Permissions checked via `RequirePermission` middleware, never inline in handlers

## Go Conventions

- `context.Context` as first arg for all I/O functions
- Wrap errors with context: `fmt.Errorf("doing x: %w", err)`
- No `panic` outside `main.go` and tests
- Return early on errors — avoid deep nesting
- Use `errors.Is` / `errors.As` for error type checks
- Prefer table-driven tests with subtests (`t.Run`), use real in-memory SQLite (`store.NewSQLiteStore(":memory:")`)
- Unexported types/functions unless they need to cross package boundaries
- Use `slog` for structured logging; always pass `context.Context` to logger when available

## Architecture Rules (arch-go)

The project uses [arch-go](https://github.com/arch-go/arch-go) to enforce microkernel layer boundaries at CI time. Rules are defined in `arch-go.yml` at the repo root.

### Layer boundaries enforced

- **Foundation** (`model`, `config`, `service`) — no internal dependencies
- **Infrastructure** (`store`, `auth`) — depends on foundation only
- **Core** (`kernel`, `discovery`) — depends on foundation + infrastructure
- **API** (`api`) — depends on core + infrastructure, never plugins or cmd
- **Plugins** (`plugins/**`) — depend on kernel + foundation, never api/auth/server/cmd
- **Entrypoint** (`cmd/**`) — composition root, may import anything

### Code quality rules

- Max 5 function parameters, 3 return values, 80 lines per function, 10 public functions per file
- `config` package must not contain interfaces
- Plugin structs implementing `Plugin` interface must end with `Plugin`

### Naming rules expansion policy

When a new naming convention emerges in the codebase (e.g. all stores end with `Store`, all handlers end with `Handler`), add a corresponding `namingRules` entry to `arch-go.yml` to enforce it. Keep naming rules minimal and only add them when a pattern is established across at least 3 instances.

### Running

```bash
make arch-test      # Run architecture validation
arch-go --html      # Generate HTML report in .arch-go/
```

## Feature Development Checklist

Every feature or bugfix must complete all of the following before it is considered done:

### 1. Unit & Integration Tests

- Write table-driven Go tests in `_test.go` files co-located with the package
- Test error paths and edge cases, not just the happy path
- For new store methods: test against in-memory SQLite
- For new API handlers: test status codes, response body shape, and auth enforcement
- Run `make test` — all tests must pass

### 2. E2E Tests

- Add or update Playwright scenarios in `e2e/tests/`
- Cover the full user flow through the UI (login → action → result)
- Reuse helpers from `e2e/tests/helpers.ts` (loginViaUI, loginViaAPI, authHeader)
- Run `make e2e-test` — all E2E tests must pass

### 3. API Documentation

- Update `docs/api.md` for any new or changed endpoints
- Include: HTTP method, path, request body schema, response schema, error codes
- Document permission requirements (`catalog:read`, `users:write`, etc.)

### 4. Architecture Documentation

- Update `docs/architecture.md` if the change affects system design, data flow, or component responsibilities
- All new diagrams must be written in **Mermaid**
- Example sequence diagram for a new flow:

  ```mermaid
  sequenceDiagram
      participant C as Client
      participant A as API
      participant K as Kernel
      C->>A: POST /api/v1/resource
      A->>K: kernel.Store().Create(ctx, entry)
      K-->>A: entry, nil
      A-->>C: 201 Created
  ```

### 5. End-User Guide

- Update `docs/end-user-guide.md` (and `docs/user-guide.md` if applicable) for any UI-visible change
- Include screenshots showing the feature from the user's perspective
- Screenshots: capture the relevant UI state, annotate if the interaction is non-obvious
- For new UI pages or dialogs: show the full flow (empty state → fill form → result)

### 6. Settings & Configuration

- If a new config key is added: document it in `docs/settings.md` and add a default in `internal/config/`
- If a new setting is added to the settings store: include it in migration seeding

## Key Files

- `internal/db/db.go` — DB abstraction (`Dialect`, `Open`)
- `internal/db/migrations.go` — versioned schema migrations
- `internal/store/sql_store.go` — CatalogEntry CRUD
- `internal/store/user_store.go` — User CRUD + lockout logic
- `internal/auth/` — password hashing, JWT, bootstrap, permissions
- `internal/api/router.go` — chi router + middleware wiring
- `internal/api/handlers.go` — catalog API handlers
- `internal/api/auth_handlers.go` — auth endpoints (login/logout/refresh/me)
- `internal/kernel/kernel.go` — kernel interface and service wiring
- `internal/kernel/plugin_manager.go` — plugin lifecycle management
- `cmd/agentlens/main.go` — startup wiring
- `web/embed.go` — embeds `web/dist/` into the binary
- `e2e/tests/helpers.ts` — shared Playwright test utilities

## Git Workflow

- Branch naming: `feat/short-description`, `fix/short-description`
- Commit messages: `feat(component): description`
- Run `make test` before committing
- Open PRs against `main`; CI must be green (lint + test + build)

<!-- rtk-instructions v2 -->
# RTK (Rust Token Killer) - Token-Optimized Commands

## Golden Rule

**Always prefix commands with `rtk`**. If RTK has a dedicated filter, it uses it. If not, it passes through unchanged. This means RTK is always safe to use.

**Important**: Even in command chains with `&&`, use `rtk`:
```bash
# ❌ Wrong
git add . && git commit -m "msg" && git push

# ✅ Correct
rtk git add . && rtk git commit -m "msg" && rtk git push
```

## RTK Commands by Workflow

### Build & Compile (80-90% savings)
```bash
rtk cargo build         # Cargo build output
rtk cargo check         # Cargo check output
rtk cargo clippy        # Clippy warnings grouped by file (80%)
rtk tsc                 # TypeScript errors grouped by file/code (83%)
rtk lint                # ESLint/Biome violations grouped (84%)
rtk prettier --check    # Files needing format only (70%)
rtk next build          # Next.js build with route metrics (87%)
```

### Test (90-99% savings)
```bash
rtk cargo test          # Cargo test failures only (90%)
rtk vitest run          # Vitest failures only (99.5%)
rtk playwright test     # Playwright failures only (94%)
rtk test <cmd>          # Generic test wrapper - failures only
```

### Git (59-80% savings)
```bash
rtk git status          # Compact status
rtk git log             # Compact log (works with all git flags)
rtk git diff            # Compact diff (80%)
rtk git show            # Compact show (80%)
rtk git add             # Ultra-compact confirmations (59%)
rtk git commit          # Ultra-compact confirmations (59%)
rtk git push            # Ultra-compact confirmations
rtk git pull            # Ultra-compact confirmations
rtk git branch          # Compact branch list
rtk git fetch           # Compact fetch
rtk git stash           # Compact stash
rtk git worktree        # Compact worktree
```

Note: Git passthrough works for ALL subcommands, even those not explicitly listed.

### GitHub (26-87% savings)
```bash
rtk gh pr view <num>    # Compact PR view (87%)
rtk gh pr checks        # Compact PR checks (79%)
rtk gh run list         # Compact workflow runs (82%)
rtk gh issue list       # Compact issue list (80%)
rtk gh api              # Compact API responses (26%)
```

### JavaScript/TypeScript Tooling (70-90% savings)
```bash
rtk pnpm list           # Compact dependency tree (70%)
rtk pnpm outdated       # Compact outdated packages (80%)
rtk pnpm install        # Compact install output (90%)
rtk npm run <script>    # Compact npm script output
rtk npx <cmd>           # Compact npx command output
rtk prisma              # Prisma without ASCII art (88%)
```

### Files & Search (60-75% savings)
```bash
rtk ls <path>           # Tree format, compact (65%)
rtk read <file>         # Code reading with filtering (60%)
rtk grep <pattern>      # Search grouped by file (75%)
rtk find <pattern>      # Find grouped by directory (70%)
```

### Analysis & Debug (70-90% savings)
```bash
rtk err <cmd>           # Filter errors only from any command
rtk log <file>          # Deduplicated logs with counts
rtk json <file>         # JSON structure without values
rtk deps                # Dependency overview
rtk env                 # Environment variables compact
rtk summary <cmd>       # Smart summary of command output
rtk diff                # Ultra-compact diffs
```

### Infrastructure (85% savings)
```bash
rtk docker ps           # Compact container list
rtk docker images       # Compact image list
rtk docker logs <c>     # Deduplicated logs
rtk kubectl get         # Compact resource list
rtk kubectl logs        # Deduplicated pod logs
```

### Network (65-70% savings)
```bash
rtk curl <url>          # Compact HTTP responses (70%)
rtk wget <url>          # Compact download output (65%)
```

### Meta Commands
```bash
rtk gain                # View token savings statistics
rtk gain --history      # View command history with savings
rtk discover            # Analyze Claude Code sessions for missed RTK usage
rtk proxy <cmd>         # Run command without filtering (for debugging)
rtk init                # Add RTK instructions to CLAUDE.md
rtk init --global       # Add RTK to ~/.claude/CLAUDE.md
```

## Token Savings Overview

| Category | Commands | Typical Savings |
|----------|----------|-----------------|
| Tests | vitest, playwright, cargo test | 90-99% |
| Build | next, tsc, lint, prettier | 70-87% |
| Git | status, log, diff, add, commit | 59-80% |
| GitHub | gh pr, gh run, gh issue | 26-87% |
| Package Managers | pnpm, npm, npx | 70-90% |
| Files | ls, read, grep, find | 60-75% |
| Infrastructure | docker, kubectl | 85% |
| Network | curl, wget | 65-70% |

Overall average: **60-90% token reduction** on common development operations.
<!-- /rtk-instructions -->
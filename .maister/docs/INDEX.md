# Documentation Index

**IMPORTANT**: Read this file at the beginning of any development task to understand available documentation and standards.

This file is the master map of all project documentation. Documentation lives under `.maister/docs/`:

- **`project/`** — what this product is, where it is going, what it is built with
- **`standards/`** — technical standards and conventions the team enforces

---

## Project Documentation

Located in `.maister/docs/project/`. Start here when onboarding or planning work.

### Vision (`project/vision.md`)

AgentLens positioning as a self-hosted service discovery and catalog platform for AI agents: single source of truth for agent identity/capabilities/health across protocols (A2A, MCP), Product Archetype domain model (`AgentType` vs. `CatalogEntry`), microkernel + plugins architectural bet, and the four near-term priorities — OSS polish, enterprise features, observability maturity, plugin ecosystem.

### Roadmap (`project/roadmap.md`)

Current release state (Helm/app `0.2.0`), recent shipped work (OTel observability #22, security capability #18), and planned enhancements organized by the four priorities. High-priority items include quickstart-in-5-minutes, plugin author guide, plugin scaffold CLI, PostgreSQL CI parity, and reference Grafana dashboards. Technical debt tracks CORS allowlist, SPA code-splitting, bundle-size budget, and multi-dialect migration tests. Effort scale: S (2–3 days) / M (1 week) / L (2+ weeks).

### Tech Stack (`project/tech-stack.md`)

Go 1.26.1 backend (~85% of `internal/`) + TypeScript 5.5 / React 18 frontend embedded via `embed.FS`. Key frameworks: chi 5.1.0 router, GORM 1.31.1 with dual-dialect SQLite + PostgreSQL, `golang-jwt/jwt` v5, bcrypt cost 12, `k8s.io/client-go` 0.29.0, OpenTelemetry 1.43.0 (9 packages), Prometheus client. Frontend: TanStack React Query 5.96.2, Tailwind 3.4.10, shadcn/ui. Testing: `testing` + `testify`, SQLite `:memory:`, Vitest (80% coverage), Playwright, `arch-go` (100% layer compliance). Build: Bun + Vite 8, Make, golangci-lint v2.11.4, Lefthook.

### Architecture (`project/architecture.md`)

Microkernel + plugins design with Product Archetype domain model. Includes Mermaid diagram of source plugins (k8s/static/push) → parser plugins (A2A/MCP) → core (kernel/discovery/health) → REST API (chi + JWT/RBAC) → GORM store → SQLite/PostgreSQL, with embedded React SPA. Documents the six enforced layer boundaries (Foundation → Infrastructure → Core → API → Plugins → Entrypoint), code-quality rules (max 5 params, 80 lines/fn, `Plugin` suffix), kernel lifecycle (`Register → InitAll → StartAll → StopAll` with `ErrLicenseRequired` silent-skip), discovery/request data flows, configuration model, and deployment topology (single CGO-enabled Docker image, Helm chart `0.2.0`, `/healthz` + `/readyz` + `/metrics`).

---

## Technical Standards

Technical standards are grouped by category under `.maister/docs/standards/`. Each file enumerates named standards using `### Standard Name` headers so agents can locate, cite, and update individual rules.

### Global Standards

Located in `.maister/docs/standards/global/`. Language-agnostic conventions that apply across the entire codebase.

#### Coding Style (`standards/global/coding-style.md`)

Naming consistency, automatic formatting, descriptive names, focused functions, uniform indentation, no dead code, no backward compatibility unless required, and DRY.

#### Commenting (`standards/global/commenting.md`)

Let code speak through structure/naming, comment sparingly only when logic isn't self-evident, no change/changelog comments.

#### Development Conventions (`standards/global/conventions.md`)

Predictable structure, up-to-date documentation, clean version control, environment variables for secrets, minimal dependencies, consistent reviews, testing standards, feature flags, changelog updates, build only what's needed.

#### Error Handling (`standards/global/error-handling.md`)

Clear user messages without leaking internals, fail fast on invalid input, typed exceptions, centralized handling at boundaries, graceful degradation, retry with exponential backoff, and guaranteed resource cleanup.

#### Minimal Implementation (`standards/global/minimal-implementation.md`)

Build only what's called, clear purpose per method, delete exploration artifacts, no future stubs, no speculative abstractions, review before commit, unused code is debt.

#### PR Feature Checklist (`standards/global/pr-checklist.md`)

Seven-step PR gate (`make test`, `make e2e-test`, `docs/api.md`, `docs/architecture.md`, `docs/end-user-guide.md`, `docs/settings.md`, `make arch-test`). Documentation must match implementation exactly (casing, endpoint paths, config keys, defaults). New permissions must be documented across `docs/auth.md`, `docs/api.md`, and `README.md` plus covered by `RequirePermission` enforcement tests.

#### Validation (`standards/global/validation.md`)

Server-side validation always, client-side for feedback only, validate early, specific field errors, allowlists over blocklists, type/format checks, input sanitization against injection, business-rule validation, consistent enforcement across all entry points.

#### Development Workflow (`standards/global/workflow.md`)

Read `.maister/docs/INDEX.md` before any task. Prefer code-review-graph MCP tools (`semantic_search_nodes`, `query_graph`, `get_impact_radius`, `detect_changes`, `get_affected_flows`, `get_architecture_overview`, `refactor_tool`) over Grep/Glob/Read — max 3 graph calls per turn, `detail_level="minimal"` default. Execute `/maister:*` commands via the Skill tool immediately. PR title + type checkbox must match actual scope. CI gates (lint + test + build) must pass before commit.

---

### Architecture Standards

Located in `.maister/docs/standards/architecture/`. Architecture-level standards derived from ADRs and `arch-go` enforcement.

#### Domain Model (`standards/architecture/domain-model.md`)

Capabilities belong to `AgentType` (single `capabilities` table, union-struct pattern for sub-variants), computed view fields (`auth_summary`, `security_detail`) are not stored, full replacement on capability update (delete-all + insert-all), discovery upsert by `endpoint` with `AgentKey = SHA256(protocol + endpoint)`, and `SourcePush` entries protected from discovery overwrites. Sources: ADR-001, ADR-004, ADR-008.

#### Layering (`standards/architecture/layering.md`)

Strict layer boundaries enforced by arch-go at 100% (foundation → infrastructure → core → api, plugins isolated to `kernel`+`foundation`, cmd as composition root), function complexity limits (max 5 params, 3 returns, 80 lines, 10 public fns/file), no interfaces in `internal/config`, and adding `namingRules` to `arch-go.yml` once a pattern appears 3+ times. Sources: `arch-go.yml`, ADR-003.

#### Observability (`standards/architecture/observability.md`)

OpenTelemetry lives in `internal/telemetry/` as infrastructure (not a plugin), `telemetry.Init()` runs before `pm.InitAll()`, `provider.Shutdown()` runs after `pm.StopAll()`, and providers register globally via `otel.SetTracerProvider()` / `otel.SetMeterProvider()` so any package can obtain a tracer/meter. Source: ADR-009.

#### Plugin System (`standards/architecture/plugins.md`)

Plugin struct suffix rule (structs implementing `Plugin` must end with `Plugin`; per-plugin subpackages use `Plugin` as canonical name, satisfying the suffix rule via package namespacing), plugin lifecycle contract (`Register → Init → Start → Stop`, `ErrLicenseRequired` causes silent skip), and kernel isolation (plugins depend on the kernel interface; kernel never imports plugins; plugins register from the composition root). Source: ADR-003.

---

### Security Standards

Located in `.maister/docs/standards/security/`. Non-negotiable rules audited on every PR.

#### Authentication (`standards/security/authentication.md`)

JWT session cookie flags (`HttpOnly=true`, `Secure=true`, `SameSite=Strict`), bcrypt cost 12 with ≥10-char passwords (upper+lower+digit+special), 5-fail / 15-minute account lockout that tests must not bypass (use separate accounts instead), and lock-check before password verification to prevent timing attacks. Source: ADR-005.

#### Authorization (`standards/security/authorization.md`)

Permissions enforced via `RequirePermission` middleware at route registration (never call `auth.HasPermission(...)` inline inside handlers; use `auth.Perm*` constants, not raw strings), `resource:action` permission format (e.g., `catalog:read`, `users:write`), and system role (`IsSystem=true`) + last-active-admin protection (undeletable/unmodifiable via API). Source: ADR-005.

#### Data Handling (`standards/security/data-handling.md`)

Never log secrets (passwords, tokens, any secret material via slog/errors/comments); sensitive fields tagged `json:"-"` and `gorm:"type:text"` (password_hash, failed-attempt counters, lock timestamps, raw agent card bytes); GORM parameterized queries only (no raw string interpolation); input validation at the API boundary with stable HTTP mapping (duplicate→409, missing/invalid→400, not-found→404, 500 only for unexpected); don't leak internal error detail to clients (generic messages + server-side `slog.InfoContext` logging, `errors.Is` over string-match); CORS currently `*` — do not widen without approval.

---

### Backend Standards

Located in `.maister/docs/standards/backend/`. Go + GORM + dual-dialect database conventions.

#### API Design (`standards/backend/api.md`)

RESTful principles, consistent resource naming, versioning strategy, plural nouns for collections, limited URL nesting (2–3 levels), query parameters for filtering/sorting/pagination, proper HTTP status codes, and rate-limit headers.

#### Database Dialects (`standards/backend/database-dialects.md`)

Branch on `db.Dialect()` for dialect-specific SQL, no SQLite-specific DDL types in migrations/GORM tags (BLOB/DATETIME/BOOLEAN vs BYTEA/TIMESTAMPTZ; `ALTER TABLE DROP COLUMN IF EXISTS` is SQLite-unsafe), forward-only idempotent migrations (no Down function — rollback via backup/restore; versions strictly increase), migrations must work for both SQLite and PostgreSQL (verify locally), discovery upsert by `endpoint` with `AgentKey = SHA256(protocol+endpoint)`, and GORM parameterized queries only.

#### Go Conventions (`standards/backend/go-conventions.md`)

`context.Context` as first arg on all I/O functions, use request context with deadlines for DB/external calls (`slog.InfoContext(r.Context(), ...)` for audit correlation), error wrapping with `fmt.Errorf("doing x: %w", err)` + `errors.Is`/`errors.As` (no string-matching on `err.Error()`), slog structured logging with context fields (include `component`/`plugin`), no panic outside `main.go` and tests, snake_case Go filenames, three-group import ordering (stdlib / third-party / internal), errcheck enforced (no ignored error returns), sort Go map keys before serializing or building derived identifiers, exported identifier doc comments start with the identifier name, and chi handler signature with `JSONResponse`/`ErrorResponse` helpers plus standard middleware stack (Recovery, Logger, CORS, RequestID, otelhttp).

#### Database Migrations (`standards/backend/migrations.md`)

Reversible where possible, small and focused changes, zero-downtime awareness, separation of schema and data migrations, careful indexing on large tables (concurrent options), descriptive names, and version-control discipline (never modify migrations after deployment).

#### Models (`standards/backend/models.md`)

Clear singular model names with plural tables, created/updated timestamps, database-level constraints (NOT NULL, UNIQUE, foreign keys), appropriate column types, indexed foreign keys and frequently queried columns, multi-layer validation (model + DB), clear relationships with cascade behaviors, and practical normalization.

#### Database Queries (`standards/backend/queries.md`)

Parameterized queries always, avoid N+1 (eager loading / joins), select only needed columns, index strategic columns used in WHERE/JOIN/ORDER BY, transactions for related operations, query timeouts to prevent runaway queries, and caching expensive queries when appropriate.

---

### Frontend Standards

Located in `.maister/docs/standards/frontend/`. React + TypeScript + Tailwind + shadcn/ui conventions.

#### Accessibility (`standards/frontend/accessibility.md`)

Semantic HTML, keyboard navigation with visible focus, 4.5:1 color contrast, alt text and form labels, screen-reader testing, ARIA when semantic HTML isn't enough, ordered heading structure (h1–h6), focus management in dynamic content. AgentLens-specific: icon-only buttons need an accessible name (`aria-label` or visually-hidden text; accordion toggles also need `aria-expanded`+`aria-controls`); Radix `TooltipTrigger` wrapping `Badge`-style custom elements must use `asChild` to avoid nested interactive elements (see `StatusBadge`).

#### Build & Tooling (`standards/frontend/build-and-tooling.md`)

Bun 1.3.11 pinned as canonical package manager and script runner (`--frozen-lockfile`; pinned via `.bun-version` and Dockerfile frontend stage), Vite 8 + React 18 + Tailwind 3 stack with Radix primitives, React Query, React Router v6 (dev server proxies `/api` and `/healthz` to `localhost:8080`), and canonical npm scripts: `dev` (vite), `build` (`tsc && vite build` — typecheck must pass before bundling), `preview`, `test`, `test:watch`, `type-check`.

#### Components (`standards/frontend/components.md`)

Single responsibility, reusability via configurable props, composability over monoliths, clear documented prop interfaces with defaults, encapsulation of implementation details, consistent naming, local state kept close to where it is used, minimal props (split or compose when many), and documented usage/props/examples for team adoption.

#### CSS (`standards/frontend/css.md`)

Consistent methodology (Tailwind in AgentLens), work with the framework rather than fighting it, design tokens for colors/spacing/typography, minimize custom CSS, and production optimization via purging/tree-shaking.

#### Responsive Design (`standards/frontend/responsive.md`)

Mobile-first progressive enhancement, standard breakpoints, fluid layouts with percentage widths, relative units (rem/em) over fixed pixels, cross-device testing, touch-friendly tap targets (≥44x44px), mobile performance (optimized assets), readable typography across breakpoints, and content priority on smaller screens.

#### State & Data Fetching (`standards/frontend/state-and-data.md`)

TanStack React Query for all REST server state (`useQuery({ queryKey, queryFn })`, `useCatalogQuery` with URL-synced filter state), `AuthContext` + `ThemeContext` for client-global state (session: `user`, `isAuthenticated`, `permissions`, `hasPermission`), and flat API response types in `web/src/types.ts` mirroring `CatalogEntry.MarshalJSON()` (the `capabilities` field replaces the old `skills` field; each capability has a `kind` discriminator and a `properties` object).

#### TypeScript Conventions (`standards/frontend/typescript.md`)

Strict TypeScript with unused-symbol checks (`strict: true`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`, `isolatedModules`, `noEmit`; target ES2020; `moduleResolution: bundler`; JSX `react-jsx`; type-check via `make web-lint`), and `@/*` path alias mapping to `./src/*` declared in `web/tsconfig.json`, `web/vite.config.ts`, and `web/vitest.config.ts` for consistent builds/type-checks/tests.

#### UI Stack (`standards/frontend/ui-stack.md`)

shadcn/ui primitives imported via `@/components/ui/<name>`; higher-level components compose these rather than restyling raw HTML. PascalCase `.tsx` filenames for all React components in `src/components/` and `src/routes/**/components/` (shadcn primitives under `components/ui/` are the lone kebab-case exception). Folder layout: `web/src/hooks/`, `web/src/contexts/`, `web/src/pages/` for top-level, and `web/src/routes/<feature>/` with co-located `components/` subfolders. Tailwind + shadcn/ui HSL tokens with class-based dark mode (`darkMode: ['class']`; theme extends border/input/ring/background/foreground/primary/secondary/destructive/muted/accent/popover/card via `hsl(var(--x))`; container centered with 1400px max-width at 2xl; `tailwindcss-animate` plugin).

---

### Testing Standards

Located in `.maister/docs/standards/testing/`. Go testing + frontend Vitest + Playwright E2E conventions.

#### Test Writing (`standards/testing/test-writing.md`)

Test behavior not implementation (allows safe refactoring), clear test names (`shouldReturnErrorWhenUserNotFound`), mock external dependencies, fast unit-test execution, risk-based prioritization (business criticality × likelihood of bugs), balance coverage and velocity, critical-path focus, and depth matched to the risk profile of the code.

#### Go Testing (`standards/testing/go-testing.md`)

Table-driven subtests with `t.Run` + testify (`require` for fatal, `assert` for non-fatal; `_test.go` co-located), in-memory SQLite for store tests (`store.NewSQLiteStore(":memory:")` / `db.OpenMemory()` — no external DB required), API handler test coverage (status codes, response shape, auth enforcement across authenticated and unauthenticated paths), register `t.Cleanup(func() { ... })` to close in-memory DB handles (prevents FD leaks across the suite; follow `internal/db/migrate_test.go` `testDB` helper), and use separate accounts per test rather than bypassing the 5-fail / 15-min account lockout.

#### Frontend Testing (`standards/testing/frontend-testing.md`)

Vitest coverage thresholds 80/80/75/80 (lines / functions / branches / statements; v8 provider; reporters text/text-summary/html/lcov; exclusions: `main.tsx`, `test-setup.ts`, `.d.ts`, `.test.*`, `components/ui/**`, `types.ts`; enforced via `make web-test-coverage` in CI), jsdom + Testing Library setup (`globals: true`, shared `src/test-setup.ts`; `@testing-library/react` 16, `@testing-library/jest-dom` 6, `@testing-library/user-event` 14), co-located `.test.tsx`/`.test.ts` sibling for every component/hook/context, and restore global mocks using `vi.spyOn(...).mockReturnValue(...)` + `afterEach(() => spy.mockRestore())` (not direct assignment; properties may need `configurable: true`).

#### End-to-End Testing (`standards/testing/e2e.md`)

Playwright serial single-worker run on Chromium (`fullyParallel: false`, `workers: 1`; 60s test / 10s expect timeouts; on CI `forbidOnly: true`, `retries: 1`, reporter `github`; locally `list` reporter with no retries; trace on first retry, screenshots only on failure; `BASE = http://localhost:${AGENTLENS_PORT ?? 18080}`), real SQLite backend with masked admin password (CGO_ENABLED=1 binary on port 18080 with ephemeral `DATA_DIR`, waits for `/healthz`, masks bootstrap admin password via `::add-mask::$ADMIN_PW`, 20-minute CI workflow timeout), reuse shared Playwright helpers from `e2e/tests/helpers.ts` (`loginViaUI`, `loginViaAPI`, `authHeader`, `BASE`, `ADMIN_USER`, `adminPassword`) — never reimplement login inline, and `data-testid` screenshots for UI changes saved under `docs/images/` and referenced from `docs/end-user-guide.md`.

---

### DevOps Standards

Located in `.maister/docs/standards/devops/`. CI/CD, shell commands, commits, containers, diagrams, and git hooks.

#### CI/CD Gates (`standards/devops/ci-gates.md`)

Lint + test + build gate (`make vet`, `make lint`, `make web-lint` in the `lint` job; `build` job declares `needs: [lint, test, test-frontend]`). Go + frontend coverage gates (`make test-coverage` uploads `coverage.out`; Vitest thresholds 80/80/75/80 enforced on `src/**/*.{ts,tsx}` with excludes). Security scanning gate (CodeQL go+javascript-typescript matrix, govulncheck, Trivy CRITICAL+HIGH on the built Docker image with SARIF upload, `helm lint` + `helm template` validation; weekly Monday 06:00 UTC schedule). Playwright E2E gate against a real SQLite-backed binary. Semantic release automation (auto-bumped versions, GHCR images at `ghcr.io/<owner>/agentlens:<ver>` and `:latest`, Helm chart pushed to `oci://ghcr.io/<owner>/charts`, tags `v<app>` + `helm/v<chart>`, concurrency lock `release-${{ github.ref }}`). MkDocs Material docs auto-deploy on `main` when `docs/**`, `mkdocs.yml`, or `requirements-docs.txt` changes.

#### Command Conventions (`standards/devops/commands.md`)

RTK (Rust Token Killer) prefix mandatory for every shell command — no exceptions. Applies to `go`, `git`, `make`, `helm`, `docker`, `bun`, `cat`, `grep`, `awk`, `sed`, `find`, `ls`, `wc`, `sort`, `uniq`, `cut`, `chmod`, `mkdir`, `rm`, and any other shell tool. Safe passthrough when no filter exists. Each command in a pipeline gets its own `rtk` prefix (e.g., `rtk cat file.go | rtk grep func`).

#### Commit Conventions (`standards/devops/commits.md`)

Conventional Commits required: `<type>[(scope)]: <subject>`. Allowed types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`, `perf`, `revert`. Subject line max 100 characters; scope must be lower-case. Enforced via `web/commitlint.config.ts` in the `commit-msg` lefthook. The hook runs `perl -i` first to strip AI co-author trailers (`Co-Authored-By:` lines matching `claude|copilot|gemini|chatgpt|openai|cursor`) before commitlint validation. Branch naming: `feat/short-description` or `fix/short-description`.

#### Containers & Kubernetes (`standards/devops/containers.md`)

Multi-stage distroless nonroot Docker image: (1) `oven/bun:1.3.11-alpine` compiles the frontend (`bun install --frozen-lockfile` → `bun run build`); (2) `golang:1.26.1` cross-compiles with `CGO_ENABLED=1` (gcc, libc6-dev); (3) runtime `gcr.io/distroless/base-debian12:nonroot` as UID/GID 65532 exposing port 8080; version injected via `-ldflags "-X main.version=$(VERSION)"`. Hardened Helm pod-security defaults (`runAsNonRoot=true`, `runAsUser/Group/fsGroup=65532`, `seccompProfile: RuntimeDefault`, `readOnlyRootFilesystem=true`, `allowPrivilegeEscalation=false`, `capabilities.drop: [ALL]`, `automountServiceAccountToken=false`; default requests 100m/128Mi, limits 500m/512Mi; `/healthz` liveness, `/readyz` readiness). Strict `helm lint --strict` with default values + `ci/ci-values.yaml`, then `helm template ... --debug > /dev/null`, then `./scripts/test-helm-templates.sh`; Bitnami `postgresql ~16.x` optional subchart gated by `postgresql.enabled`; Chart `apiVersion v2`, `version 0.2.0`, `appVersion 0.2.0`. CGO_ENABLED=1 is required for all build/test targets because the SQLite driver is a cgo package.

#### Diagram Conventions (`standards/devops/diagrams.md`)

Mermaid-only diagrams across all documentation — no PlantUML, no ASCII art. `docs/architecture.md` is updated in Mermaid whenever the design changes.

#### Git Hooks — Lefthook (`standards/devops/git-hooks.md`)

Pre-commit hooks run in parallel: `go-fmt` (`test -z "$(gofmt -l .)"`), `go-lint` (`golangci-lint run`), `web-lint` (`cd web && bun run type-check`). Commit-msg runs `strip-ai-coauthors` (priority 1) then commitlint (`bunx commitlint --config commitlint.config.ts --edit {1}`). Pre-push runs `go-test` + `web-test` in parallel (priority 1), then `arch-test` (priority 2), blocking push on any failure. Install once via `rtk make hooks` — activation is explicit, not auto-run from `make all`. Source: `lefthook.yml`, ADR-002.

---

## CLAUDE.md Integration

`CLAUDE.md` in the project root references this index via `@.maister/docs/INDEX.md` in the "Coding Standards & Conventions" section so the AI assistant reads it before starting any task. Do not break that link.

---

*Last Updated*: 2026-04-17
*Entry count*: 4 project docs + 37 standards across 7 categories = **41 total entries**.

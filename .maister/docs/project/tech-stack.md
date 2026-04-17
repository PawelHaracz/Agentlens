# Technology Stack

## Overview

This document describes the technology choices and rationale for **AgentLens** — a self-hosted AI agent service discovery platform that catalogs, monitors, and exposes AI agents running across Kubernetes clusters and static endpoints.

The stack is deliberately split into a Go backend (microkernel + plugins) and an embedded React SPA served from the same binary. Single-binary delivery simplifies ops; plugin boundaries keep the core small and extensible.

---

## Languages

### Go 1.26.1 — Primary Backend Language
- **Usage**: ~85% of backend codebase (13K+ LOC in `internal/`)
- **Rationale**:
  - Single static binary with embedded frontend (via `embed.FS`) — zero-runtime deployment.
  - Strong concurrency primitives for polling-based discovery (k8s, HTTP probes).
  - First-class Kubernetes ecosystem (`k8s.io/client-go`).
  - Static typing + `arch-go` enforce architectural boundaries at build time.
- **Key features used**: generics (model registries), `context.Context` propagation, structured errors with `%w` wrapping, `slog` structured logging, `embed.FS` for SPA bundling.

### TypeScript 5.5 — Frontend Language
- **Usage**: ~100% of frontend (`web/src/`)
- **Rationale**: Type safety across React components, API response typings shared via generated or hand-written interfaces, editor tooling.
- **Key features used**: strict mode, generics for query hooks, discriminated unions for capability polymorphism on the client.

---

## Frameworks

### Backend

| Framework | Version | Purpose | Rationale |
|-----------|---------|---------|-----------|
| **chi** | 5.1.0 | HTTP router | Idiomatic Go, composable middleware, no hidden magic. Used in `internal/api/router.go`. |
| **GORM** | 1.31.1 | ORM | Dual-dialect support (SQLite + PostgreSQL) via `db.Dialect()` branching. Migrations remain hand-authored and idempotent. |
| **golang-jwt/jwt** | 5.3.1 | Authentication | Standard JWT with HS256; paired with HttpOnly/Secure/SameSite=Strict cookies. |
| **bcrypt** (`golang.org/x/crypto`) | 0.49.0 | Password hashing | Cost 12, minimum 10 chars with upper+lower+digit+special. |
| **k8s.io/client-go** | 0.29.0 | Kubernetes API | Powers the `plugins/sources/k8s` discovery plugin (annotation-based). |
| **Prometheus client** | 1.23.2 | Metrics | `/metrics` endpoint for scrape-based observability. |
| **OpenTelemetry** | 1.43.0 | Traces/metrics/logs | Nine OTel packages; ADR-009 treats OTel as infrastructure rather than an opt-in plugin. |

### Frontend

| Framework | Version | Purpose | Rationale |
|-----------|---------|---------|-----------|
| **React** | 18.3.1 | UI | Mainstream, deep ecosystem, concurrent rendering. |
| **React Router** | 6.26.2 | Client routing | File-based route organization under `src/routes/`. |
| **TanStack React Query** | 5.96.2 | Server state | Caching, retries, and auto-invalidation for catalog/capability endpoints. |
| **Tailwind CSS** | 3.4.10 | Styling | Utility-first; pairs cleanly with shadcn/ui tokens. |
| **shadcn/ui** (via Radix UI) | 15+ packages | Primitives | Copy-in component model; no runtime CSS framework lock-in. |
| **OpenTelemetry JS** | 0.214.0+ | Browser tracing | End-to-end traces from UI → API. |

### Testing

| Framework | Scope | Rationale |
|-----------|-------|-----------|
| **Go `testing` + `testify/assert`** | Backend unit/integration | Table-driven `t.Run` subtests, co-located `_test.go`. |
| **SQLite `:memory:`** | Store tests | Zero external dependency, still exercises real GORM/SQL paths. |
| **Vitest 4.1.2** | Frontend unit | Enforced 80% coverage threshold. |
| **@testing-library/react** | Component tests | Behavior-focused rather than implementation-focused. |
| **Playwright** | E2E | Full browser flows via `e2e/run-e2e.sh`; shared helpers in `e2e/tests/helpers.ts`. |
| **arch-go** | Architecture tests | 100% layer-boundary compliance enforced in CI. |

---

## Database

### SQLite (default) — `gorm.io/driver/sqlite` 1.14.22
- **Type**: Embedded relational (single file, CGO-backed)
- **Rationale**: Zero-ops for self-hosted single-instance deployments; ideal default for OSS adoption.
- **Schema notes**: JSON stored as `TEXT`, timestamps as `DATETIME`. See `internal/db/db.go`.

### PostgreSQL (enterprise) — `gorm.io/driver/postgres` 1.6.0
- **Type**: Relational (external managed service or in-cluster)
- **Rationale**: Horizontal read scalability, `JSONB` queries, multi-instance deployments.
- **Schema notes**: JSON stored as `JSONB`, timestamps as `TIMESTAMPTZ`. Branch on `db.Dialect()` — never assume SQLite-only syntax.

All migrations live in `internal/db/migrations.go`, are forward-only, and must be idempotent (see ADR-006).

---

## Build Tools & Package Management

- **Go modules** (`go.mod`, `go.sum`) — 34 direct dependencies, all current.
- **Bun** — frontend package manager (`web/bun.lock`, fast installs).
- **Vite 8.0.3** — frontend bundler and dev server.
- **Make** — canonical task runner (`make all` = format → lint → test → arch-test → build).

---

## Infrastructure

### Containerization
- **Docker** — single image; `CGO_ENABLED=1` is required for the SQLite driver.
- **docker-compose** — local stack including `docker-compose.otel.yml` for observability testing.

### Orchestration & Packaging
- **Kubernetes** — primary deployment target.
- **Helm chart** — `deploy/helm/agentlens/` (chart `0.2.0`, app `0.2.0`). Bitnami `postgresql ~16.x` as optional dependency for enterprise PG.
- **Annotation-based discovery** — pods opt in via annotations consumed by the k8s source plugin.

### CI/CD
- **GitHub Actions** — test, lint, build, and Helm template validation.
- **Lefthook** — git hooks (pre-commit format/lint, commit-msg conventional commits).

### Observability
- **OpenTelemetry** — traces, metrics, logs pipeline (OTel Collector friendly).
- **Prometheus** — `/metrics` endpoint.
- **Liveness/readiness** — `/healthz`, `/readyz`.

---

## Development Tools

### Linting & Formatting
- **golangci-lint** v2.11.4 — Go static analysis.
- **gofmt -s -w** — simplified Go formatting.
- **TypeScript compiler** (`bunx tsc --noEmit`) — type-only frontend lint.
- **arch-go** — layer-boundary and function-complexity enforcement (100% threshold).

### Type Checking
- Go: compiler-enforced (strict module graph).
- TypeScript: `strict: true`, checked via `make web-lint`.

---

## Key Dependencies

**Backend highlights** (`go.mod`):
- `github.com/go-chi/chi/v5`
- `gorm.io/gorm`, `gorm.io/driver/sqlite`, `gorm.io/driver/postgres`
- `github.com/golang-jwt/jwt/v5`
- `k8s.io/client-go`, `k8s.io/api`, `k8s.io/apimachinery`
- `go.opentelemetry.io/otel/*` (9 packages)
- `github.com/prometheus/client_golang`

**Frontend highlights** (`web/package.json`):
- `react`, `react-dom`, `react-router-dom`
- `@tanstack/react-query`
- `@radix-ui/*` (15+ primitives)
- `tailwindcss`, `class-variance-authority`, `lucide-react`
- `@opentelemetry/*` (instrumentation + context propagation)

---

## Version Management

- Go toolchain pinned via `go.mod` (`go 1.26.1`).
- Frontend deps pinned via `bun.lock`.
- Helm chart `appVersion` tracks release tags.
- Conventional Commits (enforced by Lefthook) drive changelog/release cadence.

---

*Last Updated*: 2026-04-17
*Auto-detected*: all versions, frameworks, build tooling, CI config (via `project-analyzer`).
*User-provided*: rationale framing, OSS-polish positioning.

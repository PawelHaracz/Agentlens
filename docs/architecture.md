# Architecture

This document describes the internal architecture of AgentLens — a self-hosted service discovery platform for AI agents.

---

## High-Level Overview

```mermaid
graph TD
    subgraph AgentLens
        API[REST API<br/>Chi Router]
        UI[Web UI<br/>React + Vite]
        DM[Discovery<br/>Manager]
        HC[Health<br/>Checker]

        AUTH[Auth Middleware<br/>JWT / RequirePermission]

        API --> AUTH
        UI --> AUTH
        DM --> AUTH
        HC --> AUTH

        AUTH --> KERNEL[Kernel Core<br/>Plugin Manager]

        KERNEL --> DB[(Database<br/>SQLite / PostgreSQL)]
        KERNEL --> PARSERS[Parser Plugins<br/>A2A / MCP]
        KERNEL --> SOURCES[Source Plugins<br/>Static / K8s]
    end

    AGENTS[External Agents<br/>A2A / MCP] -.->|discovered by| SOURCES
    BROWSER[Browser] -->|HTTP| API
    BROWSER -->|Static files| UI
```

---

## Domain Model: Product Archetype Pattern

AgentLens models each discovered agent or server as a **CatalogEntry**, following the Product Archetype pattern. This pattern treats each protocol as a *ProductType* and each discovered instance as a *CatalogEntry* that wraps it.

### Core Types

| Type | Description |
| ---- | ----------- |
| `CatalogEntry` | A discovered agent/server in the catalog. Contains identity, display name, description, protocol, endpoint, version, status, source, provider, categories, skills, validity, metadata, and raw card JSON. |
| `Protocol` | The agent communication protocol: `a2a` (Agent-to-Agent), `mcp` (Model Context Protocol), `a2ui` (Agent-to-UI). |
| `Provider` | Organization and team that owns the agent. |
| `Validity` | Time-bounded availability with `from`, `to`, and `last_seen` timestamps. |
| `Skill` | A capability exposed by a catalog entry, with name, description, tags, and input/output modes. |
| `Status` | Health status: `healthy`, `degraded`, `down`, `unknown`. |
| `SourceType` | How the entry was discovered: `k8s`, `config`, `push`, `upstream`. |

### Auth & Identity Types

| Type | Description |
| ---- | ----------- |
| `User` | An authenticated user with username, display name, email, password hash, role, and active/locked state. |
| `Role` | A named set of permissions. Three built-in system roles: `admin`, `editor`, `viewer`. |
| `Setting` | A key-value configuration entry scoped to a category (e.g., `ui.theme`, `app.name`). |

### CatalogEntry Relationships

```mermaid
graph TD
    CE[CatalogEntry] --> P[Protocol<br/>ProductType]
    CE --> PR[Provider<br/>organization + team]
    CE --> V[Validity<br/>time-bounded]
    CE --> S[Skills&#91;&#93;<br/>ProductFeatureType]
    CE --> C[Categories&#91;&#93;<br/>search/navigation]
    CE --> M[Metadata<br/>flexible key-value]
    CE --> RC[RawCard<br/>original protocol card JSON]
```

---

## Microkernel Plugin Architecture

AgentLens uses a **microkernel** design. The core kernel manages shared resources (store, config, logger, license), and all extensible behavior is implemented through plugins.

### Plugin Interface

Every plugin implements the base `Plugin` interface:

```go
type Plugin interface {
    Name() string
    Version() string
    Type() PluginType
    Init(k Kernel) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

Specialized interfaces extend `Plugin`:

- **`ParserPlugin`** — parses protocol-specific cards into `CatalogEntry`. Registered per-protocol. Methods: `Protocol()`, `Parse(raw, source)`, `CardPath()`.
- **`SourcePlugin`** — discovers catalog entries from a specific source. Method: `Discover(ctx)`.

### Plugin Types

| Type | Description | Examples |
| ---- | ----------- | ------- |
| `parser` | Parses protocol-specific card JSON | A2A parser, MCP parser |
| `source` | Discovers catalog entries | Static config, Kubernetes |
| `middleware` | HTTP middleware hooks | SSO, RBAC (enterprise) |
| `store` | Alternative store backends | PostgreSQL (enterprise) |

### Plugin Lifecycle

```mermaid
graph LR
    R[Register] --> I[InitAll]
    I --> S[StartAll]
    S --> RUN([Running])
    RUN --> STOP[StopAll]

    I -->|ErrLicenseRequired| SKIP[Skipped<br/>not started/stopped]
```

The `PluginManager` tracks which plugins were successfully initialized in a separate `initialized` slice. Only those are started and stopped — plugins that were skipped during init (e.g., enterprise plugins without a license) are never started.

### Kernel Interface

The kernel exposes these services to plugins:

| Method | Returns |
| ------ | ------- |
| `Store()` | Data store (SQLite or PostgreSQL) |
| `Config()` | Application configuration |
| `Logger()` | Structured logger (`slog`) |
| `License()` | License information |
| `Parser(protocol)` | Protocol-specific parser plugin |
| `RegisterRoutes(prefix, handler)` | Register HTTP route handlers |
| `RegisterMiddleware(mw)` | Register HTTP middleware |

---

## Component Details

### REST API

Built with [Chi router](https://github.com/go-chi/chi). All routes under `/api/v1/` are protected by the `RequireAuth` JWT middleware. The `RequirePermission` middleware enforces per-route permission checks.

**Auth routes (public):**

| Endpoint | Method | Description |
| -------- | ------ | ----------- |
| `/api/v1/auth/login` | POST | Authenticate and obtain a JWT token |
| `/api/v1/auth/logout` | POST | Clear the session cookie |

**Auth routes (protected):**

| Endpoint | Method | Permission | Description |
| -------- | ------ | ---------- | ----------- |
| `/api/v1/auth/refresh` | POST | Any | Refresh JWT token |
| `/api/v1/auth/me` | GET | Any | Current user info |
| `/api/v1/auth/password` | PUT | Any | Change password |

**Catalog routes (protected):**

| Endpoint | Method | Permission | Description |
| -------- | ------ | ---------- | ----------- |
| `/api/v1/catalog` | GET | `catalog:read` | List entries (with filters) |
| `/api/v1/catalog` | POST | `catalog:write` | Push-register an entry |
| `/api/v1/catalog/validate` | POST | `catalog:write` | Validate an A2A agent card (dry-run) |
| `/api/v1/catalog/register` | POST | `catalog:write` | Register an A2A agent from a raw card JSON |
| `/api/v1/catalog/import` | POST | `catalog:write` | Fetch and import an agent card from a URL |
| `/api/v1/catalog/{id}` | GET | `catalog:read` | Get entry by ID |
| `/api/v1/catalog/{id}` | DELETE | `catalog:delete` | Delete entry |
| `/api/v1/catalog/{id}/card` | GET | `catalog:read` | Get raw protocol card JSON |
| `/api/v1/skills` | GET | `catalog:read` | Search entries by skill name |
| `/api/v1/stats` | GET | `catalog:read` | Aggregate statistics |

**User/Role/Settings routes (protected):**

| Endpoint | Method | Permission |
| -------- | ------ | ---------- |
| `/api/v1/users` | GET / POST | `users:read` / `users:write` |
| `/api/v1/users/{id}` | GET / PUT / DELETE | `users:read` / `users:write` / `users:delete` |
| `/api/v1/roles` | GET / POST | `roles:read` / `roles:write` |
| `/api/v1/roles/{id}` | PUT / DELETE | `roles:write` |
| `/api/v1/settings` | GET / PUT | `settings:read` / `settings:write` |
| `/api/v1/settings/{category}` | GET | `settings:read` |

See [api.md](api.md) for full API documentation.

### Auth Layer

Authentication uses **JWT tokens** signed with HS256:

```mermaid
sequenceDiagram
    participant B as Browser
    participant A as API
    participant US as UserStore
    participant JWT as JWTService

    B->>A: POST /auth/login (username, password)
    A->>US: GetByUsername()
    US-->>A: User
    A->>A: CheckPassword() [bcrypt cost 12]

    alt Password incorrect
        A->>US: IncrementFailedAttempts()
        Note over US: Lock after 5 fails (15 min)
        A-->>B: 401 Unauthorized
    else Password correct
        A->>US: ResetFailedAttempts()
        A->>JWT: GenerateToken(user, role)
        JWT-->>A: JWT token
        A-->>B: 200 + token + HttpOnly cookie
    end

    B->>A: GET /api/v1/catalog (Authorization: Bearer)
    A->>JWT: ValidateToken()
    A->>A: RequirePermission(catalog:read)
    A-->>B: 200 OK + entries
```

**JWT Claims:**

```json
{
  "user_id": "...",
  "username": "admin",
  "role_id": "role-admin",
  "permissions": ["catalog:read", "catalog:write", "..."],
  "exp": "...",
  "iss": "agentlens"
}
```

The JWT secret is loaded from `AGENTLENS_JWT_SECRET`. If not set, a random secret is generated at startup (tokens will not survive restarts).

**Admin Bootstrap:**

On first startup, if no users exist, the `BootstrapAdmin` function creates an `admin` user with a cryptographically random 20-character password (upper + lower + digit + special) and prints it to stdout.

### Database Layer

AgentLens uses [GORM](https://gorm.io/) as its ORM with support for two backends:

| Backend | Use case |
| ------- | -------- |
| **SQLite** (default) | Single-instance, zero-config, file-based |
| **PostgreSQL** | Production, multi-instance, high-availability |

The active dialect is selected by `database.dialect` in the config (or `AGENTLENS_DB_DIALECT`).

**Schema Migrations:**

Migrations are versioned and tracked in the `schema_migrations` table. They run automatically at startup. Current migrations:

| Version | Description |
| ------- | ----------- |
| 1 | Create `catalog_entries` table |
| 2 | Create `roles` and `users` tables |
| 3 | Seed default roles (admin, editor, viewer) |
| 4 | Create `settings` table with defaults |

**Store Interface:**

```go
type Store interface {
    Create(ctx, entry) error
    Get(ctx, id) (*CatalogEntry, error)
    List(ctx, filter) ([]*CatalogEntry, error)
    Update(ctx, entry) error
    Delete(ctx, id) error
    FindByEndpoint(ctx, endpoint) (*CatalogEntry, error)
    SearchSkills(ctx, query) ([]*CatalogEntry, error)
    Stats(ctx) (*Stats, error)
}
```

Alongside `Store`, the data layer includes:

- **`UserStore`** — CRUD for users, failed-attempt tracking, lockout management
- **`RoleStore`** — CRUD for roles (system roles protected from deletion)
- **`SettingsStore`** — key-value settings with category scoping

### Discovery Manager

The discovery manager polls sources at a configurable interval (`poll_interval`). For each discovered entry, it upserts into the store keyed by `endpoint` (which has a UNIQUE constraint).

```mermaid
sequenceDiagram
    participant SP as Source Plugin
    participant DM as Discovery Manager
    participant ST as Store

    loop Every poll_interval
        DM->>SP: Discover(ctx)
        SP-->>DM: []CatalogEntry

        loop For each entry
            DM->>ST: FindByEndpoint(endpoint)
            alt Entry exists
                DM->>ST: Update(entry)
            else New entry
                DM->>ST: Create(entry)
            end
        end

        DM->>ST: Mark missing entries as "down"
    end
```

**Sources:**

1. **Static source** — reads from `sources:` in the config file, fetches card JSON from each URL, and passes it through the appropriate parser plugin.
2. **Kubernetes source** — watches Kubernetes Services annotated with `agentlens.io/type`, constructs card URLs from service endpoints + card paths, and discovers agents.

### Web Dashboard

React + Vite + TypeScript frontend using [shadcn/ui](https://ui.shadcn.com/) components. Embedded into the Go binary at build time via `embed.FS`. Components:

- **AuthContext** — JWT token management, 401-redirect, permission checking
- **ThemeContext** — light/dark/system theme with CSS class switching
- **LoginPage** — authentication form
- **Layout** — sticky navbar with user avatar dropdown, mobile hamburger
- **CatalogList** — paginated table with protocol/status badges and filters
- **EntryDetail** — full entry view with skills, metadata, categories, and raw card JSON
- **RegisterAgentDialog** — multi-tab registration modal: Paste JSON, Upload File, Import from URL
- **CardPreview** — renders a validated agent card preview before registration
- **SettingsPage** — 4-tab management UI (General, Users, Roles, My Account)
- **ProtectedRoute** — auth guard that redirects unauthenticated users to `/login`

### Health Checker

A plugin that periodically pings each catalog entry's endpoint and updates its status (`healthy`, `degraded`, `down`). Configurable interval, timeout, and concurrency.

### Enterprise Plugins (License-Gated)

These plugins are registered but gracefully skipped when no enterprise license is present:

| Plugin | Type | Description |
| ------ | ---- | ----------- |
| SSO | middleware | Single Sign-On integration |
| RBAC | middleware | Role-Based Access Control |
| Audit | middleware | Audit logging |
| PostgreSQL | store | PostgreSQL store backend |

---

## Data Flow

### Request Flow

```mermaid
graph LR
    REQ[HTTP Request] --> ROUTER[Chi Router]
    ROUTER --> AUTH[RequireAuth]
    AUTH -->|Valid token| PERM[RequirePermission]
    PERM -->|Authorized| HANDLER[API Handler]
    HANDLER --> STORE[Store]
    STORE --> DB[(Database)]

    AUTH -->|No token| R401[401 Unauthorized]
    PERM -->|Insufficient| R403[403 Forbidden]

    ROUTER --> STATIC[Static Files]
    STATIC --> SPA[React SPA]
```

---

## Directory Structure

```
agentlens/
├── cmd/agentlens/          # Application entrypoint
│   └── main.go
├── internal/
│   ├── api/                # REST API handlers, router, auth middleware
│   ├── auth/               # JWT service, password hashing, bootstrap
│   ├── config/             # Configuration loading (YAML + env vars)
│   ├── db/                 # GORM DB wrapper, migration framework
│   ├── discovery/          # Discovery manager + sources
│   ├── health/             # Health checker
│   ├── kernel/             # Microkernel core + plugin manager
│   ├── model/              # Domain model (CatalogEntry, User, Role, Setting)
│   ├── server/             # HTTP server lifecycle
│   ├── service/            # Shared services (CardFetcher for URL import)
│   └── store/              # GORM-backed stores (catalog, user, role, settings)
├── plugins/
│   ├── enterprise/         # License-gated enterprise plugins
│   │   ├── audit/
│   │   ├── postgres/
│   │   ├── rbac/
│   │   └── sso/
│   ├── health/             # Health checker plugin
│   ├── parsers/            # Protocol parser plugins
│   │   ├── a2a/
│   │   └── mcp/
│   └── sources/            # Discovery source plugins
│       ├── k8s/
│       └── static/
├── web/                    # React frontend (embedded)
│   └── src/
│       ├── contexts/       # AuthContext, ThemeContext
│       ├── pages/          # LoginPage, SettingsPage
│       └── components/     # Layout, CatalogList, EntryDetail, RegisterAgentDialog, etc.
├── deploy/
│   └── helm/agentlens/     # Helm chart
├── examples/
│   ├── docker-compose.yaml          # SQLite setup
│   ├── docker-compose.postgres.yaml # PostgreSQL setup
│   └── mock-agents/                 # Mock A2A and MCP agents
├── scripts/                # Release and CI scripts
├── docs/                   # Documentation
├── Dockerfile              # Multi-stage Docker build (distroless)
├── Makefile                # Build automation
└── .github/workflows/      # CI, code scanning, E2E, release
```

---

## Configuration

AgentLens is configured via YAML file and/or environment variables (prefixed with `AGENTLENS_`):

| Setting | Env Var | Default | Description |
| ------- | ------- | ------- | ----------- |
| `port` | `AGENTLENS_PORT` | 8080 | HTTP port |
| `data_dir` | `AGENTLENS_DATA_DIR` | `./data` | Data directory |
| `log_level` | `AGENTLENS_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `license_key` | `AGENTLENS_LICENSE_KEY` | — | Enterprise license key |
| `poll_interval` | `AGENTLENS_POLL_INTERVAL` | `5m` | Discovery poll interval |
| `database.dialect` | `AGENTLENS_DB_DIALECT` | `sqlite` | Database backend (sqlite/postgres) |
| `database.sqlite.path` | `AGENTLENS_DB_SQLITE_PATH` | `./data/agentlens.db` | SQLite file path |
| `database.postgres.host` | `AGENTLENS_DB_POSTGRES_HOST` | `localhost` | PostgreSQL host |
| `database.postgres.port` | `AGENTLENS_DB_POSTGRES_PORT` | `5432` | PostgreSQL port |
| `auth.jwt_secret` | `AGENTLENS_JWT_SECRET` | (auto) | JWT signing secret |
| `auth.session_duration` | `AGENTLENS_SESSION_DURATION` | `24h` | Token expiry |
| `kubernetes.enabled` | `AGENTLENS_KUBERNETES_ENABLED` | `false` | Enable K8s discovery |
| `health_check.enabled` | `AGENTLENS_HEALTH_CHECK_ENABLED` | `true` | Enable health checks |
| `health_check.interval` | `AGENTLENS_HEALTH_CHECK_INTERVAL` | `30s` | Health check interval |
| `health_check.timeout` | `AGENTLENS_HEALTH_CHECK_TIMEOUT` | `5s` | Per-endpoint timeout |
| `health_check.concurrency` | `AGENTLENS_HEALTH_CHECK_CONCURRENCY` | `10` | Max concurrent checks |

---

## Deployment

See [DevOps Guide](devops-guide.md) for full deployment documentation including Docker, Helm, CI/CD, and release pipeline details.

### Docker

Multi-stage build: Bun (frontend) → Go (backend) → Distroless (runtime).

```bash
docker build -t agentlens .
docker run -p 8080:8080 \
  -e AGENTLENS_JWT_SECRET=your-secret \
  agentlens
```

### Docker Compose (SQLite)

```bash
cd examples && docker compose up
```

### Docker Compose (PostgreSQL)

```bash
cd examples && docker compose -f docker-compose.postgres.yaml up
```

### Kubernetes (Helm)

```bash
helm install agentlens deploy/helm/agentlens \
  -n agentlens --create-namespace \
  --set auth.jwtSecret=your-secret
```

The Helm chart creates: Deployment, Service, ServiceAccount, ClusterRole (for K8s discovery), ClusterRoleBinding, ConfigMap, and optionally a PersistentVolumeClaim for SQLite storage.

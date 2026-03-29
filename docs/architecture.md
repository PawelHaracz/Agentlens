# Architecture

This document describes the internal architecture of AgentLens — a self-hosted service discovery platform for AI agents.

---

## High-Level Overview

```
┌───────────────────────────────────────────────────────────────┐
│                        AgentLens                              │
│                                                               │
│  ┌─────────┐   ┌──────────┐   ┌──────────┐   ┌───────────┐  │
│  │ REST API │   │ Web UI   │   │ Discovery│   │  Health    │  │
│  │ (Chi)    │   │ (React)  │   │ Manager  │   │  Checker   │  │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘   └─────┬─────┘  │
│       │              │              │               │         │
│       └──────────────┴──────┬───────┴───────────────┘         │
│                             │                                 │
│                    ┌────────▼────────┐                        │
│                    │   Kernel (Core) │                        │
│                    │   Plugin Mgr    │                        │
│                    └────────┬────────┘                        │
│                             │                                 │
│         ┌───────────────────┼───────────────────┐             │
│         │                   │                   │             │
│  ┌──────▼──────┐   ┌───────▼──────┐   ┌────────▼────────┐   │
│  │  SQLite     │   │  Parser      │   │  Source          │   │
│  │  Store      │   │  Plugins     │   │  Plugins         │   │
│  └─────────────┘   │  (A2A, MCP)  │   │  (Static, K8s)   │   │
│                    └──────────────┘   └──────────────────┘   │
└───────────────────────────────────────────────────────────────┘
```

---

## Domain Model: Product Archetype Pattern

AgentLens models each discovered agent or server as a **CatalogEntry**, following the Product Archetype pattern. This pattern treats each protocol as a *ProductType* and each discovered instance as a *CatalogEntry* that wraps it.

### Core Types

| Type | Description |
|------|-------------|
| `CatalogEntry` | A discovered agent/server in the catalog. Contains identity, display name, description, protocol, endpoint, version, status, source, provider, categories, skills, validity, metadata, and raw card JSON. |
| `Protocol` | The agent communication protocol: `a2a` (Agent-to-Agent), `mcp` (Model Context Protocol), `a2ui` (Agent-to-UI). |
| `Provider` | Organization and team that owns the agent. |
| `Validity` | Time-bounded availability with `from`, `to`, and `last_seen` timestamps. |
| `Skill` | A capability exposed by a catalog entry, with name, description, tags, and input/output modes. |
| `Status` | Health status: `healthy`, `degraded`, `down`, `unknown`. |
| `SourceType` | How the entry was discovered: `k8s`, `config`, `push`, `upstream`. |

### Relationships

```
CatalogEntry
├── Protocol (ProductType)
├── Provider (organization + team)
├── Validity (time-bounded)
├── Skills[] (ProductFeatureType)
├── Categories[] (for search/navigation)
├── Metadata (flexible key-value)
└── RawCard (original protocol card JSON)
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
|------|-------------|---------|
| `parser` | Parses protocol-specific card JSON | A2A parser, MCP parser |
| `source` | Discovers catalog entries | Static config, Kubernetes |
| `middleware` | HTTP middleware hooks | SSO, RBAC (enterprise) |
| `store` | Alternative store backends | PostgreSQL (enterprise) |

### Plugin Lifecycle

```
Register → InitAll → StartAll → [running] → StopAll
                │
                └── Plugins returning ErrLicenseRequired
                    are skipped (not started/stopped)
```

The `PluginManager` tracks which plugins were successfully initialized in a separate `initialized` slice. Only those are started and stopped — plugins that were skipped during init (e.g., enterprise plugins without a license) are never started.

### Kernel Interface

The kernel exposes these services to plugins:

| Method | Returns |
|--------|---------|
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

Built with [Chi router](https://github.com/go-chi/chi). Routes are under `/api/v1/`:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/healthz` | GET | Health check |
| `/api/v1/catalog` | GET | List catalog entries (with filters) |
| `/api/v1/catalog` | POST | Push-register a catalog entry |
| `/api/v1/catalog/{id}` | GET | Get entry by ID |
| `/api/v1/catalog/{id}` | DELETE | Delete entry |
| `/api/v1/catalog/{id}/card` | GET | Get raw protocol card JSON |
| `/api/v1/skills` | GET | Search entries by skill name |
| `/api/v1/stats` | GET | Aggregate statistics |

See [api.md](api.md) for full API documentation.

### Discovery Manager

The discovery manager polls sources at a configurable interval (`poll_interval`). For each discovered entry, it upserts into the store keyed by `endpoint` (which has a UNIQUE constraint).

**Sources:**

1. **Static source** — reads from `sources:` in the config file, fetches card JSON from each URL, and passes it through the appropriate parser plugin.
2. **Kubernetes source** — watches Kubernetes Services annotated with `agentlens.io/type`, constructs card URLs from service endpoints + card paths, and discovers agents.

### Store

SQLite (default) with a single `catalog_entries` table. The `endpoint` column has a UNIQUE constraint to prevent duplicates. The store interface is:

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
    Close() error
}
```

### Web Dashboard

React + Vite + TypeScript frontend using [shadcn/ui](https://ui.shadcn.com/) components. Embedded into the Go binary at build time via `embed.FS`. Components:

- **CatalogList** — paginated table of catalog entries with protocol/status badges
- **EntryDetail** — detail view with skills, metadata, and raw card JSON
- **SearchBar** — full-text search with protocol/status filters
- **StatsBar** — aggregate counts by status and source

### Health Checker

A plugin that periodically pings each catalog entry's endpoint and updates its status (`healthy`, `degraded`, `down`). Configurable interval, timeout, and concurrency.

### Enterprise Plugins (License-Gated)

These plugins are registered but gracefully skipped when no enterprise license is present:

| Plugin | Type | Description |
|--------|------|-------------|
| SSO | middleware | Single Sign-On integration |
| RBAC | middleware | Role-Based Access Control |
| Audit | middleware | Audit logging |
| PostgreSQL | store | PostgreSQL store backend |

---

## Directory Structure

```
agentlens/
├── cmd/agentlens/          # Application entrypoint
│   └── main.go
├── internal/
│   ├── api/                # REST API handlers + router
│   ├── config/             # Configuration loading
│   ├── discovery/          # Discovery manager + sources
│   ├── health/             # Health checker
│   ├── kernel/             # Microkernel core + plugin manager
│   ├── model/              # Domain model (CatalogEntry, Skill, etc.)
│   ├── server/             # HTTP server lifecycle
│   └── store/              # SQLite store + migrations
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
├── deploy/
│   └── helm/agentlens/     # Helm chart
├── examples/
│   ├── docker-compose.yaml
│   └── mock-agents/        # Mock A2A and MCP agents
├── docs/                   # Documentation
├── Dockerfile              # Multi-stage Docker build
├── Makefile                # Build automation
└── .github/workflows/      # CI and code scanning
```

---

## Data Flow

### Discovery Flow

```
Source Plugin          Discovery Manager        Store
     │                       │                    │
     │  Discover(ctx)        │                    │
     │◄──────────────────────│                    │
     │                       │                    │
     │  []CatalogEntry       │                    │
     │──────────────────────►│                    │
     │                       │  FindByEndpoint()  │
     │                       │───────────────────►│
     │                       │                    │
     │                       │  Create/Update     │
     │                       │───────────────────►│
     │                       │                    │
```

### Request Flow

```
HTTP Request → Chi Router → API Handler → Store → SQLite
                                │
                                └── Web UI (embedded static files)
```

---

## Configuration

AgentLens is configured via YAML file and/or environment variables (prefixed with `AGENTLENS_`):

| Setting | Env Var | Default | Description |
|---------|---------|---------|-------------|
| `port` | `AGENTLENS_PORT` | 8080 | HTTP port |
| `data_dir` | `AGENTLENS_DATA_DIR` | `./data` | SQLite data directory |
| `log_level` | `AGENTLENS_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `license_key` | `AGENTLENS_LICENSE_KEY` | — | Enterprise license key |
| `poll_interval` | `AGENTLENS_POLL_INTERVAL` | `5m` | Discovery poll interval |
| `kubernetes.enabled` | `AGENTLENS_KUBERNETES_ENABLED` | `false` | Enable K8s discovery |
| `health_check.enabled` | `AGENTLENS_HEALTH_CHECK_ENABLED` | `true` | Enable health checks |
| `health_check.interval` | `AGENTLENS_HEALTH_CHECK_INTERVAL` | `30s` | Health check interval |
| `health_check.timeout` | `AGENTLENS_HEALTH_CHECK_TIMEOUT` | `5s` | Health check timeout |
| `health_check.concurrency` | `AGENTLENS_HEALTH_CHECK_CONCURRENCY` | `10` | Max concurrent checks |

---

## Deployment

### Docker

Multi-stage build: Node.js (frontend) → Go (backend) → Alpine (runtime).

```bash
docker build -t agentlens .
docker run -p 8080:8080 agentlens
```

### Kubernetes (Helm)

```bash
helm install agentlens deploy/helm/agentlens -n agentlens --create-namespace
```

The Helm chart creates: Deployment, Service, ServiceAccount, ClusterRole (for K8s discovery), ClusterRoleBinding, and ConfigMap.

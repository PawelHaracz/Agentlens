# Developer Guide

This guide covers setting up a development environment, building, testing, and extending AgentLens.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Makefile Targets](#makefile-targets)
- [Building](#building)
- [Testing](#testing)
- [Linting](#linting)
- [Git Hooks](#git-hooks)
- [Writing Plugins](#writing-plugins)
- [Frontend Development](#frontend-development)
- [Docker](#docker)
- [Helm Chart](#helm-chart)
- [CI/CD](#cicd)
- [Code Style](#code-style)

---

## Prerequisites

- **Go** 1.26.1 (with CGO enabled for SQLite)
- **Bun** 1.3+ (install via [bun.sh](https://bun.sh))
- **golangci-lint** (install via `make tools`)
- **Docker** (for container builds and scanning)
- **Helm** 3+ (for chart development)

---

## Getting Started

```bash
# Clone the repository
git clone https://github.com/PawelHaracz/agentlens
cd agentlens

# Install Go dependencies
make deps

# Install frontend dependencies
make web-install

# Build everything
make web-build
make build

# Run the server
make run
```

The server starts on `http://localhost:8080`.

---

## Project Structure

```
agentlens/
├── cmd/agentlens/          # Application entrypoint (main.go)
├── internal/               # Private application packages
│   ├── api/                # REST API handlers and router
│   ├── config/             # Configuration loading and env overrides
│   ├── discovery/          # Discovery manager and source orchestration
│   ├── health/             # Health checker logic
│   ├── kernel/             # Microkernel core, plugin interfaces, plugin manager
│   ├── model/              # Domain model types (CatalogEntry, Skill, etc.)
│   ├── server/             # HTTP server lifecycle management
│   ├── service/            # Shared services (CardFetcher for URL import)
│   └── store/              # SQLite store, migrations, query builders
├── plugins/                # Plugin implementations
│   ├── enterprise/         # License-gated plugins (SSO, RBAC, audit, PostgreSQL)
│   ├── health/             # Health checker plugin
│   ├── parsers/            # Protocol card parsers (A2A, MCP)
│   │   ├── a2a/
│   │   └── mcp/
│   └── sources/            # Discovery source plugins
│       ├── k8s/            # Kubernetes service discovery
│       └── static/         # Static config source
├── web/                    # React + Vite + shadcn/ui frontend
│   └── src/
│       ├── components/     # UI components (CatalogList, EntryDetail, RegisterAgentDialog, etc.)
│       │   └── ui/         # shadcn/ui base components
│       ├── api.ts          # API client functions
│       ├── types.ts        # TypeScript type definitions
│       └── App.tsx         # Root application component
├── deploy/helm/agentlens/  # Helm chart
├── examples/               # Docker Compose + mock agents
├── docs/                   # Documentation
├── .github/workflows/      # CI and code scanning workflows
├── Dockerfile              # Multi-stage Docker build
├── Makefile                # Build automation
└── go.mod                  # Go module definition
```

---

## Makefile Targets

Run `make help` to see all available targets:

| Target | Description |
|--------|-------------|
| `make all` | Format, lint, test, and build |
| `make build` | Build the Go binary (CGO enabled for SQLite) |
| `make test` | Run all Go tests |
| `make test-coverage` | Run tests with coverage report |
| `make test-coverage-html` | Generate HTML coverage report |
| `make test-race` | Run tests with Go race detector |
| `make lint` | Run golangci-lint |
| `make vet` | Run go vet |
| `make format` | Format Go source files |
| `make run` | Build and run the server |
| `make clean` | Remove build artifacts |
| `make deps` | Download and tidy Go dependencies |
| `make tools` | Install golangci-lint, arch-go, lefthook |
| `make hooks` | Install git hooks via lefthook (run once after cloning) |
| `make web-install` | Install frontend dependencies (bun) |
| `make web-build` | Build the frontend |
| `make web-lint` | TypeScript type check |
| `make docker-build` | Build the Docker image locally |
| `make docker-scan` | Scan Docker image with Trivy |
| `make helm-lint` | Lint and validate the Helm chart |

---

## Building

### Go Backend

```bash
make build
```

This compiles the binary to `bin/agentlens` with `CGO_ENABLED=1` (required for the SQLite driver).

### Frontend

```bash
make web-install   # Install bun dependencies
make web-build     # Build production bundle
```

The frontend builds to `web/dist/` and is embedded into the Go binary via `embed.FS`.

### Docker Image

```bash
make docker-build
```

This runs a multi-stage Docker build:
1. **Stage 1** (Node.js) — builds the frontend
2. **Stage 2** (Go) — builds the backend binary
3. **Stage 3** (Alpine) — minimal runtime image

---

## Testing

### Run All Tests

```bash
make test
```

### With Coverage

```bash
make test-coverage        # Print coverage to terminal
make test-coverage-html   # Open coverage.html in browser
```

### With Race Detector

```bash
make test-race
```

### Test Files

Tests live alongside the code they test (Go convention):

- `internal/api/handlers_test.go`
- `internal/api/validate_handler_test.go`
- `internal/api/register_handler_test.go`
- `internal/api/import_handler_test.go`
- `internal/api/auth_handlers_test.go`
- `internal/api/user_handlers_test.go`
- `internal/api/role_handlers_test.go`
- `internal/api/settings_handlers_test.go`
- `internal/config/config_test.go`
- `internal/discovery/a2a_test.go`, `mcp_test.go`, `k8s_test.go`, `manager_test.go`
- `internal/health/checker_test.go`
- `internal/store/sqlite_test.go`, `user_store_test.go`, `role_store_test.go`, `settings_store_test.go`
- `plugins/parsers/a2a/validation_test.go`

### API Handler Test Pattern

API handler tests use a shared `testRouter` helper (in `internal/api/test_helpers_test.go`) that wires a full in-memory stack. The helper creates a real `kernel.Core`, registers A2A and MCP parsers, and passes the kernel to `api.RouterDeps`:

```go
database, _ := db.OpenMemory()
// ... run migrations ...
catalogStore := store.NewSQLStore(database)

core := kernel.NewCore(catalogStore, nil, slog.Default(), kernel.LicenseInfo{})
a2aParser := a2aplugin.New()
_ = a2aParser.Init(core)
core.RegisterParser(a2aParser)
mcpParser := mcpplugin.New()
_ = mcpParser.Init(core)
core.RegisterParser(mcpParser)

router := api.NewRouter(api.RouterDeps{
    Kernel:        core,
    UserStore:     userStore,
    RoleStore:     roleStore,
    SettingsStore: settingsStore,
    JWTService:    jwtService,
})
```

Tests that exercise the import endpoint (`POST /api/v1/catalog/import`) inject a stub `service.Fetcher` via `RouterDeps.CardFetcher` to avoid real outbound HTTP.

---

## Linting

### Go

```bash
make lint       # golangci-lint
make vet        # go vet
make format     # gofmt
```

### Frontend (TypeScript)

```bash
make web-lint   # tsc --noEmit
```

---

## Git Hooks

AgentLens uses [lefthook](https://github.com/evilmartians/lefthook) to manage git hooks checked into the repo. Hooks provide a local quality gate that catches issues before they reach CI.

### What each hook does

| Hook | When | Jobs |
|------|------|------|
| **pre-commit** | On every `git commit` | `gofmt` check (parallel), `golangci-lint` (parallel), TypeScript type-check (parallel) |
| **commit-msg** | After pre-commit | commitlint validates conventional commit format |
| **pre-push** | On `git push` | `make test` + `make web-test` (parallel), then `make arch-test` |

### Install hooks (once per clone)

```bash
make hooks
```

Run this once after cloning. It installs the lefthook binary and activates all hooks. It is **not** run automatically by `make all` to avoid surprising contributors.

### Verify hooks are installed

```bash
lefthook list
```

Expected output lists `pre-commit`, `commit-msg`, and `pre-push`.

### Commit message format

Hooks enforce [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>[(scope)]: <subject>
```

Allowed types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`, `ci`, `build`, `perf`, `revert`

Examples:
```
feat: add kubernetes source plugin
feat(api): expose capabilities endpoint
fix(auth): correct lockout timer reset
chore: bump go version to 1.26.1
```

### Skipping hooks

```bash
git commit --no-verify -m "wip: ..."   # skip pre-commit + commit-msg
git push --no-verify                    # skip pre-push
LEFTHOOK=0 git commit ...              # disable lefthook entirely
```

---

## Writing Plugins

AgentLens is designed to be extended through plugins. All plugins implement the `kernel.Plugin` interface.

### API Handler and Router Wiring

The API layer depends on `kernel.Kernel`, not on the store directly. `NewHandler(k kernel.Kernel)` obtains the catalog store via `k.Store()` and looks up parsers via `k.Parser(protocol)`. The `RouterDeps` struct follows the same pattern — it carries a `Kernel` field rather than a raw store:

```go
// RouterDeps holds all dependencies for the router.
type RouterDeps struct {
    Kernel        kernel.Kernel   // required; provides store + parser lookup
    UserStore     *store.UserStore
    RoleStore     *store.RoleStore
    SettingsStore *store.SettingsStore
    JWTService    *auth.JWTService
    // CardFetcher is optional. When nil, a default CardFetcher with SSRF
    // protection is used (for POST /api/v1/catalog/import).
    CardFetcher   service.Fetcher
}
```

In `main.go` the `core` kernel is wired after all plugins have been initialised:

```go
core := kernel.NewCore(catalogStore, cfg, logger, lic)
// ... register and init plugins ...
router := api.NewRouter(api.RouterDeps{
    Kernel:        core,
    UserStore:     userStore,
    RoleStore:     roleStore,
    SettingsStore: settingsStore,
    JWTService:    jwtService,
})
```

### Creating a Parser Plugin

A parser plugin converts protocol-specific card JSON into an `AgentType` (Product Archetype pattern). The `ParserPlugin` interface requires both `Parse` and `Validate` methods.

`Parse` returns `*model.AgentType` populated with the protocol, endpoint, version, provider, capabilities, and raw definition. The handler is responsible for wrapping the `AgentType` in a `CatalogEntry` (display name, status, source, validity) before persisting.

```go
package myplugin

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/PawelHaracz/agentlens/internal/kernel"
    "github.com/PawelHaracz/agentlens/internal/model"
)

type MyParser struct {
    log *slog.Logger
}

func New() *MyParser { return &MyParser{} }

func (p *MyParser) Name() string              { return "my-protocol-parser" }
func (p *MyParser) Version() string           { return "0.1.0" }
func (p *MyParser) Type() kernel.PluginType   { return kernel.PluginTypeParser }
func (p *MyParser) Protocol() model.Protocol  { return "my-protocol" }
func (p *MyParser) CardPath() string          { return "/.well-known/my-card.json" }

func (p *MyParser) Init(k kernel.Kernel) error {
    p.log = k.Logger().With("plugin", p.Name())
    return nil
}

func (p *MyParser) Start(ctx context.Context) error { return nil }
func (p *MyParser) Stop(ctx context.Context) error  { return nil }

// Validate checks the raw card JSON for spec compliance without persisting anything.
// Called by the validate endpoint and the import endpoint before Parse.
func (p *MyParser) Validate(raw []byte) kernel.ValidationResult {
    // Inspect the raw JSON and return structured diagnostics.
    return kernel.ValidationResult{
        Valid:       true,
        SpecVersion: "1.0",
        Errors:      nil,
        Warnings:    nil,
        Preview:     map[string]any{"display_name": "My Agent"},
    }
}

// Parse converts a raw card JSON blob into an AgentType.
// The returned AgentType has Provider and Capabilities populated.
// Source (push, k8s, config) is a catalog concern — the handler adds it.
func (p *MyParser) Parse(raw []byte) (*model.AgentType, error) {
    var card struct {
        Name     string `json:"name"`
        Endpoint string `json:"endpoint"`
        Version  string `json:"version"`
    }
    if err := json.Unmarshal(raw, &card); err != nil {
        return nil, fmt.Errorf("parsing my-protocol card: %w", err)
    }
    if card.Name == "" {
        return nil, fmt.Errorf("my-protocol card missing required field: name")
    }
    return &model.AgentType{
        Protocol:      "my-protocol",
        Endpoint:      card.Endpoint,
        Version:       card.Version,
        RawDefinition: raw,
        AgentKey:      model.ComputeAgentKey("my-protocol", card.Endpoint),
    }, nil
}
```

Register in `cmd/agentlens/main.go`:

```go
pm.Register(myplugin.New())
```

### Creating a Source Plugin

A source plugin discovers agents from a specific location and returns `AgentType` values. The discovery manager wraps each into a `CatalogEntry`.

```go
package mysource

import (
    "context"
    "github.com/PawelHaracz/agentlens/internal/kernel"
    "github.com/PawelHaracz/agentlens/internal/model"
)

type MySource struct {
    kernel kernel.Kernel
}

func New() *MySource { return &MySource{} }

func (s *MySource) Name() string            { return "my-source" }
func (s *MySource) Version() string         { return "0.1.0" }
func (s *MySource) Type() kernel.PluginType { return kernel.PluginTypeSource }

func (s *MySource) Init(k kernel.Kernel) error {
    s.kernel = k
    return nil
}

func (s *MySource) Start(ctx context.Context) error { return nil }
func (s *MySource) Stop(ctx context.Context) error  { return nil }

func (s *MySource) Discover(ctx context.Context) ([]*model.AgentType, error) {
    // Discover agents from your source.
    // Use s.kernel.Parser(protocol).Parse(raw) to convert card JSON into AgentType.
    return agentTypes, nil
}
```

### Enterprise Plugins (License-Gated)

Return `kernel.ErrLicenseRequired` from `Init()` to gate a plugin behind an enterprise license:

```go
func (p *MyPlugin) Init(k kernel.Kernel) error {
    if !k.License().IsEnterprise() {
        return kernel.ErrLicenseRequired
    }
    // Initialize plugin
    return nil
}
```

The plugin manager will log a warning and skip the plugin — it will not be started or stopped.

---

## Frontend Development

### Development Server

```bash
cd web
bun run dev
```

This starts the Vite dev server with hot module replacement. The frontend proxies API requests to `http://localhost:8080`.

### Component Library

The UI uses [shadcn/ui](https://ui.shadcn.com/) components built on Radix UI + Tailwind CSS:

- `Badge` — protocol and status badges
- `Button` — action buttons
- `Card` — entry detail cards
- `Table` — catalog list table
- `Input` — search input
- `Select` — filter dropdowns
- `ScrollArea` — scrollable containers
- `Separator` — visual dividers
- `Skeleton` — loading placeholders

### Adding UI Components

Components are in `web/src/components/`. The shadcn/ui base components are in `web/src/components/ui/`.

### TypeScript Types

All API types are defined in `web/src/types.ts`. These mirror the flat JSON response shape produced by `CatalogEntry.MarshalJSON()`:

- `CatalogEntry` — flat catalog entry type (merges AgentType + CatalogEntry fields)
- `Capability` — polymorphic agent capability with `kind`, `name`, `description`, `properties`
- `Protocol`, `Status`, `SourceType` — enum types
- `Provider`, `Validity` — nested types
- `Stats`, `ListFilter` — API-specific types

The `capabilities` field on `CatalogEntry` replaces the old `skills` field. Each capability has a `kind` discriminator (e.g. `a2a.skill`, `mcp.tool`) and a `properties` object with protocol-specific fields.

---

## Docker

### Build Image

```bash
make docker-build
```

### Scan for Vulnerabilities

```bash
make docker-scan
```

Requires [Trivy](https://trivy.dev/) installed locally. In CI, scanning runs automatically via the code-scanning workflow.

---

## Helm Chart

The Helm chart is in `deploy/helm/agentlens/`.

### Lint the Chart

```bash
make helm-lint
```

### Template Preview

```bash
helm template agentlens deploy/helm/agentlens --debug
```

### Key Values

See `deploy/helm/agentlens/values.yaml` for all configurable values. Key settings:

| Value | Default | Description |
|-------|---------|-------------|
| `replicaCount` | `1` | Number of replicas |
| `image.repository` | `ghcr.io/pawelharacz/agentlens` | Container image |
| `service.port` | `80` | Service port |
| `env.AGENTLENS_KUBERNETES_ENABLED` | `true` | Enable K8s discovery |
| `persistence.enabled` | `true` | Enable persistent storage |
| `persistence.size` | `1Gi` | Storage size |

---

## CI/CD

### CI Workflow (`.github/workflows/ci.yml`)

Triggered on PRs to `main`:

| Job | Description |
|-----|-------------|
| **Lint** | `go vet` + `golangci-lint` + TypeScript type check |
| **Test** | Go tests with coverage, uploads `coverage.out` artifact |
| **Build** | Full frontend + backend build (gates on lint + test) |

### Code Scanning (`.github/workflows/code-scanning.yml`)

Triggered on PRs, push to `main`, and weekly schedule:

| Job | Description |
|-----|-------------|
| **CodeQL** | Static analysis for Go and JavaScript/TypeScript |
| **govulncheck** | Go dependency vulnerability scanning |
| **Docker Scan** | Trivy vulnerability scan of the Docker image |
| **Helm Lint** | Helm chart linting and template validation |

---

## Code Style

- **Go**: Follow standard Go conventions. Run `make format` before committing.
- **Frontend**: TypeScript with strict mode. Use shadcn/ui patterns for UI components.
- **Commits**: Use [Conventional Commits](https://www.conventionalcommits.org/) format: `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `ci:`.
- **Testing**: Place tests alongside source files (`_test.go`). Use `testify` for assertions.
- **Logging**: Use `slog` structured logging. Include `component` and `plugin` fields.

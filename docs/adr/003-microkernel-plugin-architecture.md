# ADR-003: Microkernel Plugin Architecture

Date: 2026-04-11
Status: Accepted
Related: [ADR-004](004-product-catalog-pattern.md) (complementary pattern)

## Context

AgentLens is an agent catalog that must support multiple agent protocols (A2A, MCP, future protocols), multiple discovery sources (static config, Kubernetes, future: Azure, AWS), enterprise features (SSO, RBAC, audit logging, PostgreSQL), and pluggable storage backends. These requirements create tension between two forces:

1. **Core stability** — the catalog API, domain model, and authentication must remain stable and well-tested regardless of which extensions are active.
2. **Extension velocity** — new protocols, sources, and enterprise features must be addable without modifying core code or risking regressions in unrelated areas.

A monolithic architecture would couple all of these concerns, making it impossible to ship a community edition without enterprise code, test protocol parsers without database setup, or add a new discovery source without touching the HTTP layer.

## Decision

Use the **microkernel (plugin) architectural pattern** with a minimal kernel that provides shared infrastructure and a plugin interface for all extension points.

### Kernel responsibilities

The kernel (`internal/kernel/`) provides exactly four things to plugins:

- **Store** — data persistence via the `store.Store` interface
- **Config** — application configuration (read-only)
- **Logger** — structured logging
- **License** — feature gating for enterprise plugins

Plugins register routes and middleware back into the kernel. The kernel never depends on any plugin — dependency flows one way: plugin → kernel → foundation.

### Plugin lifecycle

All plugins implement a uniform interface:

```
Register → InitAll → StartAll → [running] → StopAll
```

- **Register**: plugin added to manager (no side effects)
- **Init**: plugin receives kernel reference, performs setup. Returns `ErrLicenseRequired` to silently skip.
- **Start**: plugin begins background work (e.g., health check polling)
- **Stop**: reverse-order teardown, errors collected (not short-circuited)

### Plugin types

| Type | Purpose | Examples |
|------|---------|---------|
| `parser` | Parse protocol-specific agent cards into `AgentType` | `a2a-parser`, `mcp-parser` |
| `source` | Discover agents from external systems | `static-source`, `k8s-source` |
| `middleware` | Add HTTP middleware or background services | `health-checker`, enterprise SSO/RBAC/audit |
| `store` | Provide alternative storage backends | enterprise PostgreSQL |
| `cardstore` | Persist raw card bytes | `card-store` |

### Layer enforcement

Architecture boundaries are enforced by `arch-go` at 100% compliance:

```
Foundation  (model, config)       — no internal deps
Infrastructure (store, auth)      — foundation only
Core (kernel, discovery)          — foundation + infrastructure
Plugins (plugins/**)              — kernel + foundation; never api/auth/cmd
API (api)                         — core + infrastructure; never plugins
Entrypoint (cmd/**)               — composition root
```

Plugins depend on the kernel interface, never on concrete kernel internals. The API layer depends on the kernel, never on plugins. Only `cmd/agentlens/main.go` wires plugins to the kernel.

## Use Cases

### 1. Adding a new agent protocol

A new protocol (e.g., `a2ui`) requires only a new `ParserPlugin` in `plugins/parsers/a2ui/`. The plugin implements `Parse()`, `Validate()`, `Protocol()`, and `CardPath()`. It is registered in `main.go` with `pm.Register(a2uiplugin.New())`. No changes to the kernel, API, store, or existing parsers.

### 2. Enterprise feature gating

Enterprise plugins (SSO, RBAC, audit, PostgreSQL) check `k.License().HasFeature("sso")` during `Init()`. Without a license, they return `ErrLicenseRequired` and the plugin manager skips them with a warning log. The community binary contains the same code — enterprise features are present but inert. No conditional compilation, no build tags, no separate binaries.

### 3. Adding a new discovery source

A Kubernetes-based deployment discovers agents via Service annotations (`agentlens.io/type`). A cloud deployment discovers agents from Azure Resource Graph. Each is a `SourcePlugin` that implements `Discover(ctx) ([]*AgentType, error)`. The discovery manager iterates all registered sources — adding Azure discovery means adding one file in `plugins/sources/azure/` and one `pm.Register()` call. Zero changes to existing sources or the catalog API.

### 4. Background services without coupling

The health checker runs as a `PluginTypeMiddleware` plugin with a real `Start()` that launches a background goroutine. It defines a local `proberStore` interface mirroring only the store methods it needs, keeping it decoupled from the full `store.Store` interface. The kernel's `StopAll()` cancels its context. Other plugins (e.g., future metrics exporter) follow the same pattern.

### 5. Pluggable storage backends

The community edition uses SQLite via the default store. Enterprise customers register the `enterprise-postgres` plugin, which (when licensed) replaces the storage backend. The plugin layer boundary prevents the PostgreSQL driver from leaking into the community build's dependency tree in the future if build tags are added.

## Consequences

### Positive

- New protocols, sources, and features are addable without modifying core code.
- Enterprise features gate cleanly via license check — single binary, no build variants needed today.
- Plugins are independently testable — parsers need no database, sources need no HTTP server.
- `arch-go` enforces boundaries at CI time — accidental coupling is caught before merge.
- Reverse-order stop ensures clean shutdown (last started = first stopped).

### Negative / Trade-offs

- Plugin wiring in `main.go` is manual — no auto-discovery or dynamic loading.
- Adding a new plugin type requires changes to the kernel interface (e.g., adding `CardStorePlugin` required `RegisterCardStore` + `CardStore()` on `Core`).
- Type assertions are needed in `InitAll()` to detect plugin sub-types (`ParserPlugin`, `CardStorePlugin`), adding a small maintenance surface.

### Neutral

- All plugins are compiled into the same binary. Dynamic loading (shared libraries) is explicitly out of scope — Go's static linking model makes in-process plugins the pragmatic choice.
- The kernel interface (`Kernel`) is narrow by design. Plugins that need more than Store/Config/Logger/License must define local interfaces for the specific store methods they need.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **Monolithic package structure** | Couples all features; impossible to test parsers independently or gate enterprise features without build tags |
| **Go plugin (dynamic .so loading)** | Fragile, requires exact Go version match, no cross-compilation, not production-ready for most deployments |
| **gRPC sidecar plugins (HashiCorp go-plugin style)** | Overkill for in-process extensions; adds serialization overhead, process management complexity, and deployment burden |
| **Build tags for enterprise features** | Requires separate CI pipelines and binaries; harder to test the full matrix; license gating via `ErrLicenseRequired` achieves the same separation at runtime |

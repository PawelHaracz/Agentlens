# ADR-009: OpenTelemetry as Infrastructure, Not Plugin

Date: 2026-04-12
Status: Accepted
Related: [ADR-003](003-microkernel-plugin-architecture.md) (plugin extension model)

## Context

AgentLens needs full observability — distributed tracing, metrics, and structured log export via OpenTelemetry. The microkernel architecture (ADR-003) establishes plugins as the extension mechanism: capabilities are added via `Plugin` implementations registered with the `PluginManager`.

The question is whether OTel should follow this pattern or be treated differently.

Three forces are in tension:

1. **Lifecycle ordering** — telemetry must be available before any plugin initializes (so plugin init/start operations produce spans) and must flush after all plugins stop (so shutdown operations are captured). The plugin lifecycle is `InitAll → StartAll → [running] → StopAll`. Telemetry needs to wrap this entire lifecycle.

2. **Cross-cutting scope** — every layer uses telemetry: API handlers, store queries, health probes, parsers. A plugin sits in the `plugins/` layer, which arch-go constrains to depend only on `kernel` + foundation. A telemetry plugin couldn't be imported by `api` or `store` without violating layer boundaries.

3. **Zero-cost when disabled** — when `telemetry.enabled=false`, the system must have zero overhead. OTel's global no-op providers achieve this naturally. A plugin would add registration overhead, lifecycle management, and conditional checks.

## Decision

Place OpenTelemetry in `internal/telemetry/` as **infrastructure** (same layer as `store` and `auth`), not as a microkernel plugin.

- `telemetry.Init()` is called in `main.go` **before** `pm.InitAll()`.
- `provider.Shutdown()` is deferred in `main.go` and runs **after** `pm.StopAll()`.
- Providers are registered globally via `otel.SetTracerProvider()` / `otel.SetMeterProvider()`. Any package calls `otel.Tracer("agentlens.xyz")` without explicit dependency injection.
- When disabled, global providers remain as no-ops. Zero goroutines, zero connections.
- `internal/telemetry/` is added to arch-go as infrastructure layer with the same constraints as `store` and `auth`.

## Consequences

### Positive

- Telemetry outlives all plugins — every span from plugin init through shutdown is captured and flushed.
- No layer boundary violations — packages use `otel.Tracer()` (external dependency), not an internal import.
- Zero overhead when disabled — no plugin registration, no lifecycle methods, no conditional paths.
- Consistent with how OTel is used across the Go ecosystem (global providers as infrastructure).

### Negative / Trade-offs

- **Global state** — `otel.SetTracerProvider()` is process-global. Tests must reset globals via `t.Cleanup()`.
- **Not discoverable via plugin manager** — `pm.List()` won't show telemetry. Operators check telemetry status via `/metrics` or log output, not the plugin registry.

### Neutral

- The store tracing decorator in `internal/telemetry/storetracer.go` defines its own interface matching the traced methods via Go structural typing. It does not import `internal/store/` — the decorator is wired at the composition root (`main.go`).

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **Microkernel plugin** | Cannot wrap plugin lifecycle (init before, flush after). Layer boundary violations if other packages import it |
| **Explicit DI (pass providers through deps)** | Invasive — touches every constructor signature. Over-engineered for cross-cutting infrastructure |
| **Middleware-only (HTTP spans only)** | Misses highest-value instrumentation: parser failures, probe latency, store slow queries |

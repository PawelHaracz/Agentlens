# Architecture: Observability

OpenTelemetry is treated as infrastructure, not as an optional plugin. This ensures traces/metrics/logs are available before plugins initialize.

### OpenTelemetry Lives in `internal/telemetry/` as Infrastructure

Place OTel setup in `internal/telemetry/` at the same layer as `store` and `auth`. Treating telemetry as a plugin would create a chicken-and-egg problem: plugins would need telemetry before the telemetry plugin has started.

Startup/shutdown order in `main.go`:

```go
provider, err := telemetry.Init(ctx, cfg)
if err != nil { /* ... */ }
defer provider.Shutdown(ctx) // runs AFTER pm.StopAll()

// ... wire kernel, store, auth ...

if err := pm.InitAll(ctx); err != nil { /* ... */ }
if err := pm.StartAll(ctx); err != nil { /* ... */ }
```

- `telemetry.Init()` runs **before** `pm.InitAll()`.
- `provider.Shutdown()` runs **after** `pm.StopAll()`.
- Providers register globally via `otel.SetTracerProvider()` / `otel.SetMeterProvider()` so any package can obtain a tracer/meter without being passed one.

Source: ADR-009.

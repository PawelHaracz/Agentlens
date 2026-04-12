# ADR-010: Dual-Output Structured Logging with OTel Bridge

Date: 2026-04-12
Status: Accepted
Related: [ADR-009](009-opentelemetry-as-infrastructure.md) (OTel as infrastructure), [ADR-006](006-dual-dialect-database-and-forward-only-migrations.md) (dual-output precedent)

## Context

When OpenTelemetry is enabled, AgentLens needs to export structured logs via OTLP so they appear in the operator's observability platform alongside traces and metrics. The canonical OTel Go approach is to replace `slog.SetDefault()` with the `otelslog` bridge handler — all logs route through OTLP.

Two forces push against this default:

1. **`kubectl logs` must always work** — platform engineers debug production issues by tailing pod logs. If log output is redirected exclusively to OTLP, `kubectl logs` shows nothing. When the collector is down, logs are lost entirely. This is unacceptable for a tool that SREs evaluate for production.

2. **Volume control** — a registry under load generates debug/trace-level logs (every HTTP request, every probe cycle). Sending all of this through OTLP overwhelms the collector and inflates storage costs. Operators need stdout to show everything (for `kubectl logs --since=5m` debugging) but OTLP to receive only actionable log levels.

ADR-006 established a precedent for dual-output patterns: one binary, two database dialects, same data model. This is the logging equivalent — one log call, two outputs, different filtering.

## Decision

When telemetry is enabled, replace the default `slog` handler with a **fan-out handler** that writes to both outputs simultaneously:

```go
type fanoutHandler struct {
    stdout      slog.Handler   // existing JSON handler → os.Stdout
    bridge      slog.Handler   // otelslog bridge → OTLP
    exportLevel slog.Level     // minimum level for OTLP export
}
```

- **stdout** receives all logs at the configured `logLevel` (unchanged from current behavior). Always active. Never replaced.
- **OTLP bridge** receives only logs at or above `logExportLevel` (default: `info`). Active only when telemetry is enabled.
- Both outputs inject `trace_id` and `span_id` from the active span context when available.
- Configuration: `telemetry.log_export_level` / `AGENTLENS_OTEL_LOG_EXPORT_LEVEL`.

When telemetry is disabled: existing slog setup (stdout JSON) is untouched. Zero overhead.

## Consequences

### Positive

- `kubectl logs` always works — stdout output is never removed or redirected.
- Logs in the observability platform are correlated with traces via `trace_id` / `span_id`.
- Operators control OTLP log volume independently from local debug verbosity.
- Collector outage doesn't cause log loss — stdout remains the reliable fallback.

### Negative / Trade-offs

- **Two writes per log record** — minor overhead when both outputs are active. Acceptable for a registry's log volume (not a high-throughput data plane).
- **Custom handler** — the fan-out handler is not part of the OTel SDK. Small amount of custom code (~50 lines) to maintain.
- **Divergent filtering** — stdout and OTLP may show different log lines for the same timeframe. Operators must be aware that OTLP is filtered.

### Neutral

- `logExportLevel` defaults to `info`, matching common production collector configurations.
- When `logLevel=debug` and `logExportLevel=info`, debug logs appear only in `kubectl logs`, not in the observability platform. This is the expected behavior.

## Alternatives considered

| Option | Why rejected |
|--------|--------------|
| **Replace slog with OTel bridge (canonical approach)** | `kubectl logs` goes silent. Collector outage = total log loss. Unacceptable for production infrastructure |
| **Skip log export entirely** | Traces + metrics cover most debugging, but log correlation (`trace_id` in logs) is the highest-value feature for incident response. Worth the small overhead |
| **Custom slog handler with trace injection only (no OTLP)** | Gets 80% of the value (trace-correlated stdout logs) but misses centralized log aggregation. Operators still need to SSH/kubectl into each pod |

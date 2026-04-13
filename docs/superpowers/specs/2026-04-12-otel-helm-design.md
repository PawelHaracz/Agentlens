# OpenTelemetry Observability & Production-Ready Helm Chart

**Date:** 2026-04-12
**Status:** Draft
**Tier:** 2 — SHOULD HAVE (final Tier 2 features)
**Effort:** L (7–9 days)
**Branch:** `feat/devops/otel` — single branch, single PR

---

## Goal

Ship full observability (traces, metrics, structured logs via OpenTelemetry) and a production-grade Helm chart (security, scaling, PostgreSQL, monitoring). These features are tightly coupled — the Helm chart's ServiceMonitor consumes the Prometheus endpoint that OTel exposes, and the telemetry config is wired through the chart's `values.yaml`.

After this work, AgentLens goes from "works on my laptop" to "approved by an SRE for staging".

---

## Design Decisions (from brainstorming)

| Decision | Choice | Rationale |
|---|---|---|
| Spec/branch/PR | Single unit | Helm + OTel are tightly coupled; avoids broken cross-references |
| Slog bridge | Fan-out: stdout + OTLP, configurable `logExportLevel` | Operators need `kubectl logs` always; OTLP filtered separately |
| Telemetry arch-go layer | Infrastructure | Imports foundation only; wired from `cmd/`; globals for cross-cutting |
| Dockerfile/UID | Distroless `:nonroot` (65532) + k8s enforcement | Defense-in-depth; `docker run` and k8s both secure |
| Metrics endpoint | Same port (8080), route-level bypass | Consistent with existing `/healthz` pattern |
| Integration tests | Docker Compose + Jaeger; Helm lint/template; no Kind | Proves OTel works e2e; chart correctness via template tests |
| OTel integration pattern | Global providers + thin wrappers | Canonical OTel-Go; minimal DI changes; store stays decoupled |
| Frontend telemetry | `traceparent` propagation via fetch instrumentation | Full e2e trace from browser → backend → probed agents |

---

## Scope

### In scope

**Observability:**
1. OTLP/gRPC and OTLP/HTTP exporter, configurable via environment variables
2. Distributed tracing for all HTTP handler operations with trace context propagation (`traceparent`)
3. Custom spans for: parser execution, health probe per entry, store queries
4. Metrics: request count/latency histogram, health probe results, parser success/failure, catalog entry gauge
5. Structured log export via OTel slog bridge with fan-out (stdout + OTLP)
6. Prometheus `/metrics` pull endpoint (needed by Helm ServiceMonitor)
7. Graceful shutdown: flush pending telemetry on SIGTERM
8. Frontend `traceparent` propagation via `@opentelemetry/instrumentation-fetch`

**Helm chart:**
9. Resource requests/limits, PodDisruptionBudget, HPA
10. Liveness (`/healthz`), readiness (`/readyz`), startup probes
11. Ingress (networking.k8s.io/v1) + Gateway API (gateway.networking.k8s.io/v1)
12. ServiceMonitor (Prometheus Operator) pointing at `/metrics`
13. PostgreSQL subchart (Bitnami) as optional dependency + external DB config
14. `helm test` hook, `values.schema.json` validation
15. SecurityContext: non-root (65532), read-only rootfs, dropped capabilities
16. PVC for SQLite mode, NetworkPolicy (optional), topology spread constraints
17. Init container for PostgreSQL readiness wait

### Out of scope

- Custom Grafana dashboards / JSON models
- Continuous profiling (pprof)
- Frontend RUM (document-load, user-interaction instrumentation)
- Operator / CRD-based deployment
- Multi-replica active-active with SQLite
- Cert-manager TLS integration
- ArgoCD / FluxCD application manifests
- Kind cluster integration tests (follow-up)

---

## Part A: Telemetry Package — `internal/telemetry/`

### Provider struct

```go
package telemetry

type Provider struct {
    TracerProvider  *sdktrace.TracerProvider
    MeterProvider   *sdkmetric.MeterProvider
    LoggerProvider  *sdklog.LoggerProvider
    PromHandler     http.Handler   // nil when Prometheus disabled
    Shutdown        func(ctx context.Context) error
}

func Init(ctx context.Context, cfg config.TelemetryConfig, version string) (*Provider, error)
```

### Behavior matrix

| `Enabled` | `Endpoint` | Result |
|---|---|---|
| `false` | any | No-op provider, nil PromHandler, no goroutines |
| `true` | empty + no `OTEL_EXPORTER_OTLP_ENDPOINT` | Log warning, fall back to no-op |
| `true` | valid | Full provider: trace + metric + log exporters |
| `true` + `prometheus.enabled` | any | PromHandler non-nil, registered by caller at `/metrics` |

### Env var precedence

`AGENTLENS_OTEL_ENDPOINT` > `OTEL_EXPORTER_OTLP_ENDPOINT` > empty (no-op).

### Sampler

`ParentBased(TraceIDRatioBased(cfg.TracesSampleRate))` — respects incoming `traceparent`.

### Shutdown

Called from `main` with 5s context timeout. Flushes all three providers (trace, metric, log).

### Fan-out slog handler

```go
type fanoutHandler struct {
    stdout      slog.Handler   // existing JSON handler
    bridge      slog.Handler   // otelslog bridge
    exportLevel slog.Level     // filter for OTLP export
}
```

All logs go to stdout at configured `logLevel`. Only logs >= `logExportLevel` (default `info`) go to OTLP bridge. `trace_id`/`span_id` injected into both outputs when span is active.

### Naming convention

```
Tracer/Meter: "agentlens.<package>"     e.g. "agentlens.api", "agentlens.health"
Span names:   "<HTTP method> <route>"   for handlers (auto by otelhttp)
              "<operation>"             for internal ops, e.g. "health.probe"
```

### Arch-go placement

Infrastructure layer. `telemetry` may import `config`, `model`, `service` (foundation). Must NOT import `api`, `kernel`, `store`, `plugins`, `cmd`.

---

## Part B: Configuration

### New types in `internal/config/config.go`

```go
type TelemetryConfig struct {
    Enabled          bool              `yaml:"enabled"`
    Endpoint         string            `yaml:"endpoint"`
    Protocol         string            `yaml:"protocol"`
    Insecure         bool              `yaml:"insecure"`
    ServiceName      string            `yaml:"serviceName"`
    Environment      string            `yaml:"environment"`
    TracesSampler    string            `yaml:"tracesSampler"`
    TracesSampleRate float64           `yaml:"tracesSampleRate"`
    MetricsInterval  time.Duration     `yaml:"metricsInterval"`
    LogExportLevel   string            `yaml:"logExportLevel"`
    Headers          map[string]string `yaml:"headers"`
    Prometheus       PrometheusConfig  `yaml:"prometheus"`
}

type PrometheusConfig struct {
    Enabled bool `yaml:"enabled"`
}
```

Added to `Config`:

```go
type Config struct {
    // ... existing fields ...
    Telemetry TelemetryConfig `yaml:"telemetry"`
}
```

### Defaults

| Field | Default | Rationale |
|---|---|---|
| `enabled` | `false` | Zero overhead when off |
| `protocol` | `grpc` | Standard OTel default |
| `insecure` | `true` | Cluster-internal collectors |
| `serviceName` | `agentlens` | |
| `environment` | `production` | |
| `tracesSampler` | `parentbased_traceidratio` | Respects caller's sampling |
| `tracesSampleRate` | `1.0` | Safe for registry traffic |
| `metricsInterval` | `30s` | Matches health probe interval |
| `logExportLevel` | `info` | Debug stays stdout-only |
| `prometheus.enabled` | `false` | |

### Env var mapping

| Env var | Field |
|---|---|
| `AGENTLENS_OTEL_ENABLED` | `telemetry.enabled` |
| `AGENTLENS_OTEL_ENDPOINT` | `telemetry.endpoint` |
| `AGENTLENS_OTEL_PROTOCOL` | `telemetry.protocol` |
| `AGENTLENS_OTEL_INSECURE` | `telemetry.insecure` |
| `AGENTLENS_OTEL_SERVICE_NAME` | `telemetry.serviceName` |
| `AGENTLENS_OTEL_ENVIRONMENT` | `telemetry.environment` |
| `AGENTLENS_OTEL_TRACES_SAMPLER` | `telemetry.tracesSampler` |
| `AGENTLENS_OTEL_TRACES_SAMPLE_RATE` | `telemetry.tracesSampleRate` |
| `AGENTLENS_OTEL_METRICS_INTERVAL` | `telemetry.metricsInterval` |
| `AGENTLENS_OTEL_LOG_EXPORT_LEVEL` | `telemetry.logExportLevel` |
| `AGENTLENS_OTEL_HEADERS` | `telemetry.headers` (comma-separated `k=v`) |
| `AGENTLENS_METRICS_PROMETHEUS_ENABLED` | `telemetry.prometheus.enabled` |

Fallback: if `AGENTLENS_OTEL_ENDPOINT` empty, check `OTEL_EXPORTER_OTLP_ENDPOINT`.

New `applyTelemetryEnv(&cfg.Telemetry)` function following existing patterns.

---

## Part C: Instrumentation Points

### 1. HTTP middleware — all API requests

Wrap chi router with `otelhttp.NewHandler()` — **outermost** in middleware chain.

- Automatic span per request with `http.request.method`, `http.response.status_code`, `url.path`
- Trace context extraction from incoming `traceparent` header
- Trace context injection into response `traceresponse` header

### 2. Health prober

Wrap prober's `http.Client` transport with `otelhttp.NewTransport()` — outgoing requests carry `traceparent`.

**Span** `health.probe` with attributes:

| Attribute | Type | Value |
|---|---|---|
| `agentlens.entry.id` | string | catalog entry ID |
| `agentlens.entry.name` | string | display name |
| `agentlens.probe.url` | string | probed URL |
| `agentlens.probe.latency_ms` | int64 | measured latency |
| `agentlens.probe.result` | string | `success` / `degraded` / `failure` / `timeout` / `skipped` |
| `agentlens.probe.state_before` | string | lifecycle state before probe |
| `agentlens.probe.state_after` | string | lifecycle state after probe |
| `agentlens.probe.error` | string | error message (truncated 256 chars) |

State transitions emit span event:

```go
span.AddEvent("state_transition", trace.WithAttributes(
    attribute.String("from", string(before)),
    attribute.String("to", string(after)),
))
```

**Metrics** (meter `agentlens.health`):

| Metric | Type | Attributes |
|---|---|---|
| `agentlens.health.probes.total` | Counter | `result`, `protocol` |
| `agentlens.health.probes.latency` | Histogram | `result`, `protocol` |
| `agentlens.health.state_transitions.total` | Counter | `from`, `to`, `protocol` |

Histogram buckets: 10, 50, 100, 250, 500, 1000, 2500, 5000 ms.

### 3. Parsers (A2A + MCP)

**Span** `parser.<type>.parse` with attributes:

| Attribute | Type | Value |
|---|---|---|
| `agentlens.parser.type` | string | `a2a` or `mcp` |
| `agentlens.parser.input_size` | int64 | byte length |
| `agentlens.parser.spec_version` | string | detected version |
| `agentlens.parser.skill_count` | int | skills parsed |
| `agentlens.parser.extension_count` | int | extensions parsed |
| `agentlens.parser.security_scheme_count` | int | security schemes parsed |

On failure: `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)`.

**Metrics:**

| Metric | Type | Attributes |
|---|---|---|
| `agentlens.parser.invocations.total` | Counter | `type`, `result`, `spec_version` |
| `agentlens.parser.duration` | Histogram | `type`, `result` |

### 4. Store tracing — `internal/telemetry/storetracer.go`

Decorator in `internal/telemetry/storetracer.go`. Defines its own interface matching the 6 traced methods (Go structural typing — no import of `internal/store/` needed). Wired in `main.go`: `tracedStore := telemetry.NewTracedStore(catalogStore, dbDialect)`. Store package never imports OTel.

| Span name | When |
|---|---|
| `store.catalog.create` | POST catalog entry |
| `store.catalog.get` | GET catalog entry by ID |
| `store.catalog.list` | GET catalog list |
| `store.catalog.update_health` | Health probe update |
| `store.skills.list` | Skill aggregation |
| `store.skills.list_agents` | Skill detail query |

Attributes: `db.system` = `sqlite` or `postgresql`, `db.operation`, `agentlens.store.result_count`.

### 5. Authentication events

On `POST /api/v1/auth/login`:

```go
span.AddEvent("auth.login", trace.WithAttributes(
    attribute.String("username", username),
    attribute.String("result", "success"),
    attribute.String("reason", ""),
))
```

**Never record passwords or tokens as attributes.**

Metric: `agentlens.auth.logins.total` Counter with `result`, `reason`.

### 6. Catalog gauge

`agentlens.catalog.entries` UpDownCounter via async callback. Attributes: `protocol`, `state`. Queries `SELECT count(*) ... GROUP BY` at metrics interval.

---

## Part D: Application Endpoints & Routing

### Existing

`GET /healthz` — already in `router.go:44`. Keep as-is.

### New endpoints

| Endpoint | Purpose | Auth | Condition |
|---|---|---|---|
| `GET /readyz` | Readiness — `SELECT 1` against DB | None | Always |
| `GET /metrics` | Prometheus exposition format | None | `prometheus.enabled` |

### Route registration order

```
1. otelhttp.NewHandler() wrapper       ← NEW, outermost
2. RecoveryMiddleware
3. LoggerMiddleware
4. CORSMiddleware
5. RequestID
6. GET /healthz                         ← exists
7. GET /readyz                          ← NEW
8. GET /metrics                         ← NEW, conditional
9. GET /api/v1/telemetry/config         ← NEW, public (no auth, frontend config)
10. /api/v1/* routes (auth gated)
11. SPA fallback /*
```

### RouterDeps change

```go
type RouterDeps struct {
    // ... existing fields ...
    PromHandler  http.Handler // nil = /metrics not registered
}
```

### `/readyz` implementation

Calls `database.DB.DB()` → `sqlDB.PingContext(ctx)`. Returns 200 `{"status":"ok"}` or 503 `{"status":"error","reason":"database unreachable"}`.

### `/metrics` implementation

`telemetry.Provider.PromHandler` passed into `RouterDeps`. Router registers `r.Handle("/metrics", promHandler)` only when non-nil.

---

## Part E: Startup & Shutdown Sequence

### Startup order

```
1.  Load config
2.  Setup slog (stdout JSON — baseline)
3.  telemetry.Init(ctx, cfg.Telemetry, version)     ← NEW
4.  IF telemetry enabled: replace slog with fan-out  ← NEW
5.  defer provider.Shutdown(5s timeout)               ← NEW
6.  Open DB + migrations
7.  Bootstrap admin
8.  Init stores
9.  Init JWT
10. Kernel + plugin manager
11. Register + init + start plugins
12. Discovery manager
13. Router (with PromHandler in RouterDeps)           ← CHANGED
14. HTTP server (blocks on signal)
```

### Shutdown sequence

```
1. SIGTERM received → server.Start() returns
2. HTTP server drains (existing 30s timeout)
3. pm.StopAll() — stops health prober, plugins
4. cancel() context — stops discovery manager
5. provider.Shutdown(5s) — flushes traces/metrics/logs to collector  ← NEW
6. database close
```

Telemetry init BEFORE plugins. Telemetry shutdown AFTER plugins stop. All plugin operations generate spans that get flushed.

### Version string

```go
var version = "dev" // set by -ldflags "-X main.version=v0.x.x"
```

---

## Part F: Frontend Telemetry — `web/src/telemetry.ts`

### Dependencies

```
@opentelemetry/api
@opentelemetry/sdk-trace-web
@opentelemetry/instrumentation-fetch
@opentelemetry/exporter-trace-otlp-http
@opentelemetry/resources
@opentelemetry/semantic-conventions
```

### Init module

```typescript
export function initTelemetry(config: { endpoint: string; serviceName: string })
```

- Creates `WebTracerProvider` with OTLP/HTTP exporter
- Registers `FetchInstrumentation` — auto-instruments all `fetch()` calls
- Injects `traceparent` header on every API request to `/api/*`
- Resource attributes: `service.name`, `service.version`, `deployment.environment`

### Configuration delivery

Backend endpoint `GET /api/v1/telemetry/config` (public, no auth):

```json
{
  "enabled": true,
  "endpoint": "http://collector.example.com:4318/v1/traces",
  "serviceName": "agentlens-web"
}
```

When `telemetry.enabled=false`, returns `{"enabled": false}`. Frontend skips init.

### App integration

```typescript
const resp = await fetch('/api/v1/telemetry/config');
const cfg = await resp.json();
if (cfg.enabled) {
  const { initTelemetry } = await import('./telemetry');
  initTelemetry(cfg);
}
```

Dynamic import — zero JS overhead when disabled.

### Collector routing

Frontend sends traces directly to OTel collector via OTLP/HTTP. No backend proxy. Collector endpoint must be browser-reachable (CORS on collector side).

### Scope boundary

Fetch instrumentation only. No document-load, no user-interaction, no error tracking.

### End-to-end trace flow

```
Browser fetch(/api/v1/catalog)              [frontend span]
  └─ HTTP GET /api/v1/catalog               [otelhttp server span]
       └─ store.catalog.list                [store decorator span]

Browser fetch(/api/v1/catalog/{id}/probe)   [frontend span]
  └─ HTTP POST /catalog/{id}/probe          [otelhttp server span]
       └─ health.probe                      [prober span]
            └─ HTTP GET agent-endpoint      [otelhttp client span → agent]
```

---

## Part G: Helm Chart

### Chart structure

```
deploy/helm/agentlens/
  Chart.yaml
  values.yaml
  values.schema.json
  templates/
    _helpers.tpl
    deployment.yaml
    service.yaml
    serviceaccount.yaml
    configmap.yaml
    secret.yaml
    ingress.yaml                (conditional: ingress.enabled)
    gateway-httproute.yaml      (conditional: gateway.enabled)
    hpa.yaml                    (conditional: autoscaling.enabled)
    pdb.yaml                    (conditional: pdb.enabled)
    servicemonitor.yaml         (conditional: metrics.serviceMonitor.enabled)
    networkpolicy.yaml          (conditional: networkPolicy.enabled)
    pvc.yaml                    (conditional: database.dialect == "sqlite")
    tests/
      test-connection.yaml
  ci/
    ci-values.yaml
```

### Chart.yaml

```yaml
apiVersion: v2
name: agentlens
description: AI Agent Discovery Platform — Traefik for AI agents
type: application
version: 0.2.0
appVersion: "0.x.x"
home: https://github.com/PawelHaracz/Agentlens
sources:
  - https://github.com/PawelHaracz/Agentlens
maintainers:
  - name: Pawel Haracz
    url: https://github.com/PawelHaracz
keywords: [ai, agents, a2a, mcp, service-discovery, registry]
dependencies:
  - name: postgresql
    version: "~16.x"
    repository: https://charts.bitnami.com/bitnami
    condition: postgresql.enabled
```

### Dockerfile changes

- Tag: `gcr.io/distroless/base-debian12:nonroot` (UID 65532)
- Add `USER 65532` directive

### Security context

```yaml
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  fsGroup: 65532
  seccompProfile:
    type: RuntimeDefault

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: [ALL]
```

### Volume mounts

| Mount | Source | Why |
|---|---|---|
| `/tmp` | emptyDir | `readOnlyRootFilesystem` blocks Go's `os.TempDir()` |
| `/etc/agentlens/config.yaml` | ConfigMap | App config |
| `/data` | PVC (SQLite only) | DB file persistence |

### Init container (PostgreSQL mode)

```yaml
- name: wait-postgres
  image: busybox:1.36
  command: ['sh', '-c', 'until nc -z $DB_HOST $DB_PORT; do sleep 2; done']
```

Timeout via `activeDeadlineSeconds: 120`.

### Auto-toggle Prometheus

When `metrics.serviceMonitor.enabled=true`, deployment template auto-sets `AGENTLENS_METRICS_PROMETHEUS_ENABLED=true`.

### Multi-replica guard

`replicaCount > 1` + `database.dialect=sqlite` → `helm template` fails with clear message.

### Secret handling

- `admin-password`: `randAlphaNum 24` if not provided
- `database-password`: from `database.external.password` or subchart ref
- Never in configmap

### Probes

| Probe | Path | Purpose |
|---|---|---|
| Liveness | `/healthz` | Process alive. No DB check. |
| Readiness | `/readyz` | Can serve. DB reachable. |
| Startup | `/healthz` | 30 x 5s = 150s for slow migrations |

### values.yaml

Full values specification follows the structure defined in the original spec with these adjustments:
- `podSecurityContext.runAsUser`: 65532 (not 65534)
- `podSecurityContext.runAsGroup`: 65532
- `podSecurityContext.fsGroup`: 65532
- `telemetry.logExportLevel`: `info` (new field)

All other values as specified in the original spec's values.yaml section.

---

## Part H: Testing Strategy

### Unit tests — `internal/telemetry/`

| # | Test | Assertion |
|---|---|---|
| 1 | Init disabled | No-op provider, nil PromHandler, no error |
| 2 | Init enabled, empty endpoint | No-op provider, warning logged |
| 3 | Init enabled, valid config | Non-nil TracerProvider, MeterProvider, LoggerProvider |
| 4 | Init with Prometheus enabled | PromHandler non-nil |
| 5 | Shutdown | Flushes without error (in-memory exporter) |
| 6 | Fan-out slog handler | Logs appear in both stdout + OTLP; logExportLevel filters OTLP |

### Instrumentation tests — in-memory exporter

| # | Test | Assertion |
|---|---|---|
| 7 | HTTP middleware | Request → span with correct method, path, status |
| 8 | Health probe success | Span `health.probe` with `result=success`, counter incremented |
| 9 | Health probe state transition | Span event `state_transition` with from/to |
| 10 | Parser success (A2A) | Span `parser.a2a.parse` with spec_version, skill_count |
| 11 | Parser failure | Span has error, counter `result=error` incremented |
| 12 | Store decorator | Span `store.catalog.list` with `db.system` |
| 13 | Auth login event | Span event `auth.login`, no password in attributes |
| 14 | Catalog gauge | 3 seeded entries → correct counts per protocol/state |
| 15 | Prometheus endpoint | GET `/metrics` → 200, contains `agentlens_health_probes_total` |

### Frontend tests — Vitest

| # | Test | Assertion |
|---|---|---|
| 16 | Config disabled | No OTel init, no fetch instrumentation |
| 17 | Config enabled | TracerProvider created, fetch instrumented |
| 18 | Dynamic import | OTel packages not loaded when disabled |

### Helm lint + template tests

| # | Test | Assertion |
|---|---|---|
| 19 | Default values | `helm lint` zero warnings |
| 20 | SQLite mode | PVC rendered, no PostgreSQL subchart, replicaCount: 1 |
| 21 | PostgreSQL subchart | StatefulSet + init container, no PVC |
| 22 | External PostgreSQL | External DB env, no subchart |
| 23 | Ingress | Correct hosts/TLS |
| 24 | Gateway API | HTTPRoute with parentRefs |
| 25 | HPA | Correct targets |
| 26 | PDB | minAvailable: 1 |
| 27 | ServiceMonitor | Correct labels, endpoint, auto-enables Prometheus env |
| 28 | NetworkPolicy | DNS egress allowed |
| 29 | Security context | runAsNonRoot, readOnlyRootFilesystem, drop ALL |
| 30 | Schema validation | `helm lint --strict` catches bad values |
| 31 | Multi-replica guard | replicaCount: 3 + sqlite → template fails |

### Integration test — Docker Compose + Jaeger

| # | Test | Assertion |
|---|---|---|
| 32 | End-to-end traces | AgentLens + Jaeger. Register agent, probe, query. Jaeger API returns traces with expected spans |

### E2E (Playwright) — OTel smoke

| # | Test | Assertion |
|---|---|---|
| 33 | OTLP receiver | AgentLens + mock OTLP receiver. Verify receiver got trace + metrics batches |

---

## Part I: Acceptance Criteria

### Observability

1. `AGENTLENS_OTEL_ENABLED=true AGENTLENS_OTEL_ENDPOINT=localhost:4317` → traces visible in Jaeger within 30s
2. `AGENTLENS_OTEL_PROTOCOL=http` switches to OTLP/HTTP
3. `AGENTLENS_OTEL_ENABLED=false` (default) → zero overhead, no connections
4. Incoming `traceparent` header → used as parent span → full distributed trace
5. Health probe spans include latency, result, state transition events
6. Parser spans include spec version, skill count, error recording
7. `agentlens.catalog.entries` gauge accurate by protocol and state
8. `slog` output includes `trace_id`/`span_id` when enabled
9. Logs fan-out to stdout AND OTLP; `logExportLevel` filters OTLP side
10. `GET /metrics` returns Prometheus exposition format with all OTel metrics
11. SIGTERM → telemetry flushed within 5s
12. Falls back to `OTEL_EXPORTER_OTLP_ENDPOINT` when `AGENTLENS_OTEL_ENDPOINT` not set
13. No passwords or tokens in any span attribute or log record
14. Frontend fetch calls carry `traceparent` → connected to backend traces

### Helm chart

15. `helm install` with defaults → working SQLite deployment
16. `helm install --set database.dialect=postgres --set postgresql.enabled=true` → working PostgreSQL deployment
17. External PostgreSQL mode works
18. `helm test` passes
19. Pod runs as non-root (UID 65532) with read-only rootfs
20. PDB prevents draining last pod
21. Ingress and Gateway API render correctly
22. ServiceMonitor scrapes `/metrics` successfully
23. `replicaCount > 1` + SQLite → fails with clear error
24. `helm lint --strict` passes
25. Init container prevents CrashLoopBackOff on slow PostgreSQL

---

## Part J: Known Traps

1. **Do not import OTel packages in `internal/store/`.** Use decorator in `telemetry`.
2. **Do not instrument every SQL query.** Only the 6 key operations.
3. **Do not record passwords, tokens, or secrets as span attributes.**
4. **Do not use `AlwaysOnSampler` as default.** Use `ParentBased(TraceIDRatioBased(1.0))`.
5. **Do not start exporter goroutines when telemetry is disabled.**
6. **Do not block shutdown on telemetry flush.** 5-second context timeout.
7. **Do not add OTel as a microkernel plugin.** Infrastructure, not plugin. See ADR-009.
8. **Do not gate PostgreSQL behind enterprise license.**
9. **Do not hardcode the image tag.** Default to `Chart.appVersion`.
10. **Do not use `latest` tag anywhere.**
11. **Do not add database checks to liveness probe.** Liveness = process alive.
12. **Do not skip init container for PostgreSQL.**
13. **Do not forget `/tmp` emptyDir mount.** `readOnlyRootFilesystem` blocks writes.
14. **Do not use `helm.sh/hook` for database migrations.** AgentLens runs migrations on startup.
15. **Do not put passwords in `configmap.yaml`.** Passwords → `secret.yaml` → `secretKeyRef`.
16. **Do not use deprecated Ingress API.** `networking.k8s.io/v1` only.
17. **Do not replace slog default with OTel bridge exclusively.** Fan-out to both. See ADR-010.
18. **Do not use global `otel.Tracer()` inside hot loops.** Trace at operation boundaries.
19. **Do not gate observability behind enterprise license.** OSS Core.

---

## Part K: ADRs

Two new ADRs required:

- **ADR-009: OpenTelemetry as Infrastructure, Not Plugin** — OTel lives in `internal/telemetry/` (infrastructure layer), initialized in `main` before plugins, shutdown after. Not a microkernel plugin despite ADR-003 establishing plugins as the extension mechanism.

- **ADR-010: Dual-Output Structured Logging with OTel Bridge** — Logs fan-out to stdout (JSON, all levels) AND OTLP bridge (filtered by `logExportLevel`). Stdout never replaced. Operators depend on `kubectl logs` always working.

---

## Go Dependencies

```
go.opentelemetry.io/otel                         v1.28+
go.opentelemetry.io/otel/sdk                     v1.28+
go.opentelemetry.io/otel/sdk/metric              v1.28+
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc
go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp
go.opentelemetry.io/otel/exporters/prometheus
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
go.opentelemetry.io/otel/bridge/otelslog
github.com/prometheus/client_golang
```

Pin exact versions. Do not use `latest`.

## Frontend Dependencies

```
@opentelemetry/api
@opentelemetry/sdk-trace-web
@opentelemetry/instrumentation-fetch
@opentelemetry/exporter-trace-otlp-http
@opentelemetry/resources
@opentelemetry/semantic-conventions
```

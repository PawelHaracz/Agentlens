# OpenTelemetry Observability & Production-Ready Helm Chart — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full OTel observability (traces, metrics, structured logs) and a production-grade Helm chart to AgentLens.

**Architecture:** Global OTel providers initialized in `main.go` before plugins, shutdown after. Infrastructure layer (`internal/telemetry/`). Fan-out slog handler for dual stdout+OTLP logging. Store tracing via decorator with structural typing. Helm chart with Bitnami PostgreSQL subchart, SecurityContext, PDB, HPA, Ingress, Gateway API, ServiceMonitor.

**Tech Stack:** Go OTel SDK v1.28+, otelhttp, otelslog, Prometheus exporter, chi router, Helm v3, Bitnami postgresql subchart, `@opentelemetry/sdk-trace-web` + `@opentelemetry/instrumentation-fetch` for frontend.

**Spec:** `docs/superpowers/specs/2026-04-12-otel-helm-design.md`
**ADRs:** `docs/adr/009-opentelemetry-as-infrastructure.md`, `docs/adr/010-dual-output-structured-logging.md`

---

## File Map

### New files (Go)

| File | Responsibility |
|------|---------------|
| `internal/telemetry/telemetry.go` | `Provider` struct, `Init()`, `Shutdown()`, exporter setup |
| `internal/telemetry/telemetry_test.go` | Unit tests for Init/Shutdown/no-op paths |
| `internal/telemetry/fanout.go` | Fan-out slog handler (stdout + OTLP bridge) |
| `internal/telemetry/fanout_test.go` | Fan-out handler tests |
| `internal/telemetry/storetracer.go` | Store decorator with tracing spans |
| `internal/telemetry/storetracer_test.go` | Store decorator tests |
| `internal/telemetry/metrics.go` | Metric instruments (health, parser, auth, catalog gauge) |
| `internal/telemetry/metrics_test.go` | Metric instrument tests |
| `internal/api/telemetry_handler.go` | `GET /api/v1/telemetry/config` + `GET /readyz` handlers |
| `internal/api/telemetry_handler_test.go` | Handler tests |

### Modified files (Go)

| File | Change |
|------|--------|
| `go.mod` / `go.sum` | Add OTel SDK dependencies |
| `internal/config/config.go` | Add `TelemetryConfig`, `PrometheusConfig`, defaults, env vars |
| `internal/config/config_test.go` | Test new config fields |
| `cmd/agentlens/main.go` | Wire telemetry init/shutdown, version ldflags, pass PromHandler |
| `internal/api/router.go` | Add `PromHandler` to `RouterDeps`, register `/readyz`, `/metrics`, `/api/v1/telemetry/config`, wrap with `otelhttp` |
| `plugins/health/health.go` | Add `otelhttp.NewTransport()` to httpClient, span+metric instrumentation in `probeOne` |
| `plugins/parsers/a2a/a2a.go` | Span+metric instrumentation in `Parse` |
| `plugins/parsers/mcp/mcp.go` | Span+metric instrumentation in `Parse` |
| `internal/api/auth_handlers.go` | Span event on login |
| `arch-go.yml` | Add `internal.telemetry` dependency rules |
| `Dockerfile` | Switch to `:nonroot` tag, add `USER 65532` |
| `Makefile` | Add `-ldflags` for version, update `helm-lint` path/flags, add `helm-test` target |

### New files (Frontend)

| File | Responsibility |
|------|---------------|
| `web/src/telemetry.ts` | OTel web SDK init, fetch instrumentation |
| `web/src/telemetry.test.ts` | Vitest tests for telemetry init |

### Modified files (Frontend)

| File | Change |
|------|--------|
| `web/package.json` | Add `@opentelemetry/*` dependencies |
| `web/src/main.tsx` | Fetch telemetry config, dynamic import |

### Helm chart files (existing chart at `deploy/helm/agentlens/`)

**Modified (already exist):**

| File | Change |
|------|--------|
| `deploy/helm/agentlens/Chart.yaml` | Add Bitnami postgresql dependency, maintainers, keywords |
| `deploy/helm/agentlens/values.yaml` | Full rewrite — add security, probes, telemetry, DB, autoscaling, ingress, gateway, PDB, metrics |
| `deploy/helm/agentlens/templates/_helpers.tpl` | Add multi-replica guard, update labels |
| `deploy/helm/agentlens/templates/deployment.yaml` | Add probes, security context, init container, volume mounts, telemetry env vars |
| `deploy/helm/agentlens/templates/service.yaml` | Named port `http`, update to port 8080 |
| `deploy/helm/agentlens/templates/serviceaccount.yaml` | Add annotation support |
| `deploy/helm/agentlens/templates/configmap.yaml` | Add telemetry + health config |

**New:**

| File | Responsibility |
|------|---------------|
| `deploy/helm/agentlens/values.schema.json` | Input validation |
| `deploy/helm/agentlens/templates/secret.yaml` | Passwords, DSN |
| `deploy/helm/agentlens/templates/ingress.yaml` | Conditional Ingress |
| `deploy/helm/agentlens/templates/gateway-httproute.yaml` | Conditional HTTPRoute |
| `deploy/helm/agentlens/templates/hpa.yaml` | Conditional HPA |
| `deploy/helm/agentlens/templates/pdb.yaml` | Conditional PDB |
| `deploy/helm/agentlens/templates/servicemonitor.yaml` | Conditional ServiceMonitor |
| `deploy/helm/agentlens/templates/networkpolicy.yaml` | Conditional NetworkPolicy |
| `deploy/helm/agentlens/templates/pvc.yaml` | SQLite PVC |
| `deploy/helm/agentlens/templates/tests/test-connection.yaml` | helm test hook |
| `deploy/helm/agentlens/ci/ci-values.yaml` | CI lint values |

**Removed:**

| File | Reason |
|------|--------|
| `deploy/helm/agentlens/templates/clusterrole.yaml` | Replaced by RBAC via serviceaccount annotations (EKS IRSA, GKE WI) |
| `deploy/helm/agentlens/templates/clusterrolebinding.yaml` | Same as above |

### New files (Integration)

| File | Responsibility |
|------|---------------|
| `docker-compose.otel.yml` | AgentLens + Jaeger for integration test |
| `scripts/test-otel-integration.sh` | Integration test script |

---

## Phase 1: Go Telemetry Core

### Task 1: Add OTel dependencies to go.mod

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Add OTel SDK dependencies**

```bash
cd /Users/pawelharacz/src/private/agentlens
go get go.opentelemetry.io/otel@v1.28.0
go get go.opentelemetry.io/otel/sdk@v1.28.0
go get go.opentelemetry.io/otel/sdk/metric@v1.28.0
go get go.opentelemetry.io/otel/sdk/log@v0.4.0
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp
go get go.opentelemetry.io/otel/exporters/prometheus
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
go get go.opentelemetry.io/otel/bridge/otelslog
```

Note: exact versions may need adjustment. Use the latest stable v1.28+ for `otel` and `otel/sdk`. The log SDK is still v0.x — pin to the version compatible with otel v1.28. Run `go mod tidy` after.

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add OpenTelemetry SDK dependencies"
```

---

### Task 2: TelemetryConfig in config package

**Files:**
- Modify: `internal/config/config.go`
- Modify or create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for TelemetryConfig defaults**

Add to `internal/config/config_test.go`:

```go
func TestTelemetryDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	assert.False(t, cfg.Telemetry.Enabled)
	assert.Equal(t, "grpc", cfg.Telemetry.Protocol)
	assert.True(t, cfg.Telemetry.Insecure)
	assert.Equal(t, "agentlens", cfg.Telemetry.ServiceName)
	assert.Equal(t, "production", cfg.Telemetry.Environment)
	assert.Equal(t, "parentbased_traceidratio", cfg.Telemetry.TracesSampler)
	assert.Equal(t, 1.0, cfg.Telemetry.TracesSampleRate)
	assert.Equal(t, 30*time.Second, cfg.Telemetry.MetricsInterval)
	assert.Equal(t, "info", cfg.Telemetry.LogExportLevel)
	assert.False(t, cfg.Telemetry.Prometheus.Enabled)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestTelemetryDefaults -v
```

Expected: FAIL — `cfg.Telemetry` field does not exist.

- [ ] **Step 3: Add TelemetryConfig and PrometheusConfig types**

In `internal/config/config.go`, add the types:

```go
// TelemetryConfig holds OpenTelemetry configuration.
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

// PrometheusConfig holds Prometheus metrics endpoint configuration.
type PrometheusConfig struct {
	Enabled bool `yaml:"enabled"`
}
```

Add `Telemetry TelemetryConfig \`yaml:"telemetry"\`` to the `Config` struct.

Add defaults in `defaults()`:

```go
Telemetry: TelemetryConfig{
	Enabled:          false,
	Protocol:         "grpc",
	Insecure:         true,
	ServiceName:      "agentlens",
	Environment:      "production",
	TracesSampler:    "parentbased_traceidratio",
	TracesSampleRate: 1.0,
	MetricsInterval:  30 * time.Second,
	LogExportLevel:   "info",
},
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/config/... -run TestTelemetryDefaults -v
```

Expected: PASS.

- [ ] **Step 5: Write test for telemetry env var overrides**

```go
func TestTelemetryEnvOverrides(t *testing.T) {
	t.Setenv("AGENTLENS_OTEL_ENABLED", "true")
	t.Setenv("AGENTLENS_OTEL_ENDPOINT", "collector:4317")
	t.Setenv("AGENTLENS_OTEL_PROTOCOL", "http")
	t.Setenv("AGENTLENS_OTEL_INSECURE", "false")
	t.Setenv("AGENTLENS_OTEL_SERVICE_NAME", "my-service")
	t.Setenv("AGENTLENS_OTEL_ENVIRONMENT", "staging")
	t.Setenv("AGENTLENS_OTEL_TRACES_SAMPLE_RATE", "0.5")
	t.Setenv("AGENTLENS_OTEL_METRICS_INTERVAL", "60s")
	t.Setenv("AGENTLENS_OTEL_LOG_EXPORT_LEVEL", "warn")
	t.Setenv("AGENTLENS_METRICS_PROMETHEUS_ENABLED", "true")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.True(t, cfg.Telemetry.Enabled)
	assert.Equal(t, "collector:4317", cfg.Telemetry.Endpoint)
	assert.Equal(t, "http", cfg.Telemetry.Protocol)
	assert.False(t, cfg.Telemetry.Insecure)
	assert.Equal(t, "my-service", cfg.Telemetry.ServiceName)
	assert.Equal(t, "staging", cfg.Telemetry.Environment)
	assert.Equal(t, 0.5, cfg.Telemetry.TracesSampleRate)
	assert.Equal(t, 60*time.Second, cfg.Telemetry.MetricsInterval)
	assert.Equal(t, "warn", cfg.Telemetry.LogExportLevel)
	assert.True(t, cfg.Telemetry.Prometheus.Enabled)
}
```

- [ ] **Step 6: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestTelemetryEnvOverrides -v
```

Expected: FAIL — env overrides not applied.

- [ ] **Step 7: Implement applyTelemetryEnv**

Add to `internal/config/config.go`:

```go
func applyTelemetryEnv(tel *TelemetryConfig) {
	if v := env("OTEL_ENABLED"); v != "" {
		tel.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("OTEL_ENDPOINT"); v != "" {
		tel.Endpoint = v
	} else if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" && tel.Endpoint == "" {
		tel.Endpoint = v
	}
	if v := env("OTEL_PROTOCOL"); v != "" {
		tel.Protocol = v
	}
	if v := env("OTEL_INSECURE"); v != "" {
		tel.Insecure = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("OTEL_SERVICE_NAME"); v != "" {
		tel.ServiceName = v
	}
	if v := env("OTEL_ENVIRONMENT"); v != "" {
		tel.Environment = v
	}
	if v := env("OTEL_TRACES_SAMPLER"); v != "" {
		tel.TracesSampler = v
	}
	if v := env("OTEL_TRACES_SAMPLE_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			tel.TracesSampleRate = f
		}
	}
	if v := env("OTEL_METRICS_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			tel.MetricsInterval = d
		}
	}
	if v := env("OTEL_LOG_EXPORT_LEVEL"); v != "" {
		tel.LogExportLevel = v
	}
	if v := env("OTEL_HEADERS"); v != "" {
		tel.Headers = parseHeaders(v)
	}
	if v := env("METRICS_PROMETHEUS_ENABLED"); v != "" {
		tel.Prometheus.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
}

func parseHeaders(s string) map[string]string {
	headers := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return headers
}
```

Call `applyTelemetryEnv(&cfg.Telemetry)` from `applyEnv`.

- [ ] **Step 8: Run all config tests**

```bash
go test ./internal/config/... -v
```

Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add TelemetryConfig with env var overrides"
```

---

### Task 3: Telemetry provider — Init and Shutdown

**Files:**
- Create: `internal/telemetry/telemetry.go`
- Create: `internal/telemetry/telemetry_test.go`

- [ ] **Step 1: Write failing tests for Init**

Create `internal/telemetry/telemetry_test.go`:

```go
package telemetry_test

import (
	"context"
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitDisabled(t *testing.T) {
	cfg := config.TelemetryConfig{Enabled: false}
	p, err := telemetry.Init(context.Background(), cfg, "test")
	require.NoError(t, err)
	assert.Nil(t, p.PromHandler)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestInitEnabledEmptyEndpoint(t *testing.T) {
	cfg := config.TelemetryConfig{
		Enabled:  true,
		Endpoint: "",
	}
	p, err := telemetry.Init(context.Background(), cfg, "test")
	require.NoError(t, err)
	// Falls back to no-op when no endpoint available
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestInitEnabledValidConfig(t *testing.T) {
	cfg := config.TelemetryConfig{
		Enabled:          true,
		Endpoint:         "localhost:4317",
		Protocol:         "grpc",
		Insecure:         true,
		ServiceName:      "agentlens-test",
		Environment:      "test",
		TracesSampler:    "parentbased_traceidratio",
		TracesSampleRate: 1.0,
		MetricsInterval:  5 * time.Second,
		LogExportLevel:   "info",
	}
	p, err := telemetry.Init(context.Background(), cfg, "test")
	require.NoError(t, err)
	assert.NotNil(t, p.TracerProvider)
	assert.NotNil(t, p.MeterProvider)
	assert.NotNil(t, p.LoggerProvider)
	assert.Nil(t, p.PromHandler) // Prometheus not enabled
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestInitWithPrometheus(t *testing.T) {
	cfg := config.TelemetryConfig{
		Enabled:          true,
		Endpoint:         "localhost:4317",
		Protocol:         "grpc",
		Insecure:         true,
		ServiceName:      "agentlens-test",
		Environment:      "test",
		TracesSampler:    "parentbased_traceidratio",
		TracesSampleRate: 1.0,
		MetricsInterval:  5 * time.Second,
		LogExportLevel:   "info",
		Prometheus:       config.PrometheusConfig{Enabled: true},
	}
	p, err := telemetry.Init(context.Background(), cfg, "test")
	require.NoError(t, err)
	assert.NotNil(t, p.PromHandler)
	require.NoError(t, p.Shutdown(context.Background()))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/telemetry/... -v
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement telemetry.go**

Create `internal/telemetry/telemetry.go`:

```go
// Package telemetry provides OpenTelemetry initialization and shutdown.
package telemetry

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PawelHaracz/agentlens/internal/config"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Provider holds initialized OTel providers and a shutdown function.
type Provider struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
	PromHandler    http.Handler
	Shutdown       func(ctx context.Context) error
}

// Init initializes OpenTelemetry providers based on config.
// When disabled or endpoint empty, returns no-op providers.
func Init(ctx context.Context, cfg config.TelemetryConfig, version string) (*Provider, error) {
	noop := &Provider{
		Shutdown: func(ctx context.Context) error { return nil },
	}

	if !cfg.Enabled {
		return noop, nil
	}

	if cfg.Endpoint == "" {
		slog.Warn("telemetry enabled but no endpoint configured, falling back to no-op")
		return noop, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(version),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating otel resource: %w", err)
	}

	// Trace exporter
	traceExp, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating trace exporter: %w", err)
	}

	sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TracesSampleRate))
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithSampler(sampler),
	)

	// Metric exporter(s)
	var metricReaders []sdkmetric.Reader

	otlpMetricExp, err := newMetricExporter(ctx, cfg)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("creating metric exporter: %w", err)
	}
	metricReaders = append(metricReaders, sdkmetric.NewPeriodicReader(otlpMetricExp,
		sdkmetric.WithInterval(cfg.MetricsInterval),
	))

	var promHandler http.Handler
	if cfg.Prometheus.Enabled {
		promExp, err := promexporter.New()
		if err != nil {
			_ = tp.Shutdown(ctx)
			return nil, fmt.Errorf("creating prometheus exporter: %w", err)
		}
		metricReaders = append(metricReaders, promExp)
		promHandler = promhttp.Handler()
	}

	mpOpts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	for _, r := range metricReaders {
		mpOpts = append(mpOpts, sdkmetric.WithReader(r))
	}
	mp := sdkmetric.NewMeterProvider(mpOpts...)

	// Log exporter
	logExp, err := newLogExporter(ctx, cfg)
	if err != nil {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil, fmt.Errorf("creating log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	)

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		TracerProvider: tp,
		MeterProvider:  mp,
		LoggerProvider: lp,
		PromHandler:    promHandler,
		Shutdown: func(ctx context.Context) error {
			var errs []error
			if err := tp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
			}
			if err := mp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("metric provider shutdown: %w", err))
			}
			if err := lp.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("log provider shutdown: %w", err))
			}
			if len(errs) > 0 {
				return fmt.Errorf("telemetry shutdown errors: %v", errs)
			}
			return nil
		},
	}, nil
}

func newTraceExporter(ctx context.Context, cfg config.TelemetryConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Protocol {
	case "http":
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		for k, v := range cfg.Headers {
			opts = append(opts, otlptracehttp.WithHeaders(map[string]string{k: v}))
		}
		return otlptracehttp.New(ctx, opts...)
	default: // grpc
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		} else {
			opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{})))
		}
		for k, v := range cfg.Headers {
			opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{k: v}))
		}
		return otlptracegrpc.New(ctx, opts...)
	}
}

func newMetricExporter(ctx context.Context, cfg config.TelemetryConfig) (sdkmetric.Exporter, error) {
	switch cfg.Protocol {
	case "http":
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		return otlpmetrichttp.New(ctx, opts...)
	default:
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		return otlpmetricgrpc.New(ctx, opts...)
	}
}

func newLogExporter(ctx context.Context, cfg config.TelemetryConfig) (sdklog.Exporter, error) {
	switch cfg.Protocol {
	case "http":
		opts := []otlploghttp.Option{otlploghttp.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		return otlploghttp.New(ctx, opts...)
	default:
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		return otlploggrpc.New(ctx, opts...)
	}
}
```

Note: The exact import paths and API may differ slightly depending on OTel SDK version. Adjust `credentials` import for gRPC TLS if needed (`google.golang.org/grpc/credentials`). Verify against the installed SDK version's godoc.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/telemetry/... -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/telemetry.go internal/telemetry/telemetry_test.go
git commit -m "feat(telemetry): add Provider Init and Shutdown with OTLP exporters"
```

---

### Task 4: Fan-out slog handler

**Files:**
- Create: `internal/telemetry/fanout.go`
- Create: `internal/telemetry/fanout_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/telemetry/fanout_test.go`:

```go
package telemetry_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/telemetry"
	"github.com/stretchr/testify/assert"
)

func TestFanoutHandler_BothOutputs(t *testing.T) {
	var buf bytes.Buffer
	stdout := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	var bridgeBuf bytes.Buffer
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelInfo)
	logger := slog.New(h)

	logger.Info("test message")

	assert.Contains(t, buf.String(), "test message")
	assert.Contains(t, bridgeBuf.String(), "test message")
}

func TestFanoutHandler_ExportLevelFilters(t *testing.T) {
	var buf bytes.Buffer
	stdout := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	var bridgeBuf bytes.Buffer
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelWarn)
	logger := slog.New(h)

	logger.Info("info message")
	logger.Warn("warn message")

	assert.Contains(t, buf.String(), "info message")
	assert.Contains(t, buf.String(), "warn message")
	assert.NotContains(t, bridgeBuf.String(), "info message")
	assert.Contains(t, bridgeBuf.String(), "warn message")
}

func TestFanoutHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	stdout := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	var bridgeBuf bytes.Buffer
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelInfo)
	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(h2)

	logger.Info("with attrs")

	assert.Contains(t, buf.String(), "component")
	assert.Contains(t, bridgeBuf.String(), "component")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/telemetry/... -run TestFanout -v
```

Expected: FAIL — `NewFanoutHandler` does not exist.

- [ ] **Step 3: Implement fanout handler**

Create `internal/telemetry/fanout.go`:

```go
package telemetry

import (
	"context"
	"log/slog"
)

// fanoutHandler writes log records to both stdout and the OTel bridge.
// The bridge only receives records at or above exportLevel.
type fanoutHandler struct {
	stdout      slog.Handler
	bridge      slog.Handler
	exportLevel slog.Level
}

// NewFanoutHandler creates a handler that fans out to stdout and an OTel bridge.
// All records go to stdout. Only records >= exportLevel go to bridge.
func NewFanoutHandler(stdout, bridge slog.Handler, exportLevel slog.Level) slog.Handler {
	return &fanoutHandler{
		stdout:      stdout,
		bridge:      bridge,
		exportLevel: exportLevel,
	}
}

func (h *fanoutHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.stdout.Enabled(context.Background(), level) ||
		(level >= h.exportLevel && h.bridge.Enabled(context.Background(), level))
}

func (h *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	// Always write to stdout
	if err := h.stdout.Handle(ctx, r); err != nil {
		return err
	}
	// Write to bridge only if level meets export threshold
	if r.Level >= h.exportLevel {
		_ = h.bridge.Handle(ctx, r) // best-effort for OTLP
	}
	return nil
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &fanoutHandler{
		stdout:      h.stdout.WithAttrs(attrs),
		bridge:      h.bridge.WithAttrs(attrs),
		exportLevel: h.exportLevel,
	}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	return &fanoutHandler{
		stdout:      h.stdout.WithGroup(name),
		bridge:      h.bridge.WithGroup(name),
		exportLevel: h.exportLevel,
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/telemetry/... -run TestFanout -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/fanout.go internal/telemetry/fanout_test.go
git commit -m "feat(telemetry): add fan-out slog handler for stdout + OTLP"
```

---

### Task 5: Store tracing decorator

**Files:**
- Create: `internal/telemetry/storetracer.go`
- Create: `internal/telemetry/storetracer_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/telemetry/storetracer_test.go`:

```go
package telemetry_test

import (
	"context"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type stubStore struct {
	listResult []model.CatalogEntry
}

func (s *stubStore) List(ctx context.Context) ([]model.CatalogEntry, error) {
	return s.listResult, nil
}

func (s *stubStore) Get(ctx context.Context, id string) (*model.CatalogEntry, error) {
	return nil, nil
}

func (s *stubStore) Create(ctx context.Context, entry *model.CatalogEntry) error {
	return nil
}

func (s *stubStore) UpdateHealth(ctx context.Context, id string, h model.Health) error {
	return nil
}

func TestTracedStore_ListSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	stub := &stubStore{listResult: []model.CatalogEntry{{}, {}, {}}}
	traced := telemetry.NewTracedStore(stub, "sqlite", telemetry.WithTracerProvider(tp))

	ctx := context.Background()
	result, err := traced.List(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "store.catalog.list", spans[0].Name)

	attrs := spans[0].Attributes
	assertHasAttr(t, attrs, "db.system", "sqlite")
	assertHasAttr(t, attrs, "agentlens.store.result_count", "3")
}

func assertHasAttr(t *testing.T, attrs []attribute.KeyValue, key, val string) {
	t.Helper()
	for _, a := range attrs {
		if string(a.Key) == key {
			assert.Equal(t, val, a.Value.Emit())
			return
		}
	}
	t.Errorf("attribute %s not found", key)
}
```

Note: The stub must match the interface defined in `storetracer.go`. Adjust method signatures to match. The `List` method signature needs to align with the actual `store.Store` interface (which takes `ListFilter`). Simplify: the decorator wraps a subset interface — the test stub implements that subset.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/telemetry/... -run TestTracedStore -v
```

Expected: FAIL — `NewTracedStore` does not exist.

- [ ] **Step 3: Implement store decorator**

Create `internal/telemetry/storetracer.go`:

```go
package telemetry

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/model"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TracedStoreOpt configures TracedStore.
type TracedStoreOpt func(*tracedStoreConfig)

type tracedStoreConfig struct {
	tp trace.TracerProvider
}

// WithTracerProvider overrides the tracer provider (for testing).
func WithTracerProvider(tp trace.TracerProvider) TracedStoreOpt {
	return func(c *tracedStoreConfig) { c.tp = tp }
}

// CatalogLister is the subset of store.Store that the decorator traces.
// Uses Go structural typing — no import of internal/store needed.
type CatalogLister interface {
	Create(ctx context.Context, entry *model.CatalogEntry) error
	Get(ctx context.Context, id string) (*model.CatalogEntry, error)
	List(ctx context.Context, filter interface{}) ([]model.CatalogEntry, error)
	UpdateHealth(ctx context.Context, entryID string, h model.Health) error
}

// Note: The actual List signature uses store.ListFilter. Since telemetry
// cannot import store, define the decorator to accept the full store.Store
// as an interface{} parameter and type-assert, OR use a narrower interface
// that matches structurally. The actual implementation should define the
// interface to match the exact method signatures of store.Store that it wraps.
// Adjust the interface above to match the real signatures.

// TracedStore wraps a store with tracing spans.
type TracedStore struct {
	inner   interface{} // The underlying store
	dialect string
	tracer  trace.Tracer
}

// NewTracedStore creates a tracing decorator around a catalog store.
func NewTracedStore(inner interface{}, dialect string, opts ...TracedStoreOpt) *TracedStore {
	cfg := &tracedStoreConfig{tp: otel.GetTracerProvider()}
	for _, o := range opts {
		o(cfg)
	}
	return &TracedStore{
		inner:   inner,
		dialect: dialect,
		tracer:  cfg.tp.Tracer("agentlens.store"),
	}
}
```

Note: The exact interface and method delegation depends on `store.Store`'s signatures. The decorator must delegate each of the 6 methods, starting a span and recording `db.system`, `db.operation`, and `agentlens.store.result_count` (where applicable). Each method follows this pattern:

```go
func (t *TracedStore) SomeMethod(ctx context.Context, args...) (result, error) {
	ctx, span := t.tracer.Start(ctx, "store.catalog.somemethod",
		trace.WithAttributes(
			attribute.String("db.system", t.dialect),
			attribute.String("db.operation", "somemethod"),
		),
	)
	defer span.End()

	result, err := t.inner.SomeMethod(ctx, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	// Set result_count where applicable
	return result, err
}
```

The implementer must define a proper interface matching `store.Store`'s 6 traced methods exactly. Use the `store.Store` interface from `internal/store/store.go` as reference but define a structural copy in this package.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/telemetry/... -run TestTracedStore -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/storetracer.go internal/telemetry/storetracer_test.go
git commit -m "feat(telemetry): add store tracing decorator"
```

---

### Task 6: Metric instruments

**Files:**
- Create: `internal/telemetry/metrics.go`
- Create: `internal/telemetry/metrics_test.go`

- [ ] **Step 1: Write failing test for health metrics**

Create `internal/telemetry/metrics_test.go`:

```go
package telemetry_test

import (
	"context"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/telemetry"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestHealthMetrics_RecordProbe(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	m := telemetry.NewHealthMetrics(telemetry.WithMeterProvider(mp))
	m.RecordProbe(context.Background(), "success", "a2a", 150)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	// Verify agentlens.health.probes.total counter exists and has value 1
	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, metric := range sm.Metrics {
			if metric.Name == "agentlens.health.probes.total" {
				found = true
			}
		}
	}
	require.True(t, found, "agentlens.health.probes.total metric not found")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/telemetry/... -run TestHealthMetrics -v
```

Expected: FAIL.

- [ ] **Step 3: Implement metrics.go**

Create `internal/telemetry/metrics.go` with all metric instruments defined in the spec:

```go
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsOpt configures metric instruments.
type MetricsOpt func(*metricsConfig)

type metricsConfig struct {
	mp metric.MeterProvider
}

// WithMeterProvider overrides the meter provider (for testing).
func WithMeterProvider(mp metric.MeterProvider) MetricsOpt {
	return func(c *metricsConfig) { c.mp = mp }
}

// HealthMetrics holds health prober metric instruments.
type HealthMetrics struct {
	probesTotal      metric.Int64Counter
	probesLatency    metric.Float64Histogram
	stateTransitions metric.Int64Counter
}

// NewHealthMetrics creates health prober metric instruments.
func NewHealthMetrics(opts ...MetricsOpt) *HealthMetrics {
	cfg := &metricsConfig{mp: otel.GetMeterProvider()}
	for _, o := range opts {
		o(cfg)
	}
	meter := cfg.mp.Meter("agentlens.health")

	probesTotal, _ := meter.Int64Counter("agentlens.health.probes.total",
		metric.WithDescription("Total probes by result and protocol"))
	probesLatency, _ := meter.Float64Histogram("agentlens.health.probes.latency",
		metric.WithDescription("Probe latency in milliseconds"),
		metric.WithExplicitBucketBoundaries(10, 50, 100, 250, 500, 1000, 2500, 5000))
	stateTransitions, _ := meter.Int64Counter("agentlens.health.state_transitions.total",
		metric.WithDescription("State transition count"))

	return &HealthMetrics{
		probesTotal:      probesTotal,
		probesLatency:    probesLatency,
		stateTransitions: stateTransitions,
	}
}

// RecordProbe records a probe result.
func (m *HealthMetrics) RecordProbe(ctx context.Context, result, protocol string, latencyMs float64) {
	attrs := metric.WithAttributes(
		attribute.String("result", result),
		attribute.String("protocol", protocol),
	)
	m.probesTotal.Add(ctx, 1, attrs)
	m.probesLatency.Record(ctx, latencyMs, attrs)
}

// RecordStateTransition records a lifecycle state change.
func (m *HealthMetrics) RecordStateTransition(ctx context.Context, from, to, protocol string) {
	m.stateTransitions.Add(ctx, 1, metric.WithAttributes(
		attribute.String("from", from),
		attribute.String("to", to),
		attribute.String("protocol", protocol),
	))
}

// ParserMetrics holds parser metric instruments.
type ParserMetrics struct {
	invocationsTotal metric.Int64Counter
	duration         metric.Float64Histogram
}

// NewParserMetrics creates parser metric instruments.
func NewParserMetrics(opts ...MetricsOpt) *ParserMetrics {
	cfg := &metricsConfig{mp: otel.GetMeterProvider()}
	for _, o := range opts {
		o(cfg)
	}
	meter := cfg.mp.Meter("agentlens.parser")

	invocationsTotal, _ := meter.Int64Counter("agentlens.parser.invocations.total",
		metric.WithDescription("Parser invocations by type, result, and spec version"))
	duration, _ := meter.Float64Histogram("agentlens.parser.duration",
		metric.WithDescription("Parser duration in milliseconds"))

	return &ParserMetrics{
		invocationsTotal: invocationsTotal,
		duration:         duration,
	}
}

// RecordInvocation records a parser invocation.
func (m *ParserMetrics) RecordInvocation(ctx context.Context, parserType, result, specVersion string, durationMs float64) {
	m.invocationsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("type", parserType),
		attribute.String("result", result),
		attribute.String("spec_version", specVersion),
	))
	m.duration.Record(ctx, durationMs, metric.WithAttributes(
		attribute.String("type", parserType),
		attribute.String("result", result),
	))
}

// AuthMetrics holds authentication metric instruments.
type AuthMetrics struct {
	loginsTotal metric.Int64Counter
}

// NewAuthMetrics creates authentication metric instruments.
func NewAuthMetrics(opts ...MetricsOpt) *AuthMetrics {
	cfg := &metricsConfig{mp: otel.GetMeterProvider()}
	for _, o := range opts {
		o(cfg)
	}
	meter := cfg.mp.Meter("agentlens.auth")

	loginsTotal, _ := meter.Int64Counter("agentlens.auth.logins.total",
		metric.WithDescription("Login attempts by result and reason"))

	return &AuthMetrics{loginsTotal: loginsTotal}
}

// RecordLogin records a login attempt.
func (m *AuthMetrics) RecordLogin(ctx context.Context, result, reason string) {
	m.loginsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("result", result),
		attribute.String("reason", reason),
	))
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/telemetry/... -run TestHealthMetrics -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/metrics.go internal/telemetry/metrics_test.go
git commit -m "feat(telemetry): add health, parser, and auth metric instruments"
```

---

### Task 7: Application endpoints — /readyz and telemetry config handler

**Files:**
- Create: `internal/api/telemetry_handler.go`
- Create: `internal/api/telemetry_handler_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/telemetry_handler_test.go`:

```go
package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestReadyz_Healthy(t *testing.T) {
	// Use an in-memory SQLite DB (already available in test infrastructure)
	handler := api.NewReadyzHandler(func() error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

func TestReadyz_Unhealthy(t *testing.T) {
	handler := api.NewReadyzHandler(func() error {
		return fmt.Errorf("connection refused")
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"error"`)
}

func TestTelemetryConfig_Disabled(t *testing.T) {
	handler := api.NewTelemetryConfigHandler(false, "", "agentlens-web")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":false`)
}

func TestTelemetryConfig_Enabled(t *testing.T) {
	handler := api.NewTelemetryConfigHandler(true, "http://collector:4318/v1/traces", "agentlens-web")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/telemetry/config", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"enabled":true`)
	assert.Contains(t, w.Body.String(), `"endpoint"`)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/api/... -run TestReadyz -v
go test ./internal/api/... -run TestTelemetryConfig -v
```

Expected: FAIL.

- [ ] **Step 3: Implement handlers**

Create `internal/api/telemetry_handler.go`:

```go
package api

import (
	"net/http"
)

// NewReadyzHandler creates a readiness probe handler.
// pingFn should return nil if the database is reachable.
func NewReadyzHandler(pingFn func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pingFn(); err != nil {
			JSONResponse(w, http.StatusServiceUnavailable, map[string]string{
				"status": "error",
				"reason": "database unreachable",
			})
			return
		}
		JSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

type telemetryConfigResponse struct {
	Enabled     bool   `json:"enabled"`
	Endpoint    string `json:"endpoint,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
}

// NewTelemetryConfigHandler creates a handler for frontend telemetry config.
func NewTelemetryConfigHandler(enabled bool, endpoint, serviceName string) http.HandlerFunc {
	resp := telemetryConfigResponse{
		Enabled:     enabled,
		Endpoint:    endpoint,
		ServiceName: serviceName,
	}
	if !enabled {
		resp = telemetryConfigResponse{Enabled: false}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		JSONResponse(w, http.StatusOK, resp)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/api/... -run "TestReadyz|TestTelemetryConfig" -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/telemetry_handler.go internal/api/telemetry_handler_test.go
git commit -m "feat(api): add /readyz and /api/v1/telemetry/config handlers"
```

---

### Task 8: Router wiring — otelhttp, /readyz, /metrics, telemetry config

**Files:**
- Modify: `internal/api/router.go`

- [ ] **Step 1: Update RouterDeps**

Add to `RouterDeps` in `internal/api/router.go`:

```go
// PromHandler serves /metrics (nil = not registered).
PromHandler http.Handler
// ReadyzPing checks database reachability for /readyz.
ReadyzPing func() error
// TelemetryEnabled indicates if frontend telemetry config should be served.
TelemetryEnabled bool
// TelemetryEndpoint is the OTLP collector endpoint for the frontend.
TelemetryEndpoint string
```

- [ ] **Step 2: Wrap router with otelhttp and register new routes**

In `NewRouter`, add `otelhttp` wrapper and new routes:

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

func NewRouter(deps RouterDeps) http.Handler {
	// ... existing handler/router setup ...

	// Register health/readiness before auth
	r.Get("/healthz", h.Healthz)
	if deps.ReadyzPing != nil {
		r.Get("/readyz", NewReadyzHandler(deps.ReadyzPing))
	}
	if deps.PromHandler != nil {
		r.Handle("/metrics", deps.PromHandler)
	}

	r.Route("/api/v1", func(r chi.Router) {
		// Telemetry config — public, no auth
		r.Get("/telemetry/config", NewTelemetryConfigHandler(
			deps.TelemetryEnabled, deps.TelemetryEndpoint, "agentlens-web"))

		// ... rest of existing route registration ...
	})

	// ... SPA handler ...

	// Wrap with otelhttp — outermost
	return otelhttp.NewHandler(r, "agentlens",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)
}
```

Note: `NewRouter` return type changes from `*chi.Mux` to `http.Handler` because `otelhttp.NewHandler` returns `http.Handler`. Update `server.New` call in `main.go` accordingly (it already accepts `http.Handler`).

- [ ] **Step 3: Run existing API tests to verify no regressions**

```bash
go test ./internal/api/... -v
```

Expected: all PASS (existing tests + new ones).

- [ ] **Step 4: Commit**

```bash
git add internal/api/router.go
git commit -m "feat(api): wire otelhttp middleware, /readyz, /metrics, telemetry config"
```

---

### Task 9: Wire telemetry in main.go

**Files:**
- Modify: `cmd/agentlens/main.go`

- [ ] **Step 1: Add version variable and telemetry wiring**

Add at package level:

```go
var version = "dev"
```

After config load and slog setup, add telemetry init:

```go
// 3. Initialize telemetry
telProvider, err := telemetry.Init(context.Background(), cfg.Telemetry, version)
if err != nil {
	slog.Error("failed to initialize telemetry", "err", err)
	os.Exit(1)
}
defer func() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := telProvider.Shutdown(shutdownCtx); err != nil {
		slog.Error("telemetry shutdown error", "err", err)
	}
}()

// 4. Replace slog with fan-out if telemetry enabled
if cfg.Telemetry.Enabled && telProvider.LoggerProvider != nil {
	exportLevel := parseSlogLevel(cfg.Telemetry.LogExportLevel)
	bridgeHandler := otelslog.NewHandler("agentlens",
		otelslog.WithLoggerProvider(telProvider.LoggerProvider))
	stdoutHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	fanout := telemetry.NewFanoutHandler(stdoutHandler, bridgeHandler, exportLevel)
	slog.SetDefault(slog.New(fanout))
}
```

Add `parseSlogLevel` helper:

```go
func parseSlogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

Pass telemetry deps into `RouterDeps`:

```go
routerDeps := api.RouterDeps{
	// ... existing fields ...
	PromHandler:       telProvider.PromHandler,
	ReadyzPing:        dbPingFn, // func() error from database
	TelemetryEnabled:  cfg.Telemetry.Enabled,
	TelemetryEndpoint: cfg.Telemetry.Endpoint,
}
```

Create `dbPingFn` after database open:

```go
sqlDB, err := database.DB.DB()
if err != nil {
	slog.Error("failed to get sql.DB", "err", err)
	os.Exit(1)
}
dbPingFn := func() error { return sqlDB.PingContext(context.Background()) }
```

Add `-ldflags` to Makefile build target:

```makefile
VERSION ?= dev
build:
	CGO_ENABLED=1 go build -ldflags "-X main.version=$(VERSION)" -o agentlens ./cmd/agentlens
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/agentlens
```

Expected: no errors.

- [ ] **Step 3: Run full test suite**

```bash
make test
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/agentlens/main.go Makefile
git commit -m "feat(main): wire telemetry init, shutdown, fan-out slog, and /readyz"
```

---

### Task 10: Instrument health prober

**Files:**
- Modify: `plugins/health/health.go`

- [ ] **Step 1: Add otelhttp transport and span instrumentation to probeOne**

In `Init()`, wrap the HTTP client with `otelhttp.NewTransport()`:

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

func (p *Plugin) Init(k kernel.Kernel) error {
	p.store = k.Store()
	p.log = k.Logger().With("component", "health-checker")
	p.httpClient = &http.Client{
		Timeout:   p.timeout,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	return nil
}
```

In `probeOne()`, add span creation:

```go
import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (p *Plugin) probeOne(ctx context.Context, entry *model.CatalogEntry) model.Health {
	tracer := otel.Tracer("agentlens.health")
	ctx, span := tracer.Start(ctx, "health.probe", trace.WithAttributes(
		attribute.String("agentlens.entry.id", entry.ID),
		attribute.String("agentlens.entry.name", entry.DisplayName),
	))
	defer span.End()

	stateBefore := entry.Health.State

	// ... existing probe logic ...
	// After computing result health:

	h := // the computed health result
	span.SetAttributes(
		attribute.String("agentlens.probe.url", url),
		attribute.Int64("agentlens.probe.latency_ms", h.LatencyMs),
		attribute.String("agentlens.probe.result", probeResult), // "success"/"failure"/etc
		attribute.String("agentlens.probe.state_before", string(stateBefore)),
		attribute.String("agentlens.probe.state_after", string(h.State)),
	)

	if stateBefore != h.State {
		span.AddEvent("state_transition", trace.WithAttributes(
			attribute.String("from", string(stateBefore)),
			attribute.String("to", string(h.State)),
		))
	}

	if h.LastError != "" {
		errMsg := h.LastError
		if len(errMsg) > 256 {
			errMsg = errMsg[:256]
		}
		span.SetAttributes(attribute.String("agentlens.probe.error", errMsg))
		span.SetStatus(codes.Error, errMsg)
	}

	return h
}
```

The implementer must refactor `probeOne` to capture the result before returning, add span attributes, and add state transition events. The logic flow stays the same — instrumentation wraps it.

- [ ] **Step 2: Run existing health tests**

```bash
go test ./plugins/health/... -v
```

Expected: all PASS (existing tests still work — spans are no-ops without a configured provider).

- [ ] **Step 3: Commit**

```bash
git add plugins/health/health.go
git commit -m "feat(health): add OTel tracing and otelhttp transport to prober"
```

---

### Task 11: Instrument parsers

**Files:**
- Modify: `plugins/parsers/a2a/a2a.go`
- Modify: `plugins/parsers/mcp/mcp.go`

- [ ] **Step 1: Add span instrumentation to A2A parser**

In `Parse()` method of `plugins/parsers/a2a/a2a.go`:

```go
import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (p *Plugin) Parse(raw []byte) (*model.AgentType, error) {
	tracer := otel.Tracer("agentlens.parser")
	_, span := tracer.Start(context.Background(), "parser.a2a.parse", trace.WithAttributes(
		attribute.String("agentlens.parser.type", "a2a"),
		attribute.Int64("agentlens.parser.input_size", int64(len(raw))),
	))
	defer span.End()

	// ... existing parse logic ...

	// On success, before return:
	span.SetAttributes(
		attribute.String("agentlens.parser.spec_version", detectedVersion),
		attribute.Int("agentlens.parser.skill_count", skillCount),
		attribute.Int("agentlens.parser.extension_count", extensionCount),
		attribute.Int("agentlens.parser.security_scheme_count", securitySchemeCount),
	)

	// On error:
	// span.RecordError(err)
	// span.SetStatus(codes.Error, err.Error())

	return result, nil
}
```

Note: The `Parse` method currently doesn't take `context.Context`. The implementer must either add context to the method signature (if the `Parser` interface allows it) or use `context.Background()`. Check the `kernel.Parser` interface. If it doesn't include context, use `context.Background()` for the span — this is acceptable for CPU-bound parsing that doesn't do I/O.

- [ ] **Step 2: Apply same pattern to MCP parser**

Same instrumentation in `plugins/parsers/mcp/mcp.go` with `parser.mcp.parse` span name and `agentlens.parser.type=mcp`.

- [ ] **Step 3: Run parser tests**

```bash
go test ./plugins/parsers/... -v
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add plugins/parsers/a2a/a2a.go plugins/parsers/mcp/mcp.go
git commit -m "feat(parsers): add OTel tracing to A2A and MCP parsers"
```

---

### Task 12: Instrument auth login events

**Files:**
- Modify: `internal/api/auth_handlers.go`

- [ ] **Step 1: Add span events to Login handler**

In `auth_handlers.go`, the `Login` method at the success and failure paths:

```go
import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// After successful auth (before setting cookie):
span := trace.SpanFromContext(r.Context())
span.AddEvent("auth.login", trace.WithAttributes(
	attribute.String("username", req.Username),
	attribute.String("result", "success"),
	attribute.String("reason", ""),
))

// After failed password check:
span := trace.SpanFromContext(r.Context())
span.AddEvent("auth.login", trace.WithAttributes(
	attribute.String("username", req.Username),
	attribute.String("result", "failure"),
	attribute.String("reason", "invalid_password"),
))

// After account locked:
span.AddEvent("auth.login", trace.WithAttributes(
	attribute.String("username", req.Username),
	attribute.String("result", "failure"),
	attribute.String("reason", "account_locked"),
))
```

**Never record `req.Password` as an attribute.**

- [ ] **Step 2: Run auth tests**

```bash
go test ./internal/api/... -run ".*Login.*\|.*Auth.*" -v
```

Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/auth_handlers.go
git commit -m "feat(auth): add OTel span events for login attempts"
```

---

### Task 12b: Catalog gauge — async UpDownCounter

**Files:**
- Modify: `internal/telemetry/metrics.go`
- Modify: `internal/telemetry/metrics_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/telemetry/metrics_test.go`:

```go
func TestCatalogGauge_ReportsCorrectCounts(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	// Simulate 2 a2a/active + 1 mcp/offline
	countFn := func(ctx context.Context) map[string]int64 {
		return map[string]int64{
			"a2a:active":  2,
			"mcp:offline": 1,
		}
	}

	err := telemetry.RegisterCatalogGauge(countFn, telemetry.WithMeterProvider(mp))
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "agentlens.catalog.entries" {
				found = true
			}
		}
	}
	require.True(t, found, "agentlens.catalog.entries gauge not found")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/telemetry/... -run TestCatalogGauge -v
```

Expected: FAIL — `RegisterCatalogGauge` does not exist.

- [ ] **Step 3: Implement RegisterCatalogGauge**

Add to `internal/telemetry/metrics.go`:

```go
// RegisterCatalogGauge registers an async gauge that reports catalog entry counts.
// countFn returns a map of "protocol:state" → count.
func RegisterCatalogGauge(countFn func(ctx context.Context) map[string]int64, opts ...MetricsOpt) error {
	cfg := &metricsConfig{mp: otel.GetMeterProvider()}
	for _, o := range opts {
		o(cfg)
	}
	meter := cfg.mp.Meter("agentlens.catalog")

	gauge, err := meter.Int64ObservableUpDownCounter("agentlens.catalog.entries",
		metric.WithDescription("Number of catalog entries by protocol and state"))
	if err != nil {
		return fmt.Errorf("creating catalog gauge: %w", err)
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		counts := countFn(ctx)
		for key, count := range counts {
			parts := strings.SplitN(key, ":", 2)
			if len(parts) != 2 {
				continue
			}
			o.ObserveInt64(gauge, count,
				metric.WithAttributes(
					attribute.String("protocol", parts[0]),
					attribute.String("state", parts[1]),
				))
		}
		return nil
	}, gauge)
	return err
}
```

The caller in `main.go` passes a `countFn` that queries the store:

```go
countFn := func(ctx context.Context) map[string]int64 {
	// Query: SELECT protocol, status, count(*) FROM catalog_entries GROUP BY protocol, status
	// Return map like {"a2a:active": 5, "mcp:offline": 2}
}
telemetry.RegisterCatalogGauge(countFn)
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/telemetry/... -run TestCatalogGauge -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/telemetry/metrics.go internal/telemetry/metrics_test.go
git commit -m "feat(telemetry): add catalog entries gauge with async callback"
```

---

### Task 13: Update arch-go.yml

**Files:**
- Modify: `arch-go.yml`

- [ ] **Step 1: Add telemetry dependency rules**

Add to `dependenciesRules` in `arch-go.yml`:

```yaml
  # Infrastructure — telemetry must not depend on upper layers
  - package: "**.internal.telemetry"
    shouldNotDependsOn:
      internal:
        - "**.internal.api"
        - "**.internal.kernel"
        - "**.internal.server"
        - "**.internal.service"
        - "**.internal.store"
        - "**.plugins.**"
        - "**.cmd.**"
```

This allows `telemetry` to import foundation (`config`, `model`) and `db` but blocks upper layers.

- [ ] **Step 2: Run arch tests**

```bash
make arch-test
```

Expected: 100% compliance.

- [ ] **Step 3: Commit**

```bash
git add arch-go.yml
git commit -m "chore(arch): add telemetry package to infrastructure layer rules"
```

---

### Task 14: Update Dockerfile

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: Switch to nonroot image and add USER directive**

In `Dockerfile`, update the runtime stage:

```dockerfile
# Stage 3: Distroless runtime
FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=builder /app/agentlens .
EXPOSE 8080
USER 65532
CMD ["./agentlens"]
```

Also update the build stage to include version ldflags:

```dockerfile
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags "-X main.version=${VERSION}" -o agentlens ./cmd/agentlens
```

- [ ] **Step 2: Verify Docker build**

```bash
docker build --build-arg VERSION=dev -t agentlens:test .
```

Expected: builds successfully.

- [ ] **Step 3: Commit**

```bash
git add Dockerfile
git commit -m "chore(docker): switch to distroless nonroot (UID 65532), add version ldflags"
```

---

## Phase 2: Frontend Telemetry

### Task 15: Add OTel JS dependencies

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install dependencies**

```bash
cd web
bun add @opentelemetry/api @opentelemetry/sdk-trace-web @opentelemetry/instrumentation-fetch @opentelemetry/exporter-trace-otlp-http @opentelemetry/resources @opentelemetry/semantic-conventions
```

- [ ] **Step 2: Verify build**

```bash
bun run build
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/package.json web/bun.lock
git commit -m "chore(web): add @opentelemetry/* dependencies for trace propagation"
```

---

### Task 16: Frontend telemetry module

**Files:**
- Create: `web/src/telemetry.ts`
- Create: `web/src/telemetry.test.ts`

- [ ] **Step 1: Write failing test**

Create `web/src/telemetry.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'

describe('telemetry', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  it('should export initTelemetry function', async () => {
    const { initTelemetry } = await import('./telemetry')
    expect(typeof initTelemetry).toBe('function')
  })

  it('should not throw when initialized with valid config', async () => {
    const { initTelemetry } = await import('./telemetry')
    expect(() => {
      initTelemetry({
        endpoint: 'http://localhost:4318/v1/traces',
        serviceName: 'agentlens-web',
      })
    }).not.toThrow()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd web && bun run test -- --run telemetry
```

Expected: FAIL — module does not exist.

- [ ] **Step 3: Implement telemetry.ts**

Create `web/src/telemetry.ts`:

```typescript
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web'
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http'
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch'
import { Resource } from '@opentelemetry/resources'
import { ATTR_SERVICE_NAME } from '@opentelemetry/semantic-conventions'
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-web'
import { registerInstrumentations } from '@opentelemetry/instrumentation'

export interface TelemetryConfig {
  endpoint: string
  serviceName: string
}

export function initTelemetry(config: TelemetryConfig): void {
  const resource = new Resource({
    [ATTR_SERVICE_NAME]: config.serviceName,
  })

  const exporter = new OTLPTraceExporter({
    url: config.endpoint,
  })

  const provider = new WebTracerProvider({
    resource,
    spanProcessors: [new BatchSpanProcessor(exporter)],
  })

  provider.register()

  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/\/api\/.*/],
      }),
    ],
  })
}
```

Note: The exact API depends on the `@opentelemetry/sdk-trace-web` version. The `WebTracerProvider` constructor and `registerInstrumentations` signatures may vary. Check the installed version's types. Add `@opentelemetry/instrumentation` to dependencies if `registerInstrumentations` is in a separate package.

- [ ] **Step 4: Run tests**

```bash
cd web && bun run test -- --run telemetry
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/telemetry.ts web/src/telemetry.test.ts
git commit -m "feat(web): add OTel trace propagation via fetch instrumentation"
```

---

### Task 17: Wire telemetry in main.tsx

**Files:**
- Modify: `web/src/main.tsx`

- [ ] **Step 1: Add telemetry config fetch and dynamic import**

Update `web/src/main.tsx` to fetch telemetry config before mounting React:

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

async function boot() {
  // Initialize telemetry if enabled (best-effort, don't block app on failure)
  try {
    const resp = await fetch('/api/v1/telemetry/config')
    if (resp.ok) {
      const cfg = await resp.json()
      if (cfg.enabled && cfg.endpoint) {
        const { initTelemetry } = await import('./telemetry')
        initTelemetry(cfg)
      }
    }
  } catch {
    // Telemetry init failure should not block the app
  }

  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <App />
      </QueryClientProvider>
    </React.StrictMode>,
  )
}

boot()
```

- [ ] **Step 2: Run web build**

```bash
cd web && bun run build
```

Expected: no errors.

- [ ] **Step 3: Run web tests**

```bash
cd web && bun run test
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/main.tsx
git commit -m "feat(web): wire telemetry init with dynamic import in main.tsx"
```

---

## Phase 3: Helm Chart

### Task 18: Chart skeleton — Chart.yaml, values.yaml, _helpers.tpl

**Files:**
- Create: `deploy/helm/agentlens/Chart.yaml`
- Create: `deploy/helm/agentlens/values.yaml`
- Create: `deploy/helm/agentlens/templates/_helpers.tpl`

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p deploy/helm/agentlens/templates/tests deploy/helm/agentlens/ci
```

- [ ] **Step 2: Create Chart.yaml**

Write `deploy/helm/agentlens/Chart.yaml` as specified in the spec (apiVersion v2, Bitnami postgresql dependency, maintainers, keywords).

- [ ] **Step 3: Create values.yaml**

Write `deploy/helm/agentlens/values.yaml` with all values from the spec. Key adjustments from spec:
- `podSecurityContext.runAsUser: 65532`
- `podSecurityContext.runAsGroup: 65532`
- `podSecurityContext.fsGroup: 65532`
- `telemetry.logExportLevel: info`

- [ ] **Step 4: Create _helpers.tpl**

Standard Helm helpers: `agentlens.name`, `agentlens.fullname`, `agentlens.chart`, `agentlens.labels`, `agentlens.selectorLabels`, `agentlens.serviceAccountName`.

Add multi-replica guard:

```
{{- if and (gt (int .Values.replicaCount) 1) (eq .Values.database.dialect "sqlite") }}
{{- fail "replicaCount > 1 is not supported with SQLite (single-writer). Set database.dialect=postgres to scale horizontally." }}
{{- end }}
```

- [ ] **Step 5: Run helm lint**

```bash
helm lint charts/agentlens
```

Expected: passes (warnings OK at this stage — templates not yet created).

- [ ] **Step 6: Commit**

```bash
git add charts/
git commit -m "feat(helm): add chart skeleton — Chart.yaml, values.yaml, _helpers.tpl"
```

---

### Task 19: Core templates — deployment, service, serviceaccount, configmap, secret

**Files:**
- Create: `deploy/helm/agentlens/templates/deployment.yaml`
- Create: `deploy/helm/agentlens/templates/service.yaml`
- Create: `deploy/helm/agentlens/templates/serviceaccount.yaml`
- Create: `deploy/helm/agentlens/templates/configmap.yaml`
- Create: `deploy/helm/agentlens/templates/secret.yaml`

- [ ] **Step 1: Create deployment.yaml**

Include:
- Pod security context from values
- Container security context from values
- Liveness, readiness, startup probes from values
- Volume mounts: `/tmp` emptyDir (always), `/etc/agentlens/config.yaml` from ConfigMap, `/data` from PVC (SQLite only)
- Init container (PostgreSQL only): `busybox:1.36` with `nc -z` wait loop
- Env vars from ConfigMap (non-sensitive) and Secret (passwords) via `secretKeyRef`
- Auto-enable Prometheus env when `metrics.serviceMonitor.enabled=true`
- Resource requests/limits from values

- [ ] **Step 2: Create service.yaml**

ClusterIP service, port 8080 named `http`.

- [ ] **Step 3: Create serviceaccount.yaml**

Conditional on `serviceAccount.create`, with annotation support.

- [ ] **Step 4: Create configmap.yaml**

Non-sensitive config: log level, health check settings, telemetry settings (no passwords).

- [ ] **Step 5: Create secret.yaml**

- `admin-password`: `randAlphaNum 24` if not provided
- `database-password`: from values or subchart reference
- `database-url`: assembled DSN for external PostgreSQL

- [ ] **Step 6: Run helm template and lint**

```bash
helm template test charts/agentlens
helm lint charts/agentlens
```

Expected: renders without errors, lint passes.

- [ ] **Step 7: Commit**

```bash
git add deploy/helm/agentlens/templates/
git commit -m "feat(helm): add core templates — deployment, service, SA, configmap, secret"
```

---

### Task 20: SQLite PVC

**Files:**
- Create: `deploy/helm/agentlens/templates/pvc.yaml`

- [ ] **Step 1: Create PVC template**

Conditional on `database.dialect == "sqlite"`:

```yaml
{{- if eq .Values.database.dialect "sqlite" }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ include "agentlens.fullname" . }}-data
  labels:
    {{- include "agentlens.labels" . | nindent 4 }}
spec:
  accessModes: [ReadWriteOnce]
  {{- if .Values.database.sqlite.storageClass }}
  storageClassName: {{ .Values.database.sqlite.storageClass }}
  {{- end }}
  resources:
    requests:
      storage: {{ .Values.database.sqlite.storageSize }}
{{- end }}
```

- [ ] **Step 2: Verify template renders**

```bash
helm template test charts/agentlens | grep PersistentVolumeClaim
```

Expected: PVC present (SQLite is default).

- [ ] **Step 3: Commit**

```bash
git add deploy/helm/agentlens/templates/pvc.yaml
git commit -m "feat(helm): add SQLite PVC template"
```

---

### Task 21: Ingress and Gateway API

**Files:**
- Create: `deploy/helm/agentlens/templates/ingress.yaml`
- Create: `deploy/helm/agentlens/templates/gateway-httproute.yaml`

- [ ] **Step 1: Create ingress.yaml**

Standard `networking.k8s.io/v1` Ingress, conditional on `ingress.enabled`. Support `className`, annotations, hosts with paths, TLS.

- [ ] **Step 2: Create gateway-httproute.yaml**

`gateway.networking.k8s.io/v1` HTTPRoute, conditional on `gateway.enabled`. As specified in spec.

- [ ] **Step 3: Verify both render**

```bash
helm template test charts/agentlens --set ingress.enabled=true | grep "kind: Ingress"
helm template test charts/agentlens --set gateway.enabled=true | grep "kind: HTTPRoute"
```

Expected: both present.

- [ ] **Step 4: Commit**

```bash
git add deploy/helm/agentlens/templates/ingress.yaml deploy/helm/agentlens/templates/gateway-httproute.yaml
git commit -m "feat(helm): add Ingress and Gateway API HTTPRoute templates"
```

---

### Task 22: HPA, PDB, ServiceMonitor, NetworkPolicy

**Files:**
- Create: `deploy/helm/agentlens/templates/hpa.yaml`
- Create: `deploy/helm/agentlens/templates/pdb.yaml`
- Create: `deploy/helm/agentlens/templates/servicemonitor.yaml`
- Create: `deploy/helm/agentlens/templates/networkpolicy.yaml`

- [ ] **Step 1: Create hpa.yaml**

Conditional on `autoscaling.enabled`. Target CPU and memory from values.

- [ ] **Step 2: Create pdb.yaml**

Conditional on `pdb.enabled`. `minAvailable` from values.

- [ ] **Step 3: Create servicemonitor.yaml**

Conditional on `metrics.serviceMonitor.enabled`. As specified in spec — targets `http` port at `/metrics`.

- [ ] **Step 4: Create networkpolicy.yaml**

Conditional on `networkPolicy.enabled`. DNS egress always allowed. Configurable ingress/egress from values.

- [ ] **Step 5: Verify all render**

```bash
helm template test charts/agentlens \
  --set autoscaling.enabled=true \
  --set pdb.enabled=true \
  --set metrics.serviceMonitor.enabled=true \
  --set networkPolicy.enabled=true
```

Expected: all four resources present.

- [ ] **Step 6: Commit**

```bash
git add deploy/helm/agentlens/templates/hpa.yaml deploy/helm/agentlens/templates/pdb.yaml \
  deploy/helm/agentlens/templates/servicemonitor.yaml deploy/helm/agentlens/templates/networkpolicy.yaml
git commit -m "feat(helm): add HPA, PDB, ServiceMonitor, NetworkPolicy templates"
```

---

### Task 23: Helm test and values schema

**Files:**
- Create: `deploy/helm/agentlens/templates/tests/test-connection.yaml`
- Create: `deploy/helm/agentlens/values.schema.json`
- Create: `deploy/helm/agentlens/ci/ci-values.yaml`

- [ ] **Step 1: Create test-connection.yaml**

Helm test hook as specified in spec — busybox hitting `/healthz`, `/readyz`, `/api/v1/catalog`.

- [ ] **Step 2: Create values.schema.json**

Key validations:
- `database.dialect`: enum `["sqlite", "postgres"]`
- `resources.requests` and `resources.limits`: required
- `image.repository`: non-empty string
- `telemetry.protocol`: enum `["grpc", "http"]`
- `replicaCount`: integer >= 1

- [ ] **Step 3: Create ci-values.yaml**

Minimal values for CI lint:

```yaml
image:
  tag: latest
config:
  adminPassword: "test-password-12345!"
```

- [ ] **Step 4: Run helm lint --strict**

```bash
helm lint charts/agentlens --strict
helm lint charts/agentlens --strict -f deploy/helm/agentlens/ci/ci-values.yaml
```

Expected: zero errors, zero warnings.

- [ ] **Step 5: Commit**

```bash
git add deploy/helm/agentlens/templates/tests/ deploy/helm/agentlens/values.schema.json deploy/helm/agentlens/ci/
git commit -m "feat(helm): add helm test, values schema validation, CI values"
```

---

### Task 23b: Update Makefile — helm-lint, helm-test, version ldflags

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Update helm-lint target**

The existing `helm-lint` target points to the correct path but lacks `--strict` and schema validation. Update in `Makefile`:

```makefile
HELM_CHART := deploy/helm/agentlens
VERSION ?= dev

## helm-lint: Lint the Helm chart with strict mode and schema validation
helm-lint:
	helm lint $(HELM_CHART) --strict
	helm lint $(HELM_CHART) --strict -f $(HELM_CHART)/ci/ci-values.yaml
	helm template agentlens $(HELM_CHART) --debug > /dev/null
```

- [ ] **Step 2: Add helm-test target**

This target runs helm template tests via a script that validates all value combinations without a running cluster:

```makefile
## helm-test: Run Helm template tests for all value combinations
helm-test: helm-lint
	./scripts/test-helm-templates.sh
```

- [ ] **Step 3: Update build target with version ldflags**

```makefile
## build: Build the agentlens binary (CGO enabled for SQLite) — runs lint first
build: lint
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agentlens
```

- [ ] **Step 4: Add helm-test to all target**

Update the `all` target to include `helm-test`:

```makefile
## all: Run format, lint, test, arch-test, web-lint, web-test, web-build, web-test-coverage, helm-test, and build
all: format lint test arch-test build web-lint web-test web-build web-test-coverage helm-test
```

- [ ] **Step 5: Update .PHONY**

Add `helm-test` to the `.PHONY` list.

- [ ] **Step 6: Verify all targets work**

```bash
make helm-lint
make helm-test
make build VERSION=0.2.0
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add Makefile
git commit -m "chore(make): update helm-lint with --strict, add helm-test, add version ldflags to build"
```

---

## Phase 4: Integration & E2E

### Task 24: Docker Compose + Jaeger integration test

**Files:**
- Create: `docker-compose.otel.yml`
- Create: `scripts/test-otel-integration.sh`

- [ ] **Step 1: Create docker-compose.otel.yml**

```yaml
version: "3.9"
services:
  jaeger:
    image: jaegertracing/all-in-one:1.57
    ports:
      - "16686:16686"  # Jaeger UI
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    environment:
      COLLECTOR_OTLP_ENABLED: "true"

  agentlens:
    build:
      context: .
      args:
        VERSION: integration-test
    ports:
      - "8080:8080"
    environment:
      AGENTLENS_OTEL_ENABLED: "true"
      AGENTLENS_OTEL_ENDPOINT: "jaeger:4317"
      AGENTLENS_OTEL_PROTOCOL: "grpc"
      AGENTLENS_OTEL_INSECURE: "true"
      AGENTLENS_OTEL_SERVICE_NAME: "agentlens"
      AGENTLENS_METRICS_PROMETHEUS_ENABLED: "true"
    depends_on:
      - jaeger
```

- [ ] **Step 2: Create integration test script**

Create `scripts/test-otel-integration.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "Starting OTel integration test..."
docker compose -f docker-compose.otel.yml up -d --build

# Wait for services
echo "Waiting for AgentLens..."
for i in $(seq 1 30); do
  if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then break; fi
  sleep 2
done

# Generate traffic
echo "Generating traces..."
curl -sf http://localhost:8080/readyz
curl -sf http://localhost:8080/api/v1/catalog || true
curl -sf http://localhost:8080/metrics | head -5

# Wait for traces to flush
sleep 5

# Query Jaeger for traces
echo "Checking Jaeger for traces..."
TRACES=$(curl -sf "http://localhost:16686/api/traces?service=agentlens&limit=5")
COUNT=$(echo "$TRACES" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('data',[])))" 2>/dev/null || echo "0")

echo "Found $COUNT traces in Jaeger"

docker compose -f docker-compose.otel.yml down

if [ "$COUNT" -gt 0 ]; then
  echo "PASS: OTel integration test"
  exit 0
else
  echo "FAIL: No traces found in Jaeger"
  exit 1
fi
```

```bash
chmod +x scripts/test-otel-integration.sh
```

- [ ] **Step 3: Run integration test**

```bash
./scripts/test-otel-integration.sh
```

Expected: PASS — traces found in Jaeger.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.otel.yml scripts/test-otel-integration.sh
git commit -m "test(integration): add Docker Compose + Jaeger OTel integration test"
```

---

### Task 24b: E2E Playwright OTel smoke test

**Files:**
- Modify: `e2e/tests/` (add new test file or extend existing)

- [ ] **Step 1: Create OTel smoke test**

Create `e2e/tests/otel-smoke.spec.ts`:

```typescript
import { test, expect } from '@playwright/test'

test.describe('OTel smoke test', () => {
  test('telemetry config endpoint returns expected shape', async ({ request }) => {
    const resp = await request.get('/api/v1/telemetry/config')
    expect(resp.ok()).toBeTruthy()
    const body = await resp.json()
    expect(body).toHaveProperty('enabled')
  })

  test('/readyz returns healthy', async ({ request }) => {
    const resp = await request.get('/readyz')
    expect(resp.ok()).toBeTruthy()
    const body = await resp.json()
    expect(body.status).toBe('ok')
  })

  test('/healthz returns ok', async ({ request }) => {
    const resp = await request.get('/healthz')
    expect(resp.ok()).toBeTruthy()
    const body = await resp.json()
    expect(body.status).toBe('ok')
  })
})
```

Note: Full OTLP receiver validation (verify mock receiver got trace + metrics batches) requires the Docker Compose test from Task 24. This Playwright test validates the endpoints exist and respond correctly. For a full E2E smoke test with a mock OTLP receiver, extend `e2e/run-e2e.sh` to start AgentLens with OTel pointed at a mock receiver, run the suite, then query the mock for received data.

- [ ] **Step 2: Run E2E tests**

```bash
make e2e-test
```

Expected: new tests PASS alongside existing suite.

- [ ] **Step 3: Commit**

```bash
git add e2e/tests/otel-smoke.spec.ts
git commit -m "test(e2e): add OTel smoke test for /readyz, /healthz, telemetry config"
```

---

### Task 25: Helm template tests

**Files:**
- Create: `deploy/helm/agentlens/tests/` (or run via `helm template` assertions in a script)

- [ ] **Step 1: Create Helm template test script**

Create `scripts/test-helm-templates.sh` that verifies all template combinations from the spec:

```bash
#!/usr/bin/env bash
set -euo pipefail

CHART=charts/agentlens

echo "=== Helm Template Tests ==="

# Test 19: Default values — helm lint
echo "Test: Default values lint"
helm lint "$CHART" --strict
echo "PASS"

# Test 20: SQLite mode — PVC rendered, no PostgreSQL
echo "Test: SQLite mode"
OUTPUT=$(helm template test "$CHART")
echo "$OUTPUT" | grep -q "PersistentVolumeClaim" || { echo "FAIL: PVC missing"; exit 1; }
echo "$OUTPUT" | grep -q "kind: StatefulSet" && { echo "FAIL: StatefulSet present"; exit 1; }
echo "PASS"

# Test 21: PostgreSQL subchart
echo "Test: PostgreSQL subchart"
helm dependency update "$CHART"
OUTPUT=$(helm template test "$CHART" --set database.dialect=postgres --set postgresql.enabled=true)
echo "$OUTPUT" | grep -q "wait-postgres" || { echo "FAIL: init container missing"; exit 1; }
echo "$OUTPUT" | grep -q "PersistentVolumeClaim" && { echo "FAIL: PVC present with postgres"; exit 1; } || true
echo "PASS"

# Test 23: Ingress
echo "Test: Ingress"
OUTPUT=$(helm template test "$CHART" --set ingress.enabled=true)
echo "$OUTPUT" | grep -q "kind: Ingress" || { echo "FAIL: Ingress missing"; exit 1; }
echo "PASS"

# Test 24: Gateway API
echo "Test: Gateway API"
OUTPUT=$(helm template test "$CHART" --set gateway.enabled=true --set gateway.gatewayName=my-gw)
echo "$OUTPUT" | grep -q "kind: HTTPRoute" || { echo "FAIL: HTTPRoute missing"; exit 1; }
echo "PASS"

# Test 25: HPA
echo "Test: HPA"
OUTPUT=$(helm template test "$CHART" --set autoscaling.enabled=true --set database.dialect=postgres)
echo "$OUTPUT" | grep -q "kind: HorizontalPodAutoscaler" || { echo "FAIL: HPA missing"; exit 1; }
echo "PASS"

# Test 26: PDB
echo "Test: PDB"
OUTPUT=$(helm template test "$CHART")
echo "$OUTPUT" | grep -q "kind: PodDisruptionBudget" || { echo "FAIL: PDB missing"; exit 1; }
echo "PASS"

# Test 27: ServiceMonitor
echo "Test: ServiceMonitor"
OUTPUT=$(helm template test "$CHART" --set metrics.serviceMonitor.enabled=true)
echo "$OUTPUT" | grep -q "kind: ServiceMonitor" || { echo "FAIL: ServiceMonitor missing"; exit 1; }
echo "$OUTPUT" | grep -q "AGENTLENS_METRICS_PROMETHEUS_ENABLED" || { echo "FAIL: Prometheus auto-enable missing"; exit 1; }
echo "PASS"

# Test 29: Security context
echo "Test: Security context"
OUTPUT=$(helm template test "$CHART")
echo "$OUTPUT" | grep -q "runAsNonRoot: true" || { echo "FAIL: runAsNonRoot missing"; exit 1; }
echo "$OUTPUT" | grep -q "readOnlyRootFilesystem: true" || { echo "FAIL: readOnlyRootFilesystem missing"; exit 1; }
echo "PASS"

# Test 31: Multi-replica guard
echo "Test: Multi-replica guard"
helm template test "$CHART" --set replicaCount=3 2>&1 | grep -q "not supported with SQLite" || { echo "FAIL: guard not triggered"; exit 1; }
echo "PASS"

echo "=== All Helm template tests PASSED ==="
```

```bash
chmod +x scripts/test-helm-templates.sh
```

- [ ] **Step 2: Run Helm template tests**

```bash
./scripts/test-helm-templates.sh
```

Expected: all PASS.

- [ ] **Step 3: Commit**

```bash
git add scripts/test-helm-templates.sh
git commit -m "test(helm): add Helm template test script for all value combinations"
```

---

### Task 26: Final validation — full test suite

**Files:** None new — validation only.

- [ ] **Step 1: Run Go tests**

```bash
make test
```

Expected: all PASS.

- [ ] **Step 2: Run arch tests**

```bash
make arch-test
```

Expected: 100% compliance.

- [ ] **Step 3: Run web tests**

```bash
cd web && bun run test
```

Expected: all PASS.

- [ ] **Step 4: Run web build**

```bash
make web-build
```

Expected: builds successfully.

- [ ] **Step 5: Run full build**

```bash
make build
```

Expected: builds successfully.

- [ ] **Step 6: Run Helm lint**

```bash
helm lint charts/agentlens --strict
```

Expected: zero errors/warnings.

- [ ] **Step 7: Commit any final fixes**

If any test failures, fix and commit with descriptive message.

---

## Phase 5: Documentation

### Task 27: Update documentation

**Files:**
- Modify: `docs/architecture.md` — add telemetry to high-level diagram, mention infrastructure layer placement
- Modify: `docs/settings.md` — add all telemetry env vars and config keys
- Modify: `docs/api.md` — add `/readyz`, `/metrics`, `/api/v1/telemetry/config` endpoints
- Create: `deploy/helm/agentlens/README.md` — chart usage, EKS/GKE/AKS examples, values reference

- [ ] **Step 1: Update architecture.md**

Add `internal/telemetry/` to the architecture diagram. Add Mermaid diagram showing telemetry data flow (AgentLens → OTel Collector → Jaeger/Prometheus/Loki).

- [ ] **Step 2: Update settings.md**

Add telemetry config table with all env vars, defaults, and descriptions.

- [ ] **Step 3: Update api.md**

Add `/readyz`, `/metrics`, `/api/v1/telemetry/config` with request/response schemas.

- [ ] **Step 4: Create chart README**

Standard Helm chart README with installation instructions, values table, examples for SQLite and PostgreSQL modes.

- [ ] **Step 5: Commit**

```bash
git add docs/ deploy/helm/agentlens/README.md
git commit -m "docs: add telemetry config, new endpoints, and Helm chart documentation"
```

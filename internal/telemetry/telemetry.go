// Package telemetry provides OpenTelemetry initialization and shutdown for AgentLens.
// It is an infrastructure-layer package — initialized in main before plugins, shut down after.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Provider holds initialized OTel providers and a Shutdown function.
// When telemetry is disabled, TracerProvider/MeterProvider/LoggerProvider are nil,
// and Shutdown is a no-op.
type Provider struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider
	// PromHandler serves /metrics when Prometheus is enabled. Nil otherwise.
	PromHandler http.Handler
	// Shutdown flushes all pending telemetry. Must be called on process exit.
	Shutdown func(ctx context.Context) error
}

// Init initializes OpenTelemetry providers based on cfg.
// When disabled, returns a no-op Provider (nil providers, no-op Shutdown).
// When enabled but endpoint empty, only Prometheus is initialized (if configured).
// Registers global TracerProvider, MeterProvider, and TextMapPropagator.
func Init(ctx context.Context, cfg config.TelemetryConfig, version string) (*Provider, error) {
	noop := &Provider{
		Shutdown: func(ctx context.Context) error { return nil },
	}

	if !cfg.Enabled {
		// Prometheus can be enabled independently of OTLP export.
		if cfg.Prometheus.Enabled {
			return initPrometheusOnly(ctx, cfg, version)
		}
		return noop, nil
	}

	// Prometheus-only mode: OTLP enabled but no endpoint configured.
	if cfg.Endpoint == "" {
		if !cfg.Prometheus.Enabled {
			slog.Warn("telemetry enabled but no endpoint and prometheus disabled, falling back to no-op")
			return noop, nil
		}
		return initPrometheusOnly(ctx, cfg, version)
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

	p, err := newSDKProviders(ctx, res, cfg)
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(p.tp)
	otel.SetMeterProvider(p.mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Provider{
		TracerProvider: p.tp,
		MeterProvider:  p.mp,
		LoggerProvider: p.lp,
		PromHandler:    p.promHandler,
		Shutdown:       newShutdown(p.tp, p.mp, p.lp),
	}, nil
}

type sdkProviders struct {
	tp          *sdktrace.TracerProvider
	mp          *sdkmetric.MeterProvider
	lp          *sdklog.LoggerProvider
	promHandler http.Handler
}

// newSDKProviders constructs trace, metric, and log SDK providers from the given resource and config.
func newSDKProviders(ctx context.Context, res *resource.Resource, cfg config.TelemetryConfig) (*sdkProviders, error) {
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

	mp, promHandler, err := newMeterProvider(ctx, res, cfg, tp)
	if err != nil {
		return nil, err
	}

	lp, err := newLoggerProvider(ctx, res, cfg, tp, mp)
	if err != nil {
		return nil, err
	}

	return &sdkProviders{tp: tp, mp: mp, lp: lp, promHandler: promHandler}, nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, cfg config.TelemetryConfig, tp *sdktrace.TracerProvider) (*sdkmetric.MeterProvider, http.Handler, error) {
	otlpMetricExp, err := newMetricExporter(ctx, cfg)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, nil, fmt.Errorf("creating metric exporter: %w", err)
	}

	var metricReaders []sdkmetric.Reader
	metricReaders = append(metricReaders, sdkmetric.NewPeriodicReader(otlpMetricExp,
		sdkmetric.WithInterval(cfg.MetricsInterval),
	))

	var promHandler http.Handler
	if cfg.Prometheus.Enabled {
		promExp, promErr := promexporter.New()
		if promErr != nil {
			_ = tp.Shutdown(ctx)
			return nil, nil, fmt.Errorf("creating prometheus exporter: %w", promErr)
		}
		metricReaders = append(metricReaders, promExp)
		promHandler = promhttp.Handler()
	}

	mpOpts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	for _, r := range metricReaders {
		mpOpts = append(mpOpts, sdkmetric.WithReader(r))
	}
	return sdkmetric.NewMeterProvider(mpOpts...), promHandler, nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource, cfg config.TelemetryConfig, tp *sdktrace.TracerProvider, mp *sdkmetric.MeterProvider) (*sdklog.LoggerProvider, error) {
	logExp, err := newLogExporter(ctx, cfg)
	if err != nil {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil, fmt.Errorf("creating log exporter: %w", err)
	}
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	), nil
}

func newShutdown(tp *sdktrace.TracerProvider, mp *sdkmetric.MeterProvider, lp *sdklog.LoggerProvider) func(context.Context) error {
	return func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil && !isContextErr(err) {
			errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
		}
		if err := mp.Shutdown(ctx); err != nil && !isContextErr(err) {
			errs = append(errs, fmt.Errorf("metric provider shutdown: %w", err))
		}
		if err := lp.Shutdown(ctx); err != nil && !isContextErr(err) {
			errs = append(errs, fmt.Errorf("log provider shutdown: %w", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("telemetry shutdown errors: %v", errs)
		}
		return nil
	}
}

// isContextErr returns true if the error is or wraps a context deadline/cancellation.
// Shutdown errors of this kind are best-effort — the process is exiting and the
// collector is either unreachable or the caller explicitly set a short deadline.
func isContextErr(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// initPrometheusOnly sets up a MeterProvider with only the Prometheus exporter.
// Used when telemetry is enabled and Prometheus is configured but no OTLP endpoint is set.
func initPrometheusOnly(ctx context.Context, cfg config.TelemetryConfig, version string) (*Provider, error) {
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

	promExp, err := promexporter.New()
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExp),
	)
	otel.SetMeterProvider(mp)

	return &Provider{
		MeterProvider: mp,
		PromHandler:   promhttp.Handler(),
		Shutdown: func(ctx context.Context) error {
			if err := mp.Shutdown(ctx); err != nil && !isContextErr(err) {
				return fmt.Errorf("metric provider shutdown: %w", err)
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
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
		}
		return otlptracehttp.New(ctx, opts...)
	default: // grpc
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
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
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
		}
		return otlpmetrichttp.New(ctx, opts...)
	default:
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
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
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlploghttp.WithHeaders(cfg.Headers))
		}
		return otlploghttp.New(ctx, opts...)
	default:
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		if len(cfg.Headers) > 0 {
			opts = append(opts, otlploggrpc.WithHeaders(cfg.Headers))
		}
		return otlploggrpc.New(ctx, opts...)
	}
}

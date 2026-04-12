package telemetry

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsOption configures metric instruments.
type MetricsOption func(*metricsCfg)

type metricsCfg struct {
	mp metric.MeterProvider
}

// WithMeterProvider overrides the meter provider (useful in tests).
func WithMeterProvider(mp metric.MeterProvider) MetricsOption {
	return func(c *metricsCfg) { c.mp = mp }
}

func newMetricsCfg(opts []MetricsOption) *metricsCfg {
	cfg := &metricsCfg{mp: otel.GetMeterProvider()}
	for _, o := range opts {
		o(cfg)
	}
	return cfg
}

// HealthMetrics holds metric instruments for the health prober.
type HealthMetrics struct {
	probesTotal      metric.Int64Counter
	probesLatency    metric.Float64Histogram
	stateTransitions metric.Int64Counter
}

// NewHealthMetrics creates health prober metric instruments.
func NewHealthMetrics(opts ...MetricsOption) *HealthMetrics {
	cfg := newMetricsCfg(opts)
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

// RecordProbe records a probe result with latency.
func (m *HealthMetrics) RecordProbe(ctx context.Context, result, protocol string, latencyMs float64) {
	attrs := metric.WithAttributes(
		attribute.String("result", result),
		attribute.String("protocol", protocol),
	)
	m.probesTotal.Add(ctx, 1, attrs)
	m.probesLatency.Record(ctx, latencyMs, attrs)
}

// RecordStateTransition records a lifecycle state transition.
func (m *HealthMetrics) RecordStateTransition(ctx context.Context, from, to, protocol string) {
	m.stateTransitions.Add(ctx, 1, metric.WithAttributes(
		attribute.String("from", from),
		attribute.String("to", to),
		attribute.String("protocol", protocol),
	))
}

// ParserMetrics holds metric instruments for parsers.
type ParserMetrics struct {
	invocationsTotal metric.Int64Counter
	duration         metric.Float64Histogram
}

// NewParserMetrics creates parser metric instruments.
func NewParserMetrics(opts ...MetricsOption) *ParserMetrics {
	cfg := newMetricsCfg(opts)
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

// AuthMetrics holds metric instruments for authentication.
type AuthMetrics struct {
	loginsTotal metric.Int64Counter
}

// NewAuthMetrics creates authentication metric instruments.
func NewAuthMetrics(opts ...MetricsOption) *AuthMetrics {
	cfg := newMetricsCfg(opts)
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

// RegisterCatalogGauge registers an async UpDownCounter reporting catalog entry counts.
// countFn returns a map of "protocol:state" → count, called at each metrics collection.
func RegisterCatalogGauge(countFn func(ctx context.Context) map[string]int64, opts ...MetricsOption) error {
	cfg := newMetricsCfg(opts)
	meter := cfg.mp.Meter("agentlens.catalog")

	gauge, err := meter.Int64ObservableUpDownCounter("agentlens.catalog.entries",
		metric.WithDescription("Number of catalog entries by protocol and state"))
	if err != nil {
		return fmt.Errorf("creating catalog gauge: %w", err)
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		for key, count := range countFn(ctx) {
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

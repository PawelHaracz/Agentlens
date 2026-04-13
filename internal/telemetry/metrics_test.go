package telemetry_test

import (
	"context"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/telemetry"
	"github.com/stretchr/testify/assert"
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

	names := collectMetricNames(rm)
	assert.Contains(t, names, "agentlens.health.probes.total")
	assert.Contains(t, names, "agentlens.health.probes.latency")
}

func TestHealthMetrics_RecordStateTransition(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	m := telemetry.NewHealthMetrics(telemetry.WithMeterProvider(mp))
	m.RecordStateTransition(context.Background(), "active", "offline", "a2a")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	names := collectMetricNames(rm)
	assert.Contains(t, names, "agentlens.health.state_transitions.total")
}

func TestParserMetrics_RecordInvocation(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	m := telemetry.NewParserMetrics(telemetry.WithMeterProvider(mp))
	m.RecordInvocation(context.Background(), "a2a", "success", "1.0", 5.0)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	names := collectMetricNames(rm)
	assert.Contains(t, names, "agentlens.parser.invocations.total")
	assert.Contains(t, names, "agentlens.parser.duration")
}

func TestAuthMetrics_RecordLogin(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	m := telemetry.NewAuthMetrics(telemetry.WithMeterProvider(mp))
	m.RecordLogin(context.Background(), "success", "")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	names := collectMetricNames(rm)
	assert.Contains(t, names, "agentlens.auth.logins.total")
}

func TestCatalogGauge(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	countFn := func(ctx context.Context) map[string]int64 {
		return map[string]int64{
			"active":  2,
			"offline": 1,
		}
	}

	err := telemetry.RegisterCatalogGauge(countFn, telemetry.WithMeterProvider(mp))
	require.NoError(t, err)

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	names := collectMetricNames(rm)
	assert.Contains(t, names, "agentlens.catalog.entries")
}

func collectMetricNames(rm metricdata.ResourceMetrics) map[string]bool {
	names := make(map[string]bool)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names[m.Name] = true
		}
	}
	return names
}

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
	assert.NotNil(t, p.Shutdown)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestInitEnabledEmptyEndpoint(t *testing.T) {
	cfg := config.TelemetryConfig{
		Enabled:  true,
		Endpoint: "",
		Protocol: "grpc",
	}
	p, err := telemetry.Init(context.Background(), cfg, "test")
	require.NoError(t, err)
	assert.Nil(t, p.PromHandler)
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestInitEnabledValidConfig(t *testing.T) {
	// Use a non-existent collector — we just want to check the provider is configured,
	// not that it can actually connect (exporters are async/non-blocking on init)
	cfg := config.TelemetryConfig{
		Enabled:          true,
		Endpoint:         "localhost:14317", // deliberately wrong port to avoid real connection
		Protocol:         "grpc",
		Insecure:         true,
		ServiceName:      "agentlens-test",
		Environment:      "test",
		TracesSampler:    "parentbased_traceidratio",
		TracesSampleRate: 1.0,
		MetricsInterval:  5 * time.Second,
		LogExportLevel:   "info",
	}
	p, err := telemetry.Init(context.Background(), cfg, "v0.0.1")
	require.NoError(t, err)
	assert.NotNil(t, p.TracerProvider)
	assert.NotNil(t, p.MeterProvider)
	assert.NotNil(t, p.LoggerProvider)
	assert.Nil(t, p.PromHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, p.Shutdown(ctx))
}

func TestInitWithPrometheus(t *testing.T) {
	cfg := config.TelemetryConfig{
		Enabled:          true,
		Endpoint:         "localhost:14317",
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
	p, err := telemetry.Init(context.Background(), cfg, "v0.0.1")
	require.NoError(t, err)
	assert.NotNil(t, p.PromHandler)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, p.Shutdown(ctx))
}

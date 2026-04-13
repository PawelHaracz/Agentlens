package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/config"
)

func TestTelemetryDefaults(t *testing.T) {
	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.False(t, cfg.Telemetry.Enabled)
	assert.Equal(t, "grpc", cfg.Telemetry.Protocol)
	assert.True(t, cfg.Telemetry.Insecure)
	assert.Equal(t, "agentlens", cfg.Telemetry.ServiceName)
	assert.Equal(t, "production", cfg.Telemetry.Environment)
	assert.Equal(t, 1.0, cfg.Telemetry.TracesSampleRate)
	assert.Equal(t, 30*time.Second, cfg.Telemetry.MetricsInterval)
	assert.Equal(t, "info", cfg.Telemetry.LogExportLevel)
	assert.False(t, cfg.Telemetry.Prometheus.Enabled)
}

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

	cfg, err := config.Load("")
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

func TestTelemetryOtelEndpointFallback(t *testing.T) {
	t.Setenv("AGENTLENS_OTEL_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "fallback:4317")
	// AGENTLENS_OTEL_ENDPOINT not set

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, "fallback:4317", cfg.Telemetry.Endpoint)
}

func TestTelemetryAgentlensEndpointTakesPrecedence(t *testing.T) {
	t.Setenv("AGENTLENS_OTEL_ENDPOINT", "primary:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "fallback:4317")

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, "primary:4317", cfg.Telemetry.Endpoint)
}

func TestDefaults(t *testing.T) {
	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "./data", cfg.DataDir)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 5*time.Minute, cfg.PollInterval)
	assert.True(t, cfg.HealthCheck.Enabled)
	assert.Equal(t, 30*time.Second, cfg.HealthCheck.Interval)
	assert.Equal(t, 5*time.Second, cfg.HealthCheck.Timeout)
	assert.Equal(t, 8, cfg.HealthCheck.Concurrency)
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("AGENTLENS_PORT", "9090")
	t.Setenv("AGENTLENS_LOG_LEVEL", "debug")
	t.Setenv("AGENTLENS_DATA_DIR", "/tmp/data")

	cfg, err := config.Load("")
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "/tmp/data", cfg.DataDir)
}

func TestYAMLLoad(t *testing.T) {
	content := `
port: 7070
data_dir: /var/data
log_level: warn
poll_interval: 10m
sources:
  - name: my-agent
    type: a2a
    url: http://example.com/agent
kubernetes:
  enabled: true
  namespaces: ["default", "agents"]
`
	f, err := os.CreateTemp("", "agentlens-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	cfg, err := config.Load(f.Name())
	require.NoError(t, err)
	assert.Equal(t, 7070, cfg.Port)
	assert.Equal(t, "/var/data", cfg.DataDir)
	assert.Equal(t, "warn", cfg.LogLevel)
	assert.Equal(t, 10*time.Minute, cfg.PollInterval)
	assert.Len(t, cfg.Sources, 1)
	assert.Equal(t, "my-agent", cfg.Sources[0].Name)
	assert.True(t, cfg.Kubernetes.Enabled)
	assert.Equal(t, []string{"default", "agents"}, cfg.Kubernetes.Namespaces)
}

package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/config"
)

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
	assert.Equal(t, 10, cfg.HealthCheck.Concurrency)
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("AGENTLENS_PORT", "9090")
	os.Setenv("AGENTLENS_LOG_LEVEL", "debug")
	os.Setenv("AGENTLENS_DATA_DIR", "/tmp/data")
	defer func() {
		os.Unsetenv("AGENTLENS_PORT")
		os.Unsetenv("AGENTLENS_LOG_LEVEL")
		os.Unsetenv("AGENTLENS_DATA_DIR")
	}()

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
	defer os.Remove(f.Name())
	_, err = f.WriteString(content)
	require.NoError(t, err)
	f.Close()

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

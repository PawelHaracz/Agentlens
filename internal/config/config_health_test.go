package config_test

import (
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/config"
)

func TestHealthCheckConfigDefaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HealthCheck.DegradedLatency != 1500*time.Millisecond {
		t.Errorf("DegradedLatency = %v, want 1500ms", cfg.HealthCheck.DegradedLatency)
	}
	if cfg.HealthCheck.FailureThreshold != 3 {
		t.Errorf("FailureThreshold = %v, want 3", cfg.HealthCheck.FailureThreshold)
	}
	if cfg.HealthCheck.Concurrency != 8 {
		t.Errorf("Concurrency = %v, want 8", cfg.HealthCheck.Concurrency)
	}
}

func TestHealthCheckConfigEnvOverride(t *testing.T) {
	t.Setenv("AGENTLENS_HEALTH_CHECK_DEGRADED_LATENCY", "2s")
	t.Setenv("AGENTLENS_HEALTH_CHECK_FAILURE_THRESHOLD", "5")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HealthCheck.DegradedLatency != 2*time.Second {
		t.Errorf("DegradedLatency = %v, want 2s", cfg.HealthCheck.DegradedLatency)
	}
	if cfg.HealthCheck.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %v, want 5", cfg.HealthCheck.FailureThreshold)
	}
}

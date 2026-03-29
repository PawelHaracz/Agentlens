// Package config handles loading and validating AgentLens configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// AuthConfig holds authentication configuration for a source.
type AuthConfig struct {
	Type     string `yaml:"type"`
	TokenEnv string `yaml:"token_env"`
}

// SourceConfig describes a static agent discovery source.
type SourceConfig struct {
	Name         string        `yaml:"name"`
	Type         string        `yaml:"type"` // a2a | mcp | a2ui
	URL          string        `yaml:"url"`
	PollInterval time.Duration `yaml:"poll_interval"`
	Auth         AuthConfig    `yaml:"auth"`
}

// KubernetesConfig holds Kubernetes discovery settings.
type KubernetesConfig struct {
	Enabled    bool     `yaml:"enabled"`
	Namespaces []string `yaml:"namespaces"`
}

// HealthCheckConfig holds health-check settings.
type HealthCheckConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Interval    time.Duration `yaml:"interval"`
	Timeout     time.Duration `yaml:"timeout"`
	Concurrency int           `yaml:"concurrency"`
}

// Config holds all AgentLens configuration.
type Config struct {
	Port         int               `yaml:"port"`
	DataDir      string            `yaml:"data_dir"`
	LogLevel     string            `yaml:"log_level"`
	PollInterval time.Duration     `yaml:"poll_interval"`
	Sources      []SourceConfig    `yaml:"sources"`
	Kubernetes   KubernetesConfig  `yaml:"kubernetes"`
	HealthCheck  HealthCheckConfig `yaml:"health_check"`
}

// Load reads configuration from a YAML file at path (may be empty) and applies
// environment-variable overrides prefixed with AGENTLENS_.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	applyEnv(cfg)
	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Port:         8080,
		DataDir:      "./data",
		LogLevel:     "info",
		PollInterval: 5 * time.Minute,
		HealthCheck: HealthCheckConfig{
			Enabled:     true,
			Interval:    30 * time.Second,
			Timeout:     5 * time.Second,
			Concurrency: 10,
		},
	}
}

func applyEnv(cfg *Config) {
	if v := env("PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Port = n
		}
	}
	if v := env("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := env("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := env("POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PollInterval = d
		}
	}
	if v := env("KUBERNETES_ENABLED"); v != "" {
		cfg.Kubernetes.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("HEALTH_CHECK_ENABLED"); v != "" {
		cfg.HealthCheck.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("HEALTH_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HealthCheck.Interval = d
		}
	}
	if v := env("HEALTH_CHECK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HealthCheck.Timeout = d
		}
	}
	if v := env("HEALTH_CHECK_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.HealthCheck.Concurrency = n
		}
	}
}

func env(key string) string {
	return os.Getenv("AGENTLENS_" + key)
}

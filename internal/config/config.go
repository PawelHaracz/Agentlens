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
	Enabled          bool          `yaml:"enabled"`
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	Concurrency      int           `yaml:"concurrency"`
	DegradedLatency  time.Duration `yaml:"degraded_latency"`
	FailureThreshold int           `yaml:"failure_threshold"`
}

// SQLiteConfig holds SQLite-specific database settings.
type SQLiteConfig struct {
	Path string `yaml:"path"`
}

// PostgresConfig holds PostgreSQL-specific database settings.
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// DSN returns a PostgreSQL connection string.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode)
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Dialect  string         `yaml:"dialect"`
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	Postgres PostgresConfig `yaml:"postgres"`
}

// AuthTokenConfig holds JWT and session settings.
type AuthTokenConfig struct {
	JWTSecret       string        `yaml:"jwt_secret"`
	SessionDuration time.Duration `yaml:"session_duration"`
}

// Config holds all AgentLens configuration.
type Config struct {
	Port         int               `yaml:"port"`
	DataDir      string            `yaml:"data_dir"`
	LogLevel     string            `yaml:"log_level"`
	LicenseKey   string            `yaml:"license_key"`
	PollInterval time.Duration     `yaml:"poll_interval"`
	Sources      []SourceConfig    `yaml:"sources"`
	Kubernetes   KubernetesConfig  `yaml:"kubernetes"`
	HealthCheck  HealthCheckConfig `yaml:"health_check"`
	Database     DatabaseConfig    `yaml:"database"`
	Auth         AuthTokenConfig   `yaml:"auth"`
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
			Enabled:          true,
			Interval:         30 * time.Second,
			Timeout:          5 * time.Second,
			Concurrency:      8,
			DegradedLatency:  1500 * time.Millisecond,
			FailureThreshold: 3,
		},
		Database: DatabaseConfig{
			Dialect: "sqlite",
			SQLite:  SQLiteConfig{Path: "./data/agentlens.db"},
			Postgres: PostgresConfig{
				Host:    "localhost",
				Port:    5432,
				User:    "agentlens",
				DBName:  "agentlens",
				SSLMode: "disable",
			},
		},
		Auth: AuthTokenConfig{
			SessionDuration: 24 * time.Hour,
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
	if v := env("LICENSE_KEY"); v != "" {
		cfg.LicenseKey = v
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
	if v := env("HEALTH_CHECK_DEGRADED_LATENCY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HealthCheck.DegradedLatency = d
		}
	}
	if v := env("HEALTH_CHECK_FAILURE_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.HealthCheck.FailureThreshold = n
		}
	}
	// Database env overrides
	if v := env("DB_DIALECT"); v != "" {
		cfg.Database.Dialect = v
	}
	if v := env("DB_SQLITE_PATH"); v != "" {
		cfg.Database.SQLite.Path = v
	}
	if v := env("DB_POSTGRES_HOST"); v != "" {
		cfg.Database.Postgres.Host = v
	}
	if v := env("DB_POSTGRES_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Database.Postgres.Port = n
		}
	}
	if v := env("DB_POSTGRES_USER"); v != "" {
		cfg.Database.Postgres.User = v
	}
	if v := env("DB_POSTGRES_PASSWORD"); v != "" {
		cfg.Database.Postgres.Password = v
	}
	if v := env("DB_POSTGRES_DBNAME"); v != "" {
		cfg.Database.Postgres.DBName = v
	}
	if v := env("DB_POSTGRES_SSLMODE"); v != "" {
		cfg.Database.Postgres.SSLMode = v
	}
	// Auth env overrides
	if v := env("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := env("SESSION_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Auth.SessionDuration = d
		}
	}
}

func env(key string) string {
	return os.Getenv("AGENTLENS_" + key)
}

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

// TelemetryConfig holds OpenTelemetry configuration.
type TelemetryConfig struct {
	Enabled          bool              `yaml:"enabled"`
	Endpoint         string            `yaml:"endpoint"`
	FrontendEndpoint string            `yaml:"frontend_endpoint"`
	Protocol         string            `yaml:"protocol"`
	Insecure         bool              `yaml:"insecure"`
	ServiceName      string            `yaml:"service_name"`
	Environment      string            `yaml:"environment"`
	TracesSampleRate float64           `yaml:"traces_sample_rate"`
	MetricsInterval  time.Duration     `yaml:"metrics_interval"`
	LogExportLevel   string            `yaml:"log_export_level"`
	Headers          map[string]string `yaml:"headers"`
	Prometheus       PrometheusConfig  `yaml:"prometheus"`
}

// PrometheusConfig holds Prometheus metrics endpoint configuration.
type PrometheusConfig struct {
	Enabled bool `yaml:"enabled"`
}

// MCPServerConfig holds MCP Discovery Server plugin settings.
type MCPServerConfig struct {
	Enabled        bool          `yaml:"enabled"`
	PublicURL      string        `yaml:"public_url"`
	AllowedOrigins []string      `yaml:"allowed_origins"`
	AuditEnabled   bool          `yaml:"audit_enabled"`
	SessionTTL     time.Duration `yaml:"session_ttl"`
	ReaperInterval time.Duration `yaml:"reaper_interval"`
}

// DexConfig holds Dex-specific federation configuration.
type DexConfig struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	JWKSURL      string `yaml:"jwks_url"`
}

// FederationConfig configures the optional OIDC federation provider.
// Only the "dex" provider is supported in v1.
type FederationConfig struct {
	Enabled  bool      `yaml:"enabled"`
	Provider string    `yaml:"provider"` // "dex" or ""
	Audience string    `yaml:"audience"` // expected JWT aud claim (typically the MCP public URL)
	Dex      DexConfig `yaml:"dex"`
}

// Validate returns an error if the federation config is inconsistent.
func (f *FederationConfig) Validate() error {
	if !f.Enabled {
		return nil
	}
	if f.Provider == "" {
		return fmt.Errorf("federation.provider is required when federation is enabled")
	}
	if f.Provider != "dex" {
		return fmt.Errorf("federation.provider %q is not supported; only \"dex\" is supported in v1", f.Provider)
	}
	if f.Dex.Issuer == "" {
		return fmt.Errorf("federation.dex.issuer is required when provider=dex")
	}
	if f.Audience == "" {
		return fmt.Errorf("federation.audience is required when federation is enabled")
	}
	return nil
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
	Telemetry    TelemetryConfig   `yaml:"telemetry"`
	MCP          MCPServerConfig   `yaml:"mcp_server"`
	Federation   FederationConfig  `yaml:"federation"`
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
		Telemetry: TelemetryConfig{
			Enabled:          false,
			Protocol:         "grpc",
			Insecure:         true,
			ServiceName:      "agentlens",
			Environment:      "production",
			TracesSampleRate: 1.0,
			MetricsInterval:  30 * time.Second,
			LogExportLevel:   "info",
		},
		MCP: MCPServerConfig{
			Enabled:        false,
			AuditEnabled:   true,
			SessionTTL:     30 * time.Minute,
			ReaperInterval: 60 * time.Second,
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
	applyHealthCheckEnv(&cfg.HealthCheck)
	applyDatabaseEnv(&cfg.Database)
	applyTelemetryEnv(&cfg.Telemetry)
	applyMCPEnv(&cfg.MCP)
	applyFederationEnv(&cfg.Federation)
	if v := env("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := env("SESSION_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Auth.SessionDuration = d
		}
	}
}

func applyHealthCheckEnv(hc *HealthCheckConfig) {
	if v := env("HEALTH_CHECK_ENABLED"); v != "" {
		hc.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("HEALTH_CHECK_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			hc.Interval = d
		}
	}
	if v := env("HEALTH_CHECK_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			hc.Timeout = d
		}
	}
	if v := env("HEALTH_CHECK_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			hc.Concurrency = n
		}
	}
	if v := env("HEALTH_CHECK_DEGRADED_LATENCY"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			hc.DegradedLatency = d
		}
	}
	if v := env("HEALTH_CHECK_FAILURE_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			hc.FailureThreshold = n
		}
	}
}

func applyDatabaseEnv(db *DatabaseConfig) {
	if v := env("DB_DIALECT"); v != "" {
		db.Dialect = v
	}
	if v := env("DB_SQLITE_PATH"); v != "" {
		db.SQLite.Path = v
	}
	if v := env("DB_POSTGRES_HOST"); v != "" {
		db.Postgres.Host = v
	}
	if v := env("DB_POSTGRES_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			db.Postgres.Port = n
		}
	}
	if v := env("DB_POSTGRES_USER"); v != "" {
		db.Postgres.User = v
	}
	if v := env("DB_POSTGRES_PASSWORD"); v != "" {
		db.Postgres.Password = v
	}
	if v := env("DB_POSTGRES_DBNAME"); v != "" {
		db.Postgres.DBName = v
	}
	if v := env("DB_POSTGRES_SSLMODE"); v != "" {
		db.Postgres.SSLMode = v
	}
}

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
	if v := env("OTEL_FRONTEND_ENDPOINT"); v != "" {
		tel.FrontendEndpoint = v
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

func env(key string) string {
	return os.Getenv("AGENTLENS_" + key)
}

func applyMCPEnv(m *MCPServerConfig) {
	if v := env("MCP_ENABLED"); v != "" {
		m.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("MCP_PUBLIC_URL"); v != "" {
		m.PublicURL = v
	}
	if v := env("MCP_ALLOWED_ORIGINS"); v != "" {
		m.AllowedOrigins = strings.Split(v, ",")
	}
	if v := env("MCP_AUDIT_ENABLED"); v != "" {
		m.AuditEnabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("MCP_SESSION_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			m.SessionTTL = d
		}
	}
	if v := env("MCP_REAPER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			m.ReaperInterval = d
		}
	}
}

func applyFederationEnv(f *FederationConfig) {
	if v := env("FEDERATION_ENABLED"); v != "" {
		f.Enabled = strings.EqualFold(v, "true") || v == "1"
	}
	if v := env("FEDERATION_PROVIDER"); v != "" {
		f.Provider = v
	}
	if v := env("FEDERATION_AUDIENCE"); v != "" {
		f.Audience = v
	}
	if v := env("FEDERATION_DEX_ISSUER"); v != "" {
		f.Dex.Issuer = v
	}
	if v := env("FEDERATION_DEX_CLIENT_ID"); v != "" {
		f.Dex.ClientID = v
	}
	if v := env("FEDERATION_DEX_CLIENT_SECRET"); v != "" {
		f.Dex.ClientSecret = v
	}
	if v := env("FEDERATION_DEX_JWKS_URL"); v != "" {
		f.Dex.JWKSURL = v
	}
}

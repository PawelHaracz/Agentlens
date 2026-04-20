package mcpserver

import (
	"fmt"
	"time"

	"github.com/PawelHaracz/agentlens/internal/config"
)

// pluginConfig holds the resolved, validated MCP plugin configuration.
type pluginConfig struct {
	enabled        bool
	publicURL      string
	allowedOrigins []string
	auditEnabled   bool
	sessionTTL     time.Duration
	reaperInterval time.Duration
}

// validate returns an error when Enabled=true but required fields are empty.
func validate(cfg config.MCPServerConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.PublicURL == "" {
		return fmt.Errorf("mcp_server.public_url is required when mcp_server.enabled=true")
	}
	if cfg.SessionTTL <= 0 {
		return fmt.Errorf("mcp_server.session_ttl must be positive")
	}
	return nil
}

func resolveConfig(cfg config.MCPServerConfig) pluginConfig {
	ttl := cfg.SessionTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	ri := cfg.ReaperInterval
	if ri <= 0 {
		ri = 60 * time.Second
	}
	return pluginConfig{
		enabled:        cfg.Enabled,
		publicURL:      cfg.PublicURL,
		allowedOrigins: cfg.AllowedOrigins,
		auditEnabled:   cfg.AuditEnabled,
		sessionTTL:     ttl,
		reaperInterval: ri,
	}
}

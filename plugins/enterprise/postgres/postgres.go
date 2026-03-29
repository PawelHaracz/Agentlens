// Package postgres provides the PostgreSQL store backend enterprise plugin.
package postgres

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/kernel"
)

const featureName = "postgres"

// Plugin implements the PostgreSQL store backend plugin.
type Plugin struct{}

// New creates a new PostgreSQL plugin.
func New() *Plugin { return &Plugin{} }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "enterprise-postgres" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeStore }

// Init checks for the postgres enterprise license feature.
func (p *Plugin) Init(k kernel.Kernel) error {
	if !k.License().HasFeature(featureName) {
		return fmt.Errorf("feature %q: %w", featureName, kernel.ErrLicenseRequired)
	}
	// Future: initialize PostgreSQL connection pool, run migrations
	return nil
}

// Start starts the plugin (no-op for stub).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for stub).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Package audit provides the audit logging enterprise plugin.
package audit

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/kernel"
)

const featureName = "audit"

// Plugin implements the audit logging plugin.
type Plugin struct{}

// New creates a new audit plugin.
func New() *Plugin { return &Plugin{} }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "enterprise-audit" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeMiddleware }

// Init checks for the audit enterprise license feature.
func (p *Plugin) Init(k kernel.Kernel) error {
	if !k.License().HasFeature(featureName) {
		return fmt.Errorf("feature %q: %w", featureName, kernel.ErrLicenseRequired)
	}
	// Future: configure audit log destination, retention policy
	return nil
}

// Start starts the plugin (no-op for stub).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for stub).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

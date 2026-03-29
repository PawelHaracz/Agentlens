// Package rbac provides the role-based access control enterprise plugin.
package rbac

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/kernel"
)

const featureName = "rbac"

// Plugin implements the RBAC middleware plugin.
type Plugin struct{}

// New creates a new RBAC plugin.
func New() *Plugin { return &Plugin{} }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "enterprise-rbac" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeMiddleware }

// Init checks for the RBAC enterprise license feature.
func (p *Plugin) Init(k kernel.Kernel) error {
	if !k.License().HasFeature(featureName) {
		return fmt.Errorf("feature %q: %w", featureName, kernel.ErrLicenseRequired)
	}
	// Future: configure team-scoped access control policies
	return nil
}

// Start starts the plugin (no-op for stub).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for stub).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Package sso provides the OIDC single sign-on enterprise plugin.
package sso

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/kernel"
)

const featureName = "sso"

// Plugin implements the SSO/OIDC middleware plugin.
type Plugin struct{}

// New creates a new SSO plugin.
func New() *Plugin { return &Plugin{} }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "enterprise-sso" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeMiddleware }

// Init checks for the SSO enterprise license feature.
func (p *Plugin) Init(k kernel.Kernel) error {
	if !k.License().HasFeature(featureName) {
		return fmt.Errorf("feature %q: %w", featureName, kernel.ErrLicenseRequired)
	}
	// Future: configure OIDC provider, client ID, redirect URIs
	return nil
}

// Start starts the plugin (no-op for stub).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for stub).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

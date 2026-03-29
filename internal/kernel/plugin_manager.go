package kernel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// ErrLicenseRequired is returned when a plugin requires an enterprise license.
var ErrLicenseRequired = errors.New("enterprise license required")

// PluginManager manages the lifecycle of all plugins.
type PluginManager struct {
	plugins []Plugin
	core    *Core
	log     *slog.Logger
}

// NewPluginManager creates a new PluginManager.
func NewPluginManager(core *Core) *PluginManager {
	return &PluginManager{
		core: core,
		log:  core.Logger().With("component", "plugin-manager"),
	}
}

// Register adds a plugin to the manager.
func (pm *PluginManager) Register(p Plugin) {
	pm.plugins = append(pm.plugins, p)
}

// InitAll initializes all registered plugins.
// Plugins that return ErrLicenseRequired are skipped with a warning.
func (pm *PluginManager) InitAll() error {
	for _, p := range pm.plugins {
		if err := p.Init(pm.core); err != nil {
			if errors.Is(err, ErrLicenseRequired) {
				pm.log.Warn("plugin skipped: enterprise license required",
					"plugin", p.Name(), "type", p.Type())
				continue
			}
			return fmt.Errorf("initializing plugin %s: %w", p.Name(), err)
		}

		// Register parser plugins with the kernel
		if pp, ok := p.(ParserPlugin); ok {
			pm.core.RegisterParser(pp)
		}

		pm.log.Info("plugin initialized", "plugin", p.Name(), "type", p.Type(), "version", p.Version())
	}
	return nil
}

// StartAll starts all initialized plugins.
func (pm *PluginManager) StartAll(ctx context.Context) error {
	for _, p := range pm.plugins {
		if err := p.Start(ctx); err != nil {
			if errors.Is(err, ErrLicenseRequired) {
				continue
			}
			return fmt.Errorf("starting plugin %s: %w", p.Name(), err)
		}
	}
	return nil
}

// StopAll stops all plugins in reverse order.
func (pm *PluginManager) StopAll(ctx context.Context) error {
	var errs []error
	for i := len(pm.plugins) - 1; i >= 0; i-- {
		if err := pm.plugins[i].Stop(ctx); err != nil {
			if !errors.Is(err, ErrLicenseRequired) {
				errs = append(errs, fmt.Errorf("stopping plugin %s: %w", pm.plugins[i].Name(), err))
			}
		}
	}
	return errors.Join(errs...)
}

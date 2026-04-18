// Package mcpserver implements the MCP Discovery Server plugin.
// It exposes a read-only MCP endpoint at /api/mcp using the Streamable HTTP
// transport (MCP spec 2025-11-25), with dual auth (service-account API keys +
// OAuth via Dex) handled by the composition-root middleware chain.
//
// Plugin is behind mcp_server.enabled=false by default.
// All work (routes, session reaper, async worker) is skipped when disabled.
package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver/wire"
)

// Plugin implements the MCP Discovery Server kernel.Plugin.
// Struct is named Plugin — arch-go's Plugin-suffix rule is satisfied via the
// mcpserver package namespace (plugins/mcpserver.Plugin).
type Plugin struct {
	cfg      pluginConfig
	sessions *sessionManager
	worker   *asyncWorker
	registry ToolRegistry

	// raw handler exposed for composition-root wrapping
	handler *dispatcher

	cancelReaper context.CancelFunc
	startedAt    time.Time
	log          *slog.Logger
}

// New creates a Plugin. sessionStore must not be nil.
func New(store sessionStore) *Plugin {
	return &Plugin{sessions: newSessionManager(store, 30*time.Minute)}
}

// NewForTest creates a Plugin for unit tests.
func NewForTest(store sessionStore) *Plugin { return New(store) }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "mcp-discovery-server" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeMiddleware }

// SetRegistry allows Group D to inject the ToolRegistry before Start.
func (p *Plugin) SetRegistry(r ToolRegistry) { p.registry = r }

// Handler returns the raw http.Handler for the MCP endpoint.
// The composition root wraps it with Origin → AuthDispatch → Scope before
// calling kernel.RegisterRoutes("/api/mcp", wrapped).
func (p *Plugin) Handler() *dispatcher { return p.handler }

// Init configures the plugin from kernel.
func (p *Plugin) Init(k kernel.Kernel) error {
	cfg := k.Config().MCP
	if err := validate(cfg); err != nil {
		return fmt.Errorf("mcp plugin config: %w", err)
	}
	p.cfg = resolveConfig(cfg)
	p.log = k.Logger().With("component", "mcp-discovery-server")

	if !p.cfg.enabled {
		p.log.Info("mcp plugin disabled — skipping route registration")
		return nil
	}

	// Update session TTL from resolved config.
	p.sessions.ttl = p.cfg.sessionTTL

	// Build the JSON-RPC dispatcher.
	p.worker = newAsyncWorker(p.sessions.store)
	p.handler = &dispatcher{
		sessions: p.sessions,
		registry: p.registry,
		cfg:      p.cfg,
		worker:   p.worker,
	}

	// Build the Streamable HTTP transport wrapping the dispatcher.
	transport := wire.NewStreamableHTTP(p.handler, p.sessions.IsActive, nil)
	k.RegisterRoutes("/api/mcp", transport.Handler())
	k.RegisterRoutes("/api/mcp/status", newStatusHandler(p.sessions, time.Now()))

	return nil
}

// Start launches the session reaper and async last_seen updater.
func (p *Plugin) Start(ctx context.Context) error {
	if !p.cfg.enabled {
		return nil
	}
	p.startedAt = time.Now()
	reaperCtx, cancel := context.WithCancel(ctx)
	p.cancelReaper = cancel

	go p.runReaper(reaperCtx)
	go p.worker.Run(reaperCtx)
	return nil
}

// Stop flushes the async worker and cancels the reaper.
func (p *Plugin) Stop(_ context.Context) error {
	if p.cancelReaper != nil {
		p.cancelReaper()
	}
	if p.worker != nil {
		p.worker.Flush()
	}
	return nil
}

func (p *Plugin) runReaper(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.reaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.sessions.Reap(ctx)
		}
	}
}

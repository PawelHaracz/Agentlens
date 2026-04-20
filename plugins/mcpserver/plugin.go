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
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	mcptools "github.com/PawelHaracz/agentlens/plugins/mcpserver/tools"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver/wire"
)

// Plugin implements the MCP Discovery Server kernel.Plugin.
type Plugin struct {
	cfg          pluginConfig
	sessions     *sessionManager
	worker       *asyncWorker
	registry     ToolRegistry
	loopback     mcptools.LoopbackFunc
	metrics      *mcpMetrics
	catalogStore store.Store

	handler          *dispatcher  // JSON-RPC dispatcher (inner)
	transportHandler http.Handler // Streamable HTTP transport (wraps dispatcher; exposed via Handler())
	cancelReaper     context.CancelFunc
	startedAt        time.Time
	log              *slog.Logger
}

// New creates a Plugin. sessionStore must not be nil.
func New(sess sessionStore) *Plugin {
	return &Plugin{sessions: newSessionManager(sess, 30*time.Minute)}
}

// NewForTest creates a Plugin for unit tests.
func NewForTest(sess sessionStore) *Plugin { return New(sess) }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "mcp-discovery-server" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeMiddleware }

// SetLoopback injects the loopback function and builds the ToolRegistry.
// Called by the composition root after pm.InitAll and before pm.StartAll.
func (p *Plugin) SetLoopback(fn mcptools.LoopbackFunc) {
	p.loopback = fn
	if p.registry == nil && fn != nil {
		reg := mcptools.New()
		mcptools.RegisterAll(reg, fn)
		p.registry = reg
		if p.handler != nil {
			p.handler.registry = reg
		}
	}
}

// Handler returns the Streamable HTTP transport wrapping the JSON-RPC dispatcher.
// Composition root wraps it with Origin → AuthDispatch → Scope before
// calling kernel.RegisterRoutes("/api/mcp", wrapped). The transport enforces
// session validation, protocol headers, and GET/SSE routing; the wrapped
// handler must NOT bypass these by pointing at the raw dispatcher.
func (p *Plugin) Handler() http.Handler { return p.transportHandler }

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

	// F.8: warn when audit logging is disabled.
	if !p.cfg.auditEnabled {
		slog.Warn("MCP audit logging disabled — forensic trail unavailable")
	}

	p.catalogStore = k.Store()
	p.metrics = newMCPMetrics()
	p.sessions.ttl = p.cfg.sessionTTL

	p.worker = newAsyncWorker(p.sessions.store)
	p.handler = &dispatcher{
		sessions: p.sessions,
		registry: p.registry,
		cfg:      p.cfg,
		worker:   p.worker,
	}

	transport := wire.NewStreamableHTTP(p.handler, p.sessions.IsActive, nil)
	p.transportHandler = transport.Handler()
	k.RegisterRoutes("/api/mcp", p.transportHandler)
	k.RegisterRoutes("/api/mcp/status", newStatusHandler(p.sessions, time.Now()))

	// F.2: Self-register as a catalog entry (idempotent).
	if err := p.selfRegister(context.Background()); err != nil {
		p.log.Warn("mcp self-registration failed (non-fatal)", "err", err)
	}

	return nil
}

// selfRegister creates or updates the MCP server's own catalog entry.
// Endpoint is disambiguated by PublicURL so multi-instance deploys get distinct keys (M6).
func (p *Plugin) selfRegister(ctx context.Context) error {
	if p.catalogStore == nil {
		return nil // no store injected (tests or disabled mode)
	}
	endpoint := "agentlens:mcp-discovery:" + p.cfg.publicURL
	agentKey := model.ComputeAgentKey(model.ProtocolMCP, endpoint)

	existing, err := p.catalogStore.FindByEndpoint(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("finding existing catalog entry: %w", err)
	}
	if existing != nil {
		return nil // already registered; idempotent
	}

	agentTypeID := uuid.New().String()
	entry := &model.CatalogEntry{
		ID:          uuid.New().String(),
		DisplayName: "AgentLens MCP Discovery Server",
		Description: "Read-only MCP Discovery Server embedded in AgentLens. " +
			"Exposes 4 tools: agent_search, agent_get, capabilities_list, agent_card.",
		Source: model.SourcePush,
		Status: model.LifecycleRegistered,
		AgentType: &model.AgentType{
			ID:       agentTypeID,
			AgentKey: agentKey,
			Protocol: model.ProtocolMCP,
			Endpoint: endpoint,
			Version:  "1.0.0",
		},
	}
	if err := p.catalogStore.Create(ctx, entry); err != nil {
		return fmt.Errorf("creating self-registration entry: %w", err)
	}
	p.log.Info("mcp: self-registered in catalog", "entry_id", entry.ID)
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

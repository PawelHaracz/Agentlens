package kernel

import (
	"log/slog"
	"net/http"

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// Core implements the Kernel interface.
type Core struct {
	store       store.Store
	config      *config.Config
	logger      *slog.Logger
	license     LicenseInfo
	parsers     map[model.Protocol]ParserPlugin
	routes      map[string]http.Handler
	middlewares []func(http.Handler) http.Handler
}

// NewCore creates a new Core kernel.
func NewCore(s store.Store, cfg *config.Config, logger *slog.Logger, lic LicenseInfo) *Core {
	return &Core{
		store:   s,
		config:  cfg,
		logger:  logger,
		license: lic,
		parsers: make(map[model.Protocol]ParserPlugin),
		routes:  make(map[string]http.Handler),
	}
}

// Store returns the data store.
func (c *Core) Store() store.Store { return c.store }

// Config returns the configuration.
func (c *Core) Config() *config.Config { return c.config }

// Logger returns the logger.
func (c *Core) Logger() *slog.Logger { return c.logger }

// License returns the license info.
func (c *Core) License() LicenseInfo { return c.license }

// Parser returns the parser plugin for the given protocol.
func (c *Core) Parser(protocol model.Protocol) (ParserPlugin, bool) {
	p, ok := c.parsers[protocol]
	return p, ok
}

// RegisterParser registers a parser plugin for a protocol.
func (c *Core) RegisterParser(p ParserPlugin) {
	c.parsers[p.Protocol()] = p
}

// RegisterRoutes registers route handlers for a prefix.
func (c *Core) RegisterRoutes(prefix string, handler http.Handler) {
	c.routes[prefix] = handler
}

// RegisterMiddleware registers HTTP middleware.
func (c *Core) RegisterMiddleware(mw func(http.Handler) http.Handler) {
	c.middlewares = append(c.middlewares, mw)
}

// Routes returns all registered route handlers.
func (c *Core) Routes() map[string]http.Handler { return c.routes }

// Middlewares returns all registered middleware.
func (c *Core) Middlewares() []func(http.Handler) http.Handler { return c.middlewares }

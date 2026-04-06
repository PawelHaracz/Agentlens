package kernel

import "net/http"

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

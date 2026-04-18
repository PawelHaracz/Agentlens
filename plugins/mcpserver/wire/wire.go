// Package wire defines the MCP transport interface and the Streamable HTTP
// implementation (MCP spec 2025-11-25). The WireImpl interface allows the
// concrete transport to be swapped in a future release (e.g. for an
// official Go MCP SDK) without changing the plugin core.
package wire

import "net/http"

// WireImpl is the transport contract: given a JSON-RPC dispatcher handler,
// it returns the HTTP handler to mount at the MCP route prefix.
// POST dispatches JSON-RPC; GET streams server-sent events.
type WireImpl interface {
	// Handler returns the http.Handler for the MCP endpoint.
	// The dispatcher receives decoded JSON-RPC calls.
	Handler() http.Handler
}

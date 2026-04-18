package wire

import (
	"fmt"
	"net/http"
)

const (
	headerSessionID       = "MCP-Session-Id"
	headerProtocolVersion = "MCP-Protocol-Version"
	// CurrentProtocolVersion is the MCP spec version this transport targets.
	CurrentProtocolVersion = "2025-11-25"
)

// HeaderSessionID returns the MCP-Session-Id header name.
func HeaderSessionID() string { return headerSessionID }

// ProtocolVersionHeader returns the MCP-Protocol-Version header name.
func ProtocolVersionHeader() string { return headerProtocolVersion }

// StreamableHTTP is the MCP Streamable HTTP transport (spec 2025-11-25).
// POST carries JSON-RPC requests; GET streams server-sent events.
//
// The dispatcher is a raw http.Handler (produced by the plugin) that handles
// the JSON-RPC payload. The transport adds session-id management, protocol
// version echoing, and SSE scaffolding around it.
type StreamableHTTP struct {
	dispatcher http.Handler
	isActive   func(sessionID string) bool
	newSession func(w http.ResponseWriter, r *http.Request) (string, error)
}

// NewStreamableHTTP creates a StreamableHTTP transport.
//   - dispatcher: handles JSON-RPC once transport validation passes
//   - isActive: returns true if the session ID is live
//   - newSession: creates a session row and returns the new session ID
func NewStreamableHTTP(
	dispatcher http.Handler,
	isActive func(string) bool,
	newSession func(http.ResponseWriter, *http.Request) (string, error),
) *StreamableHTTP {
	return &StreamableHTTP{
		dispatcher: dispatcher,
		isActive:   isActive,
		newSession: newSession,
	}
}

// Handler returns the http.Handler for POST+GET at the /api/mcp prefix.
func (s *StreamableHTTP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.route)
	return mux
}

func (s *StreamableHTTP) route(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handlePost(w, r)
	case http.MethodGet:
		s.handleGet(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePost processes a JSON-RPC request.
// On `initialize` method: creates a session and returns MCP-Session-Id.
// All other methods require a valid session ID in the request.
func (s *StreamableHTTP) handlePost(w http.ResponseWriter, r *http.Request) {
	// Echo the MCP-Protocol-Version header per spec §5.2.
	proto := r.Header.Get(headerProtocolVersion)
	if proto == "" {
		proto = CurrentProtocolVersion
	}
	w.Header().Set(headerProtocolVersion, proto)

	sessionID := r.Header.Get(headerSessionID)

	// initialize is the only method allowed without a pre-existing session.
	// The dispatcher will call newSession when it processes an "initialize" method.
	if sessionID == "" {
		// Delegate to dispatcher; it will call newSession if the method is "initialize".
		s.dispatcher.ServeHTTP(w, r.WithContext(
			withNewSessionFn(r.Context(), func(id string) {
				w.Header().Set(headerSessionID, id)
			}),
		))
		return
	}

	if !s.isActive(sessionID) {
		http.Error(w, fmt.Sprintf(`{"error":"unknown or expired session %q"}`, sessionID), http.StatusNotFound)
		return
	}

	s.dispatcher.ServeHTTP(w, r)
}

// handleGet serves the server-sent events stream for server→client messages.
// v1 holds the connection open; actual push is not yet implemented.
func (s *StreamableHTTP) handleGet(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get(headerSessionID)
	if sessionID == "" || !s.isActive(sessionID) {
		http.Error(w, `{"error":"valid session required for SSE"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set(headerProtocolVersion, CurrentProtocolVersion)
	w.WriteHeader(http.StatusOK)

	// Block until client disconnects (server-initiated messages deferred to v2).
	flusher, ok := w.(http.Flusher)
	if ok {
		flusher.Flush()
	}
	<-r.Context().Done()
}

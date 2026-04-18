package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// statusResponse is the body returned by GET /api/mcp/status.
type statusResponse struct {
	Enabled        bool      `json:"enabled"`
	ActiveSessions int64     `json:"active_sessions"`
	UptimeSeconds  float64   `json:"uptime_seconds"`
	StartedAt      time.Time `json:"started_at"`
}

// newStatusHandler returns a handler for /api/mcp/status.
// The handler is registered separately from the main MCP route so unauthenticated
// clients can check whether the plugin is running.
func newStatusHandler(sm *sessionManager, startedAt time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active, err := sm.CountActive(context.Background())
		if err != nil {
			slog.WarnContext(r.Context(), "mcp: status: failed to count sessions", "err", err)
		}

		resp := statusResponse{
			Enabled:        true,
			ActiveSessions: active,
			UptimeSeconds:  time.Since(startedAt).Seconds(),
			StartedAt:      startedAt,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.ErrorContext(r.Context(), "mcp: status encode error", "err", err)
		}
	})
}

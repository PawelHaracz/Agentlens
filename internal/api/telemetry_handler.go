package api

import (
	"net/http"
)

// NewReadyzHandler creates a readiness probe handler.
// pingFn should return nil if the database is reachable.
func NewReadyzHandler(pingFn func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pingFn(); err != nil {
			JSONResponse(w, http.StatusServiceUnavailable, map[string]string{
				"status": "error",
				"reason": "database unreachable",
			})
			return
		}
		JSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

type telemetryConfigResponse struct {
	Enabled     bool   `json:"enabled"`
	Endpoint    string `json:"endpoint,omitempty"`
	ServiceName string `json:"serviceName,omitempty"`
}

// NewTelemetryConfigHandler creates a public handler for frontend telemetry configuration.
// The frontend fetches this on startup to decide whether to initialize OTel.
func NewTelemetryConfigHandler(enabled bool, endpoint, serviceName string) http.HandlerFunc {
	var resp telemetryConfigResponse
	if enabled {
		resp = telemetryConfigResponse{
			Enabled:     true,
			Endpoint:    endpoint,
			ServiceName: serviceName,
		}
	}
	// When disabled, resp is zero value: {Enabled: false}
	return func(w http.ResponseWriter, r *http.Request) {
		JSONResponse(w, http.StatusOK, resp)
	}
}

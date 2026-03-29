// Package api provides HTTP handlers and middleware for AgentLens.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// JSONResponse writes a JSON response with the given status code.
func JSONResponse(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "err", err)
	}
}

// ErrorResponse writes a JSON error response.
func ErrorResponse(w http.ResponseWriter, status int, msg string) {
	JSONResponse(w, status, map[string]string{"error": msg})
}

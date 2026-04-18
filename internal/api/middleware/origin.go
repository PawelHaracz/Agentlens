package middleware

import (
	"net/http"
	"slices"
)

// OriginValidation returns a middleware that enforces the MCP spec requirement:
// requests to MCP endpoints MUST include an Origin header, and the Origin value
// MUST appear in the allowlist (if allowlist is non-empty).
//
// Empty allowlist with present Origin → 403 (strict default).
// Non-empty allowlist with Origin in list → pass through.
// Missing Origin header → 403 always.
//
// Per spec §5.5 and M-new-2 (scoped CORS — global CORS untouched).
func OriginValidation(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				http.Error(w, `{"error":"missing Origin header"}`, http.StatusForbidden)
				return
			}
			if len(allowedOrigins) > 0 && !slices.Contains(allowedOrigins, origin) {
				http.Error(w, `{"error":"Origin not allowed"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

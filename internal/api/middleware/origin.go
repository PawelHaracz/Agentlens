package middleware

import (
	"net/http"
	"slices"
)

// OriginValidation returns a middleware that enforces the MCP spec requirement
// for the /api/mcp route group. This middleware is NOT applied globally.
//
// Strict-default policy (spec §5.5, scope-clarifications §2):
//   - Missing Origin header                  → 403 always
//   - Present Origin, empty allowlist        → 403 (operator must explicitly allow)
//   - Present Origin, not in allowlist       → 403
//   - Present Origin, found in allowlist     → pass through
//
// The global CORSMiddleware (Access-Control-Allow-Origin: *) is untouched.
func OriginValidation(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				http.Error(w, `{"error":"missing Origin header"}`, http.StatusForbidden)
				return
			}
			if !slices.Contains(allowedOrigins, origin) {
				http.Error(w, `{"error":"Origin not allowed"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

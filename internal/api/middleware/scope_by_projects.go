package middleware

import (
	"net/http"

	"github.com/PawelHaracz/agentlens/internal/model/ctxkey"
)

// ScopeByAccessibleProjects injects the principal's accessible project IDs
// from ctx into the request context so downstream catalog handlers can
// build CatalogFilter.ProjectIDs without touching the URL (M4 resolution).
//
// If the principal has no explicit project IDs (nil or empty slice),
// no injection occurs and the handler falls back to default-project-only
// scoping at the store level.
//
// This middleware reads from ctxkey.ProjectIDs and writes nothing to the URL —
// user-supplied ?projects= query params have no effect on MCP-dispatched calls.
func ScopeByAccessibleProjects(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := ctxkey.ProjectIDs(r.Context())
		if len(ids) > 0 {
			ctx := ctxkey.WithProjectIDs(r.Context(), ids)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

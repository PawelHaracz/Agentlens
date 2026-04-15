package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/store"
)

type ancestorCacheKey struct{}

// RequireProjectPermission checks that the authenticated user has the given permission
// in the project identified by the chi URL parameter projectIDParam.
//
// Resolution order:
//  1. Global bypass: if user's JWT permissions contain `permission` → ALLOW
//  2. Group global roles: if any ancestor group carries a global role with `permission` → ALLOW
//  3. Project roles: if user or ancestor group has a project_member relationship to the project
//     with a role that grants `permission` → ALLOW
//  4. DENY
func RequireProjectPermission(ps *store.PartyStore, us *store.UserStore, projectIDParam, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Step 1: global bypass (from JWT context — no DB call)
			if auth.HasPermission(PermissionsFromContext(ctx), permission) {
				next.ServeHTTP(w, r)
				return
			}

			userID := UserIDFromContext(ctx)
			projectID := chi.URLParam(r, projectIDParam)

			// Resolve user's party
			userParty, err := ps.GetPartyByUserID(ctx, userID)
			if err != nil || userParty == nil {
				ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			// Get/cache ancestor IDs for this request
			ancestorIDs, ctx := cachedAncestorIDs(ctx, ps, userParty.ID)

			// Step 2: group global roles
			if len(ancestorIDs) > 0 {
				groupRoles, err := us.GetRolesForParties(ctx, ancestorIDs)
				if err == nil {
					for _, role := range groupRoles {
						if auth.HasPermission([]string(role.Permissions), permission) {
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}

			// Step 3: project-scoped roles
			fromIDs := append([]string{userParty.ID}, ancestorIDs...)
			projectRoles, err := ps.GetProjectRoles(ctx, fromIDs, projectID)
			if err == nil {
				for _, role := range projectRoles {
					if auth.ProjectRoleHasPermission(role, permission) {
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}

			ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
		})
	}
}

// cachedAncestorIDs returns ancestor group IDs from context cache (fast path)
// or from the DB (first call per request). Returns an updated context with the cache set.
// Returns a defensive copy of the slice so callers cannot mutate the cached value.
func cachedAncestorIDs(ctx context.Context, ps *store.PartyStore, partyID string) ([]string, context.Context) {
	if cached, ok := ctx.Value(ancestorCacheKey{}).([]string); ok {
		return append([]string(nil), cached...), ctx
	}
	ids, err := ps.AncestorGroupIDs(ctx, partyID)
	if err != nil {
		ids = nil
	}
	// Store a copy in the cache; return a copy to the caller
	cached := append([]string(nil), ids...)
	return cached, context.WithValue(ctx, ancestorCacheKey{}, cached)
}

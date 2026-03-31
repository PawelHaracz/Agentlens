package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/PawelHaracz/agentlens/internal/auth"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const (
	ctxUserID      contextKey = "userID"
	ctxUsername     contextKey = "username"
	ctxRoleID      contextKey = "roleID"
	ctxPermissions contextKey = "permissions"
)

// RequireAuth middleware validates JWT tokens from Authorization header or cookie.
func RequireAuth(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenString string

			// Try Authorization: Bearer <token> header first.
			if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			// Fall back to agentlens_token cookie.
			if tokenString == "" {
				if cookie, err := r.Cookie("agentlens_token"); err == nil {
					tokenString = cookie.Value
				}
			}

			if tokenString == "" {
				ErrorResponse(w, http.StatusUnauthorized, "authentication required")
				return
			}

			claims, err := jwtService.ValidateToken(tokenString)
			if err != nil {
				ErrorResponse(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Set user info in context.
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxUsername, claims.Username)
			ctx = context.WithValue(ctx, ctxRoleID, claims.RoleID)
			ctx = context.WithValue(ctx, ctxPermissions, claims.Permissions)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission middleware checks if the authenticated user has the required permission.
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			perms := PermissionsFromContext(r.Context())
			if !auth.HasPermission(perms, perm) {
				ErrorResponse(w, http.StatusForbidden, "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromContext returns the user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

// UsernameFromContext returns the username from the request context.
func UsernameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxUsername).(string); ok {
		return v
	}
	return ""
}

// RoleIDFromContext returns the role ID from the request context.
func RoleIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRoleID).(string); ok {
		return v
	}
	return ""
}

// PermissionsFromContext returns the permissions from the request context.
func PermissionsFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(ctxPermissions).([]string); ok {
		return v
	}
	return nil
}

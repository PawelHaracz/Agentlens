// Package middleware provides MCP-specific HTTP middleware.
package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/auth/apikey"
	"github.com/PawelHaracz/agentlens/internal/auth/federation"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/model/ctxkey"
)

// ProjectResolver resolves accessible project IDs for a principal.
// For service accounts: scopes determine access. For users: party memberships.
type ProjectResolver interface {
	AccessibleProjectIDs(ctx context.Context, principalID string, kind model.PrincipalType) ([]string, error)
}

// AuthDispatch is a middleware that authenticates MCP requests via three ordered
// paths (API key → local JWT → federation JWT), normalising the result to a
// *model.SessionPrincipalRef stored in ctx.
//
// Dispatch order per spec §3:
//  1. Authorization: Bearer agentlens_sk_... → API-key path
//  2. Authorization: Bearer <jwt> validated by local JWTService → user_local
//  3. Authorization: Bearer <jwt> validated by federation registry → user_federated
//
// On any path the middleware injects ctxkey.PrincipalRefKey and
// ctxkey.ProjectIDsKey before calling next.
func AuthDispatch(
	keyValidator *apikey.Validator,
	jwtSvc *auth.JWTService,
	fedRegistry *federation.Registry,
	resolver ProjectResolver,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractBearer(r)
			if rawToken == "" {
				writeMCPChallenge(w, "")
				return
			}

			ref, err := dispatch(r.Context(), rawToken, keyValidator, jwtSvc, fedRegistry)
			if err != nil {
				slog.DebugContext(r.Context(), "mcp auth dispatch failed", "err", err)
				if errors.Is(err, apikey.ErrRateLimited) {
					w.Header().Set("Retry-After", "60")
					http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
					return
				}
				writeMCPChallenge(w, "")
				return
			}

			// Resolve accessible project IDs if a resolver is provided.
			if resolver != nil {
				ids, resolveErr := resolver.AccessibleProjectIDs(r.Context(), ref.ID, ref.Kind)
				if resolveErr != nil {
					slog.WarnContext(r.Context(), "failed to resolve project IDs", "err", resolveErr)
				} else {
					ref.AccessibleProjectIDs = ids
				}
			}

			ctx := ctxkey.WithPrincipalRef(r.Context(), ref)
			ctx = ctxkey.WithProjectIDs(ctx, ref.AccessibleProjectIDs)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func dispatch(
	ctx context.Context,
	rawToken string,
	kv *apikey.Validator,
	jwtSvc *auth.JWTService,
	fedReg *federation.Registry,
) (*model.SessionPrincipalRef, error) {
	// Path 1: service-account API key.
	if strings.HasPrefix(rawToken, "agentlens_sk_") {
		return kv.Validate(ctx, rawToken)
	}

	// Path 2: local JWT.
	if jwtSvc != nil {
		if claims, err := jwtSvc.ValidateToken(rawToken); err == nil {
			return &model.SessionPrincipalRef{
				ID:          claims.UserID,
				Kind:        model.PrincipalTypeUserLocal,
				PartyID:     claims.UserID,
				Permissions: claims.Permissions,
				AuthMethod:  "jwt_local",
			}, nil
		}
	}

	// Path 3: federation JWT.
	if fedReg != nil {
		if prov, err := fedReg.Default(); err == nil {
			if fedClaims, err := prov.VerifyIDToken(ctx, rawToken); err == nil {
				return &model.SessionPrincipalRef{
					ID:         fedClaims.Sub,
					Kind:       model.PrincipalTypeUserFederated,
					PartyID:    fedClaims.Sub,
					AuthMethod: "jwt_federated:dex",
				}, nil
			}
		}
	}

	return nil, apikey.ErrInvalidCredential
}

// extractBearer extracts a Bearer token from the Authorization header.
func extractBearer(r *http.Request) string {
	hdr := r.Header.Get("Authorization")
	if strings.HasPrefix(hdr, "Bearer ") {
		return strings.TrimPrefix(hdr, "Bearer ")
	}
	return ""
}

// writeMCPChallenge writes a spec-compliant 401 with WWW-Authenticate header.
func writeMCPChallenge(w http.ResponseWriter, resourceMetadataURL string) {
	if resourceMetadataURL != "" {
		w.Header().Set("WWW-Authenticate",
			`Bearer resource_metadata="`+resourceMetadataURL+`"`)
	} else {
		w.Header().Set("WWW-Authenticate", `Bearer`)
	}
	http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
}

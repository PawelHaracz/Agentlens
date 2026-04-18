// Package ctxkey defines request-context keys for MCP authentication.
// Lives in internal/model (foundation layer) so plugins/mcpserver/ can read
// authenticated principal data from context without importing internal/api
// or internal/auth (both forbidden by arch-go for plugin packages).
package ctxkey

import (
	"context"

	"github.com/PawelHaracz/agentlens/internal/model"
)

type key string

const (
	principalRefKey key = "mcp.principal_ref"
	projectIDsKey   key = "mcp.accessible_project_ids"
)

// WithPrincipalRef returns a new context carrying ref.
func WithPrincipalRef(ctx context.Context, ref *model.SessionPrincipalRef) context.Context {
	return context.WithValue(ctx, principalRefKey, ref)
}

// PrincipalRef extracts the SessionPrincipalRef from ctx.
// Returns nil if not set.
func PrincipalRef(ctx context.Context) *model.SessionPrincipalRef {
	v, _ := ctx.Value(principalRefKey).(*model.SessionPrincipalRef)
	return v
}

// WithProjectIDs returns a new context carrying the accessible project IDs.
func WithProjectIDs(ctx context.Context, ids []string) context.Context {
	return context.WithValue(ctx, projectIDsKey, ids)
}

// ProjectIDs extracts the accessible project ID slice from ctx.
// Returns nil if not set.
func ProjectIDs(ctx context.Context) []string {
	v, _ := ctx.Value(projectIDsKey).([]string)
	return v
}

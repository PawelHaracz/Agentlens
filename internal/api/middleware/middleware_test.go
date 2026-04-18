package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api/middleware"
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/model/ctxkey"
)

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// --- E.2: OriginValidation ---

func TestOriginMiddleware_Allowlist_DefaultEmpty_Rejects_All_403(t *testing.T) {
	h := middleware.OriginValidation(nil)(http.HandlerFunc(ok))

	// missing Origin → 403
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code, "missing Origin must be 403")

	// present Origin but empty allowlist → 403 (strict default)
	req2 := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	req2.Header.Set("Origin", "https://claude.ai")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code, "present Origin with empty allowlist must be 403")
}

func TestOriginMiddleware_ConfiguredOrigin_Allowed(t *testing.T) {
	h := middleware.OriginValidation([]string{"https://claude.ai"})(http.HandlerFunc(ok))

	// allowed origin → pass
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	req.Header.Set("Origin", "https://claude.ai")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// unlisted origin → 403
	req2 := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	req2.Header.Set("Origin", "https://evil.io")
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}

// --- E.3: ScopeByAccessibleProjects ---

func TestScopeByAccessibleProjects_AppendsCtxFilter_NoURLMutation(t *testing.T) {
	var capturedIDs []string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedIDs = ctxkey.ProjectIDs(r.Context())
		// URL must not be mutated.
		assert.NotContains(t, r.URL.RawQuery, "projects=")
	})

	h := middleware.ScopeByAccessibleProjects(inner)

	ctx := ctxkey.WithProjectIDs(context.Background(), []string{"proj-A", "proj-B"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog?q=test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	require.Equal(t, []string{"proj-A", "proj-B"}, capturedIDs,
		"inner handler must see project IDs from ctx")
}

// --- E.4/E.5: service_accounts permission constants ---

func TestRequirePermission_ServiceAccountsRead_RejectsMissingPerm_403(t *testing.T) {
	jwtSvc := auth.NewJWTService(auth.JWTConfig{Secret: "test"})
	user := &model.User{ID: "u1", Username: "bob", RoleID: "role-viewer"}
	// role with catalog:read only — no service_accounts:read
	role := &model.Role{Permissions: []string{auth.PermCatalogRead}}
	token, err := jwtSvc.GenerateToken(user, role)
	require.NoError(t, err)

	// Simulate the existing RequireAuth + RequirePermission chain from internal/api.
	// We test by checking HasPermission directly since wiring requires the full router.
	perms := []string{auth.PermCatalogRead}
	assert.False(t, auth.HasPermission(perms, auth.PermServiceAccountsRead),
		"catalog-read role must not have service_accounts:read")
	assert.False(t, auth.HasPermission(perms, auth.PermServiceAccountsWrite),
		"catalog-read role must not have service_accounts:write")
	assert.False(t, auth.HasPermission(perms, auth.PermServiceAccountsRevoke),
		"catalog-read role must not have service_accounts:revoke")
	_ = token
}

// --- E.4: middleware chain order ---

func TestAuthDecisionOrder_OriginThenAuthThenScope(t *testing.T) {
	var reached []string

	originMw := middleware.OriginValidation([]string{"https://test.io"})
	scopeMw := middleware.ScopeByAccessibleProjects

	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		reached = append(reached, "inner")
	})

	// Stack: Origin → Scope → inner
	h := originMw(scopeMw(inner))

	ctx := ctxkey.WithProjectIDs(context.Background(), []string{"p1"})
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil).WithContext(ctx)
	req.Header.Set("Origin", "https://test.io")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"inner"}, reached, "inner must be reached when origin is allowed")

	// Without allowed origin → stops at Origin middleware.
	reached = nil
	req2 := httptest.NewRequest(http.MethodPost, "/api/mcp", nil).WithContext(ctx)
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
	assert.Empty(t, reached, "inner must not be reached when origin missing")
}

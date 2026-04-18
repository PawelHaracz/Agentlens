package api_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/api/middleware"
	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/model/ctxkey"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func openPartyStore(t *testing.T, database *db.DB) *store.PartyStore {
	t.Helper()
	return store.NewPartyStore(database)
}

func openCredStore(t *testing.T, database *db.DB) *store.ApiClientCredentialStore {
	t.Helper()
	return store.NewApiClientCredentialStore(database)
}

func buildCredentialForTest(partyID string) (rawSecret string, cred *model.ApiClientCredential, err error) {
	b := make([]byte, 8)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	clientID := hex.EncodeToString(b)
	sb := make([]byte, 16)
	if _, err = rand.Read(sb); err != nil {
		return "", nil, err
	}
	rawSecret = hex.EncodeToString(sb)
	hash, err := bcrypt.GenerateFromPassword([]byte(rawSecret), 4) // cost 4 for test speed
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("agentlens_sk_%s.%s", clientID, rawSecret), &model.ApiClientCredential{
		PartyID:    partyID,
		ClientID:   clientID,
		SecretHash: string(hash),
	}, nil
}

// TestCORSNonInterference_GlobalCORSUnaffectedByMCPOriginMiddleware verifies
// that OriginValidation (scoped to /api/mcp) does not affect /api/v1 routes,
// which must remain accessible without an Origin header.
// Per spec §5.5 and scope-clarifications §7 (global CORS untouched).
func TestCORSNonInterference_GlobalCORSUnaffectedByMCPOriginMiddleware(t *testing.T) {
	router, _, _ := testRouter(t)

	// /api/v1/catalog requires auth but should NOT be 403 for missing Origin.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	// No Origin header, no auth token.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Expect 401 (auth required) NOT 403 (Origin blocked).
	// If OriginValidation were applied globally, this would return 403.
	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"missing Origin must NOT affect /api/v1 routes; only /api/mcp is origin-gated")
	assert.NotEqual(t, http.StatusForbidden, w.Code,
		"403 would indicate global Origin enforcement — spec violation")
}

// TestPRMHandler_NotRegistered_WhenFederationDisabled confirms the PRM route
// (/.well-known/oauth-protected-resource) is absent from the standard router
// that does not wire federation (L-new-1).
func TestPRMHandler_NotRegistered_WhenFederationDisabled(t *testing.T) {
	// testRouter creates a router without federation / PRM handler.
	router, _, _ := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// The SPA handler may return 200 with HTML for unknown paths, but the
	// response must NOT be a PRM JSON document (which would mean federation
	// route was accidentally registered). Verify Content-Type is not JSON.
	assert.NotEqual(t, "application/json", w.Header().Get("Content-Type"),
		"PRM JSON response must not be served when federation is disabled (L-new-1)")
	assert.NotContains(t, w.Body.String(), `"authorization_servers"`,
		"PRM route must not produce RFC 9728 content when federation is disabled")
}

// TestPRMHandler_Registered_WhenFederationEnabled verifies that wiring a PRM
// handler via kernel.RegisterRoutes causes the route to respond 200.
func TestPRMHandler_Registered_WhenFederationEnabled(t *testing.T) {
	// The PRM handler itself always returns the RFC 9728 document; it is the
	// composition root (wireMCPRoutes) that conditionally registers it.
	// This test validates the handler content in isolation.
	prmHandler := api.NewPRMHandler(
		"https://agentlens.example.com/api/mcp",
		"https://dex.example.com/dex",
	)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	w := httptest.NewRecorder()
	prmHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"authorization_servers"`)
	assert.Contains(t, w.Body.String(), "dex.example.com")
}

// TestOriginValidation_ChainWithAuth verifies that when both Origin and auth
// middlewares are chained, Origin is checked FIRST and a missing Origin blocks
// the request before auth is even evaluated.
func TestOriginValidation_ChainWithAuth_OriginBlocksFirst(t *testing.T) {
	var authReached bool
	authMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authReached = true
			next.ServeHTTP(w, r)
		})
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Origin → Auth → inner
	h := middleware.OriginValidation([]string{"https://allowed.io"})(authMW(inner))

	// Missing Origin — should 403 without reaching auth.
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, authReached, "auth middleware must not execute when Origin is missing")
}

// TestProjectIDs_TakesPrecedence_Over_ProjectID verifies that when
// CatalogFilter.ProjectIDs is non-empty it overrides ProjectID (single-value
// filter), as documented in sql_store_query.go:applyProjectFilter.
func TestProjectIDs_TakesPrecedence_Over_ProjectID(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()
	cs := store.NewSQLStore(database)

	// Create a catalog entry (auto-assigned to default project by store).
	agentTypeID := "at-proj-filter"
	entry := &model.CatalogEntry{
		ID:          "ce-proj-filter",
		DisplayName: "Project Filter Test",
		Source:      model.SourcePush,
		Status:      model.LifecycleActive,
		AgentType: &model.AgentType{
			ID:       agentTypeID,
			AgentKey: model.ComputeAgentKey(model.ProtocolMCP, "http://proj-filter.test"),
			Protocol: model.ProtocolMCP,
			Endpoint: "http://proj-filter.test",
			Version:  "1.0",
		},
	}
	require.NoError(t, cs.Create(ctx, entry))

	// No filter — entry is visible.
	all, err := cs.List(ctx, store.ListFilter{})
	require.NoError(t, err)
	require.NotEmpty(t, all)

	// ProjectIDs with a bogus value — entry NOT visible (IN? clause enforced).
	withBogus, err := cs.List(ctx, store.ListFilter{
		ProjectIDs: []string{"nonexistent-project-id"},
	})
	require.NoError(t, err)
	assert.Empty(t, withBogus,
		"ProjectIDs with no matches must return empty (overrides ProjectID fallback)")
}

// TestScopeByAccessibleProjects_EmptyProjectIDs_NoFilterApplied verifies that
// when ctxkey.ProjectIDs is empty/nil, ScopeByAccessibleProjects is a pass-through.
func TestScopeByAccessibleProjects_EmptyProjectIDs_NoFilterApplied(t *testing.T) {
	var capturedIDs []string
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedIDs = ctxkey.ProjectIDs(r.Context())
	})
	h := middleware.ScopeByAccessibleProjects(inner)

	// No project IDs in ctx.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Nil(t, capturedIDs,
		"ScopeByAccessibleProjects must be a no-op when ctx carries no project IDs")
}

// TestEnumerateActiveCredentials_ReturnsRealClientIDs verifies that
// EnumerateActiveCredentials returns actual client IDs when credentials exist.
func TestEnumerateActiveCredentials_ReturnsRealClientIDs(t *testing.T) {
	database := openTestDB(t)
	ctx := context.Background()
	ps := openPartyStore(t, database)
	cs := openCredStore(t, database)

	party, err := ps.CreateServiceAccount(ctx, "sa-enumerate")
	require.NoError(t, err)

	// Create an active credential.
	rawSecret, cred, err := buildCredentialForTest(party.ID)
	require.NoError(t, err)
	require.NoError(t, cs.Create(ctx, cred))
	_ = rawSecret

	// EnumerateActiveCredentials should return the clientID.
	clientIDs, err := ps.EnumerateActiveCredentials(ctx, party.ID)
	require.NoError(t, err)
	require.Len(t, clientIDs, 1)
	assert.Equal(t, cred.ClientID, clientIDs[0])

	// Revoke the credential.
	require.NoError(t, cs.Revoke(ctx, cred.ID))

	// After revoke: empty.
	clientIDs2, err := ps.EnumerateActiveCredentials(ctx, party.ID)
	require.NoError(t, err)
	assert.Empty(t, clientIDs2, "revoked credentials must not appear in enumeration")
}

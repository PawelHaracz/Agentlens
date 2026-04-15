package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPartyTestRouter(t *testing.T) (http.Handler, *store.PartyStore) {
	t.Helper()
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)
	require.NoError(t, db.NewMigrator(database, db.AllMigrations()).Migrate(context.Background()))

	catalogStore := store.NewSQLStore(database)
	ps := store.NewPartyStore(database)
	core := kernel.NewCore(catalogStore, nil, slog.Default(), kernel.LicenseInfo{})

	jwtSvc := auth.NewJWTService(auth.JWTConfig{Secret: "test-secret", Expiration: 24 * 3600 * 1e9})
	deps := api.RouterDeps{
		Kernel:        core,
		JWTService:    jwtSvc,
		UserStore:     store.NewUserStore(database),
		RoleStore:     store.NewRoleStore(database),
		SettingsStore: store.NewSettingsStore(database),
		PartyStore:    ps,
	}
	return api.NewRouter(deps), ps
}

func partyAuthHeader(t *testing.T, permissions []string) string {
	t.Helper()
	jwtSvc := auth.NewJWTService(auth.JWTConfig{Secret: "test-secret", Expiration: 24 * 3600 * 1e9})
	user := &model.User{ID: "user1", Username: "alice", RoleID: "r1"}
	role := &model.Role{ID: "r1", Permissions: model.JSONSlice(permissions)}
	token, err := jwtSvc.GenerateToken(user, role)
	require.NoError(t, err)
	return "Bearer " + token
}

func TestCreateGroup_ReturnsCreated(t *testing.T) {
	router, _ := newPartyTestRouter(t)

	body, _ := json.Marshal(map[string]string{"name": "eng-team"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", bytes.NewReader(body))
	req.Header.Set("Authorization", partyAuthHeader(t, []string{"users:write"}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp model.Party
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "eng-team", resp.Name)
	assert.Equal(t, model.PartyKindGroup, resp.Kind)
}

func TestCreateGroup_Forbidden_WithoutPermission(t *testing.T) {
	router, _ := newPartyTestRouter(t)

	body, _ := json.Marshal(map[string]string{"name": "eng-team"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", bytes.NewReader(body))
	req.Header.Set("Authorization", partyAuthHeader(t, []string{"catalog:read"}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestListGroups_ReturnsOK(t *testing.T) {
	router, ps := newPartyTestRouter(t)
	require.NoError(t, ps.CreateParty(context.Background(), &model.Party{Kind: model.PartyKindGroup, Name: "g1"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/groups", nil)
	req.Header.Set("Authorization", partyAuthHeader(t, []string{"users:read"}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var groups []model.Party
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &groups))
	assert.GreaterOrEqual(t, len(groups), 1)
}

func TestRequireProjectPermission_GlobalAdminBypasses(t *testing.T) {
	router, ps := newPartyTestRouter(t)

	// Create a project
	proj := &model.Party{Kind: model.PartyKindProject, Name: "myproject"}
	require.NoError(t, ps.CreateParty(context.Background(), proj))

	// global admin permissions bypass project check
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+proj.ID, nil)
	req.Header.Set("Authorization", partyAuthHeader(t, []string{
		"catalog:read", "catalog:write", "catalog:delete",
		"users:read", "users:write", "users:delete",
		"roles:read", "roles:write",
		"settings:read", "settings:write",
	}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 200 because kind matches and global admin bypasses project permission check
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCatalogAssignToProject_ReturnsUnauthorized(t *testing.T) {
	router, _ := newPartyTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/nonexistent/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListCatalog_FilteredByProject_ReturnsOK(t *testing.T) {
	router, _ := newPartyTestRouter(t)

	// Without project filter — existing behavior, no change expected
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", partyAuthHeader(t, []string{"catalog:read"}))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// With project filter — must also return 200 (empty list is fine)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/catalog?project=nonexistent-id", nil)
	req2.Header.Set("Authorization", partyAuthHeader(t, []string{"catalog:read"}))
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

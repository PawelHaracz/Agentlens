package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func TestRoleList(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodGet, "/api/v1/roles", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var roles []map[string]interface{}
	require.NoError(t, decodeJSON(w, &roles))
	assert.GreaterOrEqual(t, len(roles), 1)
}

func TestRoleCreate(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"name":"custom-role","description":"Can edit things","permissions":["catalog:read","catalog:write"]}`
	req := authRequest(http.MethodPost, "/api/v1/roles", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var role map[string]interface{}
	require.NoError(t, decodeJSON(w, &role))
	assert.Equal(t, "custom-role", role["name"])
}

func TestRoleCreate_DuplicateName(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"name":"admin","description":"Duplicate","permissions":["catalog:read"]}`
	req := authRequest(http.MethodPost, "/api/v1/roles", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestRoleUpdate(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createViewerRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"description":"Updated viewer description"}`
	req := authRequest(http.MethodPut, "/api/v1/roles/role-viewer", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var role map[string]interface{}
	require.NoError(t, decodeJSON(w, &role))
	assert.Equal(t, "Updated viewer description", role["description"])
}

func TestRoleDelete_SystemRoleBlocked(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodDelete, "/api/v1/roles/role-admin", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, decodeJSON(w, &resp))
	assert.Contains(t, resp["error"], "cannot delete system role")
}

func TestRoleDelete_NonSystemRole(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	// Create a non-system role for deletion.
	roleStore := store.NewRoleStore(database)
	require.NoError(t, roleStore.Create(context.Background(), &model.Role{
		ID:          "role-custom",
		Name:        "custom",
		Permissions: model.JSONSlice{"catalog:read"},
		IsSystem:    false,
	}))

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodDelete, "/api/v1/roles/role-custom", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestRoleDelete_NotFound(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodDelete, "/api/v1/roles/nonexistent", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

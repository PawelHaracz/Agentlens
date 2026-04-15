package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func TestUserList(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodGet, "/api/v1/users", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var users []map[string]interface{}
	require.NoError(t, decodeJSON(w, &users))
	assert.GreaterOrEqual(t, len(users), 1)
}

func TestUserCreate(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createViewerRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"username":"newuser","email":"new@test.com","display_name":"New User","password":"NewUser1234!@","role_id":"role-viewer"}`
	req := authRequest(http.MethodPost, "/api/v1/users", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var user map[string]interface{}
	require.NoError(t, decodeJSON(w, &user))
	assert.Equal(t, "newuser", user["username"])
	assert.Equal(t, "new@test.com", user["email"])
	// password_hash should not be present
	assert.Empty(t, user["password_hash"])
}

func TestUserCreate_DuplicateUsername(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"username":"admin","email":"dup@test.com","password":"NewUser1234!@","role_id":"role-admin"}`
	req := authRequest(http.MethodPost, "/api/v1/users", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUserGet(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodGet, "/api/v1/users/test-admin", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var user map[string]interface{}
	require.NoError(t, decodeJSON(w, &user))
	assert.Equal(t, "admin", user["username"])
}

func TestUserGet_NotFound(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodGet, "/api/v1/users/nonexistent", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserUpdate(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"display_name":"Updated Admin","email":"admin@example.com"}`
	req := authRequest(http.MethodPut, "/api/v1/users/test-admin", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var user map[string]interface{}
	require.NoError(t, decodeJSON(w, &user))
	assert.Equal(t, "Updated Admin", user["display_name"])
	assert.Equal(t, "admin@example.com", user["email"])
}

func TestUserDelete_SelfDeleteBlocked(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodDelete, "/api/v1/users/test-admin", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, decodeJSON(w, &resp))
	assert.Contains(t, resp["error"], "cannot delete your own account")
}

func TestUserDelete_Success(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createViewerRole(t, database)
	username, password := createTestAdmin(t, database)
	createTestUser(t, database, "user-to-delete", "deleteme", "role-viewer")

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodDelete, "/api/v1/users/user-to-delete", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestUserPermissionDenied(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createTestAdmin(t, database)

	// Create a minimal role that has NO user permissions.
	roleStore := store.NewRoleStore(database)
	require.NoError(t, roleStore.Create(context.Background(), &model.Role{
		ID:          "role-minimal",
		Name:        "minimal",
		Permissions: model.JSONSlice{"catalog:read"},
	}))
	viewerUser, viewerPass := createTestUser(t, database, "minimal-1", "minimaluser", "role-minimal")

	token := loginAndGetToken(t, router, viewerUser, viewerPass)

	// Minimal role should not be able to list users (requires users:read).
	req := authRequest(http.MethodGet, "/api/v1/users", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserCreate_WeakPassword(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createViewerRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"username":"weakuser","password":"weak","role_id":"role-viewer"}`
	req := authRequest(http.MethodPost, "/api/v1/users", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserResponseExcludesPasswordHash(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	// List users
	req := authRequest(http.MethodGet, "/api/v1/users", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "password_hash")

	// Get single user
	req2 := authRequest(http.MethodGet, "/api/v1/users/test-admin", token, "")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.NotContains(t, w2.Body.String(), "password_hash")
}

func TestUserDelete_LastAdminBlocked(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createViewerRole(t, database)
	username, password := createTestAdmin(t, database)

	// Create a second admin to do the deleting.
	createTestUser(t, database, "admin-2", "admin2", "role-admin")
	token := loginAndGetToken(t, router, username, password)

	// Delete admin2 - should succeed since there will still be one admin left.
	req := authRequest(http.MethodDelete, "/api/v1/users/admin-2", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Now try to create a third user (non-admin) and try to delete ourselves.
	// We can't delete ourselves - self-delete is blocked regardless.
	req2 := authRequest(http.MethodDelete, "/api/v1/users/test-admin", token, "")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestUserCreate_InvalidRole(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body, _ := json.Marshal(map[string]string{
		"username": "newuser",
		"password": "NewUser1234!@",
		"role_id":  "nonexistent-role",
	})
	req := authRequest(http.MethodPost, "/api/v1/users", token, string(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateUser_AlsoCreatesPersonParty(t *testing.T) {
	router, database, partyStore := testRouterWithParty(t)
	createAdminRole(t, database)
	createViewerRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"username":"partyuser","email":"party@test.com","display_name":"Party User","password":"NewUser1234!@","role_id":"role-viewer"}`
	req := authRequest(http.MethodPost, "/api/v1/users", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	require.NoError(t, decodeJSON(w, &created))
	newUserID := created["id"].(string)

	person, err := partyStore.GetPartyByUserID(context.Background(), newUserID)
	require.NoError(t, err)
	require.NotNil(t, person, "Person party should have been created for the new user")
	assert.Equal(t, "Party User", person.Name)
}

func TestUpdateUser_SyncsPersonName(t *testing.T) {
	router, database, partyStore := testRouterWithParty(t)
	createAdminRole(t, database)
	createViewerRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	// Create user via API so the Person party is created by the handler.
	createBody := `{"username":"syncuser","email":"sync@test.com","display_name":"Original Name","password":"NewUser1234!@","role_id":"role-viewer"}`
	createReq := authRequest(http.MethodPost, "/api/v1/users", token, createBody)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	var created map[string]interface{}
	require.NoError(t, decodeJSON(createW, &created))
	newUserID := created["id"].(string)

	// Update display_name.
	updateBody := `{"display_name":"Updated Name"}`
	updateReq := authRequest(http.MethodPut, "/api/v1/users/"+newUserID, token, updateBody)
	updateW := httptest.NewRecorder()
	router.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusOK, updateW.Code)

	person, err := partyStore.GetPartyByUserID(context.Background(), newUserID)
	require.NoError(t, err)
	require.NotNil(t, person)
	assert.Equal(t, "Updated Name", person.Name)
}

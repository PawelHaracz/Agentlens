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

func TestSettingsGetAll(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	// Seed some settings.
	settingsStore := store.NewSettingsStore(database)
	require.NoError(t, settingsStore.Set(context.Background(), "site_name", "TestSite"))
	require.NoError(t, settingsStore.Set(context.Background(), "theme", "dark"))

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodGet, "/api/v1/settings", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var settings []model.Setting
	require.NoError(t, decodeJSON(w, &settings))
	assert.GreaterOrEqual(t, len(settings), 2)
}

func TestSettingsGetByCategory(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	// Seed settings with categories.
	settingsStore := store.NewSettingsStore(database)
	require.NoError(t, settingsStore.Set(context.Background(), "ui_theme", "dark"))

	// Manually set the category for the setting.
	database.DB.Model(&model.Setting{}).Where("key = ?", "ui_theme").Update("category", "ui")

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodGet, "/api/v1/settings/ui", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var settings []model.Setting
	require.NoError(t, decodeJSON(w, &settings))
	assert.GreaterOrEqual(t, len(settings), 1)
	for _, s := range settings {
		assert.Equal(t, "ui", s.Category)
	}
}

func TestSettingsUpdate(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"app_name":"AgentLens Test","max_results":"100"}`
	req := authRequest(http.MethodPut, "/api/v1/settings", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the settings were saved.
	settingsStore := store.NewSettingsStore(database)
	setting, err := settingsStore.Get(context.Background(), "app_name")
	require.NoError(t, err)
	require.NotNil(t, setting)
	assert.Equal(t, "AgentLens Test", setting.Value)

	setting2, err := settingsStore.Get(context.Background(), "max_results")
	require.NoError(t, err)
	require.NotNil(t, setting2)
	assert.Equal(t, "100", setting2.Value)
}

func TestSettingsPermissionDenied(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createTestAdmin(t, database)

	// Create a minimal role that has NO settings permissions.
	roleStore := store.NewRoleStore(database)
	require.NoError(t, roleStore.Create(context.Background(), &model.Role{
		ID:          "role-no-settings",
		Name:        "no-settings",
		Permissions: model.JSONSlice{"catalog:read"},
	}))
	viewerUser, viewerPass := createTestUser(t, database, "viewer-settings", "settingsviewer", "role-no-settings")

	token := loginAndGetToken(t, router, viewerUser, viewerPass)

	// Should not be able to read settings.
	req := authRequest(http.MethodGet, "/api/v1/settings", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSettingsUpdate_PermissionDenied(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createTestAdmin(t, database)

	// Create a minimal role that has NO settings:write permissions.
	roleStore := store.NewRoleStore(database)
	require.NoError(t, roleStore.Create(context.Background(), &model.Role{
		ID:          "role-no-settings-w",
		Name:        "no-settings-w",
		Permissions: model.JSONSlice{"catalog:read"},
	}))
	viewerUser, viewerPass := createTestUser(t, database, "viewer-settings-w", "settingsviewerw", "role-no-settings-w")

	token := loginAndGetToken(t, router, viewerUser, viewerPass)

	body := `{"key":"value"}`
	req := authRequest(http.MethodPut, "/api/v1/settings", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

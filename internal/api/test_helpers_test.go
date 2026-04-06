package api_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	a2aplugin "github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
	mcpplugin "github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
)

// testRouter creates a chi router for testing with a full in-memory DB.
func testRouter(t *testing.T) (http.Handler, *db.DB, *auth.JWTService) {
	t.Helper()

	database, err := db.OpenMemory()
	require.NoError(t, err)

	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(context.Background()))

	catalogStore := store.NewSQLStore(database)
	userStore := store.NewUserStore(database)
	roleStore := store.NewRoleStore(database)
	settingsStore := store.NewSettingsStore(database)

	core := kernel.NewCore(catalogStore, nil, slog.Default(), kernel.LicenseInfo{})
	a2aParser := a2aplugin.New()
	_ = a2aParser.Init(core)
	core.RegisterParser(a2aParser)
	mcpParser := mcpplugin.New()
	_ = mcpParser.Init(core)
	core.RegisterParser(mcpParser)

	jwtService := auth.NewJWTService(auth.JWTConfig{
		Secret:        "test-secret",
		Expiration:    time.Hour,
		RefreshWindow: 10 * time.Minute,
	})

	router := api.NewRouter(api.RouterDeps{
		Kernel:        core,
		UserStore:     userStore,
		RoleStore:     roleStore,
		SettingsStore: settingsStore,
		JWTService:    jwtService,
	})

	t.Cleanup(func() {
		sqlDB, _ := database.DB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return router, database, jwtService
}

// createAdminRole returns the admin role — already seeded by migration003.
func createAdminRole(t *testing.T, database *db.DB) *model.Role {
	t.Helper()
	roleStore := store.NewRoleStore(database)
	role, err := roleStore.GetByID(context.Background(), "role-admin")
	require.NoError(t, err)
	require.NotNil(t, role, "admin role should exist from migrations")
	return role
}

// createViewerRole returns the viewer role — already seeded by migration003.
func createViewerRole(t *testing.T, database *db.DB) *model.Role {
	t.Helper()
	roleStore := store.NewRoleStore(database)
	role, err := roleStore.GetByID(context.Background(), "role-viewer")
	require.NoError(t, err)
	require.NotNil(t, role, "viewer role should exist from migrations")
	return role
}

// createTestAdmin creates an admin user and returns the username and password.
func createTestAdmin(t *testing.T, database *db.DB) (string, string) {
	t.Helper()
	userStore := store.NewUserStore(database)
	hash, err := auth.HashPassword("Test1234!@ab")
	require.NoError(t, err)
	user := &model.User{
		ID:           "test-admin",
		Username:     "admin",
		PasswordHash: hash,
		RoleID:       "role-admin",
		IsActive:     true,
	}
	require.NoError(t, userStore.Create(context.Background(), user))
	return "admin", "Test1234!@ab"
}

// createTestUser creates a user with the given role and returns user details.
func createTestUser(t *testing.T, database *db.DB, id, username, roleID string) (string, string) {
	t.Helper()
	userStore := store.NewUserStore(database)
	password := "Test1234!@ab"
	hash, err := auth.HashPassword(password)
	require.NoError(t, err)
	user := &model.User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		RoleID:       roleID,
		IsActive:     true,
	}
	require.NoError(t, userStore.Create(context.Background(), user))
	return username, password
}

// loginAndGetToken performs a login and returns the JWT token.
func loginAndGetToken(t *testing.T, router http.Handler, username, password string) string {
	t.Helper()
	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "login failed: %s", w.Body.String())

	var resp map[string]interface{}
	require.NoError(t, decodeJSON(w, &resp))
	token, ok := resp["token"].(string)
	require.True(t, ok, "token not found in response")
	return token
}

// authRequest creates an authenticated HTTP request.
func authRequest(method, path, token string, body string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, stringReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestLogin_Success(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, decodeJSON(w, &resp))
	assert.NotEmpty(t, resp["token"])
	assert.NotNil(t, resp["user"])

	// Check that httpOnly cookie is set.
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "agentlens_token" {
			found = true
			assert.True(t, c.HttpOnly)
		}
	}
	assert.True(t, found, "agentlens_token cookie should be set")
}

func TestLogin_InvalidCredentials(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createTestAdmin(t, database)

	body := `{"username":"admin","password":"WrongPassword123!@"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_UserNotFound(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)

	body := `{"username":"nonexistent","password":"Test1234!@ab"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_LockedAccount(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	createTestAdmin(t, database)

	// Lock the account.
	userStore := store.NewUserStore(database)
	lockUntil := time.Now().Add(time.Hour)
	require.NoError(t, userStore.LockUser(context.Background(), "test-admin", lockUntil))

	body := `{"username":"admin","password":"Test1234!@ab"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusLocked, w.Code)
}

func TestLogin_MissingFields(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)

	body := `{"username":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMe_Success(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodGet, "/api/v1/auth/me", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, decodeJSON(w, &resp))
	assert.Equal(t, "admin", resp["username"])
	// password_hash should NOT be in the response.
	assert.Empty(t, resp["password_hash"])
}

func TestMe_Unauthenticated(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefresh_Success(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	req := authRequest(http.MethodPost, "/api/v1/auth/refresh", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, decodeJSON(w, &resp))
	assert.NotEmpty(t, resp["token"])
}

func TestChangePassword_Success(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"current_password":"Test1234!@ab","new_password":"NewPass5678!@cd"}`
	req := authRequest(http.MethodPut, "/api/v1/auth/password", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify we can login with the new password.
	loginBody := `{"username":"admin","password":"NewPass5678!@cd"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(loginBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"current_password":"WrongPassword!@12","new_password":"NewPass5678!@cd"}`
	req := authRequest(http.MethodPut, "/api/v1/auth/password", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestChangePassword_WeakNewPassword(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	body := `{"current_password":"Test1234!@ab","new_password":"weak"}`
	req := authRequest(http.MethodPut, "/api/v1/auth/password", token, body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogout(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check that the cookie is cleared.
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "agentlens_token" {
			assert.Equal(t, -1, c.MaxAge)
		}
	}
}

func TestCookieAuth(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	// Use cookie-based auth instead of header.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "agentlens_token", Value: token})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogin_ResponseExcludesPasswordHash(t *testing.T) {
	router, database, _ := testRouter(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	body := `{"username":"` + username + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Ensure password_hash is not in the JSON response body.
	assert.NotContains(t, w.Body.String(), "password_hash")
}

// testRouterWithParty creates a test router that also wires in a PartyStore.
func testRouterWithParty(t *testing.T) (http.Handler, *db.DB, *store.PartyStore) {
	t.Helper()

	database, err := db.OpenMemory()
	require.NoError(t, err)

	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(t.Context()))

	catalogStore := store.NewSQLStore(database)
	userStore := store.NewUserStore(database)
	roleStore := store.NewRoleStore(database)
	settingsStore := store.NewSettingsStore(database)
	partyStore := store.NewPartyStore(database)

	core := kernel.NewCore(catalogStore, nil, nil, kernel.LicenseInfo{})
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
		PartyStore:    partyStore,
	})

	t.Cleanup(func() {
		sqlDB, _ := database.DB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return router, database, partyStore
}

func TestMyProjectsHandler_ReturnsMemberships(t *testing.T) {
	router, database, partyStore := testRouterWithParty(t)
	createAdminRole(t, database)
	username, password := createTestAdmin(t, database)

	token := loginAndGetToken(t, router, username, password)

	// Get the admin user's person party (created by migration007 seed or find default project).
	// Seed a Person for the admin and add them to the default project.
	userStore := store.NewUserStore(database)
	user, err := userStore.GetByID(t.Context(), "test-admin")
	require.NoError(t, err)
	require.NotNil(t, user)

	require.NoError(t, partyStore.CreatePersonForUser(t.Context(), user))

	person, err := partyStore.GetPartyByUserID(t.Context(), "test-admin")
	require.NoError(t, err)
	require.NotNil(t, person)

	project, err := partyStore.GetDefaultProject(t.Context())
	require.NoError(t, err)
	require.NotNil(t, project)

	require.NoError(t, partyStore.AddMember(t.Context(), &model.PartyRelationship{
		FromPartyID:      person.ID,
		FromRole:         "project:owner",
		ToPartyID:        project.ID,
		ToRole:           "project",
		RelationshipName: "project_member",
	}))

	req := authRequest(http.MethodGet, "/api/v1/auth/me/projects", token, "")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var memberships []map[string]interface{}
	require.NoError(t, decodeJSON(w, &memberships))
	assert.Len(t, memberships, 1)
	assert.Equal(t, "project:owner", memberships[0]["role"])
}

func TestMyProjectsHandler_Unauthenticated401(t *testing.T) {
	router, database, _ := testRouterWithParty(t)
	createAdminRole(t, database)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api/middleware"
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/auth/apikey"
	"github.com/PawelHaracz/agentlens/internal/auth/credcache"
	"github.com/PawelHaracz/agentlens/internal/auth/ratelimit"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/model/ctxkey"
)

// stubCredStore for dispatch tests.
type stubCredStore struct {
	creds map[string]*model.ApiClientCredential
}

func (s *stubCredStore) GetByClientID(_ context.Context, id string) (*model.ApiClientCredential, error) {
	return s.creds[id], nil
}

func hashForTest(t *testing.T, s string) string {
	t.Helper()
	h, _ := bcrypt.GenerateFromPassword([]byte(s), 4)
	return string(h)
}

func newApiKeyValidator(t *testing.T, creds map[string]*model.ApiClientCredential) *apikey.Validator {
	t.Helper()
	return apikey.New(&stubCredStore{creds}, credcache.New(), ratelimit.New())
}

func captureRef(ref *model.SessionPrincipalRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := ctxkey.PrincipalRef(r.Context())
		if got == nil || ref == nil {
			return
		}
		*ref = *got
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthDispatch_Order_ApiKey_Then_LocalJWT_Then_Federation(t *testing.T) {
	// API key should be tried first.
	creds := map[string]*model.ApiClientCredential{
		"sk-id": {PartyID: "party-sa", ClientID: "sk-id", SecretHash: hashForTest(t, "secret")},
	}
	kv := newApiKeyValidator(t, creds)

	var captured model.SessionPrincipalRef
	mw := middleware.AuthDispatch(kv, nil, nil, nil)
	handler := mw(captureRef(&captured))

	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	req.Header.Set("Authorization", "Bearer agentlens_sk_sk-id.secret")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, model.PrincipalTypeServiceAccount, captured.Kind)
	assert.Equal(t, "party-sa", captured.PartyID)
}

func TestAuthDispatch_Normalizes_To_SessionPrincipalRef_In_Ctx(t *testing.T) {
	// Local JWT path.
	jwtSvc := auth.NewJWTService(auth.JWTConfig{Secret: "test-secret"})
	user := &model.User{ID: "user-1", Username: "alice", RoleID: "role-admin"}
	role := &model.Role{Permissions: []string{"catalog:read"}}
	token, err := jwtSvc.GenerateToken(user, role)
	require.NoError(t, err)

	var captured model.SessionPrincipalRef
	mw := middleware.AuthDispatch(newApiKeyValidator(t, nil), jwtSvc, nil, nil)
	handler := mw(captureRef(&captured))

	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, model.PrincipalTypeUserLocal, captured.Kind)
	assert.Equal(t, "user-1", captured.ID)
	assert.Contains(t, captured.Permissions, "catalog:read")
}

func TestAuthDispatch_MissingAuth_Returns401(t *testing.T) {
	mw := middleware.AuthDispatch(newApiKeyValidator(t, nil), nil, nil, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.NotEmpty(t, w.Header().Get("WWW-Authenticate"))
}

func TestRateLimit_429_After_Threshold(t *testing.T) {
	store := &stubCredStore{map[string]*model.ApiClientCredential{
		"rl-id": {PartyID: "p", ClientID: "rl-id", SecretHash: hashForTest(t, "right")},
	}}
	limiter := ratelimit.NewWithOptions(3, 60*time.Second)
	cache := credcache.New()
	kv := apikey.New(store, cache, limiter)

	mw := middleware.AuthDispatch(kv, nil, nil, nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var lastCode int
	for i := 0; i < 8; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
		req.Header.Set("Authorization", "Bearer agentlens_sk_rl-id.wrong")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		lastCode = w.Code
	}
	assert.Equal(t, http.StatusUnauthorized, lastCode, "rate-limited requests return 401 (challenge)")
}

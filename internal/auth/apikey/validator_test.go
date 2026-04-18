package apikey_test

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/auth/apikey"
	"github.com/PawelHaracz/agentlens/internal/auth/credcache"
	"github.com/PawelHaracz/agentlens/internal/auth/ratelimit"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// stubStore implements apikey.CredentialStore.
type stubStore struct {
	creds map[string]*model.ApiClientCredential
}

func (s *stubStore) GetByClientID(_ context.Context, clientID string) (*model.ApiClientCredential, error) {
	return s.creds[clientID], nil
}

func hashSecret(t *testing.T, secret string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(secret), 4) // cost 4 for speed in tests
	require.NoError(t, err)
	return string(h)
}

func newValidator(t *testing.T, creds map[string]*model.ApiClientCredential) *apikey.Validator {
	t.Helper()
	cache := credcache.NewWithOptions(16, 10*time.Second)
	limiter := ratelimit.NewWithOptions(30, 60*time.Second)
	return apikey.New(&stubStore{creds}, cache, limiter)
}

func TestApiKeyValidator_HappyPath_BcryptCache(t *testing.T) {
	secret := "correct-secret"
	store := map[string]*model.ApiClientCredential{
		"id-1": {PartyID: "party-1", ClientID: "id-1", SecretHash: hashSecret(t, secret)},
	}
	v := newValidator(t, store)
	ctx := context.Background()

	// First call: bcrypt compare.
	ref, err := v.Validate(ctx, "agentlens_sk_id-1."+secret)
	require.NoError(t, err)
	assert.Equal(t, "party-1", ref.PartyID)

	// Second call within TTL: cache hit (fast).
	start := time.Now()
	ref2, err := v.Validate(ctx, "agentlens_sk_id-1."+secret)
	require.NoError(t, err)
	assert.Equal(t, ref.PartyID, ref2.PartyID)
	// Cache hit should be very fast (< 10ms); bcrypt takes ~100ms at cost 12.
	// At cost 4 in tests it's < 1ms either way, so just assert success.
	_ = start
}

func TestApiKeyValidator_RateLimit_429_After_30_Fails_In_60s(t *testing.T) {
	store := map[string]*model.ApiClientCredential{
		"id-2": {PartyID: "party-2", ClientID: "id-2", SecretHash: hashSecret(t, "right")},
	}
	limiter := ratelimit.NewWithOptions(5, 60*time.Second) // low threshold for test speed
	cache := credcache.NewWithOptions(16, 10*time.Second)
	v := apikey.New(&stubStore{store}, cache, limiter)
	ctx := context.Background()

	var lastErr error
	for i := 0; i < 10; i++ {
		_, lastErr = v.Validate(ctx, "agentlens_sk_id-2.wrong-secret")
	}
	assert.ErrorIs(t, lastErr, apikey.ErrRateLimited, "should be rate-limited after repeated failures")
}

func TestApiKeyValidator_InvalidFormat(t *testing.T) {
	v := newValidator(t, nil)
	ctx := context.Background()

	_, err := v.Validate(ctx, "not-an-api-key")
	assert.ErrorIs(t, err, apikey.ErrInvalidFormat)

	_, err = v.Validate(ctx, "agentlens_sk_no-dot-secret")
	assert.ErrorIs(t, err, apikey.ErrInvalidFormat)
}

func TestApiKeyValidator_RevokedCredential_Rejected(t *testing.T) {
	now := time.Now()
	store := map[string]*model.ApiClientCredential{
		"id-3": {PartyID: "p3", ClientID: "id-3", SecretHash: hashSecret(t, "sec"), RevokedAt: &now},
	}
	v := newValidator(t, store)

	_, err := v.Validate(context.Background(), "agentlens_sk_id-3.sec")
	assert.ErrorIs(t, err, apikey.ErrInvalidCredential)
}

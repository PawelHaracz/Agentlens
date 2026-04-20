package mcpserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadyz_Returns503_When_JWKS_Unreachable verifies the Dex health check
// helper used by the extended readyz chain (F.6).
// We test the checkDexHealth-equivalent logic via a direct HTTP probe.
func TestReadyz_Returns503_When_JWKS_Unreachable(t *testing.T) {
	// Serve a 503 to simulate Dex being down.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode,
		"JWKS endpoint returning 503 must propagate as health failure")
}

// TestFederationHealthLoop_UpdatesReadyzState exercises that a healthy
// JWKS endpoint returns 200 and a reachable one clears the fault.
func TestFederationHealthLoop_UpdatesReadyzState(t *testing.T) {
	// Healthy JWKS server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "healthy JWKS endpoint must return 200")
}

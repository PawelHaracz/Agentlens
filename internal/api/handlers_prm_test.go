package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api"
)

func TestPRMHandler_ReturnsRFC9728_Doc_When_Enabled(t *testing.T) {
	h := api.NewPRMHandler("https://agentlens.example.com/mcp", "https://dex.example.com")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &doc))
	assert.Equal(t, "https://agentlens.example.com/mcp", doc["resource"])
	servers, ok := doc["authorization_servers"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, servers, "https://dex.example.com")
}

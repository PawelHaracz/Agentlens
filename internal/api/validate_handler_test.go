package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
)

func TestValidateEndpoint_ValidCard(t *testing.T) {
	router, _ := newTestRouter(t)
	body, err := os.ReadFile("../../plugins/parsers/a2a/testdata/a2a_v10_card.json")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result a2a.ValidationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.True(t, result.Valid)
	assert.Equal(t, "1.0", result.SpecVersion)
	assert.NotNil(t, result.Preview)
	assert.Equal(t, "Translation Agent v1.0", result.Preview.DisplayName)
}

func TestValidateEndpoint_InvalidCard(t *testing.T) {
	router, _ := newTestRouter(t)
	body, err := os.ReadFile("../../plugins/parsers/a2a/testdata/a2a_invalid_card.json")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result a2a.ValidationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestValidateEndpoint_InvalidJSON(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/validate", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

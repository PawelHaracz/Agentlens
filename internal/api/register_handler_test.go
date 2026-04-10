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

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestRegisterAgentCard_ValidCard(t *testing.T) {
	router, _ := newTestRouter(t)
	body, err := os.ReadFile("../../plugins/parsers/a2a/testdata/a2a_v10_card.json")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Check scalar fields via a partial decode.
	var entry struct {
		ID          string               `json:"id"`
		DisplayName string               `json:"display_name"`
		Endpoint    string               `json:"endpoint"`
		Protocol    model.Protocol       `json:"protocol"`
		SpecVersion string               `json:"spec_version"`
		Source      model.SourceType     `json:"source"`
		Status      model.LifecycleState `json:"status"`
		CreatedAt   string               `json:"created_at"`
		UpdatedAt   string               `json:"updated_at"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "Translation Agent v1.0", entry.DisplayName)
	assert.Equal(t, "https://translate.example.com/a2a", entry.Endpoint)
	assert.Equal(t, model.ProtocolA2A, entry.Protocol)
	assert.Equal(t, "1.0", entry.SpecVersion)
	assert.Equal(t, model.SourcePush, entry.Source)
	assert.Equal(t, model.LifecycleRegistered, entry.Status)
	assert.NotEmpty(t, entry.CreatedAt)
	assert.NotEmpty(t, entry.UpdatedAt)

	// Verify capabilities is present and non-empty.
	capabilities, ok := resp["capabilities"]
	assert.True(t, ok, "response should include capabilities")
	assert.NotEqual(t, "[]", string(capabilities), "capabilities should not be empty")
}

func TestRegisterAgentCard_InvalidCard(t *testing.T) {
	router, _ := newTestRouter(t)
	body, err := os.ReadFile("../../plugins/parsers/a2a/testdata/a2a_invalid_card.json")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result kernel.ValidationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.False(t, result.Valid)
	assert.NotEmpty(t, result.Errors)
}

func TestRegisterAgentCard_InvalidJSON(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result kernel.ValidationResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.False(t, result.Valid)
}

func TestRegisterAgentCard_EmptyBody(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/register", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "empty")
}

func TestRegisterAgentCard_DuplicateEndpoint(t *testing.T) {
	router, _ := newTestRouter(t)
	body, err := os.ReadFile("../../plugins/parsers/a2a/testdata/a2a_v10_card.json")
	require.NoError(t, err)

	// First registration should succeed.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Second registration with the same card (same endpoint) should conflict.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/catalog/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "endpoint already exists")
}

func TestRegisterAgentCard_SecurityDetailRoundtrip(t *testing.T) {
	router, _ := newTestRouter(t)
	card := []byte(`{
		"version": "1.0",
		"name": "Security Roundtrip Agent",
		"description": "Tests security_detail roundtrip",
		"url": "https://sec-roundtrip.example.com",
		"securitySchemes": {
			"httpAuth": {"type": "http", "scheme": "Bearer", "bearerFormat": "JWT"},
			"apiKeyAuth": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
		},
		"securityRequirements": [{"httpAuth": []}, {"apiKeyAuth": []}],
		"skills": []
	}`)

	// Register
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/register", bytes.NewReader(card))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var created map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)

	// Fetch via GET
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/"+id, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var fetched map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &fetched))

	secDetail, ok := fetched["security_detail"].(map[string]interface{})
	require.True(t, ok, "Expected security_detail in GET response, got: %v", fetched["security_detail"])

	schemes, ok := secDetail["security_schemes"].([]interface{})
	require.True(t, ok, "Expected security_schemes array")
	assert.Len(t, schemes, 2, "Expected 2 schemes")

	// Check that scheme_names are preserved
	schemeNames := make([]string, 0, len(schemes))
	for _, s := range schemes {
		sm := s.(map[string]interface{})
		if sn, ok := sm["scheme_name"].(string); ok {
			schemeNames = append(schemeNames, sn)
		}
	}
	assert.ElementsMatch(t, []string{"httpAuth", "apiKeyAuth"}, schemeNames)

	reqs, ok := secDetail["security_requirements"].([]interface{})
	require.True(t, ok, "Expected security_requirements array")
	assert.Len(t, reqs, 2, "Expected 2 requirements")
}

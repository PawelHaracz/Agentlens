package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/service"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// mockFetcher implements service.Fetcher for tests.
// It bypasses SSRF checks and returns pre-configured data.
type mockFetcher struct {
	rawJSON  json.RawMessage
	protocol string
	fetchErr error
}

func (m *mockFetcher) ValidateURL(rawURL string) error { return nil }
func (m *mockFetcher) Fetch(_ context.Context, _ string) (*service.FetchResult, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return &service.FetchResult{
		RawJSON:          m.rawJSON,
		DetectedProtocol: m.protocol,
	}, nil
}

// routerWithMockFetcher builds a test router that uses the provided mock fetcher.
func routerWithMockFetcher(t *testing.T, f service.Fetcher) (http.Handler, store.Store) {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return api.NewRouter(api.RouterDeps{Store: s, CardFetcher: f}), s
}

// validA2ACard is a minimal valid A2A v1.0 agent card for import tests.
const validA2ACard = `{
  "name": "Import Test Agent",
  "description": "Test agent for URL import",
  "version": "1.0.0",
  "supportedInterfaces": [{"url": "https://import-test.example.com/a2a", "binding": "jsonrpc"}],
  "capabilities": {"supportsExtendedAgentCard": true},
  "skills": [
    {"id": "s1", "name": "Skill One", "description": "Does something"}
  ]
}`

// validMCPCard is a minimal valid MCP server card for import tests.
const validMCPCard = `{
  "name": "Import MCP Server",
  "description": "Test MCP server for URL import",
  "version": "1.0.0",
  "remotes": [{"url": "https://mcp-import-test.example.com/mcp"}],
  "tools": [{"name": "tool1", "description": "A tool"}]
}`

func TestImportCatalogEntry_HappyPath_A2A(t *testing.T) {
	f := &mockFetcher{rawJSON: json.RawMessage(validA2ACard), protocol: "a2a"}
	router, _ := routerWithMockFetcher(t, f)

	body := `{"url":"https://example.com/.well-known/agent.json"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var entry struct {
		ID          string           `json:"id"`
		DisplayName string           `json:"display_name"`
		Protocol    model.Protocol   `json:"protocol"`
		Source      model.SourceType `json:"source"`
		Status      model.Status     `json:"status"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "Import Test Agent", entry.DisplayName)
	assert.Equal(t, model.ProtocolA2A, entry.Protocol)
	assert.Equal(t, model.SourcePush, entry.Source)
	assert.Equal(t, model.StatusUnknown, entry.Status)
}

func TestImportCatalogEntry_HappyPath_MCP_AutoDetect(t *testing.T) {
	// protocol detected from "tools" field (no URL pattern match)
	f := &mockFetcher{rawJSON: json.RawMessage(validMCPCard), protocol: "mcp"}
	router, _ := routerWithMockFetcher(t, f)

	body := `{"url":"https://example.com/server"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var entry struct {
		Protocol model.Protocol `json:"protocol"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.Equal(t, model.ProtocolMCP, entry.Protocol)
}

func TestImportCatalogEntry_ExplicitProtocolOverride(t *testing.T) {
	// Mock returns no detected protocol; caller sets it explicitly.
	f := &mockFetcher{rawJSON: json.RawMessage(validA2ACard), protocol: ""}
	router, _ := routerWithMockFetcher(t, f)

	body := `{"url":"https://example.com/card","protocol":"a2a"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

	var entry struct {
		Protocol model.Protocol `json:"protocol"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.Equal(t, model.ProtocolA2A, entry.Protocol)
}

func TestImportCatalogEntry_InvalidURL_Empty(t *testing.T) {
	// URL validation uses the real service.ValidateURL via the real fetcher (default).
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(`{"url":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportCatalogEntry_InvalidURL_BadScheme(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(`{"url":"ftp://example.com/card"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportCatalogEntry_PrivateIP_Blocked(t *testing.T) {
	// URL validation is done by the real fetcher; mock fetcher bypasses it.
	// Use the default (real) router so that SSRF protection is active.
	router, _ := newTestRouter(t)
	for _, addr := range []string{
		"http://127.0.0.1/card",
		"http://localhost/card",
		"http://10.0.0.1/card",
		"http://192.168.1.1/card",
		"http://172.16.0.1/card",
		"http://169.254.169.254/card",
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(`{"url":"`+addr+`"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "expected block for %s", addr)
	}
}

func TestImportCatalogEntry_NonJSON_Response(t *testing.T) {
	f := &mockFetcher{fetchErr: fmt.Errorf("response is not valid JSON")}
	router, _ := routerWithMockFetcher(t, f)

	body := `{"url":"https://example.com/.well-known/agent.json"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestImportCatalogEntry_InvalidCardSchema(t *testing.T) {
	// Returns valid JSON but not a valid A2A card (missing required fields).
	f := &mockFetcher{rawJSON: json.RawMessage(`{"skills": [], "foo": "bar"}`), protocol: "a2a"}
	router, _ := routerWithMockFetcher(t, f)

	body := `{"url":"https://example.com/.well-known/agent.json"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestImportCatalogEntry_URLUnreachable(t *testing.T) {
	f := &mockFetcher{fetchErr: fmt.Errorf("fetching url: connection refused")}
	router, _ := routerWithMockFetcher(t, f)

	body := `{"url":"https://unreachable.example.com/.well-known/agent.json"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestImportCatalogEntry_DuplicateEndpoint(t *testing.T) {
	f := &mockFetcher{rawJSON: json.RawMessage(validA2ACard), protocol: "a2a"}
	router, _ := routerWithMockFetcher(t, f)

	body := `{"url":"https://example.com/.well-known/agent.json"}`

	// First import — should succeed.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "first import should succeed: %s", w.Body.String())

	// Second import — same endpoint, should conflict.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["error"], "endpoint already exists")
}

func TestImportCatalogEntry_InvalidRequestBody(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/import", stringReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}


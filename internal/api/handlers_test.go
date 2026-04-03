package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	a2aplugin "github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
	mcpplugin "github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
)

func newTestRouter(t *testing.T) (http.Handler, store.Store) {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	core := kernel.NewCore(s, nil, slog.Default(), kernel.LicenseInfo{})
	a2aParser := a2aplugin.New()
	_ = a2aParser.Init(core)
	core.RegisterParser(a2aParser)
	mcpParser := mcpplugin.New()
	_ = mcpParser.Init(core)
	core.RegisterParser(mcpParser)

	return api.NewRouter(api.RouterDeps{Kernel: core}), s
}

func TestHealthz(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
}

func TestListCatalog_Empty(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateEntry(t *testing.T) {
	router, _ := newTestRouter(t)

	body := map[string]interface{}{
		"display_name": "My Entry",
		"description":  "A great entry",
		"protocol":     "a2a",
		"endpoint":     "http://agent.example.com",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var entry model.CatalogEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "My Entry", entry.DisplayName)
	assert.Equal(t, model.SourcePush, entry.Source)
	assert.Equal(t, model.StatusUnknown, entry.Status)
}

func TestGetEntry_NotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/does-not-exist", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteEntry(t *testing.T) {
	router, s := newTestRouter(t)

	now := time.Now().UTC()
	agentType := &model.AgentType{
		ID:            "at-del-1",
		AgentKey:      model.ComputeAgentKey(model.ProtocolA2A, "http://del.example.com"),
		Protocol:      model.ProtocolA2A,
		Endpoint:      "http://del.example.com",
		RawDefinition: []byte("{}"),
		CreatedOn:     now,
	}
	e := &model.CatalogEntry{
		ID:          "del-1",
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: "Del Entry",
		Status:      model.StatusUnknown,
		Source:      model.SourcePush,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	require.NoError(t, s.Create(context.Background(), e))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/catalog/del-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestGetStats(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

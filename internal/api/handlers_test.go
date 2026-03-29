package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func newTestRouter(t *testing.T) (http.Handler, store.Store) {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return api.NewRouter(s), s
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

func TestListAgents_Empty(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateAgent(t *testing.T) {
	router, _ := newTestRouter(t)

	body := map[string]interface{}{
		"name":        "My Agent",
		"description": "A great agent",
		"protocol":    "a2a",
		"endpoint":    "http://agent.example.com",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var agent model.Agent
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &agent))
	assert.NotEmpty(t, agent.ID)
	assert.Equal(t, "My Agent", agent.Name)
	assert.Equal(t, model.SourcePush, agent.Source)
	assert.Equal(t, model.StatusUnknown, agent.Status)
}

func TestGetAgent_NotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/does-not-exist", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteAgent(t *testing.T) {
	router, s := newTestRouter(t)

	now := time.Now().UTC()
	a := &model.Agent{
		ID: "del-1", Name: "Del Agent", Protocol: model.ProtocolA2A,
		Endpoint: "http://del.example.com", Status: model.StatusUnknown,
		Source: model.SourcePush, LastSeen: now, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, s.Create(context.Background(), a))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/del-1", nil)
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

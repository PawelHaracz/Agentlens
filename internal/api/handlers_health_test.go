package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func makeTestCatalogEntry(id string, state model.LifecycleState) *model.CatalogEntry {
	now := time.Now().UTC()
	agentType := &model.AgentType{
		ID:            id + "-type",
		Protocol:      model.ProtocolA2A,
		Endpoint:      "http://test-" + id + ".example.com",
		Version:       "1.0.0",
		RawDefinition: []byte("{}"),
		CreatedOn:     now,
	}
	agentType.AgentKey = model.ComputeAgentKey(agentType.Protocol, agentType.Endpoint)
	e := &model.CatalogEntry{
		ID:          id,
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: "Test " + id,
		Source:      model.SourcePush,
		Status:      state,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	e.SyncFromDB()
	return e
}

func TestListCatalogStateFilter(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	// Create entries with different lifecycle states
	entries := []*model.CatalogEntry{
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleActive),
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleOffline),
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleDeprecated),
	}
	for _, e := range entries {
		require.NoError(t, s.Create(ctx, e))
		require.NoError(t, s.SetLifecycle(ctx, e.ID, e.Status))
	}

	activeID := entries[0].ID
	offlineID := entries[1].ID
	deprecatedID := entries[2].ID

	// Test: filter by single state (active)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog?state=active", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	ids := make(map[string]bool)
	for _, entry := range resp {
		ids[entry["id"].(string)] = true
	}
	assert.True(t, ids[activeID], "active entry should be in result")
	assert.False(t, ids[offlineID], "offline entry should NOT be in result")
	assert.False(t, ids[deprecatedID], "deprecated entry should NOT be in result")
}

func TestListCatalogStateFilterMultiple(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	// Create entries with different lifecycle states
	entries := []*model.CatalogEntry{
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleActive),
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleOffline),
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleDeprecated),
	}
	for _, e := range entries {
		require.NoError(t, s.Create(ctx, e))
		require.NoError(t, s.SetLifecycle(ctx, e.ID, e.Status))
	}

	activeID := entries[0].ID
	offlineID := entries[1].ID
	deprecatedID := entries[2].ID

	// Test: filter by multiple states (active,offline)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog?state=active,offline", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	ids := make(map[string]bool)
	for _, entry := range resp {
		ids[entry["id"].(string)] = true
	}
	assert.True(t, ids[activeID], "active entry should be in result")
	assert.True(t, ids[offlineID], "offline entry should be in result")
	assert.False(t, ids[deprecatedID], "deprecated entry should NOT be in result")
}

func TestListCatalogInvalidState(t *testing.T) {
	router, _ := newTestRouter(t)

	// Test: invalid state value should return 400
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog?state=bogus", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListCatalogStateFilterBackwardCompat(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()

	// Create entries with different lifecycle states
	entries := []*model.CatalogEntry{
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleActive),
		makeTestCatalogEntry(uuid.NewString(), model.LifecycleOffline),
	}
	for _, e := range entries {
		require.NoError(t, s.Create(ctx, e))
		require.NoError(t, s.SetLifecycle(ctx, e.ID, e.Status))
	}

	activeID := entries[0].ID

	// Test: backward-compat status param should work as single state filter
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog?status=active", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	ids := make(map[string]bool)
	for _, entry := range resp {
		ids[entry["id"].(string)] = true
	}
	assert.True(t, ids[activeID], "active entry should be in result via status param")
}

func TestCatalogEntryResponseIncludesHealth(t *testing.T) {
	entry := makeTestCatalogEntry("health-json-test", model.LifecycleActive)

	b, err := json.Marshal(entry)
	require.NoError(t, err)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &out))

	assert.Equal(t, "active", out["status"], "status should be active")
	health, ok := out["health"].(map[string]interface{})
	assert.True(t, ok, "health field should exist in JSON response")
	assert.Equal(t, "active", health["state"], "health.state should be active")
}

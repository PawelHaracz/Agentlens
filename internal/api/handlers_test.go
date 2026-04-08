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
	"gorm.io/gorm"

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

// newTestRouterWithCardStore builds a test router that has a mock CardStorePlugin registered.
func newTestRouterWithCardStore(t *testing.T, cs kernel.CardStorePlugin) (http.Handler, store.Store) {
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
	core.RegisterCardStore(cs)

	return api.NewRouter(api.RouterDeps{Kernel: core}), s
}

// mockCardStore is a simple in-memory CardStorePlugin for tests.
type mockCardStore struct {
	cards map[string]*model.RawCard
}

func (m *mockCardStore) Name() string                  { return "mock-card-store" }
func (m *mockCardStore) Version() string               { return "0.0.1" }
func (m *mockCardStore) Type() kernel.PluginType       { return kernel.PluginTypeCardStore }
func (m *mockCardStore) Init(_ kernel.Kernel) error    { return nil }
func (m *mockCardStore) Start(_ context.Context) error { return nil }
func (m *mockCardStore) Stop(_ context.Context) error  { return nil }
func (m *mockCardStore) StoreCard(_ context.Context, id string, data []byte, ct string) error {
	m.cards[id] = &model.RawCard{AgentTypeID: id, Data: data, ContentType: ct, FetchedAt: time.Now()}
	return nil
}
func (m *mockCardStore) GetCard(_ context.Context, id string) (*model.RawCard, error) {
	c, ok := m.cards[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return c, nil
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

	var entry struct {
		ID          string               `json:"id"`
		DisplayName string               `json:"display_name"`
		Source      model.SourceType     `json:"source"`
		Status      model.LifecycleState `json:"status"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entry))
	assert.NotEmpty(t, entry.ID)
	assert.Equal(t, "My Entry", entry.DisplayName)
	assert.Equal(t, model.SourcePush, entry.Source)
	assert.Equal(t, model.LifecycleRegistered, entry.Status)
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
		ID:        "at-del-1",
		AgentKey:  model.ComputeAgentKey(model.ProtocolA2A, "http://del.example.com"),
		Protocol:  model.ProtocolA2A,
		Endpoint:  "http://del.example.com",
		CreatedOn: now,
	}
	e := &model.CatalogEntry{
		ID:          "del-1",
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: "Del Entry",
		Status:      model.LifecycleRegistered,
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

func TestCreateEntry_DuplicateEndpoint(t *testing.T) {
	router, _ := newTestRouter(t)

	body := map[string]interface{}{
		"display_name": "First Agent",
		"protocol":     "a2a",
		"endpoint":     "http://dup.example.com",
		"version":      "1.0.0",
	}
	bodyBytes, _ := json.Marshal(body)

	// First creation succeeds.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Same endpoint, different version → must still return 409.
	body2 := map[string]interface{}{
		"display_name": "Duplicate Agent",
		"protocol":     "a2a",
		"endpoint":     "http://dup.example.com",
	}
	bodyBytes2, _ := json.Marshal(body2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/catalog", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestGetStats(t *testing.T) {
	router, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetEntryCard_FromCardStore(t *testing.T) {
	t.Run("non-existent entry returns 404", func(t *testing.T) {
		router, _ := newTestRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/does-not-exist/card", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("existing entry but no card store returns 404", func(t *testing.T) {
		router, s := newTestRouter(t)

		now := time.Now().UTC()
		agentType := &model.AgentType{
			ID:        "at-card-1",
			AgentKey:  model.ComputeAgentKey(model.ProtocolA2A, "http://card.example.com"),
			Protocol:  model.ProtocolA2A,
			Endpoint:  "http://card.example.com",
			CreatedOn: now,
		}
		e := &model.CatalogEntry{
			ID:          "entry-card-1",
			AgentTypeID: agentType.ID,
			AgentType:   agentType,
			DisplayName: "Card Entry",
			Status:      model.LifecycleRegistered,
			Source:      model.SourcePush,
			Validity:    model.Validity{LastSeen: now},
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		require.NoError(t, s.Create(context.Background(), e))

		// newTestRouter does not register a CardStorePlugin, so CardStore() returns nil → 404.
		req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/entry-card-1/card", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("existing entry with card store returns 200 and card data", func(t *testing.T) {
		cs := &mockCardStore{cards: make(map[string]*model.RawCard)}
		router, s := newTestRouterWithCardStore(t, cs)

		now := time.Now().UTC()
		agentType := &model.AgentType{
			ID:        "at-card-happy",
			AgentKey:  model.ComputeAgentKey(model.ProtocolA2A, "http://happy.example.com"),
			Protocol:  model.ProtocolA2A,
			Endpoint:  "http://happy.example.com",
			CreatedOn: now,
		}
		e := &model.CatalogEntry{
			ID:          "entry-card-happy",
			AgentTypeID: agentType.ID,
			AgentType:   agentType,
			DisplayName: "Happy Card Entry",
			Status:      model.LifecycleRegistered,
			Source:      model.SourcePush,
			Validity:    model.Validity{LastSeen: now},
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		require.NoError(t, s.Create(context.Background(), e))

		cardData := []byte(`{"name":"happy-agent"}`)
		require.NoError(t, cs.StoreCard(context.Background(), agentType.ID, cardData, "application/json"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/entry-card-happy/card", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, cardData, w.Body.Bytes())
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})
}

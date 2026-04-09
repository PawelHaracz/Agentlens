package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestCapabilityHandlerListCapabilities(t *testing.T) {
	router, st := newTestRouter(t)

	// Seed test data
	ctx := context.Background()
	now := time.Now().UTC()
	agentType := &model.AgentType{
		ID:       "at-cap-1",
		AgentKey: model.ComputeAgentKey(model.ProtocolA2A, "http://test.local/a2a"),
		Protocol: model.ProtocolA2A,
		Endpoint: "http://test.local/a2a",
		Capabilities: []model.Capability{
			&model.A2ASkill{
				Name:        "Test Skill",
				Description: "Test description",
				Tags:        []string{"test"},
			},
		},
		CreatedOn: now,
	}
	entry := &model.CatalogEntry{
		ID:          "test-entry-1",
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: "Test Agent",
		Status:      model.LifecycleActive,
		Source:      model.SourcePush,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := st.Create(ctx, entry); err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	t.Run("list capabilities", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result model.CapabilityListResult
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("expected total=1, got %d", result.Total)
		}
		if len(result.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(result.Items))
		}
		if result.Items[0].Kind != "a2a.skill" {
			t.Errorf("expected kind=a2a.skill, got %s", result.Items[0].Kind)
		}
		if result.Items[0].Name != "Test Skill" {
			t.Errorf("expected name='Test Skill', got %s", result.Items[0].Name)
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?kind=a2a.skill", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("unknown kind returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?kind=unknown.kind", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("unknown sort returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities?sort=invalid", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})
}

func TestCapabilityHandlerGetCapabilityAgents(t *testing.T) {
	router, st := newTestRouter(t)

	// Seed test data
	ctx := context.Background()
	now := time.Now().UTC()
	agentType := &model.AgentType{
		ID:          "at-cap-2",
		AgentKey:    model.ComputeAgentKey(model.ProtocolA2A, "http://test.local/a2a"),
		Protocol:    model.ProtocolA2A,
		Endpoint:    "http://test.local/a2a",
		SpecVersion: "1.0",
		Capabilities: []model.Capability{
			&model.A2ASkill{
				Name:        "Test Skill",
				Description: "Test description",
			},
		},
		CreatedOn: now,
	}
	entry := &model.CatalogEntry{
		ID:          "test-entry-1",
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: "Test Agent",
		Status:      model.LifecycleActive,
		Source:      model.SourcePush,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := st.Create(ctx, entry); err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	t.Run("get capability agents", func(t *testing.T) {
		key := "a2a.skill::Test%20Skill"
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/"+key, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var result map[string]any
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		capability, ok := result["capability"].(map[string]any)
		if !ok {
			t.Fatalf("missing capability field")
		}
		if capability["kind"] != "a2a.skill" {
			t.Errorf("expected kind=a2a.skill, got %v", capability["kind"])
		}
		if capability["name"] != "Test Skill" {
			t.Errorf("expected name='Test Skill', got %v", capability["name"])
		}

		agents, ok := result["agents"].([]any)
		if !ok {
			t.Fatalf("missing agents field")
		}
		if len(agents) != 1 {
			t.Errorf("expected 1 agent, got %d", len(agents))
		}
	})

	t.Run("malformed key returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/no-separator", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("non-existent capability returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/a2a.skill::NonExistent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

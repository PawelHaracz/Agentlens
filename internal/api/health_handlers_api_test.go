package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/api"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	a2aplugin "github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
	mcpplugin "github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
)

// mockProber is a test double for api.HealthProber.
type mockProber struct {
	health model.Health
	err    error
}

func (m *mockProber) ProbeEntry(_ context.Context, _ string) (model.Health, error) {
	return m.health, m.err
}

// buildTestRouterWithProber creates a test router with no auth and an optional prober.
func buildTestRouterWithProber(t *testing.T, s *store.SQLStore, prober api.HealthProber) http.Handler {
	t.Helper()
	t.Cleanup(func() { _ = s.Close() })

	core := kernel.NewCore(s, nil, slog.Default(), kernel.LicenseInfo{})
	a2aParser := a2aplugin.New()
	_ = a2aParser.Init(core)
	core.RegisterParser(a2aParser)
	mcpParser := mcpplugin.New()
	_ = mcpParser.Init(core)
	core.RegisterParser(mcpParser)

	return api.NewRouter(api.RouterDeps{
		Kernel:       core,
		HealthProber: prober,
	})
}

func TestPatchLifecycleDeprecate(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	entry := makeTestCatalogEntry(uuid.NewString(), model.LifecycleActive)
	if err := s.Create(context.Background(), entry); err != nil {
		t.Fatalf("create: %v", err)
	}

	router := buildTestRouterWithProber(t, s, nil)

	body, _ := json.Marshal(map[string]string{"state": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/"+entry.ID+"/lifecycle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "deprecated" {
		t.Errorf("status = %v, want deprecated", resp["status"])
	}
}

func TestPatchLifecycleInvalidState(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	entry := makeTestCatalogEntry(uuid.NewString(), model.LifecycleActive)
	if err := s.Create(context.Background(), entry); err != nil {
		t.Fatalf("create: %v", err)
	}

	router := buildTestRouterWithProber(t, s, nil)
	body, _ := json.Marshal(map[string]string{"state": "offline"}) // not allowed via PATCH
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/"+entry.ID+"/lifecycle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for offline state", w.Code)
	}
}

func TestPatchLifecycleNotFound(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	router := buildTestRouterWithProber(t, s, nil)
	body, _ := json.Marshal(map[string]string{"state": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/catalog/does-not-exist/lifecycle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestPostProbeNoProber(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	entry := makeTestCatalogEntry(uuid.NewString(), model.LifecycleRegistered)
	if err := s.Create(context.Background(), entry); err != nil {
		t.Fatalf("create: %v", err)
	}

	router := buildTestRouterWithProber(t, s, nil) // nil prober → 503
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/"+entry.ID+"/probe", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when no prober", w.Code)
	}
}

func TestPostProbeWithProber(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	entry := makeTestCatalogEntry(uuid.NewString(), model.LifecycleRegistered)
	if err := s.Create(context.Background(), entry); err != nil {
		t.Fatalf("create: %v", err)
	}

	prober := &mockProber{health: model.Health{State: model.LifecycleActive, LatencyMs: 42}}
	router := buildTestRouterWithProber(t, s, prober)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/"+entry.ID+"/probe", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var health map[string]any
	_ = json.NewDecoder(w.Body).Decode(&health)
	if health["state"] != "active" {
		t.Errorf("health.state = %v, want active", health["state"])
	}
}

func TestPostProbeRateLimit(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	entry := makeTestCatalogEntry(uuid.NewString(), model.LifecycleRegistered)
	if err := s.Create(context.Background(), entry); err != nil {
		t.Fatalf("create: %v", err)
	}

	prober := &mockProber{health: model.Health{State: model.LifecycleActive}}
	router := buildTestRouterWithProber(t, s, prober)

	// First call should succeed
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/"+entry.ID+"/probe", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first probe status = %d, want 200", w1.Code)
	}

	// Immediate second call should be rate-limited (429)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/catalog/"+entry.ID+"/probe", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second probe status = %d, want 429", w2.Code)
	}
}

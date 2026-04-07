package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/plugins/health"
)

func entryWithEndpoint(endpoint string) *model.CatalogEntry {
	return &model.CatalogEntry{
		ID:     "test-entry",
		Status: model.LifecycleRegistered,
		AgentType: &model.AgentType{
			Protocol: model.ProtocolA2A,
			Endpoint: endpoint,
		},
	}
}

// Test: 200 fast → active
func TestProbeOneFreshActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := health.NewForTest(1500*time.Millisecond, 3)
	entry := entryWithEndpoint(srv.URL)

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if h.State != model.LifecycleActive {
		t.Errorf("State = %v, want active", h.State)
	}
	if h.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %v, want 0", h.ConsecutiveFailures)
	}
}

// Test: 200 slow → degraded
func TestProbeOneSlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := health.NewForTest(10*time.Millisecond, 3) // degradedLatency=10ms so 20ms triggers degraded
	entry := entryWithEndpoint(srv.URL)

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if h.State != model.LifecycleDegraded {
		t.Errorf("State = %v, want degraded (slow response)", h.State)
	}
}

// Test: 500 once → degraded, failures=1
func TestProbeOneServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := health.NewForTest(1500*time.Millisecond, 3)
	entry := entryWithEndpoint(srv.URL)

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if h.State != model.LifecycleDegraded {
		t.Errorf("State = %v, want degraded (single 500)", h.State)
	}
	if h.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %v, want 1", h.ConsecutiveFailures)
	}
}

// Test: 3 consecutive failures → offline
func TestProbeOneReachesOffline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := health.NewForTest(1500*time.Millisecond, 3)
	entry := entryWithEndpoint(srv.URL)
	entry.Health = model.Health{
		State:               model.LifecycleDegraded,
		ConsecutiveFailures: 2,
	}

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if h.State != model.LifecycleOffline {
		t.Errorf("State = %v, want offline (3 consecutive failures)", h.State)
	}
}

// Test: offline → 200 fast → active, failures reset
func TestProbeOneRecovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := health.NewForTest(1500*time.Millisecond, 3)
	entry := entryWithEndpoint(srv.URL)
	entry.Health = model.Health{
		State:               model.LifecycleOffline,
		ConsecutiveFailures: 5,
	}
	entry.Status = model.LifecycleOffline

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if h.State != model.LifecycleActive {
		t.Errorf("State = %v, want active (recovery)", h.State)
	}
	if h.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %v, want 0 after recovery", h.ConsecutiveFailures)
	}
}

// Test: deprecated entry → no HTTP call
func TestProbeOneSkipsDeprecated(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := health.NewForTest(1500*time.Millisecond, 3)
	entry := entryWithEndpoint(srv.URL)
	entry.Status = model.LifecycleDeprecated
	entry.Health = model.Health{State: model.LifecycleDeprecated}

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if called {
		t.Error("HTTP call was made for a deprecated entry — should have been skipped")
	}
	if h.State != model.LifecycleDeprecated {
		t.Errorf("State = %v, want deprecated (passthrough)", h.State)
	}
}

// Test: no URL → offline, no HTTP call
func TestProbeOneNoURL(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	p := health.NewForTest(1500*time.Millisecond, 3)
	entry := &model.CatalogEntry{
		ID:        "no-url",
		Status:    model.LifecycleRegistered,
		AgentType: &model.AgentType{Protocol: model.ProtocolMCP, Endpoint: ""},
	}

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if called {
		t.Error("HTTP call should not happen when there is no URL")
	}
	if h.State != model.LifecycleOffline {
		t.Errorf("State = %v, want offline (no URL)", h.State)
	}
	if h.LastError == "" {
		t.Error("LastError should be set when there is no URL")
	}
}

// Test: probe timeout → counted as failure
func TestProbeOneTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := health.NewForTestWithTimeout(1500*time.Millisecond, 3, 50*time.Millisecond)
	entry := entryWithEndpoint(srv.URL)

	h, err := p.ProbeOneForTest(context.Background(), entry)
	if err != nil {
		t.Fatalf("ProbeOne: %v", err)
	}
	if h.State == model.LifecycleActive {
		t.Error("timed out probe should not result in active state")
	}
	if h.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %v, want 1 after timeout", h.ConsecutiveFailures)
	}
}

package mcpserver_test

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

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver/wire"
)

// --- stub session store ---

type stubSessionStore struct {
	sessions map[string]*model.McpSession
	active   int64
}

func newStubStore() *stubSessionStore {
	return &stubSessionStore{sessions: make(map[string]*model.McpSession)}
}

func (s *stubSessionStore) Create(_ context.Context, sess *model.McpSession) error {
	s.sessions[sess.ID] = sess
	s.active++
	return nil
}
func (s *stubSessionStore) GetByID(_ context.Context, id string) (*model.McpSession, error) {
	return s.sessions[id], nil
}
func (s *stubSessionStore) UpdateInitialized(_ context.Context, id string, at time.Time) error {
	if sess, ok := s.sessions[id]; ok {
		sess.InitializedAt = &at
	}
	return nil
}
func (s *stubSessionStore) UpdateLastSeen(_ context.Context, _ string, _ time.Time) error {
	return nil
}
func (s *stubSessionStore) Revoke(_ context.Context, id string) error {
	if sess, ok := s.sessions[id]; ok {
		now := time.Now()
		sess.RevokedAt = &now
		s.active--
	}
	return nil
}
func (s *stubSessionStore) ReapExpired(_ context.Context, before time.Time) (int64, error) {
	var count int64
	for _, sess := range s.sessions {
		if sess.RevokedAt == nil && sess.ExpiresAt.Before(before) {
			now := time.Now()
			sess.RevokedAt = &now
			s.active--
			count++
		}
	}
	return count, nil
}
func (s *stubSessionStore) ReapOrphanedPrincipals(_ context.Context) (int64, error) { return 0, nil }
func (s *stubSessionStore) CountActive(_ context.Context) (int64, error)            { return s.active, nil }

// --- fake kernel ---

type fakeKernel struct {
	cfg    *config.Config
	routes map[string]http.Handler
}

func newFakeKernel(mcpEnabled bool) *fakeKernel {
	cfg := &config.Config{}
	cfg.MCP = config.MCPServerConfig{
		Enabled:        mcpEnabled,
		PublicURL:      "http://localhost:8080/api/mcp",
		AuditEnabled:   true,
		SessionTTL:     30 * time.Minute,
		ReaperInterval: 60 * time.Second,
	}
	return &fakeKernel{cfg: cfg, routes: make(map[string]http.Handler)}
}

func (k *fakeKernel) Store() store.Store                                   { return nil }
func (k *fakeKernel) Config() *config.Config                               { return k.cfg }
func (k *fakeKernel) Logger() *slog.Logger                                 { return slog.Default() }
func (k *fakeKernel) License() kernel.LicenseInfo                          { return kernel.LicenseInfo{} }
func (k *fakeKernel) Parser(_ model.Protocol) (kernel.ParserPlugin, bool)  { return nil, false }
func (k *fakeKernel) RegisterRoutes(prefix string, h http.Handler)         { k.routes[prefix] = h }
func (k *fakeKernel) RegisterMiddleware(_ func(http.Handler) http.Handler) {}
func (k *fakeKernel) CardStore() kernel.CardStorePlugin                    { return nil }
func (k *fakeKernel) Routes() map[string]http.Handler                      { return k.routes }

// --- helpers ---

func initPlugin(t *testing.T, enabled bool) (*mcpserver.Plugin, *fakeKernel, *stubSessionStore) {
	t.Helper()
	ss := newStubStore()
	p := mcpserver.NewForTest(ss)
	k := newFakeKernel(enabled)
	require.NoError(t, p.Init(k))
	return p, k, ss
}

func postJSON(t *testing.T, h http.Handler, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func mcpReq(method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": "1", "method": method, "params": params}
}

// --- tests ---

func TestPlugin_Lifecycle_Register_Init_Start_Stop(t *testing.T) {
	p, _, _ := initPlugin(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, p.Start(ctx))
	require.NoError(t, p.Stop(context.Background()))
}

func TestPlugin_Disabled_WhenFlagFalse_NoRoutesRegistered(t *testing.T) {
	_, k, _ := initPlugin(t, false)
	assert.Empty(t, k.routes, "disabled plugin must register no routes")
}

func TestStreamableHTTP_POST_InitializeHandshake_Returns_SessionID(t *testing.T) {
	_, k, _ := initPlugin(t, true)
	h, ok := k.routes["/api/mcp"]
	require.True(t, ok, "/api/mcp route must be registered when enabled")

	w := postJSON(t, h, mcpReq("initialize", map[string]any{}))
	assert.Equal(t, http.StatusOK, w.Code)

	sessionID := w.Header().Get(wire.HeaderSessionID())
	assert.NotEmpty(t, sessionID, "MCP-Session-Id must be set after initialize")

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result, _ := resp["result"].(map[string]any)
	assert.Equal(t, wire.CurrentProtocolVersion, result["protocolVersion"])
}

func TestStreamableHTTP_GET_ServerSideStreaming(t *testing.T) {
	_, k, _ := initPlugin(t, true)
	h := k.routes["/api/mcp"]

	// Create a live session first.
	w := postJSON(t, h, mcpReq("initialize", map[string]any{}))
	sessionID := w.Header().Get(wire.HeaderSessionID())
	require.NotEmpty(t, sessionID)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/mcp", nil).WithContext(ctx)
	req.Header.Set(wire.HeaderSessionID(), sessionID)

	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "text/event-stream", rw.Header().Get("Content-Type"))
}

func TestStreamableHTTP_EchoesMCPProtocolVersion_Header(t *testing.T) {
	_, k, _ := initPlugin(t, true)
	h := k.routes["/api/mcp"]

	req := httptest.NewRequest(http.MethodPost, "/api/mcp",
		bytes.NewBufferString(`{"jsonrpc":"2.0","id":"1","method":"initialize","params":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(wire.ProtocolVersionHeader(), "2025-11-25")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)

	assert.Equal(t, "2025-11-25", rw.Header().Get(wire.ProtocolVersionHeader()))
}

func TestSession_Init_PersistedToDB_InitializedAtSet(t *testing.T) {
	_, k, ss := initPlugin(t, true)
	h := k.routes["/api/mcp"]

	w := postJSON(t, h, mcpReq("initialize", map[string]any{}))
	sessionID := w.Header().Get(wire.HeaderSessionID())
	require.NotEmpty(t, sessionID)

	sess := ss.sessions[sessionID]
	require.NotNil(t, sess, "session must be written to store")
	assert.NotNil(t, sess.InitializedAt, "initialized_at must be set")
}

func TestStatusEndpoint_ReturnsSessionStats(t *testing.T) {
	_, k, _ := initPlugin(t, true)

	// Create a session to have at least one active.
	h := k.routes["/api/mcp"]
	postJSON(t, h, mcpReq("initialize", map[string]any{}))

	statusH, ok := k.routes["/api/mcp/status"]
	require.True(t, ok, "/api/mcp/status route must be registered")

	req := httptest.NewRequest(http.MethodGet, "/api/mcp/status", nil)
	rw := httptest.NewRecorder()
	statusH.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["enabled"])
	assert.GreaterOrEqual(t, resp["active_sessions"].(float64), float64(1))
}

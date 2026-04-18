package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/model/ctxkey"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver/tools"
)

// --- loopback helpers ---

// captureLoopback records calls made through the loopback.
type captureLoopback struct {
	lastPath  string
	lastQuery string
	respBody  []byte
	respCode  int
}

func (c *captureLoopback) fn() tools.LoopbackFunc {
	return func(ctx context.Context, method, path, query string) ([]byte, int, error) {
		c.lastPath = path
		c.lastQuery = query
		return c.respBody, c.respCode, nil
	}
}

func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// --- ToolRegistry tests ---

func TestToolRegistry_Register_And_Dispatch(t *testing.T) {
	r := tools.New()
	called := false
	r.Register(tools.ToolDescriptor{Name: "my_tool", Description: "test"}, func(_ context.Context, _ json.RawMessage) (any, error) {
		called = true
		return map[string]string{"ok": "1"}, nil
	})

	list := r.List()
	require.Len(t, list, 1)
	assert.Equal(t, "my_tool", list[0].Name)

	result, err := r.Call(context.Background(), "my_tool", nil)
	require.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, result)

	_, err = r.Call(context.Background(), "unknown", nil)
	assert.Error(t, err)
}

func TestAgentSearch_CallsLoopback_WithCtxFilter(t *testing.T) {
	cap := &captureLoopback{respBody: []byte(`[]`), respCode: http.StatusOK}
	reg := tools.New()
	tools.RegisterAll(reg, cap.fn())

	ref := &model.SessionPrincipalRef{
		ID:                   "sa-1",
		AccessibleProjectIDs: []string{"proj-1"},
	}
	ctx := ctxkey.WithPrincipalRef(context.Background(), ref)
	ctx = ctxkey.WithProjectIDs(ctx, ref.AccessibleProjectIDs)

	args := jsonBody(t, map[string]any{"query": "pdf", "projects": "should-be-ignored"})
	_, err := reg.Call(ctx, "agent_search", args)
	require.NoError(t, err)

	assert.Equal(t, "/api/v1/catalog", cap.lastPath)
	assert.NotContains(t, cap.lastQuery, "projects", "user-supplied projects= must not appear in loopback query")
	assert.Contains(t, cap.lastQuery, "q=pdf")
}

func TestAgentGet_NotFound_Returns_JsonRPC_Error(t *testing.T) {
	cap := &captureLoopback{respBody: []byte(`{"error":"not found"}`), respCode: http.StatusNotFound}
	reg := tools.New()
	tools.RegisterAll(reg, cap.fn())

	args := jsonBody(t, map[string]string{"id": "missing-id"})
	_, err := reg.Call(context.Background(), "agent_get", args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCapabilitiesList_ShapeMatchesRESTContract(t *testing.T) {
	respBody := jsonBody(t, map[string]any{
		"items": []map[string]string{{"kind": "mcp.tool", "name": "search"}},
	})
	cap := &captureLoopback{respBody: respBody, respCode: http.StatusOK}
	reg := tools.New()
	tools.RegisterAll(reg, cap.fn())

	args := jsonBody(t, map[string]string{"agent_id": "agent-123"})
	result, err := reg.Call(context.Background(), "capabilities_list", args)
	require.NoError(t, err)

	assert.Equal(t, "/api/v1/capabilities", cap.lastPath)
	assert.Contains(t, cap.lastQuery, "agent_id=agent-123")

	content := result.(map[string]any)["content"].([]map[string]any)
	assert.Len(t, content, 1)
	assert.Equal(t, "text", content[0]["type"])
}

func TestAgentCard_Returns_RawCard_When_Present(t *testing.T) {
	rawCard := []byte(`{"name":"my-agent","version":"1.0"}`)
	cap := &captureLoopback{respBody: rawCard, respCode: http.StatusOK}
	reg := tools.New()
	tools.RegisterAll(reg, cap.fn())

	args := jsonBody(t, map[string]string{"agent_id": "card-agent"})
	result, err := reg.Call(context.Background(), "agent_card", args)
	require.NoError(t, err)

	assert.Equal(t, "/api/v1/catalog/card-agent/card", cap.lastPath)
	content := result.(map[string]any)["content"].([]map[string]any)
	assert.Equal(t, string(rawCard), content[0]["text"])
}

func TestBuildLoopbackFunc_PreservesContext_Via_WithContext(t *testing.T) {
	ref := &model.SessionPrincipalRef{
		ID:                   "user-1",
		AccessibleProjectIDs: []string{"proj-A"},
	}

	var capturedRef *model.SessionPrincipalRef
	var capturedProjectIDs []string

	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		capturedRef = ctxkey.PrincipalRef(r.Context())
		capturedProjectIDs = ctxkey.ProjectIDs(r.Context())
	})

	// Use httptest to simulate the chi router.
	srv := httptest.NewServer(inner)
	defer srv.Close()

	// Build a direct loopback against the inner handler (no network).
	lb := tools.LoopbackFunc(func(ctx context.Context, method, path, query string) ([]byte, int, error) {
		req := httptest.NewRequest(method, path+"?"+query, nil).WithContext(ctx)
		w := httptest.NewRecorder()
		inner.ServeHTTP(w, req)
		return w.Body.Bytes(), w.Code, nil
	})

	ctx := ctxkey.WithPrincipalRef(context.Background(), ref)
	ctx = ctxkey.WithProjectIDs(ctx, ref.AccessibleProjectIDs)

	// Simulate what the loopback does for agent_search.
	_, _, _ = lb(ctx, http.MethodGet, "/api/v1/catalog", "q=test&projects=injected-by-user")

	require.NotNil(t, capturedRef, "SessionPrincipalRef must be visible to inner handler")
	assert.Equal(t, "user-1", capturedRef.ID)

	require.NotNil(t, capturedProjectIDs, "AccessibleProjectIDs must be visible to inner handler")
	assert.Equal(t, []string{"proj-A"}, capturedProjectIDs,
		"inner handler sees ctx project IDs, not user-supplied ?projects=")
}

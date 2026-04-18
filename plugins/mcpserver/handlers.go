package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/model/ctxkey"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver/tools"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver/wire"
)

// ToolRegistry handles tool dispatch. Implemented by *tools.Registry.
type ToolRegistry interface {
	Call(ctx context.Context, tool string, params json.RawMessage) (any, error)
	List() []tools.ToolDescriptor
}

// dispatcher is the JSON-RPC handler used by the Streamable HTTP transport.
type dispatcher struct {
	sessions *sessionManager
	registry ToolRegistry
	cfg      pluginConfig
	worker   *asyncWorker
}

func (d *dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, errResponse(nil, CodeParseError, "parse error"))
		return
	}
	if req.JSONRPC != "2.0" {
		writeJSON(w, errResponse(req.ID, CodeInvalidRequest, "invalid JSON-RPC version"))
		return
	}

	switch req.Method {
	case "initialize":
		d.handleInitialize(w, r, req)
	case "ping":
		writeJSON(w, okResponse(req.ID, map[string]string{}))
	case "tools/list":
		d.handleToolsList(w, req)
	case "tools/call":
		d.handleToolsCall(w, r.Context(), req)
	default:
		writeJSON(w, errResponse(req.ID, CodeMethodNotFound, "method not found: "+req.Method))
	}
}

func (d *dispatcher) handleInitialize(w http.ResponseWriter, r *http.Request, req rpcRequest) {
	ref := ctxkey.PrincipalRef(r.Context())
	principalID, principalType := "<anonymous>", model.PrincipalTypeUserLocal
	if ref != nil {
		principalID = ref.ID
		principalType = ref.Kind
	}

	sess := &model.McpSession{
		ID:              uuid.New().String(),
		PrincipalID:     principalID,
		PrincipalType:   principalType,
		ProtocolVersion: wire.CurrentProtocolVersion,
		LastSeenAt:      time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(d.cfg.sessionTTL),
	}

	if err := d.sessions.Create(r.Context(), sess); err != nil {
		slog.ErrorContext(r.Context(), "mcp: failed to create session", "err", err)
		writeJSON(w, errResponse(req.ID, CodeInternalError, "session creation failed"))
		return
	}

	if fn := wire.NewSessionFn(r.Context()); fn != nil {
		fn(sess.ID)
	}

	if err := d.sessions.MarkInitialized(r.Context(), sess.ID); err != nil {
		slog.WarnContext(r.Context(), "mcp: failed to mark session initialized", "err", err)
	}

	writeJSON(w, okResponse(req.ID, map[string]any{
		"protocolVersion": wire.CurrentProtocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]string{"name": "agentlens", "version": "1.0.0"},
	}))
}

func (d *dispatcher) handleToolsList(w http.ResponseWriter, req rpcRequest) {
	var list []tools.ToolDescriptor
	if d.registry != nil {
		list = d.registry.List()
	}
	if list == nil {
		list = []tools.ToolDescriptor{}
	}
	writeJSON(w, okResponse(req.ID, map[string]any{"tools": list}))
}

func (d *dispatcher) handleToolsCall(w http.ResponseWriter, ctx context.Context, req rpcRequest) {
	if d.registry == nil {
		writeJSON(w, errResponse(req.ID, CodeMethodNotFound, "no tools registered"))
		return
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		writeJSON(w, errResponse(req.ID, CodeInvalidParams, "invalid params"))
		return
	}
	result, err := d.registry.Call(ctx, p.Name, p.Arguments)
	if err != nil {
		writeJSON(w, errResponse(req.ID, CodeInternalError, err.Error()))
		return
	}
	writeJSON(w, okResponse(req.ID, result))
}

func writeJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("mcp: failed to write JSON response", "err", err)
	}
}

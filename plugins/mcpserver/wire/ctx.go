package wire

import "context"

type ctxKey string

const newSessionKey ctxKey = "mcp.wire.newSessionFn"

// withNewSessionFn stores a callback in ctx that the dispatcher calls after
// creating a session, so the transport can set the MCP-Session-Id header.
func withNewSessionFn(ctx context.Context, fn func(sessionID string)) context.Context {
	return context.WithValue(ctx, newSessionKey, fn)
}

// NewSessionFn extracts the session-created callback from ctx, if present.
func NewSessionFn(ctx context.Context) func(string) {
	fn, _ := ctx.Value(newSessionKey).(func(string))
	return fn
}

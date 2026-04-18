// Package audit provides structured audit logging for MCP tool invocations.
// All log entries use slog.InfoContext and never include secret material.
package audit

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// Entry represents a single auditable MCP event.
type Entry struct {
	PrincipalID   string
	PrincipalKind string
	AuthMethod    string
	Tool          string
	ProjectIDs    []string
	Outcome       string // "success" | "error" | "forbidden"
	ErrorMessage  string // generic; never contains secret material
}

// Logger emits scrubbed audit log entries via slog.
type Logger struct {
	enabled bool
}

// New creates an audit logger. When enabled is false, Log is a no-op.
func New(enabled bool) *Logger {
	return &Logger{enabled: enabled}
}

// Log emits an audit entry. Secret-bearing fields (raw keys, tokens, hashes)
// must never appear in Entry — enforce this at the call site.
func (l *Logger) Log(ctx context.Context, e Entry) {
	if !l.enabled {
		return
	}
	slog.InfoContext(ctx, "mcp.audit",
		"principal_id", scrub(e.PrincipalID),
		"principal_kind", e.PrincipalKind,
		"auth_method", e.AuthMethod,
		"tool", e.Tool,
		"project_ids", strings.Join(e.ProjectIDs, ","),
		"outcome", e.Outcome,
		"error", e.ErrorMessage,
		"ts", time.Now().UTC().Format(time.RFC3339),
	)
}

// scrub replaces empty strings with "<unset>" for audit clarity.
func scrub(s string) string {
	if s == "" {
		return "<unset>"
	}
	return s
}

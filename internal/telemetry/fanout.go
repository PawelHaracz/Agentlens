package telemetry

import (
	"context"
	"log/slog"
)

// fanoutHandler writes log records to both a stdout handler and an OTel bridge handler.
// All records go to stdout at the configured log level.
// Only records at or above exportLevel go to the bridge (for OTLP export).
// This implements ADR-010: stdout is never removed, kubectl logs always works.
type fanoutHandler struct {
	stdout      slog.Handler
	bridge      slog.Handler
	exportLevel slog.Level
}

// NewFanoutHandler creates a slog.Handler that writes to both stdout and an OTel bridge.
// stdout receives all records (subject to its own level configuration).
// bridge receives only records at or above exportLevel.
func NewFanoutHandler(stdout, bridge slog.Handler, exportLevel slog.Level) slog.Handler {
	return &fanoutHandler{
		stdout:      stdout,
		bridge:      bridge,
		exportLevel: exportLevel,
	}
}

// Enabled reports whether the handler handles records at the given level.
// A record is handled if either stdout or bridge would handle it.
func (h *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.stdout.Enabled(ctx, level) ||
		(level >= h.exportLevel && h.bridge.Enabled(ctx, level))
}

// Handle dispatches the record to stdout (always) and bridge (if level >= exportLevel).
// Bridge errors are best-effort and discarded — a collector outage must not break logging.
func (h *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.stdout.Handle(ctx, r); err != nil {
		return err
	}
	if r.Level >= h.exportLevel {
		_ = h.bridge.Handle(ctx, r)
	}
	return nil
}

// WithAttrs returns a new handler with the given attributes pre-set on both outputs.
func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &fanoutHandler{
		stdout:      h.stdout.WithAttrs(attrs),
		bridge:      h.bridge.WithAttrs(attrs),
		exportLevel: h.exportLevel,
	}
}

// WithGroup returns a new handler with the given group name applied to both outputs.
func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	return &fanoutHandler{
		stdout:      h.stdout.WithGroup(name),
		bridge:      h.bridge.WithGroup(name),
		exportLevel: h.exportLevel,
	}
}

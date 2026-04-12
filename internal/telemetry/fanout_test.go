package telemetry_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFanoutHandler_BothOutputs(t *testing.T) {
	var stdoutBuf, bridgeBuf bytes.Buffer
	stdout := slog.NewJSONHandler(&stdoutBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelInfo)
	logger := slog.New(h)
	logger.Info("test message")

	assert.Contains(t, stdoutBuf.String(), "test message", "stdout should receive the log")
	assert.Contains(t, bridgeBuf.String(), "test message", "bridge should receive the log at info level")
}

func TestFanoutHandler_ExportLevelFilters(t *testing.T) {
	var stdoutBuf, bridgeBuf bytes.Buffer
	stdout := slog.NewJSONHandler(&stdoutBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelWarn)
	logger := slog.New(h)
	logger.Info("info message")
	logger.Warn("warn message")

	assert.Contains(t, stdoutBuf.String(), "info message", "stdout should receive info")
	assert.Contains(t, stdoutBuf.String(), "warn message", "stdout should receive warn")
	assert.NotContains(t, bridgeBuf.String(), "info message", "bridge should NOT receive info (below exportLevel)")
	assert.Contains(t, bridgeBuf.String(), "warn message", "bridge should receive warn (at exportLevel)")
}

func TestFanoutHandler_WithAttrs(t *testing.T) {
	var stdoutBuf, bridgeBuf bytes.Buffer
	stdout := slog.NewJSONHandler(&stdoutBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelInfo)
	h2 := h.WithAttrs([]slog.Attr{slog.String("component", "test")})
	logger := slog.New(h2)
	logger.Info("with attrs")

	assert.Contains(t, stdoutBuf.String(), "component")
	assert.Contains(t, bridgeBuf.String(), "component")
}

func TestFanoutHandler_WithGroup(t *testing.T) {
	var stdoutBuf, bridgeBuf bytes.Buffer
	stdout := slog.NewJSONHandler(&stdoutBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelDebug})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelInfo)
	h2 := h.WithGroup("mygroup")
	logger := slog.New(h2)
	logger.Info("grouped", slog.String("key", "val"))

	assert.Contains(t, stdoutBuf.String(), "mygroup")
	assert.Contains(t, bridgeBuf.String(), "mygroup")
}

func TestFanoutHandler_Enabled(t *testing.T) {
	var stdoutBuf, bridgeBuf bytes.Buffer
	stdout := slog.NewJSONHandler(&stdoutBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	bridge := slog.NewJSONHandler(&bridgeBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	h := telemetry.NewFanoutHandler(stdout, bridge, slog.LevelWarn)

	// Debug is disabled at stdout level
	assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))
	// Info is enabled (stdout level is info)
	assert.True(t, h.Enabled(context.Background(), slog.LevelInfo))
	// Warn is enabled
	assert.True(t, h.Enabled(context.Background(), slog.LevelWarn))
}

// suppress unused import warning — require is used indirectly via testify patterns
var _ = require.NoError

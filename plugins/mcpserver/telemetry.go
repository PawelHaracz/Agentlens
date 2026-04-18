package mcpserver

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// mcpMetrics holds all OTel metric instruments for the MCP plugin.
type mcpMetrics struct {
	invocations metric.Int64Counter
	toolCalls   metric.Int64Counter
	credHits    metric.Int64Counter
	credMisses  metric.Int64Counter
}

// newMCPMetrics registers the agentlens_mcp_* metric set.
// Errors are non-fatal — instruments may be nil when telemetry is disabled.
func newMCPMetrics() *mcpMetrics {
	meter := otel.Meter("agentlens.mcp")

	invocations, _ := meter.Int64Counter("agentlens_mcp_invocations_total",
		metric.WithDescription("Total MCP JSON-RPC requests received"))

	toolCalls, _ := meter.Int64Counter("agentlens_mcp_tool_calls_total",
		metric.WithDescription("Total MCP tool/call dispatches by tool name"))

	credHits, _ := meter.Int64Counter("agentlens_mcp_credcache_hits_total",
		metric.WithDescription("API-key bcrypt cache hits"))

	credMisses, _ := meter.Int64Counter("agentlens_mcp_credcache_misses_total",
		metric.WithDescription("API-key bcrypt cache misses (full bcrypt compare required)"))

	return &mcpMetrics{
		invocations: invocations,
		toolCalls:   toolCalls,
		credHits:    credHits,
		credMisses:  credMisses,
	}
}

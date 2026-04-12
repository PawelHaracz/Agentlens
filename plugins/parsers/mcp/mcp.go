// Package mcp provides the MCP protocol parser plugin.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/telemetry"
)

type mcpCard struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Version     string        `json:"version"`
	Remotes     []mcpRemote   `json:"remotes,omitempty"`
	Tools       []mcpTool     `json:"tools,omitempty"`
	Resources   []mcpResource `json:"resources,omitempty"`
	Prompts     []mcpPrompt   `json:"prompts,omitempty"`
}

type mcpRemote struct {
	URL string `json:"url"`
}

type mcpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

type mcpResource struct {
	Name        string `json:"name"`
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
}

type mcpPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Arguments   []any  `json:"arguments,omitempty"`
}

// Plugin implements the MCP parser plugin.
type Plugin struct {
	initialized bool
	metrics     *telemetry.ParserMetrics
}

// New creates a new MCP parser plugin.
func New() *Plugin {
	return &Plugin{
		metrics: telemetry.NewParserMetrics(),
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "mcp-parser" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeParser }

// Protocol returns the protocol this parser handles.
func (p *Plugin) Protocol() model.Protocol { return model.ProtocolMCP }

// CardPath returns the default card path for MCP.
func (p *Plugin) CardPath() string { return "/.well-known/mcp/server.json" }

// Init initializes the plugin.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.initialized = true
	p.metrics = telemetry.NewParserMetrics()
	return nil
}

// Start starts the plugin (no-op for parser).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for parser).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Parse parses an MCP server card JSON blob into an AgentType.
func (p *Plugin) Parse(raw []byte) (*model.AgentType, error) {
	start := time.Now()

	tracer := otel.Tracer("agentlens.parser")
	ctx, span := tracer.Start(context.Background(), "parser.mcp.parse",
		trace.WithAttributes(
			attribute.String("agentlens.parser.type", "mcp"),
			attribute.Int64("agentlens.parser.input_size", int64(len(raw))),
		),
	)
	defer span.End()

	var card mcpCard
	if err := json.Unmarshal(raw, &card); err != nil {
		err = fmt.Errorf("parsing mcp card: %w", err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.metrics.RecordInvocation(ctx, "mcp", "error", "", time.Since(start).Seconds()*1000)
		return nil, err
	}
	if card.Name == "" {
		err := fmt.Errorf("mcp card missing required field: name")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.metrics.RecordInvocation(ctx, "mcp", "error", "", time.Since(start).Seconds()*1000)
		return nil, err
	}
	if len(card.Remotes) == 0 || card.Remotes[0].URL == "" {
		err := fmt.Errorf("mcp card missing required field: remotes[0].url")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		p.metrics.RecordInvocation(ctx, "mcp", "error", "", time.Since(start).Seconds()*1000)
		return nil, err
	}

	var caps []model.Capability
	for _, t := range card.Tools {
		caps = append(caps, &model.MCPTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	for _, r := range card.Resources {
		caps = append(caps, &model.MCPResource{
			Name:        r.Name,
			URI:         r.URI,
			Description: r.Description,
		})
	}
	for _, pr := range card.Prompts {
		caps = append(caps, &model.MCPPrompt{
			Name:        pr.Name,
			Description: pr.Description,
			Arguments:   pr.Arguments,
		})
	}

	specVersion := card.Version
	toolCount := len(card.Tools)

	span.SetAttributes(
		attribute.String("agentlens.parser.spec_version", specVersion),
		attribute.Int("agentlens.parser.tool_count", toolCount),
	)
	p.metrics.RecordInvocation(ctx, "mcp", "success", specVersion, time.Since(start).Seconds()*1000)

	return &model.AgentType{
		Protocol:     model.ProtocolMCP,
		Endpoint:     card.Remotes[0].URL,
		Version:      card.Version,
		Capabilities: caps,
		AgentKey:     model.ComputeAgentKey(model.ProtocolMCP, card.Remotes[0].URL),
		CreatedOn:    time.Now().UTC(),
		RawBytes:     raw,
	}, nil
}

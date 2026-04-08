// Package mcp provides the MCP protocol parser plugin.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
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
}

// New creates a new MCP parser plugin.
func New() *Plugin { return &Plugin{} }

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
	return nil
}

// Start starts the plugin (no-op for parser).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for parser).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Parse parses an MCP server card JSON blob into an AgentType.
func (p *Plugin) Parse(raw []byte) (*model.AgentType, error) {
	var card mcpCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("parsing mcp card: %w", err)
	}
	if card.Name == "" {
		return nil, fmt.Errorf("mcp card missing required field: name")
	}
	if len(card.Remotes) == 0 || card.Remotes[0].URL == "" {
		return nil, fmt.Errorf("mcp card missing required field: remotes[0].url")
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

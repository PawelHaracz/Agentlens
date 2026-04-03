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
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Remotes     []mcpRemote `json:"remotes,omitempty"`
	Tools       []mcpTool   `json:"tools,omitempty"`
}

type mcpRemote struct {
	URL string `json:"url"`
}

type mcpTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
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

// Validate validates a raw JSON MCP server card and returns a structured result
// that satisfies the kernel.ParserPlugin interface.
func (p *Plugin) Validate(raw []byte) kernel.ValidationResult {
	var card mcpCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return kernel.ValidationResult{
			Valid:    false,
			Errors:   []kernel.ValidationError{{Field: "json", Message: "invalid JSON: " + err.Error()}},
			Warnings: []string{},
		}
	}

	var errs []kernel.ValidationError
	if card.Name == "" {
		errs = append(errs, kernel.ValidationError{Field: "name", Message: "name is required"})
	}
	if len(card.Remotes) == 0 || card.Remotes[0].URL == "" {
		errs = append(errs, kernel.ValidationError{Field: "remotes", Message: "remotes[0].url is required"})
	}

	if len(errs) > 0 {
		return kernel.ValidationResult{
			Valid:    false,
			Errors:   errs,
			Warnings: []string{},
		}
	}

	return kernel.ValidationResult{
		Valid:    true,
		Errors:   []kernel.ValidationError{},
		Warnings: []string{},
		Preview: map[string]any{
			"display_name": card.Name,
			"description":  card.Description,
			"protocol":     "mcp",
			"tools_count":  len(card.Tools),
		},
	}
}

// Parse parses an MCP server card JSON blob into a CatalogEntry.
func (p *Plugin) Parse(raw []byte, source model.SourceType) (*model.CatalogEntry, error) {
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

	endpoint := card.Remotes[0].URL

	skills := make([]model.Skill, 0, len(card.Tools))
	for _, t := range card.Tools {
		skills = append(skills, model.Skill{
			Name:        t.Name,
			Description: t.Description,
		})
	}

	now := time.Now().UTC()
	return &model.CatalogEntry{
		DisplayName: card.Name,
		Description: card.Description,
		Protocol:    model.ProtocolMCP,
		Endpoint:    endpoint,
		Version:     card.Version,
		Status:      model.StatusUnknown,
		Source:      source,
		Skills:      skills,
		Validity:    model.Validity{LastSeen: now},
		RawCard:     json.RawMessage(raw),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

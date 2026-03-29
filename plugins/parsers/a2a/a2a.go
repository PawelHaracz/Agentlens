// Package a2a provides the A2A protocol parser plugin.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// a2aCard is the JSON structure of an A2A agent card.
type a2aCard struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	URL         string       `json:"url"`
	Version     string       `json:"version"`
	Provider    *a2aProvider `json:"provider,omitempty"`
	Skills      []a2aSkill   `json:"skills,omitempty"`
}

type a2aProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

type a2aSkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// Plugin implements the A2A parser plugin.
type Plugin struct {
	initialized bool
}

// New creates a new A2A parser plugin.
func New() *Plugin { return &Plugin{} }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "a2a-parser" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeParser }

// Protocol returns the protocol this parser handles.
func (p *Plugin) Protocol() model.Protocol { return model.ProtocolA2A }

// CardPath returns the default card path for A2A.
func (p *Plugin) CardPath() string { return "/.well-known/agent-card.json" }

// Init initializes the plugin.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.initialized = true
	return nil
}

// Start starts the plugin (no-op for parser).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op for parser).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Parse parses an A2A agent card JSON blob into a CatalogEntry.
func (p *Plugin) Parse(raw []byte, source model.SourceType) (*model.CatalogEntry, error) {
	var card a2aCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("parsing a2a card: %w", err)
	}
	if card.Name == "" {
		return nil, fmt.Errorf("a2a card missing required field: name")
	}
	if card.URL == "" {
		return nil, fmt.Errorf("a2a card missing required field: url")
	}

	skills := make([]model.Skill, 0, len(card.Skills))
	for _, s := range card.Skills {
		skills = append(skills, model.Skill{
			Name:        s.Name,
			Description: s.Description,
			InputModes:  s.InputModes,
			OutputModes: s.OutputModes,
		})
	}

	var provider model.Provider
	if card.Provider != nil {
		provider.Organization = card.Provider.Organization
		provider.Team = card.Provider.Organization
		provider.URL = card.Provider.URL
	}

	now := time.Now().UTC()
	return &model.CatalogEntry{
		DisplayName: card.Name,
		Description: card.Description,
		Protocol:    model.ProtocolA2A,
		Endpoint:    card.URL,
		Version:     card.Version,
		Status:      model.StatusUnknown,
		Source:      source,
		Provider:    provider,
		Skills:      skills,
		Validity:    model.Validity{LastSeen: now},
		RawCard:     json.RawMessage(raw),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

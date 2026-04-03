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

type a2aProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
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
	var card fullCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("parsing a2a card: %w", err)
	}
	if card.Name == "" {
		return nil, fmt.Errorf("a2a card missing required field: name")
	}

	endpoint, err := resolveEndpoint(&card)
	if err != nil {
		return nil, err
	}

	skills := buildSkills(&card)
	provider := buildProvider(&card)
	specVersion := detectSpecVersion(&card)
	typedMeta := buildTypedMeta(&card)

	now := time.Now().UTC()
	return &model.CatalogEntry{
		DisplayName: card.Name,
		Description: card.Description,
		Protocol:    model.ProtocolA2A,
		Endpoint:    endpoint,
		Version:     card.Version,
		Status:      model.StatusUnknown,
		Source:      source,
		Provider:    provider,
		Skills:      skills,
		SpecVersion: specVersion,
		TypedMeta:   typedMeta,
		Validity:    model.Validity{LastSeen: now},
		RawCard:     json.RawMessage(raw),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// resolveEndpoint determines the agent endpoint from the card, preferring
// the first supportedInterfaces URL over the top-level url field.
func resolveEndpoint(card *fullCard) (string, error) {
	endpoint := card.URL
	if len(card.SupportedInterfaces) > 0 && card.SupportedInterfaces[0].URL != "" {
		endpoint = card.SupportedInterfaces[0].URL
	}
	if endpoint == "" {
		return "", fmt.Errorf("a2a card missing required field: url or supportedInterfaces")
	}
	return endpoint, nil
}

// buildSkills converts raw card skills into model.Skill values.
func buildSkills(card *fullCard) []model.Skill {
	skills := make([]model.Skill, 0, len(card.Skills))
	for _, s := range card.Skills {
		skills = append(skills, model.Skill{
			Name:        s.Name,
			Description: s.Description,
			Tags:        s.Tags,
			InputModes:  s.InputModes,
			OutputModes: s.OutputModes,
		})
	}
	return skills
}

// buildProvider constructs a model.Provider from the card's provider field.
func buildProvider(card *fullCard) model.Provider {
	var provider model.Provider
	if card.Provider != nil {
		provider.Organization = card.Provider.Organization
		provider.Team = card.Provider.Organization
		provider.URL = card.Provider.URL
	}
	return provider
}

// buildTypedMeta assembles typed metadata from extensions, security schemes,
// interfaces, and signatures declared in the card.
func buildTypedMeta(card *fullCard) []model.TypedMetadata {
	var typedMeta []model.TypedMetadata

	if card.Capabilities != nil {
		for _, ext := range card.Capabilities.Extensions {
			typedMeta = append(typedMeta, &model.A2AExtension{
				URI:      ext.URI,
				Required: ext.Required,
			})
		}
	}

	for _, sec := range card.SecuritySchemes {
		typedMeta = append(typedMeta, &model.A2ASecurityScheme{
			Type:   sec.Type,
			Method: sec.Method,
			Name:   sec.Name,
		})
	}

	for _, iface := range card.SupportedInterfaces {
		typedMeta = append(typedMeta, &model.A2AInterface{
			URL:     iface.URL,
			Binding: iface.Binding,
		})
	}

	for _, sig := range card.Signatures {
		typedMeta = append(typedMeta, &model.A2ASignature{
			Algorithm: sig.Algorithm,
			KeyID:     sig.KeyID,
		})
	}

	return typedMeta
}

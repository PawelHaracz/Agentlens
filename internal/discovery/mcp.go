package discovery

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// mcpCard is the JSON structure of an MCP server card.
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

// ParseMCPCard parses an MCP server card JSON blob into a CatalogEntry.
func ParseMCPCard(raw []byte, source model.SourceType) (*model.CatalogEntry, error) {
	var card mcpCard
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("parsing mcp card: %w", err)
	}
	if card.Name == "" {
		return nil, fmt.Errorf("mcp card missing required field: name")
	}

	var endpoint string
	if len(card.Remotes) > 0 {
		endpoint = card.Remotes[0].URL
	}

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
		RawCard:     json.RawMessage(raw),
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

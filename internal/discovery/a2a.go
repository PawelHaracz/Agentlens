package discovery

import (
	"encoding/json"
	"fmt"
	"time"

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
}

type a2aSkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// ParseA2ACard parses an A2A agent card JSON blob into an Agent.
func ParseA2ACard(raw []byte, source model.SourceType) (*model.Agent, error) {
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

	var team string
	if card.Provider != nil {
		team = card.Provider.Organization
	}

	now := time.Now().UTC()
	return &model.Agent{
		Name:        card.Name,
		Description: card.Description,
		Protocol:    model.ProtocolA2A,
		Endpoint:    card.URL,
		Version:     card.Version,
		Status:      model.StatusUnknown,
		Source:      source,
		Team:        team,
		Skills:      skills,
		RawCard:     json.RawMessage(raw),
		LastSeen:    now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

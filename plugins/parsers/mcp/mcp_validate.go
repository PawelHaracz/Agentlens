package mcp

import (
	"encoding/json"

	"github.com/PawelHaracz/agentlens/internal/kernel"
)

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

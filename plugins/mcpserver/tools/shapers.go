package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// agentSearchInput is the JSON-schema validated input for agent_search.
type agentSearchInput struct {
	Query    string `json:"query"`
	Protocol string `json:"protocol,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// buildSearchQuery builds the URL query string for /api/v1/catalog.
// User-supplied project filter is intentionally EXCLUDED — project scope
// is controlled exclusively via ctx AccessibleProjectIDs (M4 resolution).
func buildSearchQuery(input agentSearchInput) string {
	params := url.Values{}
	if input.Query != "" {
		params.Set("q", input.Query)
	}
	if input.Protocol != "" {
		params.Set("protocol", input.Protocol)
	}
	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	params.Set("limit", fmt.Sprintf("%d", limit))
	return params.Encode()
}

// parseAgentSearch decodes agent_search arguments.
func parseAgentSearch(raw json.RawMessage) (agentSearchInput, error) {
	var in agentSearchInput
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return in, fmt.Errorf("invalid agent_search args: %w", err)
		}
	}
	return in, nil
}

// parseID decodes a single {id} or {agent_id} argument.
func parseID(raw json.RawMessage, field string) (string, error) {
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	v := m[field]
	if v == "" {
		return "", fmt.Errorf("missing required field %q", field)
	}
	return v, nil
}

// wrapContent wraps a raw REST body in MCP content format.
func wrapContent(body []byte) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(body)},
		},
	}
}

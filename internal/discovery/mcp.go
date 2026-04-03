package discovery

import (
	"github.com/PawelHaracz/agentlens/internal/model"
	mcpplugin "github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
)

// ParseMCPCard parses an MCP server card JSON blob into an AgentType.
func ParseMCPCard(raw []byte) (*model.AgentType, error) {
	p := mcpplugin.New()
	return p.Parse(raw)
}

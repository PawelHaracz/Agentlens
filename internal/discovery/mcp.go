package discovery

import (
	"github.com/PawelHaracz/agentlens/internal/model"
	mcpplugin "github.com/PawelHaracz/agentlens/plugins/parsers/mcp"
)

// ParseMCPCard parses an MCP server card JSON blob into a CatalogEntry.
func ParseMCPCard(raw []byte, source model.SourceType) (*model.CatalogEntry, error) {
	p := mcpplugin.New()
	return p.Parse(raw, source)
}

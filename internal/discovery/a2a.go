package discovery

import (
	"github.com/PawelHaracz/agentlens/internal/model"
	a2aplugin "github.com/PawelHaracz/agentlens/plugins/parsers/a2a"
)

// ParseA2ACard parses an A2A agent card JSON blob into a CatalogEntry.
func ParseA2ACard(raw []byte, source model.SourceType) (*model.CatalogEntry, error) {
	p := a2aplugin.New()
	return p.Parse(raw, source)
}

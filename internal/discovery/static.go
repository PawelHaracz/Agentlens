package discovery

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// StaticSource discovers agents from statically-configured URLs.
type StaticSource struct {
	sources []config.SourceConfig
	crawler *Crawler
	log     *slog.Logger
}

// NewStaticSource creates a StaticSource from configuration.
func NewStaticSource(sources []config.SourceConfig) *StaticSource {
	return &StaticSource{
		sources: sources,
		crawler: NewCrawler(),
		log:     slog.With("component", "static-source"),
	}
}

// Name returns the source identifier.
func (s *StaticSource) Name() string { return "static" }

// Discover fetches and parses all configured agent cards.
func (s *StaticSource) Discover(ctx context.Context) ([]*model.CatalogEntry, error) {
	var entries []*model.CatalogEntry
	for _, src := range s.sources {
		entry, err := s.fetchOne(ctx, src)
		if err != nil {
			s.log.Warn("failed to discover entry", "name", src.Name, "url", src.URL, "err", err)
			continue
		}
		entry.DisplayName = src.Name
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *StaticSource) fetchOne(ctx context.Context, src config.SourceConfig) (*model.CatalogEntry, error) {
	raw, err := s.crawler.FetchCard(ctx, src.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching card: %w", err)
	}

	switch src.Type {
	case "a2a":
		return ParseA2ACard(raw, model.SourceConfig)
	case "mcp":
		return ParseMCPCard(raw, model.SourceConfig)
	default:
		return ParseA2ACard(raw, model.SourceConfig)
	}
}

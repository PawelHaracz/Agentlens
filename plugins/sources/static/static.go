// Package static provides the static configuration discovery source plugin.
package static

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// Plugin implements the static source plugin.
type Plugin struct {
	sources []config.SourceConfig
	crawler *discovery.Crawler
	kern    kernel.Kernel
	log     *slog.Logger
}

// New creates a new static source plugin.
func New(sources []config.SourceConfig) *Plugin {
	return &Plugin{sources: sources}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "static-source" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeSource }

// Init initializes the plugin.
func (p *Plugin) Init(k kernel.Kernel) error {
	p.kern = k
	p.crawler = discovery.NewCrawler()
	p.log = k.Logger().With("component", "static-source")
	return nil
}

// Start starts the plugin (no-op, discovery manager drives it).
func (p *Plugin) Start(ctx context.Context) error { return nil }

// Stop stops the plugin (no-op).
func (p *Plugin) Stop(ctx context.Context) error { return nil }

// Discover fetches and parses all configured agent cards.
func (p *Plugin) Discover(ctx context.Context) ([]*model.AgentType, error) {
	var agentTypes []*model.AgentType
	for _, src := range p.sources {
		at, err := p.fetchOne(ctx, src)
		if err != nil {
			p.log.Warn("failed to discover entry", "name", src.Name, "url", src.URL, "err", err)
			continue
		}
		agentTypes = append(agentTypes, at)
	}
	return agentTypes, nil
}

func (p *Plugin) fetchOne(ctx context.Context, src config.SourceConfig) (*model.AgentType, error) {
	raw, err := p.crawler.FetchCard(ctx, src.URL)
	if err != nil {
		return nil, fmt.Errorf("fetching card: %w", err)
	}

	protocol := model.Protocol(src.Type)
	parser, ok := p.kern.Parser(protocol)
	if !ok {
		return nil, fmt.Errorf("no parser for protocol %s", src.Type)
	}
	at, err := parser.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing card: %w", err)
	}
	at.Endpoint = src.URL
	return at, nil
}

// Package kernel provides the microkernel plugin architecture for AgentLens.
package kernel

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/PawelHaracz/agentlens/internal/config"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// PluginType categorizes plugins.
type PluginType string

const (
	PluginTypeParser     PluginType = "parser"
	PluginTypeSource     PluginType = "source"
	PluginTypeMiddleware PluginType = "middleware"
	PluginTypeStore      PluginType = "store"
)

// Plugin is the base interface all plugins implement.
type Plugin interface {
	Name() string
	Version() string
	Type() PluginType
	Init(k Kernel) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// ParserPlugin parses protocol-specific cards into CatalogEntry.
type ParserPlugin interface {
	Plugin
	Protocol() model.Protocol
	Parse(raw []byte, source model.SourceType) (*model.CatalogEntry, error)
	CardPath() string
}

// SourcePlugin discovers catalog entries from a specific source.
type SourcePlugin interface {
	Plugin
	Discover(ctx context.Context) ([]*model.CatalogEntry, error)
}

// Kernel is what the core exposes to plugins.
type Kernel interface {
	Store() store.Store
	Config() *config.Config
	Logger() *slog.Logger
	License() LicenseInfo
	Parser(protocol model.Protocol) (ParserPlugin, bool)
	RegisterRoutes(prefix string, handler http.Handler)
	RegisterMiddleware(mw func(http.Handler) http.Handler)
}

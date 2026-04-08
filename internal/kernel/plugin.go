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
	PluginTypeCardStore  PluginType = "cardstore"
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

// ValidationError represents a single field-level validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult is the structured output of ParserPlugin.Validate.
type ValidationResult struct {
	Valid       bool              `json:"valid"`
	SpecVersion string            `json:"spec_version"`
	Errors      []ValidationError `json:"errors"`
	Warnings    []string          `json:"warnings"`
	Preview     map[string]any    `json:"preview,omitempty"`
}

// ParserPlugin parses protocol-specific cards into CatalogEntry.
type ParserPlugin interface {
	Plugin
	Protocol() model.Protocol
	Parse(raw []byte) (*model.AgentType, error)
	Validate(raw []byte) ValidationResult
	CardPath() string
}

// SourcePlugin discovers catalog entries from a specific source.
type SourcePlugin interface {
	Plugin
	Discover(ctx context.Context) ([]*model.AgentType, error)
}

// CardStorePlugin persists verbatim raw card bytes keyed by AgentTypeID.
type CardStorePlugin interface {
	Plugin
	StoreCard(ctx context.Context, agentTypeID string, data []byte, contentType string) error
	GetCard(ctx context.Context, agentTypeID string) (*model.RawCard, error)
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
	CardStore() CardStorePlugin
}

// Package store provides interfaces and implementations for persisting catalog data.
package store

import (
	"context"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// Store defines the interface for catalog entry persistence.
type Store interface {
	// Provider
	UpsertProvider(ctx context.Context, provider *model.Provider) (*model.Provider, error)

	// CatalogEntry (always loaded with AgentType, Provider, Capabilities)
	Create(ctx context.Context, entry *model.CatalogEntry) error
	Get(ctx context.Context, id string) (*model.CatalogEntry, error)
	Update(ctx context.Context, entry *model.CatalogEntry) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ListFilter) ([]model.CatalogEntry, error)
	FindByEndpoint(ctx context.Context, endpoint string) (*model.CatalogEntry, error)
	SearchCapabilities(ctx context.Context, query string) ([]model.CatalogEntry, error)
	Stats(ctx context.Context) (*StoreStats, error)

	// UpdateHealth persists health check results for a single entry.
	// It also updates validity_last_seen when LastSuccessAt is non-nil.
	UpdateHealth(ctx context.Context, entryID string, h model.Health) error

	// ListForProbing returns entries due for a probe: not deprecated, and either
	// never probed or last probed before olderThan. Ordered NULLS FIRST, capped by limit.
	ListForProbing(ctx context.Context, olderThan time.Time, limit int) ([]model.CatalogEntry, error)

	// SetLifecycle sets the lifecycle state of an entry (admin/editor action).
	SetLifecycle(ctx context.Context, entryID string, state model.LifecycleState) error

	Close() error
}

// ListFilter holds filtering parameters for listing catalog entries.
type ListFilter struct {
	Protocol   *model.Protocol
	States     []model.LifecycleState // filter by one or more lifecycle states (IN clause)
	Source     *model.SourceType
	Team       string
	Query      string
	Categories []string
	Limit      int
	Offset     int
	Sort       string // "lastSuccessAt_desc" (default) | "displayName_asc" | "createdAt_desc"
}

// StoreStats holds aggregate statistics about stored catalog entries.
type StoreStats struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
	BySource map[string]int `json:"by_source"`
}

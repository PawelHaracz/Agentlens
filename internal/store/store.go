// Package store provides interfaces and implementations for persisting catalog data.
package store

import (
	"context"

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

	Close() error
}

// ListFilter holds filtering parameters for listing catalog entries.
type ListFilter struct {
	Protocol   *model.Protocol
	Status     *model.LifecycleState
	Source     *model.SourceType
	Team       string
	Query      string
	Categories []string
	Limit      int
	Offset     int
}

// StoreStats holds aggregate statistics about stored catalog entries.
type StoreStats struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
	BySource map[string]int `json:"by_source"`
}

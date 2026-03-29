// Package store provides interfaces and implementations for persisting agent data.
package store

import (
	"context"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// Store defines the interface for agent persistence.
type Store interface {
	Create(ctx context.Context, agent *model.Agent) error
	Get(ctx context.Context, id string) (*model.Agent, error)
	Update(ctx context.Context, agent *model.Agent) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ListFilter) ([]model.Agent, error)
	FindByEndpoint(ctx context.Context, endpoint string) (*model.Agent, error)
	SearchSkills(ctx context.Context, query string) ([]model.Agent, error)
	Stats(ctx context.Context) (*StoreStats, error)
	Close() error
}

// ListFilter holds filtering parameters for listing agents.
type ListFilter struct {
	Protocol  *model.Protocol
	Status    *model.Status
	Source    *model.SourceType
	Team      string
	Query     string
	Tags      []string
	Limit     int
	Offset    int
}

// StoreStats holds aggregate statistics about stored agents.
type StoreStats struct {
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
	BySource map[string]int `json:"by_source"`
}

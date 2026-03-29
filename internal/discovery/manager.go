package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// Source is the interface implemented by all discovery sources.
type Source interface {
	Name() string
	Discover(ctx context.Context) ([]*model.CatalogEntry, error)
}

// Manager orchestrates periodic discovery from all sources.
type Manager struct {
	sources      []Source
	store        store.Store
	pollInterval time.Duration
	log          *slog.Logger
}

// NewManager creates a new Manager.
func NewManager(sources []Source, s store.Store, pollInterval time.Duration) *Manager {
	return &Manager{
		sources:      sources,
		store:        s,
		pollInterval: pollInterval,
		log:          slog.With("component", "discovery-manager"),
	}
}

// Run starts the poll loop and blocks until ctx is cancelled.
func (m *Manager) Run(ctx context.Context) error {
	m.log.Info("starting discovery manager", "interval", m.pollInterval)
	if err := m.runOnce(ctx); err != nil {
		m.log.Warn("initial discovery failed", "err", err)
	}
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := m.runOnce(ctx); err != nil {
				m.log.Warn("discovery cycle failed", "err", err)
			}
		}
	}
}

func (m *Manager) runOnce(ctx context.Context) error {
	for _, src := range m.sources {
		entries, err := src.Discover(ctx)
		if err != nil {
			m.log.Warn("source discovery failed", "source", src.Name(), "err", err)
			continue
		}
		if err := m.upsert(ctx, src.Name(), entries); err != nil {
			m.log.Warn("upsert failed", "source", src.Name(), "err", err)
		}
	}
	return nil
}

func (m *Manager) upsert(ctx context.Context, sourceName string, entries []*model.CatalogEntry) error {
	seen := make(map[string]bool)

	for _, e := range entries {
		existing, err := m.store.FindByEndpoint(ctx, e.Endpoint)
		if err != nil {
			return fmt.Errorf("finding by endpoint: %w", err)
		}
		now := time.Now().UTC()
		if existing != nil {
			// Do not touch push-registered entries
			if existing.Source == model.SourcePush {
				continue
			}
			existing.DisplayName = e.DisplayName
			existing.Description = e.Description
			existing.Protocol = e.Protocol
			existing.Version = e.Version
			existing.Skills = e.Skills
			existing.Categories = e.Categories
			existing.Provider = e.Provider
			existing.Metadata = e.Metadata
			existing.RawCard = e.RawCard
			existing.Validity.LastSeen = now
			existing.UpdatedAt = now
			if err := m.store.Update(ctx, existing); err != nil {
				m.log.Warn("failed to update entry", "id", existing.ID, "err", err)
			}
			seen[existing.ID] = true
		} else {
			e.ID = uuid.NewString()
			e.Validity.LastSeen = now
			e.CreatedAt = now
			e.UpdatedAt = now
			if err := m.store.Create(ctx, e); err != nil {
				m.log.Warn("failed to create entry", "endpoint", e.Endpoint, "err", err)
			} else {
				seen[e.ID] = true
			}
		}
	}

	// Mark missing non-push entries as down
	src := m.sourceType(sourceName)
	existing, err := m.store.List(ctx, store.ListFilter{Source: &src})
	if err != nil {
		return fmt.Errorf("listing entries for source: %w", err)
	}
	for _, e := range existing {
		if !seen[e.ID] {
			e.Status = model.StatusDown
			e.UpdatedAt = time.Now().UTC()
			if err := m.store.Update(ctx, &e); err != nil {
				m.log.Warn("failed to mark entry down", "id", e.ID, "err", err)
			}
		}
	}
	return nil
}

func (m *Manager) sourceType(name string) model.SourceType {
	switch name {
	case "k8s":
		return model.SourceK8s
	case "static":
		return model.SourceConfig
	default:
		return model.SourceUpstream
	}
}

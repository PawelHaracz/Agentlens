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
	Discover(ctx context.Context) ([]*model.AgentType, error)
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
		agentTypes, err := src.Discover(ctx)
		if err != nil {
			m.log.Warn("source discovery failed", "source", src.Name(), "err", err)
			continue
		}
		if err := m.upsert(ctx, src.Name(), agentTypes); err != nil {
			m.log.Warn("upsert failed", "source", src.Name(), "err", err)
		}
	}
	return nil
}

func (m *Manager) upsert(ctx context.Context, sourceName string, agentTypes []*model.AgentType) error {
	source := m.sourceType(sourceName)
	seen := make(map[string]bool)

	for _, at := range agentTypes {
		// Compute the agent key from protocol + endpoint.
		at.AgentKey = model.ComputeAgentKey(at.Protocol, at.Endpoint)

		existing, err := m.store.FindByEndpoint(ctx, at.Endpoint)
		if err != nil {
			return fmt.Errorf("finding by endpoint: %w", err)
		}
		now := time.Now().UTC()
		if existing != nil {
			// Never overwrite push-registered entries.
			if existing.Source == model.SourcePush {
				continue
			}
			// Update AgentType fields on the existing entry.
			if existing.AgentType != nil {
				existing.AgentType.Protocol = at.Protocol
				existing.AgentType.Version = at.Version
				existing.AgentType.SpecVersion = at.SpecVersion
				existing.AgentType.Provider = at.Provider
				existing.AgentType.RawDefinition = at.RawDefinition
				existing.AgentType.Capabilities = at.Capabilities
			}
			// Update CatalogEntry timestamps.
			existing.Validity.LastSeen = now
			existing.UpdatedAt = now
			if err := m.store.Update(ctx, existing); err != nil {
				m.log.Warn("failed to update entry", "id", existing.ID, "err", err)
			}
			seen[existing.ID] = true
		} else {
			// New entry: create AgentType + CatalogEntry wrapper.
			at.ID = uuid.NewString()
			entry := &model.CatalogEntry{
				ID:          uuid.NewString(),
				AgentTypeID: at.ID,
				AgentType:   at,
				DisplayName: at.Endpoint,
				Source:      source,
				Status:      model.StatusUnknown,
				Validity:    model.Validity{LastSeen: now},
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := m.store.Create(ctx, entry); err != nil {
				m.log.Warn("failed to create entry", "endpoint", at.Endpoint, "err", err)
			} else {
				seen[entry.ID] = true
			}
		}
	}

	// Mark missing non-push entries as down.
	allEntries, err := m.store.List(ctx, store.ListFilter{Source: &source})
	if err != nil {
		return fmt.Errorf("listing entries for source: %w", err)
	}
	for _, e := range allEntries {
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

package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/kernel"
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
	cardStoreMu  sync.RWMutex
	cardStore    kernel.CardStorePlugin // may be nil
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

// SetCardStore injects the card store plugin (called after plugin init).
func (m *Manager) SetCardStore(cs kernel.CardStorePlugin) {
	m.cardStoreMu.Lock()
	defer m.cardStoreMu.Unlock()
	m.cardStore = cs
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
		at.AgentKey = model.ComputeAgentKey(at.Protocol, at.Endpoint)

		existing, err := m.store.FindByEndpoint(ctx, at.Endpoint)
		if err != nil {
			return fmt.Errorf("finding by endpoint: %w", err)
		}
		now := time.Now().UTC()
		if existing != nil {
			if existing.Source == model.SourcePush {
				continue
			}
			if existing.AgentType != nil {
				existing.AgentType.Protocol = at.Protocol
				existing.AgentType.Version = at.Version
				existing.AgentType.SpecVersion = at.SpecVersion
				existing.AgentType.Provider = at.Provider
				existing.AgentType.Capabilities = at.Capabilities
			}
			existing.Validity.LastSeen = now
			existing.UpdatedAt = now
			if err := m.store.Update(ctx, existing); err != nil {
				m.log.Warn("failed to update entry", "id", existing.ID, "err", err)
			} else if existing.AgentType != nil {
				m.maybeStoreCard(ctx, existing.AgentType.ID, at.RawBytes)
			}
			seen[existing.ID] = true
		} else {
			at.ID = uuid.NewString()
			entry := &model.CatalogEntry{
				ID:          uuid.NewString(),
				AgentTypeID: at.ID,
				AgentType:   at,
				DisplayName: at.Endpoint,
				Source:      source,
				Status:      model.LifecycleRegistered,
				Validity:    model.Validity{LastSeen: now},
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := m.store.Create(ctx, entry); err != nil {
				m.log.Warn("failed to create entry", "endpoint", at.Endpoint, "err", err)
			} else {
				m.maybeStoreCard(ctx, at.ID, at.RawBytes)
				seen[entry.ID] = true
			}
		}
	}

	allEntries, err := m.store.List(ctx, store.ListFilter{Source: &source})
	if err != nil {
		return fmt.Errorf("listing entries for source: %w", err)
	}
	for _, e := range allEntries {
		if !seen[e.ID] {
			e.Status = model.LifecycleOffline
			e.UpdatedAt = time.Now().UTC()
			if err := m.store.Update(ctx, &e); err != nil {
				m.log.Warn("failed to mark entry down", "id", e.ID, "err", err)
			}
		}
	}
	return nil
}

func (m *Manager) maybeStoreCard(ctx context.Context, agentTypeID string, rawBytes []byte) {
	if len(rawBytes) == 0 {
		return
	}
	m.cardStoreMu.RLock()
	cs := m.cardStore
	m.cardStoreMu.RUnlock()
	if cs == nil {
		return
	}
	if err := cs.StoreCard(ctx, agentTypeID, rawBytes, "application/json"); err != nil {
		m.log.Warn("failed to store raw card", "agent_type_id", agentTypeID, "err", err)
	}
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

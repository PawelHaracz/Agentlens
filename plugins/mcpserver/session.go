package mcpserver

import (
	"context"
	"sync"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// sessionStore is the minimal interface the plugin needs from MCPSessionStore.
type sessionStore interface {
	Create(ctx context.Context, sess *model.McpSession) error
	GetByID(ctx context.Context, id string) (*model.McpSession, error)
	UpdateInitialized(ctx context.Context, id string, at time.Time) error
	UpdateLastSeen(ctx context.Context, id string, at time.Time) error
	Revoke(ctx context.Context, id string) error
	ReapExpired(ctx context.Context, before time.Time) (int64, error)
	ReapOrphanedPrincipals(ctx context.Context) (int64, error)
	CountActive(ctx context.Context) (int64, error)
}

// sessionManager wraps the DB store with an in-memory active-sessions index
// for hot-path lookups (session validation on every tool call).
type sessionManager struct {
	store sessionStore
	mu    sync.RWMutex
	index map[string]*model.McpSession // sessionID → cached active session
	ttl   time.Duration
}

func newSessionManager(store sessionStore, ttl time.Duration) *sessionManager {
	return &sessionManager{
		store: store,
		index: make(map[string]*model.McpSession),
		ttl:   ttl,
	}
}

// Create persists a new session and caches it.
func (m *sessionManager) Create(ctx context.Context, sess *model.McpSession) error {
	if err := m.store.Create(ctx, sess); err != nil {
		return err
	}
	m.mu.Lock()
	m.index[sess.ID] = sess
	m.mu.Unlock()
	return nil
}

// IsActive returns true if the session exists in the in-memory index and is not revoked.
func (m *sessionManager) IsActive(sessionID string) bool {
	m.mu.RLock()
	sess, ok := m.index[sessionID]
	m.mu.RUnlock()
	return ok && sess.IsActive(time.Now())
}

// MarkInitialized sets initialized_at on DB and in the in-memory cache.
func (m *sessionManager) MarkInitialized(ctx context.Context, sessionID string) error {
	now := time.Now().UTC()
	if err := m.store.UpdateInitialized(ctx, sessionID, now); err != nil {
		return err
	}
	m.mu.Lock()
	if sess, ok := m.index[sessionID]; ok {
		sess.InitializedAt = &now
	}
	m.mu.Unlock()
	return nil
}

// Revoke removes the session from the in-memory index and soft-deletes in DB.
func (m *sessionManager) Revoke(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	delete(m.index, sessionID)
	m.mu.Unlock()
	return m.store.Revoke(ctx, sessionID)
}

// CountActive returns the DB count of active sessions.
func (m *sessionManager) CountActive(ctx context.Context) (int64, error) {
	return m.store.CountActive(ctx)
}

// Reap expires and orphaned sessions. Called by the reaper goroutine.
func (m *sessionManager) Reap(ctx context.Context) {
	_, _ = m.store.ReapExpired(ctx, time.Now())
	_, _ = m.store.ReapOrphanedPrincipals(ctx)
	// Evict expired from in-memory index.
	m.mu.Lock()
	now := time.Now()
	for id, sess := range m.index {
		if !sess.IsActive(now) {
			delete(m.index, id)
		}
	}
	m.mu.Unlock()
}

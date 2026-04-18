package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// MCPSessionStore manages MCP session rows.
type MCPSessionStore struct {
	db *db.DB
}

// NewMCPSessionStore creates a new store backed by the given database.
func NewMCPSessionStore(database *db.DB) *MCPSessionStore {
	return &MCPSessionStore{db: database}
}

// Create inserts a new MCP session. Assigns a UUID if ID is empty.
func (s *MCPSessionStore) Create(ctx context.Context, sess *model.McpSession) error {
	if sess.ID == "" {
		sess.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	if sess.LastSeenAt.IsZero() {
		sess.LastSeenAt = now
	}
	if err := s.db.WithContext(ctx).Create(sess).Error; err != nil {
		return fmt.Errorf("creating mcp session: %w", err)
	}
	return nil
}

// GetByID returns the session with the given ID, or nil if not found.
func (s *MCPSessionStore) GetByID(ctx context.Context, id string) (*model.McpSession, error) {
	var sess model.McpSession
	err := s.db.WithContext(ctx).First(&sess, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting mcp session: %w", err)
	}
	return &sess, nil
}

// UpdateInitialized sets initialized_at to mark the MCP handshake as complete.
// Idempotent: subsequent calls are no-ops if initialized_at is already set.
func (s *MCPSessionStore) UpdateInitialized(ctx context.Context, sessionID string, at time.Time) error {
	if err := s.db.WithContext(ctx).
		Model(&model.McpSession{}).
		Where("id = ? AND initialized_at IS NULL", sessionID).
		Update("initialized_at", at).Error; err != nil {
		return fmt.Errorf("setting initialized_at: %w", err)
	}
	return nil
}

// UpdateLastSeen updates last_seen_at. Designed to be called from the async
// bounded-channel updater in the plugin; errors are non-fatal.
func (s *MCPSessionStore) UpdateLastSeen(ctx context.Context, sessionID string, at time.Time) error {
	if err := s.db.WithContext(ctx).
		Model(&model.McpSession{}).
		Where("id = ?", sessionID).
		Update("last_seen_at", at).Error; err != nil {
		return fmt.Errorf("updating last_seen_at: %w", err)
	}
	return nil
}

// Revoke soft-deletes a session by setting revoked_at. Idempotent.
func (s *MCPSessionStore) Revoke(ctx context.Context, sessionID string) error {
	if err := s.db.WithContext(ctx).
		Model(&model.McpSession{}).
		Where("id = ? AND revoked_at IS NULL", sessionID).
		Update("revoked_at", time.Now().UTC()).Error; err != nil {
		return fmt.Errorf("revoking mcp session: %w", err)
	}
	return nil
}

// ReapExpired revokes all sessions whose expires_at is before the given time
// and that have not already been revoked. Used by the plugin's session reaper.
func (s *MCPSessionStore) ReapExpired(ctx context.Context, before time.Time) (int64, error) {
	result := s.db.WithContext(ctx).
		Model(&model.McpSession{}).
		Where("expires_at < ? AND revoked_at IS NULL", before).
		Update("revoked_at", time.Now().UTC())
	if result.Error != nil {
		return 0, fmt.Errorf("reaping expired sessions: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ReapOrphanedPrincipals revokes active sessions whose principal no longer
// exists — party deleted or user deleted. The check is intentionally
// conservative: it only revokes sessions whose principal_id appears in neither
// parties nor users tables.
func (s *MCPSessionStore) ReapOrphanedPrincipals(ctx context.Context) (int64, error) {
	now := time.Now().UTC()

	// Sessions with principal_type = service_account reference parties.id.
	// Sessions with principal_type = user_local | user_federated reference users.id.
	// Revoke active sessions whose principal no longer resolves.
	result := s.db.WithContext(ctx).Exec(`
		UPDATE mcp_sessions
		SET revoked_at = ?
		WHERE revoked_at IS NULL
		  AND (
		    (principal_type = 'service_account' AND NOT EXISTS (SELECT 1 FROM parties WHERE id = principal_id))
		    OR
		    (principal_type IN ('user_local','user_federated') AND NOT EXISTS (SELECT 1 FROM users WHERE id = principal_id))
		  )
	`, now)
	if result.Error != nil {
		return 0, fmt.Errorf("reaping orphaned sessions: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// CountActive returns the number of active (non-revoked, non-expired) sessions.
func (s *MCPSessionStore) CountActive(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&model.McpSession{}).
		Where("revoked_at IS NULL AND expires_at > ?", time.Now().UTC()).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("counting active sessions: %w", err)
	}
	return count, nil
}

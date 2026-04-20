package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// UserExternalIdentityStore manages federated identity records.
type UserExternalIdentityStore struct {
	db *db.DB
}

// NewUserExternalIdentityStore creates a new store.
func NewUserExternalIdentityStore(database *db.DB) *UserExternalIdentityStore {
	return &UserExternalIdentityStore{db: database}
}

// UpsertPending creates or updates an external identity record when a federated
// user logs in. If the identity is already approved or rejected, last_seen_at
// is updated but status is not changed. New identities land in "pending".
func (s *UserExternalIdentityStore) UpsertPending(ctx context.Context, identity *model.UserExternalIdentity) error {
	if identity.ID == "" {
		identity.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if identity.CreatedAt.IsZero() {
		identity.CreatedAt = now
	}
	identity.LastSeenAt = &now
	if identity.Status == "" {
		identity.Status = model.ExternalIdentityStatusPending
	}

	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "provider_name"}, {Name: "sub"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_seen_at": now,
			"email":        identity.Email,
			"display_name": identity.DisplayName,
		}),
	}).Create(identity)
	if result.Error != nil {
		return fmt.Errorf("upserting external identity: %w", result.Error)
	}
	return nil
}

// Approve marks a pending identity as approved and optionally links it to a
// user. Sets approved_at to now.
func (s *UserExternalIdentityStore) Approve(ctx context.Context, identityID string, userID *string) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":      model.ExternalIdentityStatusApproved,
		"approved_at": now,
	}
	if userID != nil {
		updates["user_id"] = *userID
	}
	if err := s.db.WithContext(ctx).
		Model(&model.UserExternalIdentity{}).
		Where("id = ? AND status = 'pending'", identityID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("approving external identity: %w", err)
	}
	return nil
}

// Reject marks a pending identity as rejected.
func (s *UserExternalIdentityStore) Reject(ctx context.Context, identityID string) error {
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).
		Model(&model.UserExternalIdentity{}).
		Where("id = ? AND status = 'pending'", identityID).
		Updates(map[string]interface{}{
			"status":      model.ExternalIdentityStatusRejected,
			"rejected_at": now,
		}).Error; err != nil {
		return fmt.Errorf("rejecting external identity: %w", err)
	}
	return nil
}

// ListPending returns all identities with status=pending, ordered oldest first.
func (s *UserExternalIdentityStore) ListPending(ctx context.Context) ([]model.UserExternalIdentity, error) {
	var identities []model.UserExternalIdentity
	if err := s.db.WithContext(ctx).
		Where("status = 'pending'").
		Order("created_at ASC").
		Find(&identities).Error; err != nil {
		return nil, fmt.Errorf("listing pending identities: %w", err)
	}
	return identities, nil
}

// GetByProviderSub returns the identity for a given (provider_name, sub) pair,
// or nil if not found.
func (s *UserExternalIdentityStore) GetByProviderSub(ctx context.Context, providerName, sub string) (*model.UserExternalIdentity, error) {
	var identity model.UserExternalIdentity
	err := s.db.WithContext(ctx).
		Where("provider_name = ? AND sub = ?", providerName, sub).
		First(&identity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting external identity by provider+sub: %w", err)
	}
	return &identity, nil
}

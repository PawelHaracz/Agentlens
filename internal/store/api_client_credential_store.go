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

// ApiClientCredentialStore manages service-account API-key credentials.
type ApiClientCredentialStore struct {
	db *db.DB
}

// NewApiClientCredentialStore creates a new store backed by the given database.
func NewApiClientCredentialStore(database *db.DB) *ApiClientCredentialStore {
	return &ApiClientCredentialStore{db: database}
}

// Create inserts a new credential. Assigns a UUID if ID is empty.
func (s *ApiClientCredentialStore) Create(ctx context.Context, c *model.ApiClientCredential) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if err := s.db.WithContext(ctx).Create(c).Error; err != nil {
		return fmt.Errorf("creating api client credential: %w", err)
	}
	return nil
}

// GetByClientID returns the credential with the given client_id, or nil if not found.
func (s *ApiClientCredentialStore) GetByClientID(ctx context.Context, clientID string) (*model.ApiClientCredential, error) {
	var c model.ApiClientCredential
	err := s.db.WithContext(ctx).First(&c, "client_id = ?", clientID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting credential by client_id: %w", err)
	}
	return &c, nil
}

// GetActiveForParty returns the single active (non-revoked) credential for
// the given party, or nil if none exists.
func (s *ApiClientCredentialStore) GetActiveForParty(ctx context.Context, partyID string) (*model.ApiClientCredential, error) {
	var c model.ApiClientCredential
	err := s.db.WithContext(ctx).
		Where("party_id = ? AND revoked_at IS NULL", partyID).
		First(&c).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting active credential for party: %w", err)
	}
	return &c, nil
}

// RotateSecret revokes the currently active credential for partyID (UPDATE first)
// and inserts a new one (INSERT second) in a single transaction.
//
// The UPDATE-before-INSERT ordering ensures the partial unique index
// (party_id WHERE revoked_at IS NULL) is never violated within the transaction.
// Concurrent rotation races surface as gorm.ErrDuplicatedKey on the UPDATE step;
// callers should use errors.Is(err, gorm.ErrDuplicatedKey) for detection per M-new-2.
func (s *ApiClientCredentialStore) RotateSecret(ctx context.Context, partyID string, newCred *model.ApiClientCredential) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Step A: revoke current active credential.
		result := tx.Model(&model.ApiClientCredential{}).
			Where("party_id = ? AND revoked_at IS NULL", partyID).
			Update("revoked_at", now)
		if result.Error != nil {
			return fmt.Errorf("revoking old credential: %w", result.Error)
		}

		// Step B: insert new credential.
		if newCred.ID == "" {
			newCred.ID = uuid.New().String()
		}
		newCred.CreatedAt = now
		if err := tx.Create(newCred).Error; err != nil {
			return fmt.Errorf("inserting new credential: %w", err)
		}
		return nil
	})
}

// Revoke marks a credential as revoked by setting revoked_at.
// Returns nil if the credential is already revoked (idempotent).
func (s *ApiClientCredentialStore) Revoke(ctx context.Context, credentialID string) error {
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).
		Model(&model.ApiClientCredential{}).
		Where("id = ? AND revoked_at IS NULL", credentialID).
		Update("revoked_at", now)
	if result.Error != nil {
		return fmt.Errorf("revoking credential: %w", result.Error)
	}
	return nil
}

// ListForParty returns all credentials (active + revoked) for the given party,
// ordered newest first.
func (s *ApiClientCredentialStore) ListForParty(ctx context.Context, partyID string) ([]model.ApiClientCredential, error) {
	var creds []model.ApiClientCredential
	if err := s.db.WithContext(ctx).
		Where("party_id = ?", partyID).
		Order("created_at DESC").
		Find(&creds).Error; err != nil {
		return nil, fmt.Errorf("listing credentials for party: %w", err)
	}
	return creds, nil
}

// EnumerateActiveForParty returns the client_ids of all active credentials for
// the given party. Used by the service-account deletion handler to invalidate
// credcache entries BEFORE the DB cascade removes the rows (H6-residual).
//
// Callers must invoke credcache.Invalidate(clientID) for each returned ID
// before issuing the DELETE on the party row.
func (s *ApiClientCredentialStore) EnumerateActiveForParty(ctx context.Context, partyID string) ([]string, error) {
	var ids []string
	if err := s.db.WithContext(ctx).
		Model(&model.ApiClientCredential{}).
		Where("party_id = ? AND revoked_at IS NULL", partyID).
		Pluck("client_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("enumerating active credentials for party: %w", err)
	}
	return ids, nil
}

// UpdateLastUsed sets last_used_at for a credential. Called asynchronously by
// the credcache updater goroutine; errors are logged but not fatal.
func (s *ApiClientCredentialStore) UpdateLastUsed(ctx context.Context, clientID string, at time.Time) error {
	if err := s.db.WithContext(ctx).
		Model(&model.ApiClientCredential{}).
		Where("client_id = ?", clientID).
		Update("last_used_at", at).Error; err != nil {
		return fmt.Errorf("updating last_used_at: %w", err)
	}
	return nil
}

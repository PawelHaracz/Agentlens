package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// UpsertProvider finds an existing Provider by (organization, team) or creates a new one.
func (s *SQLStore) UpsertProvider(ctx context.Context, provider *model.Provider) (*model.Provider, error) {
	var existing model.Provider
	result := s.gdb.WithContext(ctx).
		Where("organization = ? AND team = ?", provider.Organization, provider.Team).
		First(&existing)

	if result.Error == nil {
		// Found — return the existing record.
		return &existing, nil
	}

	// Not found — create with a new UUID.
	provider.ID = uuid.NewString()
	if provider.CreatedOn.IsZero() {
		provider.CreatedOn = time.Now().UTC()
	}
	if err := s.gdb.WithContext(ctx).Create(provider).Error; err != nil {
		return nil, fmt.Errorf("creating provider: %w", err)
	}
	return provider, nil
}

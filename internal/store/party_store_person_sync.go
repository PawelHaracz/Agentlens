package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// personName returns the formula display_name || username.
func personName(u *model.User) string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Username
}

// CreatePersonForUser upserts a Person party linked 1:1 to the user.
// Idempotent: no-op if a Person with matching user_id already exists.
func (s *PartyStore) CreatePersonForUser(ctx context.Context, u *model.User) error {
	var existing model.Party
	err := s.db.WithContext(ctx).Where("user_id = ?", u.ID).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("checking existing person: %w", err)
	}
	id := uuid.New().String()
	p := &model.Party{
		ID:     id,
		Kind:   model.PartyKindPerson,
		Name:   personName(u),
		UserID: &u.ID,
	}
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("creating person for user %s: %w", u.ID, err)
	}
	return nil
}

// UpdatePersonForUser resyncs the Person's name to match the user's
// current display_name || username. No-op if no Person exists.
func (s *PartyStore) UpdatePersonForUser(ctx context.Context, u *model.User) error {
	result := s.db.WithContext(ctx).Model(&model.Party{}).
		Where("user_id = ?", u.ID).
		Update("name", personName(u))
	if result.Error != nil {
		return fmt.Errorf("updating person for user %s: %w", u.ID, result.Error)
	}
	return nil
}

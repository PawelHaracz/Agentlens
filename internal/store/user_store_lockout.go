package store

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// IncrementFailedAttempts increments the failed_attempts counter.
// If the counter reaches maxFailedAttempts, the user is automatically locked.
func (s *UserStore) IncrementFailedAttempts(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, "id = ?", id).Error; err != nil {
			return fmt.Errorf("finding user for failed attempt increment: %w", err)
		}
		user.FailedAttempts++
		if user.FailedAttempts >= maxFailedAttempts {
			lockUntil := time.Now().UTC().Add(lockDuration)
			user.LockedUntil = &lockUntil
		}
		if err := tx.Save(&user).Error; err != nil {
			return fmt.Errorf("saving incremented failed attempts: %w", err)
		}
		return nil
	})
}

// ResetFailedAttempts resets the failed_attempts counter and clears lock.
func (s *UserStore) ResetFailedAttempts(ctx context.Context, id string) error {
	err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"failed_attempts": 0,
			"locked_until":    nil,
		}).Error
	if err != nil {
		return fmt.Errorf("resetting failed attempts: %w", err)
	}
	return nil
}

// LockUser locks the user until the given time.
func (s *UserStore) LockUser(ctx context.Context, id string, until time.Time) error {
	err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		Update("locked_until", until).Error
	if err != nil {
		return fmt.Errorf("locking user: %w", err)
	}
	return nil
}

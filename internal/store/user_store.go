package store

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

const maxFailedAttempts = 5
const lockDuration = 15 * time.Minute

// UserStore provides CRUD operations for users.
type UserStore struct {
	db *db.DB
}

// NewUserStore creates a new UserStore backed by the given database.
func NewUserStore(database *db.DB) *UserStore {
	return &UserStore{db: database}
}

// Create inserts a new user.
func (s *UserStore) Create(ctx context.Context, user *model.User) error {
	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by ID with its role preloaded.
func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).Preload("Role").First(&user, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return &user, nil
}

// GetByUsername retrieves a user by username with its role preloaded.
func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	err := s.db.WithContext(ctx).Preload("Role").First(&user, "username = ?", username).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("getting user by username: %w", err)
	}
	return &user, nil
}

// Update saves changes to an existing user.
func (s *UserStore) Update(ctx context.Context, user *model.User) error {
	if err := s.db.WithContext(ctx).Save(user).Error; err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

// Delete removes a user by ID.
func (s *UserStore) Delete(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Delete(&model.User{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return nil
}

// List returns users with their roles, applying limit and offset for pagination.
func (s *UserStore) List(ctx context.Context, limit, offset int) ([]model.User, error) {
	var users []model.User
	query := s.db.WithContext(ctx).Preload("Role").Order("username")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	return users, nil
}

// UpdateLastLogin sets the user's last_login timestamp to now.
func (s *UserStore) UpdateLastLogin(ctx context.Context, id string) error {
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("last_login", now).Error
	if err != nil {
		return fmt.Errorf("updating last login: %w", err)
	}
	return nil
}

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

// Count returns the total number of users.
func (s *UserStore) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

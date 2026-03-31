package store

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// SettingsStore provides access to key-value settings.
type SettingsStore struct {
	db *db.DB
}

// NewSettingsStore creates a new SettingsStore backed by the given database.
func NewSettingsStore(database *db.DB) *SettingsStore {
	return &SettingsStore{db: database}
}

// Get retrieves a single setting by key.
func (s *SettingsStore) Get(ctx context.Context, key string) (*model.Setting, error) {
	var setting model.Setting
	err := s.db.WithContext(ctx).First(&setting, "key = ?", key).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("getting setting: %w", err)
	}
	return &setting, nil
}

// Set creates or updates a setting value by key.
func (s *SettingsStore) Set(ctx context.Context, key, value string) error {
	var setting model.Setting
	err := s.db.WithContext(ctx).First(&setting, "key = ?", key).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("looking up setting: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		setting = model.Setting{
			Key:       key,
			Value:     value,
			UpdatedAt: time.Now().UTC(),
		}
		if err := s.db.WithContext(ctx).Create(&setting).Error; err != nil {
			return fmt.Errorf("creating setting: %w", err)
		}
		return nil
	}

	setting.Value = value
	setting.UpdatedAt = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(&setting).Error; err != nil {
		return fmt.Errorf("updating setting: %w", err)
	}
	return nil
}

// GetByCategory returns all settings in the given category.
func (s *SettingsStore) GetByCategory(ctx context.Context, category string) ([]model.Setting, error) {
	var settings []model.Setting
	if err := s.db.WithContext(ctx).Where("category = ?", category).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("getting settings by category: %w", err)
	}
	return settings, nil
}

// GetAll returns all settings.
func (s *SettingsStore) GetAll(ctx context.Context) ([]model.Setting, error) {
	var settings []model.Setting
	if err := s.db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("getting all settings: %w", err)
	}
	return settings, nil
}

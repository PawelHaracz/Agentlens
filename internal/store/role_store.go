package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// RoleStore provides CRUD operations for roles.
type RoleStore struct {
	db *db.DB
}

// NewRoleStore creates a new RoleStore backed by the given database.
func NewRoleStore(database *db.DB) *RoleStore {
	return &RoleStore{db: database}
}

// GetByID retrieves a role by its ID.
func (s *RoleStore) GetByID(ctx context.Context, id string) (*model.Role, error) {
	var role model.Role
	err := s.db.WithContext(ctx).First(&role, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("getting role by id: %w", err)
	}
	return &role, nil
}

// GetByName retrieves a role by its unique name.
func (s *RoleStore) GetByName(ctx context.Context, name string) (*model.Role, error) {
	var role model.Role
	err := s.db.WithContext(ctx).First(&role, "name = ?", name).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("getting role by name: %w", err)
	}
	return &role, nil
}

// List returns all roles ordered by name.
func (s *RoleStore) List(ctx context.Context) ([]model.Role, error) {
	var roles []model.Role
	if err := s.db.WithContext(ctx).Order("name").Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	return roles, nil
}

// Create inserts a new role.
func (s *RoleStore) Create(ctx context.Context, role *model.Role) error {
	if err := s.db.WithContext(ctx).Create(role).Error; err != nil {
		return fmt.Errorf("creating role: %w", err)
	}
	return nil
}

// Update saves changes to an existing role.
func (s *RoleStore) Update(ctx context.Context, role *model.Role) error {
	if err := s.db.WithContext(ctx).Save(role).Error; err != nil {
		return fmt.Errorf("updating role: %w", err)
	}
	return nil
}

// Delete removes a role by ID. System roles (is_system=true) cannot be deleted.
func (s *RoleStore) Delete(ctx context.Context, id string) error {
	var role model.Role
	if err := s.db.WithContext(ctx).First(&role, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("finding role for deletion: %w", err)
	}
	if role.IsSystem {
		return fmt.Errorf("cannot delete system role")
	}
	if err := s.db.WithContext(ctx).Delete(&model.Role{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("deleting role: %w", err)
	}
	return nil
}

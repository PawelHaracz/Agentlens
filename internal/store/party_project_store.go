package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/model"
	"gorm.io/gorm"
)

// GetDefaultProject returns the system-seeded default project party.
func (s *PartyStore) GetDefaultProject(ctx context.Context) (*model.Party, error) {
	var p model.Party
	err := s.db.WithContext(ctx).First(&p, "kind = ? AND is_system = ?", model.PartyKindProject, true).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("default project not found — was migration007 run?")
		}
		return nil, fmt.Errorf("getting default project: %w", err)
	}
	return &p, nil
}

// AssignToProject adds a catalog entry to a project (idempotent).
func (s *PartyStore) AssignToProject(ctx context.Context, catalogEntryID, projectPartyID string) error {
	m := model.CatalogProjectMembership{
		CatalogEntryID: catalogEntryID,
		ProjectPartyID: projectPartyID,
	}
	return s.db.WithContext(ctx).
		Where("catalog_entry_id = ? AND project_party_id = ?", catalogEntryID, projectPartyID).
		FirstOrCreate(&m).Error
}

// RemoveFromProject removes a catalog entry from a project.
func (s *PartyStore) RemoveFromProject(ctx context.Context, catalogEntryID, projectPartyID string) error {
	return s.db.WithContext(ctx).
		Where("catalog_entry_id = ? AND project_party_id = ?", catalogEntryID, projectPartyID).
		Delete(&model.CatalogProjectMembership{}).Error
}

// ListProjectsForCatalogEntry returns all project parties a catalog entry belongs to.
func (s *PartyStore) ListProjectsForCatalogEntry(ctx context.Context, catalogEntryID string) ([]model.Party, error) {
	var projects []model.Party
	if err := s.db.WithContext(ctx).
		Joins("JOIN catalog_project_memberships cpm ON cpm.project_party_id = parties.id").
		Where("cpm.catalog_entry_id = ?", catalogEntryID).
		Find(&projects).Error; err != nil {
		return nil, fmt.Errorf("listing projects for catalog entry: %w", err)
	}
	return projects, nil
}

// ListCatalogEntryIDsForProject returns catalog entry IDs belonging to a project.
func (s *PartyStore) ListCatalogEntryIDsForProject(ctx context.Context, projectPartyID string) ([]string, error) {
	var ids []string
	if err := s.db.WithContext(ctx).
		Model(&model.CatalogProjectMembership{}).
		Select("catalog_entry_id").
		Where("project_party_id = ?", projectPartyID).
		Find(&ids).Error; err != nil {
		return nil, fmt.Errorf("listing catalog entry ids for project: %w", err)
	}
	return ids, nil
}

// GetProjectRoles returns the from_role values for parties in fromPartyIDs that
// have a project_member relationship to projectPartyID.
func (s *PartyStore) GetProjectRoles(ctx context.Context, fromPartyIDs []string, projectPartyID string) ([]string, error) {
	if len(fromPartyIDs) == 0 {
		return nil, nil
	}
	var roles []string
	if err := s.db.WithContext(ctx).
		Model(&model.PartyRelationship{}).
		Select("from_role").
		Where("relationship_name = ? AND to_party_id = ? AND from_party_id IN ?",
			"project_member", projectPartyID, fromPartyIDs).
		Find(&roles).Error; err != nil {
		return nil, fmt.Errorf("getting project roles: %w", err)
	}
	return roles, nil
}

package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// ErrMemberNotFound is returned by UpdateMemberRole when no PartyRelationship
// matches the supplied (from, to, relationship, oldRole) tuple. Callers should
// map this to HTTP 404, not 500.
var ErrMemberNotFound = errors.New("member relationship not found")

// AddMember creates a PartyRelationship and rebuilds the full closure table if it is a
// containment relationship (as defined by model.ContainmentRelationships).
// Detects and rejects cycles before inserting. Idempotent: duplicate edges silently ignored.
func (s *PartyStore) AddMember(ctx context.Context, rel *model.PartyRelationship) error {
	if rel.ID == "" {
		rel.ID = uuid.New().String()
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if model.ContainmentRelationships[rel.RelationshipName] {
			// Self-loop check
			if rel.FromPartyID == rel.ToPartyID {
				return fmt.Errorf("cannot add party %s as member of itself", rel.FromPartyID)
			}
			// Cycle check: would adding from→to create a cycle?
			// A cycle exists if `to` is already a transitive member of `from`
			// (i.e., `from` is an ancestor of `to` in the current closure).
			var count int64
			if err := tx.Model(&model.PartyGroupClosure{}).
				Where("member_party_id = ? AND ancestor_party_id = ?", rel.ToPartyID, rel.FromPartyID).
				Count(&count).Error; err != nil {
				return fmt.Errorf("cycle detection: %w", err)
			}
			if count > 0 {
				return fmt.Errorf("adding %s → %s would create a cycle", rel.FromPartyID, rel.ToPartyID)
			}
		}

		result := tx.Where(
			"from_party_id = ? AND from_role = ? AND to_party_id = ? AND to_role = ? AND relationship_name = ?",
			rel.FromPartyID, rel.FromRole, rel.ToPartyID, rel.ToRole, rel.RelationshipName,
		).FirstOrCreate(rel)
		if result.Error != nil {
			return fmt.Errorf("adding member relationship: %w", result.Error)
		}
		if model.ContainmentRelationships[rel.RelationshipName] {
			return rebuildAllClosures(tx)
		}
		return nil
	})
}

// RemoveMember deletes the relationship from fromPartyID to toPartyID with the
// given relationship name, then rebuilds the full closure if it is a containment relationship.
func (s *PartyStore) RemoveMember(ctx context.Context, fromPartyID, toPartyID, relationshipName string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"from_party_id = ? AND to_party_id = ? AND relationship_name = ?",
			fromPartyID, toPartyID, relationshipName,
		).Delete(&model.PartyRelationship{}).Error; err != nil {
			return fmt.Errorf("removing member relationship: %w", err)
		}
		if model.ContainmentRelationships[relationshipName] {
			return rebuildAllClosures(tx)
		}
		return nil
	})
}

// ListMembers returns all PartyRelationship records where ToPartyID = toPartyID
// and RelationshipName = relationshipName.
func (s *PartyStore) ListMembers(ctx context.Context, toPartyID, relationshipName string) ([]model.PartyRelationship, error) {
	var rels []model.PartyRelationship
	if err := s.db.WithContext(ctx).
		Where("to_party_id = ? AND relationship_name = ?", toPartyID, relationshipName).
		Find(&rels).Error; err != nil {
		return nil, fmt.Errorf("listing members: %w", err)
	}
	return rels, nil
}

// UpdateMemberRoleParams holds the parameters for UpdateMemberRole.
// OldRole must be the current from_role value to disambiguate when multiple rows
// share the same (FromPartyID, ToPartyID, RelationshipName) but differ in from_role.
type UpdateMemberRoleParams struct {
	FromPartyID      string
	ToPartyID        string
	RelationshipName string
	OldRole          string
	NewRole          string
}

// UpdateMemberRole changes the FromRole of an existing PartyRelationship in place.
// The caller must supply OldRole in params to disambiguate the exact row being updated.
// No closure rebuild is performed — the containment graph is unchanged.
// Returns an error if no matching relationship exists.
func (s *PartyStore) UpdateMemberRole(ctx context.Context, params UpdateMemberRoleParams) error {
	result := s.db.WithContext(ctx).Model(&model.PartyRelationship{}).
		Where("from_party_id = ? AND to_party_id = ? AND relationship_name = ? AND from_role = ?",
			params.FromPartyID, params.ToPartyID, params.RelationshipName, params.OldRole).
		Update("from_role", params.NewRole)
	if result.Error != nil {
		return fmt.Errorf("updating member role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: from=%s to=%s relationship=%s role=%s",
			ErrMemberNotFound, params.FromPartyID, params.ToPartyID, params.RelationshipName, params.OldRole)
	}
	return nil
}

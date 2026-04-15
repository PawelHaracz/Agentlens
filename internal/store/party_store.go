package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// PartyStore provides CRUD operations for the party archetype graph.
type PartyStore struct {
	db *db.DB
}

// NewPartyStore creates a new PartyStore backed by the given database.
func NewPartyStore(database *db.DB) *PartyStore {
	return &PartyStore{db: database}
}

// CreateParty inserts a new party. Assigns a UUID if ID is empty.
// Returns error if Kind is not in ValidPartyKinds.
func (s *PartyStore) CreateParty(ctx context.Context, p *model.Party) error {
	if !model.ValidPartyKinds[p.Kind] {
		return fmt.Errorf("invalid party kind: %q", p.Kind)
	}
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("creating party: %w", err)
	}
	return nil
}

// GetParty returns the party with the given ID, or nil if not found.
func (s *PartyStore) GetParty(ctx context.Context, id string) (*model.Party, error) {
	var p model.Party
	err := s.db.WithContext(ctx).First(&p, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting party: %w", err)
	}
	return &p, nil
}

// GetPartyByUserID returns the Person party linked to the given user ID.
func (s *PartyStore) GetPartyByUserID(ctx context.Context, userID string) (*model.Party, error) {
	var p model.Party
	err := s.db.WithContext(ctx).First(&p, "user_id = ?", userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting party by user id: %w", err)
	}
	return &p, nil
}

// ListParties returns all parties of the given kind.
func (s *PartyStore) ListParties(ctx context.Context, kind model.PartyKind) ([]model.Party, error) {
	var parties []model.Party
	if err := s.db.WithContext(ctx).Where("kind = ?", kind).Find(&parties).Error; err != nil {
		return nil, fmt.Errorf("listing parties: %w", err)
	}
	return parties, nil
}

// ListAllParties returns all parties regardless of kind.
func (s *PartyStore) ListAllParties(ctx context.Context) ([]model.Party, error) {
	var parties []model.Party
	if err := s.db.WithContext(ctx).Find(&parties).Error; err != nil {
		return nil, fmt.Errorf("listing all parties: %w", err)
	}
	return parties, nil
}

// DeleteParty deletes a non-system party and cascades: removes all party_relationships,
// party_group_closures, global_party_roles, and catalog_project_memberships rows that
// reference the party, then rebuilds the full closure. Returns error if not found or is_system.
func (s *PartyStore) DeleteParty(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var party model.Party
		if err := tx.First(&party, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("party %q not found", id)
			}
			return fmt.Errorf("finding party: %w", err)
		}
		if party.IsSystem {
			return fmt.Errorf("party %q is a system party and cannot be deleted", id)
		}

		// Cascade: party_relationships (as from or to)
		if err := tx.Where("from_party_id = ? OR to_party_id = ?", id, id).
			Delete(&model.PartyRelationship{}).Error; err != nil {
			return fmt.Errorf("deleting relationships for party: %w", err)
		}

		// Cascade: global_party_roles
		if err := tx.Where("party_id = ?", id).
			Delete(&model.GlobalPartyRole{}).Error; err != nil {
			return fmt.Errorf("deleting global party roles: %w", err)
		}

		// Cascade: catalog_project_memberships (if party is a project)
		if err := tx.Where("project_party_id = ?", id).
			Delete(&model.CatalogProjectMembership{}).Error; err != nil {
			return fmt.Errorf("deleting catalog project memberships: %w", err)
		}

		// Rebuild closure after cascading relationship deletion
		if err := rebuildAllClosures(tx); err != nil {
			return fmt.Errorf("rebuilding closure after party deletion: %w", err)
		}

		// Delete the party itself
		if err := tx.Delete(&party).Error; err != nil {
			return fmt.Errorf("deleting party: %w", err)
		}
		return nil
	})
}

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

// AncestorGroupIDs returns all ancestor party IDs for the given party from the
// pre-computed party_group_closures table.
func (s *PartyStore) AncestorGroupIDs(ctx context.Context, partyID string) ([]string, error) {
	var ids []string
	if err := s.db.WithContext(ctx).
		Model(&model.PartyGroupClosure{}).
		Select("ancestor_party_id").
		Where("member_party_id = ?", partyID).
		Find(&ids).Error; err != nil {
		return nil, fmt.Errorf("loading ancestor group ids: %w", err)
	}
	return ids, nil
}

// rebuildAllClosures recomputes the entire party_group_closures table from scratch.
// Correct regardless of edge insertion order — a scoped rebuild (per-group) would miss
// transitive members when relationships are added out of order.
// Must run inside an existing transaction.
func rebuildAllClosures(tx *gorm.DB) error {
	// Clear all closure rows
	if err := tx.Exec("DELETE FROM party_group_closures").Error; err != nil {
		return fmt.Errorf("clearing party_group_closures: %w", err)
	}

	names := model.ContainmentRelationshipNames()
	if len(names) == 0 {
		return nil
	}

	// Build placeholders for IN clause explicitly — more reliable than GORM Exec IN expansion.
	placeholders := make([]string, len(names))
	args := make([]interface{}, 0, len(names)*2)
	for i, n := range names {
		placeholders[i] = "?"
		args = append(args, n)
	}
	// args holds [names... names...] for the two IN clauses
	args = append(args, args...)

	inClause := strings.Join(placeholders, ",")
	sql := fmt.Sprintf(`
		INSERT INTO party_group_closures (member_party_id, ancestor_party_id)
		WITH RECURSIVE pairs(member_id, ancestor_id) AS (
			SELECT from_party_id, to_party_id
			FROM party_relationships
			WHERE relationship_name IN (%s)
			UNION
			SELECT p.member_id, pr.to_party_id
			FROM pairs p
			JOIN party_relationships pr ON pr.from_party_id = p.ancestor_id
			WHERE pr.relationship_name IN (%s)
		)
		SELECT DISTINCT member_id, ancestor_id FROM pairs
	`, inClause, inClause)

	return tx.Exec(sql, args...).Error
}

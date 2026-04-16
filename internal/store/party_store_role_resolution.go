package store

import (
	"context"
	"fmt"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// projectRoleRank implements ADR-014's highest-privilege tie-break.
// Unknown roles rank -1 so they never win over known roles.
var projectRoleRank = map[string]int{
	"project:owner":     2,
	"project:developer": 1,
	"project:viewer":    0,
}

func rankRole(role string) int {
	if r, ok := projectRoleRank[role]; ok {
		return r
	}
	return -1
}

// ResolveUserProjects returns the user's project memberships — direct and
// transitive — with the highest-privilege role applied when the user reaches
// the same project via multiple paths. See ADR-014.
// Returns an empty slice (never nil) when the user has no memberships or
// no Person party exists.
func (s *PartyStore) ResolveUserProjects(ctx context.Context, userID string) ([]model.UserProjectMembership, error) {
	person, err := s.GetPartyByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("looking up person for user %s: %w", userID, err)
	}
	if person == nil {
		return []model.UserProjectMembership{}, nil
	}

	var ancestors []string
	if err := s.db.WithContext(ctx).
		Model(&model.PartyGroupClosure{}).
		Where("member_party_id = ?", person.ID).
		Pluck("ancestor_party_id", &ancestors).Error; err != nil {
		return nil, fmt.Errorf("reading closure ancestors: %w", err)
	}
	reachable := append([]string{person.ID}, ancestors...)

	type row struct {
		ToPartyID string `gorm:"column:to_party_id"`
		FromRole  string `gorm:"column:from_role"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("party_relationships pr").
		Select("pr.to_party_id, pr.from_role").
		Joins("JOIN parties p ON p.id = pr.to_party_id").
		Where("pr.from_party_id IN ?", reachable).
		Where("pr.relationship_name = ?", "project_member").
		Where("p.kind = ?", model.PartyKindProject).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("reading project memberships: %w", err)
	}

	bestRole := make(map[string]string, len(rows))
	for _, r := range rows {
		if existing, ok := bestRole[r.ToPartyID]; !ok || rankRole(r.FromRole) > rankRole(existing) {
			bestRole[r.ToPartyID] = r.FromRole
		}
	}
	if len(bestRole) == 0 {
		return []model.UserProjectMembership{}, nil
	}

	projectIDs := make([]string, 0, len(bestRole))
	for id := range bestRole {
		projectIDs = append(projectIDs, id)
	}
	var projects []model.Party
	if err := s.db.WithContext(ctx).
		Where("id IN ?", projectIDs).
		Order("name ASC").
		Find(&projects).Error; err != nil {
		return nil, fmt.Errorf("fetching projects: %w", err)
	}

	out := make([]model.UserProjectMembership, 0, len(projects))
	for _, p := range projects {
		out = append(out, model.UserProjectMembership{Project: p, Role: bestRole[p.ID]})
	}
	return out, nil
}

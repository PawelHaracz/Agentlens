package store_test

import (
	"context"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUserWithPerson(t *testing.T, s *store.PartyStore) (userID, personID string) {
	t.Helper()
	userID = uuid.New().String()
	u := &model.User{ID: userID, Username: "user-" + userID[:8]}
	if err := s.CreatePersonForUser(context.Background(), u); err != nil {
		t.Fatalf("create person: %v", err)
	}
	p, _ := s.GetPartyByUserID(context.Background(), userID)
	return userID, p.ID
}

func addProjectMembership(t *testing.T, s *store.PartyStore, fromPartyID, role, projectID string) {
	t.Helper()
	err := s.AddMember(context.Background(), &model.PartyRelationship{
		FromPartyID:      fromPartyID,
		FromRole:         role,
		ToPartyID:        projectID,
		ToRole:           "project",
		RelationshipName: "project_member",
	})
	require.NoError(t, err)
}

func newProject(t *testing.T, s *store.PartyStore, name string) string {
	t.Helper()
	p := &model.Party{Kind: model.PartyKindProject, Name: name}
	require.NoError(t, s.CreateParty(context.Background(), p))
	return p.ID
}

func newGroup(t *testing.T, s *store.PartyStore, name string) string {
	t.Helper()
	g := &model.Party{Kind: model.PartyKindGroup, Name: name}
	require.NoError(t, s.CreateParty(context.Background(), g))
	return g.ID
}

func addGroupMember(t *testing.T, s *store.PartyStore, memberID, groupID string) {
	t.Helper()
	err := s.AddMember(context.Background(), &model.PartyRelationship{
		FromPartyID:      memberID,
		FromRole:         "member",
		ToPartyID:        groupID,
		ToRole:           "group",
		RelationshipName: "group_member",
	})
	require.NoError(t, err)
}

func TestResolveUserProjects_EmptyWhenNoMemberships(t *testing.T) {
	s := newTestPartyStore(t)
	userID, _ := newUserWithPerson(t, s)

	memberships, err := s.ResolveUserProjects(context.Background(), userID)
	require.NoError(t, err)
	assert.Empty(t, memberships)
	assert.NotNil(t, memberships)
}

func TestResolveUserProjects_EmptyWhenNoPerson(t *testing.T) {
	s := newTestPartyStore(t)

	memberships, err := s.ResolveUserProjects(context.Background(), uuid.New().String())
	require.NoError(t, err)
	assert.Empty(t, memberships)
	assert.NotNil(t, memberships)
}

func TestResolveUserProjects_DirectMembership(t *testing.T) {
	s := newTestPartyStore(t)
	userID, personID := newUserWithPerson(t, s)
	projectID := newProject(t, s, "alpha")
	addProjectMembership(t, s, personID, "project:developer", projectID)

	memberships, err := s.ResolveUserProjects(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.Equal(t, projectID, memberships[0].Project.ID)
	assert.Equal(t, "project:developer", memberships[0].Role)
}

func TestResolveUserProjects_TransitiveViaGroup(t *testing.T) {
	s := newTestPartyStore(t)
	userID, personID := newUserWithPerson(t, s)
	groupID := newGroup(t, s, "eng")
	projectID := newProject(t, s, "beta")

	addGroupMember(t, s, personID, groupID)
	addProjectMembership(t, s, groupID, "project:owner", projectID)

	memberships, err := s.ResolveUserProjects(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.Equal(t, projectID, memberships[0].Project.ID)
	assert.Equal(t, "project:owner", memberships[0].Role)
}

func TestResolveUserProjects_HighestPrivilegeWinsOnMultiPath(t *testing.T) {
	s := newTestPartyStore(t)
	userID, personID := newUserWithPerson(t, s)
	viewerGroup := newGroup(t, s, "viewers")
	ownerGroup := newGroup(t, s, "owners")
	projectID := newProject(t, s, "gamma")

	addGroupMember(t, s, personID, viewerGroup)
	addGroupMember(t, s, personID, ownerGroup)
	addProjectMembership(t, s, viewerGroup, "project:viewer", projectID)
	addProjectMembership(t, s, ownerGroup, "project:owner", projectID)

	memberships, err := s.ResolveUserProjects(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, memberships, 1)
	assert.Equal(t, projectID, memberships[0].Project.ID)
	assert.Equal(t, "project:owner", memberships[0].Role, "highest privilege should win")
}

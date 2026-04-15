package store_test

import (
	"context"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPartyDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)
	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(context.Background()))
	return database
}

func TestPartyStore_CreateAndGet(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	p := &model.Party{Kind: model.PartyKindGroup, Name: "eng-team"}
	require.NoError(t, s.CreateParty(ctx, p))
	assert.NotEmpty(t, p.ID)

	got, err := s.GetParty(ctx, p.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "eng-team", got.Name)
	assert.Equal(t, model.PartyKindGroup, got.Kind)
}

func TestPartyStore_CreateParty_InvalidKind(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	err := s.CreateParty(context.Background(), &model.Party{Kind: "bad-kind", Name: "x"})
	assert.Error(t, err)
}

func TestPartyStore_ListParties(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	require.NoError(t, s.CreateParty(ctx, &model.Party{Kind: model.PartyKindGroup, Name: "g1"}))
	require.NoError(t, s.CreateParty(ctx, &model.Party{Kind: model.PartyKindGroup, Name: "g2"}))
	require.NoError(t, s.CreateParty(ctx, &model.Party{Kind: model.PartyKindProject, Name: "p1"}))

	groups, err := s.ListParties(ctx, model.PartyKindGroup)
	require.NoError(t, err)
	assert.Len(t, groups, 2)

	projects, err := s.ListParties(ctx, model.PartyKindProject)
	require.NoError(t, err)
	// migration seeded "default" project, so expect at least 2
	assert.GreaterOrEqual(t, len(projects), 2)
}

func TestPartyStore_DeleteParty(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	p := &model.Party{Kind: model.PartyKindGroup, Name: "to-delete"}
	require.NoError(t, s.CreateParty(ctx, p))
	require.NoError(t, s.DeleteParty(ctx, p.ID))

	got, err := s.GetParty(ctx, p.ID)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestPartyStore_DeleteParty_SystemPartyRejected(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	def, err := s.GetDefaultProject(ctx)
	require.NoError(t, err)
	require.NotNil(t, def)

	err = s.DeleteParty(ctx, def.ID)
	assert.Error(t, err)
}

func TestPartyStore_AddMember_RebuildsClosure(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	// Create group hierarchy: alice → eng → platform
	alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
	eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
	platform := &model.Party{Kind: model.PartyKindGroup, Name: "platform"}
	require.NoError(t, s.CreateParty(ctx, alice))
	require.NoError(t, s.CreateParty(ctx, eng))
	require.NoError(t, s.CreateParty(ctx, platform))

	// alice → eng
	require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: alice.ID, FromRole: "member",
		ToPartyID: eng.ID, ToRole: "group",
		RelationshipName: "group_member",
	}))
	// eng → platform
	require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: eng.ID, FromRole: "member",
		ToPartyID: platform.ID, ToRole: "group",
		RelationshipName: "group_member",
	}))

	// Alice should have both eng and platform as ancestors
	ancestors, err := s.AncestorGroupIDs(ctx, alice.ID)
	require.NoError(t, err)
	assert.Contains(t, ancestors, eng.ID)
	assert.Contains(t, ancestors, platform.ID)
}

// TestPartyStore_AddMember_OutOfOrderHierarchy tests the correctness of the full-table
// closure rebuild when relationships are added in the "wrong" order (parent→grandparent
// before child→parent). A scoped rebuild would miss (alice, platform) in this scenario.
func TestPartyStore_AddMember_OutOfOrderHierarchy(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
	eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
	platform := &model.Party{Kind: model.PartyKindGroup, Name: "platform"}
	require.NoError(t, s.CreateParty(ctx, alice))
	require.NoError(t, s.CreateParty(ctx, eng))
	require.NoError(t, s.CreateParty(ctx, platform))

	// Add eng → platform FIRST
	require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: eng.ID, FromRole: "member",
		ToPartyID: platform.ID, ToRole: "group",
		RelationshipName: "group_member",
	}))
	// Then add alice → eng
	require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: alice.ID, FromRole: "member",
		ToPartyID: eng.ID, ToRole: "group",
		RelationshipName: "group_member",
	}))

	// Alice must have platform as a transitive ancestor even with reversed insertion order
	ancestors, err := s.AncestorGroupIDs(ctx, alice.ID)
	require.NoError(t, err)
	assert.Contains(t, ancestors, eng.ID, "alice should be in eng")
	assert.Contains(t, ancestors, platform.ID, "alice should transitively be in platform")
}

func TestPartyStore_AddMember_CycleRejected(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
	platform := &model.Party{Kind: model.PartyKindGroup, Name: "platform"}
	require.NoError(t, s.CreateParty(ctx, eng))
	require.NoError(t, s.CreateParty(ctx, platform))

	// eng → platform
	require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: eng.ID, FromRole: "member",
		ToPartyID: platform.ID, ToRole: "group",
		RelationshipName: "group_member",
	}))

	// platform → eng would create a cycle — must be rejected
	err := s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: platform.ID, FromRole: "member",
		ToPartyID: eng.ID, ToRole: "group",
		RelationshipName: "group_member",
	})
	assert.Error(t, err, "cycle should be rejected")
}

func TestPartyStore_RemoveMember_UpdatesClosure(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
	eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
	require.NoError(t, s.CreateParty(ctx, alice))
	require.NoError(t, s.CreateParty(ctx, eng))

	rel := &model.PartyRelationship{
		FromPartyID: alice.ID, FromRole: "member",
		ToPartyID: eng.ID, ToRole: "group",
		RelationshipName: "group_member",
	}
	require.NoError(t, s.AddMember(ctx, rel))

	ancestors, _ := s.AncestorGroupIDs(ctx, alice.ID)
	assert.Contains(t, ancestors, eng.ID)

	require.NoError(t, s.RemoveMember(ctx, alice.ID, eng.ID, "group_member"))

	ancestors, err := s.AncestorGroupIDs(ctx, alice.ID)
	require.NoError(t, err)
	assert.NotContains(t, ancestors, eng.ID)
}

func TestPartyStore_ListMembers(t *testing.T) {
	s := store.NewPartyStore(newTestPartyDB(t))
	ctx := context.Background()

	alice := &model.Party{Kind: model.PartyKindPerson, Name: "alice"}
	bob := &model.Party{Kind: model.PartyKindPerson, Name: "bob"}
	eng := &model.Party{Kind: model.PartyKindGroup, Name: "eng"}
	require.NoError(t, s.CreateParty(ctx, alice))
	require.NoError(t, s.CreateParty(ctx, bob))
	require.NoError(t, s.CreateParty(ctx, eng))

	require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: alice.ID, FromRole: "member",
		ToPartyID: eng.ID, ToRole: "group",
		RelationshipName: "group_member",
	}))
	require.NoError(t, s.AddMember(ctx, &model.PartyRelationship{
		FromPartyID: bob.ID, FromRole: "member",
		ToPartyID: eng.ID, ToRole: "group",
		RelationshipName: "group_member",
	}))

	rels, err := s.ListMembers(ctx, eng.ID, "group_member")
	require.NoError(t, err)
	assert.Len(t, rels, 2)
}

func newTestPartyStore(t *testing.T) *store.PartyStore {
	t.Helper()
	return store.NewPartyStore(newTestPartyDB(t))
}

func TestPartyStore_UpdateMemberRole(t *testing.T) {
	ctx := context.Background()
	s := newTestPartyStore(t)

	project := &model.Party{ID: "proj-1", Kind: model.PartyKindProject, Name: "p1"}
	require.NoError(t, s.CreateParty(ctx, project))
	person := &model.Party{ID: "person-1", Kind: model.PartyKindPerson, Name: "alice"}
	require.NoError(t, s.CreateParty(ctx, person))
	rel := &model.PartyRelationship{
		FromPartyID:      person.ID,
		FromRole:         "project:viewer",
		ToPartyID:        project.ID,
		ToRole:           "project",
		RelationshipName: "project_member",
	}
	require.NoError(t, s.AddMember(ctx, rel))

	require.NoError(t, s.UpdateMemberRole(ctx, person.ID, project.ID, "project_member", "project:developer"))

	rels, err := s.ListMembers(ctx, project.ID, "project_member")
	require.NoError(t, err)
	require.Len(t, rels, 1)
	assert.Equal(t, "project:developer", rels[0].FromRole)
}

func TestPartyStore_UpdateMemberRole_NotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestPartyStore(t)

	err := s.UpdateMemberRole(ctx, "missing", "also-missing", "project_member", "project:viewer")
	assert.Error(t, err)
}

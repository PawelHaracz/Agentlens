package store_test

import (
	"context"
	"testing"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePersonForUser_CreatesWhenMissing(t *testing.T) {
	s := newTestPartyStore(t)
	ctx := context.Background()

	id := uuid.New().String()
	u := &model.User{ID: id, Username: "alice", DisplayName: "Alice Smith"}
	require.NoError(t, s.CreatePersonForUser(ctx, u))

	got, err := s.GetPartyByUserID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.PartyKindPerson, got.Kind)
	assert.Equal(t, "Alice Smith", got.Name)
}

func TestCreatePersonForUser_FallsBackToUsernameWhenNoDisplayName(t *testing.T) {
	s := newTestPartyStore(t)
	ctx := context.Background()

	id := uuid.New().String()
	u := &model.User{ID: id, Username: "bob"}
	require.NoError(t, s.CreatePersonForUser(ctx, u))

	got, err := s.GetPartyByUserID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "bob", got.Name)
}

func TestCreatePersonForUser_IdempotentWhenExists(t *testing.T) {
	s := newTestPartyStore(t)
	ctx := context.Background()

	id := uuid.New().String()
	u := &model.User{ID: id, Username: "carol", DisplayName: "Carol"}
	require.NoError(t, s.CreatePersonForUser(ctx, u))
	require.NoError(t, s.CreatePersonForUser(ctx, u), "second call should be a no-op")
}

func TestUpdatePersonForUser_UpdatesName(t *testing.T) {
	s := newTestPartyStore(t)
	ctx := context.Background()

	id := uuid.New().String()
	u := &model.User{ID: id, Username: "dave", DisplayName: "Dave"}
	require.NoError(t, s.CreatePersonForUser(ctx, u))

	u.DisplayName = "David Updated"
	require.NoError(t, s.UpdatePersonForUser(ctx, u))

	got, err := s.GetPartyByUserID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "David Updated", got.Name)
}

func TestUpdatePersonForUser_NoOpWhenNoPerson(t *testing.T) {
	s := newTestPartyStore(t)
	ctx := context.Background()

	id := uuid.New().String()
	u := &model.User{ID: id, Username: "ghost"}
	require.NoError(t, s.UpdatePersonForUser(ctx, u), "update with no person should be a no-op")
}

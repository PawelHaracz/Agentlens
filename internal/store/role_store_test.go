package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func TestRoleStore_CreateAndGetByID(t *testing.T) {
	database := newTestDB(t)
	rs := store.NewRoleStore(database)
	ctx := context.Background()

	role := &model.Role{
		ID:          "role-custom",
		Name:        "custom",
		Description: "A custom role",
		Permissions: model.JSONSlice{"catalog:read"},
	}
	require.NoError(t, rs.Create(ctx, role))

	got, err := rs.GetByID(ctx, "role-custom")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "custom", got.Name)
	assert.Contains(t, []string(got.Permissions), "catalog:read")
}

func TestRoleStore_GetByID_NotFound(t *testing.T) {
	database := newTestDB(t)
	rs := store.NewRoleStore(database)

	got, err := rs.GetByID(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRoleStore_GetByName(t *testing.T) {
	database := newTestDB(t)
	rs := store.NewRoleStore(database)
	ctx := context.Background()

	// The "admin" role is seeded by migrations
	got, err := rs.GetByName(ctx, "admin")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "role-admin", got.ID)
}

func TestRoleStore_List(t *testing.T) {
	database := newTestDB(t)
	rs := store.NewRoleStore(database)
	ctx := context.Background()

	roles, err := rs.List(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(roles), 3, "should have at least the 3 default roles")
}

func TestRoleStore_Update(t *testing.T) {
	database := newTestDB(t)
	rs := store.NewRoleStore(database)
	ctx := context.Background()

	role := &model.Role{
		ID:          "role-upd",
		Name:        "updatable",
		Description: "before",
		Permissions: model.JSONSlice{"catalog:read"},
	}
	require.NoError(t, rs.Create(ctx, role))

	role.Description = "after"
	require.NoError(t, rs.Update(ctx, role))

	got, err := rs.GetByID(ctx, "role-upd")
	require.NoError(t, err)
	assert.Equal(t, "after", got.Description)
}

func TestRoleStore_Delete_NonSystem(t *testing.T) {
	database := newTestDB(t)
	rs := store.NewRoleStore(database)
	ctx := context.Background()

	role := &model.Role{
		ID:          "role-del",
		Name:        "deletable",
		Description: "Can be deleted",
		Permissions: model.JSONSlice{"catalog:read"},
		IsSystem:    false,
	}
	require.NoError(t, rs.Create(ctx, role))
	require.NoError(t, rs.Delete(ctx, "role-del"))

	got, err := rs.GetByID(ctx, "role-del")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRoleStore_Delete_SystemBlocked(t *testing.T) {
	database := newTestDB(t)
	rs := store.NewRoleStore(database)
	ctx := context.Background()

	// The seeded "admin" role is a system role
	err := rs.Delete(ctx, "role-admin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete system role")

	// Verify it still exists
	got, err := rs.GetByName(ctx, "admin")
	require.NoError(t, err)
	require.NotNil(t, got)
}

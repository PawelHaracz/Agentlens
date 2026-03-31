package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.OpenMemory()
	require.NoError(t, err)
	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(context.Background()))
	t.Cleanup(func() {
		sqlDB, _ := database.DB.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	})
	return database
}

func TestBootstrapAdmin_FirstRun(t *testing.T) {
	database := newTestDB(t)
	userStore := store.NewUserStore(database)
	ctx := context.Background()

	password, err := auth.BootstrapAdmin(ctx, userStore)
	require.NoError(t, err)
	assert.NotEmpty(t, password, "should return generated password")
	assert.GreaterOrEqual(t, len(password), 20)

	// Admin user should exist
	admin, err := userStore.GetByUsername(ctx, "admin")
	require.NoError(t, err)
	require.NotNil(t, admin)
	assert.Equal(t, "Administrator", admin.DisplayName)
	assert.Equal(t, "role-admin", admin.RoleID)
	assert.True(t, admin.IsActive)

	// Password should verify
	assert.True(t, auth.CheckPassword(password, admin.PasswordHash))
}

func TestBootstrapAdmin_AlreadyExists(t *testing.T) {
	database := newTestDB(t)
	userStore := store.NewUserStore(database)
	ctx := context.Background()

	// Create an existing user first
	existing := &model.User{
		ID:           "user-existing",
		Username:     "existing",
		DisplayName:  "Existing",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, userStore.Create(ctx, existing))

	password, err := auth.BootstrapAdmin(ctx, userStore)
	require.NoError(t, err)
	assert.Empty(t, password, "should return empty string when users exist")

	// No admin user should have been created
	admin, err := userStore.GetByUsername(ctx, "admin")
	require.NoError(t, err)
	assert.Nil(t, admin)
}

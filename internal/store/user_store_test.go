package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestUserStore_CreateAndGetByID(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	user := &model.User{
		ID:           "user-1",
		Username:     "alice",
		DisplayName:  "Alice",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user))

	got, err := us.GetByID(ctx, "user-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, "Alice", got.DisplayName)
	assert.NotNil(t, got.Role, "Role should be preloaded")
	assert.Equal(t, "role-viewer", got.Role.ID)
}

func TestUserStore_GetByID_NotFound(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)

	got, err := us.GetByID(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUserStore_GetByUsername(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	user := &model.User{
		ID:           "user-2",
		Username:     "bob",
		DisplayName:  "Bob",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-admin",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user))

	got, err := us.GetByUsername(ctx, "bob")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "user-2", got.ID)
	assert.NotNil(t, got.Role)
}

func TestUserStore_GetByUsername_NotFound(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)

	got, err := us.GetByUsername(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUserStore_Update(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	user := &model.User{
		ID:           "user-3",
		Username:     "charlie",
		DisplayName:  "Charlie",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user))

	user.DisplayName = "Charles"
	require.NoError(t, us.Update(ctx, user))

	got, err := us.GetByID(ctx, "user-3")
	require.NoError(t, err)
	assert.Equal(t, "Charles", got.DisplayName)
}

func TestUserStore_Delete(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	user := &model.User{
		ID:           "user-4",
		Username:     "dave",
		DisplayName:  "Dave",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user))
	require.NoError(t, us.Delete(ctx, "user-4"))

	got, err := us.GetByID(ctx, "user-4")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUserStore_List(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	for i, name := range []string{"zara", "alice", "mike"} {
		u := &model.User{
			ID:           "user-l" + string(rune('0'+i)),
			Username:     name,
			DisplayName:  name,
			PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
			RoleID:       "role-viewer",
			IsActive:     true,
		}
		require.NoError(t, us.Create(ctx, u))
	}

	users, err := us.List(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, users, 3)
	assert.Equal(t, "alice", users[0].Username, "should be ordered by username")

	users, err = us.List(ctx, 2, 0)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserStore_DuplicateUsername(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	user1 := &model.User{
		ID:           "user-dup1",
		Username:     "same",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user1))

	user2 := &model.User{
		ID:           "user-dup2",
		Username:     "same",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	err := us.Create(ctx, user2)
	require.Error(t, err)
}

func TestUserStore_LockoutFlow(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	user := &model.User{
		ID:           "user-lock",
		Username:     "lockme",
		DisplayName:  "Lock Me",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user))

	// 4 failed attempts — no lock yet
	for i := 0; i < 4; i++ {
		require.NoError(t, us.IncrementFailedAttempts(ctx, "user-lock"))
	}
	got, err := us.GetByID(ctx, "user-lock")
	require.NoError(t, err)
	assert.Equal(t, 4, got.FailedAttempts)
	assert.Nil(t, got.LockedUntil)

	// 5th attempt triggers auto-lock
	require.NoError(t, us.IncrementFailedAttempts(ctx, "user-lock"))
	got, err = us.GetByID(ctx, "user-lock")
	require.NoError(t, err)
	assert.Equal(t, 5, got.FailedAttempts)
	require.NotNil(t, got.LockedUntil)
	assert.True(t, got.LockedUntil.After(time.Now()))

	// Reset clears everything
	require.NoError(t, us.ResetFailedAttempts(ctx, "user-lock"))
	got, err = us.GetByID(ctx, "user-lock")
	require.NoError(t, err)
	assert.Equal(t, 0, got.FailedAttempts)
	assert.Nil(t, got.LockedUntil)
}

func TestUserStore_Count(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	count, err := us.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	user := &model.User{
		ID:           "user-cnt",
		Username:     "counter",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user))

	count, err = us.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestUserStore_UpdateLastLogin(t *testing.T) {
	database := newTestDB(t)
	us := store.NewUserStore(database)
	ctx := context.Background()

	user := &model.User{
		ID:           "user-ll",
		Username:     "lastlogin",
		PasswordHash: "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfa",
		RoleID:       "role-viewer",
		IsActive:     true,
	}
	require.NoError(t, us.Create(ctx, user))

	require.NoError(t, us.UpdateLastLogin(ctx, "user-ll"))

	got, err := us.GetByID(ctx, "user-ll")
	require.NoError(t, err)
	require.NotNil(t, got.LastLogin)
	assert.WithinDuration(t, time.Now(), *got.LastLogin, 5*time.Second)
}

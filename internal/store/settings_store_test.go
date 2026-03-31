package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/store"
)

func TestSettingsStore_GetExisting(t *testing.T) {
	database := newTestDB(t)
	ss := store.NewSettingsStore(database)
	ctx := context.Background()

	// The "app.name" setting is seeded by migration004
	got, err := ss.Get(ctx, "app.name")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "AgentLens", got.Value)
}

func TestSettingsStore_GetNotFound(t *testing.T) {
	database := newTestDB(t)
	ss := store.NewSettingsStore(database)

	got, err := ss.Get(context.Background(), "nonexistent.key")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestSettingsStore_SetNew(t *testing.T) {
	database := newTestDB(t)
	ss := store.NewSettingsStore(database)
	ctx := context.Background()

	require.NoError(t, ss.Set(ctx, "test.key", "test-value"))

	got, err := ss.Get(ctx, "test.key")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "test-value", got.Value)
}

func TestSettingsStore_SetUpdate(t *testing.T) {
	database := newTestDB(t)
	ss := store.NewSettingsStore(database)
	ctx := context.Background()

	// Update the seeded "app.name" setting
	require.NoError(t, ss.Set(ctx, "app.name", "NewName"))

	got, err := ss.Get(ctx, "app.name")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "NewName", got.Value)
}

func TestSettingsStore_GetByCategory(t *testing.T) {
	database := newTestDB(t)
	ss := store.NewSettingsStore(database)
	ctx := context.Background()

	settings, err := ss.GetByCategory(ctx, "auth")
	require.NoError(t, err)
	assert.Len(t, settings, 2, "migration seeds 2 auth settings")
}

func TestSettingsStore_GetAll(t *testing.T) {
	database := newTestDB(t)
	ss := store.NewSettingsStore(database)
	ctx := context.Background()

	settings, err := ss.GetAll(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(settings), 3, "migration seeds 3 default settings")
}

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	d, err := OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, _ := d.DB.DB()
		sqlDB.Close()
	})
	return d
}

func TestMigrate_Fresh(t *testing.T) {
	d := testDB(t)
	m := NewMigrator(d, AllMigrations())
	ctx := context.Background()

	require.NoError(t, m.Migrate(ctx))

	ver, err := m.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, ver)

	// Verify tables exist by querying them.
	assert.True(t, d.Migrator().HasTable("catalog_entries"))
	assert.True(t, d.Migrator().HasTable("roles"))
	assert.True(t, d.Migrator().HasTable("users"))
	assert.True(t, d.Migrator().HasTable("settings"))
}

func TestMigrate_Idempotent(t *testing.T) {
	d := testDB(t)
	m := NewMigrator(d, AllMigrations())
	ctx := context.Background()

	require.NoError(t, m.Migrate(ctx))
	require.NoError(t, m.Migrate(ctx))

	ver, err := m.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, ver)
}

func TestMigrate_CurrentVersion(t *testing.T) {
	d := testDB(t)
	m := NewMigrator(d, AllMigrations())
	ctx := context.Background()

	// Before any migration, version should be 0.
	ver, err := m.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, ver)

	// Run only the first two migrations.
	partial := NewMigrator(d, AllMigrations()[:2])
	require.NoError(t, partial.Migrate(ctx))

	ver, err = m.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, ver)

	// Now run all migrations.
	require.NoError(t, m.Migrate(ctx))
	ver, err = m.CurrentVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, ver)
}

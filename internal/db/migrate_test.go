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
		_ = sqlDB.Close()
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
	assert.Equal(t, 10, ver)

	// Verify tables exist by querying them.
	assert.True(t, d.Migrator().HasTable("providers"))
	assert.True(t, d.Migrator().HasTable("agent_types"))
	assert.True(t, d.Migrator().HasTable("capabilities"))
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
	assert.Equal(t, 10, ver)
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
	assert.Equal(t, 10, ver)
}

func TestMigration006_RawCardsCreated(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		sqlDB, _ := database.DB.DB()
		_ = sqlDB.Close()
	}()
	migrator := NewMigrator(database, AllMigrations())
	if err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Verify raw_cards table exists.
	if !database.Migrator().HasTable("raw_cards") {
		t.Error("raw_cards table should exist after migration 006")
	}
	// Verify agent_types table still exists.
	if !database.Migrator().HasTable("agent_types") {
		t.Error("agent_types table should still exist")
	}
}

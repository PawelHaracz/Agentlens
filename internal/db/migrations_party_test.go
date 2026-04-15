package db_test

import (
	"testing"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration008_BackfillCreatesPersonsForOrphanUsers(t *testing.T) {
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)

	// Run only migrations 1-7.
	all := db.AllMigrations()
	var upToSeven []db.Migration
	for _, m := range all {
		if m.Version <= 7 {
			upToSeven = append(upToSeven, m)
		}
	}
	migrator := db.NewMigrator(database, upToSeven)
	require.NoError(t, migrator.Migrate(t.Context()))

	sqlDB, err := database.DB.DB()
	require.NoError(t, err)

	// Insert a role and a user directly (bypassing the API) with no matching Person.
	_, err = sqlDB.Exec(`
		INSERT INTO roles (id, name, permissions, is_system, created_at, updated_at)
		VALUES ('role-orphan', 'orphan', '[]', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`
		INSERT INTO users (id, username, display_name, email, password_hash, role_id, is_active, created_at, updated_at)
		VALUES ('orphan-user-1', 'orphanuser', 'Orphan User', '', 'hash', 'role-orphan', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	require.NoError(t, err)

	// Confirm no Person exists yet.
	var personCount int
	row := sqlDB.QueryRow("SELECT COUNT(*) FROM parties WHERE user_id='orphan-user-1'")
	require.NoError(t, row.Scan(&personCount))
	assert.Equal(t, 0, personCount, "no Person should exist before migration008")

	// Run migration008.
	migrator8 := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator8.Migrate(t.Context()))

	// Assert Person was created.
	var name string
	row = sqlDB.QueryRow("SELECT name FROM parties WHERE user_id='orphan-user-1' AND kind='person'")
	require.NoError(t, row.Scan(&name))
	assert.Equal(t, "Orphan User", name)
}

func TestMigration008_Idempotent(t *testing.T) {
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)

	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(t.Context()))

	// Run full migration set a second time — must not error.
	migrator2 := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator2.Migrate(t.Context()))
}

func TestMigration007_TablesExist(t *testing.T) {
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)

	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(t.Context()))

	sqlDB, err := database.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	tables := []string{
		"parties",
		"party_relationships",
		"party_group_closures",
		"global_party_roles",
		"catalog_project_memberships",
		"party_identifiers",
	}
	for _, tbl := range tables {
		var count int
		row := sqlDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tbl)
		require.NoError(t, row.Scan(&count))
		assert.Equal(t, 1, count, "table %s should exist", tbl)
	}

	// Default project should be seeded
	var partyCount int
	row := sqlDB.QueryRow("SELECT COUNT(*) FROM parties WHERE kind='project' AND is_system=1")
	require.NoError(t, row.Scan(&partyCount))
	assert.Equal(t, 1, partyCount, "default project party should exist")
}

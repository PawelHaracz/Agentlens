package db_test

import (
	"testing"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration007_TablesExist(t *testing.T) {
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)

	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(t.Context()))

	sqlDB, err := database.DB.DB()
	require.NoError(t, err)

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

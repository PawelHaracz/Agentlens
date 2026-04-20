package db_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/db"
)

func openMigrated010(t *testing.T) (*db.DB, *sql.DB) {
	t.Helper()
	database, err := db.Open(db.DialectSQLite, ":memory:")
	require.NoError(t, err)
	migrator := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator.Migrate(t.Context()))
	sqlDB, err := database.DB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return database, sqlDB
}

func TestMigration010_Idempotent_SQLite(t *testing.T) {
	database, _ := openMigrated010(t)
	// Running migrations a second time must be a no-op.
	migrator2 := db.NewMigrator(database, db.AllMigrations())
	require.NoError(t, migrator2.Migrate(t.Context()))

	ver, err := db.NewMigrator(database, db.AllMigrations()).CurrentVersion(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 10, ver)
}

func TestMigration010_TablesExist(t *testing.T) {
	_, sqlDB := openMigrated010(t)

	tables := []string{
		"api_client_credentials",
		"mcp_sessions",
		"user_external_identities",
	}
	for _, tbl := range tables {
		var count int
		row := sqlDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tbl)
		require.NoError(t, row.Scan(&count))
		assert.Equal(t, 1, count, "table %s should exist after migration010", tbl)
	}
}

func TestMigration010_PartialUniqueIndex_ExistsOnSQLite(t *testing.T) {
	_, sqlDB := openMigrated010(t)

	// Verify the partial unique index on api_client_credentials was created.
	var count int
	row := sqlDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_acc_one_active_per_party'`)
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 1, count, "partial unique index idx_acc_one_active_per_party must exist")
}

func TestMigration010_AdminRoleHasServiceAccountPerms(t *testing.T) {
	_, sqlDB := openMigrated010(t)

	var permsJSON string
	row := sqlDB.QueryRow("SELECT permissions FROM roles WHERE id='role-admin'")
	require.NoError(t, row.Scan(&permsJSON))
	assert.Contains(t, permsJSON, "service_accounts:read")
	assert.Contains(t, permsJSON, "service_accounts:write")
	assert.Contains(t, permsJSON, "service_accounts:revoke")
}

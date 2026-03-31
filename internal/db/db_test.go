package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_SQLite(t *testing.T) {
	d, err := OpenMemory()
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, DialectSQLite, d.Dialect())

	sqlDB, err := d.DB.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	require.NoError(t, sqlDB.Close())
}

func TestOpen_InvalidDialect(t *testing.T) {
	d, err := Open("mysql", "fake-dsn")
	assert.Error(t, err)
	assert.Nil(t, d)
	assert.Contains(t, err.Error(), "unsupported dialect")
}

package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func TestMigration005HealthColumns(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer s.Close()

	// Create a catalog entry
	entry := sampleEntry("migration-test-1")
	require.NoError(t, s.Create(context.Background(), entry))

	// Retrieve and verify health columns exist and are initialized properly
	got, err := s.Get(context.Background(), entry.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	// HealthLastProbedAt should be nil for new entry (never probed)
	assert.Nil(t, got.HealthLastProbedAt)
	// HealthLastSuccessAt should be nil for new entry
	assert.Nil(t, got.HealthLastSuccessAt)
	// HealthLastError should be empty for new entry
	assert.Equal(t, "", got.HealthLastError)
	// HealthLatencyMs should be 0 for new entry
	assert.Equal(t, int64(0), got.HealthLatencyMs)
	// HealthConsecutiveFailures should be 0 for new entry
	assert.Equal(t, 0, got.HealthConsecutiveFailures)
	// Status should be registered (initial lifecycle state)
	assert.Equal(t, model.LifecycleRegistered, got.Status)
}

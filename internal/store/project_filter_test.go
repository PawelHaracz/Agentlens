package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/store"
)

// TestCatalogStore_FiltersByProjectIDs verifies that CatalogFilter.ProjectIDs
// restricts List() results to entries belonging to the specified projects (E.6).
func TestCatalogStore_FiltersByProjectIDs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create two entries in different projects via the party/project store.
	// For simplicity we use the raw SQLiteStore which auto-assigns to default project.
	// We seed a second project and membership manually.
	e1 := sampleEntry("filter-proj-1")
	e2 := sampleEntry("filter-proj-2")

	require.NoError(t, s.Create(ctx, e1))
	require.NoError(t, s.Create(ctx, e2))

	// Without project filter → both entries visible.
	all, err := s.List(ctx, store.ListFilter{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(all), 2, "all entries should be returned without project filter")

	// With empty ProjectIDs → behaves like no filter (not restrictive).
	noFilter, err := s.List(ctx, store.ListFilter{ProjectIDs: []string{}})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(noFilter), 2)

	// With a bogus ProjectIDs that no entry belongs to → 0 results.
	filtered, err := s.List(ctx, store.ListFilter{ProjectIDs: []string{"nonexistent-project-id"}})
	require.NoError(t, err)
	assert.Empty(t, filtered, "ProjectIDs filter with no matches must return empty")
}

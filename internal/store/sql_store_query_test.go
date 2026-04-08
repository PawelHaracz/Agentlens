package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

// namedEntry creates a catalog entry with a specific display name for sort testing.
func namedEntry(id, displayName string) *model.CatalogEntry {
	now := time.Now().UTC().Truncate(time.Second)
	endpoint := "http://sort-test.example.com/" + id
	agentType := &model.AgentType{
		ID:        "at-sort-" + id,
		AgentKey:  model.ComputeAgentKey(model.ProtocolA2A, endpoint),
		Protocol:  model.ProtocolA2A,
		Endpoint:  endpoint,
		Version:   "1.0.0",
		CreatedOn: now,
	}
	return &model.CatalogEntry{
		ID:          "sort-" + id,
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: displayName,
		Description: "sort test entry",
		Status:      model.LifecycleActive,
		Source:      model.SourcePush,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestList_SortDisplayNameAsc(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Insert in reverse alphabetical order
	bEntry := namedEntry("b", "Bravo Agent")
	aEntry := namedEntry("a", "Alpha Agent")

	require.NoError(t, s.Create(ctx, bEntry))
	require.NoError(t, s.Create(ctx, aEntry))

	list, err := s.List(ctx, store.ListFilter{Sort: "displayName_asc"})
	require.NoError(t, err)
	require.Len(t, list, 2)

	assert.Equal(t, "Alpha Agent", list[0].DisplayName, "expected Alpha first in displayName_asc order")
	assert.Equal(t, "Bravo Agent", list[1].DisplayName, "expected Bravo second in displayName_asc order")
}

func TestList_SortCreatedAtDesc(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	older := now.Add(-10 * time.Minute)

	firstCreated := namedEntry("first", "First Created")
	firstCreated.CreatedAt = older

	secondCreated := namedEntry("second", "Second Created")
	secondCreated.CreatedAt = now

	require.NoError(t, s.Create(ctx, firstCreated))
	require.NoError(t, s.Create(ctx, secondCreated))

	list, err := s.List(ctx, store.ListFilter{Sort: "createdAt_desc"})
	require.NoError(t, err)
	require.Len(t, list, 2)

	assert.Equal(t, "Second Created", list[0].DisplayName, "expected most recently created first")
	assert.Equal(t, "First Created", list[1].DisplayName, "expected oldest created last")
}

func TestList_ExpandedSearch_Capabilities(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Entry with a capability named "translate-text"
	entryWithCap := namedEntry("cap-search", "Translation Agent")
	entryWithCap.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{
			Name:        "translate-text",
			Description: "performs language translation",
			InputModes:  []string{"text"},
			OutputModes: []string{"text"},
		},
	}
	require.NoError(t, s.Create(ctx, entryWithCap))

	// Entry without the capability
	other := namedEntry("no-cap", "Other Agent")
	require.NoError(t, s.Create(ctx, other))

	// Search by capability name
	results, err := s.List(ctx, store.ListFilter{Query: "translate"})
	require.NoError(t, err)
	require.Len(t, results, 1, "expected exactly one entry matching capability 'translate'")
	assert.Equal(t, "Translation Agent", results[0].DisplayName)
}

func TestList_ExpandedSearch_Categories(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	entry := namedEntry("cat-search", "Category Agent")
	entry.Categories = []string{"nlp", "translation"}
	require.NoError(t, s.Create(ctx, entry))

	other := namedEntry("other-cat", "Other Agent")
	other.Categories = []string{"vision"}
	require.NoError(t, s.Create(ctx, other))

	results, err := s.List(ctx, store.ListFilter{Query: "nlp"})
	require.NoError(t, err)
	require.Len(t, results, 1, "expected one entry matching category 'nlp'")
	assert.Equal(t, "Category Agent", results[0].DisplayName)
}

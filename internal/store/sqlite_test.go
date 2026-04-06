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

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sampleEntry(id string) *model.CatalogEntry {
	now := time.Now().UTC().Truncate(time.Second)
	endpoint := "http://example.com/" + id
	agentType := &model.AgentType{
		ID:            "at-" + id,
		AgentKey:      model.ComputeAgentKey(model.ProtocolA2A, endpoint),
		Protocol:      model.ProtocolA2A,
		Endpoint:      endpoint,
		Version:       "1.0.0",
		RawDefinition: []byte("{}"),
		CreatedOn:     now,
	}
	return &model.CatalogEntry{
		ID:          id,
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: "Test Entry " + id,
		Description: "A test entry",
		Status:      model.StatusUnknown,
		Source:      model.SourcePush,
		Categories:  []string{"cat1", "cat2"},
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestCreate_Get(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := sampleEntry("entry-1")
	// Add a skill capability
	a.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "skill1", Description: "does stuff", InputModes: []string{"text"}, OutputModes: []string{"text"}},
	}
	require.NoError(t, s.Create(ctx, a))

	got, err := s.Get(ctx, "entry-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, a.DisplayName, got.DisplayName)
	assert.Equal(t, model.ProtocolA2A, got.AgentType.Protocol)
	assert.Equal(t, a.Categories, got.Categories)
	assert.Len(t, got.AgentType.Capabilities, 1)
	assert.Equal(t, "skill1", got.AgentType.Capabilities[0].(*model.A2ASkill).Name)
}

func TestGet_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	got, err := s.Get(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUpdate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := sampleEntry("entry-2")
	require.NoError(t, s.Create(ctx, a))

	a.DisplayName = "Updated Name"
	a.Status = model.StatusHealthy
	a.UpdatedAt = time.Now().UTC()
	require.NoError(t, s.Update(ctx, a))

	got, err := s.Get(ctx, "entry-2")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", got.DisplayName)
	assert.Equal(t, model.StatusHealthy, got.Status)
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := sampleEntry("entry-3")
	require.NoError(t, s.Create(ctx, a))
	require.NoError(t, s.Delete(ctx, "entry-3"))

	got, err := s.Get(ctx, "entry-3")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	entries := []*model.CatalogEntry{
		sampleEntry("a1"),
		sampleEntry("a2"),
		sampleEntry("a3"),
	}
	// Override protocol for a2 (endpoint is already unique per sampleEntry)
	entries[1].AgentType.Protocol = model.ProtocolMCP
	entries[1].AgentType.AgentKey = model.ComputeAgentKey(model.ProtocolMCP, entries[1].AgentType.Endpoint)
	entries[2].Status = model.StatusHealthy

	for _, e := range entries {
		require.NoError(t, s.Create(ctx, e))
	}

	t.Run("list all", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilter{})
		require.NoError(t, err)
		assert.Len(t, list, 3)
	})

	t.Run("filter by protocol", func(t *testing.T) {
		p := model.ProtocolMCP
		list, err := s.List(ctx, store.ListFilter{Protocol: &p})
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})

	t.Run("filter by status", func(t *testing.T) {
		st := model.StatusHealthy
		list, err := s.List(ctx, store.ListFilter{Status: &st})
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})

	t.Run("limit and offset", func(t *testing.T) {
		list, err := s.List(ctx, store.ListFilter{Limit: 2, Offset: 0})
		require.NoError(t, err)
		assert.Len(t, list, 2)
	})
}

func TestFindByEndpoint(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := sampleEntry("entry-ep")
	require.NoError(t, s.Create(ctx, a))

	got, err := s.FindByEndpoint(ctx, a.AgentType.Endpoint)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, a.ID, got.ID)

	notFound, err := s.FindByEndpoint(ctx, "http://notexist")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestSearchCapabilities(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := sampleEntry("entry-skills")
	a.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "translate", Description: "language translation"},
	}
	require.NoError(t, s.Create(ctx, a))

	results, err := s.SearchCapabilities(ctx, "translate")
	require.NoError(t, err)
	assert.Len(t, results, 1)

	empty, err := s.SearchCapabilities(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Len(t, empty, 0)
}

func TestStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a1 := sampleEntry("s1")
	a1.Status = model.StatusHealthy
	a2 := sampleEntry("s2")
	a2.Status = model.StatusDown
	a3 := sampleEntry("s3")
	a3.Status = model.StatusHealthy
	a3.Source = model.SourceK8s

	for _, e := range []*model.CatalogEntry{a1, a2, a3} {
		require.NoError(t, s.Create(ctx, e))
	}

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 2, stats.ByStatus["healthy"])
	assert.Equal(t, 1, stats.ByStatus["down"])
}

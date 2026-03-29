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
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleEntry(id string) *model.CatalogEntry {
	now := time.Now().UTC().Truncate(time.Second)
	return &model.CatalogEntry{
		ID:          id,
		DisplayName: "Test Entry " + id,
		Description: "A test entry",
		Protocol:    model.ProtocolA2A,
		Endpoint:    "http://example.com/" + id,
		Version:     "1.0.0",
		Status:      model.StatusUnknown,
		Source:      model.SourcePush,
		Provider:    model.Provider{Team: "team-a"},
		Categories:  []string{"cat1", "cat2"},
		Skills: []model.Skill{
			{Name: "skill1", Description: "does stuff", InputModes: []string{"text"}, OutputModes: []string{"text"}},
		},
		Validity:  model.Validity{LastSeen: now},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCreate_Get(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := sampleEntry("entry-1")
	require.NoError(t, s.Create(ctx, a))

	got, err := s.Get(ctx, "entry-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, a.DisplayName, got.DisplayName)
	assert.Equal(t, a.Protocol, got.Protocol)
	assert.Equal(t, a.Categories, got.Categories)
	assert.Len(t, got.Skills, 1)
	assert.Equal(t, "skill1", got.Skills[0].Name)
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
	entries[1].Protocol = model.ProtocolMCP
	entries[1].Endpoint = "http://example.com/a2"
	entries[2].Status = model.StatusHealthy
	entries[2].Endpoint = "http://example.com/a3"

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

	got, err := s.FindByEndpoint(ctx, a.Endpoint)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, a.ID, got.ID)

	notFound, err := s.FindByEndpoint(ctx, "http://notexist")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestSearchSkills(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a := sampleEntry("entry-skills")
	a.Skills = []model.Skill{
		{Name: "translate", Description: "language translation"},
	}
	require.NoError(t, s.Create(ctx, a))

	results, err := s.SearchSkills(ctx, "translate")
	require.NoError(t, err)
	assert.Len(t, results, 1)

	empty, err := s.SearchSkills(ctx, "nonexistent")
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
	a2.Endpoint = "http://example.com/s2"
	a3 := sampleEntry("s3")
	a3.Status = model.StatusHealthy
	a3.Source = model.SourceK8s
	a3.Endpoint = "http://example.com/s3"

	for _, e := range []*model.CatalogEntry{a1, a2, a3} {
		require.NoError(t, s.Create(ctx, e))
	}

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 2, stats.ByStatus["healthy"])
	assert.Equal(t, 1, stats.ByStatus["down"])
}

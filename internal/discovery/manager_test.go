package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

type mockSource struct {
	name    string
	entries []*model.CatalogEntry
	err     error
}

func (m *mockSource) Name() string { return m.name }
func (m *mockSource) Discover(_ context.Context) ([]*model.CatalogEntry, error) {
	return m.entries, m.err
}

func newMockEntry(id, endpoint string) *model.CatalogEntry {
	now := time.Now().UTC()
	return &model.CatalogEntry{
		ID:          id,
		DisplayName: "Entry " + id,
		Protocol:    model.ProtocolA2A,
		Endpoint:    endpoint,
		Status:      model.StatusUnknown,
		Source:      model.SourceConfig,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestManager_Upsert_NewEntry(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	src := &mockSource{
		name:    "static",
		entries: []*model.CatalogEntry{newMockEntry("", "http://new-entry.example.com")},
	}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		_ = mgr.Run(ctx)
	}()
	time.Sleep(200 * time.Millisecond)

	entries, err := s.List(context.Background(), store.ListFilter{})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "http://new-entry.example.com", entries[0].Endpoint)
}

func TestManager_Upsert_ExistingEntry(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	existing := newMockEntry("existing-id", "http://existing.example.com")
	existing.Source = model.SourceConfig
	require.NoError(t, s.Create(ctx, existing))

	updated := newMockEntry("", "http://existing.example.com")
	updated.DisplayName = "Updated Name"
	src := &mockSource{name: "static", entries: []*model.CatalogEntry{updated}}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	mgrCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go func() {
		_ = mgr.Run(mgrCtx)
	}()
	time.Sleep(200 * time.Millisecond)

	got, err := s.Get(ctx, "existing-id")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Updated Name", got.DisplayName)
}

func TestManager_MarksMissingEntriesDown(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	existing := newMockEntry("missing-id", "http://gone.example.com")
	existing.Source = model.SourceConfig
	require.NoError(t, s.Create(ctx, existing))

	// Source returns no entries this cycle
	src := &mockSource{name: "static", entries: nil}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	mgrCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go func() {
		_ = mgr.Run(mgrCtx)
	}()
	time.Sleep(200 * time.Millisecond)

	got, err := s.Get(ctx, "missing-id")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.StatusDown, got.Status)
}

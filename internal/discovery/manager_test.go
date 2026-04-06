package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/discovery"
	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

type mockSource struct {
	name       string
	agentTypes []*model.AgentType
	err        error
}

func (m *mockSource) Name() string { return m.name }
func (m *mockSource) Discover(_ context.Context) ([]*model.AgentType, error) {
	return m.agentTypes, m.err
}

func newMockAgentType(endpoint string) *model.AgentType {
	return &model.AgentType{
		AgentKey:      model.ComputeAgentKey(model.ProtocolA2A, endpoint),
		Protocol:      model.ProtocolA2A,
		Endpoint:      endpoint,
		Version:       "1.0.0",
		RawDefinition: []byte(`{}`),
	}
}

func newMockCatalogEntry(id, endpoint string) *model.CatalogEntry {
	at := &model.AgentType{
		ID:            uuid.NewString(),
		AgentKey:      model.ComputeAgentKey(model.ProtocolA2A, endpoint),
		Protocol:      model.ProtocolA2A,
		Endpoint:      endpoint,
		Version:       "1.0.0",
		RawDefinition: []byte(`{}`),
	}
	now := time.Now().UTC()
	entry := &model.CatalogEntry{
		ID:          id,
		AgentTypeID: at.ID,
		AgentType:   at,
		DisplayName: "Entry " + endpoint,
		Status:      model.StatusUnknown,
		Source:      model.SourceConfig,
		Validity:    model.Validity{LastSeen: now},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if entry.ID == "" {
		entry.ID = uuid.NewString()
	}
	return entry
}

func TestManager_Upsert_NewEntry(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	src := &mockSource{
		name:       "static",
		agentTypes: []*model.AgentType{newMockAgentType("http://new-entry.example.com")},
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
	require.NotNil(t, entries[0].AgentType)
	assert.Equal(t, "http://new-entry.example.com", entries[0].AgentType.Endpoint)
}

func TestManager_Upsert_ExistingEntry(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	existing := newMockCatalogEntry("existing-id", "http://existing.example.com")
	existing.Source = model.SourceConfig
	require.NoError(t, s.Create(ctx, existing))

	src := &mockSource{
		name:       "static",
		agentTypes: []*model.AgentType{newMockAgentType("http://existing.example.com")},
	}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	mgrCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go func() {
		_ = mgr.Run(mgrCtx)
	}()
	time.Sleep(200 * time.Millisecond)

	got, err := s.Get(ctx, existing.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	// Entry should still exist (updated, not duplicated)
	assert.Equal(t, existing.ID, got.ID)
}

func TestManager_MarksMissingEntriesDown(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer func() { _ = s.Close() }()

	ctx := context.Background()
	existing := newMockCatalogEntry("missing-id", "http://gone.example.com")
	existing.Source = model.SourceConfig
	require.NoError(t, s.Create(ctx, existing))

	// Source returns no entries this cycle
	src := &mockSource{name: "static", agentTypes: nil}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	mgrCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go func() {
		_ = mgr.Run(mgrCtx)
	}()
	time.Sleep(200 * time.Millisecond)

	got, err := s.Get(ctx, existing.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.StatusDown, got.Status)
}

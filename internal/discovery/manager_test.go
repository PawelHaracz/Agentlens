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
	name   string
	agents []*model.Agent
	err    error
}

func (m *mockSource) Name() string { return m.name }
func (m *mockSource) Discover(_ context.Context) ([]*model.Agent, error) {
	return m.agents, m.err
}

func newMockAgent(id, endpoint string) *model.Agent {
	now := time.Now().UTC()
	return &model.Agent{
		ID:        id,
		Name:      "Agent " + id,
		Protocol:  model.ProtocolA2A,
		Endpoint:  endpoint,
		Status:    model.StatusUnknown,
		Source:    model.SourceConfig,
		LastSeen:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestManager_Upsert_NewAgent(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer s.Close()

	src := &mockSource{
		name:   "static",
		agents: []*model.Agent{newMockAgent("", "http://new-agent.example.com")},
	}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go mgr.Run(ctx)
	time.Sleep(200 * time.Millisecond)

	agents, err := s.List(context.Background(), store.ListFilter{})
	require.NoError(t, err)
	assert.Len(t, agents, 1)
	assert.Equal(t, "http://new-agent.example.com", agents[0].Endpoint)
}

func TestManager_Upsert_ExistingAgent(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()
	existing := newMockAgent("existing-id", "http://existing.example.com")
	existing.Source = model.SourceConfig
	require.NoError(t, s.Create(ctx, existing))

	updated := newMockAgent("", "http://existing.example.com")
	updated.Name = "Updated Name"
	src := &mockSource{name: "static", agents: []*model.Agent{updated}}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	mgrCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go mgr.Run(mgrCtx)
	time.Sleep(200 * time.Millisecond)

	got, err := s.Get(ctx, "existing-id")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Updated Name", got.Name)
}

func TestManager_MarksMissingAgentsDown(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	require.NoError(t, err)
	defer s.Close()

	ctx := context.Background()
	existing := newMockAgent("missing-id", "http://gone.example.com")
	existing.Source = model.SourceConfig
	require.NoError(t, s.Create(ctx, existing))

	// Source returns no agents this cycle
	src := &mockSource{name: "static", agents: nil}

	mgr := discovery.NewManager([]discovery.Source{src}, s, time.Hour)
	mgrCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	go mgr.Run(mgrCtx)
	time.Sleep(200 * time.Millisecond)

	got, err := s.Get(ctx, "missing-id")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, model.StatusDown, got.Status)
}

package mcpserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
	"github.com/PawelHaracz/agentlens/plugins/mcpserver"
)

// --- stub catalog store for self-registration tests ---

type stubCatalogStore struct {
	entries map[string]*model.CatalogEntry // endpoint → entry
	created []*model.CatalogEntry
}

func newStubCatalog() *stubCatalogStore {
	return &stubCatalogStore{entries: make(map[string]*model.CatalogEntry)}
}

func (s *stubCatalogStore) Create(_ context.Context, e *model.CatalogEntry) error {
	s.created = append(s.created, e)
	if e.AgentType != nil {
		s.entries[e.AgentType.Endpoint] = e
	}
	return nil
}
func (s *stubCatalogStore) FindByEndpoint(_ context.Context, ep string) (*model.CatalogEntry, error) {
	return s.entries[ep], nil
}

// Implement remaining store.Store methods as no-ops.
func (s *stubCatalogStore) Get(_ context.Context, _ string) (*model.CatalogEntry, error) {
	return nil, nil
}
func (s *stubCatalogStore) Update(_ context.Context, _ *model.CatalogEntry) error { return nil }
func (s *stubCatalogStore) Delete(_ context.Context, _ string) error              { return nil }
func (s *stubCatalogStore) List(_ context.Context, _ store.ListFilter) ([]model.CatalogEntry, error) {
	return nil, nil
}
func (s *stubCatalogStore) UpsertProvider(_ context.Context, _ *model.Provider) (*model.Provider, error) {
	return nil, nil
}
func (s *stubCatalogStore) ListCapabilities(_ context.Context, _ store.CapabilityFilter) (*model.CapabilityListResult, error) {
	return nil, nil
}
func (s *stubCatalogStore) ListAgentsByCapability(_ context.Context, _, _ string) ([]model.CatalogEntry, error) {
	return nil, nil
}
func (s *stubCatalogStore) Stats(_ context.Context) (*store.StoreStats, error) { return nil, nil }
func (s *stubCatalogStore) UpdateHealth(_ context.Context, _ string, _ model.Health) error {
	return nil
}
func (s *stubCatalogStore) ListForProbing(_ context.Context, _ time.Time, _ int) ([]model.CatalogEntry, error) {
	return nil, nil
}
func (s *stubCatalogStore) SetLifecycle(_ context.Context, _ string, _ model.LifecycleState) error {
	return nil
}
func (s *stubCatalogStore) Close() error { return nil }

// fakeKernelWithCatalog extends fakeKernel with a catalog store.
type fakeKernelWithCatalog struct {
	fakeKernel
	catalog store.Store
}

func (k *fakeKernelWithCatalog) Store() store.Store { return k.catalog }

func initPluginWithCatalog(t *testing.T, publicURL string, catalog store.Store) *mcpserver.Plugin {
	t.Helper()
	ss := newStubStore()
	p := mcpserver.NewForTest(ss)
	k := &fakeKernelWithCatalog{
		fakeKernel: *newFakeKernel(true),
		catalog:    catalog,
	}
	k.cfg.MCP.PublicURL = publicURL
	require.NoError(t, p.Init(k))
	return p
}

func TestSelfRegistration_CatalogEntry_Created_Idempotent_ByAgentKey(t *testing.T) {
	catalog := newStubCatalog()
	initPluginWithCatalog(t, "http://agentlens.example.com/mcp", catalog)

	require.Len(t, catalog.created, 1, "exactly one entry should be created on first init")
	entry := catalog.created[0]
	assert.Equal(t, model.SourcePush, entry.Source)
	assert.NotNil(t, entry.AgentType)
	assert.Equal(t, model.ProtocolMCP, entry.AgentType.Protocol)
	assert.Contains(t, entry.AgentType.Endpoint, "agentlens:mcp-discovery:")
	assert.NotEmpty(t, entry.AgentType.AgentKey)

	// Second init with same catalog — entry already exists, no duplicate.
	initPluginWithCatalog(t, "http://agentlens.example.com/mcp", catalog)
	assert.Len(t, catalog.created, 1, "second init must not create a duplicate entry")
}

func TestSelfRegistration_MultiInstance_DisambiguatedByPublicURL(t *testing.T) {
	catalog := newStubCatalog()

	initPluginWithCatalog(t, "http://instance-a.example.com/mcp", catalog)
	initPluginWithCatalog(t, "http://instance-b.example.com/mcp", catalog)

	require.Len(t, catalog.created, 2, "two distinct instances should produce two entries")

	keyA := catalog.created[0].AgentType.AgentKey
	keyB := catalog.created[1].AgentType.AgentKey
	assert.NotEqual(t, keyA, keyB, "different public URLs must produce different AgentKeys (M6)")
}

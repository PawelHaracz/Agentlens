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
		ID:        "at-" + id,
		AgentKey:  model.ComputeAgentKey(model.ProtocolA2A, endpoint),
		Protocol:  model.ProtocolA2A,
		Endpoint:  endpoint,
		Version:   "1.0.0",
		CreatedOn: now,
	}
	return &model.CatalogEntry{
		ID:          id,
		AgentTypeID: agentType.ID,
		AgentType:   agentType,
		DisplayName: "Test Entry " + id,
		Description: "A test entry",
		Status:      model.LifecycleRegistered,
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
	a.Status = model.LifecycleActive
	a.UpdatedAt = time.Now().UTC()
	require.NoError(t, s.Update(ctx, a))

	got, err := s.Get(ctx, "entry-2")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", got.DisplayName)
	assert.Equal(t, model.LifecycleActive, got.Status)
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
	entries[2].Status = model.LifecycleActive

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
		list, err := s.List(ctx, store.ListFilter{States: []model.LifecycleState{model.LifecycleActive}})
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

// TestSearchCapabilities removed - SearchCapabilities method replaced by ListCapabilities
// See TestListCapabilities for new capability discovery tests

func TestStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	a1 := sampleEntry("s1")
	a1.Status = model.LifecycleActive
	a2 := sampleEntry("s2")
	a2.Status = model.LifecycleOffline
	a3 := sampleEntry("s3")
	a3.Status = model.LifecycleActive
	a3.Source = model.SourceK8s

	for _, e := range []*model.CatalogEntry{a1, a2, a3} {
		require.NoError(t, s.Create(ctx, e))
	}

	stats, err := s.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Total)
	assert.Equal(t, 2, stats.ByStatus["active"])
	assert.Equal(t, 1, stats.ByStatus["offline"])
}

func TestListCapabilities(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Seed two entries with same capability name but different descriptions
	entry1 := sampleEntry("entry-1")
	entry1.AgentType.Protocol = model.ProtocolA2A
	entry1.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{
			Name:        "Translate EN-DE",
			Description: "Bidirectional translation",
			Tags:        []string{"translation", "german", "english"},
			InputModes:  []string{"text/plain"},
			OutputModes: []string{"text/plain"},
		},
	}
	entry1.Status = model.LifecycleActive
	if err := s.Create(ctx, entry1); err != nil {
		t.Fatalf("Create entry1: %v", err)
	}

	entry2 := sampleEntry("entry-2")
	entry2.DisplayName = "Polyglot Agent"
	entry2.AgentType.Protocol = model.ProtocolA2A
	entry2.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{
			Name:        "Translate EN-DE",
			Description: "Translation with context",
			Tags:        []string{"translation", "german"},
			InputModes:  []string{"text/plain", "application/json"},
			OutputModes: []string{"text/plain"},
		},
	}
	entry2.Status = model.LifecycleActive
	if err := s.Create(ctx, entry2); err != nil {
		t.Fatalf("Create entry2: %v", err)
	}

	// Seed MCP tool entry
	entry3 := sampleEntry("entry-3")
	entry3.DisplayName = "DocSearch Server"
	entry3.AgentType.Protocol = model.ProtocolMCP
	entry3.AgentType.Capabilities = []model.Capability{
		&model.MCPTool{
			Name:        "search_documents",
			Description: "Full-text search",
		},
	}
	entry3.Status = model.LifecycleActive
	if err := s.Create(ctx, entry3); err != nil {
		t.Fatalf("Create entry3: %v", err)
	}

	// Seed offline entry (should be excluded from list)
	entry4 := sampleEntry("entry-4")
	entry4.AgentType.Protocol = model.ProtocolA2A
	entry4.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Offline Skill", Description: "Should not appear"},
	}
	entry4.Status = model.LifecycleOffline
	if err := s.Create(ctx, entry4); err != nil {
		t.Fatalf("Create entry4: %v", err)
	}

	t.Run("list all capabilities", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Limit: 50,
			Sort:  "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		// Should return 3 items (entry1 skill, entry2 skill, entry3 tool) — offline excluded
		if result.Total != 3 {
			t.Errorf("expected total=3, got %d", result.Total)
		}
		if len(result.Items) != 3 {
			t.Errorf("expected 3 items, got %d", len(result.Items))
		}

		// Verify first item is entry3 tool (sorted by name: search_documents < Translate)
		if result.Items[0].Kind != "mcp.tool" {
			t.Errorf("expected first item kind=mcp.tool, got %s", result.Items[0].Kind)
		}
		if result.Items[0].Name != "search_documents" {
			t.Errorf("expected first item name=search_documents, got %s", result.Items[0].Name)
		}
		if result.Items[0].AgentID != entry3.ID {
			t.Errorf("expected first item agent_id=%s, got %s", entry3.ID, result.Items[0].AgentID)
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Kind:  "a2a.skill",
			Limit: 50,
			Sort:  "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		if result.Total != 2 {
			t.Errorf("expected total=2, got %d", result.Total)
		}
		for _, item := range result.Items {
			if item.Kind != "a2a.skill" {
				t.Errorf("expected kind=a2a.skill, got %s", item.Kind)
			}
		}
	})

	t.Run("search by query", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Query: "translation",
			Limit: 50,
			Sort:  "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		// Should match both entry1 and entry2 (description + tags contain "translation")
		if result.Total != 2 {
			t.Errorf("expected total=2, got %d", result.Total)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		result, err := s.ListCapabilities(ctx, store.CapabilityFilter{
			Limit:  1,
			Offset: 1,
			Sort:   "name_asc",
		})
		if err != nil {
			t.Fatalf("ListCapabilities: %v", err)
		}

		if len(result.Items) != 1 {
			t.Errorf("expected 1 item, got %d", len(result.Items))
		}
		if result.Total != 3 {
			t.Errorf("expected total=3, got %d", result.Total)
		}
	})
}

func TestListAgentsByCapability(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Seed entries with same capability (kind, name)
	entry1 := sampleEntry("entry-1")
	entry1.DisplayName = "Translation Agent"
	entry1.AgentType.Protocol = model.ProtocolA2A
	entry1.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Translate EN-DE", Description: "Bidirectional"},
	}
	entry1.Status = model.LifecycleActive
	if err := s.Create(ctx, entry1); err != nil {
		t.Fatalf("Create entry1: %v", err)
	}

	entry2 := sampleEntry("entry-2")
	entry2.DisplayName = "Legacy Translator"
	entry2.AgentType.Protocol = model.ProtocolA2A
	entry2.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Translate EN-DE", Description: "Legacy"},
	}
	entry2.Status = model.LifecycleOffline
	if err := s.Create(ctx, entry2); err != nil {
		t.Fatalf("Create entry2: %v", err)
	}

	entry3 := sampleEntry("entry-3")
	entry3.AgentType.Protocol = model.ProtocolA2A
	entry3.AgentType.Capabilities = []model.Capability{
		&model.A2ASkill{Name: "Other Skill", Description: "Different"},
	}
	entry3.Status = model.LifecycleActive
	if err := s.Create(ctx, entry3); err != nil {
		t.Fatalf("Create entry3: %v", err)
	}

	t.Run("list agents by capability", func(t *testing.T) {
		entries, err := s.ListAgentsByCapability(ctx, "a2a.skill", "Translate EN-DE")
		if err != nil {
			t.Fatalf("ListAgentsByCapability: %v", err)
		}

		// Should return 2 entries (entry1 + entry2), including offline
		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}

		// Verify entry IDs
		ids := map[string]bool{}
		for _, e := range entries {
			ids[e.ID] = true
		}
		if !ids[entry1.ID] || !ids[entry2.ID] {
			t.Errorf("expected entries %s and %s, got %v", entry1.ID, entry2.ID, ids)
		}
	})

	t.Run("non-existent capability returns empty", func(t *testing.T) {
		entries, err := s.ListAgentsByCapability(ctx, "a2a.skill", "NonExistent")
		if err != nil {
			t.Fatalf("ListAgentsByCapability: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})
}

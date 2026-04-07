package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
	"github.com/PawelHaracz/agentlens/internal/store"
)

func TestUpdateHealth(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	ctx := context.Background()

	entry := sampleEntry("health-update-1")
	if err := s.Create(ctx, entry); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	h := model.Health{
		State:               model.LifecycleActive,
		LastProbedAt:        &now,
		LastSuccessAt:       &now,
		LastError:           "",
		LatencyMs:           88,
		ConsecutiveFailures: 0,
	}

	if err := s.UpdateHealth(ctx, entry.ID, h); err != nil {
		t.Fatalf("UpdateHealth: %v", err)
	}

	got, err := s.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get after UpdateHealth: %v", err)
	}
	if got.Status != model.LifecycleActive {
		t.Errorf("Status = %v, want active", got.Status)
	}
	if got.Health.LatencyMs != 88 {
		t.Errorf("LatencyMs = %v, want 88", got.Health.LatencyMs)
	}
	if got.HealthLastSuccessAt == nil {
		t.Error("HealthLastSuccessAt should not be nil after successful probe")
	}
	if got.Validity.LastSeen.IsZero() {
		t.Error("Validity.LastSeen should be set after successful probe")
	}
}

func TestUpdateHealthFailure(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	ctx := context.Background()

	entry := sampleEntry("health-update-fail-1")
	if err := s.Create(ctx, entry); err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Now().UTC()
	h := model.Health{
		State:               model.LifecycleDegraded,
		LastProbedAt:        &now,
		LastSuccessAt:       nil,
		LastError:           "connection refused",
		LatencyMs:           0,
		ConsecutiveFailures: 1,
	}

	if err := s.UpdateHealth(ctx, entry.ID, h); err != nil {
		t.Fatalf("UpdateHealth: %v", err)
	}

	got, err := s.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != model.LifecycleDegraded {
		t.Errorf("Status = %v, want degraded", got.Status)
	}
	if got.Health.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures = %v, want 1", got.Health.ConsecutiveFailures)
	}
}

func TestListForProbing(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	ctx := context.Background()

	// e1: never probed → should be included
	e1 := sampleEntry("probe-list-1")
	if err := s.Create(ctx, e1); err != nil {
		t.Fatalf("Create e1: %v", err)
	}

	// e2: deprecated → should be EXCLUDED
	e2 := sampleEntry("probe-list-2")
	if err := s.Create(ctx, e2); err != nil {
		t.Fatalf("Create e2: %v", err)
	}
	if err := s.SetLifecycle(ctx, e2.ID, model.LifecycleDeprecated); err != nil {
		t.Fatalf("SetLifecycle e2: %v", err)
	}

	// e3: probed recently → should be excluded
	e3 := sampleEntry("probe-list-3")
	if err := s.Create(ctx, e3); err != nil {
		t.Fatalf("Create e3: %v", err)
	}
	recentProbe := time.Now().UTC()
	if err := s.UpdateHealth(ctx, e3.ID, model.Health{
		State:        model.LifecycleActive,
		LastProbedAt: &recentProbe,
	}); err != nil {
		t.Fatalf("UpdateHealth e3: %v", err)
	}

	olderThan := time.Now().UTC().Add(-30 * time.Second)
	entries, err := s.ListForProbing(ctx, olderThan, 10)
	if err != nil {
		t.Fatalf("ListForProbing: %v", err)
	}

	ids := make(map[string]bool)
	for _, e := range entries {
		ids[e.ID] = true
	}

	if !ids["probe-list-1"] {
		t.Error("e1 (never probed) should be in ListForProbing result")
	}
	if ids["probe-list-2"] {
		t.Error("e2 (deprecated) should NOT be in ListForProbing result")
	}
	if ids["probe-list-3"] {
		t.Error("e3 (recently probed) should NOT be in ListForProbing result")
	}
}

func TestSetLifecycle(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	ctx := context.Background()

	entry := sampleEntry("lifecycle-set-1")
	if err := s.Create(ctx, entry); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.SetLifecycle(ctx, entry.ID, model.LifecycleDeprecated); err != nil {
		t.Fatalf("SetLifecycle: %v", err)
	}

	got, err := s.Get(ctx, entry.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != model.LifecycleDeprecated {
		t.Errorf("Status = %v, want deprecated", got.Status)
	}
}

func TestListFilterByStates(t *testing.T) {
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer func() {
		_ = s.Close()
	}()
	ctx := context.Background()

	active := sampleEntry("filter-active")
	offline := sampleEntry("filter-offline")
	deprecated := sampleEntry("filter-deprecated")

	for _, e := range []*model.CatalogEntry{active, offline, deprecated} {
		if err := s.Create(ctx, e); err != nil {
			t.Fatalf("Create %s: %v", e.ID, err)
		}
	}
	// Set non-registered states
	_ = s.SetLifecycle(ctx, offline.ID, model.LifecycleOffline)
	_ = s.SetLifecycle(ctx, deprecated.ID, model.LifecycleDeprecated)
	h := model.Health{State: model.LifecycleActive, LastProbedAt: func() *time.Time { t := time.Now().UTC(); return &t }()}
	_ = s.UpdateHealth(ctx, active.ID, h)

	entries, err := s.List(ctx, store.ListFilter{
		States: []model.LifecycleState{model.LifecycleActive, model.LifecycleOffline},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	ids := make(map[string]bool)
	for _, e := range entries {
		ids[e.ID] = true
	}
	if !ids["filter-active"] {
		t.Error("active entry should be in filtered result")
	}
	if !ids["filter-offline"] {
		t.Error("offline entry should be in filtered result")
	}
	if ids["filter-deprecated"] {
		t.Error("deprecated entry should NOT be in filtered result")
	}
}

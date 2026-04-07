package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

func TestCatalogEntryHealthSyncRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	entry := model.CatalogEntry{
		Status:                    model.LifecycleActive,
		HealthLastProbedAt:        &now,
		HealthLastSuccessAt:       &now,
		HealthLastError:           "",
		HealthLatencyMs:           142,
		HealthConsecutiveFailures: 0,
	}
	entry.SyncFromDB()

	if entry.Health.State != model.LifecycleActive {
		t.Errorf("Health.State = %v, want %v", entry.Health.State, model.LifecycleActive)
	}
	if entry.Health.LatencyMs != 142 {
		t.Errorf("Health.LatencyMs = %v, want 142", entry.Health.LatencyMs)
	}
}

func TestCatalogEntryMarshalJSONIncludesHealth(t *testing.T) {
	entry := model.CatalogEntry{
		ID:              "test-id",
		DisplayName:     "Test",
		Status:          model.LifecycleActive,
		Source:          model.SourcePush,
		HealthLatencyMs: 99,
	}
	entry.SyncFromDB()

	b, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["status"] != "active" {
		t.Errorf("status = %v, want active", out["status"])
	}
	health, ok := out["health"].(map[string]any)
	if !ok {
		t.Fatal("health field missing or wrong type")
	}
	if health["state"] != "active" {
		t.Errorf("health.state = %v, want active", health["state"])
	}
}

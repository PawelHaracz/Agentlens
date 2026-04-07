package store

import (
	"context"
	"fmt"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// UpdateHealth persists a health probe result. Updates status, all health_* columns,
// and validity_last_seen when the probe succeeded.
func (s *SQLStore) UpdateHealth(ctx context.Context, entryID string, h model.Health) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":                      string(h.State),
		"health_last_probed_at":       h.LastProbedAt,
		"health_last_success_at":      h.LastSuccessAt,
		"health_last_error":           h.LastError,
		"health_latency_ms":           h.LatencyMs,
		"health_consecutive_failures": h.ConsecutiveFailures,
		"updated_at":                  now,
	}
	if h.LastSuccessAt != nil {
		updates["validity_last_seen"] = *h.LastSuccessAt
	}
	result := s.gdb.WithContext(ctx).
		Model(&model.CatalogEntry{}).
		Where("id = ?", entryID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("updating health for %s: %w", entryID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("updating health for %s: %w", entryID, model.ErrEntryNotFound)
	}
	return nil
}

// SetLifecycle updates only the lifecycle state of an entry (used by admin lifecycle API).
func (s *SQLStore) SetLifecycle(ctx context.Context, entryID string, state model.LifecycleState) error {
	result := s.gdb.WithContext(ctx).
		Model(&model.CatalogEntry{}).
		Where("id = ?", entryID).
		Updates(map[string]interface{}{
			"status":     string(state),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("setting lifecycle for %s: %w", entryID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("entry %s not found", entryID)
	}
	return nil
}

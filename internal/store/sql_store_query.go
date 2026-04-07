package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// List returns catalog entries matching the given filter.
func (s *SQLStore) List(ctx context.Context, filter ListFilter) ([]model.CatalogEntry, error) {
	query := s.gdb.WithContext(ctx).
		Model(&model.CatalogEntry{}).
		Preload("AgentType").
		Preload("AgentType.Provider").
		Joins("JOIN agent_types ON agent_types.id = catalog_entries.agent_type_id")

	if filter.Protocol != nil {
		query = query.Where("agent_types.protocol = ?", string(*filter.Protocol))
	}
	if len(filter.States) > 0 {
		states := make([]string, len(filter.States))
		for i, s := range filter.States {
			states[i] = string(s)
		}
		query = query.Where("catalog_entries.status IN ?", states)
	}
	if filter.Source != nil {
		query = query.Where("catalog_entries.source = ?", string(*filter.Source))
	}
	if filter.Team != "" {
		// Join providers to filter by team name.
		query = query.
			Joins("LEFT JOIN providers ON providers.id = agent_types.provider_id").
			Where("providers.team LIKE ?", "%"+filter.Team+"%")
	}
	if filter.Query != "" {
		q := "%" + filter.Query + "%"
		query = query.Where("catalog_entries.display_name LIKE ? OR catalog_entries.description LIKE ?", q, q)
	}
	for _, cat := range filter.Categories {
		query = query.Where("catalog_entries.categories LIKE ?", "%"+cat+"%")
	}

	query = query.Order("catalog_entries.display_name")

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var entries []model.CatalogEntry
	if err := query.Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("listing catalog entries: %w", err)
	}
	for i := range entries {
		if err := s.loadCapabilities(ctx, entries[i].AgentType); err != nil {
			return nil, err
		}
		entries[i].SyncFromDB()
	}
	return entries, nil
}

// SearchCapabilities returns catalog entries whose capabilities match the query string.
func (s *SQLStore) SearchCapabilities(ctx context.Context, query string) ([]model.CatalogEntry, error) {
	like := "%" + query + "%"

	var entries []model.CatalogEntry
	err := s.gdb.WithContext(ctx).
		Model(&model.CatalogEntry{}).
		Preload("AgentType").
		Preload("AgentType.Provider").
		Joins("JOIN agent_types ON agent_types.id = catalog_entries.agent_type_id").
		Joins("JOIN capabilities ON capabilities.agent_type_id = agent_types.id").
		Where("capabilities.name LIKE ? OR capabilities.description LIKE ?", like, like).
		Distinct("catalog_entries.*").
		Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("searching capabilities: %w", err)
	}
	for i := range entries {
		if err := s.loadCapabilities(ctx, entries[i].AgentType); err != nil {
			return nil, err
		}
		entries[i].SyncFromDB()
	}
	return entries, nil
}

// ListForProbing returns entries due for a health probe. Entries are excluded
// if deprecated or if last probed after olderThan. Results are ordered with
// never-probed entries first, capped by limit.
func (s *SQLStore) ListForProbing(ctx context.Context, olderThan time.Time, limit int) ([]model.CatalogEntry, error) {
	var entries []model.CatalogEntry
	err := s.gdb.WithContext(ctx).
		Model(&model.CatalogEntry{}).
		Preload("AgentType").
		Joins("JOIN agent_types ON agent_types.id = catalog_entries.agent_type_id").
		Where(
			"catalog_entries.status != ? AND (catalog_entries.health_last_probed_at IS NULL OR catalog_entries.health_last_probed_at < ?)",
			string(model.LifecycleDeprecated),
			olderThan,
		).
		Order("catalog_entries.health_last_probed_at NULLS FIRST").
		Limit(limit).
		Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("listing for probing: %w", err)
	}
	for i := range entries {
		entries[i].SyncFromDB()
	}
	return entries, nil
}

// Stats returns aggregate statistics about catalog entries in the store.
func (s *SQLStore) Stats(ctx context.Context) (*StoreStats, error) {
	stats := &StoreStats{
		ByStatus: make(map[string]int),
		BySource: make(map[string]int),
	}

	var total int64
	if err := s.gdb.WithContext(ctx).Model(&model.CatalogEntry{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("counting catalog entries: %w", err)
	}
	stats.Total = int(total)

	type groupResult struct {
		Key   string
		Count int
	}

	var statusCounts []groupResult
	if err := s.gdb.WithContext(ctx).Model(&model.CatalogEntry{}).
		Select("status as key, count(*) as count").
		Group("status").Find(&statusCounts).Error; err != nil {
		return nil, fmt.Errorf("counting by status: %w", err)
	}
	for _, r := range statusCounts {
		stats.ByStatus[strings.ToLower(r.Key)] = r.Count
	}

	var sourceCounts []groupResult
	if err := s.gdb.WithContext(ctx).Model(&model.CatalogEntry{}).
		Select("source as key, count(*) as count").
		Group("source").Find(&sourceCounts).Error; err != nil {
		return nil, fmt.Errorf("counting by source: %w", err)
	}
	for _, r := range sourceCounts {
		stats.BySource[strings.ToLower(r.Key)] = r.Count
	}

	return stats, nil
}

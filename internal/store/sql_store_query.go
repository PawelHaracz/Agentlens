package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// List returns catalog entries matching the given filter.
func (s *SQLStore) List(ctx context.Context, filter ListFilter) ([]model.CatalogEntry, error) {
	query := s.gdb.WithContext(ctx).Model(&model.CatalogEntry{})

	if filter.Protocol != nil {
		query = query.Where("protocol = ?", string(*filter.Protocol))
	}
	if filter.Status != nil {
		query = query.Where("status = ?", string(*filter.Status))
	}
	if filter.Source != nil {
		query = query.Where("source = ?", string(*filter.Source))
	}
	if filter.Team != "" {
		query = query.Where("provider LIKE ?", "%"+filter.Team+"%")
	}
	if filter.Query != "" {
		q := "%" + filter.Query + "%"
		query = query.Where("display_name LIKE ? OR description LIKE ?", q, q)
	}
	for _, cat := range filter.Categories {
		query = query.Where("categories LIKE ?", "%"+cat+"%")
	}

	query = query.Order("display_name")

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
		entries[i].SyncFromDB()
	}
	return entries, nil
}

// SearchSkills returns catalog entries whose skills match the query string.
func (s *SQLStore) SearchSkills(ctx context.Context, query string) ([]model.CatalogEntry, error) {
	var entries []model.CatalogEntry
	result := s.gdb.WithContext(ctx).Where("skills LIKE ?", "%"+query+"%").Find(&entries)
	if result.Error != nil {
		return nil, fmt.Errorf("searching skills: %w", result.Error)
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

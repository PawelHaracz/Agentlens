package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PawelHaracz/agentlens/internal/model"
	"gorm.io/gorm"
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
		q := "%" + strings.ToLower(filter.Query) + "%"
		// Join capabilities for skill-level search.
		// Join providers only when the Team filter hasn't already done so, to avoid a duplicate JOIN.
		if filter.Team == "" {
			query = query.Joins("LEFT JOIN providers ON providers.id = agent_types.provider_id")
		}
		query = query.
			Joins("LEFT JOIN capabilities ON capabilities.agent_type_id = agent_types.id").
			Where(
				"LOWER(catalog_entries.display_name) LIKE ? OR "+
					"LOWER(catalog_entries.description) LIKE ? OR "+
					"LOWER(capabilities.name) LIKE ? OR "+
					"LOWER(capabilities.description) LIKE ? OR "+
					"LOWER(catalog_entries.categories) LIKE ? OR "+
					"LOWER(providers.organization) LIKE ?",
				q, q, q, q, q, q,
			).
			Distinct("catalog_entries.*")
	}
	for _, cat := range filter.Categories {
		query = query.Where("catalog_entries.categories LIKE ?", "%"+cat+"%")
	}
	query = applyProjectFilter(query, filter)

	query = applyListSort(query, filter.Sort)
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

func applyListSort(query *gorm.DB, sort string) *gorm.DB {
	switch sort {
	case "displayName_asc":
		return query.Order("catalog_entries.display_name ASC")
	case "createdAt_desc":
		return query.Order("catalog_entries.created_at DESC")
	default:
		return query.Order("catalog_entries.health_last_success_at DESC NULLS LAST, catalog_entries.display_name ASC")
	}
}

// applyProjectFilter adds catalog_project_memberships JOIN for project scoping.
// ProjectIDs (multi-value, ctx-injected) takes precedence over ProjectID (single).
func applyProjectFilter(q *gorm.DB, filter ListFilter) *gorm.DB {
	if len(filter.ProjectIDs) > 0 {
		return q.
			Joins("JOIN catalog_project_memberships cpm ON cpm.catalog_entry_id = catalog_entries.id").
			Where("cpm.project_party_id IN ?", filter.ProjectIDs).
			Distinct("catalog_entries.*")
	}
	if filter.ProjectID != "" {
		return q.
			Joins("JOIN catalog_project_memberships cpm ON cpm.catalog_entry_id = catalog_entries.id").
			Where("cpm.project_party_id = ?", filter.ProjectID)
	}
	return q
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

// capabilityInstanceRow is a flat scan target for the ListCapabilities query.
type capabilityInstanceRow struct {
	Kind        string
	Name        string
	Description string
	Properties  string
	AgentID     string
	AgentName   string
	Protocol    string
	SpecVersion string
	Status      string
	ProviderOrg *string
	ProviderURL *string
	LatencyMs   int64
	HealthState string
}

// toCapabilityInstance converts a raw DB row to a CapabilityInstance,
// including parsing skill-specific fields from the properties JSON column.
func toCapabilityInstance(row capabilityInstanceRow) model.CapabilityInstance {
	inst := model.CapabilityInstance{
		Kind:        row.Kind,
		Name:        row.Name,
		Description: row.Description,
		AgentID:     row.AgentID,
		AgentName:   row.AgentName,
		Protocol:    model.Protocol(row.Protocol),
		Status:      model.LifecycleState(row.Status),
		SpecVersion: row.SpecVersion,
		HealthState: model.LifecycleState(row.HealthState),
		LatencyMs:   row.LatencyMs,
	}
	if row.ProviderOrg != nil {
		inst.ProviderOrg = row.ProviderOrg
	}
	if row.ProviderURL != nil {
		inst.ProviderURL = row.ProviderURL
	}
	if row.Kind == "a2a.skill" && row.Properties != "" {
		enrichA2ASkillFields(&inst, row.Properties)
	}
	return inst
}

// enrichA2ASkillFields parses tags, inputModes, outputModes from a JSON properties string.
func enrichA2ASkillFields(inst *model.CapabilityInstance, properties string) {
	var props map[string]any
	if err := json.Unmarshal([]byte(properties), &props); err != nil {
		return
	}
	inst.Tags = append(inst.Tags, anyStrings(props["tags"])...)
	inst.InputModes = append(inst.InputModes, anyStrings(props["inputModes"])...)
	inst.OutputModes = append(inst.OutputModes, anyStrings(props["outputModes"])...)
}

// anyStrings safely casts a []any to []string, skipping non-string elements.
func anyStrings(v any) []string {
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(slice))
	for _, el := range slice {
		if s, ok := el.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ListCapabilities returns a flat list of capability instances with agent metadata.
// Only active + degraded catalog entries are included.
// Only discoverable (user-facing) capability kinds are returned.
func (s *SQLStore) ListCapabilities(ctx context.Context, filter CapabilityFilter) (*model.CapabilityListResult, error) {
	discoverableKinds := model.DiscoverableKinds()
	if len(discoverableKinds) == 0 {
		return &model.CapabilityListResult{Total: 0, Items: []model.CapabilityInstance{}}, nil
	}

	query := s.gdb.WithContext(ctx).
		Table("capabilities c").
		Select(`c.kind, c.name, c.description, c.properties,
			ce.id AS agent_id, ce.display_name AS agent_name,
			at.protocol, at.spec_version,
			ce.status,
			p.organization AS provider_org, p.url AS provider_url,
			ce.health_latency_ms AS latency_ms, ce.status AS health_state`).
		Joins("JOIN agent_types at ON c.agent_type_id = at.id").
		Joins("JOIN catalog_entries ce ON ce.agent_type_id = at.id").
		Joins("LEFT JOIN providers p ON at.provider_id = p.id").
		Where("c.kind IN ?", discoverableKinds).
		Where("ce.status IN ?", []string{string(model.LifecycleActive), string(model.LifecycleDegraded)})

	if filter.Query != "" {
		lq := "%" + strings.ToLower(filter.Query) + "%"
		query = query.Where(
			"LOWER(c.name) LIKE ? OR LOWER(c.description) LIKE ? OR LOWER(c.properties) LIKE ?",
			lq, lq, lq,
		)
	}
	if filter.Kind != "" {
		query = query.Where("c.kind = ?", filter.Kind)
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count capabilities: %w", err)
	}

	orderClause := "LOWER(c.name) ASC, LOWER(ce.display_name) ASC"
	if filter.Sort == "agentName_asc" {
		orderClause = "LOWER(ce.display_name) ASC, LOWER(c.name) ASC"
	}
	query = query.Order(orderClause)
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var rows []capabilityInstanceRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query capabilities: %w", err)
	}

	items := make([]model.CapabilityInstance, len(rows))
	for i, row := range rows {
		items[i] = toCapabilityInstance(row)
	}
	return &model.CapabilityListResult{Total: int(total), Items: items}, nil
}

// ListAgentsByCapability returns catalog entries offering a specific capability
// identified by (kind, name). Returns all lifecycle states.
func (s *SQLStore) ListAgentsByCapability(ctx context.Context, kind, name string) ([]model.CatalogEntry, error) {
	var entries []model.CatalogEntry

	err := s.gdb.WithContext(ctx).
		Joins("JOIN agent_types ON catalog_entries.agent_type_id = agent_types.id").
		Joins("JOIN capabilities ON capabilities.agent_type_id = agent_types.id").
		Where("capabilities.kind = ? AND capabilities.name = ?", kind, name).
		Preload("AgentType").
		Preload("AgentType.Provider").
		Find(&entries).Error

	if err != nil {
		return nil, fmt.Errorf("query agents by capability: %w", err)
	}

	// Load capabilities for each entry
	for i := range entries {
		if err := s.loadCapabilities(ctx, entries[i].AgentType); err != nil {
			return nil, fmt.Errorf("load capabilities for agent %s: %w", entries[i].ID, err)
		}
		entries[i].SyncFromDB()
	}

	return entries, nil
}

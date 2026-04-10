package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// capabilityRow is a concrete GORM struct for the capabilities table.
type capabilityRow struct {
	ID          string `gorm:"primaryKey;type:text"`
	AgentTypeID string `gorm:"not null;type:text;index"`
	Kind        string `gorm:"not null;type:text"`
	Name        string `gorm:"not null;type:text"`
	Description string `gorm:"type:text;default:''"`
	Properties  string `gorm:"type:text;not null;default:'{}'"`
}

func (capabilityRow) TableName() string { return "capabilities" }

// SQLStore is a GORM-backed implementation of Store.
type SQLStore struct {
	gdb *db.DB
}

// NewSQLStore creates a new SQLStore from an existing GORM DB wrapper.
func NewSQLStore(database *db.DB) *SQLStore {
	return &SQLStore{gdb: database}
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs migrations.
// This is a backward-compatible constructor.
func NewSQLiteStore(path string) (*SQLStore, error) {
	var database *db.DB
	var err error

	if path == ":memory:" {
		database, err = db.OpenMemory()
	} else {
		database, err = db.Open(db.DialectSQLite, path)
	}
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}

	// Run migrations to create all required tables.
	migrator := db.NewMigrator(database, db.AllMigrations())
	if err := migrator.Migrate(context.Background()); err != nil {
		sqlDB, _ := database.DB.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &SQLStore{gdb: database}, nil
}

// Close closes the underlying database connection.
func (s *SQLStore) Close() error {
	sqlDB, err := s.gdb.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// capabilityToRow converts a model.Capability to a capabilityRow for DB storage.
func capabilityToRow(agentTypeID string, cap model.Capability) (capabilityRow, error) {
	data, err := json.Marshal(cap)
	if err != nil {
		return capabilityRow{}, fmt.Errorf("marshal capability: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return capabilityRow{}, fmt.Errorf("re-parse capability: %w", err)
	}

	name, _ := m["name"].(string)
	desc, _ := m["description"].(string)

	// For capabilities that have no "name" field, derive a unique name from
	// other identifying fields so the (agent_type_id, kind, name) uniqueness
	// constraint is satisfied even when multiple same-kind caps are stored.
	if name == "" {
		for _, fallback := range []string{"scheme_name", "uri", "url", "type", "algorithm"} {
			if v, ok := m[fallback].(string); ok && v != "" {
				name = v
				break
			}
		}
	}
	// For capabilities that carry a "schemes" map (e.g. a2a.security_requirement),
	// derive a stable name from the sorted scheme names so that multiple
	// same-kind capabilities on the same agent_type satisfy the unique constraint.
	if name == "" {
		if schemesRaw, ok := m["schemes"]; ok {
			if schemesMap, ok := schemesRaw.(map[string]any); ok && len(schemesMap) > 0 {
				keys := make([]string, 0, len(schemesMap))
				for k := range schemesMap {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				name = strings.Join(keys, "+")
			}
		}
	}

	// Remove common fields from properties so they are not duplicated.
	delete(m, "name")
	delete(m, "description")

	props, err := json.Marshal(m)
	if err != nil {
		return capabilityRow{}, fmt.Errorf("marshal properties: %w", err)
	}

	return capabilityRow{
		ID:          uuid.NewString(),
		AgentTypeID: agentTypeID,
		Kind:        cap.Kind(),
		Name:        name,
		Description: desc,
		Properties:  string(props),
	}, nil
}

// rowToCapability reconstructs a model.Capability from a capabilityRow.
func rowToCapability(row capabilityRow) (model.Capability, error) {
	factory, ok := model.GetCapabilityFactory(row.Kind)
	if !ok {
		return nil, fmt.Errorf("unknown capability kind: %s", row.Kind)
	}
	cap := factory()

	var props map[string]any
	if err := json.Unmarshal([]byte(row.Properties), &props); err != nil || props == nil {
		props = map[string]any{}
	}
	props["name"] = row.Name
	props["description"] = row.Description

	data, err := json.Marshal(props)
	if err != nil {
		return nil, fmt.Errorf("marshal merged props: %w", err)
	}
	if err := json.Unmarshal(data, cap); err != nil {
		return nil, fmt.Errorf("unmarshal into capability kind %q: %w", row.Kind, err)
	}
	return cap, nil
}

// loadCapabilities queries the capabilities table and attaches them to the AgentType.
func (s *SQLStore) loadCapabilities(ctx context.Context, agentType *model.AgentType) error {
	if agentType == nil {
		return nil
	}
	var rows []capabilityRow
	if err := s.gdb.WithContext(ctx).
		Where("agent_type_id = ?", agentType.ID).
		Find(&rows).Error; err != nil {
		return fmt.Errorf("loading capabilities: %w", err)
	}
	caps := make([]model.Capability, 0, len(rows))
	for _, row := range rows {
		cap, err := rowToCapability(row)
		if err != nil {
			// Unknown kinds are skipped gracefully.
			continue
		}
		caps = append(caps, cap)
	}
	agentType.Capabilities = caps
	return nil
}

// Create inserts a new catalog entry with its AgentType and Capabilities.
func (s *SQLStore) Create(ctx context.Context, entry *model.CatalogEntry) error {
	if entry.AgentType == nil {
		return fmt.Errorf("entry.AgentType must be set before Create")
	}

	entry.SyncToDB()

	return s.gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Create AgentType.
		if err := tx.Create(entry.AgentType).Error; err != nil {
			return fmt.Errorf("creating agent_type: %w", err)
		}

		// 2. Convert capabilities to rows and batch insert.
		if len(entry.AgentType.Capabilities) > 0 {
			rows := make([]capabilityRow, 0, len(entry.AgentType.Capabilities))
			for _, cap := range entry.AgentType.Capabilities {
				row, err := capabilityToRow(entry.AgentType.ID, cap)
				if err != nil {
					return fmt.Errorf("converting capability: %w", err)
				}
				rows = append(rows, row)
			}
			if err := tx.Create(&rows).Error; err != nil {
				return fmt.Errorf("inserting capabilities: %w", err)
			}
		}

		// 3. Set FK and create CatalogEntry.
		entry.AgentTypeID = entry.AgentType.ID
		if err := tx.Create(entry).Error; err != nil {
			return fmt.Errorf("inserting catalog entry: %w", err)
		}
		return nil
	})
}

// Get retrieves a catalog entry by ID, with AgentType, Provider, and Capabilities.
func (s *SQLStore) Get(ctx context.Context, id string) (*model.CatalogEntry, error) {
	var entry model.CatalogEntry
	result := s.gdb.WithContext(ctx).
		Preload("AgentType").
		Preload("AgentType.Provider").
		First(&entry, "catalog_entries.id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting catalog entry: %w", result.Error)
	}
	if err := s.loadCapabilities(ctx, entry.AgentType); err != nil {
		return nil, err
	}
	entry.SyncFromDB()
	return &entry, nil
}

// Update modifies an existing catalog entry (and its AgentType capabilities).
func (s *SQLStore) Update(ctx context.Context, entry *model.CatalogEntry) error {
	entry.SyncToDB()

	return s.gdb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update CatalogEntry fields.
		if err := tx.Save(entry).Error; err != nil {
			return fmt.Errorf("updating catalog entry: %w", err)
		}

		if entry.AgentType != nil {
			// Update AgentType fields.
			if err := tx.Save(entry.AgentType).Error; err != nil {
				return fmt.Errorf("updating agent_type: %w", err)
			}

			// Replace capabilities: delete old, insert new.
			if err := tx.Where("agent_type_id = ?", entry.AgentType.ID).
				Delete(&capabilityRow{}).Error; err != nil {
				return fmt.Errorf("deleting old capabilities: %w", err)
			}

			if len(entry.AgentType.Capabilities) > 0 {
				rows := make([]capabilityRow, 0, len(entry.AgentType.Capabilities))
				for _, cap := range entry.AgentType.Capabilities {
					row, err := capabilityToRow(entry.AgentType.ID, cap)
					if err != nil {
						return fmt.Errorf("converting capability: %w", err)
					}
					rows = append(rows, row)
				}
				if err := tx.Create(&rows).Error; err != nil {
					return fmt.Errorf("inserting updated capabilities: %w", err)
				}
			}
		}
		return nil
	})
}

// Delete removes a catalog entry by ID.
// Capabilities are removed via ON DELETE CASCADE on the AgentType → capabilities FK.
func (s *SQLStore) Delete(ctx context.Context, id string) error {
	result := s.gdb.WithContext(ctx).Delete(&model.CatalogEntry{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting catalog entry: %w", result.Error)
	}
	return nil
}

// FindByEndpoint returns the catalog entry whose AgentType has the given endpoint URL.
func (s *SQLStore) FindByEndpoint(ctx context.Context, endpoint string) (*model.CatalogEntry, error) {
	var entry model.CatalogEntry
	result := s.gdb.WithContext(ctx).
		Preload("AgentType").
		Preload("AgentType.Provider").
		Joins("JOIN agent_types ON agent_types.id = catalog_entries.agent_type_id").
		Where("agent_types.endpoint = ?", endpoint).
		First(&entry)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("finding by endpoint: %w", result.Error)
	}
	if err := s.loadCapabilities(ctx, entry.AgentType); err != nil {
		return nil, err
	}
	entry.SyncFromDB()
	return &entry, nil
}

package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// AllMigrations returns all registered migrations in order.
func AllMigrations() []Migration {
	return []Migration{
		migration001CreateTables(),
		migration002UsersAndRoles(),
		migration003DefaultRoles(),
		migration004Settings(),
		migration005HealthColumns(),
		migration006RawCards(),
	}
}

func migration001CreateTables() Migration {
	return Migration{
		Version:     1,
		Description: "create providers, agent_types, capabilities, and catalog_entries tables",
		Up: func(tx *gorm.DB) error {
			// capabilityRow is a concrete struct for GORM (interface can't be automigrated).
			type capabilityRow struct {
				ID          string `gorm:"primaryKey;type:text"`
				AgentTypeID string `gorm:"not null;type:text;index"`
				Kind        string `gorm:"not null;type:text;index"`
				Name        string `gorm:"not null;type:text"`
				Description string `gorm:"type:text;default:''"`
				Properties  string `gorm:"type:text;not null;default:'{}'"`
			}

			// Order matters for FK constraints.
			if err := tx.AutoMigrate(&model.Provider{}); err != nil {
				return err
			}
			if err := tx.AutoMigrate(&model.AgentType{}); err != nil {
				return err
			}
			if err := tx.Table("capabilities").AutoMigrate(&capabilityRow{}); err != nil {
				return err
			}
			if err := tx.AutoMigrate(&model.CatalogEntry{}); err != nil {
				return err
			}

			// Unique constraints.
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_types_key_version ON agent_types(agent_key, version)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_capabilities_type_kind_name ON capabilities(agent_type_id, kind, name)").Error; err != nil {
				return err
			}
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_providers_org_team ON providers(organization, team)").Error; err != nil {
				return err
			}
			return nil
		},
	}
}

func migration002UsersAndRoles() Migration {
	return Migration{
		Version:     2,
		Description: "create roles and users tables",
		Up: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&model.Role{}); err != nil {
				return err
			}
			return tx.AutoMigrate(&model.User{})
		},
	}
}

// migrationRole is a simplified model for seeding default roles.
type migrationRole struct {
	ID          string         `gorm:"primaryKey;type:text"`
	Name        string         `gorm:"uniqueIndex;not null;type:text"`
	Description string         `gorm:"type:text"`
	Permissions migrationPerms `gorm:"type:text;not null;default:'[]'"`
	IsSystem    bool           `gorm:"default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (migrationRole) TableName() string { return "roles" }

// migrationPerms implements driver.Valuer/sql.Scanner for []string JSON storage.
type migrationPerms []string

func (p migrationPerms) Value() (driver.Value, error) {
	if p == nil {
		return "[]", nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshaling permissions: %w", err)
	}
	return string(b), nil
}

func (p *migrationPerms) Scan(value interface{}) error {
	if value == nil {
		*p = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("migrationPerms.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, p)
}

func migration003DefaultRoles() Migration {
	return Migration{
		Version:     3,
		Description: "insert default roles",
		Up: func(tx *gorm.DB) error {
			now := time.Now().UTC()
			roles := []migrationRole{
				{
					ID:          "role-admin",
					Name:        "admin",
					Description: "Full system access",
					Permissions: migrationPerms{
						"catalog:read", "catalog:write", "catalog:delete",
						"users:read", "users:write", "users:delete",
						"roles:read", "roles:write",
						"settings:read", "settings:write",
					},
					IsSystem:  true,
					CreatedAt: now,
					UpdatedAt: now,
				},
				{
					ID:          "role-editor",
					Name:        "editor",
					Description: "Can manage catalog entries",
					Permissions: migrationPerms{
						"catalog:read", "catalog:write",
						"users:read",
						"roles:read",
						"settings:read",
					},
					IsSystem:  true,
					CreatedAt: now,
					UpdatedAt: now,
				},
				{
					ID:          "role-viewer",
					Name:        "viewer",
					Description: "Read-only access",
					Permissions: migrationPerms{
						"catalog:read",
						"users:read",
						"roles:read",
						"settings:read",
					},
					IsSystem:  true,
					CreatedAt: now,
					UpdatedAt: now,
				},
			}
			for _, r := range roles {
				// ON CONFLICT DO NOTHING: skip if role with this ID already exists.
				result := tx.Where("id = ?", r.ID).FirstOrCreate(&r)
				if result.Error != nil {
					return result.Error
				}
			}
			return nil
		},
	}
}

func migration004Settings() Migration {
	return Migration{
		Version:     4,
		Description: "create settings table and insert defaults",
		Up: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&model.Setting{}); err != nil {
				return err
			}
			now := time.Now().UTC()
			defaults := []model.Setting{
				{Key: "app.name", Value: "AgentLens", Category: "general", Description: "Application display name", UpdatedAt: now},
				{Key: "app.registration_enabled", Value: "true", Category: "auth", Description: "Allow new user registration", UpdatedAt: now},
				{Key: "app.default_role", Value: "viewer", Category: "auth", Description: "Default role for new users", UpdatedAt: now},
			}
			for _, s := range defaults {
				result := tx.Where("key = ?", s.Key).FirstOrCreate(&s)
				if result.Error != nil {
					return result.Error
				}
			}
			return nil
		},
	}
}

func migration005HealthColumns() Migration {
	return Migration{
		Version:     5,
		Description: "add health check columns to catalog_entries",
		Up: func(tx *gorm.DB) error {
			// AutoMigrate adds new columns declared on CatalogEntry (idempotent).
			if err := tx.AutoMigrate(&model.CatalogEntry{}); err != nil {
				return fmt.Errorf("automigrate catalog_entries: %w", err)
			}

			// Map existing old status values to the new lifecycle vocabulary.
			// 'healthy' → 'active', 'down' → 'offline', 'unknown' → 'registered'.
			// 'degraded' is the same string in both old and new; no update needed.
			mappings := [][2]string{
				{"healthy", "active"},
				{"down", "offline"},
				{"unknown", "registered"},
			}
			for _, m := range mappings {
				if err := tx.Exec(
					"UPDATE catalog_entries SET status = ? WHERE status = ?",
					m[1], m[0],
				).Error; err != nil {
					return fmt.Errorf("migrating status value %q: %w", m[0], err)
				}
			}

			// Create index on health_last_probed_at for efficient ListForProbing queries.
			if err := tx.Exec(
				"CREATE INDEX IF NOT EXISTS idx_catalog_entries_health_probed_at " +
					"ON catalog_entries(health_last_probed_at)",
			).Error; err != nil {
				return fmt.Errorf("creating health_probed_at index: %w", err)
			}

			return nil
		},
	}
}

func migration006RawCards() Migration {
	return Migration{
		Version:     6,
		Description: "create raw_cards table, copy raw_definition data, drop raw_definition column",
		Up: func(tx *gorm.DB) error {
			// Step 1: Create raw_cards table if not already present.
			if err := tx.Exec(`CREATE TABLE IF NOT EXISTS raw_cards (
				agent_type_id TEXT PRIMARY KEY,
				data BLOB NOT NULL,
				content_type TEXT NOT NULL DEFAULT 'application/json',
				fetched_at DATETIME NOT NULL,
				truncated BOOLEAN NOT NULL DEFAULT FALSE
			)`).Error; err != nil {
				return fmt.Errorf("creating raw_cards table: %w", err)
			}

			// Step 2: Copy existing raw_definition bytes into raw_cards only if
			// the raw_definition column still exists (it won't on fresh databases
			// created after the column was removed from the AgentType struct).
			colExists, err := columnExists(tx, "agent_types", "raw_definition")
			if err != nil {
				return fmt.Errorf("checking raw_definition column: %w", err)
			}
			if colExists {
				var insertSQL string
				if tx.Dialector.Name() == "postgres" {
					insertSQL = `
						INSERT INTO raw_cards (agent_type_id, data, content_type, fetched_at, truncated)
						SELECT id, raw_definition, 'application/json', NOW(), FALSE
						FROM agent_types
						WHERE raw_definition IS NOT NULL AND octet_length(raw_definition) > 0
						ON CONFLICT (agent_type_id) DO NOTHING`
				} else {
					insertSQL = `
						INSERT OR IGNORE INTO raw_cards (agent_type_id, data, content_type, fetched_at, truncated)
						SELECT id, raw_definition, 'application/json', CURRENT_TIMESTAMP, FALSE
						FROM agent_types
						WHERE raw_definition IS NOT NULL AND length(raw_definition) > 0`
				}
				if err := tx.Exec(insertSQL).Error; err != nil {
					return fmt.Errorf("copying raw_definition to raw_cards: %w", err)
				}

				// Step 3: Drop raw_definition column from agent_types.
				// Postgres supports DROP COLUMN IF EXISTS unconditionally.
				// SQLite 3.35+ supports it; older SQLite versions do not.
				if err := tx.Exec(`ALTER TABLE agent_types DROP COLUMN IF EXISTS raw_definition`).Error; err != nil {
					if tx.Dialector.Name() != "postgres" {
						// Old SQLite doesn't support DROP COLUMN — leave as dead column.
						slog.Warn("could not drop raw_definition column (old SQLite?), leaving as dead column", "err", err)
					} else {
						return fmt.Errorf("dropping raw_definition column: %w", err)
					}
				}
			}

			return nil
		},
	}
}

// columnExists reports whether a column exists in a given table.
func columnExists(db *gorm.DB, table, column string) (bool, error) {
	switch db.Dialector.Name() {
	case "postgres":
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?",
			table, column,
		).Scan(&count).Error
		return count > 0, err
	default:
		// SQLite: use PRAGMA table_info
		type colInfo struct {
			Name string
		}
		var cols []colInfo
		if err := db.Raw("PRAGMA table_info(" + table + ")").Scan(&cols).Error; err != nil {
			return false, err
		}
		for _, c := range cols {
			if c.Name == column {
				return true, nil
			}
		}
		return false, nil
	}
}

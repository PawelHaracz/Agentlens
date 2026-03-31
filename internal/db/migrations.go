package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// AllMigrations returns all registered migrations in order.
func AllMigrations() []Migration {
	return []Migration{
		migration001CatalogEntries(),
		migration002UsersAndRoles(),
		migration003DefaultRoles(),
		migration004Settings(),
	}
}

// catalogEntry is the GORM model for migration 001.
// It mirrors the CatalogEntry schema used by the store.
type catalogEntry struct {
	ID          string     `gorm:"primaryKey;type:text;column:id"`
	DisplayName string     `gorm:"not null;type:text;column:display_name"`
	Description string     `gorm:"type:text;default:'';column:description"`
	Protocol    string     `gorm:"not null;type:text;column:protocol;index"`
	Endpoint    string     `gorm:"uniqueIndex;type:text;column:endpoint"`
	Version     string     `gorm:"type:text;default:'';column:version"`
	Status      string     `gorm:"not null;type:text;default:'unknown';column:status;index"`
	Source      string     `gorm:"not null;type:text;column:source;index"`
	Provider    string     `gorm:"not null;type:text;default:'{}';column:provider;index"`
	Categories  string     `gorm:"not null;type:text;default:'[]';column:categories"`
	Skills      string     `gorm:"not null;type:text;default:'[]';column:skills"`
	ValidFrom   *time.Time `gorm:"column:validity_from"`
	ValidTo     *time.Time `gorm:"column:validity_to"`
	LastSeen    time.Time  `gorm:"not null;column:validity_last_seen"`
	RawCard     *string    `gorm:"type:text;column:raw_card"`
	Metadata    string     `gorm:"not null;type:text;default:'{}';column:metadata"`
	CreatedAt   time.Time  `gorm:"not null;column:created_at"`
	UpdatedAt   time.Time  `gorm:"not null;column:updated_at"`
}

func (catalogEntry) TableName() string { return "catalog_entries" }

func migration001CatalogEntries() Migration {
	return Migration{
		Version:     1,
		Description: "create catalog_entries table",
		Up: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&catalogEntry{})
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

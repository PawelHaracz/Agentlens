package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
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
		migration007PartyArchetype(),
		migration008BackfillPersonParties(),
		migration009CatalogProjectMembershipIndexes(),
		migration010MCPDiscovery(),
	}
}

// migration009CatalogProjectMembershipIndexes adds non-unique indexes on each
// half of the (catalog_entry_id, project_party_id) composite PK so lookups
// filtered by either column alone are well-indexed (notably the project
// filter on /catalog?project= and the per-entry list on /catalog/:id/projects).
func migration009CatalogProjectMembershipIndexes() Migration {
	return Migration{
		Version:     9,
		Description: "add per-column indexes on catalog_project_memberships",
		Up: func(tx *gorm.DB) error {
			if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cpm_project_party_id
				ON catalog_project_memberships(project_party_id)`).Error; err != nil {
				return fmt.Errorf("creating cpm project index: %w", err)
			}
			if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_cpm_catalog_entry_id
				ON catalog_project_memberships(catalog_entry_id)`).Error; err != nil {
				return fmt.Errorf("creating cpm entry index: %w", err)
			}
			return nil
		},
	}
}

func migration008BackfillPersonParties() Migration {
	return Migration{
		Version:     8,
		Description: "backfill person parties for users created after migration007",
		Up:          migration008Up,
	}
}

func migration008Up(tx *gorm.DB) error {
	if err := migration007SeedPersonParties(tx); err != nil {
		return fmt.Errorf("migration008 backfill: %w", err)
	}
	slog.Info("migration008: person party backfill complete")
	return nil
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
				if tx.Name() == "postgres" {
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
					if tx.Name() != "postgres" {
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

func migration007PartyArchetype() Migration {
	return Migration{
		Version:     7,
		Description: "create party archetype tables and seed default project",
		Up:          migration007Up,
	}
}

func migration007Up(tx *gorm.DB) error {
	for _, m := range []interface{}{
		&model.Party{},
		&model.PartyRelationship{},
		&model.PartyGroupClosure{},
		&model.GlobalPartyRole{},
		&model.CatalogProjectMembership{},
		&model.PartyIdentifier{},
	} {
		if err := tx.AutoMigrate(m); err != nil {
			return fmt.Errorf("auto migrate %T: %w", m, err)
		}
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_party_rel_unique
		ON party_relationships(from_party_id, from_role, to_party_id, to_role, relationship_name)`).Error; err != nil {
		return fmt.Errorf("creating party_relationships unique index: %w", err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_party_identifiers_kind_value
		ON party_identifiers(kind, value)`).Error; err != nil {
		return fmt.Errorf("creating party_identifiers unique index: %w", err)
	}
	defaultID := uuid.New().String()
	if err := tx.Exec(`
		INSERT INTO parties (id, kind, name, version, is_system, created_at, updated_at)
		SELECT ?, 'project', 'default', 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		WHERE NOT EXISTS (SELECT 1 FROM parties WHERE kind='project' AND is_system=1)
	`, defaultID).Error; err != nil {
		return fmt.Errorf("seeding default project: %w", err)
	}
	var defaultProjectID string
	if err := tx.Raw("SELECT id FROM parties WHERE kind='project' AND is_system=1 LIMIT 1").
		Scan(&defaultProjectID).Error; err != nil {
		return fmt.Errorf("reading default project id: %w", err)
	}
	if err := migration007SeedPersonParties(tx); err != nil {
		return err
	}
	if err := tx.Exec(`
		INSERT INTO catalog_project_memberships (catalog_entry_id, project_party_id, created_at)
		SELECT ce.id, ?, CURRENT_TIMESTAMP
		FROM catalog_entries ce
		WHERE NOT EXISTS (
			SELECT 1 FROM catalog_project_memberships
			WHERE catalog_entry_id = ce.id AND project_party_id = ?
		)
	`, defaultProjectID, defaultProjectID).Error; err != nil {
		return fmt.Errorf("assigning catalog entries to default project: %w", err)
	}
	slog.Info("migration007: party archetype tables created and seeded")
	return nil
}

func migration007SeedPersonParties(tx *gorm.DB) error {
	var existingUsers []struct {
		ID          string `gorm:"column:id"`
		Username    string `gorm:"column:username"`
		DisplayName string `gorm:"column:display_name"`
	}
	if err := tx.Raw("SELECT id, username, display_name FROM users").Scan(&existingUsers).Error; err != nil {
		return fmt.Errorf("reading existing users: %w", err)
	}
	for _, u := range existingUsers {
		name := u.Username
		if u.DisplayName != "" {
			name = u.DisplayName
		}
		partyID := uuid.New().String()
		userID := u.ID
		if err := tx.Exec(`
			INSERT INTO parties (id, kind, name, version, user_id, is_system, created_at, updated_at)
			SELECT ?, 'person', ?, 0, ?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			WHERE NOT EXISTS (SELECT 1 FROM parties WHERE user_id = ?)
		`, partyID, name, userID, userID).Error; err != nil {
			return fmt.Errorf("creating person party for user %s: %w", userID, err)
		}
	}
	return nil
}

// columnExistsAllowedTables is the allowlist of tables that columnExists may inspect.
// PRAGMA does not support parameterized table names, so we validate statically.
var columnExistsAllowedTables = map[string]bool{
	"agent_types":     true,
	"catalog_entries": true,
	"raw_cards":       true,
}

// columnExists reports whether a column exists in a given table.
// The table name must be in the allowlist to prevent SQL injection via PRAGMA.
func columnExists(db *gorm.DB, table, column string) (bool, error) {
	if !columnExistsAllowedTables[table] {
		return false, fmt.Errorf("columnExists: table %q is not in the allowed list", table)
	}
	switch db.Name() {
	case "postgres":
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?",
			table, column,
		).Scan(&count).Error
		return count > 0, err
	default:
		// SQLite: use PRAGMA table_info. PRAGMA does not support ? placeholders;
		// the table name is validated against the allowlist above.
		// colInfo only maps the Name column; GORM binds by column name and ignores extras (cid, type, etc.).
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

// migration010MCPDiscovery adds the MCP Discovery Server data layer:
//   - api_client_credentials — hashed service-account API keys (one active per party)
//   - mcp_sessions          — DB-backed MCP session rows with soft-delete
//   - user_external_identities — federated identity → user mappings with approval queue
//
// Partial unique indexes (one active credential per party; active = revoked_at IS NULL)
// use raw tx.Exec on both SQLite (3.8+) and PostgreSQL — no AutoMigrate magic.
// Permission seed extends the existing admin role JSON in-place.
func migration010MCPDiscovery() Migration {
	return Migration{
		Version:     10,
		Description: "mcp_discovery_v1: api_client_credentials, mcp_sessions, user_external_identities, service_account permissions",
		Up:          migration010Up,
	}
}

func migration010Up(tx *gorm.DB) error {
	if err := migration010ApiClientCredentials(tx); err != nil {
		return err
	}
	if err := migration010McpSessions(tx); err != nil {
		return err
	}
	if err := migration010UserExternalIdentities(tx); err != nil {
		return err
	}
	if err := migration010SeedPermissions(tx); err != nil {
		return err
	}
	slog.Info("migration010: MCP discovery schema created")
	return nil
}

func migration010ApiClientCredentials(tx *gorm.DB) error {
	if err := tx.Exec(`CREATE TABLE IF NOT EXISTS api_client_credentials (
		id           TEXT PRIMARY KEY,
		party_id     TEXT NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
		client_id    TEXT NOT NULL UNIQUE,
		secret_hash  TEXT NOT NULL,
		scopes       TEXT NOT NULL DEFAULT '',
		created_at   DATETIME NOT NULL,
		last_used_at DATETIME,
		expires_at   DATETIME,
		revoked_at   DATETIME
	)`).Error; err != nil {
		return fmt.Errorf("creating api_client_credentials: %w", err)
	}
	if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_acc_party_id ON api_client_credentials(party_id)`).Error; err != nil {
		return fmt.Errorf("creating acc party index: %w", err)
	}
	// Partial unique index: one active (revoked_at IS NULL) credential per party.
	// Works on SQLite >= 3.8 and PostgreSQL via the same WHERE clause syntax.
	return tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_acc_one_active_per_party ON api_client_credentials(party_id) WHERE revoked_at IS NULL`).Error
}

func migration010McpSessions(tx *gorm.DB) error {
	if err := tx.Exec(`CREATE TABLE IF NOT EXISTS mcp_sessions (
		id               TEXT PRIMARY KEY,
		principal_id     TEXT NOT NULL,
		principal_type   TEXT NOT NULL CHECK(principal_type IN ('user_local','user_federated','service_account')),
		protocol_version TEXT NOT NULL,
		created_at       DATETIME NOT NULL,
		last_seen_at     DATETIME NOT NULL,
		expires_at       DATETIME NOT NULL,
		initialized_at   DATETIME,
		revoked_at       DATETIME
	)`).Error; err != nil {
		return fmt.Errorf("creating mcp_sessions: %w", err)
	}
	if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_sessions_principal ON mcp_sessions(principal_id, principal_type)`).Error; err != nil {
		return fmt.Errorf("creating mcp_sessions principal index: %w", err)
	}
	return tx.Exec(`CREATE INDEX IF NOT EXISTS idx_mcp_sessions_expires ON mcp_sessions(expires_at)`).Error
}

func migration010UserExternalIdentities(tx *gorm.DB) error {
	if err := tx.Exec(`CREATE TABLE IF NOT EXISTS user_external_identities (
		id            TEXT PRIMARY KEY,
		provider_name TEXT NOT NULL,
		sub           TEXT NOT NULL,
		email         TEXT NOT NULL DEFAULT '',
		display_name  TEXT NOT NULL DEFAULT '',
		user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
		status        TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
		created_at    DATETIME NOT NULL,
		last_seen_at  DATETIME,
		approved_at   DATETIME,
		rejected_at   DATETIME
	)`).Error; err != nil {
		return fmt.Errorf("creating user_external_identities: %w", err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_uei_provider_sub ON user_external_identities(provider_name, sub)`).Error; err != nil {
		return fmt.Errorf("creating uei provider+sub unique index: %w", err)
	}
	return tx.Exec(`CREATE INDEX IF NOT EXISTS idx_uei_user_id ON user_external_identities(user_id)`).Error
}

func migration010SeedPermissions(tx *gorm.DB) error {
	newPerms := []string{"service_accounts:read", "service_accounts:write", "service_accounts:revoke"}
	var currentJSON string
	if err := tx.Raw(`SELECT permissions FROM roles WHERE id = 'role-admin'`).Scan(&currentJSON).Error; err != nil {
		return fmt.Errorf("reading admin role permissions: %w", err)
	}
	if currentJSON == "" {
		slog.Warn("migration010: admin role not found, skipping permission seed")
		return nil
	}
	var perms []string
	if err := json.Unmarshal([]byte(currentJSON), &perms); err != nil {
		return fmt.Errorf("unmarshaling admin permissions: %w", err)
	}
	existing := make(map[string]bool, len(perms))
	for _, p := range perms {
		existing[p] = true
	}
	for _, p := range newPerms {
		if !existing[p] {
			perms = append(perms, p)
		}
	}
	updated, err := json.Marshal(perms)
	if err != nil {
		return fmt.Errorf("marshaling updated permissions: %w", err)
	}
	return tx.Exec(`UPDATE roles SET permissions = ?, updated_at = ? WHERE id = 'role-admin'`,
		string(updated), time.Now().UTC()).Error
}

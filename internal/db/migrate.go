package db

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

// Migration represents a single versioned schema migration.
type Migration struct {
	Version     int
	Description string
	Up          func(db *gorm.DB) error
}

// SchemaMigration is the GORM model for tracking applied migrations.
type SchemaMigration struct {
	Version     int       `gorm:"primaryKey"`
	Description string    `gorm:"not null"`
	AppliedAt   time.Time `gorm:"autoCreateTime"`
}

// Migrator manages database schema migrations.
type Migrator struct {
	db         *DB
	migrations []Migration
}

// NewMigrator creates a new Migrator with the given migrations.
func NewMigrator(db *DB, migrations []Migration) *Migrator {
	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})
	return &Migrator{db: db, migrations: sorted}
}

// Migrate runs all pending migrations in order within a transaction.
func (m *Migrator) Migrate(ctx context.Context) error {
	// Ensure the schema_migrations table exists.
	if err := m.db.WithContext(ctx).AutoMigrate(&SchemaMigration{}); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	current, err := m.CurrentVersion(ctx)
	if err != nil {
		return err
	}

	for _, mig := range m.migrations {
		if mig.Version <= current {
			continue
		}
		if err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := mig.Up(tx); err != nil {
				return fmt.Errorf("migration %d (%s): %w", mig.Version, mig.Description, err)
			}
			return tx.Create(&SchemaMigration{
				Version:     mig.Version,
				Description: mig.Description,
			}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

// CurrentVersion returns the highest applied migration version, or 0 if none.
func (m *Migrator) CurrentVersion(ctx context.Context) (int, error) {
	// If the schema_migrations table doesn't exist yet, return 0.
	if !m.db.WithContext(ctx).Migrator().HasTable(&SchemaMigration{}) {
		return 0, nil
	}
	var sm SchemaMigration
	result := m.db.WithContext(ctx).Order("version DESC").First(&sm)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("querying current version: %w", result.Error)
	}
	return sm.Version, nil
}

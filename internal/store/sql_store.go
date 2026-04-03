package store

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/model"
)

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

	// Run migrations to create the catalog_entries table.
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

// Create inserts a new catalog entry into the store.
func (s *SQLStore) Create(ctx context.Context, entry *model.CatalogEntry) error {
	entry.SyncToDB()
	result := s.gdb.WithContext(ctx).Create(entry)
	if result.Error != nil {
		return fmt.Errorf("inserting catalog entry: %w", result.Error)
	}
	return nil
}

// Get retrieves a catalog entry by ID.
func (s *SQLStore) Get(ctx context.Context, id string) (*model.CatalogEntry, error) {
	var entry model.CatalogEntry
	result := s.gdb.WithContext(ctx).First(&entry, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("getting catalog entry: %w", result.Error)
	}
	entry.SyncFromDB()
	return &entry, nil
}

// Update modifies an existing catalog entry.
func (s *SQLStore) Update(ctx context.Context, entry *model.CatalogEntry) error {
	entry.SyncToDB()
	result := s.gdb.WithContext(ctx).Save(entry)
	if result.Error != nil {
		return fmt.Errorf("updating catalog entry: %w", result.Error)
	}
	return nil
}

// Delete removes a catalog entry by ID.
func (s *SQLStore) Delete(ctx context.Context, id string) error {
	result := s.gdb.WithContext(ctx).Delete(&model.CatalogEntry{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting catalog entry: %w", result.Error)
	}
	return nil
}

// FindByEndpoint returns the catalog entry with the given endpoint URL.
func (s *SQLStore) FindByEndpoint(ctx context.Context, endpoint string) (*model.CatalogEntry, error) {
	var entry model.CatalogEntry
	result := s.gdb.WithContext(ctx).Where("endpoint = ?", endpoint).First(&entry)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("finding by endpoint: %w", result.Error)
	}
	entry.SyncFromDB()
	return &entry, nil
}

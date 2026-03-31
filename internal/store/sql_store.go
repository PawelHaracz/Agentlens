package store

import (
	"context"
	"fmt"
	"strings"

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
			sqlDB.Close()
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

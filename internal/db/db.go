// Package db provides a GORM-based database abstraction layer supporting
// multiple SQL dialects (SQLite, PostgreSQL).
package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Dialect identifies which SQL database backend is in use.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// DB wraps *gorm.DB with dialect awareness.
type DB struct {
	*gorm.DB
	dialect Dialect
}

// Dialect returns the dialect used by this database connection.
func (d *DB) Dialect() Dialect {
	return d.dialect
}

// Open opens a GORM database connection for the given dialect and DSN.
// For SQLite, WAL mode, foreign keys, and busy timeout are applied via PRAGMA.
func Open(dialect Dialect, dsn string) (*DB, error) {
	var dialector gorm.Dialector
	switch dialect {
	case DialectSQLite:
		dialector = sqlite.Open(dsn)
	case DialectPostgres:
		dialector = postgres.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}

	gormDB, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening %s database: %w", dialect, err)
	}

	if dialect == DialectSQLite {
		if err := applySQLitePragmas(gormDB); err != nil {
			return nil, fmt.Errorf("applying sqlite pragmas: %w", err)
		}
	}

	return &DB{DB: gormDB, dialect: dialect}, nil
}

// OpenMemory opens an in-memory SQLite database suitable for testing.
func OpenMemory() (*DB, error) {
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("opening in-memory sqlite: %w", err)
	}

	// Pin to one connection so all goroutines share the same in-memory database.
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("getting underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := applySQLitePragmas(gormDB); err != nil {
		return nil, fmt.Errorf("applying sqlite pragmas: %w", err)
	}

	return &DB{DB: gormDB, dialect: DialectSQLite}, nil
}

func applySQLitePragmas(gormDB *gorm.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if err := gormDB.Exec(p).Error; err != nil {
			return fmt.Errorf("executing %q: %w", p, err)
		}
	}
	return nil
}

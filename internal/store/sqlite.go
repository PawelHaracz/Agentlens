package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/PawelHaracz/agentlens/internal/model"
)

// SQLiteStore is a SQLite-backed implementation of Store.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs migrations.
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := path
	if path != ":memory:" {
		dsn = path + "?_journal_mode=WAL&_foreign_keys=on"
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}
	// In-memory SQLite databases are per-connection; pin to one connection so
	// all goroutines share the same database instance.
	if path == ":memory:" {
		db.SetMaxOpenConns(1)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Create inserts a new catalog entry into the store.
func (s *SQLiteStore) Create(ctx context.Context, entry *model.CatalogEntry) error {
	provider, err := json.Marshal(entry.Provider)
	if err != nil {
		return fmt.Errorf("marshaling provider: %w", err)
	}
	categories, err := json.Marshal(entry.Categories)
	if err != nil {
		return fmt.Errorf("marshaling categories: %w", err)
	}
	skills, err := json.Marshal(entry.Skills)
	if err != nil {
		return fmt.Errorf("marshaling skills: %w", err)
	}
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	var rawCard *string
	if len(entry.RawCard) > 0 {
		rc := string(entry.RawCard)
		rawCard = &rc
	}
	var validityFrom, validityTo *time.Time
	if entry.Validity.From != nil {
		t := entry.Validity.From.UTC()
		validityFrom = &t
	}
	if entry.Validity.To != nil {
		t := entry.Validity.To.UTC()
		validityTo = &t
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO catalog_entries (id, display_name, description, protocol, endpoint, version, status, source,
		    provider, categories, skills, validity_from, validity_to, validity_last_seen, raw_card, metadata,
		    created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.DisplayName, entry.Description, string(entry.Protocol), entry.Endpoint,
		entry.Version, string(entry.Status), string(entry.Source),
		string(provider), string(categories), string(skills),
		validityFrom, validityTo, entry.Validity.LastSeen.UTC(),
		rawCard, string(metadata),
		entry.CreatedAt.UTC(), entry.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("inserting catalog entry: %w", err)
	}
	return nil
}

// Get retrieves a catalog entry by ID.
func (s *SQLiteStore) Get(ctx context.Context, id string) (*model.CatalogEntry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT * FROM catalog_entries WHERE id = ?`, id)
	entry, err := scanEntry(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting catalog entry: %w", err)
	}
	return entry, nil
}

// Update modifies an existing catalog entry.
func (s *SQLiteStore) Update(ctx context.Context, entry *model.CatalogEntry) error {
	provider, err := json.Marshal(entry.Provider)
	if err != nil {
		return fmt.Errorf("marshaling provider: %w", err)
	}
	categories, err := json.Marshal(entry.Categories)
	if err != nil {
		return fmt.Errorf("marshaling categories: %w", err)
	}
	skills, err := json.Marshal(entry.Skills)
	if err != nil {
		return fmt.Errorf("marshaling skills: %w", err)
	}
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}
	var rawCard *string
	if len(entry.RawCard) > 0 {
		rc := string(entry.RawCard)
		rawCard = &rc
	}
	var validityFrom, validityTo *time.Time
	if entry.Validity.From != nil {
		t := entry.Validity.From.UTC()
		validityFrom = &t
	}
	if entry.Validity.To != nil {
		t := entry.Validity.To.UTC()
		validityTo = &t
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE catalog_entries SET display_name=?, description=?, protocol=?, endpoint=?, version=?, status=?,
		    source=?, provider=?, categories=?, skills=?, validity_from=?, validity_to=?, validity_last_seen=?,
		    raw_card=?, metadata=?, updated_at=?
		WHERE id=?`,
		entry.DisplayName, entry.Description, string(entry.Protocol), entry.Endpoint,
		entry.Version, string(entry.Status), string(entry.Source),
		string(provider), string(categories), string(skills),
		validityFrom, validityTo, entry.Validity.LastSeen.UTC(),
		rawCard, string(metadata),
		entry.UpdatedAt.UTC(), entry.ID,
	)
	if err != nil {
		return fmt.Errorf("updating catalog entry: %w", err)
	}
	return nil
}

// Delete removes a catalog entry by ID.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM catalog_entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting catalog entry: %w", err)
	}
	return nil
}

// List returns catalog entries matching the given filter.
func (s *SQLiteStore) List(ctx context.Context, filter ListFilter) ([]model.CatalogEntry, error) {
	where := []string{"1=1"}
	args := []interface{}{}

	if filter.Protocol != nil {
		where = append(where, "protocol = ?")
		args = append(args, string(*filter.Protocol))
	}
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.Source != nil {
		where = append(where, "source = ?")
		args = append(args, string(*filter.Source))
	}
	if filter.Team != "" {
		where = append(where, "provider LIKE ?")
		args = append(args, "%"+filter.Team+"%")
	}
	if filter.Query != "" {
		where = append(where, "(display_name LIKE ? OR description LIKE ?)")
		q := "%" + filter.Query + "%"
		args = append(args, q, q)
	}
	for _, cat := range filter.Categories {
		where = append(where, "categories LIKE ?")
		args = append(args, "%"+cat+"%")
	}

	query := "SELECT * FROM catalog_entries WHERE " + strings.Join(where, " AND ") + " ORDER BY display_name"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing catalog entries: %w", err)
	}
	defer rows.Close()

	var entries []model.CatalogEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning catalog entry: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}

// FindByEndpoint returns the catalog entry with the given endpoint URL.
func (s *SQLiteStore) FindByEndpoint(ctx context.Context, endpoint string) (*model.CatalogEntry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT * FROM catalog_entries WHERE endpoint = ? LIMIT 1`, endpoint)
	entry, err := scanEntry(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding by endpoint: %w", err)
	}
	return entry, nil
}

// SearchSkills returns catalog entries whose skills match the query string.
func (s *SQLiteStore) SearchSkills(ctx context.Context, query string) ([]model.CatalogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT * FROM catalog_entries WHERE skills LIKE ?`, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("searching skills: %w", err)
	}
	defer rows.Close()

	var entries []model.CatalogEntry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning catalog entry: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}

// Stats returns aggregate statistics about catalog entries in the store.
func (s *SQLiteStore) Stats(ctx context.Context) (*StoreStats, error) {
	stats := &StoreStats{
		ByStatus: make(map[string]int),
		BySource: make(map[string]int),
	}

	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM catalog_entries`)
	if err := row.Scan(&stats.Total); err != nil {
		return nil, fmt.Errorf("counting catalog entries: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM catalog_entries GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("counting by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var v int
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		stats.ByStatus[k] = v
	}

	rows2, err := s.db.QueryContext(ctx, `SELECT source, COUNT(*) FROM catalog_entries GROUP BY source`)
	if err != nil {
		return nil, fmt.Errorf("counting by source: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var k string
		var v int
		if err := rows2.Scan(&k, &v); err != nil {
			return nil, err
		}
		stats.BySource[k] = v
	}

	return stats, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanEntry(s scanner) (*model.CatalogEntry, error) {
	var e model.CatalogEntry
	var protocol, status, source string
	var providerJSON, categoriesJSON, skillsJSON, metadataJSON string
	var rawCard sql.NullString
	var validityFrom, validityTo sql.NullTime
	var validityLastSeen, createdAt, updatedAt time.Time

	err := s.Scan(
		&e.ID, &e.DisplayName, &e.Description,
		&protocol, &e.Endpoint, &e.Version,
		&status, &source,
		&providerJSON, &categoriesJSON, &skillsJSON,
		&validityFrom, &validityTo, &validityLastSeen,
		&rawCard, &metadataJSON,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	e.Protocol = model.Protocol(protocol)
	e.Status = model.Status(status)
	e.Source = model.SourceType(source)
	e.Validity.LastSeen = validityLastSeen
	if validityFrom.Valid {
		e.Validity.From = &validityFrom.Time
	}
	if validityTo.Valid {
		e.Validity.To = &validityTo.Time
	}
	e.CreatedAt = createdAt
	e.UpdatedAt = updatedAt

	if err := json.Unmarshal([]byte(providerJSON), &e.Provider); err != nil {
		e.Provider = model.Provider{}
	}
	if err := json.Unmarshal([]byte(categoriesJSON), &e.Categories); err != nil {
		e.Categories = nil
	}
	if err := json.Unmarshal([]byte(skillsJSON), &e.Skills); err != nil {
		e.Skills = nil
	}
	if err := json.Unmarshal([]byte(metadataJSON), &e.Metadata); err != nil {
		e.Metadata = nil
	}
	if rawCard.Valid {
		e.RawCard = json.RawMessage(rawCard.String)
	}

	return &e, nil
}

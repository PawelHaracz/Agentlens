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

// Create inserts a new agent into the store.
func (s *SQLiteStore) Create(ctx context.Context, agent *model.Agent) error {
	tags, err := json.Marshal(agent.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}
	skills, err := json.Marshal(agent.Skills)
	if err != nil {
		return fmt.Errorf("marshaling skills: %w", err)
	}
	var rawCard *string
	if len(agent.RawCard) > 0 {
		s := string(agent.RawCard)
		rawCard = &s
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents (id, name, description, protocol, endpoint, version, status, source,
		    namespace, team, tags, skills, raw_card, last_seen, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.ID, agent.Name, agent.Description, string(agent.Protocol), agent.Endpoint,
		agent.Version, string(agent.Status), string(agent.Source),
		agent.Namespace, agent.Team, string(tags), string(skills), rawCard,
		agent.LastSeen.UTC(), agent.CreatedAt.UTC(), agent.UpdatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("inserting agent: %w", err)
	}
	return nil
}

// Get retrieves an agent by ID.
func (s *SQLiteStore) Get(ctx context.Context, id string) (*model.Agent, error) {
	row := s.db.QueryRowContext(ctx, `SELECT * FROM agents WHERE id = ?`, id)
	agent, err := scanAgent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting agent: %w", err)
	}
	return agent, nil
}

// Update modifies an existing agent.
func (s *SQLiteStore) Update(ctx context.Context, agent *model.Agent) error {
	tags, err := json.Marshal(agent.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}
	skills, err := json.Marshal(agent.Skills)
	if err != nil {
		return fmt.Errorf("marshaling skills: %w", err)
	}
	var rawCard *string
	if len(agent.RawCard) > 0 {
		s := string(agent.RawCard)
		rawCard = &s
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE agents SET name=?, description=?, protocol=?, endpoint=?, version=?, status=?,
		    source=?, namespace=?, team=?, tags=?, skills=?, raw_card=?, last_seen=?, updated_at=?
		WHERE id=?`,
		agent.Name, agent.Description, string(agent.Protocol), agent.Endpoint,
		agent.Version, string(agent.Status), string(agent.Source),
		agent.Namespace, agent.Team, string(tags), string(skills), rawCard,
		agent.LastSeen.UTC(), agent.UpdatedAt.UTC(), agent.ID,
	)
	if err != nil {
		return fmt.Errorf("updating agent: %w", err)
	}
	return nil
}

// Delete removes an agent by ID.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}
	return nil
}

// List returns agents matching the given filter.
func (s *SQLiteStore) List(ctx context.Context, filter ListFilter) ([]model.Agent, error) {
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
		where = append(where, "team = ?")
		args = append(args, filter.Team)
	}
	if filter.Query != "" {
		where = append(where, "(name LIKE ? OR description LIKE ?)")
		q := "%" + filter.Query + "%"
		args = append(args, q, q)
	}
	for _, tag := range filter.Tags {
		where = append(where, "tags LIKE ?")
		args = append(args, "%"+tag+"%")
	}

	query := "SELECT * FROM agents WHERE " + strings.Join(where, " AND ") + " ORDER BY name"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	defer rows.Close()

	var agents []model.Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning agent: %w", err)
		}
		agents = append(agents, *a)
	}
	return agents, rows.Err()
}

// FindByEndpoint returns the agent with the given endpoint URL.
func (s *SQLiteStore) FindByEndpoint(ctx context.Context, endpoint string) (*model.Agent, error) {
	row := s.db.QueryRowContext(ctx, `SELECT * FROM agents WHERE endpoint = ? LIMIT 1`, endpoint)
	agent, err := scanAgent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding by endpoint: %w", err)
	}
	return agent, nil
}

// SearchSkills returns agents whose skills match the query string.
func (s *SQLiteStore) SearchSkills(ctx context.Context, query string) ([]model.Agent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT * FROM agents WHERE skills LIKE ?`, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("searching skills: %w", err)
	}
	defer rows.Close()

	var agents []model.Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning agent: %w", err)
		}
		agents = append(agents, *a)
	}
	return agents, rows.Err()
}

// Stats returns aggregate statistics about agents in the store.
func (s *SQLiteStore) Stats(ctx context.Context) (*StoreStats, error) {
	stats := &StoreStats{
		ByStatus: make(map[string]int),
		BySource: make(map[string]int),
	}

	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents`)
	if err := row.Scan(&stats.Total); err != nil {
		return nil, fmt.Errorf("counting agents: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM agents GROUP BY status`)
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

	rows2, err := s.db.QueryContext(ctx, `SELECT source, COUNT(*) FROM agents GROUP BY source`)
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

func scanAgent(s scanner) (*model.Agent, error) {
	var a model.Agent
	var protocol, status, source string
	var tagsJSON, skillsJSON string
	var rawCard sql.NullString
	var lastSeen, createdAt, updatedAt time.Time

	err := s.Scan(
		&a.ID, &a.Name, &a.Description,
		&protocol, &a.Endpoint, &a.Version,
		&status, &source,
		&a.Namespace, &a.Team,
		&tagsJSON, &skillsJSON, &rawCard,
		&lastSeen, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	a.Protocol = model.Protocol(protocol)
	a.Status = model.Status(status)
	a.Source = model.SourceType(source)
	a.LastSeen = lastSeen
	a.CreatedAt = createdAt
	a.UpdatedAt = updatedAt

	if err := json.Unmarshal([]byte(tagsJSON), &a.Tags); err != nil {
		a.Tags = nil
	}
	if err := json.Unmarshal([]byte(skillsJSON), &a.Skills); err != nil {
		a.Skills = nil
	}
	if rawCard.Valid {
		a.RawCard = json.RawMessage(rawCard.String)
	}

	return &a, nil
}

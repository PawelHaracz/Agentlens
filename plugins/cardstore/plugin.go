// Package cardstore provides the CardStorePlugin for persisting raw agent card bytes.
package cardstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/PawelHaracz/agentlens/internal/db"
	"github.com/PawelHaracz/agentlens/internal/kernel"
	"github.com/PawelHaracz/agentlens/internal/model"
)

const maxCardBytes = 256 * 1024 // 256 KiB hard cap

// rawCardRow is the GORM model for the raw_cards table.
// It is internal to this plugin — callers use model.RawCard.
type rawCardRow struct {
	AgentTypeID string    `gorm:"primaryKey;type:text"`
	Data        []byte    `gorm:"not null;type:blob"`
	ContentType string    `gorm:"not null;type:text;default:'application/json'"`
	FetchedAt   time.Time `gorm:"not null"`
	Truncated   bool      `gorm:"not null;default:false"`
}

func (rawCardRow) TableName() string { return "raw_cards" }

// Plugin implements kernel.CardStorePlugin.
type Plugin struct {
	database *db.DB
}

// New creates a new Plugin. Call MigrateSchema before first use.
func New(database *db.DB) *Plugin {
	return &Plugin{database: database}
}

// Name returns the plugin name.
func (p *Plugin) Name() string { return "card-store" }

// Version returns the plugin version.
func (p *Plugin) Version() string { return "1.0.0" }

// Type returns the plugin type.
func (p *Plugin) Type() kernel.PluginType { return kernel.PluginTypeCardStore }

// Init is called by the PluginManager. AutoMigrate is an idempotent safety net
// (migration 006 creates the table first).
func (p *Plugin) Init(_ kernel.Kernel) error {
	return p.MigrateSchema(context.Background())
}

// Start is a no-op — card store is synchronous.
func (p *Plugin) Start(_ context.Context) error { return nil }

// Stop is a no-op.
func (p *Plugin) Stop(_ context.Context) error { return nil }

// StoreCard persists raw card bytes for an AgentType (upsert by agentTypeID).
// Payloads exceeding maxCardBytes are truncated and flagged.
func (p *Plugin) StoreCard(ctx context.Context, agentTypeID string, data []byte, contentType string) error {
	if agentTypeID == "" {
		return fmt.Errorf("agentTypeID must not be empty")
	}
	truncated := false
	if len(data) > maxCardBytes {
		data = data[:maxCardBytes]
		truncated = true
	}
	row := rawCardRow{
		AgentTypeID: agentTypeID,
		Data:        data,
		ContentType: contentType,
		FetchedAt:   time.Now().UTC(),
		Truncated:   truncated,
	}
	result := p.database.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "agent_type_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"data", "content_type", "fetched_at", "truncated"}),
		}).
		Create(&row)
	if result.Error != nil {
		return fmt.Errorf("storing raw card for %s: %w", agentTypeID, result.Error)
	}
	return nil
}

// GetCard retrieves the raw card for the given AgentTypeID.
// Returns an error wrapping gorm.ErrRecordNotFound if no card is stored.
func (p *Plugin) GetCard(ctx context.Context, agentTypeID string) (*model.RawCard, error) {
	var row rawCardRow
	result := p.database.WithContext(ctx).
		Where("agent_type_id = ?", agentTypeID).
		First(&row)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("no raw card stored for agent_type_id %s: %w", agentTypeID, result.Error)
		}
		return nil, fmt.Errorf("getting raw card for %s: %w", agentTypeID, result.Error)
	}
	return &model.RawCard{
		AgentTypeID: row.AgentTypeID,
		Data:        row.Data,
		ContentType: row.ContentType,
		FetchedAt:   row.FetchedAt,
		Truncated:   row.Truncated,
	}, nil
}

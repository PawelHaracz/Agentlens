package cardstore

import "context"

// MigrateSchema ensures the raw_cards table exists. Idempotent.
func (p *Plugin) MigrateSchema(_ context.Context) error {
	return p.database.AutoMigrate(&rawCardRow{})
}

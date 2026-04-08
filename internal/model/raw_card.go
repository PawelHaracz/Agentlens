package model

import "time"

// RawCard holds the verbatim card bytes for an AgentType.
// It is a pure transport struct returned by CardStorePlugin.GetCard() — it is NOT a GORM
// model and has no database tags. Persistence is handled by the CardStorePlugin's internal
// rawCardRow struct. RawCard must never be embedded in AgentType or CatalogEntry.
type RawCard struct {
	AgentTypeID string    `json:"agent_type_id"`
	Data        []byte    `json:"data"` // raw bytes; callers write directly via http.ResponseWriter, not via json.Marshal
	ContentType string    `json:"content_type"`
	FetchedAt   time.Time `json:"fetched_at"`
	Truncated   bool      `json:"truncated"`
}

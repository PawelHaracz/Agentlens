package model

import "time"

// RawCard holds the verbatim card bytes for an AgentType.
// It is owned by the CardStorePlugin — never embedded in AgentType or CatalogEntry.
type RawCard struct {
	AgentTypeID string    `json:"agent_type_id"`
	Data        []byte    `json:"data"`
	ContentType string    `json:"content_type"`
	FetchedAt   time.Time `json:"fetched_at"`
	Truncated   bool      `json:"truncated"`
}

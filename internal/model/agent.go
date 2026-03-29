// Package model defines the core data types for AgentLens.
package model

import (
	"encoding/json"
	"time"
)

// Protocol represents the agent communication protocol.
type Protocol string

const (
	ProtocolA2A  Protocol = "a2a"
	ProtocolMCP  Protocol = "mcp"
	ProtocolA2UI Protocol = "a2ui"
)

// Status represents the health status of a catalog entry.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusDown     Status = "down"
	StatusUnknown  Status = "unknown"
)

// SourceType represents how the catalog entry was discovered.
type SourceType string

const (
	SourceK8s      SourceType = "k8s"
	SourceConfig   SourceType = "config"
	SourcePush     SourceType = "push"
	SourceUpstream SourceType = "upstream"
)

// Provider represents the organization offering the catalog entry.
type Provider struct {
	Organization string `json:"organization"`
	Team         string `json:"team,omitempty"`
	URL          string `json:"url,omitempty"`
}

// Validity represents a time-bounded availability period.
type Validity struct {
	From     *time.Time `json:"from,omitempty"`
	To       *time.Time `json:"to,omitempty"`
	LastSeen time.Time  `json:"last_seen"`
}

// IsActiveAt checks if the entry is active at the given time.
func (v Validity) IsActiveAt(t time.Time) bool {
	if v.From != nil && t.Before(*v.From) {
		return false
	}
	if v.To != nil && t.After(*v.To) {
		return false
	}
	return true
}

// CatalogEntry represents a discovered agent/server in the catalog.
// This follows the Product archetype pattern where CatalogEntry
// is the commercial offering wrapping a ProductType.
type CatalogEntry struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	Protocol    Protocol          `json:"protocol"`
	Endpoint    string            `json:"endpoint"`
	Version     string            `json:"version"`
	Status      Status            `json:"status"`
	Source      SourceType        `json:"source"`
	Provider    Provider          `json:"provider,omitempty"`
	Categories  []string          `json:"categories,omitempty"`
	Skills      []Skill           `json:"skills,omitempty"`
	Validity    Validity          `json:"validity"`
	RawCard     json.RawMessage   `json:"raw_card,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

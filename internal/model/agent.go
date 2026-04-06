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

// CatalogEntry is the catalog wrapper (Product archetype) around an AgentType.
// It represents a discoverable listing of an agent in the catalog.
type CatalogEntry struct {
	ID          string            `json:"id"           gorm:"primaryKey;type:text"`
	AgentTypeID string            `json:"agent_type_id" gorm:"not null;type:text;index"`
	AgentType   *AgentType        `json:"-"            gorm:"foreignKey:AgentTypeID"`
	DisplayName string            `json:"display_name"  gorm:"not null;type:text"`
	Description string            `json:"description"   gorm:"type:text;default:''"`
	Status      Status            `json:"status"        gorm:"not null;type:text;default:'unknown';index"`
	Source      SourceType        `json:"source"        gorm:"not null;type:text;index"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Categories  []string          `json:"-" gorm:"-"`
	Metadata    map[string]string `json:"-" gorm:"-"`
	Validity    Validity          `json:"-" gorm:"-"`

	// Database-serialized JSON fields (used by GORM, hidden from JSON API).
	CategoriesJSON string     `json:"-" gorm:"column:categories;type:text;not null;default:'[]'"`
	MetadataJSON   string     `json:"-" gorm:"column:metadata;type:text;not null;default:'{}'"`
	ValidFrom      *time.Time `json:"-" gorm:"column:validity_from"`
	ValidTo        *time.Time `json:"-" gorm:"column:validity_to"`
	LastSeen       time.Time  `json:"-" gorm:"column:validity_last_seen;not null"`
}

// TableName overrides the GORM table name.
func (CatalogEntry) TableName() string { return "catalog_entries" }

// SyncToDB serializes public fields into GORM database columns.
func (e *CatalogEntry) SyncToDB() {
	if b, err := json.Marshal(e.Categories); err == nil {
		e.CategoriesJSON = string(b)
	}
	if b, err := json.Marshal(e.Metadata); err == nil {
		e.MetadataJSON = string(b)
	}
	e.ValidFrom = e.Validity.From
	e.ValidTo = e.Validity.To
	e.LastSeen = e.Validity.LastSeen
}

// SyncFromDB deserializes GORM database columns into public fields.
// If AgentType is loaded, it also calls AgentType.SyncRawDefForJSON().
func (e *CatalogEntry) SyncFromDB() {
	if e.CategoriesJSON != "" {
		_ = json.Unmarshal([]byte(e.CategoriesJSON), &e.Categories)
	}
	if e.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(e.MetadataJSON), &e.Metadata)
	}
	e.Validity = Validity{
		From:     e.ValidFrom,
		To:       e.ValidTo,
		LastSeen: e.LastSeen,
	}
	if e.AgentType != nil {
		e.AgentType.SyncRawDefForJSON()
	}
}

// MarshalJSON produces a flat, backward-compatible JSON representation that
// merges AgentType fields into the CatalogEntry output.
func (e CatalogEntry) MarshalJSON() ([]byte, error) {
	e.SyncFromDB()

	// Pull fields from AgentType when available.
	var (
		protocol     Protocol
		endpoint     string
		version      string
		specVersion  string
		provider     *Provider
		capabilities []Capability
		rawDef       json.RawMessage
	)
	if e.AgentType != nil {
		protocol = e.AgentType.Protocol
		endpoint = e.AgentType.Endpoint
		version = e.AgentType.Version
		specVersion = e.AgentType.SpecVersion
		provider = e.AgentType.Provider
		capabilities = e.AgentType.Capabilities
		rawDef = e.AgentType.RawDefJSON
	}

	var capJSON json.RawMessage
	if len(capabilities) > 0 {
		if b, err := MarshalCapabilitiesJSON(capabilities); err == nil {
			capJSON = b
		}
	}

	return json.Marshal(struct {
		ID           string            `json:"id"`
		AgentTypeID  string            `json:"agent_type_id"`
		DisplayName  string            `json:"display_name"`
		Description  string            `json:"description"`
		Protocol     Protocol          `json:"protocol,omitempty"`
		Endpoint     string            `json:"endpoint,omitempty"`
		Version      string            `json:"version,omitempty"`
		SpecVersion  string            `json:"spec_version,omitempty"`
		Status       Status            `json:"status"`
		Source       SourceType        `json:"source"`
		Provider     *Provider         `json:"provider,omitempty"`
		Categories   []string          `json:"categories,omitempty"`
		Capabilities json.RawMessage   `json:"capabilities,omitempty"`
		Validity     Validity          `json:"validity"`
		Metadata     map[string]string `json:"metadata,omitempty"`
		RawDef       json.RawMessage   `json:"raw_definition,omitempty"`
		CreatedAt    time.Time         `json:"created_at"`
		UpdatedAt    time.Time         `json:"updated_at"`
	}{
		ID:           e.ID,
		AgentTypeID:  e.AgentTypeID,
		DisplayName:  e.DisplayName,
		Description:  e.Description,
		Protocol:     protocol,
		Endpoint:     endpoint,
		Version:      version,
		SpecVersion:  specVersion,
		Status:       e.Status,
		Source:       e.Source,
		Provider:     provider,
		Categories:   e.Categories,
		Capabilities: capJSON,
		Validity:     e.Validity,
		Metadata:     e.Metadata,
		RawDef:       rawDef,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	})
}

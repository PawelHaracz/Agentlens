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
	ID          string            `json:"id" gorm:"primaryKey;type:text"`
	DisplayName string            `json:"display_name" gorm:"not null;type:text"`
	Description string            `json:"description" gorm:"type:text;default:''"`
	Protocol    Protocol          `json:"protocol" gorm:"not null;type:text;index"`
	Endpoint    string            `json:"endpoint" gorm:"uniqueIndex;type:text"`
	Version     string            `json:"version" gorm:"type:text;default:''"`
	Status      Status            `json:"status" gorm:"not null;type:text;default:'unknown';index"`
	Source      SourceType        `json:"source" gorm:"not null;type:text;index"`
	Provider    Provider          `json:"provider,omitempty" gorm:"-"`
	Categories  []string          `json:"categories,omitempty" gorm:"-"`
	Skills      []Skill           `json:"skills,omitempty" gorm:"-"`
	Validity    Validity          `json:"validity" gorm:"-"`
	RawCard     json.RawMessage   `json:"raw_card,omitempty" gorm:"-"`
	Metadata    map[string]string `json:"metadata,omitempty" gorm:"-"`
	SpecVersion string            `json:"spec_version,omitempty" gorm:"type:text;not null;default:''"`
	TypedMeta   []TypedMetadata   `json:"typed_meta,omitempty" gorm:"-"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`

	// Database-serialized JSON fields (used by GORM, hidden from JSON API).
	ProviderJSON   string     `json:"-" gorm:"column:provider;type:text;not null;default:'{}';index"`
	CategoriesJSON string     `json:"-" gorm:"column:categories;type:text;not null;default:'[]'"`
	SkillsJSON     string     `json:"-" gorm:"column:skills;type:text;not null;default:'[]'"`
	MetadataJSON   string     `json:"-" gorm:"column:metadata;type:text;not null;default:'{}'"`
	RawCardStr     *string    `json:"-" gorm:"column:raw_card;type:text"`
	ValidFrom      *time.Time `json:"-" gorm:"column:validity_from"`
	ValidTo        *time.Time `json:"-" gorm:"column:validity_to"`
	LastSeen       time.Time  `json:"-" gorm:"column:validity_last_seen;not null"`
	TypedMetaJSON  string     `json:"-" gorm:"column:typed_meta;type:text;not null;default:'[]'"`
}

// TableName overrides the GORM table name.
func (CatalogEntry) TableName() string { return "catalog_entries" }

// MarshalJSON converts the entry to JSON, syncing GORM fields to public fields first.
func (e CatalogEntry) MarshalJSON() ([]byte, error) {
	e.SyncFromDB()
	type Alias CatalogEntry
	return json.Marshal(struct {
		Alias
		Provider    Provider          `json:"provider,omitempty"`
		Categories  []string          `json:"categories,omitempty"`
		Skills      []Skill           `json:"skills,omitempty"`
		Validity    Validity          `json:"validity"`
		RawCard     json.RawMessage   `json:"raw_card,omitempty"`
		Metadata    map[string]string `json:"metadata,omitempty"`
		SpecVersion string            `json:"spec_version,omitempty"`
		TypedMeta   []TypedMetadata   `json:"typed_meta,omitempty"`
	}{
		Alias:       Alias(e),
		Provider:    e.Provider,
		Categories:  e.Categories,
		Skills:      e.Skills,
		Validity:    e.Validity,
		RawCard:     e.RawCard,
		Metadata:    e.Metadata,
		SpecVersion: e.SpecVersion,
		TypedMeta:   e.TypedMeta,
	})
}

// SyncToDB serializes public fields into GORM database columns.
func (e *CatalogEntry) SyncToDB() {
	if b, err := json.Marshal(e.Provider); err == nil {
		e.ProviderJSON = string(b)
	}
	if b, err := json.Marshal(e.Categories); err == nil {
		e.CategoriesJSON = string(b)
	}
	if b, err := json.Marshal(e.Skills); err == nil {
		e.SkillsJSON = string(b)
	}
	if b, err := json.Marshal(e.Metadata); err == nil {
		e.MetadataJSON = string(b)
	}
	if len(e.RawCard) > 0 {
		s := string(e.RawCard)
		e.RawCardStr = &s
	} else {
		e.RawCardStr = nil
	}
	e.ValidFrom = e.Validity.From
	e.ValidTo = e.Validity.To
	e.LastSeen = e.Validity.LastSeen
	if b, err := MarshalTypedMetaJSON(e.TypedMeta); err == nil {
		e.TypedMetaJSON = string(b)
	}
}

// SyncFromDB deserializes GORM database columns into public fields.
func (e *CatalogEntry) SyncFromDB() {
	if e.ProviderJSON != "" {
		_ = json.Unmarshal([]byte(e.ProviderJSON), &e.Provider)
	}
	if e.CategoriesJSON != "" {
		_ = json.Unmarshal([]byte(e.CategoriesJSON), &e.Categories)
	}
	if e.SkillsJSON != "" {
		_ = json.Unmarshal([]byte(e.SkillsJSON), &e.Skills)
	}
	if e.MetadataJSON != "" {
		_ = json.Unmarshal([]byte(e.MetadataJSON), &e.Metadata)
	}
	if e.RawCardStr != nil {
		e.RawCard = json.RawMessage(*e.RawCardStr)
	}
	if e.TypedMetaJSON != "" {
		e.TypedMeta, _ = UnmarshalTypedMetaJSON([]byte(e.TypedMetaJSON))
	}
	e.Validity = Validity{
		From:     e.ValidFrom,
		To:       e.ValidTo,
		LastSeen: e.LastSeen,
	}
}

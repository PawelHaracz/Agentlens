package model

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// AgentType represents a ProductType — "what the agent IS".
type AgentType struct {
	ID            string    `json:"id"             gorm:"primaryKey;type:text"`
	AgentKey      string    `json:"agent_key"      gorm:"not null;type:text;index"`
	Protocol      Protocol  `json:"protocol"       gorm:"not null;type:text"`
	Endpoint      string    `json:"endpoint"       gorm:"not null;type:text"`
	Version       string    `json:"version"        gorm:"not null;type:text;default:''"`
	SpecVersion   string    `json:"spec_version"   gorm:"type:text;default:''"`
	ProviderID    *string   `json:"provider_id,omitempty" gorm:"type:text;index"`
	Provider      *Provider `json:"provider,omitempty"    gorm:"foreignKey:ProviderID"`
	RawDefinition []byte    `json:"-"              gorm:"not null;type:blob"`
	CreatedOn     time.Time `json:"created_on"`

	// Capabilities loaded separately, not via GORM auto-preload.
	Capabilities []Capability `json:"capabilities,omitempty" gorm:"-"`

	// For JSON API responses — populated by SyncRawDefForJSON.
	RawDefJSON json.RawMessage `json:"raw_definition,omitempty" gorm:"-"`
}

func (AgentType) TableName() string { return "agent_types" }

// ComputeAgentKey computes SHA256(protocol + endpoint) as the partition key for versioning.
func ComputeAgentKey(protocol Protocol, endpoint string) string {
	h := sha256.Sum256([]byte(string(protocol) + endpoint))
	return fmt.Sprintf("%x", h)
}

// SyncRawDefForJSON populates RawDefJSON from RawDefinition for JSON serialization.
func (a *AgentType) SyncRawDefForJSON() {
	if len(a.RawDefinition) > 0 && json.Valid(a.RawDefinition) {
		a.RawDefJSON = json.RawMessage(a.RawDefinition)
	}
}

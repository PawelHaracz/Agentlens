package model

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// AgentType represents a ProductType — "what the agent IS".
type AgentType struct {
	ID          string    `json:"id"             gorm:"primaryKey;type:text"`
	AgentKey    string    `json:"agent_key"      gorm:"not null;type:text;index"`
	Protocol    Protocol  `json:"protocol"       gorm:"not null;type:text"`
	Endpoint    string    `json:"endpoint"       gorm:"not null;type:text"`
	Version     string    `json:"version"        gorm:"not null;type:text;default:''"`
	SpecVersion string    `json:"spec_version"   gorm:"type:text;default:''"`
	ProviderID  *string   `json:"provider_id,omitempty" gorm:"type:text;index"`
	Provider    *Provider `json:"provider,omitempty"    gorm:"foreignKey:ProviderID"`
	CreatedOn   time.Time `json:"created_on"`

	// Capabilities loaded separately, not via GORM auto-preload.
	Capabilities []Capability `json:"capabilities,omitempty" gorm:"-"`
}

func (AgentType) TableName() string { return "agent_types" }

// ComputeAgentKey computes SHA256(protocol + endpoint) as the partition key for versioning.
func ComputeAgentKey(protocol Protocol, endpoint string) string {
	h := sha256.Sum256([]byte(string(protocol) + endpoint))
	return fmt.Sprintf("%x", h)
}

package model

import "time"

// Provider represents an organization that owns agents (reusable across AgentTypes).
type Provider struct {
	ID           string    `json:"id"           gorm:"primaryKey;type:text"`
	Organization string    `json:"organization" gorm:"not null;type:text"`
	Team         string    `json:"team,omitempty" gorm:"type:text"`
	URL          string    `json:"url,omitempty"  gorm:"type:text"`
	CreatedOn    time.Time `json:"created_on"`
}

func (Provider) TableName() string { return "providers" }

package model

import "time"

// Setting represents a key-value configuration setting.
type Setting struct {
	Key         string    `json:"key" gorm:"primaryKey;type:text"`
	Value       string    `json:"value" gorm:"not null;type:text"`
	Category    string    `json:"category" gorm:"type:text;default:'general';index"`
	Description string    `json:"description" gorm:"type:text"`
	UpdatedAt   time.Time `json:"updated_at"`
}

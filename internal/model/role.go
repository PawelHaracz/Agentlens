package model

import "time"

// Role represents an authorization role in the system.
type Role struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null;type:text"`
	Description string    `json:"description" gorm:"type:text"`
	Permissions JSONSlice `json:"permissions" gorm:"type:text;not null;default:'[]'"`
	IsSystem    bool      `json:"is_system" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

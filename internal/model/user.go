package model

import "time"

// User represents an authenticated user in the system.
type User struct {
	ID             string     `json:"id" gorm:"primaryKey;type:text"`
	Username       string     `json:"username" gorm:"uniqueIndex;not null;type:text"`
	Email          string     `json:"email" gorm:"type:text"`
	DisplayName    string     `json:"display_name" gorm:"type:text"`
	PasswordHash   string     `json:"-" gorm:"not null;type:text"`
	RoleID         string     `json:"role_id" gorm:"type:text;index"`
	Role           *Role      `json:"role,omitempty" gorm:"foreignKey:RoleID"`
	IsActive       bool       `json:"is_active" gorm:"default:true"`
	LastLogin      *time.Time `json:"last_login,omitempty"`
	FailedAttempts int        `json:"-" gorm:"default:0"`
	LockedUntil    *time.Time `json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

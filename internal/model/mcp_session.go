package model

import "time"

// McpSession tracks a live MCP session established via the Streamable HTTP
// transport. Sessions are DB-backed for restart durability and operator audit.
//
// Revocation uses soft-delete (RevokedAt) so the audit trail is preserved.
// The session reaper (plugins/mcpserver) sweeps expired and orphaned sessions.
type McpSession struct {
	ID              string        `gorm:"primaryKey;type:text"   json:"id"`
	PrincipalID     string        `gorm:"not null;type:text;index" json:"principal_id"`
	PrincipalType   PrincipalType `gorm:"not null;type:text"     json:"principal_type"`
	ProtocolVersion string        `gorm:"not null;type:text"     json:"protocol_version"`
	CreatedAt       time.Time     `                              json:"created_at"`
	LastSeenAt      time.Time     `gorm:"not null"               json:"last_seen_at"`
	ExpiresAt       time.Time     `gorm:"not null"               json:"expires_at"`
	InitializedAt   *time.Time    `gorm:"type:datetime"          json:"initialized_at,omitempty"`
	RevokedAt       *time.Time    `gorm:"type:datetime"          json:"revoked_at,omitempty"`
}

// IsActive reports whether the session has not been revoked and has not expired.
func (s *McpSession) IsActive(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

// IsInitialized reports whether the MCP initialize/notifications/initialized
// handshake has completed.
func (s *McpSession) IsInitialized() bool {
	return s.InitializedAt != nil
}

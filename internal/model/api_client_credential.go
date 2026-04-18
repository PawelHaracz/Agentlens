package model

import "time"

// ApiClientCredential stores a hashed service-account API key.
// Secret format on issuance: "agentlens_sk_<ClientID>.<rawSecret>".
// Only the bcrypt hash of rawSecret is stored; the raw value is shown once.
//
// Invariant: at most one row per PartyID has RevokedAt IS NULL (enforced by a
// partial unique index in migration 010). Rotation uses UPDATE-then-INSERT in a
// single transaction to avoid momentary dual-active violations.
type ApiClientCredential struct {
	ID         string     `gorm:"primaryKey;type:text"                    json:"id"`
	PartyID    string     `gorm:"not null;type:text;index"                json:"party_id"`
	ClientID   string     `gorm:"uniqueIndex;not null;type:text"          json:"client_id"`
	SecretHash string     `gorm:"not null;type:text"                      json:"-"`
	Scopes     string     `gorm:"not null;type:text;default:''"           json:"scopes"`
	CreatedAt  time.Time  `                                               json:"created_at"`
	LastUsedAt *time.Time `gorm:"type:datetime"                           json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `gorm:"type:datetime"                           json:"expires_at,omitempty"`
	RevokedAt  *time.Time `gorm:"type:datetime"                           json:"revoked_at,omitempty"`
}

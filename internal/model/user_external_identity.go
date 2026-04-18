package model

import "time"

// ExternalIdentityStatus is the approval state for a federated login.
type ExternalIdentityStatus string

const (
	ExternalIdentityStatusPending  ExternalIdentityStatus = "pending"
	ExternalIdentityStatusApproved ExternalIdentityStatus = "approved"
	ExternalIdentityStatusRejected ExternalIdentityStatus = "rejected"
)

// UserExternalIdentity links a federated sub claim to an AgentLens user.
// When JIT provisioning is disabled (default), new federated logins land in
// "pending" status and require operator approval before the identity resolves
// to a SessionPrincipalRef.
type UserExternalIdentity struct {
	ID           string                 `gorm:"primaryKey;type:text"                   json:"id"`
	ProviderName string                 `gorm:"not null;type:text;index"               json:"provider_name"`
	Sub          string                 `gorm:"not null;type:text"                     json:"sub"`
	Email        string                 `gorm:"type:text;default:''"                   json:"email"`
	DisplayName  string                 `gorm:"type:text;default:''"                   json:"display_name"`
	UserID       *string                `gorm:"type:text;index"                        json:"user_id,omitempty"`
	Status       ExternalIdentityStatus `gorm:"not null;type:text;default:'pending'"   json:"status"`
	CreatedAt    time.Time              `                                              json:"created_at"`
	LastSeenAt   *time.Time             `gorm:"type:datetime"                          json:"last_seen_at,omitempty"`
	ApprovedAt   *time.Time             `gorm:"type:datetime"                          json:"approved_at,omitempty"`
	RejectedAt   *time.Time             `gorm:"type:datetime"                          json:"rejected_at,omitempty"`
}

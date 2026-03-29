package kernel

import "time"

// LicenseInfo holds license validation results.
type LicenseInfo struct {
	Valid      bool      `json:"valid"`
	Tier       string    `json:"tier"`
	ExpiresAt  time.Time `json:"expires_at"`
	Features   []string  `json:"features"`
	LicensedTo string    `json:"licensed_to"`
}

// CommunityLicense returns the default community license info.
func CommunityLicense() LicenseInfo {
	return LicenseInfo{Valid: true, Tier: "community"}
}

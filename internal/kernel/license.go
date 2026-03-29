package kernel

import (
	"slices"
	"time"
)

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

// ValidateLicense validates a license key and returns the license info.
// If the key is empty, returns a community license.
func ValidateLicense(key string) LicenseInfo {
	if key == "" {
		return CommunityLicense()
	}
	// Future: Parse JWT, validate HMAC-SHA256 signature, extract claims.
	// For now, return community license on any validation failure.
	return CommunityLicense()
}

// HasFeature checks if the license includes a specific feature.
func (l LicenseInfo) HasFeature(feature string) bool {
	return slices.Contains(l.Features, feature)
}

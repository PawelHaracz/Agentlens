// Package auth provides authentication and authorization utilities.
package auth

// Permission constants for role-based access control.
const (
	PermCatalogRead   = "catalog:read"
	PermCatalogWrite  = "catalog:write"
	PermCatalogDelete = "catalog:delete"
	PermUsersRead     = "users:read"
	PermUsersWrite    = "users:write"
	PermUsersDelete   = "users:delete"
	PermRolesRead     = "roles:read"
	PermRolesWrite    = "roles:write"
	PermSettingsRead  = "settings:read"
	PermSettingsWrite = "settings:write"

	// Service account management permissions (seeded in migration 010 for admin role).
	PermServiceAccountsRead   = "service_accounts:read"
	PermServiceAccountsWrite  = "service_accounts:write"
	PermServiceAccountsRevoke = "service_accounts:revoke"
)

// HasPermission checks whether the given permission list contains the required permission.
func HasPermission(perms []string, required string) bool {
	for _, p := range perms {
		if p == required {
			return true
		}
	}
	return false
}

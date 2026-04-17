package auth

import "sort"

// projectRolePermissions maps project role names to the permissions they grant.
// Static in-memory — no DB lookup. New roles = one entry here.
var projectRolePermissions = map[string][]string{
	"project:owner":     {PermCatalogRead, PermCatalogWrite, PermCatalogDelete},
	"project:developer": {PermCatalogRead, PermCatalogWrite},
	"project:viewer":    {PermCatalogRead},
}

// ProjectRoleHasPermission returns true if the project role grants the given permission.
func ProjectRoleHasPermission(role, permission string) bool {
	perms, ok := projectRolePermissions[role]
	if !ok {
		return false
	}
	return HasPermission(perms, permission)
}

// ValidProjectRoles returns all valid project-scoped role names in sorted order.
func ValidProjectRoles() []string {
	roles := make([]string, 0, len(projectRolePermissions))
	for r := range projectRolePermissions {
		roles = append(roles, r)
	}
	sort.Strings(roles)
	return roles
}

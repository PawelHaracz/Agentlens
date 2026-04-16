package auth_test

import (
	"testing"

	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestProjectRolePermissions(t *testing.T) {
	tests := []struct {
		role       string
		permission string
		want       bool
	}{
		{"project:owner", "catalog:read", true},
		{"project:owner", "catalog:write", true},
		{"project:owner", "catalog:delete", true},
		{"project:developer", "catalog:read", true},
		{"project:developer", "catalog:write", true},
		{"project:developer", "catalog:delete", false},
		{"project:viewer", "catalog:read", true},
		{"project:viewer", "catalog:write", false},
		{"project:viewer", "catalog:delete", false},
		{"project:unknown", "catalog:read", false},
	}
	for _, tc := range tests {
		t.Run(tc.role+"/"+tc.permission, func(t *testing.T) {
			got := auth.ProjectRoleHasPermission(tc.role, tc.permission)
			assert.Equal(t, tc.want, got)
		})
	}
}

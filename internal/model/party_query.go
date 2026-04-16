package model

// UserProjectMembership is a denormalized read-only view of a user's
// effective role on a project, collapsing direct and transitive paths.
// See ADR-014 for the role-resolution tie-break rule.
type UserProjectMembership struct {
	Project Party  `json:"project"`
	Role    string `json:"role"`
}

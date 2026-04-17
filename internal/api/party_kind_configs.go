package api

import (
	"github.com/PawelHaracz/agentlens/internal/auth"
	"github.com/PawelHaracz/agentlens/internal/model"
)

// PartyKindConfig drives route registration and handler behaviour for one party kind.
// Adding a new party kind = add a new PartyKindConfig registration in registerPartyRoutes.
// Zero handler code changes required.
type PartyKindConfig struct {
	Kind               model.PartyKind   // "group", "project"
	URLPrefix          string            // "groups", "projects"
	MemberRelationship string            // relationship_name for members
	ValidMemberRoles   []string          // roles a member may hold
	CreatePermission   string            // global permission required to create
	ManagePermission   string            // global permission required to delete/manage
	CanContainKinds    []model.PartyKind // which party kinds may be members
}

// groupKindConfig is the PartyKindConfig for groups.
var groupKindConfig = PartyKindConfig{
	Kind:               model.PartyKindGroup,
	URLPrefix:          "groups",
	MemberRelationship: "group_member",
	ValidMemberRoles:   []string{"member"},
	CreatePermission:   auth.PermUsersWrite,
	ManagePermission:   auth.PermUsersWrite,
	CanContainKinds:    []model.PartyKind{model.PartyKindPerson, model.PartyKindGroup},
}

// projectKindConfig is the PartyKindConfig for projects.
var projectKindConfig = PartyKindConfig{
	Kind:               model.PartyKindProject,
	URLPrefix:          "projects",
	MemberRelationship: "project_member",
	ValidMemberRoles:   []string{"project:owner", "project:developer", "project:viewer"},
	CreatePermission:   auth.PermCatalogWrite,
	ManagePermission:   auth.PermCatalogWrite,
	CanContainKinds:    []model.PartyKind{model.PartyKindPerson, model.PartyKindGroup},
}

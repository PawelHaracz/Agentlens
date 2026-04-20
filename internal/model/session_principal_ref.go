package model

// PrincipalType discriminates how a session principal was authenticated.
type PrincipalType string

const (
	PrincipalTypeUserLocal      PrincipalType = "user_local"
	PrincipalTypeUserFederated  PrincipalType = "user_federated"
	PrincipalTypeServiceAccount PrincipalType = "service_account"
)

// SessionPrincipalRef is an opaque reference to an authenticated principal, safe
// to pass across arch-go layer boundaries (including into plugins/mcpserver/).
// It is produced by composition-root middleware (internal/api) and placed into
// request context via ctxkey.SessionPrincipalRef.
//
// Plugins must not resolve this back to auth.Principal — they consume it as-is
// for session creation, audit logging, and project-scoped catalog filtering.
type SessionPrincipalRef struct {
	// ID is the canonical identifier: UserID for user principals, party ID for
	// service accounts.
	ID string

	// Kind mirrors PrincipalType; string typed for JSON serialisation without
	// coupling consumers to the enum.
	Kind PrincipalType

	// PartyID is the party graph node for this principal (person party for users,
	// service-account party for API-key principals).
	PartyID string

	// Permissions is the flat permission list derived from the role / scopes at
	// the time of authentication. Authorisation middleware re-checks this against
	// the required permission for each route.
	Permissions []string

	// AccessibleProjectIDs is the set of project party IDs visible to this
	// principal. Set by ScopeByAccessibleProjects middleware. Empty slice means
	// access restricted to the default project only.
	AccessibleProjectIDs []string

	// AuthMethod is an informational field used in audit logs. Examples:
	// "api_key", "jwt_local", "jwt_federated:dex".
	AuthMethod string
}

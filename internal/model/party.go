package model

import "time"

// PartyKind discriminates the type of party stored in the parties table.
type PartyKind string

const (
	PartyKindPerson  PartyKind = "person"
	PartyKindGroup   PartyKind = "group"
	PartyKindProject PartyKind = "project"
)

// ValidPartyKinds is the set of allowed PartyKind values.
var ValidPartyKinds = map[PartyKind]bool{
	PartyKindPerson:  true,
	PartyKindGroup:   true,
	PartyKindProject: true,
}

// ContainmentRelationships is the set of relationship names that form hierarchical
// containment and trigger party_group_closures rebuild.
// Adding a new hierarchical party kind = add its relationship name here.
var ContainmentRelationships = map[string]bool{
	"group_member": true,
}

// ContainmentRelationshipNames returns ContainmentRelationships as a string slice.
// Used for SQL IN clauses in closure rebuild.
func ContainmentRelationshipNames() []string {
	names := make([]string, 0, len(ContainmentRelationships))
	for name := range ContainmentRelationships {
		names = append(names, name)
	}
	return names
}

// Party is the unified actor. Person parties link 1:1 to User records.
// Group and Project parties are standalone.
type Party struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	Kind      PartyKind `gorm:"not null;type:text" json:"kind"`
	Name      string    `gorm:"not null;type:text" json:"name"`
	Version   int       `gorm:"not null;default:0" json:"-"`
	UserID    *string   `gorm:"uniqueIndex;type:text" json:"-"`
	IsSystem  bool      `gorm:"default:false" json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PartyRelationship is a directed named edge in the party graph.
// UNIQUE(FromPartyID, FromRole, ToPartyID, ToRole, RelationshipName) is enforced in migration.
type PartyRelationship struct {
	ID               string    `gorm:"primaryKey;type:text" json:"id"`
	FromPartyID      string    `gorm:"not null;type:text;index" json:"from_party_id"`
	FromRole         string    `gorm:"not null;type:text" json:"from_role"`
	ToPartyID        string    `gorm:"not null;type:text;index" json:"to_party_id"`
	ToRole           string    `gorm:"not null;type:text" json:"to_role"`
	RelationshipName string    `gorm:"not null;type:text" json:"relationship_name"`
	CreatedAt        time.Time `json:"created_at"`
}

// PartyGroupClosure is the pre-computed transitive closure of containment relationships.
// Never edit directly — managed exclusively by PartyStore.rebuildAllClosures.
type PartyGroupClosure struct {
	MemberPartyID   string `gorm:"primaryKey;type:text" json:"-"`
	AncestorPartyID string `gorm:"primaryKey;type:text" json:"-"`
}

// GlobalPartyRole assigns a global system role to a group party.
// Person parties derive global roles from User.RoleID (unchanged).
type GlobalPartyRole struct {
	PartyID string `gorm:"primaryKey;type:text" json:"party_id"`
	RoleID  string `gorm:"primaryKey;type:text" json:"role_id"`
}

// CatalogProjectMembership links a catalog entry to a project party (many-to-many).
type CatalogProjectMembership struct {
	CatalogEntryID string    `gorm:"primaryKey;type:text" json:"catalog_entry_id"`
	ProjectPartyID string    `gorm:"primaryKey;type:text" json:"project_party_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// PartyIdentifier stores external identifiers for parties.
// Schema added in v1, not populated until v2.
type PartyIdentifier struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	PartyID   string    `gorm:"not null;type:text;index" json:"party_id"`
	Kind      string    `gorm:"not null;type:text" json:"kind"`
	Value     string    `gorm:"not null;type:text" json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

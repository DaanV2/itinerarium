package models

// RepositoryType selects which visibility rule a knowledge Repository
// follows (see the Repository entity in docs/architecture.md).
type RepositoryType string

// The known repository types. General and template are campaign-wide
// singletons; group and character repositories are provisioned one-for-one
// when their owning Group/Character is created.
const (
	RepositoryTypeGeneral   RepositoryType = "general"
	RepositoryTypeTemplate  RepositoryType = "template"
	RepositoryTypeGroup     RepositoryType = "group"
	RepositoryTypeCharacter RepositoryType = "character"
)

// Valid reports whether t is one of the known repository types.
func (t RepositoryType) Valid() bool {
	switch t {
	case RepositoryTypeGeneral, RepositoryTypeTemplate, RepositoryTypeGroup, RepositoryTypeCharacter:
		return true
	default:
		return false
	}
}

// Repository is a named knowledge vault holding a folder tree of Documents
// (from a later M3 step). General and template repositories are visible to
// everyone; a group repository is visible to its members; a character
// repository is visible to its owner. GMs always see every repository.
//
// GroupID is set only when Type is RepositoryTypeGroup, CharacterID only
// when Type is RepositoryTypeCharacter — the service layer enforces that
// invariant when provisioning.
type Repository struct {
	Model
	Type        RepositoryType `gorm:"not null;index" json:"type"`
	GroupID     *string        `gorm:"type:uuid;uniqueIndex" json:"group_id,omitempty"`
	Group       *Group         `json:"-"`
	CharacterID *string        `gorm:"type:uuid;uniqueIndex" json:"character_id,omitempty"`
	Character   *Character     `json:"-"`
}

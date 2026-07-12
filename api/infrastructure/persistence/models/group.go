package models

// GroupType labels a group for display purposes only — every type shares the
// exact same mechanics (core domain rule 6).
type GroupType string

// The known group types. Cosmetic: behaviour is identical for all of them.
const (
	GroupTypeOrganization GroupType = "organization"
	GroupTypeFamily       GroupType = "family"
	GroupTypeOther        GroupType = "other"
)

// Valid reports whether t is one of the known group types.
func (t GroupType) Valid() bool {
	switch t {
	case GroupTypeOrganization, GroupTypeFamily, GroupTypeOther:
		return true
	default:
		return false
	}
}

// Group is a named collection of characters. Members share the group's
// inventory, money, and (from M3) knowledge repository. Access always follows
// *current* membership; the membership history lives in ActivityEntry rows.
type Group struct {
	Model
	Name        string      `gorm:"not null" json:"name"`
	Type        GroupType   `gorm:"not null" json:"type"`
	Description string      `json:"description,omitempty"`
	Members     []Character `gorm:"many2many:group_members" json:"-"`
}

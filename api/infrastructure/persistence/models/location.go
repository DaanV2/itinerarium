package models

// Location is a named place in the campaign world — a plane, town, building,
// room, or anything physical. Locations form a hierarchy through the optional
// ParentID: a location with no parent is a top-level plane, and nesting a
// location under another models "this room is inside this building is on this
// plane". Multi-plane campaigns fall out naturally from having several
// parent-less roots.
//
// M1 scopes locations to name + description + hierarchy, GM-managed. Location
// inventories (M2) and access control / player editing (M3) build on top of
// this model later; see docs/architecture.md.
type Location struct {
	Model
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description,omitempty"`
	ParentID    *string   `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	Parent      *Location `json:"-"`
}

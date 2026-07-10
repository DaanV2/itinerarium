package models

// Location is a named place — a plane, town, building, or room. Plane is a
// free-text label that groups locations into planes of existence, giving
// multi-plane campaigns structure without a separate entity.
//
// Visibility is access-controlled: GMs always see every location; a character
// sees one only through a LocationAccess grant (direct or via a group). No
// grant means the location's existence stays hidden (core domain rule 3).
type Location struct {
	Model
	Name        string `gorm:"not null" json:"name"`
	Description string `json:"description,omitempty"`
	Plane       string `json:"plane,omitempty"`
}

// LocationAccess grants a character — directly or through a group — the
// single access level a location has: view and modify, including its
// inventory (M2 roadmap: "single level (view + modify)"). Exactly one of
// CharacterID/GroupID is set; the service enforces that invariant.
type LocationAccess struct {
	Model
	LocationID  string  `gorm:"type:uuid;index;not null" json:"location_id"`
	CharacterID *string `gorm:"type:uuid;index" json:"character_id,omitempty"`
	GroupID     *string `gorm:"type:uuid;index" json:"group_id,omitempty"`
}

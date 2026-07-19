package models

// Location is a named place — a plane, town, building, or room. Plane is a
// free-text label that groups locations into planes of existence, giving
// multi-plane campaigns structure without a separate entity.
//
// Visibility is access-controlled: GMs always see every location; a character
// sees one only through a LocationAccess grant (direct or via a group). No
// grant means the location's existence stays hidden (core domain rule 3).
//
// The location's descriptive content lives in Sections, gated the same way a
// Document is: SharedOnGameDay controls when the whole description becomes
// visible to a character with location access, and sections flagged GMOnly
// are stripped server-side for players (core domain rules 1, 2).
type Location struct {
	Model
	Name            string            `gorm:"not null" json:"name"`
	Plane           string            `json:"plane,omitempty"`
	SharedOnGameDay int               `gorm:"not null;default:0" json:"shared_on_game_day"`
	Sections        []LocationSection `json:"sections"`
}

// LocationSection is one ordered slice of a Location's description content.
// GMOnly sections never reach non-GM clients — the service layer strips them
// before the response is built, mirroring DocumentSection.
type LocationSection struct {
	Model
	LocationID string `gorm:"type:uuid;index;not null" json:"location_id"`
	Position   int    `gorm:"not null" json:"position"`
	GMOnly     bool   `gorm:"not null;default:false" json:"gm_only"`
	Content    string `gorm:"not null" json:"content"`
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

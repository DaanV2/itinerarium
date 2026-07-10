package models

// Character belongs to a User; an account may own multiple characters.
// CurrentGameDay tracks each character's progress through the campaign
// independently of every other character and gates document/activity
// visibility (see docs/architecture.md).
//
// LocationID optionally associates the character with a single Location. It is
// a nullable pointer: nil means the character is not currently placed anywhere.
type Character struct {
	Model
	Name           string    `gorm:"not null" json:"name"`
	CurrentGameDay int       `gorm:"not null;default:0" json:"current_game_day"`
	UserID         string    `gorm:"type:uuid;index;not null" json:"user_id"`
	User           User      `json:"-"`
	LocationID     *string   `gorm:"type:uuid;index" json:"location_id,omitempty"`
	Location       *Location `json:"-"`
}

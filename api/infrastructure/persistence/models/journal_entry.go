package models

// JournalEntry is a per-character journal page, stamped with the character's
// current_game_day at creation time. Readable and editable only by the
// owning player and GMs (see docs/architecture.md).
type JournalEntry struct {
	Model
	CharacterID string    `gorm:"type:uuid;index;not null" json:"character_id"`
	Character   Character `json:"-"`
	GameDay     int       `gorm:"not null" json:"game_day"`
	Content     string    `gorm:"not null" json:"content"`
}

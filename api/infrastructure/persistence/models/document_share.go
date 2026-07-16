package models

// DocumentShare grants one specific character access to a Document,
// independent of the document's own Repository access rule, revealed once
// that character's current_game_day reaches SharedOnGameDay. This is the
// direct-share path (core domain rule 1, roadmap M3): a GM can hand a single
// character a document they otherwise couldn't reach through their
// repositories.
type DocumentShare struct {
	Model
	DocumentID      string    `gorm:"type:uuid;index;not null" json:"document_id"`
	Document        Document  `json:"-"`
	CharacterID     string    `gorm:"type:uuid;index;not null" json:"character_id"`
	Character       Character `json:"-"`
	SharedOnGameDay int       `gorm:"not null;default:0" json:"shared_on_game_day"`
}

package models

// Session links a set of characters to a play event. It is a GM tool: the GM
// tracks who took part and, over the course of play, advances (or rewinds)
// game day for every participant at once or for one character catching up.
// A Session does not itself carry a game day — each Character's own
// CurrentGameDay is the source of truth (see docs/architecture.md).
type Session struct {
	Model
	Name         string      `gorm:"not null" json:"name"`
	Description  string      `json:"description,omitempty"`
	Participants []Character `gorm:"many2many:session_participants" json:"-"`
}

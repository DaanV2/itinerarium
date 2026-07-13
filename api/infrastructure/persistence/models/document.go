package models

// Document is a markdown knowledge page living at a folder path inside
// exactly one Repository. Its content is an ordered list of
// DocumentSections; sections flagged gm_only are stripped server-side for
// players (core domain rule 2). Visibility is gated by the repository's own
// access rule combined with SharedOnGameDay (core domain rule 1).
//
// Documents are not versioned: the game day gates whether a character sees
// the document, and everyone who can see it sees the latest content.
type Document struct {
	Model
	RepositoryID string     `gorm:"type:uuid;index;not null" json:"repository_id"`
	Repository   Repository `json:"-"`
	// Path is the slash-separated folder path plus file name inside the
	// repository (e.g. "factions/thieves-guild"). Duplicate paths within a
	// repository are warned about, not blocked (core domain rule 7).
	Path            string   `gorm:"not null;index" json:"path"`
	Title           string   `gorm:"not null" json:"title"`
	Tags            []string `gorm:"serializer:json" json:"tags"`
	SharedOnGameDay int      `gorm:"not null;default:0" json:"shared_on_game_day"`
	// Version increments on every save; editors echo it back so a save over
	// someone else's concurrent edit warns first (core domain rule 7).
	Version  int               `gorm:"not null;default:1" json:"version"`
	Sections []DocumentSection `json:"sections"`
}

// DocumentSection is one ordered slice of a Document's markdown content.
// GMOnly sections never reach non-GM clients — the service layer strips them
// before the response is built.
type DocumentSection struct {
	Model
	DocumentID string `gorm:"type:uuid;index;not null" json:"document_id"`
	Position   int    `gorm:"not null" json:"position"`
	GMOnly     bool   `gorm:"not null;default:false" json:"gm_only"`
	Content    string `gorm:"not null" json:"content"`
}

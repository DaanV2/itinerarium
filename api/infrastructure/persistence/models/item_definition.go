package models

// ItemDefinition is an entry in the GM's default item catalog, seeded from a
// JSON/YAML file or added at runtime. It is a convenience for players picking
// known items, never a restriction: inventory items may reference a definition
// or be free-text (see docs/architecture.md and core domain rule 8).
type ItemDefinition struct {
	Model
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

package models

// InventoryItem is one line in a character's personal inventory: a named item
// and a quantity. Name is always required; ItemDefinitionID optionally links
// the line to a catalog entry, but free-text items (no definition) are always
// allowed (core domain rule 8).
//
// M1 scopes inventories to characters only. M2 generalises inventories to
// groups and locations and adds item movement between them.
type InventoryItem struct {
	Model
	CharacterID      string  `gorm:"type:uuid;index;not null" json:"character_id"`
	Name             string  `gorm:"not null" json:"name"`
	ItemDefinitionID *string `gorm:"type:uuid;index" json:"item_definition_id,omitempty"`
	Quantity         int     `gorm:"not null;default:1" json:"quantity"`
	Description      string  `json:"description,omitempty"`
}

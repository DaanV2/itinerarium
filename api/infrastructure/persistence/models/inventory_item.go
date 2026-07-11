package models

// InventoryItem is one line in an inventory: a named item and a quantity,
// owned by exactly one character, group, or location (the embedded
// InventoryOwner). Name is always required; ItemDefinitionID optionally links
// the line to a catalog entry, but free-text items (no definition) are always
// allowed (core domain rule 8).
type InventoryItem struct {
	Model
	InventoryOwner
	Name             string  `gorm:"not null" json:"name"`
	ItemDefinitionID *string `gorm:"type:uuid;index" json:"item_definition_id,omitempty"`
	Quantity         int     `gorm:"not null;default:1" json:"quantity"`
	Description      string  `json:"description,omitempty"`
}

package models

// InventoryOwner identifies which inventory a line belongs to: a character's
// personal inventory, a group's shared inventory, or a location's inventory.
// Exactly one field is set; the application layer enforces that invariant.
// It is embedded in InventoryItem (GORM flattens it into the same table) and
// doubles as the address of an inventory when listing or moving items.
type InventoryOwner struct {
	CharacterID *string `gorm:"type:uuid;index" json:"character_id,omitempty"`
	GroupID     *string `gorm:"type:uuid;index" json:"group_id,omitempty"`
	LocationID  *string `gorm:"type:uuid;index" json:"location_id,omitempty"`
}

// CharacterOwner addresses a character's personal inventory.
func CharacterOwner(id string) InventoryOwner { return InventoryOwner{CharacterID: &id} }

// GroupOwner addresses a group's shared inventory.
func GroupOwner(id string) InventoryOwner { return InventoryOwner{GroupID: &id} }

// LocationOwner addresses a location's inventory.
func LocationOwner(id string) InventoryOwner { return InventoryOwner{LocationID: &id} }

// Valid reports whether exactly one owner field is set.
func (o InventoryOwner) Valid() bool {
	count := 0
	for _, id := range []*string{o.CharacterID, o.GroupID, o.LocationID} {
		if id != nil {
			count++
		}
	}

	return count == 1
}

// Equal reports whether two owners address the same inventory.
func (o InventoryOwner) Equal(other InventoryOwner) bool {
	return equalID(o.CharacterID, other.CharacterID) &&
		equalID(o.GroupID, other.GroupID) &&
		equalID(o.LocationID, other.LocationID)
}

func equalID(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}

package models

// Currency is a GM-defined unit of money, shared by every inventory
// (character, group, location). The GM seeds the catalog from a JSON/YAML file
// or adds entries at runtime; see docs/architecture.md.
//
// Ratio expresses the value of one unit in the campaign's base unit — the
// smallest denomination, which itself has Ratio 1. For "1 gold = 10 silver =
// 100 copper", copper is the base (Ratio 1), silver is Ratio 10, gold is
// Ratio 100. Storing an integer ratio keeps all money arithmetic in whole
// base units, avoiding floating-point rounding on balances.
type Currency struct {
	Model
	Code  string `gorm:"uniqueIndex;not null" json:"code"`
	Name  string `gorm:"not null" json:"name"`
	Ratio int64  `gorm:"not null;default:1" json:"ratio"`
}

package models

// MoneyBalance is a character's holding of a single Currency. Amount is stored
// in that currency's own unit (not converted to base units). A character has
// at most one balance per currency, enforced by the composite unique index.
//
// M1 scopes money to characters only; M2 adds group and location money.
type MoneyBalance struct {
	Model
	CharacterID string   `gorm:"type:uuid;not null;uniqueIndex:idx_money_character_currency" json:"character_id"`
	CurrencyID  string   `gorm:"type:uuid;not null;uniqueIndex:idx_money_character_currency" json:"currency_id"`
	Amount      int64    `gorm:"not null;default:0" json:"amount"`
	Currency    Currency `json:"-"`
}

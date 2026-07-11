package models

// MoneyBalance is a character's or group's holding of a single Currency
// (locations store items, not money). Exactly one of CharacterID/GroupID is
// set; the application layer enforces that. Amount is stored in that
// currency's own unit (not converted to base units). An owner has at most one
// balance per currency, enforced by the composite unique indexes.
type MoneyBalance struct {
	Model
	CharacterID *string  `gorm:"type:uuid;uniqueIndex:idx_money_character_currency" json:"character_id,omitempty"`
	GroupID     *string  `gorm:"type:uuid;uniqueIndex:idx_money_group_currency" json:"group_id,omitempty"`
	CurrencyID  string   `gorm:"type:uuid;not null;uniqueIndex:idx_money_character_currency;uniqueIndex:idx_money_group_currency" json:"currency_id"`
	Amount      int64    `gorm:"not null;default:0" json:"amount"`
	Currency    Currency `json:"-"`
}

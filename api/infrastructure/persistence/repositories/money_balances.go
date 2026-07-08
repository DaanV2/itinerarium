package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// MoneyBalances provides access to per-character, per-currency money holdings.
type MoneyBalances struct{ db *persistence.Database }

// NewMoneyBalances builds a MoneyBalances repository.
func NewMoneyBalances(db *persistence.Database) *MoneyBalances {
	return &MoneyBalances{db: db}
}

// ListByCharacter returns every balance held by the character, ordered by the
// currency's ratio (highest-value denomination first).
func (r *MoneyBalances) ListByCharacter(ctx context.Context, characterID string) ([]models.MoneyBalance, error) {
	var balances []models.MoneyBalance

	err := r.db.DB().WithContext(ctx).
		Joins("Currency").
		Where("money_balances.character_id = ?", characterID).
		Order(`"Currency".ratio DESC, "Currency".code`).
		Find(&balances).Error
	if err != nil {
		return nil, err
	}

	return balances, nil
}

// GetByCharacterAndCurrency returns the character's balance in one currency,
// or ErrNotFound if no balance row exists yet.
func (r *MoneyBalances) GetByCharacterAndCurrency(
	ctx context.Context, characterID, currencyID string,
) (*models.MoneyBalance, error) {
	var balance models.MoneyBalance

	err := r.db.DB().WithContext(ctx).
		First(&balance, "character_id = ? AND currency_id = ?", characterID, currencyID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &balance, nil
}

// Set upserts the character's balance in one currency to an absolute amount.
func (r *MoneyBalances) Set(ctx context.Context, characterID, currencyID string, amount int64) (*models.MoneyBalance, error) {
	balance, err := r.GetByCharacterAndCurrency(ctx, characterID, currencyID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}

		balance = &models.MoneyBalance{CharacterID: characterID, CurrencyID: currencyID, Amount: amount}
		if createErr := r.db.DB().WithContext(ctx).Create(balance).Error; createErr != nil {
			return nil, createErr
		}

		return balance, nil
	}

	balance.Amount = amount
	if saveErr := r.db.DB().WithContext(ctx).Save(balance).Error; saveErr != nil {
		return nil, saveErr
	}

	return balance, nil
}

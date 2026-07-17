package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// MoneyBalances provides access to per-currency money holdings of characters
// and groups. Owners are addressed with models.InventoryOwner; the service
// layer guarantees only character or group owners reach this repository.
type MoneyBalances struct{ db *persistence.Database }

// NewMoneyBalances builds a MoneyBalances repository.
func NewMoneyBalances(db *persistence.Database) *MoneyBalances {
	return &MoneyBalances{db: db}
}

// moneyOwnerScope narrows a query to one owner's balances.
func moneyOwnerScope(query *gorm.DB, owner models.InventoryOwner) *gorm.DB {
	if owner.CharacterID != nil {
		return query.Where("money_balances.character_id = ?", *owner.CharacterID)
	}

	return query.Where("money_balances.group_id = ?", *owner.GroupID)
}

// ListByOwner returns every balance held by the owner, ordered by the
// currency's ratio (highest-value denomination first).
func (r *MoneyBalances) ListByOwner(
	ctx context.Context, owner models.InventoryOwner,
) ([]models.MoneyBalance, error) {
	var balances []models.MoneyBalance

	err := moneyOwnerScope(r.db.DB().WithContext(ctx).Joins("Currency"), owner).
		Order(`"Currency".ratio DESC, "Currency".code`).
		Find(&balances).Error
	if err != nil {
		return nil, err
	}

	return balances, nil
}

// GetByOwnerAndCurrency returns the owner's balance in one currency, or
// ErrNotFound if no balance row exists yet.
func (r *MoneyBalances) GetByOwnerAndCurrency(
	ctx context.Context, owner models.InventoryOwner, currencyID string,
) (*models.MoneyBalance, error) {
	var balance models.MoneyBalance

	err := moneyOwnerScope(r.db.DB().WithContext(ctx), owner).
		Where("currency_id = ?", currencyID).
		First(&balance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &balance, nil
}

// Set upserts the owner's balance in one currency to an absolute amount,
// recording any given activity entries in the same transaction.
func (r *MoneyBalances) Set(
	ctx context.Context, owner models.InventoryOwner, currencyID string, amount int64,
	entries ...*models.ActivityEntry,
) (*models.MoneyBalance, error) {
	balance, err := r.GetByOwnerAndCurrency(ctx, owner, currencyID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}

		balance = &models.MoneyBalance{
			CharacterID: owner.CharacterID,
			GroupID:     owner.GroupID,
			CurrencyID:  currencyID,
			Amount:      amount,
		}
	} else {
		balance.Amount = amount
	}

	err = r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if saveErr := tx.Save(balance).Error; saveErr != nil {
			return saveErr
		}

		return createEntries(tx, entries)
	})
	if err != nil {
		return nil, err
	}

	return balance, nil
}

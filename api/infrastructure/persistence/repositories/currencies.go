//nolint:dupl // Currencies and ItemDefinitions are parallel catalog repositories with the same CRUD+upsert shape; the one-file-per-entity convention keeps them separate.
package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Currencies provides access to the GM-defined currency catalog.
type Currencies struct{ db *persistence.Database }

// NewCurrencies builds a Currencies repository.
func NewCurrencies(db *persistence.Database) *Currencies {
	return &Currencies{db: db}
}

// Create persists a new currency.
func (r *Currencies) Create(ctx context.Context, c *models.Currency) error {
	err := r.db.DB().WithContext(ctx).Create(c).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a currency by ID, returning ErrNotFound if none matches.
func (r *Currencies) GetByID(ctx context.Context, id string) (*models.Currency, error) {
	var c models.Currency

	err := r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &c, nil
}

// GetByCode looks up a currency by its unique code, returning ErrNotFound if
// none matches.
func (r *Currencies) GetByCode(ctx context.Context, code string) (*models.Currency, error) {
	var c models.Currency

	err := r.db.DB().WithContext(ctx).First(&c, "code = ?", code).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &c, nil
}

// List returns every currency, ordered by ratio then code.
func (r *Currencies) List(ctx context.Context) ([]models.Currency, error) {
	var currencies []models.Currency

	err := r.db.DB().WithContext(ctx).Order("ratio, code").Find(&currencies).Error
	if err != nil {
		return nil, err
	}

	return currencies, nil
}

// UpsertByCode inserts a currency or, when one with the same code already
// exists, updates its name and ratio. Used to seed the catalog from a file
// idempotently across restarts.
func (r *Currencies) UpsertByCode(ctx context.Context, c *models.Currency) error {
	err := r.db.DB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "ratio", "updated_at"}),
	}).Create(c).Error
	if err != nil {
		return err
	}

	return nil
}

package application

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"
)

// ErrInvalidCurrency is returned when a currency is created with an empty code
// or a ratio below 1.
var ErrInvalidCurrency = serviceErr(KindValidation, "invalid currency")

// ErrCurrencyExists is returned when creating a currency whose code is already
// in use.
var ErrCurrencyExists = serviceErr(KindConflict, "currency code already in use")

// ErrItemDefinitionExists is returned when creating a catalog item whose name
// is already in use.
var ErrItemDefinitionExists = serviceErr(KindConflict, "item definition name already in use")

// ErrNoAmounts is returned when a conversion/simplification request carries
// no currency amounts to work with.
var ErrNoAmounts = serviceErr(KindValidation, "no amounts given")

// CatalogService owns the GM-defined currency and item catalogs. Currencies
// carry conversion ratios; item definitions are a convenience for players and
// never restrict free-text inventory items (core domain rule 8). Both catalogs
// are readable by any authenticated user; only a GM may add entries. The
// catalog can also be seeded from a JSON/YAML file at startup.
type CatalogService struct {
	currencies *repositories.Currencies
	itemDefs   *repositories.ItemDefinitions
}

// NewCatalogService builds a CatalogService.
func NewCatalogService(currencies *repositories.Currencies, itemDefs *repositories.ItemDefinitions) *CatalogService {
	return &CatalogService{currencies: currencies, itemDefs: itemDefs}
}

// ListCurrencies returns the whole currency catalog. Currencies are
// campaign-wide and not secret, so any authenticated caller may read them.
func (s *CatalogService) ListCurrencies(ctx context.Context) ([]models.Currency, error) {
	currencies, err := s.currencies.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing currencies: %w", err)
	}

	return currencies, nil
}

// ListItemDefinitions returns the whole item catalog. It is not secret, so any
// authenticated caller may read it.
func (s *CatalogService) ListItemDefinitions(ctx context.Context) ([]models.ItemDefinition, error) {
	defs, err := s.itemDefs.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing item definitions: %w", err)
	}

	return defs, nil
}

// CreateCurrency adds a currency to the catalog. Only a GM may call this.
func (s *CatalogService) CreateCurrency(
	ctx context.Context, requester Requester, code, name string, ratio int64,
) (*models.Currency, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if code == "" || name == "" || ratio < 1 {
		return nil, ErrInvalidCurrency
	}

	_, err := s.currencies.GetByCode(ctx, code)
	if err == nil {
		return nil, ErrCurrencyExists
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		return nil, fmt.Errorf("checking existing currency: %w", err)
	}

	currency := &models.Currency{Code: code, Name: name, Ratio: ratio}
	if err := s.currencies.Create(ctx, currency); err != nil {
		return nil, fmt.Errorf("creating currency: %w", err)
	}

	return currency, nil
}

// CreateItemDefinition adds an item to the catalog. Only a GM may call this.
func (s *CatalogService) CreateItemDefinition(
	ctx context.Context, requester Requester, name, description, category string,
) (*models.ItemDefinition, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if name == "" {
		return nil, ErrInvalidName
	}

	_, err := s.itemDefs.GetByName(ctx, name)
	if err == nil {
		return nil, ErrItemDefinitionExists
	}
	if !errors.Is(err, repositories.ErrNotFound) {
		return nil, fmt.Errorf("checking existing item definition: %w", err)
	}

	def := &models.ItemDefinition{Name: name, Description: description, Category: category}
	if err := s.itemDefs.Create(ctx, def); err != nil {
		return nil, fmt.Errorf("creating item definition: %w", err)
	}

	return def, nil
}

// CurrencyAmount is one entry in a conversion, addition, or simplification
// request: an amount in a currency identified by its ID or its code.
type CurrencyAmount struct {
	Currency string
	Amount   int64
}

// ConversionResult is the outcome of converting/adding a set of currency
// amounts into a single target currency. Whole is how many units of the
// target currency the total is worth; Remainder is whatever is left over
// expressed in base units, since it may not be a whole number of the target
// currency's unit (e.g. 137 cp into gp at ratio 100 leaves 37 cp over).
type ConversionResult struct {
	Currency  models.Currency
	Whole     int64
	Remainder int64
	BaseValue int64
}

// SimplifiedAmount is one denomination in a Simplify breakdown.
type SimplifiedAmount struct {
	Currency models.Currency
	Amount   int64
}

// resolveCurrency looks a currency up by ID, falling back to its code. Either
// form is accepted so callers can use whichever they have on hand. The ID
// lookup only runs when idOrCode actually parses as a UUID — Postgres/MySQL
// reject a non-UUID value against a uuid column outright, so a currency code
// must go straight to GetByCode rather than through a doomed ID lookup.
func (s *CatalogService) resolveCurrency(ctx context.Context, idOrCode string) (*models.Currency, error) {
	if _, err := uuid.Parse(idOrCode); err == nil {
		c, err := s.currencies.GetByID(ctx, idOrCode)
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, repositories.ErrNotFound) {
			return nil, fmt.Errorf("loading currency: %w", err)
		}
	}

	c, err := s.currencies.GetByCode(ctx, idOrCode)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			// In the catalog, an unknown currency is a lookup miss (404); the
			// same sentinel is a 400 when referenced by an inventory write.
			return nil, withKind(KindNotFound, ErrUnknownCurrency)
		}

		return nil, fmt.Errorf("loading currency: %w", err)
	}

	return c, nil
}

// baseValue sums a set of currency amounts into the campaign's base unit
// (amount × ratio for each entry). Every currency must exist in the catalog
// and every amount must be non-negative.
func (s *CatalogService) baseValue(ctx context.Context, amounts []CurrencyAmount) (int64, error) {
	if len(amounts) == 0 {
		return 0, ErrNoAmounts
	}

	var total int64
	for _, a := range amounts {
		if a.Amount < 0 {
			return 0, ErrInvalidAmount
		}

		c, err := s.resolveCurrency(ctx, a.Currency)
		if err != nil {
			return 0, err
		}

		total += a.Amount * c.Ratio
	}

	return total, nil
}

// Convert adds up amounts (one currency amount, or several to add together)
// and expresses the total in the target currency. Any authenticated user may
// call this — currencies and their ratios are not secret.
func (s *CatalogService) Convert(ctx context.Context, amounts []CurrencyAmount, target string) (*ConversionResult, error) {
	total, err := s.baseValue(ctx, amounts)
	if err != nil {
		return nil, err
	}

	targetCurrency, err := s.resolveCurrency(ctx, target)
	if err != nil {
		return nil, err
	}

	return &ConversionResult{
		Currency:  *targetCurrency,
		Whole:     total / targetCurrency.Ratio,
		Remainder: total % targetCurrency.Ratio,
		BaseValue: total,
	}, nil
}

// Simplify adds up amounts and re-expresses the total as the fewest coins
// across the whole catalog: greedily filling the highest-ratio currency
// first, then the next, down to the base unit. Denominations the total
// doesn't need are omitted.
func (s *CatalogService) Simplify(ctx context.Context, amounts []CurrencyAmount) ([]SimplifiedAmount, error) {
	total, err := s.baseValue(ctx, amounts)
	if err != nil {
		return nil, err
	}

	catalog, err := s.currencies.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing currencies: %w", err)
	}

	breakdown := make([]SimplifiedAmount, 0, len(catalog))
	for i := len(catalog) - 1; i >= 0; i-- {
		c := catalog[i]

		units := total / c.Ratio
		if units == 0 {
			continue
		}

		total -= units * c.Ratio
		breakdown = append(breakdown, SimplifiedAmount{Currency: c, Amount: units})
	}

	return breakdown, nil
}

// catalogFile is the on-disk shape of a seed file. YAML is a superset of JSON,
// so the same decoder handles .yaml, .yml, and .json files.
type catalogFile struct {
	Currencies []struct {
		Code  string `yaml:"code" json:"code"`
		Name  string `yaml:"name" json:"name"`
		Ratio int64  `yaml:"ratio" json:"ratio"`
	} `yaml:"currencies" json:"currencies"`
	Items []struct {
		Name        string `yaml:"name" json:"name"`
		Description string `yaml:"description" json:"description"`
		Category    string `yaml:"category" json:"category"`
	} `yaml:"items" json:"items"`
}

// LoadFile seeds the catalog from a JSON/YAML file, upserting so it is safe to
// run on every startup. Currencies are matched by code, items by name. It
// returns the number of currencies and items applied.
func (s *CatalogService) LoadFile(ctx context.Context, path string) (currencies, items int, err error) {
	raw, err := os.ReadFile(path) //nolint:gosec // operator-supplied catalog path, read-only
	if err != nil {
		return 0, 0, fmt.Errorf("reading catalog file: %w", err)
	}

	var file catalogFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return 0, 0, fmt.Errorf("parsing catalog file: %w", err)
	}

	for _, c := range file.Currencies {
		if c.Code == "" || c.Name == "" || c.Ratio < 1 {
			return currencies, items, fmt.Errorf("%w: code=%q name=%q ratio=%d", ErrInvalidCurrency, c.Code, c.Name, c.Ratio)
		}

		currency := &models.Currency{Code: c.Code, Name: c.Name, Ratio: c.Ratio}
		if err := s.currencies.UpsertByCode(ctx, currency); err != nil {
			return currencies, items, fmt.Errorf("seeding currency %q: %w", c.Code, err)
		}

		currencies++
	}

	for _, item := range file.Items {
		if item.Name == "" {
			return currencies, items, fmt.Errorf("%w: item with empty name", ErrInvalidName)
		}

		def := &models.ItemDefinition{Name: item.Name, Description: item.Description, Category: item.Category}
		if err := s.itemDefs.UpsertByName(ctx, def); err != nil {
			return currencies, items, fmt.Errorf("seeding item %q: %w", item.Name, err)
		}

		items++
	}

	return currencies, items, nil
}

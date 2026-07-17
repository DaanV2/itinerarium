package application_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCatalogEnv(t *testing.T) *application.CatalogService {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	return application.NewCatalogService(repositories.NewCurrencies(db), repositories.NewItemDefinitions(db))
}

func TestCatalogService_CreateCurrency_GMSucceeds(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	c, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)
	assert.Equal(t, "gp", c.Code)
	assert.EqualValues(t, 100, c.Ratio)
}

func TestCatalogService_CreateCurrency_PlayerForbidden(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, playerRequester, "gp", "Gold", 100)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestCatalogService_CreateCurrency_RejectsBadRatio(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 0)
	require.ErrorIs(t, err, application.ErrInvalidCurrency)
}

func TestCatalogService_CreateCurrency_RejectsDuplicateCode(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)

	_, err = svc.CreateCurrency(ctx, gmRequester, "gp", "Gold Piece", 100)
	require.ErrorIs(t, err, application.ErrCurrencyExists)
}

func TestCatalogService_CreateItemDefinition_PlayerForbidden(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateItemDefinition(ctx, playerRequester, "Torch", "", "")
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestCatalogService_CreateItemDefinition_RejectsDuplicateName(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateItemDefinition(ctx, gmRequester, "Torch", "", "gear")
	require.NoError(t, err)

	_, err = svc.CreateItemDefinition(ctx, gmRequester, "Torch", "", "")
	require.ErrorIs(t, err, application.ErrItemDefinitionExists)
}

func TestCatalogService_List_AnyAuthenticatedUser(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)
	_, err = svc.CreateItemDefinition(ctx, gmRequester, "Torch", "", "")
	require.NoError(t, err)

	currencies, err := svc.ListCurrencies(ctx)
	require.NoError(t, err)
	assert.Len(t, currencies, 1)

	items, err := svc.ListItemDefinitions(ctx)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestCatalogService_LoadFile_SeedsAndUpserts(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	path := filepath.Join(t.TempDir(), "catalog.yaml")
	seed := `
currencies:
  - code: cp
    name: Copper
    ratio: 1
  - code: gp
    name: Gold
    ratio: 100
items:
  - name: Torch
    description: A wooden torch
    category: gear
  - name: Rope (50 ft)
`
	err := os.WriteFile(path, []byte(seed), 0o600)
	require.NoError(t, err)

	curCount, itemCount, err := svc.LoadFile(ctx, path)
	require.NoError(t, err)
	assert.Equal(t, 2, curCount)
	assert.Equal(t, 2, itemCount)

	// Re-seeding with a changed ratio upserts rather than duplicating.
	updated := "currencies:\n  - code: gp\n    name: Gold\n    ratio: 250\n"
	err = os.WriteFile(path, []byte(updated), 0o600)
	require.NoError(t, err)
	_, _, err = svc.LoadFile(ctx, path)
	require.NoError(t, err)

	currencies, err := svc.ListCurrencies(ctx)
	require.NoError(t, err)
	require.Len(t, currencies, 2)

	for _, c := range currencies {
		if c.Code == "gp" {
			assert.EqualValues(t, 250, c.Ratio, "gp ratio after reseed")
		}
	}
}

func TestCatalogService_Convert_AddsAndConvertsAcrossCurrencies(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	cp, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1)
	require.NoError(t, err)
	sp, err := svc.CreateCurrency(ctx, gmRequester, "sp", "Silver", 10)
	require.NoError(t, err)
	_, err = svc.CreateCurrency(ctx, gmRequester, "pp", "Platinum", 1000)
	require.NoError(t, err)

	result, err := svc.Convert(ctx, []application.CurrencyAmount{
		{Currency: "pp", Amount: 5},
		{Currency: cp.Code, Amount: 3},
	}, sp.ID)
	require.NoError(t, err)
	// 5 pp = 5000 cp, plus 3 cp = 5003 cp base value; sp ratio 10 -> 500 sp, remainder 3.
	assert.EqualValues(t, 500, result.Whole)
	assert.EqualValues(t, 3, result.Remainder)
	assert.EqualValues(t, 5003, result.BaseValue)
	assert.Equal(t, "sp", result.Currency.Code)
}

func TestCatalogService_Convert_UnknownCurrency(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1)
	require.NoError(t, err)

	_, err = svc.Convert(ctx, []application.CurrencyAmount{{Currency: "cp", Amount: 5}}, "gp")
	require.ErrorIs(t, err, application.ErrUnknownCurrency)
}

func TestCatalogService_Convert_RejectsNegativeAmount(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1)
	require.NoError(t, err)

	_, err = svc.Convert(ctx, []application.CurrencyAmount{{Currency: "cp", Amount: -1}}, "cp")
	require.ErrorIs(t, err, application.ErrInvalidAmount)
}

func TestCatalogService_Convert_RejectsEmptyAmounts(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.Convert(ctx, nil, "cp")
	require.ErrorIs(t, err, application.ErrNoAmounts)
}

func TestCatalogService_Simplify_GreedyBreakdown(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1)
	require.NoError(t, err)
	_, err = svc.CreateCurrency(ctx, gmRequester, "sp", "Silver", 10)
	require.NoError(t, err)
	_, err = svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)

	breakdown, err := svc.Simplify(ctx, []application.CurrencyAmount{{Currency: "cp", Amount: 1234}})
	require.NoError(t, err)
	require.Len(t, breakdown, 3)

	want := map[string]int64{"gp": 12, "sp": 3, "cp": 4}
	for _, entry := range breakdown {
		assert.Equal(t, want[entry.Currency.Code], entry.Amount, "Simplify %s", entry.Currency.Code)
	}
}

func TestCatalogService_Simplify_OmitsUnneededDenominations(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1)
	require.NoError(t, err)
	_, err = svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	require.NoError(t, err)

	breakdown, err := svc.Simplify(ctx, []application.CurrencyAmount{{Currency: "gp", Amount: 2}})
	require.NoError(t, err)
	require.Len(t, breakdown, 1)
	assert.Equal(t, "gp", breakdown[0].Currency.Code)
	assert.EqualValues(t, 2, breakdown[0].Amount)
}

func TestCatalogService_LoadFile_RejectsInvalidCurrency(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	path := filepath.Join(t.TempDir(), "catalog.yaml")
	err := os.WriteFile(path, []byte("currencies:\n  - code: gp\n    name: Gold\n    ratio: 0\n"), 0o600)
	require.NoError(t, err)

	_, _, err = svc.LoadFile(ctx, path)
	require.ErrorIs(t, err, application.ErrInvalidCurrency)
}

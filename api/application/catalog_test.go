package application_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

func newTestCatalogEnv(t *testing.T) *application.CatalogService {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	return application.NewCatalogService(repositories.NewCurrencies(db), repositories.NewItemDefinitions(db))
}

func TestCatalogService_CreateCurrency_GMSucceeds(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	c, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100)
	if err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}
	if c.Code != "gp" || c.Ratio != 100 {
		t.Fatalf("currency = %+v, want code gp ratio 100", c)
	}
}

func TestCatalogService_CreateCurrency_PlayerForbidden(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, playerRequester, "gp", "Gold", 100)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("CreateCurrency(player) = %v, want ErrForbidden", err)
	}
}

func TestCatalogService_CreateCurrency_RejectsBadRatio(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 0)
	if !errors.Is(err, application.ErrInvalidCurrency) {
		t.Fatalf("CreateCurrency(ratio 0) = %v, want ErrInvalidCurrency", err)
	}
}

func TestCatalogService_CreateCurrency_RejectsDuplicateCode(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	if _, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100); err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}

	_, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold Piece", 100)
	if !errors.Is(err, application.ErrCurrencyExists) {
		t.Fatalf("CreateCurrency(dup) = %v, want ErrCurrencyExists", err)
	}
}

func TestCatalogService_CreateItemDefinition_PlayerForbidden(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.CreateItemDefinition(ctx, playerRequester, "Torch", "", "")
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("CreateItemDefinition(player) = %v, want ErrForbidden", err)
	}
}

func TestCatalogService_CreateItemDefinition_RejectsDuplicateName(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	if _, err := svc.CreateItemDefinition(ctx, gmRequester, "Torch", "", "gear"); err != nil {
		t.Fatalf("CreateItemDefinition: %v", err)
	}

	_, err := svc.CreateItemDefinition(ctx, gmRequester, "Torch", "", "")
	if !errors.Is(err, application.ErrItemDefinitionExists) {
		t.Fatalf("CreateItemDefinition(dup) = %v, want ErrItemDefinitionExists", err)
	}
}

func TestCatalogService_List_AnyAuthenticatedUser(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	if _, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100); err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}
	if _, err := svc.CreateItemDefinition(ctx, gmRequester, "Torch", "", ""); err != nil {
		t.Fatalf("CreateItemDefinition: %v", err)
	}

	currencies, err := svc.ListCurrencies(ctx)
	if err != nil {
		t.Fatalf("ListCurrencies: %v", err)
	}
	if len(currencies) != 1 {
		t.Fatalf("ListCurrencies returned %d, want 1", len(currencies))
	}

	items, err := svc.ListItemDefinitions(ctx)
	if err != nil {
		t.Fatalf("ListItemDefinitions: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("ListItemDefinitions returned %d, want 1", len(items))
	}
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
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	curCount, itemCount, err := svc.LoadFile(ctx, path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if curCount != 2 || itemCount != 2 {
		t.Fatalf("LoadFile counts = (%d, %d), want (2, 2)", curCount, itemCount)
	}

	// Re-seeding with a changed ratio upserts rather than duplicating.
	updated := "currencies:\n  - code: gp\n    name: Gold\n    ratio: 250\n"
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, _, err := svc.LoadFile(ctx, path); err != nil {
		t.Fatalf("LoadFile (reseed): %v", err)
	}

	currencies, err := svc.ListCurrencies(ctx)
	if err != nil {
		t.Fatalf("ListCurrencies: %v", err)
	}
	if len(currencies) != 2 {
		t.Fatalf("ListCurrencies returned %d, want 2 (upsert, not duplicate)", len(currencies))
	}

	for _, c := range currencies {
		if c.Code == "gp" && c.Ratio != 250 {
			t.Fatalf("gp ratio = %d, want 250 after reseed", c.Ratio)
		}
	}
}

func TestCatalogService_Convert_AddsAndConvertsAcrossCurrencies(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	cp, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1)
	if err != nil {
		t.Fatalf("CreateCurrency(cp): %v", err)
	}
	sp, err := svc.CreateCurrency(ctx, gmRequester, "sp", "Silver", 10)
	if err != nil {
		t.Fatalf("CreateCurrency(sp): %v", err)
	}
	if _, err := svc.CreateCurrency(ctx, gmRequester, "pp", "Platinum", 1000); err != nil {
		t.Fatalf("CreateCurrency(pp): %v", err)
	}

	result, err := svc.Convert(ctx, []application.CurrencyAmount{
		{Currency: "pp", Amount: 5},
		{Currency: cp.Code, Amount: 3},
	}, sp.ID)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	// 5 pp = 5000 cp, plus 3 cp = 5003 cp base value; sp ratio 10 -> 500 sp, remainder 3.
	if result.Whole != 500 || result.Remainder != 3 || result.BaseValue != 5003 {
		t.Fatalf("Convert result = %+v, want whole 500 remainder 3 base 5003", result)
	}
	if result.Currency.Code != "sp" {
		t.Fatalf("Convert currency = %s, want sp", result.Currency.Code)
	}
}

func TestCatalogService_Convert_UnknownCurrency(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	if _, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1); err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}

	_, err := svc.Convert(ctx, []application.CurrencyAmount{{Currency: "cp", Amount: 5}}, "gp")
	if !errors.Is(err, application.ErrUnknownCurrency) {
		t.Fatalf("Convert(unknown target) = %v, want ErrUnknownCurrency", err)
	}
}

func TestCatalogService_Convert_RejectsNegativeAmount(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	if _, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1); err != nil {
		t.Fatalf("CreateCurrency: %v", err)
	}

	_, err := svc.Convert(ctx, []application.CurrencyAmount{{Currency: "cp", Amount: -1}}, "cp")
	if !errors.Is(err, application.ErrInvalidAmount) {
		t.Fatalf("Convert(negative) = %v, want ErrInvalidAmount", err)
	}
}

func TestCatalogService_Convert_RejectsEmptyAmounts(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	_, err := svc.Convert(ctx, nil, "cp")
	if !errors.Is(err, application.ErrNoAmounts) {
		t.Fatalf("Convert(empty) = %v, want ErrNoAmounts", err)
	}
}

func TestCatalogService_Simplify_GreedyBreakdown(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	if _, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1); err != nil {
		t.Fatalf("CreateCurrency(cp): %v", err)
	}
	if _, err := svc.CreateCurrency(ctx, gmRequester, "sp", "Silver", 10); err != nil {
		t.Fatalf("CreateCurrency(sp): %v", err)
	}
	if _, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100); err != nil {
		t.Fatalf("CreateCurrency(gp): %v", err)
	}

	breakdown, err := svc.Simplify(ctx, []application.CurrencyAmount{{Currency: "cp", Amount: 1234}})
	if err != nil {
		t.Fatalf("Simplify: %v", err)
	}
	if len(breakdown) != 3 {
		t.Fatalf("Simplify returned %d denominations, want 3: %+v", len(breakdown), breakdown)
	}

	want := map[string]int64{"gp": 12, "sp": 3, "cp": 4}
	for _, entry := range breakdown {
		if entry.Amount != want[entry.Currency.Code] {
			t.Fatalf("Simplify %s = %d, want %d", entry.Currency.Code, entry.Amount, want[entry.Currency.Code])
		}
	}
}

func TestCatalogService_Simplify_OmitsUnneededDenominations(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	if _, err := svc.CreateCurrency(ctx, gmRequester, "cp", "Copper", 1); err != nil {
		t.Fatalf("CreateCurrency(cp): %v", err)
	}
	if _, err := svc.CreateCurrency(ctx, gmRequester, "gp", "Gold", 100); err != nil {
		t.Fatalf("CreateCurrency(gp): %v", err)
	}

	breakdown, err := svc.Simplify(ctx, []application.CurrencyAmount{{Currency: "gp", Amount: 2}})
	if err != nil {
		t.Fatalf("Simplify: %v", err)
	}
	if len(breakdown) != 1 || breakdown[0].Currency.Code != "gp" || breakdown[0].Amount != 2 {
		t.Fatalf("Simplify = %+v, want just 2 gp", breakdown)
	}
}

func TestCatalogService_LoadFile_RejectsInvalidCurrency(t *testing.T) {
	svc := newTestCatalogEnv(t)
	ctx := t.Context()

	path := filepath.Join(t.TempDir(), "catalog.yaml")
	if err := os.WriteFile(path, []byte("currencies:\n  - code: gp\n    name: Gold\n    ratio: 0\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := svc.LoadFile(ctx, path)
	if !errors.Is(err, application.ErrInvalidCurrency) {
		t.Fatalf("LoadFile(bad ratio) = %v, want ErrInvalidCurrency", err)
	}
}

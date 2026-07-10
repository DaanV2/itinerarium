package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

func newTestLocationService(t *testing.T) *application.LocationService {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	if err != nil {
		t.Fatalf("persistence.New: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	return application.NewLocationService(repositories.NewLocations(db))
}

func TestLocationService_Create_GM(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	loc, err := svc.Create(ctx, gmRequester, "Waterdeep", "City of Splendors", "Material")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if loc.Name != "Waterdeep" || loc.Plane != "Material" {
		t.Fatalf("Create returned %+v", loc)
	}
}

func TestLocationService_Create_PlayerForbidden(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, playerRequester, "Waterdeep", "", "")
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Create(player) = %v, want ErrForbidden", err)
	}
}

func TestLocationService_Create_RejectsEmptyName(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, gmRequester, "", "", "")
	if !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("Create(empty name) = %v, want ErrInvalidName", err)
	}
}

func TestLocationService_List_AnyAuthenticated(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	if _, err := svc.Create(ctx, gmRequester, "Waterdeep", "", ""); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// A player is not a GM but may still list locations.
	locations, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(locations) != 1 {
		t.Fatalf("List returned %d locations, want 1", len(locations))
	}
}

func TestLocationService_Get_UnknownLocation(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	_, err := svc.Get(ctx, playerRequester, "does-not-exist")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get(unknown) = %v, want ErrNotFound", err)
	}
}

func TestLocationService_Update_GMCanEdit(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	loc, err := svc.Create(ctx, gmRequester, "Waterdeep", "old", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newDesc := "City of Splendors"

	updated, err := svc.Update(ctx, gmRequester, loc.ID, nil, &newDesc, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != newDesc {
		t.Fatalf("Description = %q, want %q", updated.Description, newDesc)
	}
}

func TestLocationService_Update_PlayerForbidden(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	loc, err := svc.Create(ctx, gmRequester, "Waterdeep", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Neverwinter"

	_, err = svc.Update(ctx, playerRequester, loc.ID, &newName, nil, nil)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Update(player) = %v, want ErrForbidden", err)
	}
}

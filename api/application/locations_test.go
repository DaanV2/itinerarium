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

func TestLocationService_Create_GMCreatesPlane(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	l, err := svc.Create(ctx, gmRequester, "The Material Plane", "The mortal world.", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if l.Name != "The Material Plane" || l.Description != "The mortal world." {
		t.Fatalf("location = %+v, want name/description set", l)
	}
	if l.ParentID != nil {
		t.Fatalf("ParentID = %v, want nil (a plane has no parent)", l.ParentID)
	}
}

func TestLocationService_Create_NestsUnderParent(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	plane, err := svc.Create(ctx, gmRequester, "The Material Plane", "", nil)
	if err != nil {
		t.Fatalf("Create plane: %v", err)
	}

	town, err := svc.Create(ctx, gmRequester, "Neverwinter", "", &plane.ID)
	if err != nil {
		t.Fatalf("Create town: %v", err)
	}
	if town.ParentID == nil || *town.ParentID != plane.ID {
		t.Fatalf("ParentID = %v, want %q", town.ParentID, plane.ID)
	}
}

func TestLocationService_Create_PlayerForbidden(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, playerRequester, "Neverwinter", "", nil)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Create(player) = %v, want ErrForbidden", err)
	}
}

func TestLocationService_Create_RejectsEmptyName(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, gmRequester, "", "", nil)
	if !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("Create(empty name) = %v, want ErrInvalidName", err)
	}
}

func TestLocationService_Create_RejectsUnknownParent(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	missing := "does-not-exist"

	_, err := svc.Create(ctx, gmRequester, "Neverwinter", "", &missing)
	if !errors.Is(err, application.ErrUnknownParent) {
		t.Fatalf("Create(unknown parent) = %v, want ErrUnknownParent", err)
	}
}

func TestLocationService_List_AnyAuthenticatedUserSeesAll(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	if _, err := svc.Create(ctx, gmRequester, "The Material Plane", "", nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, gmRequester, "The Feywild", "", nil); err != nil {
		t.Fatalf("Create: %v", err)
	}

	locations, err := svc.List(ctx, playerRequester)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(locations) != 2 {
		t.Fatalf("List returned %d locations, want 2", len(locations))
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

func TestLocationService_Update_GMEditsNameAndDescription(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	l, err := svc.Create(ctx, gmRequester, "Neverwinter", "A city.", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Neverwinter (ruined)"
	newDesc := "A ruined city."

	updated, err := svc.Update(ctx, gmRequester, l.ID, &newName, &newDesc)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != newName || updated.Description != newDesc {
		t.Fatalf("location = %+v, want updated name/description", updated)
	}
}

func TestLocationService_Update_PlayerForbidden(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	l, err := svc.Create(ctx, gmRequester, "Neverwinter", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "Hijacked"

	_, err = svc.Update(ctx, playerRequester, l.ID, &newName, nil)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Update(player) = %v, want ErrForbidden", err)
	}
}

func TestLocationService_Update_RejectsEmptyName(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	l, err := svc.Create(ctx, gmRequester, "Neverwinter", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	empty := ""

	_, err = svc.Update(ctx, gmRequester, l.ID, &empty, nil)
	if !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("Update(empty name) = %v, want ErrInvalidName", err)
	}
}

func TestLocationService_Delete_GMRemovesLeaf(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	l, err := svc.Create(ctx, gmRequester, "Neverwinter", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(ctx, gmRequester, l.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.Get(ctx, gmRequester, l.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get(deleted) = %v, want ErrNotFound", err)
	}
}

func TestLocationService_Delete_PlayerForbidden(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	l, err := svc.Create(ctx, gmRequester, "Neverwinter", "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = svc.Delete(ctx, playerRequester, l.ID)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Delete(player) = %v, want ErrForbidden", err)
	}
}

func TestLocationService_Delete_RejectsLocationWithChildren(t *testing.T) {
	svc := newTestLocationService(t)
	ctx := t.Context()

	plane, err := svc.Create(ctx, gmRequester, "The Material Plane", "", nil)
	if err != nil {
		t.Fatalf("Create plane: %v", err)
	}
	if _, err := svc.Create(ctx, gmRequester, "Neverwinter", "", &plane.ID); err != nil {
		t.Fatalf("Create town: %v", err)
	}

	err = svc.Delete(ctx, gmRequester, plane.ID)
	if !errors.Is(err, application.ErrLocationHasChildren) {
		t.Fatalf("Delete(location with children) = %v, want ErrLocationHasChildren", err)
	}
}

package application_test

import (
	"errors"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/require"
)

type locationTestEnv struct {
	locations  *application.LocationService
	groups     *application.GroupService
	characters *application.CharacterService
}

func newTestLocationEnv(t *testing.T) locationTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	characterRepo := repositories.NewCharacters(db)
	groupRepo := repositories.NewGroups(db)
	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	charSvc := application.NewCharacterService(characterRepo, repositories.NewUsers(db), knowledgeRepo)

	return locationTestEnv{
		locations: application.NewLocationService(
			repositories.NewLocations(db),
			repositories.NewLocationAccesses(db),
			groupRepo,
			characterRepo,
			charSvc,
		),
		groups:     application.NewGroupService(groupRepo, charSvc, knowledgeRepo),
		characters: charSvc,
	}
}

func (e locationTestEnv) createLocation(t *testing.T, name string) *models.Location {
	t.Helper()

	location, err := e.locations.Create(t.Context(), gmRequester, name, "", "Material")
	require.NoError(t, err)

	return location
}

func TestLocationService_Create_PlayerForbidden(t *testing.T) {
	env := newTestLocationEnv(t)

	_, err := env.locations.Create(t.Context(), playerRequester, "The Feywild", "", "Feywild")
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Create as player = %v, want ErrForbidden", err)
	}
}

func TestLocationService_Create_RejectsEmptyName(t *testing.T) {
	env := newTestLocationEnv(t)

	_, err := env.locations.Create(t.Context(), gmRequester, "", "", "")
	if !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("Create(empty name) = %v, want ErrInvalidName", err)
	}
}

func TestLocationService_Get_UnknownLocation(t *testing.T) {
	env := newTestLocationEnv(t)

	_, err := env.locations.Get(t.Context(), gmRequester, "does-not-exist")
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get(unknown) = %v, want ErrNotFound", err)
	}
}

func TestLocationService_HiddenWithoutGrant(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "Hidden Vault")
	ownedCharacter(t, env.characters, "Aria")

	// Not in the list…
	locations, err := env.locations.List(ctx, playerRequester)
	require.NoError(t, err)
	if len(locations) != 0 {
		t.Fatalf("List leaked %d locations to an unauthorised player", len(locations))
	}

	// …and a direct read is 404, never 403 (existence must not leak).
	if _, err := env.locations.Get(ctx, playerRequester, location.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get without grant = %v, want ErrNotFound", err)
	}
}

func TestLocationService_DirectGrantRevealsLocation(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	got, err := env.locations.Get(ctx, playerRequester, location.ID)
	require.NoError(t, err)
	if got.ID != location.ID {
		t.Fatalf("Get returned %s, want %s", got.ID, location.ID)
	}

	locations, err := env.locations.List(ctx, playerRequester)
	require.NoError(t, err)
	if len(locations) != 1 {
		t.Fatalf("List returned %d locations, want 1", len(locations))
	}
}

func TestLocationService_GroupGrantRevealsLocation(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "Guild Hall")
	character := ownedCharacter(t, env.characters, "Aria")

	group, err := env.groups.Create(ctx, gmRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.NoError(t, err)
	err = env.groups.Join(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, nil, &group.ID); err != nil {
		t.Fatalf("GrantAccess(group): %v", err)
	}

	if _, err := env.locations.Get(ctx, playerRequester, location.ID); err != nil {
		t.Fatalf("Get via group grant: %v", err)
	}

	// Leaving the group takes the access away again.
	err = env.groups.Leave(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)
	if _, err := env.locations.Get(ctx, playerRequester, location.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get after leaving group = %v, want ErrNotFound", err)
	}
}

func TestLocationService_AnyoneWithAccessCanEdit(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	newDescription := "Smells of stale ale."

	// Without access the edit reads as not-found…
	_, err := env.locations.Update(ctx, playerRequester, location.ID, nil, &newDescription, nil)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Update without grant = %v, want ErrNotFound", err)
	}

	// …with access it succeeds (rule 7: seeing a location means editing it).
	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	updated, err := env.locations.Update(ctx, playerRequester, location.ID, nil, &newDescription, nil)
	require.NoError(t, err)
	if updated.Description != newDescription {
		t.Fatalf("Description = %q, want %q", updated.Description, newDescription)
	}
}

func TestLocationService_GrantAccess_Validation(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	if _, err := env.locations.GrantAccess(ctx, playerRequester, location.ID, &character.ID, nil); !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("GrantAccess as player = %v, want ErrForbidden", err)
	}

	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, nil, nil); !errors.Is(err, application.ErrInvalidGrant) {
		t.Fatalf("GrantAccess with no target = %v, want ErrInvalidGrant", err)
	}

	groupID := "some-group"
	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, &groupID); !errors.Is(err, application.ErrInvalidGrant) {
		t.Fatalf("GrantAccess with two targets = %v, want ErrInvalidGrant", err)
	}

	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}
	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil); !errors.Is(err, application.ErrAlreadyGranted) {
		t.Fatalf("duplicate GrantAccess = %v, want ErrAlreadyGranted", err)
	}
}

func TestLocationService_RevokeAccessHidesAgain(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	grant, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	err = env.locations.RevokeAccess(ctx, gmRequester, location.ID, grant.ID)
	require.NoError(t, err)

	if _, err := env.locations.Get(ctx, playerRequester, location.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Get after revoke = %v, want ErrNotFound", err)
	}
}

func TestLocationService_AssignCharacter(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	// A player cannot place a character at a location it cannot see — and the
	// error must read as not-found.
	_, err := env.locations.AssignCharacter(ctx, playerRequester, character.ID, &location.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("AssignCharacter without access = %v, want ErrNotFound", err)
	}

	if _, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	updated, err := env.locations.AssignCharacter(ctx, playerRequester, character.ID, &location.ID)
	require.NoError(t, err)
	if updated.LocationID == nil || *updated.LocationID != location.ID {
		t.Fatalf("LocationID = %v, want %s", updated.LocationID, location.ID)
	}

	cleared, err := env.locations.AssignCharacter(ctx, playerRequester, character.ID, nil)
	require.NoError(t, err)
	if cleared.LocationID != nil {
		t.Fatalf("LocationID = %v, want nil after clearing", cleared.LocationID)
	}
}

func TestLocationService_AssignCharacter_GMAnywhere_ForeignHidden(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	// GMs place any character anywhere, no grant needed.
	if _, err := env.locations.AssignCharacter(ctx, gmRequester, character.ID, &location.ID); err != nil {
		t.Fatalf("AssignCharacter as GM: %v", err)
	}

	// A different player cannot even confirm the character exists.
	_, err := env.locations.AssignCharacter(ctx, otherPlayer, character.ID, &location.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("AssignCharacter foreign character = %v, want ErrNotFound", err)
	}
}

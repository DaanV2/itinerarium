package application_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/stretchr/testify/assert"
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

	location, err := e.locations.Create(t.Context(), gmRequester, name, "Material")
	require.NoError(t, err)

	return location.Location
}

func TestLocationService_Create_PlayerForbidden(t *testing.T) {
	env := newTestLocationEnv(t)

	_, err := env.locations.Create(t.Context(), playerRequester, "The Feywild", "Feywild")
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestLocationService_Create_RejectsEmptyName(t *testing.T) {
	env := newTestLocationEnv(t)

	_, err := env.locations.Create(t.Context(), gmRequester, "", "")
	require.ErrorIs(t, err, application.ErrInvalidName)
}

func TestLocationService_Get_UnknownLocation(t *testing.T) {
	env := newTestLocationEnv(t)

	_, err := env.locations.Get(t.Context(), gmRequester, "does-not-exist")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestLocationService_HiddenWithoutGrant(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "Hidden Vault")
	ownedCharacter(t, env.characters, "Aria")

	// Not in the list…
	locations, err := env.locations.List(ctx, playerRequester)
	require.NoError(t, err)
	assert.Empty(t, locations, "List leaked locations to an unauthorised player")

	// …and a direct read is 404, never 403 (existence must not leak).
	_, err = env.locations.Get(ctx, playerRequester, location.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestLocationService_DirectGrantRevealsLocation(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	_, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	got, err := env.locations.Get(ctx, playerRequester, location.ID)
	require.NoError(t, err)
	assert.Equal(t, location.ID, got.Location.ID)

	locations, err := env.locations.List(ctx, playerRequester)
	require.NoError(t, err)
	assert.Len(t, locations, 1)
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

	_, err = env.locations.GrantAccess(ctx, gmRequester, location.ID, nil, &group.ID)
	require.NoError(t, err)

	_, err = env.locations.Get(ctx, playerRequester, location.ID)
	require.NoError(t, err)

	// Leaving the group takes the access away again.
	err = env.groups.Leave(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)
	_, err = env.locations.Get(ctx, playerRequester, location.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestLocationService_AnyoneWithAccessCanEdit(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	newName := "The Rusty Tavern"
	sections := []application.LocationSectionInput{{Content: "Smells of stale ale."}}

	// Without access the edit reads as not-found…
	_, err := env.locations.Update(ctx, playerRequester, location.ID, &newName, nil, nil, sections)
	require.ErrorIs(t, err, application.ErrNotFound)

	// …with access it succeeds (rule 7: seeing a location means editing it).
	_, err = env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	updated, err := env.locations.Update(ctx, playerRequester, location.ID, &newName, nil, nil, sections)
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Location.Name)
	if assert.Len(t, updated.Location.Sections, 1) {
		assert.Equal(t, "Smells of stale ale.", updated.Location.Sections[0].Content)
	}
}

func TestLocationService_Description_GMOnlySectionStrippedForPlayers(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")
	_, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	sections := []application.LocationSectionInput{
		{Content: "A cosy tavern by the docks."},
		{Content: "The barkeep is a Guild informant.", GMOnly: true},
	}
	_, err = env.locations.Update(ctx, gmRequester, location.ID, nil, nil, nil, sections)
	require.NoError(t, err)

	got, err := env.locations.Get(ctx, playerRequester, location.ID)
	require.NoError(t, err)
	if assert.Len(t, got.Location.Sections, 1, "GM-only section leaked to a player") {
		assert.Equal(t, "A cosy tavern by the docks.", got.Location.Sections[0].Content)
	}

	gmView, err := env.locations.Get(ctx, gmRequester, location.ID)
	require.NoError(t, err)
	assert.Len(t, gmView.Location.Sections, 2, "GM should see every section")
}

func TestLocationService_Description_GameDayGating(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")
	_, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	sharedOn := 5
	sections := []application.LocationSectionInput{{Content: "Rebuilt after the fire."}}
	_, err = env.locations.Update(ctx, gmRequester, location.ID, nil, nil, &sharedOn, sections)
	require.NoError(t, err)

	// The character hasn't reached game day 5 yet — description stays hidden…
	got, err := env.locations.Get(ctx, playerRequester, location.ID)
	require.NoError(t, err)
	assert.Empty(t, got.Location.Sections, "description revealed before its game day")
	assert.False(t, got.Revealed)

	// …but the location itself is still visible and editable (existence is
	// gated by LocationAccess, not game day).
	newName := "The Tavern (rebuilt)"
	_, err = env.locations.Update(ctx, playerRequester, location.ID, &newName, nil, nil, nil)
	require.NoError(t, err)

	_, err = env.characters.Update(ctx, gmRequester, character.ID, nil, &sharedOn)
	require.NoError(t, err)

	got, err = env.locations.Get(ctx, playerRequester, location.ID)
	require.NoError(t, err)
	if assert.Len(t, got.Location.Sections, 1) {
		assert.Equal(t, "Rebuilt after the fire.", got.Location.Sections[0].Content)
	}
	assert.True(t, got.Revealed)
}

func TestLocationService_Description_OnlyGMCanChangeRevealDay(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")
	_, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	sharedOn := 3
	_, err = env.locations.Update(ctx, playerRequester, location.ID, nil, nil, &sharedOn, nil)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestLocationService_Description_PlayerCannotMarkGMOnly(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")
	_, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	sections := []application.LocationSectionInput{{Content: "Secretly a smugglers' den.", GMOnly: true}}
	_, err = env.locations.Update(ctx, playerRequester, location.ID, nil, nil, nil, sections)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestLocationService_Description_PlayerEditOnAllGMOnlyCreatesNewSection(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")
	_, err := env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	gmSections := []application.LocationSectionInput{{Content: "GM secret notes.", GMOnly: true}}
	_, err = env.locations.Update(ctx, gmRequester, location.ID, nil, nil, nil, gmSections)
	require.NoError(t, err)

	// A player editing a location whose only visible content is empty (all
	// sections GM-only) lands as a new player-visible section (rule 7).
	playerSections := []application.LocationSectionInput{{Content: "Looks like an ordinary inn."}}
	updated, err := env.locations.Update(ctx, playerRequester, location.ID, nil, nil, nil, playerSections)
	require.NoError(t, err)
	if assert.Len(t, updated.Location.Sections, 1, "player-visible section should have been appended") {
		assert.Equal(t, "Looks like an ordinary inn.", updated.Location.Sections[0].Content)
	}

	gmView, err := env.locations.Get(ctx, gmRequester, location.ID)
	require.NoError(t, err)
	assert.Len(t, gmView.Location.Sections, 2, "GM-only section should remain alongside the new one")
}

func TestLocationService_GrantAccess_Validation(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	_, err := env.locations.GrantAccess(ctx, playerRequester, location.ID, &character.ID, nil)
	require.ErrorIs(t, err, application.ErrForbidden)

	_, err = env.locations.GrantAccess(ctx, gmRequester, location.ID, nil, nil)
	require.ErrorIs(t, err, application.ErrInvalidGrant)

	groupID := "some-group"
	_, err = env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, &groupID)
	require.ErrorIs(t, err, application.ErrInvalidGrant)

	_, err = env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)
	_, err = env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.ErrorIs(t, err, application.ErrAlreadyGranted)
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

	_, err = env.locations.Get(ctx, playerRequester, location.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestLocationService_AssignCharacter(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	// A player cannot place a character at a location it cannot see — and the
	// error must read as not-found.
	_, err := env.locations.AssignCharacter(ctx, playerRequester, character.ID, &location.ID)
	require.ErrorIs(t, err, application.ErrNotFound)

	_, err = env.locations.GrantAccess(ctx, gmRequester, location.ID, &character.ID, nil)
	require.NoError(t, err)

	updated, err := env.locations.AssignCharacter(ctx, playerRequester, character.ID, &location.ID)
	require.NoError(t, err)
	if assert.NotNil(t, updated.LocationID) {
		assert.Equal(t, location.ID, *updated.LocationID)
	}

	cleared, err := env.locations.AssignCharacter(ctx, playerRequester, character.ID, nil)
	require.NoError(t, err)
	assert.Nil(t, cleared.LocationID, "LocationID should be nil after clearing")
}

func TestLocationService_AssignCharacter_GMAnywhere_ForeignHidden(t *testing.T) {
	env := newTestLocationEnv(t)
	ctx := t.Context()
	location := env.createLocation(t, "The Tavern")
	character := ownedCharacter(t, env.characters, "Aria")

	// GMs place any character anywhere, no grant needed.
	_, err := env.locations.AssignCharacter(ctx, gmRequester, character.ID, &location.ID)
	require.NoError(t, err)

	// A different player cannot even confirm the character exists.
	_, err = env.locations.AssignCharacter(ctx, otherPlayer, character.ID, &location.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

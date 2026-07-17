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

func newTestCharactersEnv(t *testing.T) (*application.CharacterService, *repositories.Users) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	users := repositories.NewUsers(db)
	characters := repositories.NewCharacters(db)

	return application.NewCharacterService(characters, users, repositories.NewKnowledgeRepositories(db)), users
}

func TestCharacterService_Create_ForSelf(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	assert.Equal(t, playerRequester.UserID(), c.UserID)
	assert.Equal(t, 0, c.CurrentGameDay)
}

func TestCharacterService_Create_MultiplePerUser(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	_, err = svc.Create(ctx, playerRequester, "", "Beren")
	require.NoError(t, err)

	characters, err := svc.List(ctx, playerRequester)
	require.NoError(t, err)
	assert.Len(t, characters, 2)
}

func TestCharacterService_Create_RejectsEmptyName(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, playerRequester, "", "")
	require.ErrorIs(t, err, application.ErrInvalidName)
}

func TestCharacterService_Create_PlayerCannotCreateForAnotherUser(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, playerRequester, "someone-else", "Aria")
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestCharacterService_Create_GMForUnknownUser(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, gmRequester, "does-not-exist", "Aria")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestCharacterService_Create_GMForExistingUser(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	owner := &models.User{Email: "owner@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err := users.Create(ctx, owner)
	require.NoError(t, err)

	c, err := svc.Create(ctx, gmRequester, owner.ID, "Aria")
	require.NoError(t, err)
	assert.Equal(t, owner.ID, c.UserID)
}

func TestCharacterService_List_PlayerSeesOnlyOwn(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err := users.Create(ctx, other)
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	_, err = svc.Create(ctx, gmRequester, other.ID, "Beren")
	require.NoError(t, err)

	characters, err := svc.List(ctx, playerRequester)
	require.NoError(t, err)
	assert.Len(t, characters, 1)
}

func TestCharacterService_List_GMSeesAll(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err := users.Create(ctx, other)
	require.NoError(t, err)

	_, err = svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)
	_, err = svc.Create(ctx, gmRequester, other.ID, "Beren")
	require.NoError(t, err)

	characters, err := svc.List(ctx, gmRequester)
	require.NoError(t, err)
	assert.Len(t, characters, 2)
}

func TestCharacterService_Get_HidesOtherOwnersCharacter(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err := users.Create(ctx, other)
	require.NoError(t, err)

	c, err := svc.Create(ctx, gmRequester, other.ID, "Beren")
	require.NoError(t, err)

	_, err = svc.Get(ctx, playerRequester, c.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestCharacterService_Get_OwnerCanSeeOwnCharacter(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	got, err := svc.Get(ctx, playerRequester, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)
}

func TestCharacterService_Get_UnknownCharacter(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	_, err := svc.Get(ctx, gmRequester, "does-not-exist")
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestCharacterService_Update_OwnerCanRename(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	newName := "Aria the Bold"

	updated, err := svc.Update(ctx, playerRequester, c.ID, &newName, nil)
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
}

func TestCharacterService_Update_PlayerCannotSetGameDay(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	day := 5

	_, err = svc.Update(ctx, playerRequester, c.ID, nil, &day)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestCharacterService_Update_GMCanSetGameDay(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	day := 5

	updated, err := svc.Update(ctx, gmRequester, c.ID, nil, &day)
	require.NoError(t, err)
	assert.Equal(t, day, updated.CurrentGameDay)
}

func TestCharacterService_Update_RejectsNegativeGameDay(t *testing.T) {
	svc, _ := newTestCharactersEnv(t)
	ctx := t.Context()

	c, err := svc.Create(ctx, playerRequester, "", "Aria")
	require.NoError(t, err)

	day := -1

	_, err = svc.Update(ctx, gmRequester, c.ID, nil, &day)
	require.ErrorIs(t, err, application.ErrInvalidGameDay)
}

func TestCharacterService_Update_OtherOwnersCharacterIsHidden(t *testing.T) {
	svc, users := newTestCharactersEnv(t)
	ctx := t.Context()

	other := &models.User{Email: "other@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	err := users.Create(ctx, other)
	require.NoError(t, err)

	c, err := svc.Create(ctx, gmRequester, other.ID, "Beren")
	require.NoError(t, err)

	newName := "Hijacked"

	_, err = svc.Update(ctx, playerRequester, c.ID, &newName, nil)
	require.ErrorIs(t, err, application.ErrNotFound)
}

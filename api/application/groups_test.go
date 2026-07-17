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

func newTestGroupEnv(t *testing.T) (
	*application.GroupService, *application.CharacterService, *repositories.ActivityEntries,
) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	knowledgeRepo := repositories.NewKnowledgeRepositories(db)
	charSvc := application.NewCharacterService(repositories.NewCharacters(db), repositories.NewUsers(db), knowledgeRepo)
	groupSvc := application.NewGroupService(repositories.NewGroups(db), charSvc, knowledgeRepo)

	return groupSvc, charSvc, repositories.NewActivityEntries(db)
}

func createGroup(t *testing.T, svc *application.GroupService, name string) *models.Group {
	t.Helper()

	group, err := svc.Create(t.Context(), gmRequester, name, models.GroupTypeOrganization, "")
	require.NoError(t, err)

	return group
}

func TestGroupService_Create_PlayerForbidden(t *testing.T) {
	svc, _, _ := newTestGroupEnv(t)

	_, err := svc.Create(t.Context(), playerRequester, "Thieves Guild", models.GroupTypeOrganization, "")
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestGroupService_Create_RejectsInvalidInput(t *testing.T) {
	svc, _, _ := newTestGroupEnv(t)
	ctx := t.Context()

	_, err := svc.Create(ctx, gmRequester, "", models.GroupTypeFamily, "")
	require.ErrorIs(t, err, application.ErrInvalidName)

	_, err = svc.Create(ctx, gmRequester, "The Council", models.GroupType("guild"), "")
	require.ErrorIs(t, err, application.ErrInvalidGroupType)
}

func TestGroupService_TypesShareMechanics(t *testing.T) {
	svc, _, _ := newTestGroupEnv(t)
	ctx := t.Context()

	// All three types create identically — the type is cosmetic (rule 6).
	for _, groupType := range []models.GroupType{
		models.GroupTypeOrganization, models.GroupTypeFamily, models.GroupTypeOther,
	} {
		_, err := svc.Create(ctx, gmRequester, "Group "+string(groupType), groupType, "")
		require.NoError(t, err)
	}

	groups, err := svc.List(ctx, playerRequester)
	require.NoError(t, err)
	assert.Len(t, groups, 3)
}

func TestGroupService_Update_PlayerForbidden(t *testing.T) {
	svc, _, _ := newTestGroupEnv(t)
	group := createGroup(t, svc, "Thieves Guild")

	newName := "Assassins Guild"

	_, err := svc.Update(t.Context(), playerRequester, group.ID, &newName, nil, nil)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestGroupService_JoinAndLeave_RecordsGameDayStampedEvents(t *testing.T) {
	svc, charSvc, activity := newTestGroupEnv(t)
	ctx := t.Context()
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	day := 7
	_, err := charSvc.Update(ctx, gmRequester, character.ID, nil, &day)
	require.NoError(t, err)

	err = svc.Join(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	loaded, err := svc.Get(ctx, playerRequester, group.ID)
	require.NoError(t, err)
	require.Len(t, loaded.Members, 1)
	assert.Equal(t, character.ID, loaded.Members[0].ID)

	err = svc.Leave(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	entries, err := activity.ListByEntity(ctx, "group", group.ID)
	require.NoError(t, err)
	require.Len(t, entries, 2, "recorded entries, want 2 (join + leave)")
	for i, want := range []models.ActivityAction{models.ActivityActionJoined, models.ActivityActionLeft} {
		entry := entries[i]
		assert.Equal(t, want, entry.Action, "entry[%d].Action", i)
		assert.Equal(t, day, entry.GameDay, "entry[%d].GameDay", i)
		assert.Equal(t, character.ID, entry.CharacterID, "entry[%d].CharacterID", i)
		assert.Equal(t, character.Name, entry.Actor, "entry[%d].Actor", i)
	}
}

func TestGroupService_Join_ForeignCharacterHidden(t *testing.T) {
	svc, charSvc, _ := newTestGroupEnv(t)
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	// Another player must not be able to move Aria — and must not learn she
	// exists: ErrNotFound, never ErrForbidden.
	err := svc.Join(t.Context(), otherPlayer, group.ID, character.ID)
	require.ErrorIs(t, err, application.ErrNotFound)
}

func TestGroupService_Join_DuplicateRejected(t *testing.T) {
	svc, charSvc, _ := newTestGroupEnv(t)
	ctx := t.Context()
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.Join(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	err = svc.Join(ctx, playerRequester, group.ID, character.ID)
	require.ErrorIs(t, err, application.ErrAlreadyMember)
}

func TestGroupService_Leave_NonMemberRejected(t *testing.T) {
	svc, charSvc, _ := newTestGroupEnv(t)
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.Leave(t.Context(), playerRequester, group.ID, character.ID)
	require.ErrorIs(t, err, application.ErrNotMember)
}

func TestGroupService_GMManagesAnyCharacter(t *testing.T) {
	svc, charSvc, _ := newTestGroupEnv(t)
	ctx := t.Context()
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.Join(ctx, gmRequester, group.ID, character.ID)
	require.NoError(t, err)
	err = svc.Leave(ctx, gmRequester, group.ID, character.ID)
	require.NoError(t, err)
}

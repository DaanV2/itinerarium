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
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Create as player = %v, want ErrForbidden", err)
	}
}

func TestGroupService_Create_RejectsInvalidInput(t *testing.T) {
	svc, _, _ := newTestGroupEnv(t)
	ctx := t.Context()

	if _, err := svc.Create(ctx, gmRequester, "", models.GroupTypeFamily, ""); !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("Create(empty name) = %v, want ErrInvalidName", err)
	}

	_, err := svc.Create(ctx, gmRequester, "The Council", models.GroupType("guild"), "")
	if !errors.Is(err, application.ErrInvalidGroupType) {
		t.Fatalf("Create(bad type) = %v, want ErrInvalidGroupType", err)
	}
}

func TestGroupService_TypesShareMechanics(t *testing.T) {
	svc, _, _ := newTestGroupEnv(t)
	ctx := t.Context()

	// All three types create identically — the type is cosmetic (rule 6).
	for _, groupType := range []models.GroupType{
		models.GroupTypeOrganization, models.GroupTypeFamily, models.GroupTypeOther,
	} {
		if _, err := svc.Create(ctx, gmRequester, "Group "+string(groupType), groupType, ""); err != nil {
			t.Fatalf("Create(%s): %v", groupType, err)
		}
	}

	groups, err := svc.List(ctx, playerRequester)
	require.NoError(t, err)
	if len(groups) != 3 {
		t.Fatalf("List returned %d groups, want 3", len(groups))
	}
}

func TestGroupService_Update_PlayerForbidden(t *testing.T) {
	svc, _, _ := newTestGroupEnv(t)
	group := createGroup(t, svc, "Thieves Guild")

	newName := "Assassins Guild"

	_, err := svc.Update(t.Context(), playerRequester, group.ID, &newName, nil, nil)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Update as player = %v, want ErrForbidden", err)
	}
}

func TestGroupService_JoinAndLeave_RecordsGameDayStampedEvents(t *testing.T) {
	svc, charSvc, activity := newTestGroupEnv(t)
	ctx := t.Context()
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	day := 7
	if _, err := charSvc.Update(ctx, gmRequester, character.ID, nil, &day); err != nil {
		t.Fatalf("set game day: %v", err)
	}

	err := svc.Join(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	loaded, err := svc.Get(ctx, playerRequester, group.ID)
	require.NoError(t, err)
	if len(loaded.Members) != 1 || loaded.Members[0].ID != character.ID {
		t.Fatalf("Members = %v, want [%s]", loaded.Members, character.ID)
	}

	err = svc.Leave(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	entries, err := activity.ListByEntity(ctx, "group", group.ID)
	require.NoError(t, err)
	if len(entries) != 2 {
		t.Fatalf("recorded %d entries, want 2 (join + leave)", len(entries))
	}
	for i, want := range []models.ActivityAction{models.ActivityActionJoined, models.ActivityActionLeft} {
		entry := entries[i]
		if entry.Action != want {
			t.Errorf("entry[%d].Action = %s, want %s", i, entry.Action, want)
		}
		if entry.GameDay != day {
			t.Errorf("entry[%d].GameDay = %d, want %d", i, entry.GameDay, day)
		}
		if entry.CharacterID != character.ID || entry.Actor != character.Name {
			t.Errorf("entry[%d] actor = %s/%s, want %s/%s",
				i, entry.CharacterID, entry.Actor, character.ID, character.Name)
		}
	}
}

func TestGroupService_Join_ForeignCharacterHidden(t *testing.T) {
	svc, charSvc, _ := newTestGroupEnv(t)
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	// Another player must not be able to move Aria — and must not learn she
	// exists: ErrNotFound, never ErrForbidden.
	err := svc.Join(t.Context(), otherPlayer, group.ID, character.ID)
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Join with foreign character = %v, want ErrNotFound", err)
	}
}

func TestGroupService_Join_DuplicateRejected(t *testing.T) {
	svc, charSvc, _ := newTestGroupEnv(t)
	ctx := t.Context()
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.Join(ctx, playerRequester, group.ID, character.ID)
	require.NoError(t, err)

	if err := svc.Join(ctx, playerRequester, group.ID, character.ID); !errors.Is(err, application.ErrAlreadyMember) {
		t.Fatalf("second Join = %v, want ErrAlreadyMember", err)
	}
}

func TestGroupService_Leave_NonMemberRejected(t *testing.T) {
	svc, charSvc, _ := newTestGroupEnv(t)
	group := createGroup(t, svc, "Thieves Guild")
	character := ownedCharacter(t, charSvc, "Aria")

	if err := svc.Leave(t.Context(), playerRequester, group.ID, character.ID); !errors.Is(err, application.ErrNotMember) {
		t.Fatalf("Leave without membership = %v, want ErrNotMember", err)
	}
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

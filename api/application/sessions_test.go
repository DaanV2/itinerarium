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

func newTestSessionEnv(t *testing.T) (*application.SessionService, *application.CharacterService) {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err)
	err = db.Migrate()
	require.NoError(t, err)

	charSvc := application.NewCharacterService(
		repositories.NewCharacters(db), repositories.NewUsers(db), repositories.NewKnowledgeRepositories(db),
	)
	sessionSvc := application.NewSessionService(repositories.NewSessions(db), charSvc)

	return sessionSvc, charSvc
}

func createSession(t *testing.T, svc *application.SessionService, name string) *models.Session {
	t.Helper()

	session, err := svc.Create(t.Context(), gmRequester, name, "")
	require.NoError(t, err)

	return session
}

func TestSessionService_Create_PlayerForbidden(t *testing.T) {
	svc, _ := newTestSessionEnv(t)

	_, err := svc.Create(t.Context(), playerRequester, "Session One", "")
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Create as player = %v, want ErrForbidden", err)
	}
}

func TestSessionService_Create_RejectsEmptyName(t *testing.T) {
	svc, _ := newTestSessionEnv(t)

	_, err := svc.Create(t.Context(), gmRequester, "", "")
	if !errors.Is(err, application.ErrInvalidName) {
		t.Fatalf("Create(empty name) = %v, want ErrInvalidName", err)
	}
}

func TestSessionService_List_PlayerForbidden(t *testing.T) {
	svc, _ := newTestSessionEnv(t)
	createSession(t, svc, "Session One")

	_, err := svc.List(t.Context(), playerRequester)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("List as player = %v, want ErrForbidden", err)
	}
}

func TestSessionService_Update_PlayerForbidden(t *testing.T) {
	svc, _ := newTestSessionEnv(t)
	session := createSession(t, svc, "Session One")

	newName := "Session Renamed"

	_, err := svc.Update(t.Context(), playerRequester, session.ID, &newName, nil)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("Update as player = %v, want ErrForbidden", err)
	}
}

func TestSessionService_AddAndRemoveParticipant(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.AddParticipant(ctx, gmRequester, session.ID, character.ID)
	require.NoError(t, err)

	loaded, err := svc.Get(ctx, gmRequester, session.ID)
	require.NoError(t, err)
	if len(loaded.Participants) != 1 || loaded.Participants[0].ID != character.ID {
		t.Fatalf("Participants = %v, want [%s]", loaded.Participants, character.ID)
	}

	err = svc.RemoveParticipant(ctx, gmRequester, session.ID, character.ID)
	require.NoError(t, err)

	loaded, err = svc.Get(ctx, gmRequester, session.ID)
	require.NoError(t, err)
	if len(loaded.Participants) != 0 {
		t.Fatalf("Participants = %v, want none", loaded.Participants)
	}
}

func TestSessionService_AddParticipant_PlayerForbidden(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.AddParticipant(t.Context(), playerRequester, session.ID, character.ID)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("AddParticipant as player = %v, want ErrForbidden", err)
	}
}

func TestSessionService_AddParticipant_DuplicateRejected(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.AddParticipant(ctx, gmRequester, session.ID, character.ID)
	require.NoError(t, err)

	err = svc.AddParticipant(ctx, gmRequester, session.ID, character.ID)
	require.ErrorIs(t, err, application.ErrAlreadyParticipant)
}

func TestSessionService_RemoveParticipant_NonParticipantRejected(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.RemoveParticipant(t.Context(), gmRequester, session.ID, character.ID)
	if !errors.Is(err, application.ErrNotParticipant) {
		t.Fatalf("RemoveParticipant without participation = %v, want ErrNotParticipant", err)
	}
}

func TestSessionService_AdvanceGameDay_BulkAppliesToAllParticipants(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	aria := ownedCharacter(t, charSvc, "Aria")
	bram := ownedCharacter(t, charSvc, "Bram")

	err := svc.AddParticipant(ctx, gmRequester, session.ID, aria.ID)
	require.NoError(t, err)
	err = svc.AddParticipant(ctx, gmRequester, session.ID, bram.ID)
	require.NoError(t, err)

	if _, err := svc.AdvanceGameDay(ctx, gmRequester, session.ID, 3, nil); err != nil {
		t.Fatalf("AdvanceGameDay: %v", err)
	}

	for _, c := range []*models.Character{aria, bram} {
		loaded, err := charSvc.Get(ctx, gmRequester, c.ID)
		if err != nil {
			t.Fatalf("Get(%s): %v", c.Name, err)
		}
		if loaded.CurrentGameDay != 3 {
			t.Errorf("%s.CurrentGameDay = %d, want 3", c.Name, loaded.CurrentGameDay)
		}
	}

	// Rewind the whole session back to zero.
	if _, err := svc.AdvanceGameDay(ctx, gmRequester, session.ID, -3, nil); err != nil {
		t.Fatalf("AdvanceGameDay(rewind): %v", err)
	}

	loaded, err := charSvc.Get(ctx, gmRequester, aria.ID)
	require.NoError(t, err)
	if loaded.CurrentGameDay != 0 {
		t.Errorf("Aria.CurrentGameDay = %d, want 0", loaded.CurrentGameDay)
	}
}

func TestSessionService_AdvanceGameDay_SingleCharacterCatchUp(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	aria := ownedCharacter(t, charSvc, "Aria")
	bram := ownedCharacter(t, charSvc, "Bram")

	err := svc.AddParticipant(ctx, gmRequester, session.ID, aria.ID)
	require.NoError(t, err)
	err = svc.AddParticipant(ctx, gmRequester, session.ID, bram.ID)
	require.NoError(t, err)

	if _, err := svc.AdvanceGameDay(ctx, gmRequester, session.ID, 2, &bram.ID); err != nil {
		t.Fatalf("AdvanceGameDay(Bram only): %v", err)
	}

	loadedAria, err := charSvc.Get(ctx, gmRequester, aria.ID)
	require.NoError(t, err)
	if loadedAria.CurrentGameDay != 0 {
		t.Errorf("Aria.CurrentGameDay = %d, want 0 (unaffected)", loadedAria.CurrentGameDay)
	}

	loadedBram, err := charSvc.Get(ctx, gmRequester, bram.ID)
	require.NoError(t, err)
	if loadedBram.CurrentGameDay != 2 {
		t.Errorf("Bram.CurrentGameDay = %d, want 2", loadedBram.CurrentGameDay)
	}
}

func TestSessionService_AdvanceGameDay_RejectsNegativeResult(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.AddParticipant(ctx, gmRequester, session.ID, character.ID)
	require.NoError(t, err)

	_, err = svc.AdvanceGameDay(ctx, gmRequester, session.ID, -1, nil)
	if !errors.Is(err, application.ErrInvalidGameDay) {
		t.Fatalf("AdvanceGameDay(negative result) = %v, want ErrInvalidGameDay", err)
	}
}

func TestSessionService_AdvanceGameDay_UnknownCharacterRejected(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	_, err := svc.AdvanceGameDay(ctx, gmRequester, session.ID, 1, &character.ID)
	if !errors.Is(err, application.ErrNotParticipant) {
		t.Fatalf("AdvanceGameDay(non-participant) = %v, want ErrNotParticipant", err)
	}
}

func TestSessionService_AdvanceGameDay_PlayerForbidden(t *testing.T) {
	svc, _ := newTestSessionEnv(t)
	session := createSession(t, svc, "Session One")

	_, err := svc.AdvanceGameDay(t.Context(), playerRequester, session.ID, 1, nil)
	if !errors.Is(err, application.ErrForbidden) {
		t.Fatalf("AdvanceGameDay as player = %v, want ErrForbidden", err)
	}
}

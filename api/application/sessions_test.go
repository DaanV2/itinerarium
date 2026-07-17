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
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestSessionService_Create_RejectsEmptyName(t *testing.T) {
	svc, _ := newTestSessionEnv(t)

	_, err := svc.Create(t.Context(), gmRequester, "", "")
	require.ErrorIs(t, err, application.ErrInvalidName)
}

func TestSessionService_List_PlayerForbidden(t *testing.T) {
	svc, _ := newTestSessionEnv(t)
	createSession(t, svc, "Session One")

	_, err := svc.List(t.Context(), playerRequester)
	require.ErrorIs(t, err, application.ErrForbidden)
}

func TestSessionService_Update_PlayerForbidden(t *testing.T) {
	svc, _ := newTestSessionEnv(t)
	session := createSession(t, svc, "Session One")

	newName := "Session Renamed"

	_, err := svc.Update(t.Context(), playerRequester, session.ID, &newName, nil)
	require.ErrorIs(t, err, application.ErrForbidden)
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
	require.Len(t, loaded.Participants, 1)
	assert.Equal(t, character.ID, loaded.Participants[0].ID)

	err = svc.RemoveParticipant(ctx, gmRequester, session.ID, character.ID)
	require.NoError(t, err)

	loaded, err = svc.Get(ctx, gmRequester, session.ID)
	require.NoError(t, err)
	assert.Empty(t, loaded.Participants)
}

func TestSessionService_AddParticipant_PlayerForbidden(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.AddParticipant(t.Context(), playerRequester, session.ID, character.ID)
	require.ErrorIs(t, err, application.ErrForbidden)
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
	require.ErrorIs(t, err, application.ErrNotParticipant)
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

	_, err = svc.AdvanceGameDay(ctx, gmRequester, session.ID, 3, nil)
	require.NoError(t, err)

	for _, c := range []*models.Character{aria, bram} {
		loaded, err := charSvc.Get(ctx, gmRequester, c.ID)
		require.NoError(t, err, "Get(%s)", c.Name)
		assert.Equal(t, 3, loaded.CurrentGameDay, "%s.CurrentGameDay", c.Name)
	}

	// Rewind the whole session back to zero.
	_, err = svc.AdvanceGameDay(ctx, gmRequester, session.ID, -3, nil)
	require.NoError(t, err)

	loaded, err := charSvc.Get(ctx, gmRequester, aria.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, loaded.CurrentGameDay, "Aria.CurrentGameDay")
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

	_, err = svc.AdvanceGameDay(ctx, gmRequester, session.ID, 2, &bram.ID)
	require.NoError(t, err)

	loadedAria, err := charSvc.Get(ctx, gmRequester, aria.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, loadedAria.CurrentGameDay, "Aria.CurrentGameDay should be unaffected")

	loadedBram, err := charSvc.Get(ctx, gmRequester, bram.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, loadedBram.CurrentGameDay, "Bram.CurrentGameDay")
}

func TestSessionService_AdvanceGameDay_RejectsNegativeResult(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	err := svc.AddParticipant(ctx, gmRequester, session.ID, character.ID)
	require.NoError(t, err)

	_, err = svc.AdvanceGameDay(ctx, gmRequester, session.ID, -1, nil)
	require.ErrorIs(t, err, application.ErrInvalidGameDay)
}

func TestSessionService_AdvanceGameDay_UnknownCharacterRejected(t *testing.T) {
	svc, charSvc := newTestSessionEnv(t)
	ctx := t.Context()
	session := createSession(t, svc, "Session One")
	character := ownedCharacter(t, charSvc, "Aria")

	_, err := svc.AdvanceGameDay(ctx, gmRequester, session.ID, 1, &character.ID)
	require.ErrorIs(t, err, application.ErrNotParticipant)
}

func TestSessionService_AdvanceGameDay_PlayerForbidden(t *testing.T) {
	svc, _ := newTestSessionEnv(t)
	session := createSession(t, svc, "Session One")

	_, err := svc.AdvanceGameDay(t.Context(), playerRequester, session.ID, 1, nil)
	require.ErrorIs(t, err, application.ErrForbidden)
}

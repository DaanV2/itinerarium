package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrAlreadyParticipant is returned when a character is added to a session it
// already participates in.
var ErrAlreadyParticipant = serviceErr(KindConflict, "character already participates in session")

// ErrNotParticipant is returned when a character is removed from — or a game
// day advance is targeted at — a session it does not participate in.
var ErrNotParticipant = serviceErr(KindConflict, "character does not participate in session")

// SessionService manages play sessions and the characters that take part in
// them. Sessions are a GM operational tool: only a GM creates, edits, or
// manages participants, and only a GM moves game day forward or back, either
// for every participant at once or for one character catching up (core
// domain rule / M4).
type SessionService struct {
	sessions   *repositories.Sessions
	characters *CharacterService
}

// NewSessionService builds a SessionService.
func NewSessionService(sessions *repositories.Sessions, characters *CharacterService) *SessionService {
	return &SessionService{sessions: sessions, characters: characters}
}

// Create adds a new session. GM only.
func (s *SessionService) Create(
	ctx context.Context, requester Requester, name, description string,
) (*models.Session, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if name == "" {
		return nil, ErrInvalidName
	}

	session := &models.Session{Name: name, Description: description}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	return session, nil
}

// List returns every session with its participants. GM only.
func (s *SessionService) List(ctx context.Context, requester Requester) ([]models.Session, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	sessions, err := s.sessions.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	return sessions, nil
}

// Get returns one session with its current participants. GM only.
func (s *SessionService) Get(ctx context.Context, requester Requester, id string) (*models.Session, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	session, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading session: %w", err)
	}

	return session, nil
}

// Update changes a session's name and/or description. GM only.
func (s *SessionService) Update(
	ctx context.Context, requester Requester, id string, name, description *string,
) (*models.Session, error) {
	session, err := s.Get(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, ErrInvalidName
		}

		session.Name = *name
	}
	if description != nil {
		session.Description = *description
	}

	if err := s.sessions.Update(ctx, session); err != nil {
		return nil, fmt.Errorf("updating session: %w", err)
	}

	return session, nil
}

// AddParticipant adds a character to a session. GM only.
func (s *SessionService) AddParticipant(ctx context.Context, requester Requester, sessionID, characterID string) error {
	session, character, err := s.loadSessionAndCharacter(ctx, requester, sessionID, characterID)
	if err != nil {
		return err
	}

	isParticipant, err := s.sessions.IsParticipant(ctx, sessionID, characterID)
	if err != nil {
		return fmt.Errorf("checking participation: %w", err)
	}
	if isParticipant {
		return ErrAlreadyParticipant
	}

	if err := s.sessions.AddParticipant(ctx, session, character); err != nil {
		return fmt.Errorf("adding participant: %w", err)
	}

	return nil
}

// RemoveParticipant removes a character from a session. GM only.
func (s *SessionService) RemoveParticipant(
	ctx context.Context, requester Requester, sessionID, characterID string,
) error {
	session, character, err := s.loadSessionAndCharacter(ctx, requester, sessionID, characterID)
	if err != nil {
		return err
	}

	isParticipant, err := s.sessions.IsParticipant(ctx, sessionID, characterID)
	if err != nil {
		return fmt.Errorf("checking participation: %w", err)
	}
	if !isParticipant {
		return ErrNotParticipant
	}

	if err := s.sessions.RemoveParticipant(ctx, session, character); err != nil {
		return fmt.Errorf("removing participant: %w", err)
	}

	return nil
}

// AdvanceGameDay moves game day by delta (negative rewinds) for every
// participant of the session, or — when characterID is set — for just that
// one participant, letting a GM catch up a single player. The resulting game
// day can never go negative.
func (s *SessionService) AdvanceGameDay(
	ctx context.Context, requester Requester, sessionID string, delta int, characterID *string,
) (*models.Session, error) {
	session, err := s.Get(ctx, requester, sessionID)
	if err != nil {
		return nil, err
	}

	targets := session.Participants
	if characterID != nil {
		target, found := participantByID(session.Participants, *characterID)
		if !found {
			return nil, ErrNotParticipant
		}

		targets = []models.Character{target}
	}

	for i := range targets {
		newDay := targets[i].CurrentGameDay + delta
		if newDay < 0 {
			return nil, ErrInvalidGameDay
		}

		if _, err := s.characters.Update(ctx, requester, targets[i].ID, nil, &newDay); err != nil {
			return nil, fmt.Errorf("advancing game day: %w", err)
		}
	}

	return s.Get(ctx, requester, sessionID)
}

// loadSessionAndCharacter resolves both sides of a participant change.
func (s *SessionService) loadSessionAndCharacter(
	ctx context.Context, requester Requester, sessionID, characterID string,
) (*models.Session, *models.Character, error) {
	session, err := s.Get(ctx, requester, sessionID)
	if err != nil {
		return nil, nil, err
	}

	character, err := s.characters.Get(ctx, requester, characterID)
	if err != nil {
		return nil, nil, err
	}

	return session, character, nil
}

// participantByID finds a character in participants by ID.
func participantByID(participants []models.Character, id string) (models.Character, bool) {
	for i := range participants {
		if participants[i].ID == id {
			return participants[i], true
		}
	}

	return models.Character{}, false
}

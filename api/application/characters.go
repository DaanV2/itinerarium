package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidName is returned when a character is created or renamed with an
// empty name.
var ErrInvalidName = serviceErr(KindValidation, "invalid name")

// ErrInvalidGameDay is returned when current_game_day is set to a negative
// value.
var ErrInvalidGameDay = serviceErr(KindValidation, "invalid game day")

// CharacterService manages player characters. A user account may own
// multiple characters; only a GM may create a character on behalf of another
// user or move current_game_day directly (normal play advances it
// per-session, see the M4 session workflow).
type CharacterService struct {
	characters *repositories.Characters
	users      *repositories.Users
	knowledge  *repositories.KnowledgeRepositories
}

// NewCharacterService builds a CharacterService.
func NewCharacterService(
	characters *repositories.Characters, users *repositories.Users, knowledge *repositories.KnowledgeRepositories,
) *CharacterService {
	return &CharacterService{characters: characters, users: users, knowledge: knowledge}
}

// Create adds a new character owned by ownerUserID (defaulting to the
// requester). Players may only create characters for themselves; GMs may
// create one for any existing user.
func (s *CharacterService) Create(
	ctx context.Context, requester Requester, ownerUserID, name string,
) (*models.Character, error) {
	if ownerUserID == "" {
		ownerUserID = requester.UserID()
	}
	if name == "" {
		return nil, ErrInvalidName
	}
	if !requester.IsGM() && ownerUserID != requester.UserID() {
		return nil, ErrForbidden
	}

	if ownerUserID != requester.UserID() {
		if _, err := s.users.GetByID(ctx, ownerUserID); err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return nil, ErrNotFound
			}

			return nil, fmt.Errorf("looking up owner: %w", err)
		}
	}

	character := &models.Character{Name: name, UserID: ownerUserID}
	if err := s.characters.Create(ctx, character); err != nil {
		return nil, fmt.Errorf("creating character: %w", err)
	}

	if _, err := s.knowledge.EnsureForCharacter(ctx, character.ID); err != nil {
		return nil, fmt.Errorf("provisioning character repository: %w", err)
	}

	return character, nil
}

// List returns the requester's own characters, or every character for a GM.
func (s *CharacterService) List(ctx context.Context, requester Requester) ([]models.Character, error) {
	if requester.IsGM() {
		characters, err := s.characters.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing characters: %w", err)
		}

		return characters, nil
	}

	characters, err := s.characters.ListByUser(ctx, requester.UserID())
	if err != nil {
		return nil, fmt.Errorf("listing characters: %w", err)
	}

	return characters, nil
}

// Get returns a character only if the requester owns it or is a GM —
// otherwise ErrNotFound, never ErrForbidden (existence must not leak).
func (s *CharacterService) Get(ctx context.Context, requester Requester, id string) (*models.Character, error) {
	c, err := s.characters.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading character: %w", err)
	}
	if !requester.IsGM() && c.UserID != requester.UserID() {
		return nil, ErrNotFound
	}

	return c, nil
}

// Update renames a character and/or sets its current_game_day. Owners (and
// GMs) may rename; only a GM may move current_game_day directly.
func (s *CharacterService) Update(
	ctx context.Context, requester Requester, id string, name *string, currentGameDay *int,
) (*models.Character, error) {
	c, err := s.Get(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if currentGameDay != nil {
		if !requester.IsGM() {
			return nil, ErrForbidden
		}
		if *currentGameDay < 0 {
			return nil, ErrInvalidGameDay
		}

		c.CurrentGameDay = *currentGameDay
	}

	if name != nil {
		if *name == "" {
			return nil, ErrInvalidName
		}

		c.Name = *name
	}

	if err := s.characters.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("updating character: %w", err)
	}

	return c, nil
}

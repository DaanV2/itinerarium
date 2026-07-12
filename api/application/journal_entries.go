package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidContent is returned when a journal entry is created or edited
// with empty content.
var ErrInvalidContent = errors.New("invalid content")

// JournalEntryService manages per-character journal entries. Entries are
// stamped with the character's current_game_day at creation and are readable
// and editable only by the owning player and GMs — never other players.
type JournalEntryService struct {
	entries    *repositories.JournalEntries
	characters *repositories.Characters
}

// NewJournalEntryService builds a JournalEntryService.
func NewJournalEntryService(
	entries *repositories.JournalEntries, characters *repositories.Characters,
) *JournalEntryService {
	return &JournalEntryService{entries: entries, characters: characters}
}

// requireCharacterAccess returns the character only if the requester owns it
// or is a GM — otherwise ErrNotFound, never ErrForbidden (existence must not
// leak).
func (s *JournalEntryService) requireCharacterAccess(
	ctx context.Context, requester Requester, characterID string,
) (*models.Character, error) {
	c, err := s.characters.GetByID(ctx, characterID)
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

// Create adds a journal entry to a character, stamped with the character's
// current_game_day.
func (s *JournalEntryService) Create(
	ctx context.Context, requester Requester, characterID, content string,
) (*models.JournalEntry, error) {
	if content == "" {
		return nil, ErrInvalidContent
	}

	c, err := s.requireCharacterAccess(ctx, requester, characterID)
	if err != nil {
		return nil, err
	}

	entry := &models.JournalEntry{CharacterID: c.ID, GameDay: c.CurrentGameDay, Content: content}
	if err := s.entries.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("creating journal entry: %w", err)
	}

	return entry, nil
}

// List returns every journal entry for a character, ordered by game day.
func (s *JournalEntryService) List(
	ctx context.Context, requester Requester, characterID string,
) ([]models.JournalEntry, error) {
	if _, err := s.requireCharacterAccess(ctx, requester, characterID); err != nil {
		return nil, err
	}

	entries, err := s.entries.ListByCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("listing journal entries: %w", err)
	}

	return entries, nil
}

// Get returns a single journal entry only if the requester owns the
// character it belongs to or is a GM — otherwise ErrNotFound.
func (s *JournalEntryService) Get(
	ctx context.Context, requester Requester, id string,
) (*models.JournalEntry, error) {
	e, err := s.entries.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading journal entry: %w", err)
	}

	if _, err := s.requireCharacterAccess(ctx, requester, e.CharacterID); err != nil {
		return nil, err
	}

	return e, nil
}

// Update edits a journal entry's content. The game day it was stamped with
// never changes.
func (s *JournalEntryService) Update(
	ctx context.Context, requester Requester, id, content string,
) (*models.JournalEntry, error) {
	if content == "" {
		return nil, ErrInvalidContent
	}

	e, err := s.Get(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	e.Content = content
	if err := s.entries.Update(ctx, e); err != nil {
		return nil, fmt.Errorf("updating journal entry: %w", err)
	}

	return e, nil
}

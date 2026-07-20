package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ShareToGroup moves a document out of a character's private repository into
// a group's repository at a chosen SharedOnGameDay (core domain rule 5's
// counterpart for groups: sharing is a move, not a copy — the document
// leaves the character repository for good and normal group-repository
// rules apply from then on). Only documents currently in a character
// repository can be shared this way. The caller needs the usual document
// access to the source (owner + GM, enforced by getAccessible) and needs to
// be able to see the target group repository too (a player only via one of
// their characters' membership; a GM always); either check failing reads as
// ErrNotFound so membership never leaks. A path already occupied in the
// target repository warns with ErrPathCollision unless AllowCollision is
// set.
func (s *DocumentService) ShareToGroup(
	ctx context.Context, requester Requester, id string, input *ShareDocumentInput,
) (*DocumentView, error) {
	doc, repo, err := s.getAccessible(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if repo.Type != models.RepositoryTypeCharacter {
		return nil, fmt.Errorf("%w: only a document in a character repository can be shared to a group", ErrInvalidDocument)
	}

	target, err := s.repositories.Get(ctx, requester, input.TargetRepositoryID)
	if err != nil {
		return nil, err
	}
	if target.Type != models.RepositoryTypeGroup {
		return nil, fmt.Errorf("%w: documents can only be shared to a group repository", ErrInvalidDocument)
	}

	if !input.AllowCollision {
		exists, err := s.documents.ExistsAtPath(ctx, target.ID, doc.Path, doc.ID)
		if err != nil {
			return nil, fmt.Errorf("checking path collision: %w", err)
		}
		if exists {
			return nil, ErrPathCollision
		}
	}

	doc.RepositoryID = target.ID
	doc.SharedOnGameDay = input.SharedOnGameDay
	doc.Version++

	// Sharing lands the document in the group's repository — to members that
	// is a new document, so the event is recorded as an addition there.
	entry, err := s.documentEntry(ctx, requester, target, doc, models.ActivityActionAdded)
	if err != nil {
		return nil, err
	}
	if err := s.documents.Update(ctx, doc, doc.Sections, entry); err != nil {
		return nil, fmt.Errorf("sharing document: %w", err)
	}

	doc, err = s.documents.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("reloading document: %w", err)
	}

	return s.view(ctx, requester, target, doc)
}

// ShareWithCharacter directly shares a document with one character, revealed
// to them once their current_game_day reaches sharedOnGameDay — independent
// of the document's own repository access. GM only.
func (s *DocumentService) ShareWithCharacter(
	ctx context.Context, requester Requester, documentID, characterID string, sharedOnGameDay int,
) (*models.DocumentShare, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	if _, err := s.documents.GetByID(ctx, documentID); err != nil {
		return nil, notFoundOr(err, "loading document")
	}
	if _, err := s.characters.GetByID(ctx, characterID); err != nil {
		return nil, notFoundOr(err, "loading character")
	}

	exists, err := s.shares.Exists(ctx, documentID, characterID)
	if err != nil {
		return nil, fmt.Errorf("checking existing share: %w", err)
	}
	if exists {
		return nil, ErrAlreadyShared
	}

	share := &models.DocumentShare{DocumentID: documentID, CharacterID: characterID, SharedOnGameDay: sharedOnGameDay}
	if err := s.shares.Create(ctx, share); err != nil {
		return nil, fmt.Errorf("sharing document: %w", err)
	}

	return share, nil
}

// ListShares returns every direct share on a document. GM only — players
// never see the share list.
func (s *DocumentService) ListShares(
	ctx context.Context, requester Requester, documentID string,
) ([]models.DocumentShare, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	if _, err := s.documents.GetByID(ctx, documentID); err != nil {
		return nil, notFoundOr(err, "loading document")
	}

	shares, err := s.shares.ListByDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("listing shares: %w", err)
	}

	return shares, nil
}

// RevokeShare removes one direct share from a document. GM only.
func (s *DocumentService) RevokeShare(ctx context.Context, requester Requester, documentID, shareID string) error {
	if !requester.IsGM() {
		return ErrForbidden
	}

	share, err := s.shares.GetByID(ctx, shareID)
	if err != nil {
		return notFoundOr(err, "loading share")
	}
	if share.DocumentID != documentID {
		return ErrNotFound
	}

	if err := s.shares.Delete(ctx, share); err != nil {
		return fmt.Errorf("revoking share: %w", err)
	}

	return nil
}

// ListSharedWithMe returns the documents directly shared with any of the
// requester's characters whose game day has been reached, with sections
// stripped to what the requester may see.
func (s *DocumentService) ListSharedWithMe(ctx context.Context, requester Requester) ([]DocumentView, error) {
	characters, err := s.characters.ListByUser(ctx, requester.UserID())
	if err != nil {
		return nil, fmt.Errorf("listing requester characters: %w", err)
	}

	characterIDs := make([]string, len(characters))
	dayByCharacter := make(map[string]int, len(characters))
	for i := range characters {
		characterIDs[i] = characters[i].ID
		dayByCharacter[characters[i].ID] = characters[i].CurrentGameDay
	}

	shares, err := s.shares.ListByCharacters(ctx, characterIDs)
	if err != nil {
		return nil, fmt.Errorf("listing shares: %w", err)
	}

	seen := make(map[string]struct{}, len(shares))
	views := make([]DocumentView, 0, len(shares))
	for i := range shares {
		share := &shares[i]
		if dayByCharacter[share.CharacterID] < share.SharedOnGameDay {
			continue
		}
		if _, dup := seen[share.DocumentID]; dup {
			continue
		}
		seen[share.DocumentID] = struct{}{}

		doc, err := s.documents.GetByID(ctx, share.DocumentID)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				continue
			}

			return nil, fmt.Errorf("loading shared document: %w", err)
		}

		repo, err := s.repositories.GetUnchecked(ctx, doc.RepositoryID)
		if err != nil {
			return nil, err
		}

		view, err := s.view(ctx, requester, repo, doc)
		if err != nil {
			return nil, err
		}

		views = append(views, *view)
	}

	return views, nil
}

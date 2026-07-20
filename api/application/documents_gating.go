package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// getAccessible loads a document and enforces every read rule: repository
// access and, for players, the game-day gate — or, failing that, a direct
// share to one of the requester's characters that has reached its own game
// day (core domain rule 1, roadmap M3). Anything out of reach reads as
// ErrNotFound so existence never leaks.
func (s *DocumentService) getAccessible(
	ctx context.Context, requester Requester, id string,
) (*models.Document, *models.Repository, error) {
	doc, err := s.documents.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, nil, ErrNotFound
		}

		return nil, nil, fmt.Errorf("loading document: %w", err)
	}

	repo, err := s.repositories.Get(ctx, requester, doc.RepositoryID)
	if err != nil {
		if requester.IsGM() || !errors.Is(err, ErrNotFound) {
			return nil, nil, err
		}

		return s.getViaDirectShare(ctx, requester, doc)
	}

	if requester.IsGM() {
		return doc, repo, nil
	}

	day, ok, err := s.effectiveGameDay(ctx, requester, repo)
	if err != nil {
		return nil, nil, err
	}
	if ok && doc.SharedOnGameDay <= day {
		return doc, repo, nil
	}

	return s.getViaDirectShare(ctx, requester, doc)
}

// getViaDirectShare checks whether any of the requester's characters holds a
// direct share on doc that has reached its own SharedOnGameDay — the
// fallback path when the document's repository doesn't (yet) grant access.
func (s *DocumentService) getViaDirectShare(
	ctx context.Context, requester Requester, doc *models.Document,
) (*models.Document, *models.Repository, error) {
	characters, err := requesterCharacters(ctx, s.characters, requester)
	if err != nil {
		return nil, nil, fmt.Errorf("listing requester characters: %w", err)
	}

	characterIDs := make([]string, len(characters))
	dayByCharacter := make(map[string]int, len(characters))
	for i := range characters {
		characterIDs[i] = characters[i].ID
		dayByCharacter[characters[i].ID] = characters[i].CurrentGameDay
	}

	shares, err := s.shares.ListForCharacters(ctx, doc.ID, characterIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("checking direct shares: %w", err)
	}

	reached := false
	for i := range shares {
		if dayByCharacter[shares[i].CharacterID] >= shares[i].SharedOnGameDay {
			reached = true

			break
		}
	}
	if !reached {
		return nil, nil, ErrNotFound
	}

	repo, err := s.repositories.GetUnchecked(ctx, doc.RepositoryID)
	if err != nil {
		return nil, nil, err
	}

	return doc, repo, nil
}

// revealed reports whether any character with access to the repository has
// reached the given game day.
func (s *DocumentService) revealed(
	ctx context.Context, repo *models.Repository, sharedOnGameDay int,
) (bool, error) {
	switch repo.Type {
	case models.RepositoryTypeGeneral, models.RepositoryTypeTemplate:
		revealed, err := s.characters.AnyWithGameDayAtLeast(ctx, sharedOnGameDay)
		if err != nil {
			return false, fmt.Errorf("checking reveal state: %w", err)
		}

		return revealed, nil
	case models.RepositoryTypeGroup:
		if repo.GroupID == nil {
			return false, nil
		}

		group, err := cachedGroup(ctx, s.groups, *repo.GroupID)
		if err != nil {
			return false, fmt.Errorf("loading group: %w", err)
		}

		for i := range group.Members {
			if group.Members[i].CurrentGameDay >= sharedOnGameDay {
				return true, nil
			}
		}

		return false, nil
	case models.RepositoryTypeCharacter:
		if repo.CharacterID == nil {
			return false, nil
		}

		character, err := s.characters.GetByID(ctx, *repo.CharacterID)
		if err != nil {
			return false, fmt.Errorf("loading character: %w", err)
		}

		return character.CurrentGameDay >= sharedOnGameDay, nil
	default:
		return false, nil
	}
}

// repoAccessSource adapts a repository's type-based visibility to the shared
// accessSource so documents share one visibility gate with locations (gate.go).
type repoAccessSource struct {
	svc  *DocumentService
	repo *models.Repository
}

func (r repoAccessSource) charactersWithAccess(
	ctx context.Context, characters []models.Character,
) ([]models.Character, error) {
	return r.svc.charactersWithRepoAccess(ctx, r.repo, characters)
}

func (r repoAccessSource) revealed(ctx context.Context, sharedOnGameDay int) (bool, error) {
	return r.svc.revealed(ctx, r.repo, sharedOnGameDay)
}

// repoAccess builds the accessSource for one repository.
func (s *DocumentService) repoAccess(repo *models.Repository) accessSource {
	return repoAccessSource{svc: s, repo: repo}
}

// effectiveGameDay resolves the highest current_game_day among the
// requester's characters that can reach the repository. ok is false when no
// character of theirs reaches it at all.
func (s *DocumentService) effectiveGameDay(
	ctx context.Context, requester Requester, repo *models.Repository,
) (day int, ok bool, err error) {
	characters, err := requesterCharacters(ctx, s.characters, requester)
	if err != nil {
		return 0, false, fmt.Errorf("listing requester characters: %w", err)
	}

	return requesterDay(ctx, s.repoAccess(repo), characters)
}

// charactersWithRepoAccess filters the requester's characters down to those
// the repository's own visibility rule grants access.
func (s *DocumentService) charactersWithRepoAccess(
	ctx context.Context, repo *models.Repository, characters []models.Character,
) ([]models.Character, error) {
	switch repo.Type {
	case models.RepositoryTypeGeneral, models.RepositoryTypeTemplate:
		return characters, nil
	case models.RepositoryTypeCharacter:
		if repo.CharacterID == nil {
			return nil, nil
		}

		for i := range characters {
			if characters[i].ID == *repo.CharacterID {
				return characters[i : i+1], nil
			}
		}

		return nil, nil
	case models.RepositoryTypeGroup:
		if repo.GroupID == nil {
			return nil, nil
		}

		group, err := cachedGroup(ctx, s.groups, *repo.GroupID)
		if err != nil {
			return nil, fmt.Errorf("loading group: %w", err)
		}

		memberIDs := make(map[string]struct{}, len(group.Members))
		for i := range group.Members {
			memberIDs[group.Members[i].ID] = struct{}{}
		}

		var eligible []models.Character
		for i := range characters {
			if _, member := memberIDs[characters[i].ID]; member {
				eligible = append(eligible, characters[i])
			}
		}

		return eligible, nil
	default:
		return nil, nil
	}
}

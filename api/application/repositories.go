package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// RepositoryService manages knowledge repositories: the general and template
// singletons, and the one-per-group / one-per-character vaults. Repositories
// are provisioned automatically — by EnsureSystemRepositories at startup and
// by CharacterService/GroupService when their owner is created — never
// created directly by a caller.
//
// Visibility: general and template repositories are visible to everyone; a
// group repository to its members; a character repository to its owner. GMs
// always see every repository. A caller without access gets ErrNotFound,
// never ErrForbidden — existence must not leak.
type RepositoryService struct {
	repos      *repositories.KnowledgeRepositories
	groups     *repositories.Groups
	characters *repositories.Characters
}

// NewRepositoryService builds a RepositoryService.
func NewRepositoryService(
	repos *repositories.KnowledgeRepositories, groups *repositories.Groups, characters *repositories.Characters,
) *RepositoryService {
	return &RepositoryService{repos: repos, groups: groups, characters: characters}
}

// EnsureSystemRepositories provisions the campaign-wide general and template
// repositories if they don't already exist. Idempotent; call once at
// startup.
func (s *RepositoryService) EnsureSystemRepositories(ctx context.Context) error {
	if _, err := s.repos.EnsureGeneral(ctx); err != nil {
		return fmt.Errorf("ensuring general repository: %w", err)
	}
	if _, err := s.repos.EnsureTemplate(ctx); err != nil {
		return fmt.Errorf("ensuring template repository: %w", err)
	}

	return nil
}

// Get returns one repository only if the requester may see it — otherwise
// ErrNotFound, never ErrForbidden.
func (s *RepositoryService) Get(ctx context.Context, requester Requester, id string) (*models.Repository, error) {
	repo, err := s.repos.GetByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "loading repository")
	}

	if requester.IsGM() {
		return repo, nil
	}

	ok, err := s.accessible(ctx, requester, repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}

	return repo, nil
}

// List returns every repository for a GM, and the general/template
// singletons plus the requester's own character and group repositories for a
// player.
func (s *RepositoryService) List(ctx context.Context, requester Requester) ([]models.Repository, error) {
	if requester.IsGM() {
		repos, err := s.repos.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing repositories: %w", err)
		}

		return repos, nil
	}

	characterIDs, groupIDs, err := s.requesterScope(ctx, requester)
	if err != nil {
		return nil, err
	}

	repos, err := s.repos.ListVisible(ctx, characterIDs, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("listing repositories: %w", err)
	}

	return repos, nil
}

// accessible reports whether a non-GM requester may see repo.
func (s *RepositoryService) accessible(ctx context.Context, requester Requester, repo *models.Repository) (bool, error) {
	switch repo.Type {
	case models.RepositoryTypeGeneral, models.RepositoryTypeTemplate:
		return true, nil
	case models.RepositoryTypeCharacter:
		if repo.CharacterID == nil {
			return false, nil
		}

		character, err := s.characters.GetByID(ctx, *repo.CharacterID)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return false, nil
			}

			return false, fmt.Errorf("loading character: %w", err)
		}

		return character.UserID == requester.UserID(), nil
	case models.RepositoryTypeGroup:
		if repo.GroupID == nil {
			return false, nil
		}

		_, groupIDs, err := s.requesterScope(ctx, requester)
		if err != nil {
			return false, err
		}

		return slices.Contains(groupIDs, *repo.GroupID), nil
	default:
		return false, nil
	}
}

// requesterScope resolves a player's characters and those characters'
// groups — the two paths a repository grant can reach them through.
func (s *RepositoryService) requesterScope(
	ctx context.Context, requester Requester,
) (characterIDs, groupIDs []string, err error) {
	characters, err := s.characters.ListByUser(ctx, requester.UserID())
	if err != nil {
		return nil, nil, fmt.Errorf("listing requester characters: %w", err)
	}

	characterIDs = make([]string, len(characters))
	for i := range characters {
		characterIDs[i] = characters[i].ID
	}

	groupIDs, err = s.groups.GroupIDsForCharacters(ctx, characterIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving requester groups: %w", err)
	}

	return characterIDs, groupIDs, nil
}

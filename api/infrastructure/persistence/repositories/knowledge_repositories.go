package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// KnowledgeRepositories provides access to knowledge Repository rows — the
// general/template singletons plus the one-per-group and one-per-character
// vaults. Named to avoid colliding with this package's own name.
type KnowledgeRepositories struct{ db *persistence.Database }

// NewKnowledgeRepositories builds a KnowledgeRepositories repository.
func NewKnowledgeRepositories(db *persistence.Database) *KnowledgeRepositories {
	return &KnowledgeRepositories{db: db}
}

// EnsureGeneral returns the campaign-wide general repository, creating it if
// it doesn't exist yet. Idempotent.
func (r *KnowledgeRepositories) EnsureGeneral(ctx context.Context) (*models.Repository, error) {
	return r.ensureSingleton(ctx, models.RepositoryTypeGeneral)
}

// EnsureTemplate returns the campaign-wide template repository, creating it
// if it doesn't exist yet. Idempotent.
func (r *KnowledgeRepositories) EnsureTemplate(ctx context.Context) (*models.Repository, error) {
	return r.ensureSingleton(ctx, models.RepositoryTypeTemplate)
}

// ensureSingleton returns the one repository of the given type, creating it
// if it doesn't exist yet.
func (r *KnowledgeRepositories) ensureSingleton(ctx context.Context, t models.RepositoryType) (*models.Repository, error) {
	var repo models.Repository

	err := r.db.DB().WithContext(ctx).Where("type = ?", t).First(&repo).Error
	if err == nil {
		return &repo, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	repo = models.Repository{Type: t}
	if err := r.db.DB().WithContext(ctx).Create(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

// EnsureForGroup returns the group's repository, creating it if it doesn't
// exist yet. Idempotent.
func (r *KnowledgeRepositories) EnsureForGroup(ctx context.Context, groupID string) (*models.Repository, error) {
	var repo models.Repository

	err := r.db.DB().WithContext(ctx).
		Where("type = ? AND group_id = ?", models.RepositoryTypeGroup, groupID).First(&repo).Error
	if err == nil {
		return &repo, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	repo = models.Repository{Type: models.RepositoryTypeGroup, GroupID: &groupID}
	if err := r.db.DB().WithContext(ctx).Create(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

// EnsureForCharacter returns the character's repository, creating it if it
// doesn't exist yet. Idempotent.
func (r *KnowledgeRepositories) EnsureForCharacter(ctx context.Context, characterID string) (*models.Repository, error) {
	var repo models.Repository

	err := r.db.DB().WithContext(ctx).
		Where("type = ? AND character_id = ?", models.RepositoryTypeCharacter, characterID).First(&repo).Error
	if err == nil {
		return &repo, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	repo = models.Repository{Type: models.RepositoryTypeCharacter, CharacterID: &characterID}
	if err := r.db.DB().WithContext(ctx).Create(&repo).Error; err != nil {
		return nil, err
	}

	return &repo, nil
}

// GetByID looks up a repository by ID, returning ErrNotFound if none
// matches.
func (r *KnowledgeRepositories) GetByID(ctx context.Context, id string) (*models.Repository, error) {
	var repo models.Repository

	err := r.db.DB().WithContext(ctx).First(&repo, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &repo, nil
}

// List returns every repository (GM-wide view).
func (r *KnowledgeRepositories) List(ctx context.Context) ([]models.Repository, error) {
	var repos []models.Repository

	err := r.db.DB().WithContext(ctx).Order("type").Find(&repos).Error
	if err != nil {
		return nil, err
	}

	return repos, nil
}

// ListVisible returns the general/template repositories plus the group and
// character repositories reachable through the given IDs. Either slice may
// be empty.
func (r *KnowledgeRepositories) ListVisible(
	ctx context.Context, characterIDs, groupIDs []string,
) ([]models.Repository, error) {
	var repos []models.Repository

	query := r.db.DB().WithContext(ctx).Where("type IN ?", []models.RepositoryType{
		models.RepositoryTypeGeneral, models.RepositoryTypeTemplate,
	})
	if len(characterIDs) > 0 {
		query = query.Or("type = ? AND character_id IN ?", models.RepositoryTypeCharacter, characterIDs)
	}
	if len(groupIDs) > 0 {
		query = query.Or("type = ? AND group_id IN ?", models.RepositoryTypeGroup, groupIDs)
	}

	err := query.Order("type").Find(&repos).Error
	if err != nil {
		return nil, err
	}

	return repos, nil
}

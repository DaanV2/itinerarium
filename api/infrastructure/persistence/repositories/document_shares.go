package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// DocumentShares provides access to direct per-character document shares.
type DocumentShares struct{ db *persistence.Database }

// NewDocumentShares builds a DocumentShares repository.
func NewDocumentShares(db *persistence.Database) *DocumentShares {
	return &DocumentShares{db: db}
}

// Create persists a new share.
func (r *DocumentShares) Create(ctx context.Context, s *models.DocumentShare) error {
	err := r.db.DB().WithContext(ctx).Create(s).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a share by ID, returning ErrNotFound if none matches.
func (r *DocumentShares) GetByID(ctx context.Context, id string) (*models.DocumentShare, error) {
	var s models.DocumentShare

	err := r.db.DB().WithContext(ctx).First(&s, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &s, nil
}

// ListByDocument returns every share on one document.
func (r *DocumentShares) ListByDocument(ctx context.Context, documentID string) ([]models.DocumentShare, error) {
	var shares []models.DocumentShare

	err := r.db.DB().WithContext(ctx).
		Where("document_id = ?", documentID).
		Order("created_at").
		Find(&shares).Error
	if err != nil {
		return nil, err
	}

	return shares, nil
}

// ListForCharacters returns every share on the document targeting one of the
// given characters — the set a requester's characters might unlock the
// document through.
func (r *DocumentShares) ListForCharacters(
	ctx context.Context, documentID string, characterIDs []string,
) ([]models.DocumentShare, error) {
	if len(characterIDs) == 0 {
		return nil, nil
	}

	var shares []models.DocumentShare

	err := r.db.DB().WithContext(ctx).
		Where("document_id = ? AND character_id IN ?", documentID, characterIDs).
		Find(&shares).Error
	if err != nil {
		return nil, err
	}

	return shares, nil
}

// ListByCharacters returns every share targeting one of the given characters,
// across all documents — used to build a "shared with me" view.
func (r *DocumentShares) ListByCharacters(ctx context.Context, characterIDs []string) ([]models.DocumentShare, error) {
	if len(characterIDs) == 0 {
		return nil, nil
	}

	var shares []models.DocumentShare

	err := r.db.DB().WithContext(ctx).
		Where("character_id IN ?", characterIDs).
		Find(&shares).Error
	if err != nil {
		return nil, err
	}

	return shares, nil
}

// Exists reports whether the document is already shared with the character.
func (r *DocumentShares) Exists(ctx context.Context, documentID, characterID string) (bool, error) {
	var count int64

	err := r.db.DB().WithContext(ctx).
		Model(&models.DocumentShare{}).
		Where("document_id = ? AND character_id = ?", documentID, characterID).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Delete soft-deletes a share.
func (r *DocumentShares) Delete(ctx context.Context, s *models.DocumentShare) error {
	err := r.db.DB().WithContext(ctx).Delete(s).Error
	if err != nil {
		return err
	}

	return nil
}

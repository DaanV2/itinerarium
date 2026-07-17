package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Documents provides access to knowledge documents and their sections.
type Documents struct{ db *persistence.Database }

// NewDocuments builds a Documents repository.
func NewDocuments(db *persistence.Database) *Documents {
	return &Documents{db: db}
}

// Create persists a new document together with its sections, recording any
// given activity entries in the same transaction.
func (r *Documents) Create(ctx context.Context, d *models.Document, entries ...*models.ActivityEntry) error {
	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(d).Error; err != nil {
			return err
		}

		return createEntries(tx, entries)
	})
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a document by ID with its sections preloaded in order.
// It returns ErrNotFound if no document matches.
func (r *Documents) GetByID(ctx context.Context, id string) (*models.Document, error) {
	var d models.Document

	err := r.db.DB().WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB { return db.Order("position") }).
		First(&d, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &d, nil
}

// ListByRepository returns every document in a repository (without sections),
// ordered by path.
func (r *Documents) ListByRepository(ctx context.Context, repositoryID string) ([]models.Document, error) {
	var docs []models.Document

	err := r.db.DB().WithContext(ctx).
		Where("repository_id = ?", repositoryID).Order("path").Find(&docs).Error
	if err != nil {
		return nil, err
	}

	return docs, nil
}

// ExistsAtPath reports whether another document (excluding excludeID, which
// may be empty) already sits at the given path inside the repository — the
// path-collision check.
func (r *Documents) ExistsAtPath(
	ctx context.Context, repositoryID, path, excludeID string,
) (bool, error) {
	var count int64

	query := r.db.DB().WithContext(ctx).Model(&models.Document{}).
		Where("repository_id = ? AND path = ?", repositoryID, path)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}

	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Update persists the document's own fields and replaces its section rows
// with the given final list, all in one transaction — recording any given
// activity entries in that same transaction. Sections carrying an ID are
// updated in place (so player edits keep GM-only rows untouched), sections
// without an ID are inserted, and existing rows absent from the list are
// deleted.
func (r *Documents) Update(
	ctx context.Context, d *models.Document, sections []models.DocumentSection,
	entries ...*models.ActivityEntry,
) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Save(d).Error; err != nil {
			return err
		}

		keep := make([]string, 0, len(sections))
		for i := range sections {
			sections[i].DocumentID = d.ID
			sections[i].Position = i
			if err := tx.Omit(clause.Associations).Save(&sections[i]).Error; err != nil {
				return err
			}

			keep = append(keep, sections[i].ID)
		}

		query := tx.Where("document_id = ?", d.ID)
		if len(keep) > 0 {
			query = query.Where("id NOT IN ?", keep)
		}

		if err := query.Delete(&models.DocumentSection{}).Error; err != nil {
			return err
		}

		return createEntries(tx, entries)
	})
}

// Delete soft-deletes a document and its sections in one transaction,
// recording any given activity entries in that same transaction.
func (r *Documents) Delete(ctx context.Context, d *models.Document, entries ...*models.ActivityEntry) error {
	return r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", d.ID).Delete(&models.DocumentSection{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(d).Error; err != nil {
			return err
		}

		return createEntries(tx, entries)
	})
}

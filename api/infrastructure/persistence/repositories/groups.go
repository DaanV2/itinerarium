package repositories

import (
	"context"
	"errors"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"gorm.io/gorm"
)

// Groups provides access to character groups and their membership.
type Groups struct{ db *persistence.Database }

// NewGroups builds a Groups repository.
func NewGroups(db *persistence.Database) *Groups {
	return &Groups{db: db}
}

// Create persists a new group.
func (r *Groups) Create(ctx context.Context, g *models.Group) error {
	err := r.db.DB().WithContext(ctx).Create(g).Error
	if err != nil {
		return err
	}

	return nil
}

// GetByID looks up a group by ID with its current members preloaded. It
// returns ErrNotFound if no group matches.
func (r *Groups) GetByID(ctx context.Context, id string) (*models.Group, error) {
	var g models.Group

	err := r.db.DB().WithContext(ctx).Preload("Members").First(&g, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return &g, nil
}

// List returns every group with members preloaded, ordered by name.
func (r *Groups) List(ctx context.Context) ([]models.Group, error) {
	var groups []models.Group

	err := r.db.DB().WithContext(ctx).Preload("Members").Order("name").Find(&groups).Error
	if err != nil {
		return nil, err
	}

	return groups, nil
}

// Update persists changes to a group's own columns (not its member list).
func (r *Groups) Update(ctx context.Context, g *models.Group) error {
	err := r.db.DB().WithContext(ctx).Omit("Members").Save(g).Error
	if err != nil {
		return err
	}

	return nil
}

// IsMember reports whether the character is currently a member of the group.
func (r *Groups) IsMember(ctx context.Context, groupID, characterID string) (bool, error) {
	var count int64

	err := r.db.DB().WithContext(ctx).
		Table("group_members").
		Where("group_id = ? AND character_id = ?", groupID, characterID).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GroupIDsForCharacters returns the IDs of every group that has at least one
// of the given characters as a member.
func (r *Groups) GroupIDsForCharacters(ctx context.Context, characterIDs []string) ([]string, error) {
	if len(characterIDs) == 0 {
		return nil, nil
	}

	var ids []string

	err := r.db.DB().WithContext(ctx).
		Table("group_members").
		Where("character_id IN ?", characterIDs).
		Distinct().
		Pluck("group_id", &ids).Error
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// AddMember adds the character to the group and records the join event in the
// same transaction, so membership and history can never drift apart.
func (r *Groups) AddMember(
	ctx context.Context, g *models.Group, c *models.Character, entry *models.ActivityEntry,
) error {
	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(g).Association("Members").Append(c); err != nil {
			return err
		}

		return tx.Create(entry).Error
	})
	if err != nil {
		return err
	}

	return nil
}

// RemoveMember removes the character from the group and records the leave
// event in the same transaction.
func (r *Groups) RemoveMember(
	ctx context.Context, g *models.Group, c *models.Character, entry *models.ActivityEntry,
) error {
	err := r.db.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(g).Association("Members").Delete(c); err != nil {
			return err
		}

		return tx.Create(entry).Error
	})
	if err != nil {
		return err
	}

	return nil
}

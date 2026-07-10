package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidGroupType is returned when a group is created or updated with a
// type outside organization/family/other.
var ErrInvalidGroupType = errors.New("invalid group type")

// ErrAlreadyMember is returned when a character joins a group it is already a
// member of.
var ErrAlreadyMember = errors.New("character is already a member")

// ErrNotMember is returned when a character leaves a group it is not a member
// of.
var ErrNotMember = errors.New("character is not a member")

// GroupService manages groups and their membership. Groups themselves are
// campaign structure: their existence and member lists are visible to every
// authenticated user, and only a GM creates or edits them. Group *content*
// (inventory, money, and from M3 the group repository) stays member-only —
// that is enforced where the content is served, not here.
//
// Membership changes are allowed to the character's owner and to GMs, and
// every join/leave is recorded as a game-day-stamped ActivityEntry in the same
// transaction as the membership change (core domain rule 6).
type GroupService struct {
	groups     *repositories.Groups
	characters *CharacterService
}

// NewGroupService builds a GroupService.
func NewGroupService(groups *repositories.Groups, characters *CharacterService) *GroupService {
	return &GroupService{groups: groups, characters: characters}
}

// Create adds a new group. GM only.
func (s *GroupService) Create(
	ctx context.Context, requester Requester, name string, groupType models.GroupType, description string,
) (*models.Group, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if name == "" {
		return nil, ErrInvalidName
	}
	if !groupType.Valid() {
		return nil, ErrInvalidGroupType
	}

	group := &models.Group{Name: name, Type: groupType, Description: description}
	if err := s.groups.Create(ctx, group); err != nil {
		return nil, fmt.Errorf("creating group: %w", err)
	}

	return group, nil
}

// List returns every group. Group existence is not a secret — any
// authenticated user may list them (players need to see groups to join one).
func (s *GroupService) List(ctx context.Context, _ Requester) ([]models.Group, error) {
	groups, err := s.groups.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}

	return groups, nil
}

// Get returns one group with its current members.
func (s *GroupService) Get(ctx context.Context, _ Requester, id string) (*models.Group, error) {
	group, err := s.groups.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading group: %w", err)
	}

	return group, nil
}

// Update changes a group's name, type, and/or description. GM only.
func (s *GroupService) Update(
	ctx context.Context, requester Requester, id string,
	name *string, groupType *models.GroupType, description *string,
) (*models.Group, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	group, err := s.Get(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, ErrInvalidName
		}

		group.Name = *name
	}
	if groupType != nil {
		if !groupType.Valid() {
			return nil, ErrInvalidGroupType
		}

		group.Type = *groupType
	}
	if description != nil {
		group.Description = *description
	}

	if err := s.groups.Update(ctx, group); err != nil {
		return nil, fmt.Errorf("updating group: %w", err)
	}

	return group, nil
}

// Join adds a character to a group. The requester must own the character (or
// be a GM) — a foreign character reads as ErrNotFound, never a confirmation
// it exists. The join is recorded stamped with the character's current game
// day.
func (s *GroupService) Join(ctx context.Context, requester Requester, groupID, characterID string) error {
	group, character, err := s.loadGroupAndCharacter(ctx, requester, groupID, characterID)
	if err != nil {
		return err
	}

	isMember, err := s.groups.IsMember(ctx, groupID, characterID)
	if err != nil {
		return fmt.Errorf("checking membership: %w", err)
	}
	if isMember {
		return ErrAlreadyMember
	}

	entry := membershipEntry(group, character, models.ActivityActionJoined)
	if err := s.groups.AddMember(ctx, group, character, entry); err != nil {
		return fmt.Errorf("joining group: %w", err)
	}

	return nil
}

// Leave removes a character from a group under the same ownership rule as
// Join, recording the leave stamped with the character's current game day.
func (s *GroupService) Leave(ctx context.Context, requester Requester, groupID, characterID string) error {
	group, character, err := s.loadGroupAndCharacter(ctx, requester, groupID, characterID)
	if err != nil {
		return err
	}

	isMember, err := s.groups.IsMember(ctx, groupID, characterID)
	if err != nil {
		return fmt.Errorf("checking membership: %w", err)
	}
	if !isMember {
		return ErrNotMember
	}

	entry := membershipEntry(group, character, models.ActivityActionLeft)
	if err := s.groups.RemoveMember(ctx, group, character, entry); err != nil {
		return fmt.Errorf("leaving group: %w", err)
	}

	return nil
}

// loadGroupAndCharacter resolves both sides of a membership change, applying
// the character-visibility rule (owner or GM, otherwise ErrNotFound).
func (s *GroupService) loadGroupAndCharacter(
	ctx context.Context, requester Requester, groupID, characterID string,
) (*models.Group, *models.Character, error) {
	group, err := s.Get(ctx, requester, groupID)
	if err != nil {
		return nil, nil, err
	}

	character, err := s.characters.Get(ctx, requester, characterID)
	if err != nil {
		return nil, nil, err
	}

	return group, character, nil
}

// membershipEntry builds the activity-log row for a join/leave, stamped with
// the character's current game day at the moment of the change.
func membershipEntry(
	group *models.Group, character *models.Character, action models.ActivityAction,
) *models.ActivityEntry {
	return &models.ActivityEntry{
		GameDay:     character.CurrentGameDay,
		Action:      action,
		EntityType:  "group",
		EntityID:    group.ID,
		EntityName:  group.Name,
		Actor:       character.Name,
		CharacterID: character.ID,
	}
}

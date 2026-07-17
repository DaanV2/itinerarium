package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidGrant is returned when a location grant does not target exactly
// one character or group.
var ErrInvalidGrant = errors.New("grant must target exactly one character or group")

// ErrAlreadyGranted is returned when an identical location grant already
// exists.
var ErrAlreadyGranted = errors.New("access already granted")

// LocationService manages locations and their access grants. Locations have a
// single access level — view + modify — granted per-character or via group
// membership; GMs always see everything. Without a grant a location's
// existence must not leak: lists omit it and direct reads return ErrNotFound
// (core domain rule 3). Anyone who can see a location can edit it (rule 7).
type LocationService struct {
	locations    *repositories.Locations
	accesses     *repositories.LocationAccesses
	groups       *repositories.Groups
	characters   *repositories.Characters
	characterSvc *CharacterService
}

// NewLocationService builds a LocationService.
func NewLocationService(
	locations *repositories.Locations,
	accesses *repositories.LocationAccesses,
	groups *repositories.Groups,
	characters *repositories.Characters,
	characterSvc *CharacterService,
) *LocationService {
	return &LocationService{
		locations:    locations,
		accesses:     accesses,
		groups:       groups,
		characters:   characters,
		characterSvc: characterSvc,
	}
}

// Create adds a new location. GM only.
func (s *LocationService) Create(
	ctx context.Context, requester Requester, name, description, plane string,
) (*models.Location, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if name == "" {
		return nil, ErrInvalidName
	}

	location := &models.Location{Name: name, Description: description, Plane: plane}
	if err := s.locations.Create(ctx, location); err != nil {
		return nil, fmt.Errorf("creating location: %w", err)
	}

	return location, nil
}

// List returns every location for a GM, and only the accessible ones for a
// player (through any of their characters, directly or via groups).
func (s *LocationService) List(ctx context.Context, requester Requester) ([]models.Location, error) {
	if requester.IsGM() {
		locations, err := s.locations.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing locations: %w", err)
		}

		return locations, nil
	}

	characterIDs, groupIDs, err := s.requesterScope(ctx, requester)
	if err != nil {
		return nil, err
	}

	ids, err := s.accesses.AccessibleLocationIDs(ctx, characterIDs, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("resolving accessible locations: %w", err)
	}

	locations, err := s.locations.ListByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("listing locations: %w", err)
	}

	return locations, nil
}

// Get returns a location only if the requester may see it — otherwise
// ErrNotFound, never ErrForbidden.
func (s *LocationService) Get(ctx context.Context, requester Requester, id string) (*models.Location, error) {
	location, err := s.locations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading location: %w", err)
	}

	if requester.IsGM() {
		return location, nil
	}

	characterIDs, groupIDs, err := s.requesterScope(ctx, requester)
	if err != nil {
		return nil, err
	}

	ok, err := s.accesses.HasAccess(ctx, id, characterIDs, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("checking location access: %w", err)
	}
	if !ok {
		return nil, ErrNotFound
	}

	return location, nil
}

// Update edits a location's name, description, and/or plane. Anyone who can
// see the location can edit it.
func (s *LocationService) Update(
	ctx context.Context, requester Requester, id string, name, description, plane *string,
) (*models.Location, error) {
	location, err := s.Get(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, ErrInvalidName
		}

		location.Name = *name
	}
	if description != nil {
		location.Description = *description
	}
	if plane != nil {
		location.Plane = *plane
	}

	if err := s.locations.Update(ctx, location); err != nil {
		return nil, fmt.Errorf("updating location: %w", err)
	}

	return location, nil
}

// GrantAccess gives a character or group (exactly one) access to a location.
// GM only.
func (s *LocationService) GrantAccess(
	ctx context.Context, requester Requester, locationID string, characterID, groupID *string,
) (*models.LocationAccess, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if (characterID == nil) == (groupID == nil) {
		return nil, ErrInvalidGrant
	}

	if _, err := s.locations.GetByID(ctx, locationID); err != nil {
		return nil, notFoundOr(err, "loading location")
	}
	if characterID != nil {
		if _, err := s.characters.GetByID(ctx, *characterID); err != nil {
			return nil, notFoundOr(err, "loading character")
		}
	}
	if groupID != nil {
		if _, err := s.groups.GetByID(ctx, *groupID); err != nil {
			return nil, notFoundOr(err, "loading group")
		}
	}

	grant := &models.LocationAccess{LocationID: locationID, CharacterID: characterID, GroupID: groupID}

	exists, err := s.accesses.Exists(ctx, grant)
	if err != nil {
		return nil, fmt.Errorf("checking existing grant: %w", err)
	}
	if exists {
		return nil, ErrAlreadyGranted
	}

	if err := s.accesses.Create(ctx, grant); err != nil {
		return nil, fmt.Errorf("granting access: %w", err)
	}

	return grant, nil
}

// ListAccess returns every grant on a location. GM only — players never see
// the access list.
func (s *LocationService) ListAccess(
	ctx context.Context, requester Requester, locationID string,
) ([]models.LocationAccess, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	if _, err := s.locations.GetByID(ctx, locationID); err != nil {
		return nil, notFoundOr(err, "loading location")
	}

	accesses, err := s.accesses.ListByLocation(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("listing grants: %w", err)
	}

	return accesses, nil
}

// RevokeAccess removes one grant from a location. GM only.
func (s *LocationService) RevokeAccess(
	ctx context.Context, requester Requester, locationID, accessID string,
) error {
	if !requester.IsGM() {
		return ErrForbidden
	}

	grant, err := s.accesses.GetByID(ctx, accessID)
	if err != nil {
		return notFoundOr(err, "loading grant")
	}
	if grant.LocationID != locationID {
		return ErrNotFound
	}

	if err := s.accesses.Delete(ctx, grant); err != nil {
		return fmt.Errorf("revoking access: %w", err)
	}

	return nil
}

// AssignCharacter associates a character with a location (nil locationID
// clears the association). The requester must own the character or be a GM.
// A player may only place a character at a location that character can see —
// an inaccessible location reads as ErrNotFound so its existence never leaks.
func (s *LocationService) AssignCharacter(
	ctx context.Context, requester Requester, characterID string, locationID *string,
) (*models.Character, error) {
	character, err := s.characterSvc.Get(ctx, requester, characterID)
	if err != nil {
		return nil, err
	}

	if locationID != nil {
		if err := s.checkAssignable(ctx, requester, characterID, *locationID); err != nil {
			return nil, err
		}
	}

	character.LocationID = locationID
	if err := s.characters.Update(ctx, character); err != nil {
		return nil, fmt.Errorf("assigning character location: %w", err)
	}

	return character, nil
}

// checkAssignable confirms the location exists and — for players — that the
// character can see it. Failures read as ErrNotFound so existence never
// leaks.
func (s *LocationService) checkAssignable(
	ctx context.Context, requester Requester, characterID, locationID string,
) error {
	if _, err := s.locations.GetByID(ctx, locationID); err != nil {
		return notFoundOr(err, "loading location")
	}
	if requester.IsGM() {
		return nil
	}

	ok, err := s.AccessibleToCharacter(ctx, characterID, locationID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}

	return nil
}

// FrontierGameDay returns the highest current_game_day among the characters
// that can see the location (through a direct grant or a granted group) — the
// location's "present day". GM-made inventory changes are stamped with it so
// they surface to everyone with access, while characters still catching up
// see them once their own day arrives (M5 activity log).
func (s *LocationService) FrontierGameDay(ctx context.Context, locationID string) (int, error) {
	grants, err := s.accesses.ListByLocation(ctx, locationID)
	if err != nil {
		return 0, fmt.Errorf("listing grants: %w", err)
	}

	day := 0
	for i := range grants {
		grantDay, err := s.grantFrontier(ctx, &grants[i])
		if err != nil {
			return 0, err
		}
		if grantDay > day {
			day = grantDay
		}
	}

	return day, nil
}

// grantFrontier resolves the highest current_game_day reachable through one
// grant. Dangling grants (deleted character or group) count as day 0.
func (s *LocationService) grantFrontier(ctx context.Context, grant *models.LocationAccess) (int, error) {
	if grant.CharacterID != nil {
		character, err := s.characters.GetByID(ctx, *grant.CharacterID)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return 0, nil
			}

			return 0, fmt.Errorf("loading character: %w", err)
		}

		return character.CurrentGameDay, nil
	}
	if grant.GroupID == nil {
		return 0, nil
	}

	group, err := s.groups.GetByID(ctx, *grant.GroupID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return 0, nil
		}

		return 0, fmt.Errorf("loading group: %w", err)
	}

	day := 0
	for i := range group.Members {
		if group.Members[i].CurrentGameDay > day {
			day = group.Members[i].CurrentGameDay
		}
	}

	return day, nil
}

// AccessibleToCharacter reports whether one specific character may see the
// location, directly or through one of its groups. Used when associating a
// character with a location.
func (s *LocationService) AccessibleToCharacter(ctx context.Context, characterID, locationID string) (bool, error) {
	groupIDs, err := s.groups.GroupIDsForCharacters(ctx, []string{characterID})
	if err != nil {
		return false, fmt.Errorf("resolving character groups: %w", err)
	}

	ok, err := s.accesses.HasAccess(ctx, locationID, []string{characterID}, groupIDs)
	if err != nil {
		return false, fmt.Errorf("checking location access: %w", err)
	}

	return ok, nil
}

// requesterScope resolves a player's characters and those characters' groups —
// the two paths a location grant can reach them through.
func (s *LocationService) requesterScope(
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

// notFoundOr maps a repository ErrNotFound to the service-level ErrNotFound
// and wraps anything else with context.
func notFoundOr(err error, doing string) error {
	if errors.Is(err, repositories.ErrNotFound) {
		return ErrNotFound
	}

	return fmt.Errorf("%s: %w", doing, err)
}

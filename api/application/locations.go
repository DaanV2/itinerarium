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

// ErrInvalidLocation is returned when a location description payload is
// malformed — a section reference that doesn't resolve.
var ErrInvalidLocation = errors.New("invalid location")

// LocationSectionInput is one description section in an update payload. ID is
// empty for new sections and references an existing section otherwise.
type LocationSectionInput struct {
	ID      string
	Content string
	GMOnly  bool
}

// LocationView pairs a location (sections already stripped to what the
// requester may see) with whether its description counts as revealed — i.e.
// at least one character with location access has reached SharedOnGameDay.
type LocationView struct {
	Location *models.Location
	Revealed bool
}

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
	ctx context.Context, requester Requester, name, plane string,
) (*LocationView, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if name == "" {
		return nil, ErrInvalidName
	}

	location := &models.Location{Name: name, Plane: plane}
	if err := s.locations.Create(ctx, location); err != nil {
		return nil, fmt.Errorf("creating location: %w", err)
	}

	return s.view(ctx, requester, location)
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

// Get returns a location with the description sections the requester may
// see — otherwise ErrNotFound, never ErrForbidden.
func (s *LocationService) Get(ctx context.Context, requester Requester, id string) (*LocationView, error) {
	location, err := s.getAccessible(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	return s.view(ctx, requester, location)
}

// getAccessible loads a location with its full (unfiltered) sections, only if
// the requester may see it — otherwise ErrNotFound, never ErrForbidden.
func (s *LocationService) getAccessible(
	ctx context.Context, requester Requester, id string,
) (*models.Location, error) {
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

// Update edits a location's name, plane, and/or description. Anyone who can
// see the location can edit it (core domain rule 7); a nil Sections leaves
// the description untouched, only a GM may change SharedOnGameDay, and a
// player's section edits can never touch GM-only rows (core domain rule 2) —
// when every existing section is GM-only, a player's edit lands as new
// player-visible sections alongside them, exactly like documents.
func (s *LocationService) Update(
	ctx context.Context, requester Requester, id string,
	name, plane *string, sharedOnGameDay *int, sections []LocationSectionInput,
) (*LocationView, error) {
	location, err := s.getAccessible(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, ErrInvalidName
		}

		location.Name = *name
	}
	if plane != nil {
		location.Plane = *plane
	}
	if sharedOnGameDay != nil {
		if !requester.IsGM() {
			return nil, fmt.Errorf("%w: only a GM can change the reveal day", ErrForbidden)
		}

		location.SharedOnGameDay = *sharedOnGameDay
	}

	newSections := location.Sections
	if sections != nil {
		newSections, err = mergeLocationSections(requester, location.Sections, sections)
		if err != nil {
			return nil, err
		}
	}

	if err := s.locations.Update(ctx, location, newSections); err != nil {
		return nil, fmt.Errorf("updating location: %w", err)
	}

	location, err = s.locations.GetByID(ctx, location.ID)
	if err != nil {
		return nil, fmt.Errorf("reloading location: %w", err)
	}

	return s.view(ctx, requester, location)
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

// view strips GM-only sections for players and gates the whole description
// block by SharedOnGameDay, resolving the revealed flag along the way — the
// last stop before a location leaves the service layer.
func (s *LocationService) view(
	ctx context.Context, requester Requester, location *models.Location,
) (*LocationView, error) {
	revealed, err := s.revealed(ctx, location)
	if err != nil {
		return nil, err
	}

	if requester.IsGM() {
		return &LocationView{Location: location, Revealed: revealed}, nil
	}

	day, ok, err := s.requesterGameDay(ctx, requester, location.ID)
	if err != nil {
		return nil, err
	}

	visible := make([]models.LocationSection, 0, len(location.Sections))
	if ok && location.SharedOnGameDay <= day {
		for i := range location.Sections {
			if !location.Sections[i].GMOnly {
				visible = append(visible, location.Sections[i])
			}
		}
	}
	location.Sections = visible

	return &LocationView{Location: location, Revealed: revealed}, nil
}

// revealed reports whether any character with access to the location has
// reached SharedOnGameDay.
func (s *LocationService) revealed(ctx context.Context, location *models.Location) (bool, error) {
	frontier, err := s.FrontierGameDay(ctx, location.ID)
	if err != nil {
		return false, err
	}

	return frontier >= location.SharedOnGameDay, nil
}

// requesterGameDay resolves the highest current_game_day among the
// requester's own characters that can reach the location, directly or
// through a granted group. ok is false when none of them can reach it at
// all.
func (s *LocationService) requesterGameDay(
	ctx context.Context, requester Requester, locationID string,
) (day int, ok bool, err error) {
	characters, err := s.characters.ListByUser(ctx, requester.UserID())
	if err != nil {
		return 0, false, fmt.Errorf("listing requester characters: %w", err)
	}

	grants, err := s.accesses.ListByLocation(ctx, locationID)
	if err != nil {
		return 0, false, fmt.Errorf("listing grants: %w", err)
	}

	directChars := make(map[string]bool, len(grants))
	directGroups := make(map[string]bool, len(grants))
	for i := range grants {
		if grants[i].CharacterID != nil {
			directChars[*grants[i].CharacterID] = true
		}
		if grants[i].GroupID != nil {
			directGroups[*grants[i].GroupID] = true
		}
	}

	for i := range characters {
		eligible, err := s.characterEligible(ctx, characters[i].ID, directChars, directGroups)
		if err != nil {
			return 0, false, err
		}
		if eligible && (!ok || characters[i].CurrentGameDay > day) {
			day, ok = characters[i].CurrentGameDay, true
		}
	}

	return day, ok, nil
}

// characterEligible reports whether one character reaches the location
// through a direct grant or a granted group.
func (s *LocationService) characterEligible(
	ctx context.Context, characterID string, directChars, directGroups map[string]bool,
) (bool, error) {
	if directChars[characterID] {
		return true, nil
	}
	if len(directGroups) == 0 {
		return false, nil
	}

	groupIDs, err := s.groups.GroupIDsForCharacters(ctx, []string{characterID})
	if err != nil {
		return false, fmt.Errorf("resolving character groups: %w", err)
	}

	for _, gid := range groupIDs {
		if directGroups[gid] {
			return true, nil
		}
	}

	return false, nil
}

// mergeLocationSections applies a description edit: a GM may freely rebuild
// the section list, while a player's edit keeps GM-only rows exactly where
// they are and can only touch visible ones (core domain rule 7).
func mergeLocationSections(
	requester Requester, existing []models.LocationSection, inputs []LocationSectionInput,
) ([]models.LocationSection, error) {
	byID := make(map[string]models.LocationSection, len(existing))
	for i := range existing {
		byID[existing[i].ID] = existing[i]
	}

	if requester.IsGM() {
		return mergeLocationSectionsGM(byID, inputs)
	}

	return mergeLocationSectionsPlayer(existing, inputs)
}

// mergeLocationSectionsGM rebuilds the section list in the submitted order.
func mergeLocationSectionsGM(
	byID map[string]models.LocationSection, inputs []LocationSectionInput,
) ([]models.LocationSection, error) {
	final := make([]models.LocationSection, 0, len(inputs))
	for _, input := range inputs {
		if input.ID == "" {
			final = append(final, models.LocationSection{GMOnly: input.GMOnly, Content: input.Content})

			continue
		}

		sec, found := byID[input.ID]
		if !found {
			return nil, fmt.Errorf("%w: unknown section %q", ErrInvalidLocation, input.ID)
		}

		sec.Content = input.Content
		sec.GMOnly = input.GMOnly
		final = append(final, sec)
	}

	return final, nil
}

// mergeLocationSectionsPlayer keeps GM-only rows exactly where they are and
// replaces the visible rows with the submitted ones. Visible rows missing
// from the payload are deleted; submitted rows without an ID are appended. A
// section reference that isn't a visible row of this location reads as
// unknown — a stripped GM-only ID is indistinguishable from garbage, so
// nothing leaks.
func mergeLocationSectionsPlayer(
	existing []models.LocationSection, inputs []LocationSectionInput,
) ([]models.LocationSection, error) {
	edits := make([]sectionEdit, len(inputs))
	for i, in := range inputs {
		edits[i] = sectionEdit(in)
	}

	return mergeVisibleSections(
		existing, edits, ErrInvalidLocation,
		func(s models.LocationSection) string { return s.ID },
		func(s models.LocationSection) bool { return s.GMOnly },
		func(s models.LocationSection, content string) models.LocationSection {
			s.Content = content

			return s
		},
		func(content string) models.LocationSection { return models.LocationSection{Content: content} },
	)
}

// notFoundOr maps a repository ErrNotFound to the service-level ErrNotFound
// and wraps anything else with context.
func notFoundOr(err error, doing string) error {
	if errors.Is(err, repositories.ErrNotFound) {
		return ErrNotFound
	}

	return fmt.Errorf("%s: %w", doing, err)
}

package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrUnknownParent is returned when a location is created with a parent_id that
// does not resolve to an existing location.
var ErrUnknownParent = errors.New("unknown parent location")

// ErrLocationHasChildren is returned when deleting a location that still has
// nested locations. The caller must remove or re-home the children first, so a
// plane or place is never deleted out from under the locations inside it.
var ErrLocationHasChildren = errors.New("location has child locations")

// LocationService manages campaign locations — planes, towns, buildings,
// rooms. Locations form a hierarchy: a location with no parent is a top-level
// plane, and nesting one under another models physical containment, which is
// how multi-plane campaigns are supported.
//
// M1 keeps locations name + description + hierarchy and GM-managed: any
// authenticated user may read them (they are campaign-wide, not secret), but
// only a GM may create, edit, or delete. Location inventories (M2) and
// access-controlled, player-editable locations (M3) build on this later.
type LocationService struct {
	locations *repositories.Locations
}

// NewLocationService builds a LocationService.
func NewLocationService(locations *repositories.Locations) *LocationService {
	return &LocationService{locations: locations}
}

// Create adds a location. Only a GM may call this. A nil parentID creates a
// top-level plane; a non-nil parentID must reference an existing location,
// otherwise ErrUnknownParent. Because a parent must already exist at creation
// and re-parenting is not allowed, the hierarchy can never contain a cycle.
func (s *LocationService) Create(
	ctx context.Context, requester Requester, name, description string, parentID *string,
) (*models.Location, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}
	if name == "" {
		return nil, ErrInvalidName
	}

	if parentID != nil {
		if _, err := s.locations.GetByID(ctx, *parentID); err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return nil, ErrUnknownParent
			}

			return nil, fmt.Errorf("looking up parent location: %w", err)
		}
	}

	location := &models.Location{Name: name, Description: description, ParentID: parentID}
	if err := s.locations.Create(ctx, location); err != nil {
		return nil, fmt.Errorf("creating location: %w", err)
	}

	return location, nil
}

// List returns every location, ordered by name. Locations are campaign-wide and
// not secret, so any authenticated caller may read them; the client builds the
// plane/place tree from the parent_id links.
func (s *LocationService) List(ctx context.Context, _ Requester) ([]models.Location, error) {
	locations, err := s.locations.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing locations: %w", err)
	}

	return locations, nil
}

// Get returns a single location. Any authenticated caller may read it;
// ErrNotFound when it does not exist.
func (s *LocationService) Get(ctx context.Context, _ Requester, id string) (*models.Location, error) {
	l, err := s.locations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading location: %w", err)
	}

	return l, nil
}

// Update edits a location's name and/or description. Only a GM may call this.
// Re-parenting is intentionally not supported in M1 (it would allow cycles);
// parent is fixed at creation.
func (s *LocationService) Update(
	ctx context.Context, requester Requester, id string, name, description *string,
) (*models.Location, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	l, err := s.Get(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		if *name == "" {
			return nil, ErrInvalidName
		}

		l.Name = *name
	}

	if description != nil {
		l.Description = *description
	}

	if err := s.locations.Update(ctx, l); err != nil {
		return nil, fmt.Errorf("updating location: %w", err)
	}

	return l, nil
}

// Delete removes a location. Only a GM may call this. A location with nested
// locations cannot be deleted (ErrLocationHasChildren) — remove or re-home the
// children first.
func (s *LocationService) Delete(ctx context.Context, requester Requester, id string) error {
	if !requester.IsGM() {
		return ErrForbidden
	}

	l, err := s.Get(ctx, requester, id)
	if err != nil {
		return err
	}

	children, err := s.locations.CountChildren(ctx, id)
	if err != nil {
		return fmt.Errorf("counting child locations: %w", err)
	}
	if children > 0 {
		return ErrLocationHasChildren
	}

	if err := s.locations.Delete(ctx, l); err != nil {
		return fmt.Errorf("deleting location: %w", err)
	}

	return nil
}

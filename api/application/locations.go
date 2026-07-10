package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// LocationService owns the campaign's locations. In M1 a location is just a
// name, description, and plane (multi-plane support). Locations are readable by
// any authenticated user; only a GM may create or edit them.
//
// M2 adds location inventories and per-character/group access control, at which
// point visibility stops being campaign-wide; M3 opens editing to anyone who
// can see a location. Until then locations are treated like the currency/item
// catalogs: GM-authored, everyone-readable.
type LocationService struct {
	locations *repositories.Locations
}

// NewLocationService builds a LocationService.
func NewLocationService(locations *repositories.Locations) *LocationService {
	return &LocationService{locations: locations}
}

// Create adds a location to the campaign. Only a GM may call this.
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

// List returns every location. Locations are campaign-wide and not secret in
// M1, so any authenticated caller may read them.
func (s *LocationService) List(ctx context.Context) ([]models.Location, error) {
	locations, err := s.locations.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing locations: %w", err)
	}

	return locations, nil
}

// Get returns a single location. Any authenticated caller may read it; an
// unknown ID yields ErrNotFound.
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

// Update edits a location's name, description, and/or plane. Only a GM may call
// this in M1 (M3 opens editing to anyone who can see the location).
func (s *LocationService) Update(
	ctx context.Context, requester Requester, id string, name, description, plane *string,
) (*models.Location, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	l, err := s.locations.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("loading location: %w", err)
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
	if plane != nil {
		l.Plane = *plane
	}

	if err := s.locations.Update(ctx, l); err != nil {
		return nil, fmt.Errorf("updating location: %w", err)
	}

	return l, nil
}

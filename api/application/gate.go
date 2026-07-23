package application

import (
	"context"

	"github.com/DaanV2/itinerarium/api/domain"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

// accessSource abstracts, for a single game-day-gated entity, which characters
// can reach it and when it counts as revealed. It is the one seam the
// visibility gate is parameterised over: documents resolve access from their
// repository's type, locations from their access grants. The game-day maths it
// feeds lives in the domain package; this interface is the application-side seam
// that services implement.
type accessSource interface {
	// charactersWithAccess filters the given characters down to those that can
	// reach the entity through this source.
	charactersWithAccess(ctx context.Context, characters []models.Character) ([]models.Character, error)

	// revealed reports whether any character that can reach the entity — not
	// just the requester's — has reached sharedOnGameDay. A gated entity is
	// "revealed" as soon as someone with access gets there.
	revealed(ctx context.Context, sharedOnGameDay int) (bool, error)
}

// requesterDay resolves how far along the requester is on an entity: it filters
// the requester's characters to those that can reach the entity through source,
// then returns the furthest-along one's game day. ok is false when none of the
// requester's characters can reach it at all. characters is the requester's own
// character list, loaded by the caller.
func requesterDay(
	ctx context.Context, source accessSource, characters []models.Character,
) (day int, ok bool, err error) {
	eligible, err := source.charactersWithAccess(ctx, characters)
	if err != nil {
		return 0, false, err
	}

	day, ok = domain.FurthestGameDay(eligible)

	return day, ok, nil
}

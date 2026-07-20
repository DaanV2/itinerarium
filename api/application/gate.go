package application

import (
	"context"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

// accessSource abstracts, for a single game-day-gated entity, which characters
// can reach it and when it counts as revealed. It is the one seam the
// visibility gate is parameterised over: documents resolve access from their
// repository's type, locations from their access grants. Concentrating the
// security-critical game-day maths here — rather than re-deriving "the
// furthest-along character that can reach this thing" in each service — keeps
// the gating in a single audited place (roadmap M7).
type accessSource interface {
	// charactersWithAccess filters the given characters down to those that can
	// reach the entity through this source.
	charactersWithAccess(ctx context.Context, characters []models.Character) ([]models.Character, error)

	// revealed reports whether any character that can reach the entity — not
	// just the requester's — has reached sharedOnGameDay. A gated entity is
	// "revealed" as soon as someone with access gets there.
	revealed(ctx context.Context, sharedOnGameDay int) (bool, error)
}

// furthestCharacter returns the character with the highest current_game_day, or
// nil when the slice is empty. The furthest-along character is the one whose
// game day gates what a set of characters can currently see.
func furthestCharacter(characters []models.Character) *models.Character {
	var best *models.Character
	for i := range characters {
		if best == nil || characters[i].CurrentGameDay > best.CurrentGameDay {
			best = &characters[i]
		}
	}

	return best
}

// furthestGameDay returns the highest current_game_day among the characters and
// whether there were any — the "present day" of that set.
func furthestGameDay(characters []models.Character) (day int, ok bool) {
	if best := furthestCharacter(characters); best != nil {
		return best.CurrentGameDay, true
	}

	return 0, false
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

	day, ok = furthestGameDay(eligible)

	return day, ok, nil
}

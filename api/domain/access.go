// Package domain holds the security-critical rules shared by services but
// independent of storage or transport: game-day "furthest character" maths and
// the GM-only-preserving section merge (core domain rule 7). Keeping them here,
// rather than re-deriving them per service, keeps the gating in one audited
// place (roadmap M7).
package domain

import (
	"fmt"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

// FurthestCharacter returns the character with the highest current_game_day, or
// nil when the slice is empty. The furthest-along character is the one whose
// game day gates what a set of characters can currently see.
func FurthestCharacter(characters []models.Character) *models.Character {
	var best *models.Character
	for i := range characters {
		if best == nil || characters[i].CurrentGameDay > best.CurrentGameDay {
			best = &characters[i]
		}
	}

	return best
}

// FurthestGameDay returns the highest current_game_day among the characters and
// whether there were any — the "present day" of that set.
func FurthestGameDay(characters []models.Character) (day int, ok bool) {
	if best := FurthestCharacter(characters); best != nil {
		return best.CurrentGameDay, true
	}

	return 0, false
}

// SectionEdit is the common shape of a section edit in an update payload:
// content plus whether it should be GM-only. ID is empty for new sections and
// references an existing section otherwise. Document and location section
// inputs share this shape so the "anyone can edit, GM-only stays hidden" merge
// rule (core domain rule 7) is implemented once.
type SectionEdit struct {
	ID      string
	Content string
	GMOnly  bool
}

// MergeVisibleSections applies a player's section edit, shared by documents and
// locations: GM-only rows are kept exactly where they are, visible rows are
// replaced or removed by ID, and rows without an ID are appended as new visible
// content. unknownSection is wrapped and returned when a submitted ID isn't a
// visible row — a stripped GM-only ID is indistinguishable from garbage, so
// nothing leaks. gmOnlyForbidden is returned when a player tries to mark a
// section GM-only; callers supply their forbidden sentinel so the rule stays
// storage- and transport-agnostic.
func MergeVisibleSections[T any](
	existing []T, inputs []SectionEdit, unknownSection, gmOnlyForbidden error,
	id func(T) string, gmOnly func(T) bool, withContent func(T, string) T, newSection func(string) T,
) ([]T, error) {
	visibleByID := make(map[string]struct{}, len(existing))
	for _, sec := range existing {
		if !gmOnly(sec) {
			visibleByID[id(sec)] = struct{}{}
		}
	}

	submitted := make(map[string]SectionEdit, len(inputs))

	var appended []SectionEdit

	for _, input := range inputs {
		if input.GMOnly {
			return nil, fmt.Errorf("%w: only a GM can mark sections GM-only", gmOnlyForbidden)
		}
		if input.ID == "" {
			appended = append(appended, input)

			continue
		}
		if _, visible := visibleByID[input.ID]; !visible {
			return nil, fmt.Errorf("%w: unknown section %q", unknownSection, input.ID)
		}

		submitted[input.ID] = input
	}

	final := make([]T, 0, len(existing)+len(appended))
	for _, sec := range existing {
		if gmOnly(sec) {
			final = append(final, sec)

			continue
		}
		if input, kept := submitted[id(sec)]; kept {
			final = append(final, withContent(sec, input.Content))
		}
	}

	for _, input := range appended {
		final = append(final, newSection(input.Content))
	}

	return final, nil
}

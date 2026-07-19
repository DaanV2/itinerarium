package application

import "fmt"

// sectionEdit is the common shape of a section edit in an update payload:
// content plus whether it should be GM-only. ID is empty for new sections and
// references an existing section otherwise. Document and location section
// inputs share this shape so the "anyone can edit, GM-only stays hidden"
// merge rule (core domain rule 7) is implemented once.
type sectionEdit struct {
	ID      string
	Content string
	GMOnly  bool
}

// mergeVisibleSections applies a player's section edit, shared by documents
// and locations: GM-only rows are kept exactly where they are, visible rows
// are replaced or removed by ID, and rows without an ID are appended as new
// visible content. unknownSection is wrapped and returned when a submitted ID
// isn't a visible row — a stripped GM-only ID is indistinguishable from
// garbage, so nothing leaks.
func mergeVisibleSections[T any](
	existing []T, inputs []sectionEdit, unknownSection error,
	id func(T) string, gmOnly func(T) bool, withContent func(T, string) T, newSection func(string) T,
) ([]T, error) {
	visibleByID := make(map[string]struct{}, len(existing))
	for _, sec := range existing {
		if !gmOnly(sec) {
			visibleByID[id(sec)] = struct{}{}
		}
	}

	submitted := make(map[string]sectionEdit, len(inputs))

	var appended []sectionEdit

	for _, input := range inputs {
		if input.GMOnly {
			return nil, fmt.Errorf("%w: only a GM can mark sections GM-only", ErrForbidden)
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

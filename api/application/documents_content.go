package application

import (
	"fmt"
	"strings"

	"github.com/DaanV2/itinerarium/api/domain"
	"github.com/DaanV2/itinerarium/api/domain/documentfmt"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

// buildDocument turns a create payload into a Document, resolving raw
// markdown (frontmatter included) or structured sections and enforcing that
// only GMs mark sections GM-only.
func buildDocument(requester Requester, repositoryID string, input *CreateDocumentInput) (*models.Document, error) {
	title, tags, sharedOn, sections, err := resolveCreateContent(input)
	if err != nil {
		return nil, err
	}

	path, err := normalizePath(input.Path)
	if err != nil {
		return nil, err
	}

	modelSections := make([]models.DocumentSection, len(sections))
	for i, sec := range sections {
		if sec.ID != "" {
			return nil, fmt.Errorf("%w: new documents cannot reference existing sections", ErrInvalidDocument)
		}
		if sec.GMOnly && !requester.IsGM() {
			return nil, fmt.Errorf("%w: only a GM can mark sections GM-only", ErrForbidden)
		}

		modelSections[i] = models.DocumentSection{Position: i, GMOnly: sec.GMOnly, Content: sec.Content}
	}

	return &models.Document{
		RepositoryID:    repositoryID,
		Path:            path,
		Title:           titleOrFallback(title, path),
		Tags:            tags,
		SharedOnGameDay: sharedOn,
		Version:         1,
		Sections:        modelSections,
	}, nil
}

// resolveCreateContent resolves a create payload's metadata and sections,
// letting explicit fields win over frontmatter values when raw markdown is
// given.
func resolveCreateContent(
	input *CreateDocumentInput,
) (title string, tags []string, sharedOn int, sections []DocumentSectionInput, err error) {
	title, tags = input.Title, input.Tags
	if input.SharedOnGameDay != nil {
		sharedOn = *input.SharedOnGameDay
	}

	if input.Markdown == "" {
		return title, tags, sharedOn, input.Sections, nil
	}
	if len(input.Sections) > 0 {
		return "", nil, 0, nil, fmt.Errorf("%w: give either markdown or sections, not both", ErrInvalidDocument)
	}

	meta, body, err := documentfmt.Parse(input.Markdown)
	if err != nil {
		return "", nil, 0, nil, fmt.Errorf("%w: %w", ErrInvalidDocument, err)
	}

	if title == "" {
		title = meta.Title
	}
	if len(tags) == 0 {
		tags = meta.Tags
	}
	if input.SharedOnGameDay == nil && meta.GameDay != nil {
		sharedOn = *meta.GameDay
	}
	if body != "" {
		sections = []DocumentSectionInput{{Content: body}}
	}

	return title, tags, sharedOn, sections, nil
}

// mergeSections computes a document's final section list from an update.
// GMs replace the whole list (order, flags, deletions included). Players
// only replace the player-visible sections: GM-only rows stay untouched in
// place, submitted sections must reference visible rows, and anything new
// lands as player-visible sections at the end — which is exactly how an edit
// on an all-GM-only document becomes a new player-visible section.
func mergeSections(
	requester Requester, existing []models.DocumentSection, inputs []DocumentSectionInput,
) ([]models.DocumentSection, error) {
	byID := make(map[string]models.DocumentSection, len(existing))
	for i := range existing {
		byID[existing[i].ID] = existing[i]
	}

	if requester.IsGM() {
		return mergeSectionsGM(byID, inputs)
	}

	return mergeSectionsPlayer(existing, inputs)
}

// mergeSectionsGM rebuilds the section list in the submitted order.
func mergeSectionsGM(
	byID map[string]models.DocumentSection, inputs []DocumentSectionInput,
) ([]models.DocumentSection, error) {
	final := make([]models.DocumentSection, 0, len(inputs))
	for _, input := range inputs {
		if input.ID == "" {
			final = append(final, models.DocumentSection{GMOnly: input.GMOnly, Content: input.Content})

			continue
		}

		sec, found := byID[input.ID]
		if !found {
			return nil, fmt.Errorf("%w: unknown section %q", ErrInvalidDocument, input.ID)
		}

		sec.Content = input.Content
		sec.GMOnly = input.GMOnly
		final = append(final, sec)
	}

	return final, nil
}

// mergeSectionsPlayer keeps GM-only rows exactly where they are and replaces
// the visible rows with the submitted ones. Visible rows missing from the
// payload are deleted; submitted rows without an ID are appended. A section
// reference that isn't a visible row of this document reads as unknown — a
// stripped GM-only ID is indistinguishable from garbage, so nothing leaks.
func mergeSectionsPlayer(
	existing []models.DocumentSection, inputs []DocumentSectionInput,
) ([]models.DocumentSection, error) {
	edits := make([]domain.SectionEdit, len(inputs))
	for i, in := range inputs {
		edits[i] = domain.SectionEdit(in)
	}

	return domain.MergeVisibleSections(
		existing, edits, ErrInvalidDocument, ErrForbidden,
		func(s models.DocumentSection) string { return s.ID },
		func(s models.DocumentSection) bool { return s.GMOnly },
		func(s models.DocumentSection, content string) models.DocumentSection {
			s.Content = content

			return s
		},
		func(content string) models.DocumentSection { return models.DocumentSection{Content: content} },
	)
}

// normalizePath cleans a document path into slash-separated non-empty
// segments, rejecting traversal and blank paths.
func normalizePath(path string) (string, error) {
	segments := strings.Split(path, "/")
	cleaned := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if segment == "." || segment == ".." {
			return "", fmt.Errorf("%w: path cannot contain %q", ErrInvalidDocument, segment)
		}

		cleaned = append(cleaned, segment)
	}

	if len(cleaned) == 0 {
		return "", fmt.Errorf("%w: path is required", ErrInvalidDocument)
	}

	return strings.Join(cleaned, "/"), nil
}

// titleOrFallback returns the given title, falling back to the path's last
// segment (the file name) when it's blank.
func titleOrFallback(title, path string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}

	segments := strings.Split(path, "/")

	return segments[len(segments)-1]
}

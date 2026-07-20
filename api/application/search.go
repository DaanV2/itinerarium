package application

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidQuery is returned when a search request carries no query text.
var ErrInvalidQuery = serviceErr(KindValidation, "search query is required")

// Search match fields — which part of a document the query text was found in.
const (
	SearchMatchTitle   = "title"
	SearchMatchPath    = "path"
	SearchMatchTags    = "tags"
	SearchMatchContent = "content"
)

// searchSnippetBefore/After bound the content snippet around the first match.
const (
	searchSnippetBefore = 40
	searchSnippetAfter  = 120
)

// SearchResult is one document matching a full-text search, with the sections
// already stripped to what the requester may see. MatchedIn lists which
// fields contained the query; Snippet is a short excerpt around the first
// content match (empty when only metadata matched).
type SearchResult struct {
	Document  *models.Document
	MatchedIn []string
	Snippet   string
}

// Search runs a full-text search over document titles, paths (file names),
// tags, and section content (roadmap M6). Access filtering happens before
// anything is returned: players only match documents they can currently see —
// visible repository plus game day reached, or a reached direct share — and
// GM-only sections are excluded from both matching and the returned sections,
// so an inaccessible document never surfaces, not even as a hit count. GMs
// search across everything, GM-only content included.
func (s *DocumentService) Search(ctx context.Context, requester Requester, query string) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrInvalidQuery
	}

	if requester.IsGM() {
		docs, err := s.documents.Search(ctx, query, nil, true)
		if err != nil {
			return nil, fmt.Errorf("searching documents: %w", err)
		}

		return buildSearchResults(docs, query), nil
	}

	scope, gate, err := s.searchScope(ctx, requester)
	if err != nil {
		return nil, err
	}

	docs, err := s.documents.Search(ctx, query, scope, false)
	if err != nil {
		return nil, fmt.Errorf("searching documents: %w", err)
	}

	visible := make([]models.Document, 0, len(docs))
	for i := range docs {
		if !gate.visible(&docs[i]) {
			continue
		}

		stripped := make([]models.DocumentSection, 0, len(docs[i].Sections))
		for j := range docs[i].Sections {
			if !docs[i].Sections[j].GMOnly {
				stripped = append(stripped, docs[i].Sections[j])
			}
		}
		docs[i].Sections = stripped

		visible = append(visible, docs[i])
	}

	return buildSearchResults(visible, query), nil
}

// searchGate holds the visibility facts a player's search results are checked
// against: the effective game day per accessible repository and the set of
// documents unlocked through a reached direct share.
type searchGate struct {
	dayByRepo    map[string]int
	sharedDocIDs map[string]struct{}
}

// visible reports whether the player may see the document right now.
func (g *searchGate) visible(doc *models.Document) bool {
	if day, ok := g.dayByRepo[doc.RepositoryID]; ok && doc.SharedOnGameDay <= day {
		return true
	}

	_, shared := g.sharedDocIDs[doc.ID]

	return shared
}

// searchScope resolves which repositories and directly shared documents a
// player's search may touch, along with the game-day gate to apply on top.
func (s *DocumentService) searchScope(
	ctx context.Context, requester Requester,
) (*repositories.DocumentSearchScope, *searchGate, error) {
	characters, err := requesterCharacters(ctx, s.characters, requester)
	if err != nil {
		return nil, nil, fmt.Errorf("listing requester characters: %w", err)
	}

	repos, err := s.repositories.List(ctx, requester)
	if err != nil {
		return nil, nil, err
	}

	gate := &searchGate{dayByRepo: map[string]int{}, sharedDocIDs: map[string]struct{}{}}

	repoIDs := make([]string, 0, len(repos))
	for i := range repos {
		eligible, err := s.charactersWithRepoAccess(ctx, &repos[i], characters)
		if err != nil {
			return nil, nil, err
		}

		day, ok := furthestGameDay(eligible)
		if !ok {
			continue
		}

		gate.dayByRepo[repos[i].ID] = day
		repoIDs = append(repoIDs, repos[i].ID)
	}

	characterIDs := make([]string, len(characters))
	dayByCharacter := make(map[string]int, len(characters))
	for i := range characters {
		characterIDs[i] = characters[i].ID
		dayByCharacter[characters[i].ID] = characters[i].CurrentGameDay
	}

	shares, err := s.shares.ListByCharacters(ctx, characterIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("listing direct shares: %w", err)
	}

	docIDs := make([]string, 0, len(shares))
	for i := range shares {
		if dayByCharacter[shares[i].CharacterID] < shares[i].SharedOnGameDay {
			continue
		}
		if _, ok := gate.sharedDocIDs[shares[i].DocumentID]; ok {
			continue
		}

		gate.sharedDocIDs[shares[i].DocumentID] = struct{}{}
		docIDs = append(docIDs, shares[i].DocumentID)
	}

	return &repositories.DocumentSearchScope{RepositoryIDs: repoIDs, DocumentIDs: docIDs}, gate, nil
}

// buildSearchResults recomputes per-field matches and a content snippet from
// the (already stripped) documents. A document whose visible fields no longer
// contain the query — which the database scope should have prevented — is
// dropped rather than leaked.
func buildSearchResults(docs []models.Document, query string) []SearchResult {
	results := make([]SearchResult, 0, len(docs))
	for i := range docs {
		result := searchResultFor(&docs[i], query)
		if result == nil {
			continue
		}

		results = append(results, *result)
	}

	return results
}

// searchResultFor computes which fields of one document match the query, and
// the snippet around the first content match. It returns nil when nothing
// visible matches.
func searchResultFor(doc *models.Document, query string) *SearchResult {
	result := &SearchResult{Document: doc}

	if foldIndex(doc.Title, query) >= 0 {
		result.MatchedIn = append(result.MatchedIn, SearchMatchTitle)
	}
	if foldIndex(doc.Path, query) >= 0 {
		result.MatchedIn = append(result.MatchedIn, SearchMatchPath)
	}
	for _, tag := range doc.Tags {
		if foldIndex(tag, query) >= 0 {
			result.MatchedIn = append(result.MatchedIn, SearchMatchTags)

			break
		}
	}

	for i := range doc.Sections {
		idx := foldIndex(doc.Sections[i].Content, query)
		if idx < 0 {
			continue
		}

		result.MatchedIn = append(result.MatchedIn, SearchMatchContent)
		result.Snippet = snippetAround(doc.Sections[i].Content, idx, query)

		break
	}

	if len(result.MatchedIn) == 0 {
		return nil
	}

	return result
}

// foldIndex returns the rune index of the first case-insensitive occurrence
// of substr in s, or -1. Rune-based so snippet offsets never split UTF-8.
func foldIndex(s, substr string) int {
	if substr == "" {
		return 0
	}

	runes := []rune(s)
	sub := []rune(substr)
	for i := range sub {
		sub[i] = unicode.ToLower(sub[i])
	}
	for i := range runes {
		runes[i] = unicode.ToLower(runes[i])
	}

	for i := 0; i+len(sub) <= len(runes); i++ {
		match := true
		for j := range sub {
			if runes[i+j] != sub[j] {
				match = false

				break
			}
		}
		if match {
			return i
		}
	}

	return -1
}

// snippetAround extracts a short excerpt of content around the match at rune
// index idx, with ellipses marking truncation.
func snippetAround(content string, idx int, query string) string {
	runes := []rune(content)
	queryLen := len([]rune(query))

	start := idx - searchSnippetBefore
	if start < 0 {
		start = 0
	}

	end := idx + queryLen + searchSnippetAfter
	if end > len(runes) {
		end = len(runes)
	}

	snippet := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}

	return snippet
}

package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/google/uuid"
)

// ErrPathCollision is returned when a document is created at (or moved to) a
// path that already exists in the same repository and the caller didn't set
// AllowCollision. A warning, not a block: resubmitting with AllowCollision
// proceeds (core domain rule 7).
var ErrPathCollision = errors.New("a document already exists at this path")

// ErrConcurrentEdit is returned when a document changed since the editor
// loaded it and the caller didn't set Force. A warning, not a block:
// resubmitting with Force overwrites (core domain rule 7).
var ErrConcurrentEdit = errors.New("the document changed since it was loaded")

// ErrInvalidDocument is returned when a document payload is malformed: empty
// or invalid path, a section reference that doesn't resolve, or both raw
// markdown and structured sections in one request.
var ErrInvalidDocument = errors.New("invalid document")

// ErrAlreadyShared is returned when a document is already directly shared
// with the given character.
var ErrAlreadyShared = errors.New("document already shared with this character")

// DocumentSectionInput is one section in a create/update payload. ID is empty
// for new sections and references an existing section otherwise.
type DocumentSectionInput struct {
	ID      string
	Content string
	GMOnly  bool
}

// CreateDocumentInput carries a new document. Either Sections or Markdown
// (raw markdown with optional YAML frontmatter for title/tags/game_day) is
// given, never both. SharedOnGameDay nil means: frontmatter value, or 0.
type CreateDocumentInput struct {
	Path            string
	Title           string
	Tags            []string
	SharedOnGameDay *int
	Sections        []DocumentSectionInput
	Markdown        string
	AllowCollision  bool
}

// UpdateDocumentInput carries a full replacement of a document's metadata and
// its caller-visible sections. ExpectedVersion is the document version the
// editor loaded; when it no longer matches, the update fails with
// ErrConcurrentEdit unless Force is set (nil skips the check).
type UpdateDocumentInput struct {
	Path            string
	Title           string
	Tags            []string
	SharedOnGameDay int
	Sections        []DocumentSectionInput
	ExpectedVersion *int
	Force           bool
	AllowCollision  bool
}

// ShareDocumentInput carries a share-to-group request: move a document
// currently in a character's private repository into a group repository,
// revealed to members from SharedOnGameDay onward.
type ShareDocumentInput struct {
	TargetRepositoryID string
	SharedOnGameDay    int
	AllowCollision     bool
}

// DocumentView pairs a document (sections already stripped to what the
// requester may see) with whether it counts as revealed — i.e. at least one
// character with repository access has reached its SharedOnGameDay. The
// editor shows a warning on revealed documents: edits are immediately
// visible, there is no versioning.
type DocumentView struct {
	Document *models.Document
	Revealed bool
}

// DocumentService manages knowledge documents. It owns every document
// permission rule: repository access, per-character game-day gating,
// server-side GM-only stripping, existence hiding (404 never 403), open
// editing, and the path-collision / concurrent-edit warnings.
type DocumentService struct {
	documents    *repositories.Documents
	repositories *RepositoryService
	characters   *repositories.Characters
	groups       *repositories.Groups
	shares       *repositories.DocumentShares
}

// NewDocumentService builds a DocumentService.
func NewDocumentService(
	documents *repositories.Documents,
	repos *RepositoryService,
	characters *repositories.Characters,
	groups *repositories.Groups,
	shares *repositories.DocumentShares,
) *DocumentService {
	return &DocumentService{
		documents: documents, repositories: repos, characters: characters, groups: groups, shares: shares,
	}
}

// ListByRepository returns the documents in a repository that the requester
// may see: all of them for a GM, and for players only those whose
// SharedOnGameDay has been reached by one of their characters with access to
// the repository. An inaccessible repository reads as ErrNotFound.
func (s *DocumentService) ListByRepository(
	ctx context.Context, requester Requester, repositoryID string,
) ([]models.Document, error) {
	repo, err := s.repositories.Get(ctx, requester, repositoryID)
	if err != nil {
		return nil, err
	}

	docs, err := s.documents.ListByRepository(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}

	if requester.IsGM() {
		return docs, nil
	}

	day, ok, err := s.effectiveGameDay(ctx, requester, repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []models.Document{}, nil
	}

	visible := make([]models.Document, 0, len(docs))
	for i := range docs {
		if docs[i].SharedOnGameDay <= day {
			visible = append(visible, docs[i])
		}
	}

	return visible, nil
}

// FolderNode is one folder (or the repository root) in a document folder
// tree. Folders and Documents are each sorted alphabetically by name/title.
type FolderNode struct {
	Name      string
	Path      string
	Folders   []*FolderNode
	Documents []models.Document
}

// FolderTree arranges the repository's documents the requester may see into
// a folder tree by path. It is built purely from ListByRepository's result,
// so a folder appears only when it (directly or through a subfolder)
// contains at least one document the requester may see — hidden means
// invisible for folders too (core domain rule 3, roadmap M3).
func (s *DocumentService) FolderTree(
	ctx context.Context, requester Requester, repositoryID string,
) (*FolderNode, error) {
	docs, err := s.ListByRepository(ctx, requester, repositoryID)
	if err != nil {
		return nil, err
	}

	return buildFolderTree(docs), nil
}

// buildFolderTree groups documents into nested folders by path segment and
// sorts every level alphabetically.
func buildFolderTree(docs []models.Document) *FolderNode {
	root := &FolderNode{}
	folders := make(map[string]*FolderNode)

	for i := range docs {
		doc := docs[i]
		segments := strings.Split(doc.Path, "/")

		node := root
		path := ""
		for _, seg := range segments[:len(segments)-1] {
			if path == "" {
				path = seg
			} else {
				path += "/" + seg
			}

			child, ok := folders[path]
			if !ok {
				child = &FolderNode{Name: seg, Path: path}
				folders[path] = child
				node.Folders = append(node.Folders, child)
			}
			node = child
		}

		node.Documents = append(node.Documents, doc)
	}

	sortFolderTree(root)

	return root
}

// sortFolderTree sorts a folder's subfolders by name and documents by title,
// recursively.
func sortFolderTree(node *FolderNode) {
	sort.Slice(node.Folders, func(i, j int) bool { return node.Folders[i].Name < node.Folders[j].Name })
	sort.Slice(node.Documents, func(i, j int) bool { return node.Documents[i].Title < node.Documents[j].Title })

	for _, f := range node.Folders {
		sortFolderTree(f)
	}
}

// Get returns one document with the sections the requester may see. Players
// get ErrNotFound when the repository is out of reach or no character of
// theirs has reached SharedOnGameDay; GM-only sections are stripped before
// the document leaves this method.
func (s *DocumentService) Get(ctx context.Context, requester Requester, id string) (*DocumentView, error) {
	doc, repo, err := s.getAccessible(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	return s.view(ctx, requester, repo, doc)
}

// Create adds a document to a repository. Anyone who can see the repository
// can create documents in it; only GMs may mark sections GM-only. A path
// that already exists in the repository fails with ErrPathCollision unless
// AllowCollision is set.
func (s *DocumentService) Create(
	ctx context.Context, requester Requester, repositoryID string, input *CreateDocumentInput,
) (*DocumentView, error) {
	repo, err := s.repositories.Get(ctx, requester, repositoryID)
	if err != nil {
		return nil, err
	}

	doc, err := buildDocument(requester, repo.ID, input)
	if err != nil {
		return nil, err
	}

	if !input.AllowCollision {
		exists, err := s.documents.ExistsAtPath(ctx, repo.ID, doc.Path, "")
		if err != nil {
			return nil, fmt.Errorf("checking path collision: %w", err)
		}
		if exists {
			return nil, ErrPathCollision
		}
	}

	// Pre-assign the ID so the activity entry can reference the new document.
	doc.ID = uuid.NewString()

	entry, err := s.documentEntry(ctx, requester, repo, doc, models.ActivityActionAdded)
	if err != nil {
		return nil, err
	}
	if err := s.documents.Create(ctx, doc, entry); err != nil {
		return nil, fmt.Errorf("creating document: %w", err)
	}

	return s.view(ctx, requester, repo, doc)
}

// Update replaces a document's metadata and sections. Anyone who can see the
// document can edit it (core domain rule 7); player edits can never touch
// GM-only sections or the reveal day, and when every existing section is
// GM-only a player's edit lands as new player-visible sections alongside
// them.
func (s *DocumentService) Update(
	ctx context.Context, requester Requester, id string, input *UpdateDocumentInput,
) (*DocumentView, error) {
	doc, repo, err := s.getAccessible(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if input.ExpectedVersion != nil && !input.Force && *input.ExpectedVersion != doc.Version {
		return nil, ErrConcurrentEdit
	}

	if err := s.applyMetadata(ctx, requester, doc, input); err != nil {
		return nil, err
	}

	sections, err := mergeSections(requester, doc.Sections, input.Sections)
	if err != nil {
		return nil, err
	}

	doc.Version++

	entry, err := s.documentEntry(ctx, requester, repo, doc, models.ActivityActionUpdated)
	if err != nil {
		return nil, err
	}
	if err := s.documents.Update(ctx, doc, sections, entry); err != nil {
		return nil, fmt.Errorf("updating document: %w", err)
	}

	// Reload so the response reflects exactly what the database stored.
	doc, err = s.documents.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("reloading document: %w", err)
	}

	return s.view(ctx, requester, repo, doc)
}

// ShareToGroup moves a document out of a character's private repository into
// a group's repository at a chosen SharedOnGameDay (core domain rule 5's
// counterpart for groups: sharing is a move, not a copy — the document
// leaves the character repository for good and normal group-repository
// rules apply from then on). Only documents currently in a character
// repository can be shared this way. The caller needs the usual document
// access to the source (owner + GM, enforced by getAccessible) and needs to
// be able to see the target group repository too (a player only via one of
// their characters' membership; a GM always); either check failing reads as
// ErrNotFound so membership never leaks. A path already occupied in the
// target repository warns with ErrPathCollision unless AllowCollision is
// set.
func (s *DocumentService) ShareToGroup(
	ctx context.Context, requester Requester, id string, input *ShareDocumentInput,
) (*DocumentView, error) {
	doc, repo, err := s.getAccessible(ctx, requester, id)
	if err != nil {
		return nil, err
	}

	if repo.Type != models.RepositoryTypeCharacter {
		return nil, fmt.Errorf("%w: only a document in a character repository can be shared to a group", ErrInvalidDocument)
	}

	target, err := s.repositories.Get(ctx, requester, input.TargetRepositoryID)
	if err != nil {
		return nil, err
	}
	if target.Type != models.RepositoryTypeGroup {
		return nil, fmt.Errorf("%w: documents can only be shared to a group repository", ErrInvalidDocument)
	}

	if !input.AllowCollision {
		exists, err := s.documents.ExistsAtPath(ctx, target.ID, doc.Path, doc.ID)
		if err != nil {
			return nil, fmt.Errorf("checking path collision: %w", err)
		}
		if exists {
			return nil, ErrPathCollision
		}
	}

	doc.RepositoryID = target.ID
	doc.SharedOnGameDay = input.SharedOnGameDay
	doc.Version++

	// Sharing lands the document in the group's repository — to members that
	// is a new document, so the event is recorded as an addition there.
	entry, err := s.documentEntry(ctx, requester, target, doc, models.ActivityActionAdded)
	if err != nil {
		return nil, err
	}
	if err := s.documents.Update(ctx, doc, doc.Sections, entry); err != nil {
		return nil, fmt.Errorf("sharing document: %w", err)
	}

	doc, err = s.documents.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("reloading document: %w", err)
	}

	return s.view(ctx, requester, target, doc)
}

// documentEntry builds the activity-log row for a document change (roadmap
// M5). The entry is scoped to the repository that gates who may see the
// document and stamped with the document's reveal day, so it surfaces in a
// character's feed exactly when the document itself does — an entry about an
// unrevealed document never leaks its existence (core domain rule 3). The
// actor is "GM" for GM changes, otherwise the requester's furthest-along
// character with access to the repository.
func (s *DocumentService) documentEntry(
	ctx context.Context, requester Requester, repo *models.Repository, doc *models.Document,
	action models.ActivityAction,
) (*models.ActivityEntry, error) {
	actor := activityActorGM
	if !requester.IsGM() {
		characters, err := s.characters.ListByUser(ctx, requester.UserID())
		if err != nil {
			return nil, fmt.Errorf("listing requester characters: %w", err)
		}

		eligible, err := s.charactersWithRepoAccess(ctx, repo, characters)
		if err != nil {
			return nil, err
		}

		actor = ""
		var best *models.Character
		for i := range eligible {
			if best == nil || eligible[i].CurrentGameDay > best.CurrentGameDay {
				best = &eligible[i]
			}
		}
		if best != nil {
			actor = best.Name
		}
	}

	return &models.ActivityEntry{
		GameDay:    doc.SharedOnGameDay,
		Action:     action,
		EntityType: "document",
		EntityID:   doc.ID,
		EntityName: doc.Title,
		Actor:      actor,
		ScopeType:  models.ActivityScopeRepository,
		ScopeID:    repo.ID,
	}, nil
}

// getAccessible loads a document and enforces every read rule: repository
// access and, for players, the game-day gate — or, failing that, a direct
// share to one of the requester's characters that has reached its own game
// day (core domain rule 1, roadmap M3). Anything out of reach reads as
// ErrNotFound so existence never leaks.
func (s *DocumentService) getAccessible(
	ctx context.Context, requester Requester, id string,
) (*models.Document, *models.Repository, error) {
	doc, err := s.documents.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, nil, ErrNotFound
		}

		return nil, nil, fmt.Errorf("loading document: %w", err)
	}

	repo, err := s.repositories.Get(ctx, requester, doc.RepositoryID)
	if err != nil {
		if requester.IsGM() || !errors.Is(err, ErrNotFound) {
			return nil, nil, err
		}

		return s.getViaDirectShare(ctx, requester, doc)
	}

	if requester.IsGM() {
		return doc, repo, nil
	}

	day, ok, err := s.effectiveGameDay(ctx, requester, repo)
	if err != nil {
		return nil, nil, err
	}
	if ok && doc.SharedOnGameDay <= day {
		return doc, repo, nil
	}

	return s.getViaDirectShare(ctx, requester, doc)
}

// getViaDirectShare checks whether any of the requester's characters holds a
// direct share on doc that has reached its own SharedOnGameDay — the
// fallback path when the document's repository doesn't (yet) grant access.
func (s *DocumentService) getViaDirectShare(
	ctx context.Context, requester Requester, doc *models.Document,
) (*models.Document, *models.Repository, error) {
	characters, err := s.characters.ListByUser(ctx, requester.UserID())
	if err != nil {
		return nil, nil, fmt.Errorf("listing requester characters: %w", err)
	}

	characterIDs := make([]string, len(characters))
	dayByCharacter := make(map[string]int, len(characters))
	for i := range characters {
		characterIDs[i] = characters[i].ID
		dayByCharacter[characters[i].ID] = characters[i].CurrentGameDay
	}

	shares, err := s.shares.ListForCharacters(ctx, doc.ID, characterIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("checking direct shares: %w", err)
	}

	reached := false
	for i := range shares {
		if dayByCharacter[shares[i].CharacterID] >= shares[i].SharedOnGameDay {
			reached = true

			break
		}
	}
	if !reached {
		return nil, nil, ErrNotFound
	}

	repo, err := s.repositories.GetUnchecked(ctx, doc.RepositoryID)
	if err != nil {
		return nil, nil, err
	}

	return doc, repo, nil
}

// Delete removes a document and its sections. GM only — open editing (core
// domain rule 7) covers a document's content, not its existence. The removal
// is recorded in the activity log in the same transaction, scoped to the
// document's repository.
func (s *DocumentService) Delete(ctx context.Context, requester Requester, id string) error {
	if !requester.IsGM() {
		return ErrForbidden
	}

	doc, err := s.documents.GetByID(ctx, id)
	if err != nil {
		return notFoundOr(err, "loading document")
	}

	repo, err := s.repositories.GetUnchecked(ctx, doc.RepositoryID)
	if err != nil {
		return err
	}

	entry, err := s.documentEntry(ctx, requester, repo, doc, models.ActivityActionRemoved)
	if err != nil {
		return err
	}
	if err := s.documents.Delete(ctx, doc, entry); err != nil {
		return fmt.Errorf("deleting document: %w", err)
	}

	return nil
}

// ShareWithCharacter directly shares a document with one character, revealed
// to them once their current_game_day reaches sharedOnGameDay — independent
// of the document's own repository access. GM only.
func (s *DocumentService) ShareWithCharacter(
	ctx context.Context, requester Requester, documentID, characterID string, sharedOnGameDay int,
) (*models.DocumentShare, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	if _, err := s.documents.GetByID(ctx, documentID); err != nil {
		return nil, notFoundOr(err, "loading document")
	}
	if _, err := s.characters.GetByID(ctx, characterID); err != nil {
		return nil, notFoundOr(err, "loading character")
	}

	exists, err := s.shares.Exists(ctx, documentID, characterID)
	if err != nil {
		return nil, fmt.Errorf("checking existing share: %w", err)
	}
	if exists {
		return nil, ErrAlreadyShared
	}

	share := &models.DocumentShare{DocumentID: documentID, CharacterID: characterID, SharedOnGameDay: sharedOnGameDay}
	if err := s.shares.Create(ctx, share); err != nil {
		return nil, fmt.Errorf("sharing document: %w", err)
	}

	return share, nil
}

// ListShares returns every direct share on a document. GM only — players
// never see the share list.
func (s *DocumentService) ListShares(
	ctx context.Context, requester Requester, documentID string,
) ([]models.DocumentShare, error) {
	if !requester.IsGM() {
		return nil, ErrForbidden
	}

	if _, err := s.documents.GetByID(ctx, documentID); err != nil {
		return nil, notFoundOr(err, "loading document")
	}

	shares, err := s.shares.ListByDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("listing shares: %w", err)
	}

	return shares, nil
}

// RevokeShare removes one direct share from a document. GM only.
func (s *DocumentService) RevokeShare(ctx context.Context, requester Requester, documentID, shareID string) error {
	if !requester.IsGM() {
		return ErrForbidden
	}

	share, err := s.shares.GetByID(ctx, shareID)
	if err != nil {
		return notFoundOr(err, "loading share")
	}
	if share.DocumentID != documentID {
		return ErrNotFound
	}

	if err := s.shares.Delete(ctx, share); err != nil {
		return fmt.Errorf("revoking share: %w", err)
	}

	return nil
}

// ListSharedWithMe returns the documents directly shared with any of the
// requester's characters whose game day has been reached, with sections
// stripped to what the requester may see.
func (s *DocumentService) ListSharedWithMe(ctx context.Context, requester Requester) ([]DocumentView, error) {
	characters, err := s.characters.ListByUser(ctx, requester.UserID())
	if err != nil {
		return nil, fmt.Errorf("listing requester characters: %w", err)
	}

	characterIDs := make([]string, len(characters))
	dayByCharacter := make(map[string]int, len(characters))
	for i := range characters {
		characterIDs[i] = characters[i].ID
		dayByCharacter[characters[i].ID] = characters[i].CurrentGameDay
	}

	shares, err := s.shares.ListByCharacters(ctx, characterIDs)
	if err != nil {
		return nil, fmt.Errorf("listing shares: %w", err)
	}

	seen := make(map[string]struct{}, len(shares))
	views := make([]DocumentView, 0, len(shares))
	for i := range shares {
		share := &shares[i]
		if dayByCharacter[share.CharacterID] < share.SharedOnGameDay {
			continue
		}
		if _, dup := seen[share.DocumentID]; dup {
			continue
		}
		seen[share.DocumentID] = struct{}{}

		doc, err := s.documents.GetByID(ctx, share.DocumentID)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				continue
			}

			return nil, fmt.Errorf("loading shared document: %w", err)
		}

		repo, err := s.repositories.GetUnchecked(ctx, doc.RepositoryID)
		if err != nil {
			return nil, err
		}

		view, err := s.view(ctx, requester, repo, doc)
		if err != nil {
			return nil, err
		}

		views = append(views, *view)
	}

	return views, nil
}

// applyMetadata writes the update's metadata onto the document, enforcing
// the path-collision warning and the GM-only reveal-day rule.
func (s *DocumentService) applyMetadata(
	ctx context.Context, requester Requester, doc *models.Document, input *UpdateDocumentInput,
) error {
	path, err := normalizePath(input.Path)
	if err != nil {
		return err
	}

	if path != doc.Path && !input.AllowCollision {
		exists, err := s.documents.ExistsAtPath(ctx, doc.RepositoryID, path, doc.ID)
		if err != nil {
			return fmt.Errorf("checking path collision: %w", err)
		}
		if exists {
			return ErrPathCollision
		}
	}

	if input.SharedOnGameDay != doc.SharedOnGameDay && !requester.IsGM() {
		return fmt.Errorf("%w: only a GM can change the reveal day", ErrForbidden)
	}

	doc.Path = path
	doc.Title = titleOrFallback(input.Title, path)
	doc.Tags = input.Tags
	doc.SharedOnGameDay = input.SharedOnGameDay

	return nil
}

// view strips GM-only sections for players and resolves the revealed flag —
// the last stop before a document leaves the service layer.
func (s *DocumentService) view(
	ctx context.Context, requester Requester, repo *models.Repository, doc *models.Document,
) (*DocumentView, error) {
	if requester.IsGM() {
		revealed, err := s.revealed(ctx, repo, doc.SharedOnGameDay)
		if err != nil {
			return nil, err
		}

		return &DocumentView{Document: doc, Revealed: revealed}, nil
	}

	visible := make([]models.DocumentSection, 0, len(doc.Sections))
	for i := range doc.Sections {
		if !doc.Sections[i].GMOnly {
			visible = append(visible, doc.Sections[i])
		}
	}
	doc.Sections = visible

	// A player only ever holds a document that is revealed to them.
	return &DocumentView{Document: doc, Revealed: true}, nil
}

// revealed reports whether any character with access to the repository has
// reached the given game day.
func (s *DocumentService) revealed(
	ctx context.Context, repo *models.Repository, sharedOnGameDay int,
) (bool, error) {
	switch repo.Type {
	case models.RepositoryTypeGeneral, models.RepositoryTypeTemplate:
		revealed, err := s.characters.AnyWithGameDayAtLeast(ctx, sharedOnGameDay)
		if err != nil {
			return false, fmt.Errorf("checking reveal state: %w", err)
		}

		return revealed, nil
	case models.RepositoryTypeGroup:
		if repo.GroupID == nil {
			return false, nil
		}

		group, err := s.groups.GetByID(ctx, *repo.GroupID)
		if err != nil {
			return false, fmt.Errorf("loading group: %w", err)
		}

		for i := range group.Members {
			if group.Members[i].CurrentGameDay >= sharedOnGameDay {
				return true, nil
			}
		}

		return false, nil
	case models.RepositoryTypeCharacter:
		if repo.CharacterID == nil {
			return false, nil
		}

		character, err := s.characters.GetByID(ctx, *repo.CharacterID)
		if err != nil {
			return false, fmt.Errorf("loading character: %w", err)
		}

		return character.CurrentGameDay >= sharedOnGameDay, nil
	default:
		return false, nil
	}
}

// effectiveGameDay resolves the highest current_game_day among the
// requester's characters that can reach the repository. ok is false when no
// character of theirs reaches it at all.
func (s *DocumentService) effectiveGameDay(
	ctx context.Context, requester Requester, repo *models.Repository,
) (day int, ok bool, err error) {
	characters, err := s.characters.ListByUser(ctx, requester.UserID())
	if err != nil {
		return 0, false, fmt.Errorf("listing requester characters: %w", err)
	}

	eligible, err := s.charactersWithRepoAccess(ctx, repo, characters)
	if err != nil {
		return 0, false, err
	}

	for i := range eligible {
		if !ok || eligible[i].CurrentGameDay > day {
			day, ok = eligible[i].CurrentGameDay, true
		}
	}

	return day, ok, nil
}

// charactersWithRepoAccess filters the requester's characters down to those
// the repository's own visibility rule grants access.
func (s *DocumentService) charactersWithRepoAccess(
	ctx context.Context, repo *models.Repository, characters []models.Character,
) ([]models.Character, error) {
	switch repo.Type {
	case models.RepositoryTypeGeneral, models.RepositoryTypeTemplate:
		return characters, nil
	case models.RepositoryTypeCharacter:
		if repo.CharacterID == nil {
			return nil, nil
		}

		for i := range characters {
			if characters[i].ID == *repo.CharacterID {
				return characters[i : i+1], nil
			}
		}

		return nil, nil
	case models.RepositoryTypeGroup:
		if repo.GroupID == nil {
			return nil, nil
		}

		group, err := s.groups.GetByID(ctx, *repo.GroupID)
		if err != nil {
			return nil, fmt.Errorf("loading group: %w", err)
		}

		memberIDs := make(map[string]struct{}, len(group.Members))
		for i := range group.Members {
			memberIDs[group.Members[i].ID] = struct{}{}
		}

		var eligible []models.Character
		for i := range characters {
			if _, member := memberIDs[characters[i].ID]; member {
				eligible = append(eligible, characters[i])
			}
		}

		return eligible, nil
	default:
		return nil, nil
	}
}

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

	meta, body, err := parseFrontmatter(input.Markdown)
	if err != nil {
		return "", nil, 0, nil, err
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
	visibleByID := make(map[string]struct{}, len(existing))
	for i := range existing {
		if !existing[i].GMOnly {
			visibleByID[existing[i].ID] = struct{}{}
		}
	}

	submitted := make(map[string]DocumentSectionInput, len(inputs))
	var appended []DocumentSectionInput
	for _, input := range inputs {
		if input.GMOnly {
			return nil, fmt.Errorf("%w: only a GM can mark sections GM-only", ErrForbidden)
		}
		if input.ID == "" {
			appended = append(appended, input)

			continue
		}
		if _, visible := visibleByID[input.ID]; !visible {
			return nil, fmt.Errorf("%w: unknown section %q", ErrInvalidDocument, input.ID)
		}

		submitted[input.ID] = input
	}

	final := make([]models.DocumentSection, 0, len(existing)+len(appended))
	for i := range existing {
		sec := existing[i]
		if sec.GMOnly {
			final = append(final, sec)

			continue
		}
		if input, kept := submitted[sec.ID]; kept {
			sec.Content = input.Content
			final = append(final, sec)
		}
	}

	for _, input := range appended {
		final = append(final, models.DocumentSection{Content: input.Content})
	}

	return final, nil
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

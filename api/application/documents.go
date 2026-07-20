package application

import (
	"context"
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
var ErrPathCollision = codedServiceErr(KindConflict, "path_collision", "a document already exists at this path")

// ErrConcurrentEdit is returned when a document changed since the editor
// loaded it and the caller didn't set Force. A warning, not a block:
// resubmitting with Force overwrites (core domain rule 7).
var ErrConcurrentEdit = codedServiceErr(KindConflict, "concurrent_edit", "the document changed since it was loaded")

// ErrInvalidDocument is returned when a document payload is malformed: empty
// or invalid path, a section reference that doesn't resolve, or both raw
// markdown and structured sections in one request.
var ErrInvalidDocument = serviceErr(KindValidation, "invalid document")

// ErrAlreadyShared is returned when a document is already directly shared
// with the given character.
var ErrAlreadyShared = serviceErr(KindConflict, "document already shared with this character")

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
		characters, err := requesterCharacters(ctx, s.characters, requester)
		if err != nil {
			return nil, fmt.Errorf("listing requester characters: %w", err)
		}

		eligible, err := s.charactersWithRepoAccess(ctx, repo, characters)
		if err != nil {
			return nil, err
		}

		actor = ""
		if best := furthestCharacter(eligible); best != nil {
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
		revealed, err := s.repoAccess(repo).revealed(ctx, doc.SharedOnGameDay)
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

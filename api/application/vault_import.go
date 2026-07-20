package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
)

// ErrInvalidImport is returned when an import request as a whole is malformed
// (no files, or no way to resolve a target repository).
var ErrInvalidImport = serviceErr(KindValidation, "invalid import")

// Per-file import outcomes. A collision is a warning, not a failure: the
// client re-submits the file with a new path (rename) or AllowCollision set
// (continue) — core domain rule 7 applied to imports.
const (
	ImportStatusImported  = "imported"
	ImportStatusCollision = "collision"
	ImportStatusError     = "error"
)

// ImportFileInput is one markdown file from an Obsidian vault. Path is the
// vault-relative file path (folders map to the document path, a trailing .md
// is dropped); Markdown is the raw file content, frontmatter included.
type ImportFileInput struct {
	Path           string
	Markdown       string
	AllowCollision bool
}

// ImportFileResult reports what happened to one imported file.
type ImportFileResult struct {
	Path         string
	Status       string
	DocumentID   string
	RepositoryID string
	Error        string
}

// VaultImportService imports Obsidian vaults as knowledge documents (roadmap
// M6). Each file becomes a document via DocumentService.Create, so every
// document rule — repository access, frontmatter parsing, path collisions,
// activity logging — applies exactly as it does for hand-created documents.
type VaultImportService struct {
	documents      *DocumentService
	repositories   *RepositoryService
	knowledgeRepos *repositories.KnowledgeRepositories
	groups         *repositories.Groups
	characters     *repositories.Characters
}

// NewVaultImportService builds a VaultImportService.
func NewVaultImportService(
	documents *DocumentService,
	repos *RepositoryService,
	knowledgeRepos *repositories.KnowledgeRepositories,
	groups *repositories.Groups,
	characters *repositories.Characters,
) *VaultImportService {
	return &VaultImportService{
		documents:      documents,
		repositories:   repos,
		knowledgeRepos: knowledgeRepos,
		groups:         groups,
		characters:     characters,
	}
}

// Import imports a batch of vault files. Files whose frontmatter names a
// `repository` go there; the rest go to defaultRepositoryID. Each file is
// imported independently and reported in order — one bad file never aborts
// the batch. Repository access is enforced per file by DocumentService.Create
// (an unreachable repository reads as not found, never as forbidden), and an
// inaccessible defaultRepositoryID fails the whole request with ErrNotFound
// before anything is written.
func (s *VaultImportService) Import(
	ctx context.Context, requester Requester, defaultRepositoryID string, files []ImportFileInput,
) ([]ImportFileResult, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no files given", ErrInvalidImport)
	}

	if defaultRepositoryID != "" {
		if _, err := s.repositories.Get(ctx, requester, defaultRepositoryID); err != nil {
			return nil, err
		}
	}

	results := make([]ImportFileResult, len(files))
	for i := range files {
		results[i] = s.importFile(ctx, requester, defaultRepositoryID, &files[i])
	}

	return results, nil
}

// importFile imports one vault file, mapping domain errors onto the per-file
// result instead of failing the batch.
func (s *VaultImportService) importFile(
	ctx context.Context, requester Requester, defaultRepositoryID string, file *ImportFileInput,
) ImportFileResult {
	result := ImportFileResult{Path: file.Path, Status: ImportStatusError}

	docPath := strings.TrimSuffix(strings.TrimSuffix(file.Path, ".md"), ".MD")
	if strings.TrimSpace(docPath) == "" {
		result.Error = "file path is required"

		return result
	}

	meta, _, err := parseFrontmatter(file.Markdown)
	if err != nil {
		result.Error = err.Error()

		return result
	}

	targetID := defaultRepositoryID
	if meta.Repository != "" {
		repo, err := s.resolveRepository(ctx, requester, meta.Repository)
		if err != nil {
			result.Error = err.Error()

			return result
		}

		targetID = repo.ID
	}
	if targetID == "" {
		result.Error = "no target repository: set one in frontmatter or pick a default"

		return result
	}

	view, err := s.documents.Create(ctx, requester, targetID, &CreateDocumentInput{
		Path:           docPath,
		Markdown:       file.Markdown,
		AllowCollision: file.AllowCollision,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrPathCollision):
			result.Status = ImportStatusCollision
			result.Error = err.Error()
		case errors.Is(err, ErrNotFound):
			// Same wording as an unresolvable frontmatter name, so an
			// existing-but-inaccessible repository is indistinguishable from a
			// nonexistent one.
			result.Error = repositoryNotFoundMessage(meta.Repository)
		default:
			result.Error = err.Error()
		}

		return result
	}

	result.Status = ImportStatusImported
	result.DocumentID = view.Document.ID
	result.RepositoryID = view.Document.RepositoryID

	return result
}

// repositoryNotFoundMessage is the one message every unreachable-repository
// path produces, whether the name didn't resolve or resolved to something the
// requester may not see — existence must not leak through wording.
func repositoryNotFoundMessage(name string) string {
	if name == "" {
		return "repository not found"
	}

	return fmt.Sprintf("repository %q not found", name)
}

// resolveRepository maps a frontmatter repository name onto a repository the
// requester may see: the "general"/"template" keywords, a group name, or a
// character name (docs/architecture.md, Document Format). Candidates the
// requester cannot access are silently dropped, so probing names never
// confirms an inaccessible group or character exists.
func (s *VaultImportService) resolveRepository(
	ctx context.Context, requester Requester, name string,
) (*models.Repository, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "general":
		repo, err := s.knowledgeRepos.EnsureGeneral(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading general repository: %w", err)
		}

		return repo, nil
	case "template":
		repo, err := s.knowledgeRepos.EnsureTemplate(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading template repository: %w", err)
		}

		return repo, nil
	}

	candidates, err := s.namedRepositoryCandidates(ctx, name)
	if err != nil {
		return nil, err
	}

	accessible := make([]*models.Repository, 0, len(candidates))
	for _, id := range candidates {
		repo, err := s.repositories.Get(ctx, requester, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}

			return nil, err
		}

		accessible = append(accessible, repo)
	}

	switch len(accessible) {
	case 0:
		return nil, errors.New(repositoryNotFoundMessage(name))
	case 1:
		return accessible[0], nil
	default:
		return nil, fmt.Errorf("repository name %q is ambiguous: rename the frontmatter target", name)
	}
}

// namedRepositoryCandidates collects the repository IDs of every group and
// character matching name (case-insensitively).
func (s *VaultImportService) namedRepositoryCandidates(ctx context.Context, name string) ([]string, error) {
	var candidates []string

	groups, err := s.groups.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	for i := range groups {
		if !strings.EqualFold(groups[i].Name, name) {
			continue
		}

		repo, err := s.knowledgeRepos.EnsureForGroup(ctx, groups[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading group repository: %w", err)
		}

		candidates = append(candidates, repo.ID)
	}

	characters, err := s.characters.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing characters: %w", err)
	}
	for i := range characters {
		if !strings.EqualFold(characters[i].Name, name) {
			continue
		}

		repo, err := s.knowledgeRepos.EnsureForCharacter(ctx, characters[i].ID)
		if err != nil {
			return nil, fmt.Errorf("loading character repository: %w", err)
		}

		candidates = append(candidates, repo.ID)
	}

	return candidates, nil
}

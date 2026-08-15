package knowledgev1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type repositoryResponse struct {
	ID          string                `json:"id"`
	Type        models.RepositoryType `json:"type"`
	GroupID     *string               `json:"group_id,omitempty"`
	CharacterID *string               `json:"character_id,omitempty"`
}

func toRepositoryResponse(r *models.Repository) repositoryResponse {
	return repositoryResponse{ID: r.ID, Type: r.Type, GroupID: r.GroupID, CharacterID: r.CharacterID}
}

// Route paths for the repository resource and its nested documents collection.
const (
	RepositoriesPath           = "/api/repositories"
	RepositoryPath             = RepositoriesPath + "/{id}"
	RepositoryDocumentsPath    = RepositoryPath + "/documents"
	RepositoryDocumentTreePath = RepositoryDocumentsPath + "/tree"
)

// RepositoryHandler serves knowledge repositories (read-only — they are
// provisioned automatically, never created by a caller) and the documents
// nested under them, under /api/repositories. Repository reads use the
// repository service; the nested documents collection uses the document
// service.
type RepositoryHandler struct {
	repositories *application.RepositoryService
	documents    *application.DocumentService
}

// NewRepositoryHandler builds the repository resource handler.
func NewRepositoryHandler(
	repositories *application.RepositoryService, documents *application.DocumentService,
) *RepositoryHandler {
	return &RepositoryHandler{repositories: repositories, documents: documents}
}

// Register wires the repository routes onto r. Each handler must be reached
// through RequireAuth.
func (h *RepositoryHandler) Register(r *transport.Router) {
	r.Handle("GET "+RepositoriesPath, h.List())
	r.Handle("GET "+RepositoryPath, h.Get())
	r.Handle("GET "+RepositoryDocumentsPath, h.ListDocuments())
	r.Handle("POST "+RepositoryDocumentsPath, h.CreateDocument())
	r.Handle("GET "+RepositoryDocumentTreePath, h.DocumentTree())
}

// List returns every repository the caller may see: all of them for a GM, the
// general/template singletons plus the caller's own character and group
// repositories for a player.
func (h *RepositoryHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		repos, err := h.repositories.List(r.Context(), transport.RequesterFrom(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]repositoryResponse, len(repos))
		for i := range repos {
			responses[i] = toRepositoryResponse(&repos[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Get returns one repository, or 404 when the caller may not see it (existence
// hidden).
func (h *RepositoryHandler) Get() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		repo, err := h.repositories.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toRepositoryResponse(repo))
	})
}

// ListDocuments returns the documents in the repository named by {id} that the
// caller may see.
func (h *RepositoryHandler) ListDocuments() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		docs, err := h.documents.ListByRepository(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]documentListItemResponse, len(docs))
		for i := range docs {
			responses[i] = toDocumentListItemResponse(&docs[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// CreateDocument adds a document to the repository named by {id}.
func (h *RepositoryHandler) CreateDocument() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		view, err := h.documents.Create(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), &application.CreateDocumentInput{
				Path:            req.Path,
				Title:           req.Title,
				Tags:            req.Tags,
				SharedOnGameDay: req.SharedOnGameDay,
				Sections:        toSectionInputs(req.Sections),
				Markdown:        req.Markdown,
				AllowCollision:  req.AllowCollision,
			})
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toDocumentResponse(view))
	})
}

// DocumentTree returns the repository named by {id} as a folder tree of the
// documents the caller may see, sorted alphabetically at every level. Folders
// with no accessible documents never appear.
func (h *RepositoryHandler) DocumentTree() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		tree, err := h.documents.FolderTree(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toFolderTreeResponse(tree))
	})
}

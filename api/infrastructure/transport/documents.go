package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
)

type documentSectionPayload struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content"`
	GMOnly  bool   `json:"gm_only"`
}

type createDocumentRequest struct {
	Path            string                   `json:"path"`
	Title           string                   `json:"title"`
	Tags            []string                 `json:"tags"`
	SharedOnGameDay *int                     `json:"shared_on_game_day"`
	Sections        []documentSectionPayload `json:"sections"`
	Markdown        string                   `json:"markdown"`
	AllowCollision  bool                     `json:"allow_collision"`
}

type updateDocumentRequest struct {
	Path            string                   `json:"path"`
	Title           string                   `json:"title"`
	Tags            []string                 `json:"tags"`
	SharedOnGameDay int                      `json:"shared_on_game_day"`
	Sections        []documentSectionPayload `json:"sections"`
	ExpectedVersion *int                     `json:"expected_version"`
	Force           bool                     `json:"force"`
	AllowCollision  bool                     `json:"allow_collision"`
}

type shareDocumentRequest struct {
	TargetRepositoryID string `json:"target_repository_id"`
	SharedOnGameDay    int    `json:"shared_on_game_day"`
	AllowCollision     bool   `json:"allow_collision"`
}

type documentListItemResponse struct {
	ID              string   `json:"id"`
	RepositoryID    string   `json:"repository_id"`
	Path            string   `json:"path"`
	Title           string   `json:"title"`
	Tags            []string `json:"tags"`
	SharedOnGameDay int      `json:"shared_on_game_day"`
}

type documentResponse struct {
	documentListItemResponse
	Version  int                      `json:"version"`
	Revealed bool                     `json:"revealed"`
	Sections []documentSectionPayload `json:"sections"`
}

type folderTreeNodeResponse struct {
	Name      string                     `json:"name"`
	Path      string                     `json:"path"`
	Folders   []folderTreeNodeResponse   `json:"folders"`
	Documents []documentListItemResponse `json:"documents"`
}

func toDocumentListItemResponse(d *models.Document) documentListItemResponse {
	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}

	return documentListItemResponse{
		ID:              d.ID,
		RepositoryID:    d.RepositoryID,
		Path:            d.Path,
		Title:           d.Title,
		Tags:            tags,
		SharedOnGameDay: d.SharedOnGameDay,
	}
}

func toDocumentResponse(v *application.DocumentView) documentResponse {
	sections := make([]documentSectionPayload, len(v.Document.Sections))
	for i := range v.Document.Sections {
		sec := &v.Document.Sections[i]
		sections[i] = documentSectionPayload{ID: sec.ID, Content: sec.Content, GMOnly: sec.GMOnly}
	}

	return documentResponse{
		documentListItemResponse: toDocumentListItemResponse(v.Document),
		Version:                  v.Document.Version,
		Revealed:                 v.Revealed,
		Sections:                 sections,
	}
}

func toSectionInputs(payloads []documentSectionPayload) []application.DocumentSectionInput {
	inputs := make([]application.DocumentSectionInput, len(payloads))
	for i, p := range payloads {
		inputs[i] = application.DocumentSectionInput{ID: p.ID, Content: p.Content, GMOnly: p.GMOnly}
	}

	return inputs
}

// ListDocumentsHandler returns the documents in the repository named by {id}
// that the caller may see. Must be wrapped in RequireAuth.
func ListDocumentsHandler(svc *application.DocumentService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		docs, err := svc.ListByRepository(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeDocumentServiceError(w, err)

			return
		}

		responses := make([]documentListItemResponse, len(docs))
		for i := range docs {
			responses[i] = toDocumentListItemResponse(&docs[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// toFolderTreeResponse converts a folder tree node, recursively.
func toFolderTreeResponse(node *application.FolderNode) folderTreeNodeResponse {
	folders := make([]folderTreeNodeResponse, len(node.Folders))
	for i, f := range node.Folders {
		folders[i] = toFolderTreeResponse(f)
	}

	docs := make([]documentListItemResponse, len(node.Documents))
	for i := range node.Documents {
		docs[i] = toDocumentListItemResponse(&node.Documents[i])
	}

	return folderTreeNodeResponse{Name: node.Name, Path: node.Path, Folders: folders, Documents: docs}
}

// GetDocumentFolderTreeHandler returns the repository named by {id} as a
// folder tree of the documents the caller may see, sorted alphabetically at
// every level. Folders with no accessible documents never appear. Must be
// wrapped in RequireAuth.
func GetDocumentFolderTreeHandler(svc *application.DocumentService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree, err := svc.FolderTree(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeDocumentServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toFolderTreeResponse(tree))
	})
}

// CreateDocumentHandler adds a document to the repository named by {id}.
// Must be wrapped in RequireAuth.
func CreateDocumentHandler(svc *application.DocumentService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		view, err := svc.Create(r.Context(), requesterFrom(r), r.PathValue("id"), &application.CreateDocumentInput{
			Path:            req.Path,
			Title:           req.Title,
			Tags:            req.Tags,
			SharedOnGameDay: req.SharedOnGameDay,
			Sections:        toSectionInputs(req.Sections),
			Markdown:        req.Markdown,
			AllowCollision:  req.AllowCollision,
		})
		if err != nil {
			writeDocumentServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toDocumentResponse(view))
	})
}

// GetDocumentHandler returns one document with the sections the caller may
// see. Must be wrapped in RequireAuth.
func GetDocumentHandler(svc *application.DocumentService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		view, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeDocumentServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toDocumentResponse(view))
	})
}

// UpdateDocumentHandler replaces a document's metadata and the caller's
// visible sections. Must be wrapped in RequireAuth.
func UpdateDocumentHandler(svc *application.DocumentService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req updateDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		view, err := svc.Update(r.Context(), requesterFrom(r), r.PathValue("id"), &application.UpdateDocumentInput{
			Path:            req.Path,
			Title:           req.Title,
			Tags:            req.Tags,
			SharedOnGameDay: req.SharedOnGameDay,
			Sections:        toSectionInputs(req.Sections),
			ExpectedVersion: req.ExpectedVersion,
			Force:           req.Force,
			AllowCollision:  req.AllowCollision,
		})
		if err != nil {
			writeDocumentServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toDocumentResponse(view))
	})
}

// ShareDocumentHandler moves the document named by {id} out of its character
// repository into a target group repository at a chosen game day. Must be
// wrapped in RequireAuth.
func ShareDocumentHandler(svc *application.DocumentService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req shareDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		view, err := svc.ShareToGroup(r.Context(), requesterFrom(r), r.PathValue("id"), &application.ShareDocumentInput{
			TargetRepositoryID: req.TargetRepositoryID,
			SharedOnGameDay:    req.SharedOnGameDay,
			AllowCollision:     req.AllowCollision,
		})
		if err != nil {
			writeDocumentServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toDocumentResponse(view))
	})
}

// writeDocumentServiceError maps DocumentService errors onto HTTP. The two
// editor warnings ride on 409 with a machine-readable code so the client can
// offer "rename or continue" / "overwrite anyway".
func writeDocumentServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, application.ErrPathCollision):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "code": "path_collision"})
	case errors.Is(err, application.ErrConcurrentEdit):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "code": "concurrent_edit"})
	case errors.Is(err, application.ErrInvalidDocument):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, application.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
}

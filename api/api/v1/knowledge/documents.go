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

// Route paths for single documents. Documents are created through their
// repository (see RepositoryHandler); reads and edits address the document
// directly.
const (
	DocumentsSharedPath   = "/api/documents/shared"
	DocumentPath          = "/api/documents/{id}"
	DocumentSharePath     = DocumentPath + "/share"
	DocumentSharesPath    = DocumentPath + "/shares"
	DocumentShareByIDPath = DocumentSharesPath + "/{shareId}"
)

// DocumentHandler serves single documents under /api/documents.
type DocumentHandler struct {
	documents *application.DocumentService
}

// NewDocumentHandler builds the single-document handler.
func NewDocumentHandler(documents *application.DocumentService) *DocumentHandler {
	return &DocumentHandler{documents: documents}
}

// Register wires the document routes onto r. Each handler must be reached
// through RequireAuth.
func (h *DocumentHandler) Register(r *transport.Router) {
	r.Handle("GET "+DocumentsSharedPath, h.ListShared())
	r.Handle("GET "+DocumentPath, h.Get())
	r.Handle("PATCH "+DocumentPath, h.Update())
	r.Handle("DELETE "+DocumentPath, h.Delete())
	r.Handle("POST "+DocumentSharePath, h.ShareToGroup())
	r.Handle("GET "+DocumentSharesPath, h.ListShares())
	r.Handle("POST "+DocumentSharesPath, h.ShareWithCharacter())
	r.Handle("DELETE "+DocumentShareByIDPath, h.RevokeShare())
}

// Get returns one document with the sections the caller may see.
func (h *DocumentHandler) Get() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		view, err := h.documents.Get(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toDocumentResponse(view))
	})
}

// Update replaces a document's metadata and the caller's visible sections.
func (h *DocumentHandler) Update() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		view, err := h.documents.Update(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), &application.UpdateDocumentInput{
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
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toDocumentResponse(view))
	})
}

// ShareToGroup moves the document named by {id} out of its character
// repository into a target group repository at a chosen game day.
func (h *DocumentHandler) ShareToGroup() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req shareDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		view, err := h.documents.ShareToGroup(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), &application.ShareDocumentInput{
				TargetRepositoryID: req.TargetRepositoryID,
				SharedOnGameDay:    req.SharedOnGameDay,
				AllowCollision:     req.AllowCollision,
			})
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toDocumentResponse(view))
	})
}

type shareDocumentWithCharacterRequest struct {
	CharacterID     string `json:"character_id"`
	SharedOnGameDay int    `json:"shared_on_game_day"`
}

type documentShareResponse struct {
	ID              string `json:"id"`
	DocumentID      string `json:"document_id"`
	CharacterID     string `json:"character_id"`
	SharedOnGameDay int    `json:"shared_on_game_day"`
}

func toDocumentShareResponse(s *models.DocumentShare) documentShareResponse {
	return documentShareResponse{
		ID: s.ID, DocumentID: s.DocumentID, CharacterID: s.CharacterID, SharedOnGameDay: s.SharedOnGameDay,
	}
}

// ShareWithCharacter lets a GM directly share the document named by {id} with
// one character on a game day.
func (h *DocumentHandler) ShareWithCharacter() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req shareDocumentWithCharacterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		if req.CharacterID == "" {
			w.WriteErrorMsg(http.StatusBadRequest, "invalid request body: missing character_id")

			return
		}

		share, err := h.documents.ShareWithCharacter(
			r.Context(), transport.RequesterFrom(r), r.PathValue("id"), req.CharacterID, req.SharedOnGameDay,
		)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toDocumentShareResponse(share))
	})
}

// ListShares lets a GM list the direct shares on a document.
func (h *DocumentHandler) ListShares() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		shares, err := h.documents.ListShares(r.Context(), transport.RequesterFrom(r), r.PathValue("id"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]documentShareResponse, len(shares))
		for i := range shares {
			responses[i] = toDocumentShareResponse(&shares[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// RevokeShare lets a GM remove one direct share from a document.
func (h *DocumentHandler) RevokeShare() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := h.documents.RevokeShare(r.Context(), transport.RequesterFrom(r), r.PathValue("id"), r.PathValue("shareId"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// ListShared returns the documents directly shared with any of the caller's
// characters whose game day has been reached.
func (h *DocumentHandler) ListShared() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		views, err := h.documents.ListSharedWithMe(r.Context(), transport.RequesterFrom(r))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]documentResponse, len(views))
		for i := range views {
			responses[i] = toDocumentResponse(&views[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Delete removes a document and its sections. GM only; the removal is recorded
// in the activity log.
func (h *DocumentHandler) Delete() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		if err := h.documents.Delete(r.Context(), transport.RequesterFrom(r), r.PathValue("id")); err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

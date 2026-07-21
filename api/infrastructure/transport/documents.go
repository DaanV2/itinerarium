package transport

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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

// ListDocumentsHandler returns the documents in the repository named by {id}
// that the caller may see. Must be wrapped in RequireAuth.
func ListDocumentsHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		docs, err := svc.ListByRepository(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		responses := make([]documentListItemResponse, len(docs))
		for i := range docs {
			responses[i] = toDocumentListItemResponse(&docs[i])
		}

		w.WriteJSON(http.StatusOK, responses)
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
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		tree, err := svc.FolderTree(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toFolderTreeResponse(tree))
	})
}

// CreateDocumentHandler adds a document to the repository named by {id}.
// Must be wrapped in RequireAuth.
func CreateDocumentHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

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
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toDocumentResponse(view))
	})
}

// GetDocumentHandler returns one document with the sections the caller may
// see. Must be wrapped in RequireAuth.
func GetDocumentHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		view, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toDocumentResponse(view))
	})
}

// UpdateDocumentHandler replaces a document's metadata and the caller's
// visible sections. Must be wrapped in RequireAuth.
func UpdateDocumentHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req updateDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

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
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, toDocumentResponse(view))
	})
}

// ShareDocumentHandler moves the document named by {id} out of its character
// repository into a target group repository at a chosen game day. Must be
// wrapped in RequireAuth.
func ShareDocumentHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req shareDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		view, err := svc.ShareToGroup(r.Context(), requesterFrom(r), r.PathValue("id"), &application.ShareDocumentInput{
			TargetRepositoryID: req.TargetRepositoryID,
			SharedOnGameDay:    req.SharedOnGameDay,
			AllowCollision:     req.AllowCollision,
		})
		if err != nil {
			writeServiceError(w, err)

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

// ShareDocumentWithCharacterHandler lets a GM directly share the document
// named by {id} with one character on a game day. Must be wrapped in
// RequireAuth.
func ShareDocumentWithCharacterHandler(svc *application.DocumentService) http.Handler {
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

		share, err := svc.ShareWithCharacter(
			r.Context(), requesterFrom(r), r.PathValue("id"), req.CharacterID, req.SharedOnGameDay,
		)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toDocumentShareResponse(share))
	})
}

// ListDocumentSharesHandler lets a GM list the direct shares on a document.
// Must be wrapped in RequireAuth.
func ListDocumentSharesHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		shares, err := svc.ListShares(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		responses := make([]documentShareResponse, len(shares))
		for i := range shares {
			responses[i] = toDocumentShareResponse(&shares[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// RevokeDocumentShareHandler lets a GM remove one direct share from a
// document. Must be wrapped in RequireAuth.
func RevokeDocumentShareHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		err := svc.RevokeShare(r.Context(), requesterFrom(r), r.PathValue("id"), r.PathValue("shareId"))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// ListSharedDocumentsHandler returns the documents directly shared with any
// of the caller's characters whose game day has been reached. Must be
// wrapped in RequireAuth.
func ListSharedDocumentsHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		views, err := svc.ListSharedWithMe(r.Context(), requesterFrom(r))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		responses := make([]documentResponse, len(views))
		for i := range views {
			responses[i] = toDocumentResponse(&views[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// DeleteDocumentHandler removes a document and its sections. GM only; the
// removal is recorded in the activity log. Must be wrapped in RequireAuth.
func DeleteDocumentHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		if err := svc.Delete(r.Context(), requesterFrom(r), r.PathValue("id")); err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

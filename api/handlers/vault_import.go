package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
	"github.com/DaanV2/itinerarium/api/transport"
)

type importVaultFileRequest struct {
	Path           string `json:"path"`
	Markdown       string `json:"markdown"`
	AllowCollision bool   `json:"allow_collision"`
}

type importVaultRequest struct {
	RepositoryID string                   `json:"repository_id"`
	Files        []importVaultFileRequest `json:"files"`
}

type importVaultFileResponse struct {
	Path         string `json:"path"`
	Status       string `json:"status"`
	DocumentID   string `json:"document_id,omitempty"`
	RepositoryID string `json:"repository_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

type importVaultResponse struct {
	Results []importVaultFileResponse `json:"results"`
}

// ImportVaultHandler imports a batch of Obsidian vault files as documents.
// Files are reported one by one: collisions come back as status "collision"
// so the client can offer rename-or-continue per file. Must be wrapped in
// RequireAuth.
func ImportVaultHandler(svc *application.VaultImportService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req importVaultRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		files := make([]application.ImportFileInput, len(req.Files))
		for i, f := range req.Files {
			files[i] = application.ImportFileInput{Path: f.Path, Markdown: f.Markdown, AllowCollision: f.AllowCollision}
		}

		results, err := svc.Import(r.Context(), transport.RequesterFrom(r), req.RepositoryID, files)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]importVaultFileResponse, len(results))
		for i, res := range results {
			responses[i] = importVaultFileResponse{
				Path:         res.Path,
				Status:       res.Status,
				DocumentID:   res.DocumentID,
				RepositoryID: res.RepositoryID,
				Error:        res.Error,
			}
		}

		w.WriteJSON(http.StatusOK, importVaultResponse{Results: responses})
	})
}

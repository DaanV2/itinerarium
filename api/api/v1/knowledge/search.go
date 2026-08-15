package knowledgev1

import (
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type searchResultResponse struct {
	documentListItemResponse
	MatchedIn []string `json:"matched_in"`
	Snippet   string   `json:"snippet,omitempty"`
}

// SearchPath runs a full-text search over the documents the caller may see.
const SearchPath = "/api/search"

// SearchHandler serves document search under /api/search.
type SearchHandler struct {
	documents *application.DocumentService
}

// NewSearchHandler builds the document-search handler.
func NewSearchHandler(documents *application.DocumentService) *SearchHandler {
	return &SearchHandler{documents: documents}
}

// Register wires the search route onto r. The handler must be reached through
// RequireAuth.
func (h *SearchHandler) Register(r *transport.Router) {
	r.Handle("GET "+SearchPath, h.Search())
}

// Search runs a full-text search (?q=...) over the documents the caller may
// see. Access filtering happens in the service before results are built —
// inaccessible documents never surface.
func (h *SearchHandler) Search() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		results, err := h.documents.Search(r.Context(), transport.RequesterFrom(r), r.URL.Query().Get("q"))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]searchResultResponse, len(results))
		for i := range results {
			responses[i] = searchResultResponse{
				documentListItemResponse: toDocumentListItemResponse(results[i].Document),
				MatchedIn:                results[i].MatchedIn,
				Snippet:                  results[i].Snippet,
			}
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

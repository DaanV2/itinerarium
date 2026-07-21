package transport

import (
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type searchResultResponse struct {
	documentListItemResponse
	MatchedIn []string `json:"matched_in"`
	Snippet   string   `json:"snippet,omitempty"`
}

// SearchDocumentsHandler runs a full-text search (?q=…) over the documents
// the caller may see. Access filtering happens in the service before results
// are built — inaccessible documents never surface. Must be wrapped in
// RequireAuth.
func SearchDocumentsHandler(svc *application.DocumentService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		results, err := svc.Search(r.Context(), requesterFrom(r), r.URL.Query().Get("q"))
		if err != nil {
			writeServiceError(w, err)

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

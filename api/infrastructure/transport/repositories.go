package transport

import (
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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

// ListRepositoriesHandler returns every repository the caller may see: all of
// them for a GM, the general/template singletons plus the caller's own
// character and group repositories for a player. Must be wrapped in
// RequireAuth.
func ListRepositoriesHandler(svc *application.RepositoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repos, err := svc.List(r.Context(), requesterFrom(r))
		if err != nil {
			writeRepositoryServiceError(w, err)

			return
		}

		responses := make([]repositoryResponse, len(repos))
		for i := range repos {
			responses[i] = toRepositoryResponse(&repos[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// GetRepositoryHandler returns one repository, or 404 when the caller may not
// see it (existence hidden). Must be wrapped in RequireAuth.
func GetRepositoryHandler(svc *application.RepositoryService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		if err != nil {
			writeRepositoryServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, toRepositoryResponse(repo))
	})
}

func writeRepositoryServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
}

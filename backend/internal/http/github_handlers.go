package http

import (
	"net/http"
	"strings"

	"github.com/repobounty/repobounty-ai/internal/github"
)

func (h *Handlers) GitHubSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter is required")
		return
	}

	parts := strings.Split(query, "/")

	if len(parts) == 1 {
		results, err := h.github.SearchUsers(r.Context(), parts[0])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to search users")
			return
		}
		if results == nil {
			results = []github.UserSearchResult{}
		}
		writeJSON(w, http.StatusOK, results)
	} else if len(parts) == 2 {
		results, err := h.github.SearchRepositories(r.Context(), parts[0], parts[1])
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to search repositories")
			return
		}
		if results == nil {
			results = []github.RepoSearchResult{}
		}
		writeJSON(w, http.StatusOK, results)
	} else {
		writeError(w, http.StatusBadRequest, "invalid query format")
	}
}

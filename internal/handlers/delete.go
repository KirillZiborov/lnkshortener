// Package handlers implements HTTP handler functions for the URL shortener service.
// It provides endpoints for creating, retrieving, and deleting shortened URLs.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/logic"
)

// BatchDeleteHandler handles the deletion of multiple shortened URLs for an authenticated user.
// It expects a DELETE request with a JSON array of short URL IDs.
// Upon successful deletion, it responds with a 202 Accepted status and processes the deletion asynchronously.
//
// Possible error codes in response:
// - 400 (Bad Request) if the request body is empty.
// - 401 (Unauthorized) if the authentification token is invalid.
// - 500 (Internal Server Error) if the server fails.
func BatchDeleteHandler(svc *logic.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Authenticate the user and retrieve the user ID.
		userID, err := auth.AuthGet(r)
		if err != nil {
			http.Error(w, "Unathorized", http.StatusUnauthorized)
		}

		var ids []string
		// Decode the JSON request body into a slice of short URL IDs.
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&ids); err != nil || len(ids) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Respond with a 202 Accepted status indicating that the deletion is being processed.
		w.WriteHeader(http.StatusAccepted)
		// Process the batch deletion asynchronously.
		svc.BatchDeleteAsync(userID, ids)
	}
}

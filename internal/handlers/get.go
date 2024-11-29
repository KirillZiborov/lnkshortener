package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/config"
)

// GetHandler handles redirection from a short URL to the original URL.
// It expects a GET request with the short URL.
// Upon finding the original URL, it redirects the client to it with a 302 Found status.
//
// Possible error codes in response:
// - 404 (Not Found) if there is no original URL for the requested short URL.
// - 410 (Gone) if the URL is deleted.
// - 500 (Internal Server Error) if the server fails.
func GetHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract a shortURL from request parameters.
		id := chi.URLParam(r, "id")
		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		// Get the original URL by the short URL.
		originalURL, deleted, err := store.GetOriginalURL(shortenedURL)
		if err != nil {
			// Get os.ErrProcessDone if the storage is fully checked but URL is not found.
			if errors.Is(err, os.ErrProcessDone) {
				http.Error(w, "Not found", http.StatusNotFound)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Check the delete flag.
		if deleted {
			http.Error(w, "URL has been deleted", http.StatusGone)
			return
		}

		// Redirect to the original URL.
		w.Header().Set("Location", originalURL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

// PingDBHandler checks the connection to the PostgreSQL database.
// It expects a GET request and responds with a 200 OK status.
//
// Possible error codes in response:
// - 500 (Internal Server Error) if the server fails to connect to a database.
func PingDBHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer cancel()

		err := db.Ping(ctx)
		if err != nil {
			http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// GetUserURLsHandler retrieves all URLs created by the authenticated user.
// It expects a GET request and responds with a JSON array of the user's URLs and a 200 OK status.
//
// Possible error codes in response:
// - 204 (No Content) if there is no user's URLs.
// - 401 (Unauthorized) if the authentification token is invalid.
// - 500 (Internal Server Error) if the server fails.
func GetUserURLsHandler(store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Authenticate and get the user ID.
		userID, err := auth.AuthGet(r)
		if err != nil {
			http.Error(w, "Unathorized", http.StatusUnauthorized)
		}

		// Retrieve the user's URLs from the storage.
		records, err := store.GetUserURLs(userID)
		if err != nil {
			http.Error(w, "Failed to get a list of user's URLs", http.StatusInternalServerError)
			return
		}

		// Respond with 204 StatusNoContent if there is no URLs found in storage.
		if len(records) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Respond with the user's URLs.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(records)
	}
}

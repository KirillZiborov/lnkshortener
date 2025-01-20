package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KirillZiborov/lnkshortener/internal/api/http/auth"
	"github.com/KirillZiborov/lnkshortener/internal/app"
)

// GetHandler handles redirection from a short URL to the original URL.
// It expects a GET request with the short URL.
// Upon finding the original URL, it redirects the client to it with a 302 Found status.
//
// Possible error codes in response:
// - 404 (Not Found) if there is no original URL for the requested short URL.
// - 410 (Gone) if the URL is deleted.
// - 500 (Internal Server Error) if the server fails.
func GetHandler(svc *app.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Extract a shortURL from request parameters.
		id := chi.URLParam(r, "id")

		// Call to GetShortURL from app.
		originalURL, err := svc.GetShortURL(r.Context(), id)
		if errors.Is(err, app.ErrURLNotFound) {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		} else if errors.Is(err, app.ErrURLDeleted) {
			http.Error(w, "URL has been deleted", http.StatusGone)
			return
		} else if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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
func GetUserURLsHandler(svc *app.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Authenticate and get the user ID.
		userID, err := auth.AuthGet(r)
		if err != nil {
			http.Error(w, "Unathorized", http.StatusUnauthorized)
		}

		// Call to GetUserURLs from app.
		records, err := svc.GetUserURLs(r.Context(), userID)
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

// StatsResponse holds a number of shortened URLs and users in the service in JSON format.
type StatsResponse struct {
	// URLs is a number of URLs in the service.
	URLs int `json:"urls"`

	// Users is a number of unique users in the service.
	Users int `json:"users"`
}

// GetStatsHandler checks if an IP address is in trusted subnet.
// It returns server stats if so, else returns 403 status code.
// It expects a GET request and responds with stats in JSON format.
//
// Possible error codes in response:
// - 403 (Forbidden) if IP is not in trusted subnet or no trusted subnet specified.
// - 500 (Internal Server Error) if the server fails.
func GetStatsHandler(svc *app.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read IP from the X-Real-IP header.
		clientIP := r.Header.Get("X-Real-IP")

		// Call to CheckTrustedSubnet from app.
		if err := svc.CheckTrustedSubnet(clientIP); err != nil {
			switch {
			case errors.Is(err, app.ErrNoTrustedSubnet),
				errors.Is(err, app.ErrIPNotInSubnet),
				errors.Is(err, app.ErrNoClientIP):
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			default:
				// CIDR parsing error.
				http.Error(w, "Invalid trusted subnet", http.StatusInternalServerError)
				return
			}
		}

		// Call to GetStats from app.
		urls, users, err := svc.GetStats()
		if err != nil {
			http.Error(w, "Failed to get stats", http.StatusInternalServerError)
			return
		}

		resp := StatsResponse{
			URLs:  urls,
			Users: users,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

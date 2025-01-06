package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
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

		// Check if the URL is deleted.
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
func GetStatsHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If the trusted_subnet field is empty then deny access.
		if cfg.TrustedSubnet == "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Read IP from the X-Real-IP header.
		clientIP := r.Header.Get("X-Real-IP")
		if clientIP == "" {
			http.Error(w, "Forbidden: no X-Real-IP", http.StatusForbidden)
			return
		}

		// Parse trusted_subnet field.
		_, cidrNet, err := net.ParseCIDR(cfg.TrustedSubnet)
		if err != nil {
			http.Error(w, "Invalid trusted subnet", http.StatusInternalServerError)
			return
		}

		// Parse and check IP.
		ip := net.ParseIP(strings.TrimSpace(clientIP))
		if ip == nil || !cidrNet.Contains(ip) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Get stats from storage.
		urlsCount, err := store.GetURLsCount()
		if err != nil {
			http.Error(w, "Failed to get number of URLs", http.StatusInternalServerError)
			return
		}

		usersCount, err := store.GetUsersCount()
		if err != nil {
			http.Error(w, "Failed to get number of users", http.StatusInternalServerError)
			return
		}

		resp := StatsResponse{
			URLs:  urlsCount,
			Users: usersCount,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/file"
)

// PostHandler handles POST request containing the original URL and creates a short URL for it.
// Upon successful creation, it responds with a 201 Created status and the shortened URL.
//
// Possible error codes in response:
// - 400 (Bad Request) if the request body is empty.
// - 401 (Unauthorized) if the authentification token is invalid.
// - 409 (Conflict) if the shortURL already exists for the original URL.
// - 500 (Internal Server Error) if the server fails.
func PostHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read and check request body for a non-empty original URL.
		url, err := io.ReadAll(r.Body)
		if err != nil || len(url) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Authentificate with an existing cookie or create a new one.
		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		// Generate a short URL.
		id := generateID()
		ourl := string(url)
		shortenedURL := cfg.BaseURL + "/" + id

		// Create a structure with information about the URL.
		urlRecord := &file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: ourl,
			UserUUID:    userID,
		}

		// Store the URL info in the file storage or database.
		shortURL, err := store.SaveURLRecord(urlRecord)
		if errors.Is(err, database.ErrorDuplicate) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(shortURL))
			return
		} else if err != nil {
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
			return
		}

		// Update the counter.
		counter++

		// Respond with the shortened URL.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortenedURL))
	}
}

// jsonRequest holds an original URL in JSON format.
type jsonRequest struct {
	URL string `json:"url"`
}

// JSONResponse holds a short URL in JSON format.
type JSONResponse struct {
	Result string `json:"result"`
}

// APIShortenHandler handles the creation of a new shortened URL in JSON format.
// It expects a POST request with a JSON payload containing the original URL.
// Upon successful creation, it responds with a 201 Created status and the shortened URL.
//
// Possible error codes in response:
// - 400 (Bad Request) if the original URL is empty.
// - 401 (Unauthorized) if the authentification token is invalid.
// - 409 (Conflict) if the shortURL already exists for the original URL.
// - 500 (Internal Server Error) if the server fails.
func APIShortenHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req jsonRequest

		// Decode JSON data from the request body and extract an original URL.
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Authentificate with an existing cookie or create a new one.
		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		// Generate a short URL.
		id := generateID()
		shortenedURL := cfg.BaseURL + "/" + id

		res := JSONResponse{
			Result: shortenedURL,
		}

		// Create a structure with information about the URL.
		urlRecord := &file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: req.URL,
			UserUUID:    userID,
		}

		// Store the URL info in the file storage or database.
		shortURL, err := store.SaveURLRecord(urlRecord)
		if errors.Is(err, database.ErrorDuplicate) {
			res := JSONResponse{
				Result: shortURL,
			}
			responseJSON, err := json.Marshal(res)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			w.Write(responseJSON)
			return
		} else if err != nil {
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
			return
		}

		// Update the counter.
		counter++

		// Respond with the shortened URL.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(res); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// BatchRequest holds correlation ID and corresponding original URL in JSON format.
type BatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchResponse holds correlation ID and corresponding short URL in JSON format.
type BatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// BatchShortenHandler handles the creation of multiple shortened URLs in a single request.
// It expects a POST request with a JSON array of original URLs.
// Upon successful creation, it responds with a 201 Created status and an array of shortened URLs.
//
// Possible error codes in response:
// - 400 (Bad Request) if the request body is empty.
// - 401 (Unauthorized) if the authentification token is invalid.
// - 500 (Internal Server Error) if the server fails.
func BatchShortenHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Authentificate with an existing cookie or create a new one.
		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		var batchRequests []BatchRequest

		// Decode JSON data from the request body and extract original URLs.
		err = json.NewDecoder(r.Body).Decode(&batchRequests)
		if err != nil || len(batchRequests) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Initialize a slice of BatchResponse.
		batchResponses := make([]BatchResponse, 0, len(batchRequests))

		// Iterate through all sent URLs.
		for _, req := range batchRequests {

			// Generate a short URL.
			id := generateID()
			shortenedURL := cfg.BaseURL + "/" + id

			// Create a structure with information about the URL.
			urlRecord := &file.URLRecord{
				UUID:        strconv.Itoa(counter),
				ShortURL:    shortenedURL,
				OriginalURL: req.OriginalURL,
				UserUUID:    userID,
			}

			// Store the URL info in the file storage or database.
			_, err := store.SaveURLRecord(urlRecord)
			if err != nil {
				http.Error(w, "Failed to save URL", http.StatusInternalServerError)
				return
			}

			// Add BatchResponse with short URL to the batchResponses slice.
			batchResponses = append(batchResponses, BatchResponse{
				CorrelationID: req.CorrelationID,
				ShortURL:      shortenedURL,
			})

			// Update the counter.
			counter++
		}

		// Respond with the array of shortened URLs.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(batchResponses)
	}
}

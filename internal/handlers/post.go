package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/logic"
)

// PostHandler handles POST request containing the original URL and creates a short URL for it.
// Upon successful creation, it responds with a 201 Created status and the shortened URL.
//
// Possible error codes in response:
// - 400 (Bad Request) if the request body is empty.
// - 401 (Unauthorized) if the authentification token is invalid.
// - 409 (Conflict) if the shortURL already exists for the original URL.
// - 500 (Internal Server Error) if the server fails.
func PostHandler(svc *logic.ShortenerService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read and check request body for a non-empty original URL.
		url, err := io.ReadAll(r.Body)
		if err != nil || len(url) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		originalURL := string(url)

		// Authentificate with an existing cookie or create a new one.
		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		// Call CreateShortURL from logic.
		shortURL, err := svc.CreateShortURL(r.Context(), originalURL, userID)
		if errors.Is(err, database.ErrorDuplicate) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(shortURL))
			return
		} else if err != nil {
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
			return
		}

		// Respond with the shortened URL.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortURL))
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
func APIShortenHandler(svc *logic.ShortenerService) http.HandlerFunc {
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

		// Call to CreateShortURL from logic.
		shortURL, err := svc.CreateShortURL(r.Context(), req.URL, userID)
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

		// Respond with the shortened URL.
		res := JSONResponse{
			Result: shortURL,
		}
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
func BatchShortenHandler(svc *logic.ShortenerService) http.HandlerFunc {
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

		// Convert batchRequests to internal logic structure BatchReq.
		reqs := make([]logic.BatchReq, 0, len(batchRequests))
		for _, br := range batchRequests {
			reqs = append(reqs, logic.BatchReq{
				CorrelationID: br.CorrelationID,
				OriginalURL:   br.OriginalURL,
			})
		}

		// Call to BatchShorten from logic.
		results, err := svc.BatchShorten(r.Context(), userID, reqs)
		if err != nil {
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
			return
		}

		// Initialize a slice of BatchResponse.
		batchResponses := make([]BatchResponse, 0, len(batchRequests))
		// Convert back to BatchResponse with JSON.
		for _, r := range results {
			batchResponses = append(batchResponses, BatchResponse{
				CorrelationID: r.CorrelationID,
				ShortURL:      r.ShortURL,
			})
		}

		// Respond with the array of shortened URLs.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(batchResponses)
	}
}

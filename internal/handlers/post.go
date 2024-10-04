package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/file"
)

func PostHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		url, err := io.ReadAll(r.Body)
		if err != nil || len(url) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		id := generateID()
		ourl := string(url)

		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		urlRecord := &file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: ourl,
			UserUUID:    userID,
		}

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

		counter++

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortenedURL))
	}
}

type jsonRequest struct {
	URL string `json:"url"`
}

type JSONResponse struct {
	Result string `json:"result"`
}

func APIShortenHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req jsonRequest

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		err = json.Unmarshal(body, &req)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		id := generateID()
		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		res := JSONResponse{
			Result: shortenedURL,
		}

		urlRecord := &file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: req.URL,
			UserUUID:    userID,
		}

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

		counter++

		responseJSON, err := json.Marshal(res)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write(responseJSON)
	}
}

type BatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func BatchShortenHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		var batchRequests []BatchRequest
		var batchResponses []BatchResponse

		err = json.NewDecoder(r.Body).Decode(&batchRequests)
		if err != nil || len(batchRequests) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		for _, req := range batchRequests {

			id := generateID()
			shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

			urlRecord := &file.URLRecord{
				UUID:        strconv.Itoa(counter),
				ShortURL:    shortenedURL,
				OriginalURL: req.OriginalURL,
				UserUUID:    userID,
			}

			_, err := store.SaveURLRecord(urlRecord)
			if err != nil {
				http.Error(w, "Failed to save URL", http.StatusInternalServerError)
				return
			}

			batchResponses = append(batchResponses, BatchResponse{
				CorrelationID: req.CorrelationID,
				ShortURL:      shortenedURL,
			})

			counter++
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(batchResponses)
	}
}

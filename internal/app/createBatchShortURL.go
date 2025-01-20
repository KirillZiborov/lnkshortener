package app

import (
	"context"

	"github.com/KirillZiborov/lnkshortener/internal/file"
)

// BatchReq holds correlation ID and corresponding original URL.
type BatchReq struct {
	CorrelationID string
	OriginalURL   string
}

// BatchRes holds correlation ID and corresponding short URL.
type BatchRes struct {
	CorrelationID string
	ShortURL      string
}

// BatchShorten handles a batch of URLs and returns a batch of corresponding short URLs.
func (s *ShortenerService) BatchShorten(ctx context.Context, userID string, requests []BatchReq) ([]BatchRes, error) {
	var results []BatchRes

	// Iterate through all sent URLs.
	for _, req := range requests {
		// Generate a short URL.
		id := generateID()
		shortURL := s.Cfg.BaseURL + "/" + id

		// Create a structure with information about the URL.
		urlRecord := &file.URLRecord{
			UUID:        id,
			ShortURL:    shortURL,
			OriginalURL: req.OriginalURL,
			UserUUID:    userID,
		}

		// Store the URL info in the file storage or database.
		_, err := s.Store.SaveURLRecord(urlRecord)
		if err != nil {
			return nil, err
		}

		// Add BatchRes with short URL to the results slice.
		results = append(results, BatchRes{
			CorrelationID: req.CorrelationID,
			ShortURL:      shortURL,
		})

		// Update the counter.
		Counter++
	}

	return results, nil
}

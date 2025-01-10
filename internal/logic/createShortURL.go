package logic

import (
	"context"
	"errors"
	"strconv"

	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/file"
)

// CreateShortURL reads the original URL, generates an ID, creates a record, and saves it.
// Returns the final short URL or an error.
func (s *ShortenerService) CreateShortURL(ctx context.Context, originalURL, userID string) (string, error) {
	// Generate a short URL.
	id := generateID()
	shortenedURL := s.Cfg.BaseURL + "/" + id

	// Create a structure with information about the URL.
	urlRecord := &file.URLRecord{
		UUID:        strconv.Itoa(Counter),
		ShortURL:    shortenedURL,
		OriginalURL: originalURL,
		UserUUID:    userID,
	}

	// Store the URL info in the file storage or database.
	shortURL, err := s.Store.SaveURLRecord(urlRecord)
	if errors.Is(err, database.ErrorDuplicate) {
		return shortURL, database.ErrorDuplicate
	} else if err != nil {
		return "", err
	}

	// Update the counter
	Counter++

	return shortenedURL, nil
}

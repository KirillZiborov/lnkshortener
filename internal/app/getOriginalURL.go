package app

import (
	"context"
	"errors"
	"fmt"
	"os"
)

var (
	// ErrURLNotFound is returned when a corresponding URL record not found in storage.
	ErrURLNotFound = errors.New("url not found")
	// ErrURLDeleted is returned when attempting to get a URL which is marked as deleted.
	ErrURLDeleted = errors.New("url deleted")
)

// GetShortURL finds the corresponding original URL by its shortened version.
func (s *ShortenerService) GetShortURL(ctx context.Context, shortID string) (originalURL string, err error) {
	// Prepend the base URL to ID to form the complete short URL.
	shortURL := fmt.Sprintf("%s/%s", s.Cfg.BaseURL, shortID)

	// Get the original URL by the short URL.
	orig, deleted, e := s.Store.GetOriginalURL(shortURL)
	if e != nil {
		// Get os.ErrProcessDone if the storage is fully checked but URL is not found.
		if errors.Is(e, os.ErrProcessDone) {
			return "", ErrURLNotFound
		}
		return "", e
	}

	// Check if the URL is deleted.
	if deleted {
		return orig, ErrURLDeleted
	}
	return orig, nil
}

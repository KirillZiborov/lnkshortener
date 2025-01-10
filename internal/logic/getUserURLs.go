package logic

import (
	"context"

	"github.com/KirillZiborov/lnkshortener/internal/file"
)

// GetUserURLs returns all non-deleted short URLs created by user.
func (s *ShortenerService) GetUserURLs(ctx context.Context, userID string) ([]file.URLRecord, error) {
	// Retrieve the user's URLs from the storage.
	records, err := s.Store.GetUserURLs(userID)
	if err != nil {
		return nil, err
	}

	return records, nil
}

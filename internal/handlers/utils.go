package handlers

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/KirillZiborov/lnkshortener/internal/file"
)

var (
	// counter helps to store URLs UUIDs.
	// It increments every time adding a new URL to the storage.
	counter = 1
)

// URLStore defines the interface for URL storage operations.
// It abstracts the underlying storage mechanism (database or file-based).
type URLStore interface {
	// SaveURLRecord saves a new URLRecord to the storage.
	// If the original URL already exists, it returns the existing short URL and an error indicating duplication.
	//
	// Parameters:
	// - urlRecord: A pointer to the URLRecord to be saved.
	//
	// Returns:
	// - The short URL string if the insertion is successful.
	// - An error if the insertion fails or if the URL already exists.
	SaveURLRecord(urlRecord *file.URLRecord) (string, error)

	// GetOriginalURL retrieves the original URL and its deletion status based on the provided short URL.
	//
	// Parameters:
	// - shortURL: The short URL identifier to look up.
	//
	// Returns:
	// - The corresponding original URL string if found.
	// - A boolean indicating whether the URL has been marked as deleted.
	// - An error if the short URL does not exist or if the query fails.
	GetOriginalURL(shortURL string) (string, bool, error)

	// GetUserURLs retrieves all URL records associated with a specific user ID.
	//
	// Parameters:
	// - userID: The user ID whose URLs are to be retrieved.
	//
	// Returns:
	// - A slice of URLRecord containing the user's URLs.
	// - An error if the query fails.
	GetUserURLs(userID string) ([]file.URLRecord, error)

	// BatchUpdateDeleteFlag marks multiple URL records as deleted based on the provided URL ID and user ID.
	//
	// Parameters:
	// - urlID: The UUID of the URL record to be marked as deleted.
	// - userID: The user ID associated with the URL record.
	//
	// Returns:
	// - An error if the update operation fails.
	BatchUpdateDeleteFlag(urlID string, userID string) error
}

// generateID is a helper function to generate a shortened URL.
// It generates and returns 8-character string.
func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

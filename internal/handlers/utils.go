package handlers

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/KirillZiborov/lnkshortener/internal/file"
)

var (
	counter = 1
)

type URLStore interface {
	SaveURLRecord(urlRecord *file.URLRecord) (string, error)
	GetOriginalURL(shortURL string) (string, bool, error)
	GetUserURLs(userID string) ([]file.URLRecord, error)
	BatchUpdateDeleteFlag(urlID string, userID string) error
}

func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

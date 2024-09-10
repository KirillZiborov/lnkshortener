package database

import (
	"context"
	"fmt"

	"github.com/KirillZiborov/lnkshortener/internal/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateURLTable(ctx context.Context, db *pgxpool.Pool) error {
	query := `
    CREATE TABLE IF NOT EXISTS urls (
        id TEXT PRIMARY KEY,
        short_url TEXT NOT NULL,
        original_url TEXT NOT NULL
    );
    `
	_, err := db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("unable to create table: %w", err)
	}
	return nil
}

type DBStore struct {
	db *pgxpool.Pool
}

func NewDBStore(db *pgxpool.Pool) *DBStore {
	return &DBStore{db: db}
}

func (store *DBStore) SaveURLRecord(urlRecord *file.URLRecord) error {
	query := `INSERT INTO urls (id, short_url, original_url) VALUES ($1, $2, $3)`

	_, err := store.db.Exec(context.Background(), query, urlRecord.UUID, urlRecord.ShortURL, urlRecord.OriginalURL)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}

	return nil
}

func (store *DBStore) GetOriginalURL(shortURL string) (string, error) {
	var originalURL string

	query := `SELECT original_url FROM urls WHERE short_url = $1`
	err := store.db.QueryRow(context.Background(), query, shortURL).Scan(&originalURL)
	if err != nil {
		return "", err
	}
	return originalURL, nil
}
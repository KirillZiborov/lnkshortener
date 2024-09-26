package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirillZiborov/lnkshortener/internal/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateURLTable(ctx context.Context, db *pgxpool.Pool) error {
	query := `
    CREATE TABLE IF NOT EXISTS urls (
        id SERIAL PRIMARY KEY,
        short_url TEXT NOT NULL,
        original_url TEXT NOT NULL,
		user_id TEXT NOT NULL
    );
	CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_original_url ON urls (original_url);
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

var ErrorDuplicate = errors.New("duplicate entry: URL already exists")

func (store *DBStore) SaveURLRecord(urlRecord *file.URLRecord) (string, error) {
	query := `INSERT INTO urls (short_url, original_url, user_id) 
			  VALUES ($1, $2, $3)
			  ON CONFLICT (original_url) DO NOTHING`

	c, err := store.db.Exec(context.Background(), query, urlRecord.ShortURL, urlRecord.OriginalURL, urlRecord.UserUUID)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}

	if c.RowsAffected() == 0 {
		existingShortURL, err := store.GetShortURL(urlRecord.OriginalURL)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return "", err
		}
		return existingShortURL, ErrorDuplicate
	}

	return urlRecord.ShortURL, nil
}

func (store *DBStore) GetShortURL(originalURL string) (string, error) {
	var shortURL string

	query := `SELECT short_url FROM urls WHERE original_url = $1`
	err := store.db.QueryRow(context.Background(), query, originalURL).Scan(&shortURL)
	if err != nil {
		return "", err
	}
	return shortURL, nil
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

func (store *DBStore) GetUserURLs(userId string) ([]file.URLRecord, error) {
	var records []file.URLRecord

	query := `SELECT short_url, original_url FROM urls WHERE user_id = $1`
	rows, err := store.db.Query(context.Background(), query, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var shortURL, originalURL string

		err := rows.Scan(&shortURL, &originalURL)
		if err != nil {
			return nil, err
		}

		records = append(records, file.URLRecord{
			ShortURL:    shortURL,
			OriginalURL: originalURL,
		})
	}
	return records, rows.Err()
}

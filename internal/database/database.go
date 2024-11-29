// Package database provides functionalities to interact with the PostgreSQL database.
// It includes functions to create necessary tables, and methods to perform operations on URL records.
package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KirillZiborov/lnkshortener/internal/file"
)

// CreateURLTable initializes the 'urls' table in the PostgreSQL database if it does not already exist.
// It defines the schema for storing shortened URLs, including indexes.
//
// Parameters:
// - ctx: The context for managing cancellation and timeouts.
// - db: The PostgreSQL connection pool.
//
// Returns:
// - An error if the table creation fails; otherwise, nil.
func CreateURLTable(ctx context.Context, db *pgxpool.Pool) error {
	query := `
    CREATE TABLE IF NOT EXISTS urls (
        id SERIAL PRIMARY KEY,
        short_url TEXT NOT NULL,
        original_url TEXT NOT NULL,
		user_id TEXT NOT NULL,
		deleted BOOL NOT NULL
    );
	CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_original_url ON urls (original_url);
    `
	_, err := db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("unable to create table: %w", err)
	}
	return nil
}

// DBStore represents a database store for URL records.
// It encapsulates the PostgreSQL connection pool to perform database operations.
type DBStore struct {
	db *pgxpool.Pool
}

// NewDBStore initializes and returns a pointer to a new instance of DBStore.
func NewDBStore(db *pgxpool.Pool) *DBStore {
	return &DBStore{db: db}
}

// ErrorDuplicate is returned when attempting to insert a URL record that already exists.
// It indicates that the original URL has already been shortened and stored.
var ErrorDuplicate = errors.New("duplicate entry: URL already exists")

// SaveURLRecord inserts a new URLRecord into the database.
// If the original URL already exists, it retrieves and returns the existing short URL.
//
// Parameters:
// - urlRecord: A pointer to the URLRecord to be saved.
//
// Returns:
// - The short URL string if the insertion is successful.
// - An error if the insertion fails or if the URL already exists.
func (store *DBStore) SaveURLRecord(urlRecord *file.URLRecord) (string, error) {
	query := `INSERT INTO urls (short_url, original_url, user_id, deleted) 
			  VALUES ($1, $2, $3, $4)
			  ON CONFLICT (original_url) DO NOTHING`

	c, err := store.db.Exec(context.Background(), query, urlRecord.ShortURL, urlRecord.OriginalURL, urlRecord.UserUUID, urlRecord.DeletedFlag)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return "", err
	}

	// Check if the short URL already exists for the given original URL.
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

// GetShortURL retrieves the short URL associated with the given original URL.
//
// Parameters:
// - originalURL: The original URL to look up.
//
// Returns:
// - The corresponding short URL string if found.
// - An error if the original URL does not exist or if the query fails.
func (store *DBStore) GetShortURL(originalURL string) (string, error) {
	var shortURL string

	query := `SELECT short_url FROM urls WHERE original_url = $1`
	err := store.db.QueryRow(context.Background(), query, originalURL).Scan(&shortURL)
	if err != nil {
		return "", err
	}
	return shortURL, nil
}

// GetOriginalURL retrieves the original URL and its deletion status based on the provided short URL.
//
// Parameters:
// - shortURL: The short URL to look up.
//
// Returns:
// - The corresponding original URL string.
// - A boolean indicating whether the URL has been marked as deleted.
// - An error if the short URL does not exist or if the query fails.
func (store *DBStore) GetOriginalURL(shortURL string) (string, bool, error) {
	var originalURL string
	var deleted bool

	query := `SELECT original_url, deleted FROM urls WHERE short_url = $1`
	err := store.db.QueryRow(context.Background(), query, shortURL).Scan(&originalURL, &deleted)
	if err != nil {
		return "", false, err
	}
	return originalURL, deleted, nil
}

// GetUserURLs retrieves all URL records associated with a given user ID.
//
// Parameters:
// - userID: The user ID whose URLs are to be retrieved.
//
// Returns:
// - A slice of URLRecord containing the user's URLs.
// - An error if the query fails.
func (store *DBStore) GetUserURLs(userID string) ([]file.URLRecord, error) {
	var records []file.URLRecord

	query := `SELECT short_url, original_url FROM urls WHERE user_id = $1`
	rows, err := store.db.Query(context.Background(), query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Iterates through all found rows.
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

// BatchUpdateDeleteFlag marks multiple URL records as deleted based on the provided short URL and user ID.
//
// Parameters:
// - urlID: The short URL identifier to be marked as deleted.
// - userID: The user ID associated with the URL.
//
// Returns:
// - An error if the update operation fails.
func (store *DBStore) BatchUpdateDeleteFlag(urlID string, userID string) error {
	query := `UPDATE urls SET deleted = TRUE WHERE short_url = $1 AND user_id = $2`
	_, err := store.db.Exec(context.Background(), query, urlID, userID)
	return err
}

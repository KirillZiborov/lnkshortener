// Package file provides functionalities to manage URL records using file-based storage.
// It includes structures and methods for creating, reading, updating, and deleting URL records.
package file

import (
	"encoding/json"
	"os"
)

// URLRecord represents a single URL mapping in the storage system.
// It contains information about the shortened URL, the original URL, the associated user,
// and a flag indicating whether the URL has been deleted.
type URLRecord struct {
	UUID        string `json:"uuid"`         // UUID uniquely identifies the URL record.
	ShortURL    string `json:"short_url"`    // ShortURL is the shortened version of the original URL.
	OriginalURL string `json:"original_url"` // OriginalURL is the original, long-form URL.
	UserUUID    string `json:"user_uuid"`    // UserUUID associates the URL with a specific user.
	DeletedFlag bool   `json:"deleted"`      // DeletedFlag indicates whether the URL has been marked as deleted.
}

// Producer is responsible for writing URL records to a file.
// It maintains a file handle and a JSON encoder for efficient encoding and writing.
type Producer struct {
	File    *os.File      // File is the file pointer where URL records are stored.
	encoder *json.Encoder // encoder encodes URLRecord structs into JSON format.
}

// Consumer is responsible for reading URL records from a file.
// It maintains a file handle and a JSON decoder for efficient decoding and reading.
type Consumer struct {
	File    *os.File      // File is the file pointer from which URL records are read.
	decoder *json.Decoder // decoder decodes JSON data into URLRecord structs.
}

// NewProducer initializes a new Producer for the specified file.
// It opens the file in write-only mode, creates it if it does not exist,
// and appends to it if it does. It also initializes a JSON encoder for the file.
//
// Parameters:
// - fileName: The name of the file where URL records will be written.
//
// Returns:
// - A pointer to a Producer instance.
// - An error if the file cannot be opened or created.
func NewProducer(fileName string) (*Producer, error) {
	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &Producer{
		File:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

// WriteURLRecord encodes and writes a single URLRecord to the file.
// It appends the JSON-encoded URLRecord to the file.
//
// Parameters:
// - urlRecord: A pointer to the URLRecord to be written.
//
// Returns:
// - An error if encoding or writing to the file fails.
func (p *Producer) WriteURLRecord(URLRecord *URLRecord) error {
	return p.encoder.Encode(&URLRecord)
}

// NewConsumer initializes a new Consumer for the specified file.
// It opens the file in read-only mode, creates it if it does not exist,
// and initializes a JSON decoder for the file.
//
// Parameters:
// - fileName: The name of the file from which URL records will be read.
//
// Returns:
// - A pointer to a Consumer instance.
// - An error if the file cannot be opened or created.
func NewConsumer(fileName string) (*Consumer, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		File:    file,
		decoder: json.NewDecoder(file),
	}, nil
}

// ReadURLRecord decodes and reads a single URLRecord from the file.
// It returns the next URLRecord in the file.
//
// Returns:
// - A pointer to the URLRecord read from the file.
// - An error if decoding fails or if the end of the file is reached.
func (c *Consumer) ReadURLRecord() (*URLRecord, error) {
	URLRecord := &URLRecord{}
	if err := c.decoder.Decode(&URLRecord); err != nil {
		return nil, err
	}

	return URLRecord, nil
}

// FindOriginalURLByShortURL searches for the original URL corresponding to a given short URL.
// It iterates through all URL records in the specified file to find a match.
//
// Parameters:
// - shortURL: The short URL to search for.
// - fileName: The name of the file containing URL records.
//
// Returns:
// - The original URL if found.
// - A boolean indicating whether the URL was found.
// - An error if file operations fail.
func FindOriginalURLByShortURL(shortURL, fileName string) (string, bool, error) {
	// Initialize a new Consumer for the file storage.
	consumer, err := NewConsumer(fileName)
	if err != nil {
		return "", false, err
	}
	defer consumer.File.Close()

	// Iterate through all URL records in the specified file to find a match.
	for {
		rec, err := consumer.ReadURLRecord()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return "", false, err
		}

		if rec.ShortURL == shortURL {
			return rec.OriginalURL, rec.DeletedFlag, nil
		}
	}

	return "", false, os.ErrProcessDone
}

// SaveURLRecord writes a single URLRecord to the specified file.
// It initializes a Producer and delegates the writing process.
//
// Parameters:
// - url: A pointer to the URLRecord to be saved.
// - fileName: The name of the file where the URLRecord will be written.
//
// Returns:
// - An error if the Producer cannot be initialized or if writing fails.
func SaveURLRecord(url *URLRecord, fileName string) error {
	producer, err := NewProducer(fileName)
	if err != nil {
		return err
	}
	defer producer.File.Close()

	return producer.WriteURLRecord(url)
}

// FileStore provides a file-based implementation of the URLStore interface.
// It manages URL records by reading from and writing to a specified file.
type FileStore struct {
	fileName string // fileName is the path to the file used for storing URL records.
}

// NewFileStore initializes and returns a new FileStore for the specified file.
// It sets the file name for future read and write operations.
func NewFileStore(fileName string) *FileStore {
	return &FileStore{fileName: fileName}
}

// SaveURLRecord saves a URLRecord to the file.
// It delegates the saving process to the SaveURLRecord function.
func (store *FileStore) SaveURLRecord(urlRecord *URLRecord) (string, error) {
	return "", SaveURLRecord(urlRecord, store.fileName)
}

// GetOriginalURL retrieves the original URL and its deletion status based on the provided short URL.
// It delegates the retrieval process to the FindOriginalURLByShortURL function.
func (store *FileStore) GetOriginalURL(shortURL string) (string, bool, error) {
	return FindOriginalURLByShortURL(shortURL, store.fileName)
}

// GetUserURLs retrieves all URL records associated with a specific user ID.
// It iterates through the file to collect URLs belonging to the given user.
//
// Parameters:
// - userID: The user ID whose URLs are to be retrieved.
//
// Returns:
// - A slice of URLRecord containing the user's URLs.
// - An error if file operations fail.
func (store *FileStore) GetUserURLs(userID string) ([]URLRecord, error) {
	var records []URLRecord

	// Initiate a new consumer.
	consumer, err := NewConsumer(store.fileName)
	if err != nil {
		return nil, err
	}
	defer consumer.File.Close()

	// Iterate through the file to collect URLs belonging to the given user.
	for {
		rec, err := consumer.ReadURLRecord()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}

		if rec.UserUUID == userID {
			records = append(records, URLRecord{
				ShortURL:    rec.ShortURL,
				OriginalURL: rec.OriginalURL,
			})
		}
	}

	return records, nil
}

// BatchUpdateDeleteFlag marks multiple URL records as deleted based on the provided URL ID and user ID.
// It reads all records, updates the deletion flag where applicable, and rewrites the entire file.
//
// Parameters:
// - urlID: The UUID of the URL record to be marked as deleted.
// - userID: The user ID associated with the URL record.
//
// Returns:
// - An error if reading or writing records fails.
func (store *FileStore) BatchUpdateDeleteFlag(urlID string, userID string) error {
	// Initiate a new consumer.
	consumer, err := NewConsumer(store.fileName)
	if err != nil {
		return err
	}
	defer consumer.File.Close()

	var updatedRecords []URLRecord
	//Read all records, updates the deletion flag where applicable.
	for {
		rec, err := consumer.ReadURLRecord()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return err
		}

		if rec.UUID == urlID && rec.UserUUID == userID {
			rec.DeletedFlag = true
		}

		updatedRecords = append(updatedRecords, *rec)
	}

	return store.SaveAllRecords(updatedRecords)
}

// SaveAllRecords writes all provided URLRecords to the file.
// It overwrites the existing file with the new set of records.
//
// Parameters:
// - records: A slice of URLRecord to be saved.
//
// Returns:
// - An error if the Producer cannot be initialized or if writing any record fails.
func (store *FileStore) SaveAllRecords(records []URLRecord) error {
	// Initiate a producer.
	producer, err := NewProducer(store.fileName)
	if err != nil {
		return err
	}
	defer producer.File.Close()

	// Overwrite the existing file with the new set of records.
	for _, record := range records {
		if err := producer.WriteURLRecord(&record); err != nil {
			return err
		}
	}

	return nil
}

// GetURLsCount counts shortened URLs in the file.
//
// Returns:
// - A number of shortened URLs.
// - An error if reading fails.
func (store *FileStore) GetURLsCount() (int, error) {
	consumer, err := NewConsumer(store.fileName)
	if err != nil {
		return 0, err
	}
	defer consumer.File.Close()

	count := 0
	for {
		_, err := consumer.ReadURLRecord()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return 0, err
		}
		count++
	}

	return count, nil
}

// GetUsersCount counts unique users
// by reading all records in the file and collecting userUUID.
//
// Returns:
// - A number of unique users.
// - An error if reading fails.
func (store *FileStore) GetUsersCount() (int, error) {
	consumer, err := NewConsumer(store.fileName)
	if err != nil {
		return 0, err
	}
	defer consumer.File.Close()

	usersSet := make(map[string]struct{})
	for {
		rec, err := consumer.ReadURLRecord()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return 0, err
		}

		usersSet[rec.UserUUID] = struct{}{}
	}

	return len(usersSet), nil
}

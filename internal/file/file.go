package file

import (
	"encoding/json"
	"os"
)

type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserUUID    string `json:"user_uuid"`
}

var URLs []URLRecord

type Producer struct {
	File    *os.File
	encoder *json.Encoder
}

type Consumer struct {
	File    *os.File
	decoder *json.Decoder
}

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

func (p *Producer) WriteURLRecord(URLRecord *URLRecord) error {
	return p.encoder.Encode(&URLRecord)
}

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

func (c *Consumer) ReadURLRecord() (*URLRecord, error) {
	URLRecord := &URLRecord{}
	if err := c.decoder.Decode(&URLRecord); err != nil {
		return nil, err
	}

	return URLRecord, nil
}

func FindOriginalURLByShortURL(shortURL, fileName string) (string, error) {
	consumer, err := NewConsumer(fileName)
	if err != nil {
		return "", err
	}
	defer consumer.File.Close()

	for {
		rec, err := consumer.ReadURLRecord()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return "", err
		}

		if rec.ShortURL == shortURL {
			return rec.OriginalURL, nil
		}
	}

	return "", os.ErrProcessDone
}

func SaveURLRecord(url *URLRecord, fileName string) error {
	producer, err := NewProducer(fileName)
	if err != nil {
		return err
	}
	defer producer.File.Close()

	return producer.WriteURLRecord(url)
}

type FileStore struct {
	fileName string
}

func NewFileStore(fileName string) *FileStore {
	return &FileStore{fileName: fileName}
}

func (store *FileStore) SaveURLRecord(urlRecord *URLRecord) (string, error) {
	return "", SaveURLRecord(urlRecord, store.fileName)
}

func (store *FileStore) GetOriginalURL(shortURL string) (string, error) {
	return FindOriginalURLByShortURL(shortURL, store.fileName)
}

func (store *FileStore) GetUserURLs(userId string) ([]URLRecord, error) {
	var records []URLRecord

	consumer, err := NewConsumer(store.fileName)
	if err != nil {
		return nil, err
	}
	defer consumer.File.Close()

	for {
		rec, err := consumer.ReadURLRecord()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}

		if rec.UserUUID == userId {
			records = append(records, URLRecord{
				ShortURL:    rec.ShortURL,
				OriginalURL: rec.OriginalURL,
			})
		}
	}

	return records, nil
}

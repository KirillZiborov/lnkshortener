package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/file"
)

func NewTestConfig() *config.Config {
	return &config.Config{
		Address:  "localhost:8080",
		BaseURL:  "http://localhost:8080",
		FilePath: "test_URLstorage.json",
		DBPath:   "",
	}
}

func BenchmarkPostHandler(b *testing.B) {
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)
	handler := PostHandler(*cfg, urlStore)

	requestBody := []byte("https://ya.ru")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "text/plain; charset=utf-8")

		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusCreated {
			b.Errorf("unexpected status code: %d", w.Code)
		}
	}
}

func BenchmarkAPIShortenHandler(b *testing.B) {
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)
	handler := APIShortenHandler(*cfg, urlStore)

	requestBody := []byte(`{"url":"https://ya.ru"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json; charset=utf-8")

		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusCreated && w.Code != http.StatusConflict && w.Code != http.StatusOK {
			b.Errorf("Unexpected status code: %d", w.Code)
		}
	}
}

func BenchmarkBatchShortenHandler(b *testing.B) {
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)
	handler := BatchShortenHandler(*cfg, urlStore)

	batchRequest := []BatchRequest{
		{OriginalURL: "https://ya1.ru", CorrelationID: "cor1"},
		{OriginalURL: "https://ya.ru", CorrelationID: "cor2"},
		{OriginalURL: "http://example.com", CorrelationID: "cor3"},
	}

	requestBody, err := json.Marshal(batchRequest)
	if err != nil {
		b.Fatalf("failed to marshal request body: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusCreated {
			b.Errorf("Unexpected status code: %d", w.Code)
		}
	}
}

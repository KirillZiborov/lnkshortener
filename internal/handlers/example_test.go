package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/KirillZiborov/lnkshortener/internal/file"
)

// ExamplePostHandler demonstrates how to use the PostHandler.
func ExamplePostHandler() {
	// Initialize the configuration.
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)

	// Initialize the handler.
	handler := PostHandler(*cfg, urlStore)

	// Create HTTP request.
	requestBody := strings.NewReader("https://ya.ru")
	req, err := http.NewRequest(http.MethodPost, "/", requestBody)
	if err != nil {
		panic(err)
	}

	// Create ResponseRecorder to record response.
	rr := httptest.NewRecorder()

	// Serve the HTTP request.
	handler(rr, req)

	// Output the response status code.
	fmt.Println(rr.Code) // Output: 201
}

// ExampleAPIShortenHandler demonstrates how to use the APIShortenHandler.
func ExampleAPIShortenHandler() {
	// Initialize the configuration.
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)

	// Initialize the handler.
	handler := APIShortenHandler(*cfg, urlStore)

	// Prepare request data.
	requestData := jsonRequest{
		URL: "http://ya.ru",
	}
	requestBody, _ := json.Marshal(requestData)

	// Create HTTP request.
	req, err := http.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(requestBody))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create ResponseRecorder to record response.
	rr := httptest.NewRecorder()

	// Serve the HTTP request.
	handler(rr, req)

	// Output the response status code.
	fmt.Println(rr.Code) // Output: 201
}

// ExampleBatchShortenHandler demonstrates how to use the BatchShortenHandler.
func ExampleBatchShortenHandler() {
	// Initialize the configuration.
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)

	// Initialize the handler.
	handler := BatchShortenHandler(*cfg, urlStore)

	// Prepare request data.
	batchRequest := []BatchRequest{
		{OriginalURL: "https://ya1.ru", CorrelationID: "cor1"},
		{OriginalURL: "https://ya.ru", CorrelationID: "cor2"},
		{OriginalURL: "http://example.com", CorrelationID: "cor3"},
	}

	requestBody, _ := json.Marshal(batchRequest)

	// Create HTTP request.
	req, err := http.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(requestBody))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create ResponseRecorder to record response.
	rr := httptest.NewRecorder()

	// Serve the HTTP request.
	handler(rr, req)

	// Output the response status code.
	fmt.Println(rr.Code) // Output: 201
}

// ExampleGetHandler demonstrates how to use the GetHandler.
func ExampleGetHandler() {
	// Initialize the configuration.
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)

	// Initialize the handler.
	handler := GetHandler(*cfg, urlStore)

	// Prepare HTTP request.
	req := httptest.NewRequest(http.MethodGet, "/123", nil)

	// Create ResponseRecorder to record response.
	rr := httptest.NewRecorder()

	// Serve the HTTP request.
	handler(rr, req)

	// Output the response status code.
	fmt.Println(rr.Code) // Output: 404
}

// ExampleGetUserURLsHandler demonstrates how to use the GetUserURLsHandler.
func ExampleGetUserURLsHandler() {
	// Initialize the configuration.
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)

	// Initialize the handler.
	handler := GetUserURLsHandler(urlStore)

	// Prepare HTTP request.
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)

	// Set Authorization header.
	req.Header.Set("Authorization", "token")

	// Create ResponseRecorder to record response.
	rr := httptest.NewRecorder()

	// Serve the HTTP request.
	handler(rr, req)

	// Output the response status code.
	fmt.Println(rr.Code) // Output: 401
}

// ExampleBatchDeleteHandler demonstrates how to use the BatchDeleteHandler.
func ExampleBatchDeleteHandler() {
	// Initialize the configuration.
	cfg := NewTestConfig()
	urlStore := file.NewFileStore(cfg.FilePath)

	// Initialize the handler.
	handler := BatchDeleteHandler(*cfg, urlStore)

	// Prepare request data.
	urls := []string{"/abc", "/def", "yaya"}
	requestBody, _ := json.Marshal(urls)

	// Prepare HTTP request.
	req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")

	// Create ResponseRecorder to record response.
	rr := httptest.NewRecorder()

	// Serve the HTTP request.
	handler(rr, req)

	// Output the response status code.
	fmt.Println(rr.Code) // Output: 401
}

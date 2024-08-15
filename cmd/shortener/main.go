package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/KirillZiborov/lnkshortener/cmd/shortener/config"
	"github.com/go-chi/chi"
)

var urlStore = make(map[string]string)

func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

func PostHandler(baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// if r.Method != http.MethodPost {
		// 	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		// 	return
		// }

		url, err := io.ReadAll(r.Body)
		if err != nil || len(url) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		id := generateID()
		urlStore[id] = string(url)

		shortenedURL := fmt.Sprintf("%s/%s", baseURL, id)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortenedURL))
	}
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	// if r.Method != http.MethodGet {
	// 	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	// 	return
	// }

	id := chi.URLParam(r, "id")

	originalURL, exists := urlStore[id]

	if !exists {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func main() {
	cfg := config.NewConfig()

	r := chi.NewRouter()

	r.Post("/", PostHandler(cfg.BaseURL))
	r.Get("/{id}", GetHandler)

	fmt.Printf("Server is running at %s\n", cfg.Address)

	err := http.ListenAndServe(cfg.Address, r)
	if err != nil {
		panic(err)
	}
}

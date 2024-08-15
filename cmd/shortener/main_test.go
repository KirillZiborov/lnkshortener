package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KirillZiborov/lnkshortener/cmd/shortener/config"
	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	cfg := &config.Config{
		Address: "localhost:8080",
		BaseURL: "http://localhost:8000",
	}

	r := chi.NewRouter()

	r.Post("/", PostHandler(cfg.BaseURL))
	r.Get("/{id}", GetHandler)

	type want struct {
		code          int
		headerMatches map[string]string
	}

	tests := []struct {
		name       string
		method     string
		url        string
		body       string
		want       want
		setupStore func()
	}{
		{
			name:   "POST 201",
			method: http.MethodPost,
			url:    "/",
			body:   "https://ya.ru",
			want: want{
				code: http.StatusCreated,
			},
		},
		{
			name:   "POST 400",
			method: http.MethodPost,
			url:    "/",
			body:   "",
			want: want{
				code: http.StatusBadRequest,
			},
		},
		{
			name:   "GET 307",
			method: http.MethodGet,
			url:    "/id",
			want: want{
				code: http.StatusTemporaryRedirect,
				headerMatches: map[string]string{
					"Location": "https://ya.ru",
				},
			},
			setupStore: func() {
				urlStore["id"] = "https://ya.ru"
			},
		},
		{
			name:   "GET 404",
			method: http.MethodGet,
			url:    "/nonexistentID",
			want: want{
				code: http.StatusNotFound,
			},
		},
		{
			name:   "GET 405",
			method: http.MethodPost,
			url:    "/id",
			want: want{
				code: http.StatusMethodNotAllowed,
			},
			setupStore: func() {
				urlStore["id"] = "https://ya.ru"
			},
		},
		{
			name:   "POST 405",
			method: http.MethodGet,
			url:    "/",
			want: want{
				code: http.StatusMethodNotAllowed,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupStore != nil {
				tc.setupStore()
			}

			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			rw := httptest.NewRecorder()

			r.ServeHTTP(rw, req)

			// Проверка код ответа
			assert.Equal(t, tc.want.code, rw.Code)

			// Получаем и проверяем тело запроса
			res := rw.Result()
			defer res.Body.Close()
			_, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			// Проверка заголовков
			for key, value := range tc.want.headerMatches {
				assert.Equal(t, value, res.Header.Get(key))
			}

			// Проверяем, что URL был правильно сохранен в urlStore при POST-запросе
			if tc.method == http.MethodPost && rw.Code == http.StatusCreated {
				shortenedURL := strings.TrimPrefix(rw.Body.String(), cfg.BaseURL+"/")
				originalURL, exists := urlStore[shortenedURL]
				assert.True(t, exists)
				assert.Equal(t, tc.body, originalURL)
			}
		})
	}
}
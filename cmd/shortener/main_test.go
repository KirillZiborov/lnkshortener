package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirillZiborov/lnkshortener/internal/api/http/handlers"
	"github.com/KirillZiborov/lnkshortener/internal/app"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/file"
)

func createTestFile(t *testing.T, fileName string) {
	file, err := os.Create(fileName)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Errorf("Failed to close file: %v", err)
	}
}

func TestServer(t *testing.T) {
	cfg := &config.Config{
		Address:  "localhost:8080",
		BaseURL:  "http://localhost:8080",
		FilePath: "test_file.json",
	}

	urlStore = file.NewFileStore(cfg.FilePath)

	createTestFile(t, cfg.FilePath)

	service := app.ShortenerService{
		Store: urlStore,
		Cfg:   cfg,
	}

	r := chi.NewRouter()

	r.Post("/", handlers.PostHandler(&service))
	r.Get("/{id}", handlers.GetHandler(&service))
	r.Post("/api/shorten", handlers.APIShortenHandler(&service))

	type want struct {
		headerMatches map[string]string
		body          string
		code          int
	}

	tests := []struct {
		name       string
		method     string
		url        string
		body       string
		setupStore func()
		want       want
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
				urlRecord := &file.URLRecord{
					UUID:        "id",
					ShortURL:    cfg.BaseURL + "/id",
					OriginalURL: "https://ya.ru",
				}

				err := file.SaveURLRecord(urlRecord, cfg.FilePath)
				require.NoError(t, err)
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
				urlRecord := &file.URLRecord{
					UUID:        "id",
					ShortURL:    cfg.BaseURL + "/id",
					OriginalURL: "https://ya.ru",
				}

				err := file.SaveURLRecord(urlRecord, cfg.FilePath)
				require.NoError(t, err)
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
		{
			name:   "POST json 201",
			method: http.MethodPost,
			url:    "/api/shorten",
			body:   `{"url":"https://practicum.yandex.ru"}`,
			want: want{
				code: http.StatusCreated,
				body: `{"result":"http://localhost:8080/`,
			},
		},
		{
			name:   "POST json 400",
			method: http.MethodPost,
			url:    "/api/shorten",
			body:   `{"invalid_json"}`,
			want: want{
				code: http.StatusBadRequest,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			createTestFile(t, cfg.FilePath)

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
			defer func() {
				if err := res.Body.Close(); err != nil {
					t.Errorf("Failed to close response body: %v", err)
				}
			}()
			respBody, err := io.ReadAll(res.Body)
			require.NoError(t, err)

			if tc.want.body != "" {
				assert.Contains(t, string(respBody), tc.want.body)
			}

			// Проверка заголовков
			for key, value := range tc.want.headerMatches {
				assert.Equal(t, value, res.Header.Get(key))
			}

			// Проверяем, что URL был правильно сохранен в urlStore при POST-запросе
			if tc.method == http.MethodPost && rw.Code == http.StatusCreated && tc.url == "/" {
				shortenedURL := rw.Body.String()

				var consumer *file.Consumer
				consumer, err = file.NewConsumer(cfg.FilePath)
				require.NoError(t, err)
				defer func() {
					if err = consumer.File.Close(); err != nil {
						t.Errorf("Failed to close file: %v", err)
					}
				}()

				var foundRecord *file.URLRecord
				for {
					var record *file.URLRecord
					record, err = consumer.ReadURLRecord()
					if err != nil {
						break
					}
					if record.ShortURL == shortenedURL {
						foundRecord = record
						break
					}
				}

				assert.NotNil(t, foundRecord)
				assert.Equal(t, tc.body, foundRecord.OriginalURL)
			}

			if tc.method == http.MethodPost && rw.Code == http.StatusCreated && tc.url == "/api/shorten" {
				var jsonResp handlers.JSONResponse
				err = json.Unmarshal(respBody, &jsonResp)
				require.NoError(t, err)

				shortenedURL := jsonResp.Result

				consumer, err := file.NewConsumer(cfg.FilePath)
				require.NoError(t, err)
				defer func() {
					if err = consumer.File.Close(); err != nil {
						t.Errorf("Failed to close file: %v", err)
					}
				}()

				var foundRecord *file.URLRecord
				for {
					record, err := consumer.ReadURLRecord()
					if err != nil {
						break
					}
					if record.ShortURL == shortenedURL {
						foundRecord = record
						break
					}
				}

				assert.NotNil(t, foundRecord)
				assert.Equal(t, "https://practicum.yandex.ru", foundRecord.OriginalURL)
			}
		})
	}
}

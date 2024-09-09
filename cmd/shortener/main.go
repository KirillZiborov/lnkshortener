package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/file"
	"github.com/KirillZiborov/lnkshortener/internal/gzip"
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var (
	sugar   zap.SugaredLogger
	counter = 1
	db      *pgxpool.Pool
)

func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

func PostHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		url, err := io.ReadAll(r.Body)
		if err != nil || len(url) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		id := generateID()
		ourl := string(url)

		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		urlRecord := file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: ourl,
		}

		err = file.SaveURLRecord(&urlRecord, cfg.FilePath)
		if err != nil {
			http.Error(w, "Failed to save URL to file", http.StatusInternalServerError)
			return
		}

		counter++

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(shortenedURL))
	}
}

type jsonRequest struct {
	URL string `json:"url"`
}

type jsonResponse struct {
	Result string `json:"result"`
}

func APIShortenHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req jsonRequest

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		err = json.Unmarshal(body, &req)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		id := generateID()
		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		res := jsonResponse{
			Result: shortenedURL,
		}

		urlRecord := file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: req.URL,
		}

		err = file.SaveURLRecord(&urlRecord, cfg.FilePath)
		if err != nil {
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
			return
		}

		counter++

		responseJSON, err := json.Marshal(res)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		w.Write(responseJSON)
	}
}

func GetHandler(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		id := chi.URLParam(r, "id")
		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		originalURL, err := file.FindOriginalURLByShortURL(shortenedURL, cfg.FilePath)
		if err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				http.Error(w, "Not found", http.StatusNotFound)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Location", originalURL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}
}

func LoggingMiddleware() func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			start := time.Now()

			responseData := &responseData{
				status: 0,
				size:   0,
			}

			ww := &loggingResponseWriter{ResponseWriter: w, responseData: responseData}

			h.ServeHTTP(ww, r)

			duration := time.Since(start)

			sugar.Infoln(
				"uri", r.RequestURI,
				"method", r.Method,
				"status", responseData.status,
				"duration", duration,
				"size", responseData.size,
			)
		})
	}
}

type (
	// берём структуру для хранения сведений об ответе
	responseData struct {
		status int
		size   int
	}

	// добавляем реализацию http.ResponseWriter
	loggingResponseWriter struct {
		http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
		responseData        *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	// записываем ответ, используя оригинальный http.ResponseWriter
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size // захватываем размер
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWriter
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // захватываем код статуса
}

func GzipMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ow := w

		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		if supportsGzip {
			cw := gzip.NewCompressWriter(w)
			ow = cw
			defer cw.Close()
		}

		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			cr, err := gzip.NewCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body = cr
			defer cr.Close()
		}

		h.ServeHTTP(ow, r)
	}
}

func PingDBHandler(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	err := db.Ping(ctx)
	if err != nil {
		http.Error(w, "Unable to connect to database", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func main() {

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	sugar = *logger.Sugar()

	cfg := config.NewConfig()

	if cfg.DBPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		db, err = pgxpool.New(ctx, cfg.DBPath)
		if err != nil {
			sugar.Fatalw("Unable to connect to database", "error", err)
			os.Exit(1)
		}
		defer db.Close()
	} else {
		sugar.Infow("Running without database")
	}

	r := chi.NewRouter()

	r.Use(LoggingMiddleware())

	r.Post("/", GzipMiddleware(PostHandler(*cfg)))
	r.Get("/{id}", GzipMiddleware(GetHandler(*cfg)))
	r.Post("/api/shorten", GzipMiddleware(APIShortenHandler(*cfg)))

	if db != nil {
		r.Get("/ping", PingDBHandler)
	}

	sugar.Infow(
		"Starting server at",
		"addr", cfg.Address,
	)

	err = http.ListenAndServe(cfg.Address, r)
	if err != nil {
		sugar.Fatalw(err.Error(), "event", "start server")
	}
}

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
	"sync"
	"time"

	"github.com/KirillZiborov/lnkshortener/internal/auth"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/file"
	"github.com/KirillZiborov/lnkshortener/internal/gzip"
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type URLStore interface {
	SaveURLRecord(urlRecord *file.URLRecord) (string, error)
	GetOriginalURL(shortURL string) (string, bool, error)
	GetUserURLs(userID string) ([]file.URLRecord, error)
	BatchUpdateDeleteFlag(urlID string, userID string) error
}

var (
	sugar    zap.SugaredLogger
	counter  = 1
	db       *pgxpool.Pool
	urlStore URLStore
)

func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(b)
}

func PostHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		url, err := io.ReadAll(r.Body)
		if err != nil || len(url) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		id := generateID()
		ourl := string(url)

		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		urlRecord := &file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: ourl,
			UserUUID:    userID,
		}

		shortURL, err := store.SaveURLRecord(urlRecord)
		if errors.Is(err, database.ErrorDuplicate) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(shortURL))
			return
		} else if err != nil {
			http.Error(w, "Failed to save URL", http.StatusInternalServerError)
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

func APIShortenHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req jsonRequest

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		userID, err := auth.AuthPost(w, r)
		if err != nil {
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

		urlRecord := &file.URLRecord{
			UUID:        strconv.Itoa(counter),
			ShortURL:    shortenedURL,
			OriginalURL: req.URL,
			UserUUID:    userID,
		}

		shortURL, err := store.SaveURLRecord(urlRecord)
		if errors.Is(err, database.ErrorDuplicate) {
			res := jsonResponse{
				Result: shortURL,
			}
			responseJSON, err := json.Marshal(res)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			w.Write(responseJSON)
			return
		} else if err != nil {
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

func GetHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		id := chi.URLParam(r, "id")
		shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

		originalURL, deleted, err := store.GetOriginalURL(shortenedURL)
		if err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				http.Error(w, "Not found", http.StatusNotFound)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		if deleted {
			http.Error(w, "URL has been deleted", http.StatusGone)
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

type BatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

func BatchShortenHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := auth.AuthPost(w, r)
		if err != nil {
			return
		}

		var batchRequests []BatchRequest
		var batchResponses []BatchResponse

		err = json.NewDecoder(r.Body).Decode(&batchRequests)
		if err != nil || len(batchRequests) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		for _, req := range batchRequests {

			id := generateID()
			shortenedURL := fmt.Sprintf("%s/%s", cfg.BaseURL, id)

			urlRecord := &file.URLRecord{
				UUID:        strconv.Itoa(counter),
				ShortURL:    shortenedURL,
				OriginalURL: req.OriginalURL,
				UserUUID:    userID,
			}

			_, err := store.SaveURLRecord(urlRecord)
			if err != nil {
				http.Error(w, "Failed to save URL", http.StatusInternalServerError)
				return
			}

			batchResponses = append(batchResponses, BatchResponse{
				CorrelationID: req.CorrelationID,
				ShortURL:      shortenedURL,
			})

			counter++
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(batchResponses)
	}
}

func GetUserURLsHandler(store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := auth.AuthGet(r)
		if err != nil {
			http.Error(w, "Unathorized", http.StatusUnauthorized)
		}

		records, err := store.GetUserURLs(userID)
		if err != nil {
			http.Error(w, "Failed to get a list of user's URLs", http.StatusInternalServerError)
			return
		}

		if len(records) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(records)
	}
}

func BatchDeleteHandler(cfg config.Config, store URLStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) == 0 {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		userID, err := auth.AuthGet(r)
		if err != nil {
			http.Error(w, "Unathorized", http.StatusUnauthorized)
		}

		var ids []string
		err = json.Unmarshal(body, &ids)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		for i, id := range ids {
			ids[i] = fmt.Sprintf("%s/%s", cfg.BaseURL, id)
		}

		w.WriteHeader(http.StatusAccepted)
		go processBatchDelete(store, ids, userID)
	}
}

func processBatchDelete(store URLStore, ids []string, userID string) {
	doneCh := make(chan struct{})
	defer close(doneCh)

	inputCh := generator(doneCh, ids)
	channels := fanOut(store, doneCh, inputCh, userID)
	resultCh := fanIn(doneCh, channels...)

	for err := range resultCh {
		if err != nil {
			fmt.Println("Failed to delete URL")
		}
	}
}

func generator(doneCh chan struct{}, input []string) chan string {
	inputCh := make(chan string)

	go func() {
		defer close(inputCh)

		for _, data := range input {
			select {
			case <-doneCh:
				return
			case inputCh <- data:
			}
		}
	}()

	return inputCh
}

func fanOut(store URLStore, doneCh chan struct{}, inputCh chan string, userID string) []chan error {
	// количество горутин add
	numWorkers := 5
	// каналы, в которые отправляются результаты
	channels := make([]chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		// получаем канал из горутины add
		addResultCh := deleteURL(store, doneCh, inputCh, userID)
		// отправляем его в слайс каналов
		channels[i] = addResultCh
	}

	// возвращаем слайс каналов
	return channels
}

// fanIn объединяет несколько каналов resultChs в один.
func fanIn(doneCh chan struct{}, resultChs ...chan error) chan error {
	// конечный выходной канал в который отправляем данные из всех каналов из слайса, назовём его результирующим
	finalCh := make(chan error)

	// понадобится для ожидания всех горутин
	var wg sync.WaitGroup

	// перебираем все входящие каналы
	for _, ch := range resultChs {
		// в горутину передавать переменную цикла нельзя, поэтому делаем так
		chClosure := ch

		// инкрементируем счётчик горутин, которые нужно подождать
		wg.Add(1)

		go func() {
			// откладываем сообщение о том, что горутина завершилась
			defer wg.Done()

			// получаем данные из канала
			for err := range chClosure {
				select {
				// выходим из горутины, если канал закрылся
				case <-doneCh:
					return
				// если не закрылся, отправляем данные в конечный выходной канал
				case finalCh <- err:
				}
			}
		}()
	}

	go func() {
		// ждём завершения всех горутин
		wg.Wait()
		// когда все горутины завершились, закрываем результирующий канал
		close(finalCh)
	}()

	// возвращаем результирующий канал
	return finalCh
}

func deleteURL(store URLStore, doneCh chan struct{}, inputCh chan string, userID string) chan error {
	resultCh := make(chan error)

	go func() {
		defer close(resultCh)
		for id := range inputCh {
			err := store.BatchUpdateDeleteFlag(id, userID)
			select {
			case <-doneCh:
				return
			case resultCh <- err:
			}
		}
	}()

	return resultCh
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

		err = database.CreateURLTable(ctx, db)
		if err != nil {
			sugar.Fatalw("Failed to create table", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		urlStore = database.NewDBStore(db)
	} else {
		sugar.Infow("Running without database")
		urlStore = file.NewFileStore(cfg.FilePath)
	}

	r := chi.NewRouter()

	r.Use(LoggingMiddleware())

	r.Post("/", GzipMiddleware(PostHandler(*cfg, urlStore)))
	r.Post("/api/shorten", GzipMiddleware(APIShortenHandler(*cfg, urlStore)))
	r.Post("/api/shorten/batch", GzipMiddleware(BatchShortenHandler(*cfg, urlStore)))
	r.Get("/{id}", GzipMiddleware(GetHandler(*cfg, urlStore)))
	r.Get("/api/user/urls", GzipMiddleware(GetUserURLsHandler(urlStore)))

	r.Delete("/api/user/urls", GzipMiddleware(BatchDeleteHandler(*cfg, urlStore)))

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

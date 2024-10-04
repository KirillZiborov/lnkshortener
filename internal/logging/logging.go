package logging

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

var Sugar zap.SugaredLogger

func Initialize() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	Sugar = *logger.Sugar()
	return nil
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

			Sugar.Infoln(
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

// Package logging provides logging utilities and middleware for HTTP server.
// It leverages the zap library to offer structured and performant logging.
package logging

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Sugar is a globally accessible SugaredLogger instance.
// It provides a more ergonomic API for logging compared to the base Zap logger.

var Sugar zap.SugaredLogger

// Initialize sets up the global SugaredLogger using Zap's development configuration.
// It must be called before using Sugar. If initialization fails, the function returns an error.
func Initialize() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	Sugar = *logger.Sugar()
	return nil
}

// LoggingMiddleware returns an HTTP middleware that logs details of each incoming HTTP request
// and its corresponding response. It captures the request URI, method, response status code,
// duration of the request, and the size of the response body.
// To use the middleware, wrap your HTTP handlers with LoggingMiddleware.
func LoggingMiddleware() func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start timer
			start := time.Now()

			responseData := &responseData{
				status: 0,
				size:   0,
			}

			ww := &loggingResponseWriter{ResponseWriter: w, responseData: responseData}

			// Serve the HTTP request using the wrapped loggingResponseWriter and original request
			h.ServeHTTP(ww, r)

			// Capture a duration of the request
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
	// responseData holds information about the HTTP response, including the status code
	// and the size of the response body. It is used internally by the logging middleware
	// to capture response details.
	responseData struct {
		status int // status captures the HTTP status code of the response.
		size   int // size captures the size of the response body in bytes.
	}

	// loggingResponseWriter is a custom implementation of http.ResponseWriter that wraps
	// the original ResponseWriter to capture the HTTP status code and response size.
	// It intercepts Write and WriteHeader calls to record the necessary response data.
	loggingResponseWriter struct {
		http.ResponseWriter               // Embeds the original http.ResponseWriter.
		responseData        *responseData // responseData - a pointer to responseData
	}
)

// Write writes the data to the connection as part of an HTTP response.
func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	// Write the response using original http.ResponseWriter.
	size, err := r.ResponseWriter.Write(b)
	// Capture the size of the response body.
	r.responseData.size += size
	return size, err
}

// WriteHeader sends an HTTP response header with the provided status code.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// Write the status code using original http.ResponseWriter.
	r.ResponseWriter.WriteHeader(statusCode)
	// Capture the status code.
	r.responseData.status = statusCode
}

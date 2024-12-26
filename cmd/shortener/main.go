// Package main implements a URL shortener server.
// It initializes configuration, logging and storage (file or database),
// sets up HTTP routes with middleware, registers pprof handlers for profiling,
// and starts the HTTP server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KirillZiborov/lnkshortener/internal/cert"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/file"
	"github.com/KirillZiborov/lnkshortener/internal/gzip"
	"github.com/KirillZiborov/lnkshortener/internal/handlers"
	"github.com/KirillZiborov/lnkshortener/internal/logging"
)

var (
	db       *pgxpool.Pool
	urlStore handlers.URLStore

	// Use go run -ldflags to set up build variables while compiling.
	buildVersion = "N/A" // Build version
	buildDate    = "N/A" // Build date
	buildCommit  = "N/A" // Build commit
)

// main is the entrypoint of the URL shortener server.
// It initializes configuration, logging and storage, sets up HTTP routes with middleware,
// registers pprof handlers for profiling, and starts the HTTP server.
func main() {
	// Print build info.
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)

	// Initialize the logging system.
	err := logging.Initialize()
	if err != nil {
		logging.Sugar.Errorw("Internal logging error", "error", err)
	}

	// Load the configuration.
	cfg := config.NewConfig()

	// Initialize storage based on the configuration.
	if cfg.DBPath != "" {
		// Establish a connection to the PostgreSQL database with a timeout.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		db, err = pgxpool.New(ctx, cfg.DBPath)
		if err != nil {
			logging.Sugar.Errorw("Unable to connect to database", "error", err)
			return
		}

		// Create the URL table in the database if it doesn't exist.
		err = database.CreateURLTable(ctx, db)
		if err != nil {
			logging.Sugar.Errorw("Failed to create table", "error", err)
			return
		}
		defer db.Close()

		// Use the database store for URL storage.
		urlStore = database.NewDBStore(db)
	} else {
		// If no database is configured, use a file-based store.
		logging.Sugar.Infow("Running without database")
		// Use the file for URL storage.
		urlStore = file.NewFileStore(cfg.FilePath)
	}

	// Setup the router with all routes and middleware.
	router := SetupRouter(*cfg, urlStore, db)

	// Create an HTTP server at the address from the configuration.
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: router,
	}

	// Use idleConnsClosed channel to notify main process about closing all connections.
	idleConnsClosed := make(chan struct{})
	// Create sigs channel to listen for syscall signals.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	// Start goroutine to handle interruptions.
	go func() {
		// Read from interruptions channel.
		sig := <-sigs
		logging.Sugar.Infof("Received signal: %s. Initiating shutdown.", sig)
		// Shutdown the server gracefully.
		if err := server.Shutdown(context.Background()); err != nil {
			logging.Sugar.Errorf("Error during server shutdown: %v", err)
		}
		// Notify main process that all commections are handled and closed.
		close(idleConnsClosed)
		logging.Sugar.Info("Server shut down gracefully.")
	}()

	// Log the server start event with the address.
	logging.Sugar.Infow(
		"Starting server at",
		"addr", cfg.Address,
	)

	// Start the HTTP server.
	if cfg.EnableHTTPS {
		logging.Sugar.Infow("Generating new TLS certificate")
		// Generate certificate and key using the CreateCertificate function.
		err = cert.CreateCertificate(cert.CertificateFilePath, cert.KeyFilePath)
		if err != nil {
			logging.Sugar.Errorw("Failed to create certificate", "error", err)
			return
		}

		err = server.ListenAndServeTLS(cert.CertificateFilePath, cert.KeyFilePath)
	} else {
		err = server.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		logging.Sugar.Errorw("Failed to start server", "error", err, "event", "start server")
		return
	}

	// Wait for the end of graceful shutdown.
	<-idleConnsClosed
}

// SetupRouter initializes the Chi router with all routes and middlewares.
// It configures routes for creating, retrieving, and deleting shortened URLs.
// Middleware:
// - LoggingMiddleware: Logs each incoming HTTP request.
// - Gzip Middleware: Compresses/decompresses data to optimize bandwidth.
//
// Routes:
// - POST "/" : Creates a new shortened URL.
// - POST "/api/shorten" : Creates a new shortened URL for JSON requests.
// - POST "/api/shorten/batch" : Creates multiple shortened URLs in batch.
// - GET "/{id}" : Redirects to the original URL based on the shortened ID.
// - GET "/api/user/urls" : Retrieves all URLs created by the user.
// - DELETE "/api/user/urls" : Deletes multiple URLs in batch.
// - GET "/ping" : Health check endpoint to verify database connection.
//
// Pprof Handlers: Provides profiling endpoints for performance analysis.
// Profiling Endpoints:
// - "/debug/pprof/" : pprof index.
// - "/debug/pprof/cmdline" : pprof cmdline.
// - "/debug/pprof/profile" : pprof profile.
// - "/debug/pprof/symbol" : pprof symbol.
// - "/debug/pprof/trace" : pprof trace.
// - "/debug/pprof/heap" : pprof heap.
func SetupRouter(cfg config.Config, store handlers.URLStore, db *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()

	// Apply global middleware.
	r.Use(logging.LoggingMiddleware())

	// Define routes with associated handlers and middleware.
	r.Post("/", gzip.Middleware(handlers.PostHandler(cfg, store)))
	r.Post("/api/shorten", gzip.Middleware(handlers.APIShortenHandler(cfg, store)))
	r.Post("/api/shorten/batch", gzip.Middleware(handlers.BatchShortenHandler(cfg, store)))
	r.Get("/{id}", gzip.Middleware(handlers.GetHandler(cfg, store)))
	r.Get("/api/user/urls", gzip.Middleware(handlers.GetUserURLsHandler(store)))
	r.Delete("/api/user/urls", gzip.Middleware(handlers.BatchDeleteHandler(cfg, store)))

	// Conditional route for database health check.
	if db != nil {
		r.Get("/ping", handlers.PingDBHandler(db))
	}

	// Register pprof routes for profiling.
	registerPprof(r)

	return r
}

// registerPprof registers pprof handlers to the provided Chi router.
// This allows profiling and debugging of the application.
func registerPprof(r *chi.Mux) {
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
}

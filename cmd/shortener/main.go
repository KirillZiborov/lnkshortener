// Package main implements a URL shortener server.
// It initializes configuration, logging and storage (file or database),
// sets up HTTP routes with middleware, registers pprof handlers for profiling,
// and starts the HTTP server.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"

	grpcapi "github.com/KirillZiborov/lnkshortener/internal/api/grpc"
	"github.com/KirillZiborov/lnkshortener/internal/api/grpc/interceptors"
	"github.com/KirillZiborov/lnkshortener/internal/api/grpc/proto"
	"github.com/KirillZiborov/lnkshortener/internal/api/http/cert"
	"github.com/KirillZiborov/lnkshortener/internal/api/http/gzip"
	"github.com/KirillZiborov/lnkshortener/internal/api/http/handlers"
	"github.com/KirillZiborov/lnkshortener/internal/app"
	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/file"
	"github.com/KirillZiborov/lnkshortener/internal/logging"
)

var (
	db       *pgxpool.Pool
	urlStore app.URLStore

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

	service := app.ShortenerService{
		Store: urlStore,
		Cfg:   cfg,
	}

	// Setup the router with all routes and middleware.
	router := SetupRouter(service, db)

	// Start the gRPC server if it is enabled.
	if cfg.GRPCAddress != "" {
		go func() {
			lis, err := net.Listen("tcp", cfg.GRPCAddress)
			if err != nil {
				logging.Sugar.Errorw("Failed to listen gRPC", "error", err)
				return
			}
			grpcServer := grpc.NewServer(
				grpc.ChainUnaryInterceptor(
					interceptors.AuthInterceptor(),
					interceptors.IPInterceptor()))

			shortenerServer := grpcapi.NewGRPCShortenerServer(&service)
			proto.RegisterShortenerServiceServer(grpcServer, shortenerServer)

			logging.Sugar.Infow(
				"Starting gRPC server at",
				"addr", cfg.GRPCAddress)
			if err := grpcServer.Serve(lis); err != nil {
				logging.Sugar.Errorw("gRPC server error", "error", err)
			}
		}()
	}

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
		"Starting HTTP server at",
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
// - GET "/api/internal/stats" : Stats (number of URLs and unique users) check endpoint.
//
// Pprof Handlers: Provides profiling endpoints for performance analysis.
// Profiling Endpoints:
// - "/debug/pprof/" : pprof index.
// - "/debug/pprof/cmdline" : pprof cmdline.
// - "/debug/pprof/profile" : pprof profile.
// - "/debug/pprof/symbol" : pprof symbol.
// - "/debug/pprof/trace" : pprof trace.
// - "/debug/pprof/heap" : pprof heap.
func SetupRouter(service app.ShortenerService, db *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()

	// Apply global middleware.
	r.Use(logging.LoggingMiddleware())

	// Define routes with associated handlers and middleware.
	r.Post("/", gzip.Middleware(handlers.PostHandler(&service)))
	r.Post("/api/shorten", gzip.Middleware(handlers.APIShortenHandler(&service)))
	r.Post("/api/shorten/batch", gzip.Middleware(handlers.BatchShortenHandler(&service)))
	r.Get("/{id}", gzip.Middleware(handlers.GetHandler(&service)))
	r.Get("/api/user/urls", gzip.Middleware(handlers.GetUserURLsHandler(&service)))
	r.Get("/api/internal/stats", gzip.Middleware(handlers.GetStatsHandler(&service)))
	r.Delete("/api/user/urls", gzip.Middleware(handlers.BatchDeleteHandler(&service)))

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

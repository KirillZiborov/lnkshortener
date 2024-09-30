package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/KirillZiborov/lnkshortener/internal/config"
	"github.com/KirillZiborov/lnkshortener/internal/database"
	"github.com/KirillZiborov/lnkshortener/internal/file"
	"github.com/KirillZiborov/lnkshortener/internal/gzip"
	"github.com/KirillZiborov/lnkshortener/internal/handlers"
	"github.com/KirillZiborov/lnkshortener/internal/logging"
	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	db       *pgxpool.Pool
	urlStore handlers.URLStore
)

func main() {

	err := logging.Initialize()
	if err != nil {
		logging.Sugar.Fatalw("Internal logging error", err)
	}

	cfg := config.NewConfig()

	if cfg.DBPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		db, err = pgxpool.New(ctx, cfg.DBPath)
		if err != nil {
			logging.Sugar.Fatalw("Unable to connect to database", "error", err)
			os.Exit(1)
		}

		err = database.CreateURLTable(ctx, db)
		if err != nil {
			logging.Sugar.Fatalw("Failed to create table", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		urlStore = database.NewDBStore(db)
	} else {
		logging.Sugar.Infow("Running without database")
		urlStore = file.NewFileStore(cfg.FilePath)
	}

	r := chi.NewRouter()

	r.Use(logging.LoggingMiddleware())

	r.Post("/", gzip.Middleware(handlers.PostHandler(*cfg, urlStore)))
	r.Post("/api/shorten", gzip.Middleware(handlers.APIShortenHandler(*cfg, urlStore)))
	r.Post("/api/shorten/batch", gzip.Middleware(handlers.BatchShortenHandler(*cfg, urlStore)))
	r.Get("/{id}", gzip.Middleware(handlers.GetHandler(*cfg, urlStore)))
	r.Get("/api/user/urls", gzip.Middleware(handlers.GetUserURLsHandler(urlStore)))

	r.Delete("/api/user/urls", gzip.Middleware(handlers.BatchDeleteHandler(*cfg, urlStore)))

	if db != nil {
		r.Get("/ping", handlers.PingDBHandler(db))
	}

	logging.Sugar.Infow(
		"Starting server at",
		"addr", cfg.Address,
	)

	err = http.ListenAndServe(cfg.Address, r)
	if err != nil {
		logging.Sugar.Fatalw(err.Error(), "event", "start server")
	}
}

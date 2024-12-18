// Package config provides functionalities to parse and manage application configuration.
// It loads configuration settings from command-line flags and environment variables.
package config

import (
	"flag"
	"os"
)

// Config represents the configuration settings for the application.
// It includes settings for the server address, base URL for shortened URLs,
// file storage path, and database connection string.
type Config struct {
	// Address specifies the address on which the HTTP server listens.
	// Example: "localhost:8080"
	Address string
	// BaseURL defines the base URL used for generating shortened URLs.
	// Example: "http://localhost:8080"
	BaseURL string
	// FilePath indicates the file path used for storing URLs
	// when a database is not configured.
	// Example: "URLstorage.json"
	FilePath string
	// DBPath contains the database connection string used to connect
	// to the PostgreSQL database. If empty, the application uses file storage.
	// Example: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
	DBPath string
	// EnableHTTPS defines connection type.
	// If true, HTTPS is enabled.
	EnableHTTPS bool
}

// NewConfig initializes and returns a new coniguration instance.
// It parses command-line flags and overrides them with environment variables if they are set.
// The priority is:
// 1. Environment Variables
// 2. Command-Line Flags
// 3. Default Values
//
// Command-Line Flags:
//
//	-a string
//	      Address of the HTTP server (default "localhost:8080")
//	-b string
//	      BaseURL for shortened URLs (default "http://localhost:8080")
//	-f string
//	      URL storage file path (default "URLstorage.json")
//	-d string
//	      Database address (default "")
//	-s bool
//	      Connection type: HTTP or HTTPS (default false - HTTP)
//
// Environment Variables:
//
//	SERVER_ADDRESS       Overrides the -a flag.
//	BASE_URL             Overrides the -b flag.
//	FILE_STORAGE_PATH    Overrides the -f flag.
//	DATABASE_DSN         Overrides the -d flag.
//	ENABLE_HTTPS         Overrides the -s flag.
func NewConfig() *Config {
	cfg := &Config{}

	// Define command-line flags and associate them with Config fields.
	flag.StringVar(&cfg.Address, "a", "localhost:8080", "Address of the HTTP server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "BaseURL for shortened URLs")
	flag.StringVar(&cfg.FilePath, "f", "", "URL storage file path")
	flag.StringVar(&cfg.DBPath, "d", "", "Database address")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "Connection type")

	// Parse the command-line flags.
	flag.Parse()

	// Override Address with the SERVER_ADDRESS environment variable if set.
	envAddress := os.Getenv("SERVER_ADDRESS")
	if envAddress != "" {
		cfg.Address = envAddress
	} else if cfg.Address == "" {
		// Ensure Address has a default value if not set via flag or environment.
		cfg.Address = "localhost:8080"
	}

	// Override BaseURL with the BASE_URL environment variable if set.
	envBaseURL := os.Getenv("BASE_URL")
	if envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	} else if cfg.BaseURL == "" {
		// Derive BaseURL from Address if not set via flag or environment.
		cfg.BaseURL = "http://" + cfg.Address
	}

	// Override FilePath with the FILE_STORAGE_PATH environment variable if set.
	if envStoragePath := os.Getenv("FILE_STORAGE_PATH"); envStoragePath != "" {
		cfg.FilePath = envStoragePath
	}

	// Ensure FilePath has a default value if not set via flag or environment.
	if cfg.FilePath == "" {
		cfg.FilePath = "URLstorage.json"
	}

	// Override DBPath with the DATABASE_DSN environment variable if set.
	if DBPath := os.Getenv("DATABASE_DSN"); DBPath != "" {
		cfg.DBPath = DBPath
	}

	// Override EnableHTTPS with the ENABLE_HTTPS environment variable if set.
	if envHTTPS := os.Getenv("ENABLE_HTTPS"); envHTTPS == "true" {
		cfg.EnableHTTPS = true
	}
	return cfg
}

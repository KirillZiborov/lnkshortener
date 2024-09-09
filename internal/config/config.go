package config

import (
	"flag"
	"os"
)

type Config struct {
	Address  string
	BaseURL  string
	FilePath string
	DBPath   string
}

func NewConfig() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "Address of the HTTP server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "BaseURL for shortened URLs")
	flag.StringVar(&cfg.FilePath, "f", "", "URL storage file path")
	flag.StringVar(&cfg.FilePath, "d", "", "Database address")

	flag.Parse()

	envAddress := os.Getenv("SERVER_ADDRESS")
	envBaseURL := os.Getenv("BASE_URL")

	if envAddress != "" {
		cfg.Address = envAddress
	} else if cfg.Address == "" {
		cfg.Address = "localhost:8080"
	}

	if envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	} else if cfg.BaseURL == "" {
		cfg.BaseURL = "http://" + cfg.Address
	}

	if envStoragePath := os.Getenv("FILE_STORAGE_PATH"); envStoragePath != "" {
		cfg.FilePath = envStoragePath
	}

	if cfg.FilePath == "" {
		cfg.FilePath = "URLstorage.json"
	}

	if DBPath := os.Getenv("DATABASE_DSN"); DBPath != "" {
		cfg.DBPath = DBPath
	}

	if cfg.DBPath == "" {
		cfg.DBPath = "database.db"
	}

	return cfg
}

// Package config provides functionalities to parse and manage application configuration.
// It loads configuration settings from environment variables, command-line flags
// and configuration file.
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/KirillZiborov/lnkshortener/internal/logging"
)

// Config represents the configuration settings for the application.
// It includes settings for the server address, base URL for shortened URLs,
// file storage path, database connection string and trusted subnet CIDR.
type Config struct {
	// Address specifies the address on which the HTTP server listens.
	// Example: "localhost:8080"
	Address string `json:"server_address"`
	// BaseURL defines the base URL used for generating shortened URLs.
	// Example: "http://localhost:8080"
	BaseURL string `json:"base_url"`
	// FilePath indicates the file path used for storing URLs
	// when a database is not configured.
	// Example: "URLstorage.json"
	FilePath string `json:"file_storage_path"`
	// DBPath contains the database connection string used to connect
	// to the PostgreSQL database. If empty, the application uses file storage.
	// Example: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
	DBPath string `json:"database_dsn"`
	// EnableHTTPS defines connection type.
	// If true, HTTPS is enabled.
	EnableHTTPS bool `json:"enable_https"`
	// TrustedSubnet defines string representation of CIDR.
	// Example: "192.168.1.0/24"
	TrustedSubnet string `json:"trusted_subnet"`
	// GRPCAddress specifies the address on which the gRPC server listens.
	// Example: "localhost:9090".
	// If empty, gRPC is disabled.
	GRPCAddress string `json:"grpc_address"`
}

// NewConfig initializes and returns a new coniguration instance.
// It parses command-line flags and overrides them with environment variables if they are set.
// The priority is:
// 1. Environment Variables
// 2. Command-Line Flags
// 3. Configuration File
// 4. Default Values
//
// 1. Environment Variables:
//
//	SERVER_ADDRESS       Overrides the -a flag.
//	BASE_URL             Overrides the -b flag.
//	FILE_STORAGE_PATH    Overrides the -f flag.
//	DATABASE_DSN         Overrides the -d flag.
//	ENABLE_HTTPS         Overrides the -s flag.
//	TRUSTED_SUBNET       Overrides the -t flag.
//	GRPC_ADDRESS       	 Overrides the -g flag.
//
// 2. Command-Line Flags:
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
//	-t string
//	      Truted subnet address (default "")
//	-g string
//	      Address of the gRPC server (default "", gRPC disabled)
//	-config string
//	      Configuration file path
//
// 3. Configuration File:
//
//	"server_address": string
//		  Analogue for environment variable SERVER_ADDRESS and -a flag
//	"base_url": string
//		  Analogue for environment variable BASE_URL and -b flag
//	"file_storage_path": string
//		  Analogue for environment variable FILE_STORAGE_PATH and -f flag
//	"database_dsn": string
//		  Analogue for environment variable DATABASE_DSN and -d flag
//	"enable_https": bool
//		  Analogue for environment variable ENABLE_HTTPS and -s flag
//	"trusted_subnet": string
//		  Analogue for environment variable TRUSTED_SUBNET and -t flag
//	"grpc_address": string
//		  Analogue for environment variable GRPC_ADDRESS and -g flag
//
// 4. Default Values:
//
//	Address:     	"localhost:8080",
//	BaseURL:     	"http://Address",
//	FilePath:    	"URLstorage.json",
//	DBPath:      	"",
//	EnableHTTPS: 	false,
//	TrustedSubnet:  "",
//	GRPCAddress:    ""
func NewConfig() *Config {
	cfg := &Config{}
	// Specify default configuration values.
	currentCfg := &Config{
		Address:       "localhost:8080",
		BaseURL:       "",
		FilePath:      "URLstorage.json",
		DBPath:        "",
		EnableHTTPS:   false,
		TrustedSubnet: "",
		GRPCAddress:   "",
	}

	// Define command-line flags and associate them with Config fields.
	flag.StringVar(&cfg.Address, "a", "", "Address of the HTTP server")
	flag.StringVar(&cfg.BaseURL, "b", "", "BaseURL for shortened URLs")
	flag.StringVar(&cfg.FilePath, "f", "", "URL storage file path")
	flag.StringVar(&cfg.DBPath, "d", "", "Database address")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "Connection type")
	flag.StringVar(&cfg.TrustedSubnet, "t", "", "Trusted subnet CIDR")
	flag.StringVar(&cfg.GRPCAddress, "g", "", "Address of the gRPC server")

	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")

	// Parse the command-line flags.
	flag.Parse()

	// Check if configuration is set in file.
	if configFilePath := getConfigFilePath(configPath); configFilePath != "" {
		// Load configuration from the file and override default values.
		if err := loadConfigFromFile(configFilePath, currentCfg); err != nil {
			logging.Sugar.Errorw("Failed to load config file at", "error", err, "addr", configFilePath)
		}
	}

	// Override Address with the SERVER_ADDRESS environment variable if set.
	envAddress := os.Getenv("SERVER_ADDRESS")
	if envAddress != "" {
		cfg.Address = envAddress
	} else if cfg.Address == "" {
		// Ensure Address has a current value if not set via flag or environment.
		cfg.Address = currentCfg.Address
	}

	// Override BaseURL with the BASE_URL environment variable if set.
	envBaseURL := os.Getenv("BASE_URL")
	if envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	} else if cfg.BaseURL == "" {
		if currentCfg.BaseURL != "" {
			cfg.BaseURL = currentCfg.BaseURL
		} else {
			// Derive BaseURL from Address if not set yet.
			cfg.BaseURL = "http://" + cfg.Address
		}
	}

	// Override FilePath with the FILE_STORAGE_PATH environment variable if set.
	if envStoragePath := os.Getenv("FILE_STORAGE_PATH"); envStoragePath != "" {
		cfg.FilePath = envStoragePath
	} else if cfg.FilePath == "" {
		// Ensure FilePath has a current value if not set via flag or environment.
		cfg.FilePath = currentCfg.FilePath
	}

	// Override DBPath with the DATABASE_DSN environment variable if set.
	if DBPath := os.Getenv("DATABASE_DSN"); DBPath != "" {
		cfg.DBPath = DBPath
	} else if cfg.DBPath == "" {
		// Look for DBPath in the config file.
		cfg.DBPath = currentCfg.DBPath
	}

	// Override EnableHTTPS with the ENABLE_HTTPS environment variable if set.
	if envHTTPS := os.Getenv("ENABLE_HTTPS"); envHTTPS == "true" {
		cfg.EnableHTTPS = true
	} else if !cfg.EnableHTTPS {
		cfg.EnableHTTPS = currentCfg.EnableHTTPS
	}

	// Override TrustedSubnet with the TRUSTED_SUBNET environment variable if set.
	if trustedSubnet := os.Getenv("TRUSTED_SUBNET"); trustedSubnet == "" {
		cfg.TrustedSubnet = trustedSubnet
	} else if cfg.TrustedSubnet == "" {
		cfg.TrustedSubnet = currentCfg.TrustedSubnet
	}

	// Override GRPCAddress with the GRPC_ADDRESS environment variable if set.
	grpcAddress := os.Getenv("GRPC_ADDRESS")
	if grpcAddress != "" {
		cfg.GRPCAddress = grpcAddress
	} else if cfg.GRPCAddress == "" {
		cfg.GRPCAddress = currentCfg.GRPCAddress
	}

	return cfg
}

// getConfigFilePath returns the path to the configuration file if exists.
func getConfigFilePath(flagPath string) string {
	if envPath := os.Getenv("CONFIG"); envPath != "" {
		return envPath
	} else if flagPath != "" {
		return flagPath
	}
	log.Println("No configuration file path provided.")
	return ""
}

// loadConfigFromFile reads the configuration from a JSON file.
func loadConfigFromFile(filePath string, cfg *Config) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}
	return nil
}

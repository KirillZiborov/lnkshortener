package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Run("ENV vars only", func(t *testing.T) {
		os.Setenv("SERVER_ADDRESS", "localhost:1717")
		os.Setenv("BASE_URL", "http://localhost:7171")
		os.Setenv("FILE_STORAGE_PATH", "storage.json")
		os.Setenv("DATABASE_DSN", "postgres://database")
		os.Setenv("ENABLE_HTTPS", "true")

		cfg := NewConfig()

		assert.Equal(t, "localhost:1717", cfg.Address)
		assert.Equal(t, "http://localhost:7171", cfg.BaseURL)
		assert.Equal(t, "storage.json", cfg.FilePath)
		assert.Equal(t, "postgres://database", cfg.DBPath)
		assert.Equal(t, true, cfg.EnableHTTPS)

		os.Unsetenv("SERVER_ADDRESS")
		os.Unsetenv("BASE_URL")
		os.Unsetenv("FILE_STORAGE_PATH")
		os.Unsetenv("DATABASE_DSN")
		os.Unsetenv("ENABLE_HTTPS")
	})

	t.Run("ENV vars + flags", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		os.Setenv("SERVER_ADDRESS", "localhost:1717")
		os.Setenv("BASE_URL", "http://localhost:7171")
		os.Setenv("FILE_STORAGE_PATH", "storage.json")
		os.Setenv("DATABASE_DSN", "postgres://database")
		os.Args = []string{"program", "-a", "localhost:2717", "-b", "http://localhost:7271",
			"-f", "fstorage.json", "-d", "postgres://fdatabase", "-s"}

		cfg := NewConfig()

		assert.Equal(t, "localhost:1717", cfg.Address)
		assert.Equal(t, "http://localhost:7171", cfg.BaseURL)
		assert.Equal(t, "storage.json", cfg.FilePath)
		assert.Equal(t, "postgres://database", cfg.DBPath)
		assert.Equal(t, true, cfg.EnableHTTPS)

		os.Unsetenv("SERVER_ADDRESS")
		os.Unsetenv("BASE_URL")
		os.Unsetenv("FILE_STORAGE_PATH")
		os.Unsetenv("DATABASE_DSN")
	})

	t.Run("flags only", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		os.Args = []string{"program", "-a", "localhost:2717", "-b", "http://localhost:7271",
			"-f", "fstorage.json", "-d", "postgres://fdatabase", "-s"}

		cfg := NewConfig()

		assert.Equal(t, "localhost:2717", cfg.Address)
		assert.Equal(t, "http://localhost:7271", cfg.BaseURL)
		assert.Equal(t, "fstorage.json", cfg.FilePath)
		assert.Equal(t, "postgres://fdatabase", cfg.DBPath)
		assert.Equal(t, true, cfg.EnableHTTPS)

	})

	t.Run("config file only", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		configFile := `{
				"server_address": "localhost:3717",
				"base_url": "http://localhost:7371",
				"file_storage_path": "/path/to/file.db",
    			"database_dsn": "postgres://database",
    			"enable_https": true
			}`

		filename := "config.json"
		tempFile, err := os.Create(filename)
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(configFile)
		require.NoError(t, err)

		os.Args = []string{"program", "-config", filename}

		cfg := NewConfig()

		assert.Equal(t, "localhost:3717", cfg.Address)
		assert.Equal(t, "http://localhost:7371", cfg.BaseURL)
		assert.Equal(t, "/path/to/file.db", cfg.FilePath)
		assert.Equal(t, "postgres://database", cfg.DBPath)
		assert.Equal(t, true, cfg.EnableHTTPS)
	})

	t.Run("ENV vars + config file", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		os.Setenv("SERVER_ADDRESS", "localhost:1717")
		os.Setenv("BASE_URL", "http://localhost:7171")
		os.Setenv("FILE_STORAGE_PATH", "storage.json")
		os.Setenv("DATABASE_DSN", "postgres://database")

		configFile := `{
				"server_address": "localhost:3717",
				"base_url": "http://localhost:7371",
				"file_storage_path": "/path/to/file.db",
    			"database_dsn": "postgres://cdatabase",
    			"enable_https": true
			}`

		tempFile, err := os.CreateTemp("", "config.json")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(configFile)
		require.NoError(t, err)

		os.Args = []string{"program", "-config", tempFile.Name()}

		cfg := NewConfig()

		// ENV vars should take priority over config file
		assert.Equal(t, "localhost:1717", cfg.Address)
		assert.Equal(t, "http://localhost:7171", cfg.BaseURL)
		assert.Equal(t, "storage.json", cfg.FilePath)
		assert.Equal(t, "postgres://database", cfg.DBPath)
		assert.Equal(t, true, cfg.EnableHTTPS)

		os.Unsetenv("SERVER_ADDRESS")
		os.Unsetenv("BASE_URL")
		os.Unsetenv("FILE_STORAGE_PATH")
		os.Unsetenv("DATABASE_DSN")
		os.Unsetenv("ENABLE_HTTPS")
	})

	t.Run("config file + flags", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		configFile := `{
				"server_address": "localhost:3717",
				"base_url": "http://localhost:7371",
				"file_storage_path": "/path/to/file.db",
    			"database_dsn": "postgres://cdatabase",
    			"enable_https": false
			}`

		tempFile, err := os.CreateTemp("", "config.json")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(configFile)
		require.NoError(t, err)

		os.Args = []string{"program", "-a", "localhost:2717", "-b", "http://localhost:7271",
			"-f", "fstorage.json", "-d", "postgres://fdatabase", "-s"}

		cfg := NewConfig()

		// Flags should take priority over config file
		assert.Equal(t, "localhost:2717", cfg.Address)
		assert.Equal(t, "http://localhost:7271", cfg.BaseURL)
		assert.Equal(t, "fstorage.json", cfg.FilePath)
		assert.Equal(t, "postgres://fdatabase", cfg.DBPath)
		assert.Equal(t, true, cfg.EnableHTTPS)
	})

	t.Run("ENV vars + config file + flags", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		os.Setenv("SERVER_ADDRESS", "localhost:1717")
		os.Setenv("BASE_URL", "http://localhost:7171")
		os.Setenv("FILE_STORAGE_PATH", "storage.json")
		os.Setenv("DATABASE_DSN", "postgres://database")
		os.Setenv("ENABLE_HTTPS", "true")

		configFile := `{
				"server_address": "localhost:3717",
				"base_url": "http://localhost:7371",
				"file_storage_path": "/path/to/file.db",
    			"database_dsn": "postgres://cdatabase",
    			"enable_https": false
			}`

		tempFile, err := os.CreateTemp("", "config.json")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.WriteString(configFile)
		require.NoError(t, err)

		os.Args = []string{"program", "-a", "localhost:2717", "-b", "http://localhost:7271",
			"-f", "fstorage.json", "-d", "postgres://fdatabase"}

		cfg := NewConfig()

		// ENV vars should take priority over flags and config file
		assert.Equal(t, "localhost:1717", cfg.Address)
		assert.Equal(t, "http://localhost:7171", cfg.BaseURL)
		assert.Equal(t, "storage.json", cfg.FilePath)
		assert.Equal(t, "postgres://database", cfg.DBPath)
		assert.Equal(t, true, cfg.EnableHTTPS)

		os.Unsetenv("SERVER_ADDRESS")
		os.Unsetenv("BASE_URL")
		os.Unsetenv("FILE_STORAGE_PATH")
		os.Unsetenv("DATABASE_DSN")
		os.Unsetenv("ENABLE_HTTPS")
	})

	t.Run("default values", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		// no flags
		os.Args = []string{"program"}

		cfg := NewConfig()

		assert.Equal(t, "localhost:8080", cfg.Address)
		assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
		assert.Equal(t, "URLstorage.json", cfg.FilePath)
		assert.Equal(t, "", cfg.DBPath)
		assert.Equal(t, false, cfg.EnableHTTPS)
	})
}

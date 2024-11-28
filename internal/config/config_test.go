package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfing(t *testing.T) {
	t.Run("ENV vars only", func(t *testing.T) {
		os.Setenv("SERVER_ADDRESS", "localhost:1717")
		os.Setenv("BASE_URL", "http://localhost:7171")

		cfg := NewConfig()

		assert.Equal(t, "localhost:1717", cfg.Address)
		assert.Equal(t, "http://localhost:7171", cfg.BaseURL)

		os.Unsetenv("SERVER_ADDRESS")
		os.Unsetenv("BASE_URL")
	})

	t.Run("ENV vars + flags", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		os.Setenv("SERVER_ADDRESS", "localhost:1717")
		os.Setenv("BASE_URL", "http://localhost:7171")
		os.Args = []string{"program", "-a", "localhost:2717", "-b", "http://localhost:7271"}

		cfg := NewConfig()

		assert.Equal(t, "localhost:1717", cfg.Address)
		assert.Equal(t, "http://localhost:7171", cfg.BaseURL)

		os.Unsetenv("SERVER_ADDRESS")
		os.Unsetenv("BASE_URL")
	})

	t.Run("flags only", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		os.Args = []string{"program", "-a", "localhost:2717", "-b", "http://localhost:7271"}

		cfg := NewConfig()

		assert.Equal(t, "localhost:2717", cfg.Address)
		assert.Equal(t, "http://localhost:7271", cfg.BaseURL)

	})

	t.Run("default values", func(t *testing.T) {
		// anti-panic: flag redefined
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		// no -a -b flags
		os.Args = []string{"program"}

		cfg := NewConfig()

		assert.Equal(t, "localhost:8080", cfg.Address)
		assert.Equal(t, "http://localhost:8080", cfg.BaseURL)

	})
}

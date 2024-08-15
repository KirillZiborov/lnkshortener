package config

import (
	"flag"
	"fmt"
)

type Config struct {
	Address string
	BaseURL string
}

func NewConfig() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.Address, "a", "localhost:8080", "Address of the HTTP server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8000", "BaseURL for shortened URLs")

	flag.Parse()

	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("http://%s", cfg.Address)
	}

	return cfg
}

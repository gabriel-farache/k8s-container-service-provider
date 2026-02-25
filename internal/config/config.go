package config

import (
	"fmt"
	"time"

	env "github.com/caarlos0/env/v11"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Address         string        `env:"ADDRESS"          envDefault:":8080"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

// Config is the root configuration for the service provider.
type Config struct {
	Server ServerConfig `envPrefix:"SP_SERVER_"`
}

// Load reads configuration from environment variables.
// Env vars: SP_SERVER_ADDRESS (default ":8080"), SP_SERVER_SHUTDOWN_TIMEOUT (default "15s").
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}
	return cfg, nil
}

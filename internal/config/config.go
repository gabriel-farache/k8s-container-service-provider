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
	ReadTimeout     time.Duration `env:"READ_TIMEOUT"     envDefault:"15s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT"    envDefault:"15s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT"     envDefault:"60s"`
}

// ProviderConfig holds service provider identity and metadata.
type ProviderConfig struct {
	Name        string `env:"NAME,notEmpty"`
	DisplayName string `env:"DISPLAY_NAME"`
	Endpoint    string `env:"ENDPOINT,notEmpty"`
	Region      string `env:"REGION"`
	Zone        string `env:"ZONE"`
}

// DCMConfig holds DCM registry connection settings.
type DCMConfig struct {
	RegistrationURL string `env:"REGISTRATION_URL,notEmpty"`
}

// Config is the root configuration for the service provider.
type Config struct {
	Server   ServerConfig   `envPrefix:"SP_SERVER_"`
	Provider ProviderConfig `envPrefix:"SP_PROVIDER_"`
	DCM      DCMConfig      `envPrefix:"SP_DCM_"`
}

// Load reads configuration from environment variables.
// Env vars: SP_SERVER_*, SP_PROVIDER_*, SP_DCM_* (see struct tags for details).
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("loading configuration: %w", err)
	}
	return cfg, nil
}

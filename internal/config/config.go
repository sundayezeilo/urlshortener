package config

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration.
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	App           AppConfig
	Observability ObservabilityConfig
}

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Port            string        `envconfig:"SERVER_PORT" required:"true"`
	Host            string        `envconfig:"SERVER_HOST" required:"true"`
	BaseURL         string        `envconfig:"SERVER_BASE_URL" required:"true"`
	ReadTimeout     time.Duration `envconfig:"SERVER_READ_TIMEOUT" required:"true"`
	WriteTimeout    time.Duration `envconfig:"SERVER_WRITE_TIMEOUT" required:"true"`
	IdleTimeout     time.Duration `envconfig:"SERVER_IDLE_TIMEOUT" required:"true"`
	ShutdownTimeout time.Duration `envconfig:"SERVER_SHUTDOWN_TIMEOUT" required:"true"`
}

// Validate validates the server configuration.
func (c *ServerConfig) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base URL cannot be empty")
	}
	if c.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}
	if c.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}
	if c.IdleTimeout <= 0 {
		return fmt.Errorf("idle timeout must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	return nil
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Host     string `envconfig:"DB_HOST" required:"true"`
	Port     string `envconfig:"DB_PORT" required:"true"`
	User     string `envconfig:"DB_USER" required:"true"`
	Password string `envconfig:"DB_PASSWORD" required:"true"`
	Name     string `envconfig:"DB_NAME" required:"true"`
	SSLMode  string `envconfig:"DB_SSLMODE" required:"true"`
	MaxConns int32  `envconfig:"DB_MAX_CONNS" required:"true"`
	MinConns int32  `envconfig:"DB_MIN_CONNS" required:"true"`
}

// Validate validates the database configuration.
func (c *DatabaseConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	if c.User == "" {
		return fmt.Errorf("user cannot be empty")
	}
	if c.Password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	if c.Name == "" {
		return fmt.Errorf("database name cannot be empty")
	}
	if c.MaxConns <= 0 {
		return fmt.Errorf("max connections must be positive")
	}
	if c.MinConns <= 0 {
		return fmt.Errorf("min connections must be positive")
	}
	if c.MinConns > c.MaxConns {
		return fmt.Errorf("min connections (%d) cannot be greater than max connections (%d)", c.MinConns, c.MaxConns)
	}

	validSSLModes := map[string]bool{
		"disable":     true,
		"require":     true,
		"verify-ca":   true,
		"verify-full": true,
	}
	if !validSSLModes[c.SSLMode] {
		return fmt.Errorf("invalid SSL mode: %s (must be one of: disable, require, verify-ca, verify-full)", c.SSLMode)
	}
	return nil
}

// ConnectionString returns the PostgreSQL connection string.
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// AppConfig holds application-specific configuration.
type AppConfig struct {
	Environment string `envconfig:"APP_ENV" required:"true"`   // development, staging, production, test
	LogLevel    string `envconfig:"LOG_LEVEL" required:"true"` // debug, info, warn, error
}

// Validate validates the app configuration.
func (c *AppConfig) Validate() error {
	validEnvs := map[string]bool{
		"development": true,
		"staging":     true,
		"production":  true,
		"test":        true,
	}
	if !validEnvs[c.Environment] {
		return fmt.Errorf("invalid environment: %s (must be one of: development, staging, production, test)", c.Environment)
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s (must be one of: debug, info, warn, error)", c.LogLevel)
	}
	return nil
}

// ObservabilityConfig holds configuration for tracing/metrics.
type ObservabilityConfig struct {
	Enabled           bool    `envconfig:"OTEL_ENABLED" required:"true"`
	ServiceName       string  `envconfig:"OTEL_SERVICE_NAME"`
	ServiceVersion    string  `envconfig:"OTEL_SERVICE_VERSION"`
	OTelEndpoint      string  `envconfig:"OTEL_ENDPOINT"`
	OTelInsecure      bool    `envconfig:"OTEL_INSECURE"`
	TracingSampleRate float64 `envconfig:"OTEL_TRACING_SAMPLE_RATE"`
}

// Validate validates the observability configuration.
func (c *ObservabilityConfig) Validate() error {
	if c.TracingSampleRate < 0 || c.TracingSampleRate > 1 {
		return fmt.Errorf("tracing sample rate must be between 0 and 1, got %f", c.TracingSampleRate)
	}

	// Only require these when observability is enabled.
	if c.Enabled {
		if c.ServiceName == "" {
			return fmt.Errorf("service name is required when observability is enabled")
		}
		if c.OTelEndpoint == "" {
			return fmt.Errorf("OTEL endpoint is required when observability is enabled")
		}
		if c.ServiceVersion == "" {
			return fmt.Errorf("service version is required when observability is enabled")
		}
	}

	return nil
}

// Load loads configuration from environment variables only.
// (Do .env loading in cmd/server/main.go for dev, not here.)
func Load() (*Config, error) {
	cfg := &Config{}

	if err := envconfig.Process("", &cfg.Server); err != nil {
		return nil, fmt.Errorf("failed to load Server config: %w", err)
	}
	if err := cfg.Server.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Server config: %w", err)
	}

	if err := envconfig.Process("", &cfg.Database); err != nil {
		return nil, fmt.Errorf("failed to load Database config: %w", err)
	}
	if err := cfg.Database.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Database config: %w", err)
	}

	if err := envconfig.Process("", &cfg.App); err != nil {
		return nil, fmt.Errorf("failed to load App config: %w", err)
	}
	if err := cfg.App.Validate(); err != nil {
		return nil, fmt.Errorf("invalid App config: %w", err)
	}

	if err := envconfig.Process("", &cfg.Observability); err != nil {
		return nil, fmt.Errorf("failed to load Observability config: %w", err)
	}
	if err := cfg.Observability.Validate(); err != nil {
		return nil, fmt.Errorf("invalid Observability config: %w", err)
	}

	return cfg, nil
}

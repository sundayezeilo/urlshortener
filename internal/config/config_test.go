package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Success(t *testing.T) {
	envVars := map[string]string{
		"SERVER_PORT":             "8080",
		"SERVER_HOST":             "0.0.0.0",
		"SERVER_BASE_URL":         "http://localhost:8080",
		"SERVER_READ_TIMEOUT":     "10s",
		"SERVER_WRITE_TIMEOUT":    "10s",
		"SERVER_IDLE_TIMEOUT":     "120s",
		"SERVER_SHUTDOWN_TIMEOUT": "30s",

		"DB_HOST":      "localhost",
		"DB_PORT":      "5432",
		"DB_USER":      "testuser",
		"DB_PASSWORD":  "testpass",
		"DB_NAME":      "testdb",
		"DB_SSLMODE":   "disable",
		"DB_MAX_CONNS": "25",
		"DB_MIN_CONNS": "5",

		"APP_ENV":   "test",
		"LOG_LEVEL": "debug",

		"OTEL_ENABLED":             "true",
		"OTEL_SERVICE_NAME":        "test-service",
		"OTEL_SERVICE_VERSION":     "1.0.0",
		"OTEL_ENDPOINT":            "localhost:4318",
		"OTEL_INSECURE":            "true",
		"OTEL_TRACING_SAMPLE_RATE": "1.0",
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Port != "8080" {
		t.Errorf("Server.Port = %s, want 8080", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %s, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.BaseURL != "http://localhost:8080" {
		t.Errorf("Server.BaseURL = %s, want http://localhost:8080", cfg.Server.BaseURL)
	}
	if cfg.Server.ReadTimeout != 10*time.Second {
		t.Errorf("Server.ReadTimeout = %v, want 10s", cfg.Server.ReadTimeout)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %s, want localhost", cfg.Database.Host)
	}
	if cfg.Database.Port != "5432" {
		t.Errorf("Database.Port = %s, want 5432", cfg.Database.Port)
	}
	if cfg.Database.User != "testuser" {
		t.Errorf("Database.User = %s, want testuser", cfg.Database.User)
	}
	if cfg.Database.MaxConns != 25 {
		t.Errorf("Database.MaxConns = %d, want 25", cfg.Database.MaxConns)
	}

	if cfg.App.Environment != "test" {
		t.Errorf("App.Environment = %s, want test", cfg.App.Environment)
	}
	if cfg.App.LogLevel != "debug" {
		t.Errorf("App.LogLevel = %s, want debug", cfg.App.LogLevel)
	}

	if !cfg.Observability.Enabled {
		t.Error("Observability.Enabled = false, want true")
	}
	if cfg.Observability.ServiceName != "test-service" {
		t.Errorf("Observability.ServiceName = %s, want test-service", cfg.Observability.ServiceName)
	}
	if cfg.Observability.TracingSampleRate != 1.0 {
		t.Errorf("Observability.TracingSampleRate = %f, want 1.0", cfg.Observability.TracingSampleRate)
	}
}

func TestLoad_MissingRequiredVariable(t *testing.T) {
	tests := []struct {
		name       string
		skipEnvVar string
	}{
		{"missing SERVER_PORT", "SERVER_PORT"},
		{"missing DB_HOST", "DB_HOST"},
		{"missing DB_NAME", "DB_NAME"},
		{"missing APP_ENV", "APP_ENV"},
		{"missing OTEL_ENABLED", "OTEL_ENABLED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()

			envVars := map[string]string{
				"SERVER_PORT":             "8080",
				"SERVER_HOST":             "0.0.0.0",
				"SERVER_BASE_URL":         "http://localhost:8080",
				"SERVER_READ_TIMEOUT":     "10s",
				"SERVER_WRITE_TIMEOUT":    "10s",
				"SERVER_IDLE_TIMEOUT":     "120s",
				"SERVER_SHUTDOWN_TIMEOUT": "30s",

				"DB_HOST":      "localhost",
				"DB_PORT":      "5432",
				"DB_USER":      "testuser",
				"DB_PASSWORD":  "testpass",
				"DB_NAME":      "testdb",
				"DB_SSLMODE":   "disable",
				"DB_MAX_CONNS": "25",
				"DB_MIN_CONNS": "5",

				"APP_ENV":   "test",
				"LOG_LEVEL": "debug",

				"OTEL_ENABLED":             "true",
				"OTEL_SERVICE_NAME":        "test-service",
				"OTEL_SERVICE_VERSION":     "1.0.0",
				"OTEL_ENDPOINT":            "localhost:4318",
				"OTEL_INSECURE":            "true",
				"OTEL_TRACING_SAMPLE_RATE": "1.0",
			}

			delete(envVars, tt.skipEnvVar)

			for key, value := range envVars {
				_ = os.Setenv(key, value)
			}

			_, err := Load()
			if err == nil {
				t.Errorf("Load() should fail when %s is missing", tt.skipEnvVar)
			}
		})
	}
}

func TestLoad_InvalidTypeConversion(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		value  string
	}{
		{"invalid duration", "SERVER_READ_TIMEOUT", "invalid"},
		{"invalid int", "DB_MAX_CONNS", "not-a-number"},
		{"invalid bool", "OTEL_ENABLED", "maybe"},
		{"invalid float", "OTEL_TRACING_SAMPLE_RATE", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := map[string]string{
				"SERVER_PORT":             "8080",
				"SERVER_HOST":             "0.0.0.0",
				"SERVER_BASE_URL":         "http://localhost:8080",
				"SERVER_READ_TIMEOUT":     "10s",
				"SERVER_WRITE_TIMEOUT":    "10s",
				"SERVER_IDLE_TIMEOUT":     "120s",
				"SERVER_SHUTDOWN_TIMEOUT": "30s",

				"DB_HOST":      "localhost",
				"DB_PORT":      "5432",
				"DB_USER":      "testuser",
				"DB_PASSWORD":  "testpass",
				"DB_NAME":      "testdb",
				"DB_SSLMODE":   "disable",
				"DB_MAX_CONNS": "25",
				"DB_MIN_CONNS": "5",

				"APP_ENV":   "test",
				"LOG_LEVEL": "debug",

				"OTEL_ENABLED":             "true",
				"OTEL_SERVICE_NAME":        "test-service",
				"OTEL_SERVICE_VERSION":     "1.0.0",
				"OTEL_ENDPOINT":            "localhost:4318",
				"OTEL_INSECURE":            "true",
				"OTEL_TRACING_SAMPLE_RATE": "1.0",
			}

			envVars[tt.envVar] = tt.value

			for key, value := range envVars {
				t.Setenv(key, value)
			}

			_, err := Load()
			if err == nil {
				t.Errorf("Load() should fail when %s has invalid value %s", tt.envVar, tt.value)
			}
		})
	}
}

func TestDatabaseConfig_ConnectionString(t *testing.T) {
	db := DatabaseConfig{
		Host:     "testhost",
		Port:     "5432",
		User:     "testuser",
		Password: "testpass",
		Name:     "testdb",
		SSLMode:  "disable",
	}

	expected := "host=testhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	got := db.ConnectionString()

	if got != expected {
		t.Errorf("ConnectionString() = %s, want %s", got, expected)
	}
}

func TestLoad_DurationParsing_WhenOTelDisabled_DoesNotRequireOTelFields(t *testing.T) {
	envVars := map[string]string{
		"SERVER_PORT":             "8080",
		"SERVER_HOST":             "0.0.0.0",
		"SERVER_BASE_URL":         "http://localhost:8080",
		"SERVER_READ_TIMEOUT":     "5m",
		"SERVER_WRITE_TIMEOUT":    "30s",
		"SERVER_IDLE_TIMEOUT":     "2h",
		"SERVER_SHUTDOWN_TIMEOUT": "1m30s",

		"DB_HOST":      "localhost",
		"DB_PORT":      "5432",
		"DB_USER":      "testuser",
		"DB_PASSWORD":  "testpass",
		"DB_NAME":      "testdb",
		"DB_SSLMODE":   "disable",
		"DB_MAX_CONNS": "25",
		"DB_MIN_CONNS": "5",

		"APP_ENV":   "test",
		"LOG_LEVEL": "debug",

		"OTEL_ENABLED": "false",
		// Intentionally omitting other OTEL_* vars
	}

	for key, value := range envVars {
		t.Setenv(key, value)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.ReadTimeout != 5*time.Minute {
		t.Errorf("Server.ReadTimeout = %v, want 5m", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30*time.Second {
		t.Errorf("Server.WriteTimeout = %v, want 30s", cfg.Server.WriteTimeout)
	}
	if cfg.Server.IdleTimeout != 2*time.Hour {
		t.Errorf("Server.IdleTimeout = %v, want 2h", cfg.Server.IdleTimeout)
	}
	if cfg.Server.ShutdownTimeout != 90*time.Second {
		t.Errorf("Server.ShutdownTimeout = %v, want 1m30s", cfg.Server.ShutdownTimeout)
	}

	if cfg.Observability.Enabled {
		t.Errorf("Observability.Enabled = true, want false")
	}
}

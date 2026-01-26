package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/sundayezeilo/urlshortener/internal/config"
	db "github.com/sundayezeilo/urlshortener/internal/db/sqlc"
	"github.com/sundayezeilo/urlshortener/internal/server"
	"github.com/sundayezeilo/urlshortener/internal/shortener"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	// Load .env only in non-production environments
	// In production/CI, env vars should already be set.
	env := os.Getenv("APP_ENV")
	if env == "development" || env == "test" {
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("no .env file found.")
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger := setupLogger(cfg.App.LogLevel)

	logger.Info("starting application",
		"env", cfg.App.Environment,
		"version", cfg.Observability.ServiceVersion,
	)

	dbPool, err := connectDatabase(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer dbPool.Close()

	// Setup dependencies
	queries := db.New(dbPool)
	repo := shortener.NewRepository(queries, nil)
	svc := shortener.NewService(repo, nil)
	handler := shortener.NewHandler(shortener.HandlerConfig{
		Service: svc,
		Logger:  logger,
		BaseURL: cfg.Server.BaseURL,
	})

	// Create and start server
	srv := server.New(cfg, logger, handler)

	logger.Info("server ready",
		"port", cfg.Server.Port,
		"base_url", cfg.Server.BaseURL,
	)

	// Start server (blocks until shutdown)
	if err := srv.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// setupLogger creates a structured logger based on the log level.
func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}

// connectDatabase establishes a connection to the PostgreSQL database.
func connectDatabase(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Set pool configuration
	poolConfig.MaxConns = cfg.Database.MaxConns
	poolConfig.MinConns = cfg.Database.MinConns

	logger.Info("connecting to database",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"database", cfg.Database.Name,
	)

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection established")

	return pool, nil
}

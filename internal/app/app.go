package app

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

// App holds the application dependencies and configuration.
type App struct {
	Config  *config.Config
	Logger  *slog.Logger
	DBPool  *pgxpool.Pool
	Server  *server.Server
	Handler *shortener.Handler
}

// New initializes and returns a new App instance with all dependencies wired up.
func New(ctx context.Context) (*App, error) {
	if err := loadEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger := setupLogger(cfg.App.LogLevel)

	logger.Info("starting application",
		"env", cfg.App.Environment,
		"version", cfg.Observability.ServiceVersion,
	)

	// Connect to database
	dbPool, err := connectDatabase(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Setup application dependencies
	queries := db.New(dbPool)
	repo := shortener.NewRepository(queries, nil)
	svc := shortener.NewService(repo, nil)
	handler := shortener.NewHandler(shortener.HandlerConfig{
		Service: svc,
		Logger:  logger,
		BaseURL: cfg.Server.BaseURL,
	})

	// Create server
	srv := server.New(cfg, logger, handler)

	logger.Info("application initialized",
		"port", cfg.Server.Port,
		"base_url", cfg.Server.BaseURL,
	)

	return &App{
		Config:  cfg,
		Logger:  logger,
		DBPool:  dbPool,
		Server:  srv,
		Handler: handler,
	}, nil
}

// Start starts the application server.
func (a *App) Start(ctx context.Context) error {
	a.Logger.Info("server starting",
		"port", a.Config.Server.Port,
		"base_url", a.Config.Server.BaseURL,
	)

	if err := a.Server.Start(ctx); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the application.
func (a *App) Shutdown() error {
	a.Logger.Info("shutting down application")

	if a.DBPool != nil {
		a.DBPool.Close()
		a.Logger.Info("database connection closed")
	}

	return nil
}

// loadEnv loads .env file only in non-production environments.
func loadEnv() error {
	env := os.Getenv("APP_ENV")
	if env == "development" || env == "test" {
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("no .env file found.")
		}
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

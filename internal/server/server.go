package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/sundayezeilo/urlshortener/internal/config"
	"github.com/sundayezeilo/urlshortener/internal/httpx"
	"github.com/sundayezeilo/urlshortener/internal/shortener"
)

// Server represents the HTTP server with all dependencies.
type Server struct {
	config  *config.Config
	logger  *slog.Logger
	handler *shortener.Handler
	server  *http.Server
}

// New creates a new Server instance.
func New(cfg *config.Config, logger *slog.Logger, handler *shortener.Handler) *Server {
	return &Server{
		config:  cfg,
		logger:  logger,
		handler: handler,
	}
}

// Start starts the HTTP server and blocks until shutdown.
func (s *Server) Start(ctx context.Context) error {
	mux := s.setupRoutes()
	handler := s.applyMiddleware(mux)
	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%s", s.config.Server.Host, s.config.Server.Port),
		Handler:      handler,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
	}

	// Listen for errors from the server
	serverErrors := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		s.logger.Info("starting http server",
			"addr", s.server.Addr,
			"env", s.config.App.Environment,
		)
		serverErrors <- s.server.ListenAndServe()
	}()

	// Listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		s.logger.Info("received shutdown signal", "signal", sig.String())

		// Create context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), s.config.Server.ShutdownTimeout)
		defer cancel()

		// Attempt graceful shutdown
		if err := s.server.Shutdown(ctx); err != nil {
			// Force close if graceful shutdown fails
			if closeErr := s.server.Close(); closeErr != nil {
				return fmt.Errorf("failed to close server: %w", closeErr)
			}
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		s.logger.Info("server stopped gracefully")
		return nil
	}
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /x/health", s.healthCheckHandler)

	mux.HandleFunc("POST /api/links", s.handler.CreateLink)
	mux.HandleFunc("GET /{slug}", s.handler.ResolveLink)

	return mux
}

// applyMiddleware wraps the handler with middleware in the correct order.
func (s *Server) applyMiddleware(handler http.Handler) http.Handler {
	return httpx.Chain(
		httpx.Recovery(s.logger), // Outermost: catch panics
		httpx.RequestID,          // Add request ID
		httpx.Logger(s.logger),   // Log requests
		httpx.CORS(nil),          // CORS headers (allow all in dev)
	)(handler)
}

// healthCheckHandler handles health check requests.
func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": s.config.Observability.ServiceName,
		"version": s.config.Observability.ServiceVersion,
	})
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("shutting down server")

	if err := s.server.Shutdown(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			s.logger.Warn("shutdown timeout exceeded, forcing close")
			return s.server.Close()
		}
		return err
	}

	return nil
}

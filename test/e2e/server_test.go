package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/sundayezeilo/urlshortener/internal/config"
	db "github.com/sundayezeilo/urlshortener/internal/db/sqlc"
	"github.com/sundayezeilo/urlshortener/internal/httpx"
	"github.com/sundayezeilo/urlshortener/internal/server"
	"github.com/sundayezeilo/urlshortener/internal/shortener"
)

// testApp holds the application components for e2e testing
type testApp struct {
	server  *server.Server
	dbPool  *pgxpool.Pool
	handler *shortener.Handler
	baseURL string
	cleanup func()
}

// setupTestApp creates a test application with a real database
func setupTestApp(t *testing.T) *testApp {
	t.Helper()
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Connect to database
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2

	dbPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Verify connection
	if err := dbPool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	// Run migrations
	if err := runMigrations(connStr); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Setup application components
	queries := db.New(dbPool)
	repo := shortener.NewRepository(queries, nil)
	svc := shortener.NewService(repo, nil)

	// Create test logger (suppress output in tests)
	logger := setupTestLogger()

	// Create handler
	baseURL := "http://localhost:8080"
	handler := shortener.NewHandler(shortener.HandlerConfig{
		Service: svc,
		Logger:  logger,
		BaseURL: baseURL,
	})

	// Create test config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:            "8080",
			Host:            "localhost",
			BaseURL:         baseURL,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
		App: config.AppConfig{
			Environment: "test",
			LogLevel:    "error",
		},
		Observability: config.ObservabilityConfig{
			ServiceName:    "urlshortener-test",
			ServiceVersion: "test",
		},
	}

	// Create server
	srv := server.New(cfg, logger, handler)

	// Cleanup function
	cleanup := func() {
		dbPool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}

	return &testApp{
		server:  srv,
		dbPool:  dbPool,
		handler: handler,
		baseURL: baseURL,
		cleanup: cleanup,
	}
}

func TestHealthCheck(t *testing.T) {
	app := setupTestApp(t)
	defer app.cleanup()

	req := httptest.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	// Create test server with middleware
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "urlshortener-test",
			"version": "test",
		})
	})

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got %s", response["status"])
	}
}

func TestCreateLink_E2E(t *testing.T) {
	app := setupTestApp(t)
	defer app.cleanup()

	tests := []struct {
		name           string
		requestBody    map[string]string
		expectedStatus int
		checkResponse  func(*testing.T, map[string]any)
	}{
		{
			name: "create link with auto-generated slug",
			requestBody: map[string]string{
				"url": "https://example.com/test",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]any) {
				if resp["slug"] == nil || resp["slug"] == "" {
					t.Error("expected slug to be generated")
				}
				if resp["original_url"] != "https://example.com/test" {
					t.Errorf("expected original_url 'https://example.com/test', got %v", resp["original_url"])
				}
				if resp["short_url"] == nil {
					t.Error("expected short_url to be set")
				}
			},
		},
		{
			name: "create link with custom slug",
			requestBody: map[string]string{
				"url":         "https://example.com/custom",
				"custom_slug": "my-custom-slug",
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]any) {
				if resp["slug"] != "my-custom-slug" {
					t.Errorf("expected slug 'my-custom-slug', got %v", resp["slug"])
				}
				if resp["original_url"] != "https://example.com/custom" {
					t.Errorf("expected original_url 'https://example.com/custom', got %v", resp["original_url"])
				}
			},
		},
		{
			name: "create link with duplicate custom slug",
			requestBody: map[string]string{
				"url":         "https://example.com/duplicate",
				"custom_slug": "duplicate-slug",
			},
			expectedStatus: http.StatusCreated,
			checkResponse:  func(t *testing.T, resp map[string]any) {},
		},
		{
			name:           "missing url",
			requestBody:    map[string]string{},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  func(t *testing.T, resp map[string]any) {},
		},
		{
			name: "invalid url format",
			requestBody: map[string]string{
				"url": "not-a-valid-url",
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse:  func(t *testing.T, resp map[string]any) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app.handler.CreateLink(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
				t.Logf("response body: %s", rr.Body.String())
			}

			if tt.expectedStatus == http.StatusCreated {
				var response map[string]any
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.checkResponse(t, response)
			}
		})
	}
}

func TestResolveLink_E2E(t *testing.T) {
	app := setupTestApp(t)
	defer app.cleanup()

	// First, create a link
	createBody := map[string]string{
		"url":         "https://example.com/redirect-test",
		"custom_slug": "test-redirect",
	}
	body, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()

	app.handler.CreateLink(createRR, createReq)

	if createRR.Code != http.StatusCreated {
		t.Fatalf("failed to create link: status %d", createRR.Code)
	}

	// Now test resolving the link
	tests := []struct {
		name           string
		slug           string
		expectedStatus int
		expectedURL    string
	}{
		{
			name:           "resolve existing slug",
			slug:           "test-redirect",
			expectedStatus: http.StatusFound,
			expectedURL:    "https://example.com/redirect-test",
		},
		{
			name:           "resolve non-existent slug",
			slug:           "non-existent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/"+tt.slug, nil)
			rr := httptest.NewRecorder()

			app.handler.ResolveLink(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedStatus == http.StatusFound {
				location := rr.Header().Get("Location")
				if location != tt.expectedURL {
					t.Errorf("expected location %s, got %s", tt.expectedURL, location)
				}
			}
		})
	}
}

func TestDuplicateSlug_E2E(t *testing.T) {
	app := setupTestApp(t)
	defer app.cleanup()

	// Create first link
	createBody := map[string]string{
		"url":         "https://example.com/first",
		"custom_slug": "duplicate-test",
	}
	body, _ := json.Marshal(createBody)
	req1 := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	rr1 := httptest.NewRecorder()

	app.handler.CreateLink(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Fatalf("failed to create first link: status %d", rr1.Code)
	}

	// Try to create second link with same slug
	createBody2 := map[string]string{
		"url":         "https://example.com/second",
		"custom_slug": "duplicate-test",
	}
	body2, _ := json.Marshal(createBody2)
	req2 := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()

	app.handler.CreateLink(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("expected status 409 (conflict), got %d", rr2.Code)
	}

	var errorResp map[string]any
	if err := json.NewDecoder(rr2.Body).Decode(&errorResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errorResp["error"] != "conflict" {
		t.Errorf("expected error code 'conflict', got %v", errorResp["error"])
	}
}

func TestAccessCountTracking_E2E(t *testing.T) {
	app := setupTestApp(t)
	defer app.cleanup()

	ctx := context.Background()

	createBody := map[string]string{
		"url":         "https://example.com/track-test",
		"custom_slug": "track-access",
	}
	body, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()

	app.handler.CreateLink(createRR, createReq)

	if createRR.Code != http.StatusCreated {
		t.Fatalf("failed to create link: status %d", createRR.Code)
	}

	// Resolve the link multiple times
	for i := range 3 {
		req := httptest.NewRequest("GET", "/track-access", nil)
		rr := httptest.NewRecorder()
		app.handler.ResolveLink(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("resolve attempt %d failed with status %d", i+1, rr.Code)
		}
	}

	// Check access count in database
	queries := db.New(app.dbPool)
	link, err := queries.GetLinkBySLug(ctx, "track-access")
	if err != nil {
		t.Fatalf("failed to get link from database: %v", err)
	}

	if link.AccessCount != 3 {
		t.Errorf("expected access count 3, got %d", link.AccessCount)
	}

	if !link.LastAccessedAt.Valid {
		t.Error("expected last_accessed_at to be set")
	}
}

func TestConcurrentLinkCreation_E2E(t *testing.T) {
	app := setupTestApp(t)
	defer app.cleanup()

	// Create multiple links concurrently with auto-generated slugs
	concurrency := 10
	errChan := make(chan error, concurrency)
	slugChan := make(chan string, concurrency)

	for i := range concurrency {
		go func(index int) {
			createBody := map[string]string{
				"url": fmt.Sprintf("https://example.com/concurrent-%d", index),
			}
			body, _ := json.Marshal(createBody)
			req := httptest.NewRequest("POST", "/api/links", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app.handler.CreateLink(rr, req)

			if rr.Code != http.StatusCreated {
				errChan <- fmt.Errorf("request %d failed with status %d", index, rr.Code)
				return
			}

			var response map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
				errChan <- err
				return
			}

			slugChan <- response["slug"].(string)
			errChan <- nil
		}(i)
	}

	// Collect results
	slugs := make(map[string]bool)
	for i := range concurrency {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent request failed: %v", err)
		}
		if i < concurrency {
			slug := <-slugChan
			if slugs[slug] {
				t.Errorf("duplicate slug generated: %s", slug)
			}
			slugs[slug] = true
		}
	}

	if len(slugs) != concurrency {
		t.Errorf("expected %d unique slugs, got %d", concurrency, len(slugs))
	}
}

// Helper functions

func runMigrations(connStr string) error {
	// This is a simplified migration runner for tests
	// In production, you'd use golang-migrate or similar
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Read and execute migration
	migrationSQL := `
			CREATE TABLE links (
		    id               UUID PRIMARY KEY,
		    original_url     TEXT NOT NULL,
		    slug             TEXT NOT NULL,
		    access_count     BIGINT NOT NULL DEFAULT 0,
		    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
		    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
		    last_accessed_at TIMESTAMPTZ,

		    CONSTRAINT links_slug_unique UNIQUE (slug),
		    CONSTRAINT links_slug_length CHECK (char_length(slug) BETWEEN 7 AND 64)
		);

		CREATE OR REPLACE FUNCTION set_updated_at()
		RETURNS trigger AS $$
		BEGIN
			IF (NEW IS DISTINCT FROM OLD) THEN
				NEW.updated_at = now();
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		DROP TRIGGER IF EXISTS links_set_updated_at ON links;

		CREATE TRIGGER links_set_updated_at
		BEFORE UPDATE ON links
		FOR EACH ROW
		EXECUTE FUNCTION set_updated_at();
	`

	_, err = pool.Exec(ctx, migrationSQL)
	return err
}

func setupTestLogger() *slog.Logger {
	// Create a no-op logger for tests
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	})
	return slog.New(handler)
}

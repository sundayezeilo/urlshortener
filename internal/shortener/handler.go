package shortener

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/sundayezeilo/urlshortener/internal/errx"
	"github.com/sundayezeilo/urlshortener/internal/httpx"
)

// HTTPCreateLinkRequest represents the JSON request body for creating a link.
type HTTPCreateLinkRequest struct {
	URL        string `json:"url"`
	CustomSlug string `json:"custom_slug,omitempty"`
}

// CreateLinkResponse represents the JSON response for a created link.
type CreateLinkResponse struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	OriginalURL string `json:"original_url"`
	ShortURL    string `json:"short_url"`
	CreatedAt   string `json:"created_at"`
}

// Handler provides HTTP handlers for the URL shortener service.
type Handler struct {
	service Service
	logger  *slog.Logger
	baseURL string
}

// HandlerConfig holds configuration for the handler.
type HandlerConfig struct {
	Service Service
	Logger  *slog.Logger
	BaseURL string // Base URL for constructing short URLs (e.g., "https://short.ly")
}

// NewHandler creates a new Handler instance.
func NewHandler(cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		service: cfg.Service,
		logger:  logger,
		baseURL: cfg.BaseURL,
	}
}

// CreateLink handles POST requests to create a new short link.
func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract request ID for tracing
	requestID := httpx.GetRequestID(ctx)

	logger := h.logger.With(
		"request_id", requestID,
		"method", r.Method,
		"path", r.URL.Path,
	)

	// Decode and validate request
	req, err := httpx.DecodeJSON[HTTPCreateLinkRequest](r)
	if err != nil {
		logger.WarnContext(ctx, "failed to decode request",
			"error", err.Error(),
		)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), nil)
		return
	}

	// Validate required fields
	if err := validateCreateRequest(req); err != nil {
		logger.WarnContext(ctx, "request validation failed",
			"error", err.Error(),
			"url", req.URL,
			"custom_slug", req.CustomSlug,
		)
		httpx.WriteError(w, http.StatusBadRequest, "validation_failed", err.Error(), nil)
		return
	}

	link, err := h.service.Create(ctx, CreateLinkRequest{
		OriginalURL: req.URL,
		CustomSlug:  req.CustomSlug,
	})
	if err != nil {
		h.handleCreateError(ctx, w, err)
		return
	}

	resp := CreateLinkResponse{
		ID:          link.ID.String(),
		Slug:        link.Slug,
		OriginalURL: link.OriginalURL,
		ShortURL:    fmt.Sprintf("%s/%s", h.baseURL, link.Slug),
		CreatedAt:   link.CreatedAt.Format(http.TimeFormat),
	}

	logger.InfoContext(ctx, "link created successfully",
		"link_id", link.ID.String(),
		"slug", link.Slug,
		"custom_slug", req.CustomSlug != "",
	)

	httpx.WriteJSON(w, http.StatusCreated, resp)
}

// ResolveLink handles GET requests to resolve a slug and redirect to the original URL.
// This increments the access count and updates tracking metadata.
func (h *Handler) ResolveLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract request ID for tracing
	requestID := httpx.GetRequestID(ctx)

	logger := h.logger.With(
		"request_id", requestID,
		"method", r.Method,
		"path", r.URL.Path,
	)

	// Extract slug from URL path
	slug := extractSlugFromPath(r.URL.Path)
	if slug == "" {
		logger.WarnContext(ctx, "missing slug in path")
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "slug is required", nil)
		return
	}

	if err := validateSlugFormat(slug); err != nil {
		logger.WarnContext(ctx, "invalid slug format",
			"slug", slug,
			"error", err.Error(),
		)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_slug", err.Error(), nil)
		return
	}

	originalURL, err := h.service.Resolve(ctx, slug)
	if err != nil {
		h.handleResolveError(ctx, w, err, slug)
		return
	}

	logger.InfoContext(ctx, "slug resolved successfully",
		"slug", slug,
		"original_url", originalURL,
		"user_agent", r.UserAgent(),
		"referer", r.Referer(),
	)

	http.Redirect(w, r, originalURL, http.StatusFound)
}

// handleCreateError handles errors from the Create service method.
func (h *Handler) handleCreateError(ctx context.Context, w http.ResponseWriter, err error) {
	kind := errx.KindOf(err)

	logAttrs := []any{
		"error", err.Error(),
		"error_kind", kind,
		"operation", errx.OpOf(err),
	}

	switch kind {
	case errx.Conflict:
		h.logger.WarnContext(ctx, "slug conflict", logAttrs...)
		httpx.WriteError(w, http.StatusConflict, "conflict",
			"This slug is already taken",
			map[string]string{
				"hint": "Try a different custom slug or let us generate one for you",
			})

	case errx.Invalid:
		h.logger.WarnContext(ctx, "invalid link request", logAttrs...)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error(), nil)

	case errx.Unavailable:
		h.logger.ErrorContext(ctx, "service unavailable", logAttrs...)
		httpx.WriteError(w, http.StatusServiceUnavailable, "unavailable",
			"Unable to create short link at this time. Please try again.", nil)

	default:
		h.logger.ErrorContext(ctx, "unexpected error creating link", logAttrs...)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error",
			"Unable to create short link at this time. Please try again.", nil)
	}
}

// handleResolveError handles errors from the Resolve service method.
func (h *Handler) handleResolveError(ctx context.Context, w http.ResponseWriter, err error, slug string) {
	kind := errx.KindOf(err)

	logAttrs := []any{
		"error", err.Error(),
		"error_kind", kind,
		"operation", errx.OpOf(err),
		"slug", slug,
	}

	switch kind {
	case errx.NotFound:
		h.logger.WarnContext(ctx, "slug not found", logAttrs...)
		httpx.WriteError(w, http.StatusNotFound, "not_found",
			"short link doesn't exist", nil)

	case errx.Invalid:
		h.logger.WarnContext(ctx, "invalid slug", logAttrs...)
		httpx.WriteError(w, http.StatusBadRequest, "invalid_slug", err.Error(), nil)

	default:
		h.logger.ErrorContext(ctx, "unexpected error resolving link", logAttrs...)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error",
			"Unable to resolve this link at this time", nil)
	}
}

// validateCreateRequest validates the HTTPCreateLinkRequest.
func validateCreateRequest(req HTTPCreateLinkRequest) error {
	if req.URL == "" {
		return errors.New("url is required")
	}
	return nil
}

// validateSlugFormat performs basic slug format validation for HTTP layer.
// This is a lightweight check before calling the service layer.
func validateSlugFormat(slug string) error {
	if slug == "" {
		return errors.New("invalid link")
	}

	if len(slug) > MaxSlugLength {
		return errors.New("invalid link")
	}
	return nil
}

// extractSlugFromPath extracts the slug from a URL path.
// For example, "/abc123" returns "abc123", "/s/abc123" returns "abc123".
func extractSlugFromPath(path string) string {
	// Remove leading slash
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// If path contains more segments (e.g., "/s/abc123"), take the last one
	// For simple cases (e.g., "/abc123"), this will just return the slug
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			lastSlash = i
			break
		}
	}

	if lastSlash >= 0 {
		return path[lastSlash+1:]
	}

	return path
}

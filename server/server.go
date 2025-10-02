package server

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/bastienwirtz/corsair/config"
	"github.com/bastienwirtz/corsair/handlers"
	"github.com/bastienwirtz/corsair/middleware"
)

// Handler implements the http.Handler interface with dynamic routing
// based on configuration. Provides CORS middleware and trailing slash normalization.
type Handler struct {
	mux    *http.ServeMux
	config config.Config
}

// NewDynamicRoutingHandler creates a new handler with routes registered from configuration.
func NewDynamicRoutingHandler(cfg config.Config) *Handler {
	handler := &Handler{
		mux:    http.NewServeMux(),
		config: cfg,
	}
	handler.registerRoutes()
	return handler
}

// registerRoutes sets up all HTTP routes based on configuration.
// Handles both the optional forward endpoint and configured proxy endpoints.
func (h *Handler) registerRoutes() {
	h.mux = http.NewServeMux()

	corsMiddleware := middleware.CORS(h.config.CORS)
	slog.Debug("Initialized CORS middleware", "origins", h.config.CORS.Origins, "credentials", h.config.CORS.Credentials)

	// Register forward endpoint if enabled
	if h.config.Server.ForwardEndpointEnabled != nil && *h.config.Server.ForwardEndpointEnabled {
		h.mux.Handle("/forward/", corsMiddleware(handlers.ForwardHandler(h.config)))
		slog.Info("Forward endpoint enabled", "path", "/forward/")
	} else {
		slog.Info("Forward endpoint disabled")
	}

	// Register dynamic endpoints defined in configuration
	registeredCount := 0
	skippedCount := 0
	for _, endpoint := range h.config.Endpoints {
		path := endpoint.Path

		// Prevent registration of reserved internal endpoints
		if strings.TrimSuffix(path, "/") == "/forward" {
			slog.Warn("Skipping reserved endpoint", "path", path, "remote_url", endpoint.RemoteURL)
			skippedCount++
			continue
		}

		// Ensure path ends with "/" for proper HTTP routing behavior.
		// Go's HTTP router treats "/api" and "/api/" differently:
		// - "/api" matches only exactly "/api"
		// - "/api/" matches "/api/", "/api/foo", "/api/bar/baz", etc.
		// This allows our proxy to handle sub-paths correctly.
		// The trailing slash middleware normalizes incoming requests to
		// match routes with a ending "/".
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}

		// Create proxy handler that will forward requests to the remote URL.
		// The ProxyHandler handles path manipulation internally by stripping
		// the endpoint path and appending the remaining path to the remote URL.
		handler := handlers.ProxyHandler(endpoint, h.config)
		handler = corsMiddleware(handler)

		// Register the handler. No StripPrefix needed here since ProxyHandler
		// handles path processing internally.
		h.mux.Handle(path, handler)
		slog.Debug("Registered endpoint", "path", path, "remote_url", endpoint.RemoteURL)
		registeredCount++
	}

	slog.Info("Route registration complete",
		"registered", registeredCount,
		"skipped", skippedCount,
		"total_configured", len(h.config.Endpoints))
}

// ServeHTTP implements the http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply trailing slash middleware to the entire mux to normalize requests
	// before they reach the router, preventing unwanted redirects.
	trailingSlashMiddleware := middleware.TrailingSlash()
	handler := trailingSlashMiddleware(h.mux)

	handler.ServeHTTP(w, r)
}

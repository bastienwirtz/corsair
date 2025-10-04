package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/bastienwirtz/corsair/config"
)

// executeProxyRequest executes the HTTP request and copies the response back to the client.
func executeProxyRequest(proxyReq *http.Request, w http.ResponseWriter, timeout time.Duration, corsConfig config.CORSConfig) {

	corsHeaders := []string{
		"access-control-allow-origin",
		"access-control-allow-methods",
		"access-control-allow-headers",
		"access-control-allow-credentials",
	}

	client := &http.Client{
		Timeout: timeout,
	}
	logger := slog.With("url", proxyReq.URL.String(), "timeout", timeout)

	logger.Debug("Executing proxy request")
	resp, err := client.Do(proxyReq)
	if err != nil {
		logger.Error("Request failed", "error", err)
		http.Error(w, "Request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	logger.Debug("Received response", "status", resp.StatusCode)

	// Forward response headers to client
	for key, values := range resp.Header {
		if corsConfig.HasAnyConfiguration() && slices.Contains(corsHeaders, strings.ToLower(key)) {
			logger.Warn("Upstream server response includes CORS headers. Dropping them to prevent conflicts with corsair configured CORS headers", "header", key)
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)

	// Stream response body back to client
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.Error("Failed to copy response body", "error", err, "url", proxyReq.URL.String())
	}
}

// ProxyHandler creates an HTTP handler that proxies requests to a configured endpoint.
func ProxyHandler(endpoint config.Endpoint, cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := slog.With("endpoint_path", endpoint.Path, "request_path", r.URL.Path, "method", r.Method)
		logger.Debug("Processing proxy request")

		// Process template variables in endpoint configuration
		config.ProcessEndpointTemplates(&endpoint)

		targetURL, err := url.Parse(endpoint.RemoteURL)
		if err != nil {
			logger.Error("Invalid remote URL in endpoint config", "error", err)
			http.Error(w, "Invalid remote URL", http.StatusInternalServerError)
			return
		}

		// Strip endpoint path and construct target path
		path := strings.TrimPrefix(r.URL.Path, endpoint.Path)
		if path == "" || path[0] != '/' {
			path = "/" + path
		}

		targetURL.Path = strings.TrimSuffix(targetURL.Path, "/") + path
		targetURL.RawQuery = r.URL.RawQuery

		logger.Debug("Constructed target URL", "target_url", targetURL.String())

		proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
		if err != nil {
			logger.Error("Failed to create proxy request", "error", err)
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		// Forward original request headers
		for key, values := range r.Header {
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}

		// Apply configured headers (override original headers if same key)
		for _, headerMap := range endpoint.Headers {
			for key, value := range headerMap {
				proxyReq.Header.Set(key, value)
			}
		}

		// Apply configured query parameters
		q := proxyReq.URL.Query()
		for _, paramMap := range endpoint.QueryParams {
			for key, value := range paramMap {
				q.Set(key, value)
			}
		}
		proxyReq.URL.RawQuery = q.Encode()
		proxyReq.Host = targetURL.Host

		timeout := cfg.GetEffectiveTimeout(endpoint)
		executeProxyRequest(proxyReq, w, timeout, cfg.CORS)
	})
}

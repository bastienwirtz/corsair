package handlers

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/bastienwirtz/corsair/config"
)

// ForwardHandler creates an HTTP handler for the /forward endpoint that allows
// ad-hoc proxying to any URL specified in the 'url' query parameter.
func ForwardHandler(cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetURLStr := r.URL.Query().Get("url")
		if targetURLStr == "" {
			slog.Warn("Forward request missing URL parameter", "path", r.URL.Path, "remote_addr", r.RemoteAddr)
			http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
			return
		}

		slog.Debug("Processing forward request", "target_url", targetURLStr, "method", r.Method, "remote_addr", r.RemoteAddr)

		targetURL, err := url.Parse(targetURLStr)
		if err != nil {
			slog.Warn("Forward request with invalid URL", "error", err, "url", targetURLStr, "remote_addr", r.RemoteAddr)
			http.Error(w, "Invalid URL in 'url' parameter", http.StatusBadRequest)
			return
		}

		// Default to HTTPS for security
		if targetURL.Scheme == "" {
			targetURL.Scheme = "https"
			slog.Debug("Defaulting to HTTPS scheme", "url", targetURL.String())
		}

		// Security: only allow HTTP/HTTPS protocols
		if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
			slog.Warn("Forward request with disallowed scheme", "scheme", targetURL.Scheme, "url", targetURL.String(), "remote_addr", r.RemoteAddr)
			http.Error(w, "Only HTTP and HTTPS URLs are allowed", http.StatusBadRequest)
			return
		}

		proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
		if err != nil {
			slog.Error("Failed to create forward request", "error", err, "url", targetURL.String())
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		// Copy headers except Host (which is set explicitly)
		for key, values := range r.Header {
			if key == "Host" {
				continue
			}
			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}

		proxyReq.Host = targetURL.Host

		// Use server default timeout for forward endpoint
		timeout := cfg.GetDefaultTimeout()

		slog.Info("Forwarding request", "target_url", targetURL.String(), "method", r.Method, "timeout", timeout)
		executeProxyRequest(proxyReq, w, timeout, cfg.CORS)
	})
}

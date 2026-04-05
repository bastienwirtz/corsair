package middleware

import (
	"context"
	"net/http"
	"strings"
)

type corsContextKey string

const OriginalPathKey corsContextKey = "originalPath"

// TrailingSlash middleware ensures that requests to paths without trailing slashes
// are internally normalized to have trailing slashes before reaching the router.
// This prevents Go's http.ServeMux from issuing 301 redirects when endpoints
// are registered with trailing slashes.
func TrailingSlash() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Only add trailing slash if:
			// 1. Path doesn't already end with "/"
			// 2. Path is not just "/" (root path)
			// 3. Path doesn't contain a file extension (avoid breaking file requests)
			if path != "/" && !strings.HasSuffix(path, "/") && !strings.Contains(path[strings.LastIndex(path, "/")+1:], ".") {

				// Create a new URL with trailing slash
				newURL := *r.URL
				newURL.Path = path + "/"

				// Update the request URL
				r.URL = &newURL
			}

			// Store original path in request context so the final target url won't include any added "/"
			ctx := context.WithValue(r.Context(), OriginalPathKey, path)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

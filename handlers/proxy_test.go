package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bastienwirtz/corsair/config"
)

func TestProxyHandler(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "test-value")

		if r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"received":"` + string(body) + `"}`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok","path":"` + r.URL.Path + `"}`))
		}
	}))
	defer mockServer.Close()

	tests := []struct {
		name           string
		endpoint       config.Endpoint
		requestMethod  string
		requestPath    string
		requestBody    string
		requestHeaders map[string]string
		envVars        map[string]string
		expectedStatus int
	}{
		{
			name: "simple GET request",
			endpoint: config.Endpoint{
				Path:      "/api",
				RemoteURL: mockServer.URL,
			},
			requestMethod:  "GET",
			requestPath:    "/api/users",
			expectedStatus: http.StatusOK,
		},
		{
			name: "POST request with body",
			endpoint: config.Endpoint{
				Path:      "/api",
				RemoteURL: mockServer.URL,
			},
			requestMethod:  "POST",
			requestPath:    "/api/create",
			requestBody:    `{"name":"test"}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name: "request with custom headers",
			endpoint: config.Endpoint{
				Path:      "/api",
				RemoteURL: mockServer.URL,
				Headers: []map[string]string{
					{"Authorization": "Bearer {{ token }}"},
					{"X-Test": "custom-value"},
				},
			},
			requestMethod: "GET",
			requestPath:   "/api/protected",
			envVars: map[string]string{
				"token": "abc123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "request with query parameters",
			endpoint: config.Endpoint{
				Path:      "/search",
				RemoteURL: mockServer.URL,
				QueryParams: []map[string]string{
					{"version": "v1"},
					{"api_key": "{{ api_key }}"},
				},
			},
			requestMethod: "GET",
			requestPath:   "/search?q=test",
			envVars: map[string]string{
				"api_key": "key123",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "path with trailing slash",
			endpoint: config.Endpoint{
				Path:      "/api/",
				RemoteURL: mockServer.URL + "/v1",
			},
			requestMethod:  "GET",
			requestPath:    "/api/users/123",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			handler := ProxyHandler(tt.endpoint, config.Config{Server: config.ServerConfig{DefaultTimeout: "10s"}})

			var req *http.Request
			if tt.requestBody != "" {
				body := strings.NewReader(tt.requestBody)
				req = httptest.NewRequest(tt.requestMethod, tt.requestPath, body)
			} else {
				req = httptest.NewRequest(tt.requestMethod, tt.requestPath, nil)
			}

			for key, value := range tt.requestHeaders {
				req.Header.Set(key, value)
			}

			if tt.requestBody != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if w.Code < 400 {
				assert.NotEmpty(t, w.Body.String())
			}
		})
	}
}

func TestProxyHandlerInvalidRemoteURL(t *testing.T) {
	endpoint := config.Endpoint{
		Path:      "/test",
		RemoteURL: "invalid-url",
	}

	handler := ProxyHandler(endpoint, config.Config{Server: config.ServerConfig{DefaultTimeout: "1s"}})
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "Request failed")
}

func TestProxyHandlerUnreachableServer(t *testing.T) {
	endpoint := config.Endpoint{
		Path:      "/test",
		RemoteURL: "http://localhost:99999",
	}

	handler := ProxyHandler(endpoint, config.Config{Server: config.ServerConfig{DefaultTimeout: "1s"}})
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "Request failed")
}

func TestCORSHeaderFiltering(t *testing.T) {
	tests := []struct {
		name               string
		corsConfig         config.CORSConfig
		upstreamHeaders    map[string]string
		expectedToFilter   bool
		expectedHeaders    []string
		unexpectedHeaders  []string
	}{
		{
			name: "no CORS config - should pass through all headers",
			corsConfig: config.CORSConfig{},
			upstreamHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "*",
				"Access-Control-Allow-Methods":     "GET, POST",
				"Access-Control-Allow-Headers":     "Content-Type",
				"Access-Control-Allow-Credentials": "true",
				"Content-Type":                     "application/json",
			},
			expectedToFilter: false,
			expectedHeaders: []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Methods",
				"Access-Control-Allow-Headers",
				"Access-Control-Allow-Credentials",
				"Content-Type",
			},
		},
		{
			name: "CORS origins configured - should filter CORS headers",
			corsConfig: config.CORSConfig{
				Origins: []string{"http://localhost:3000"},
			},
			upstreamHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "*",
				"Access-Control-Allow-Methods":     "GET, POST",
				"Access-Control-Allow-Headers":     "Content-Type",
				"Access-Control-Allow-Credentials": "true",
				"Content-Type":                     "application/json",
			},
			expectedToFilter: true,
			expectedHeaders: []string{"Content-Type"},
			unexpectedHeaders: []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Methods",
				"Access-Control-Allow-Headers",
				"Access-Control-Allow-Credentials",
			},
		},
		{
			name: "CORS methods configured - should filter CORS headers",
			corsConfig: config.CORSConfig{
				Methods: "GET, POST, PUT",
			},
			upstreamHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, POST",
				"Content-Type":                 "application/json",
				"X-Custom-Header":              "value",
			},
			expectedToFilter: true,
			expectedHeaders: []string{"Content-Type", "X-Custom-Header"},
			unexpectedHeaders: []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Methods",
			},
		},
		{
			name: "CORS credentials enabled - should filter CORS headers",
			corsConfig: config.CORSConfig{
				Credentials: true,
			},
			upstreamHeaders: map[string]string{
				"Access-Control-Allow-Credentials": "false",
				"Content-Type":                     "application/json",
			},
			expectedToFilter: true,
			expectedHeaders: []string{"Content-Type"},
			unexpectedHeaders: []string{"Access-Control-Allow-Credentials"},
		},
		{
			name: "CORS headers configured - should filter CORS headers",
			corsConfig: config.CORSConfig{
				Headers: "Content-Type, Authorization",
			},
			upstreamHeaders: map[string]string{
				"Access-Control-Allow-Headers": "X-Requested-With",
				"Content-Type":                 "application/json",
			},
			expectedToFilter: true,
			expectedHeaders: []string{"Content-Type"},
			unexpectedHeaders: []string{"Access-Control-Allow-Headers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.upstreamHeaders {
					w.Header().Set(key, value)
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status":"ok"}`))
			}))
			defer mockServer.Close()

			endpoint := config.Endpoint{
				Path:      "/test",
				RemoteURL: mockServer.URL,
			}

			cfg := config.Config{
				Server: config.ServerConfig{DefaultTimeout: "10s"},
				CORS:   tt.corsConfig,
			}

			handler := ProxyHandler(endpoint, cfg)
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			for _, expectedHeader := range tt.expectedHeaders {
				assert.NotEmpty(t, w.Header().Get(expectedHeader), "Expected header %s to be present", expectedHeader)
			}

			for _, unexpectedHeader := range tt.unexpectedHeaders {
				assert.Empty(t, w.Header().Get(unexpectedHeader), "Expected header %s to be filtered out", unexpectedHeader)
			}
		})
	}
}

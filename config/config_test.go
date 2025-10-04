package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantErr  bool
		expected *Config
	}{
		{
			name: "valid config with wildcard origins and no credentials",
			content: `
server:
  address: "0.0.0.0"
  port: 8080
  forward_endpoint_enabled: true
  default_timeout: "15s"
cors:
  allow_origins: ["*"]
  allow_methods: "GET, POST"
  allow_headers: "*"
  allow_credentials: false
endpoints:
  - path: /test
    remote_url: http://example.com
    timeout: "30s"
    headers:
      - Authorization: "Bearer token"
    query_params:
      - key: value
`,
			wantErr: false,
			expected: &Config{
				Server: ServerConfig{
					Address:                "0.0.0.0",
					Port:                   8080,
					ForwardEndpointEnabled: &[]bool{true}[0],
					DefaultTimeout:         "15s",
				},
				CORS: CORSConfig{
					Origins:     []string{"*"},
					Methods:     "GET, POST",
					Headers:     "*",
					Credentials: false,
				},
				Logging: LoggingConfig{
					Level:  "info",
					Format: "text",
				},
				Endpoints: []Endpoint{
					{
						Path:        "/test",
						RemoteURL:   "http://example.com",
						Timeout:     "30s",
						Headers:     []map[string]string{{"Authorization": "Bearer token"}},
						QueryParams: []map[string]string{{"key": "value"}},
					},
				},
			},
		},
		{
			name: "invalid CORS config - wildcard origins with credentials",
			content: `
server:
  port: 8080
cors:
  allow_origins: ["*"]
  allow_methods: "GET, POST"
  allow_headers: "*"
  allow_credentials: true
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr: true,
		},
		{
			name: "invalid server timeout format",
			content: `
server:
  port: 8080
  default_timeout: "invalid"
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr: true,
		},
		{
			name: "invalid endpoint timeout format",
			content: `
server:
  port: 8080
  default_timeout: "10s"
endpoints:
  - path: /test
    remote_url: http://example.com
    timeout: "not-a-duration"
`,
			wantErr: true,
		},
		{
			name: "config with defaults",
			content: `
endpoints:
  - path: /api
    remote_url: http://api.example.com
`,
			wantErr: false,
		},
		{
			name: "invalid yaml",
			content: `
invalid: yaml: content: [
`,
			wantErr: true,
		},
		{
			name: "missing endpoints",
			content: `
cors:
  allow_origins: ["*"]
endpoints: []
`,
			wantErr: false,
		},
		{
			name: "endpoint missing path",
			content: `
endpoints:
  - remote_url: http://example.com
`,
			wantErr: true,
		},
		{
			name: "endpoint missing remote_url",
			content: `
endpoints:
  - path: /test
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "config-test-*.yaml")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.content)
			require.NoError(t, err)
			tmpFile.Close()

			config, err := LoadConfig(tmpFile.Name())

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)

				if tt.expected != nil {
					assert.Equal(t, tt.expected.Server, config.Server)
					assert.Equal(t, tt.expected.CORS, config.CORS)
					assert.Equal(t, tt.expected.Logging, config.Logging)
					assert.Equal(t, tt.expected.Endpoints, config.Endpoints)
				}

				assert.NotZero(t, config.Server.Port)
				assert.NotEmpty(t, config.Logging.Level)
				assert.NotEmpty(t, config.Logging.Format)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	config := &Config{}
	setDefaults(config)

	assert.Equal(t, "localhost", config.Server.Address)
	assert.Equal(t, 8080, config.Server.Port)
	assert.Equal(t, "10s", config.Server.DefaultTimeout)
	assert.NotNil(t, config.Server.ForwardEndpointEnabled)
	assert.True(t, *config.Server.ForwardEndpointEnabled)
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
	assert.Equal(t, []string(nil), config.CORS.Origins)
	assert.Equal(t, "", config.CORS.Methods)
	assert.Equal(t, "", config.CORS.Headers)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				CORS: CORSConfig{
					Origins:     []string{"http://localhost:3000"},
					Credentials: true,
				},
				Endpoints: []Endpoint{
					{Path: "/test", RemoteURL: "http://example.com"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with wildcard and no credentials",
			config: &Config{
				CORS: CORSConfig{
					Origins:     []string{"*"},
					Credentials: false,
				},
				Endpoints: []Endpoint{
					{Path: "/test", RemoteURL: "http://example.com"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid CORS: wildcard with credentials",
			config: &Config{
				CORS: CORSConfig{
					Origins:     []string{"*"},
					Credentials: true,
				},
				Endpoints: []Endpoint{
					{Path: "/test", RemoteURL: "http://example.com"},
				},
			},
			wantErr: true,
		},
		{
			name:    "no endpoints",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "endpoint missing path",
			config: &Config{
				Endpoints: []Endpoint{
					{RemoteURL: "http://example.com"},
				},
			},
			wantErr: true,
		},
		{
			name: "endpoint missing remote_url",
			config: &Config{
				Endpoints: []Endpoint{
					{Path: "/test"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCORSConfig(t *testing.T) {
	tests := []struct {
		name       string
		corsConfig CORSConfig
		wantErr    bool
		errorMsg   string
	}{
		{
			name: "valid - specific origins with credentials",
			corsConfig: CORSConfig{
				Origins:     []string{"http://localhost:3000", "https://example.com"},
				Credentials: true,
			},
			wantErr: false,
		},
		{
			name: "valid - wildcard without credentials",
			corsConfig: CORSConfig{
				Origins:     []string{"*"},
				Credentials: false,
			},
			wantErr: false,
		},
		{
			name: "invalid - wildcard with credentials",
			corsConfig: CORSConfig{
				Origins:     []string{"*"},
				Credentials: true,
			},
			wantErr:  true,
			errorMsg: "allow_origins cannot contain '*' when allow_credentials is true",
		},
		{
			name: "invalid - wildcard mixed with specific origins and credentials",
			corsConfig: CORSConfig{
				Origins:     []string{"http://example.com", "*", "https://other.com"},
				Credentials: true,
			},
			wantErr:  true,
			errorMsg: "allow_origins cannot contain '*' when allow_credentials is true",
		},
		{
			name: "valid - empty origins with credentials",
			corsConfig: CORSConfig{
				Origins:     []string{},
				Credentials: true,
			},
			wantErr: false,
		},
		{
			name: "invalid - wildcard mixed with specific origins without credentials",
			corsConfig: CORSConfig{
				Origins:     []string{"*", "https://example.com"},
				Credentials: false,
			},
			wantErr:  true,
			errorMsg: "'*' wildcard must be the only origin when used",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCORSConfig(&tt.corsConfig)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCORSConfigHasAnyConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		corsConfig CORSConfig
		expected   bool
	}{
		{
			name:       "no configuration",
			corsConfig: CORSConfig{},
			expected:   false,
		},
		{
			name: "only origins configured",
			corsConfig: CORSConfig{
				Origins: []string{"http://localhost:3000"},
			},
			expected: true,
		},
		{
			name: "only methods configured",
			corsConfig: CORSConfig{
				Methods: "GET, POST",
			},
			expected: true,
		},
		{
			name: "only headers configured",
			corsConfig: CORSConfig{
				Headers: "Content-Type",
			},
			expected: true,
		},
		{
			name: "only credentials enabled",
			corsConfig: CORSConfig{
				Credentials: true,
			},
			expected: true,
		},
		{
			name: "empty origins slice",
			corsConfig: CORSConfig{
				Origins: []string{},
			},
			expected: false,
		},
		{
			name: "empty methods string",
			corsConfig: CORSConfig{
				Methods: "",
			},
			expected: false,
		},
		{
			name: "empty headers string",
			corsConfig: CORSConfig{
				Headers: "",
			},
			expected: false,
		},
		{
			name: "credentials false",
			corsConfig: CORSConfig{
				Credentials: false,
			},
			expected: false,
		},
		{
			name: "multiple configurations",
			corsConfig: CORSConfig{
				Origins:     []string{"*"},
				Methods:     "GET, POST",
				Headers:     "Content-Type",
				Credentials: true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.corsConfig.HasAnyConfiguration()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServerAddressConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantErr  bool
		expected string
	}{
		{
			name: "explicit localhost address",
			content: `
server:
  address: "localhost"
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "localhost",
		},
		{
			name: "explicit 0.0.0.0 address",
			content: `
server:
  address: "0.0.0.0"
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "0.0.0.0",
		},
		{
			name: "explicit 127.0.0.1 address",
			content: `
server:
  address: "127.0.0.1"
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "127.0.0.1",
		},
		{
			name: "no address specified - should default to localhost",
			content: `
server:
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "localhost",
		},
		{
			name: "empty address - should default to localhost",
			content: `
server:
  address: ""
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "localhost",
		},
		{
			name: "custom IP address",
			content: `
server:
  address: "192.168.1.100"
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "192.168.1.100",
		},
		{
			name: "IPv6 localhost address",
			content: `
server:
  address: "::1"
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "::1",
		},
		{
			name: "IPv6 all interfaces address",
			content: `
server:
  address: "::"
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "::",
		},
		{
			name: "IPv6 specific address",
			content: `
server:
  address: "2001:db8::1"
  port: 8080
endpoints:
  - path: /test
    remote_url: http://example.com
`,
			wantErr:  false,
			expected: "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "config-address-test-*.yaml")
			require.NoError(t, err)
			defer os.Remove(tmpFile.Name())

			_, err = tmpFile.WriteString(tt.content)
			require.NoError(t, err)
			tmpFile.Close()

			config, err := LoadConfig(tmpFile.Name())

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
				assert.Equal(t, tt.expected, config.Server.Address)
				assert.NotZero(t, config.Server.Port)
			}
		})
	}
}

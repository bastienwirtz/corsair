package config

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/goccy/go-yaml"
)

const (
	DEFAULT_TIMEOUT = 10 * time.Second
	DEFAULT_PATH    = "/etc/corsair/config.yaml"
)

type Config struct {
	Server    ServerConfig  `yaml:"server"`
	CORS      CORSConfig    `yaml:"cors"`
	Logging   LoggingConfig `yaml:"logging"`
	Endpoints []Endpoint    `yaml:"endpoints"`
}

type ServerConfig struct {
	Address                string `yaml:"address"`
	Port                   int    `yaml:"port"`
	ForwardEndpointEnabled *bool  `yaml:"forward_endpoint_enabled"`
	DefaultTimeout         string `yaml:"default_timeout"`
}

type CORSConfig struct {
	Origins     []string `yaml:"allow_origins"`
	Methods     string   `yaml:"allow_methods"`
	Headers     string   `yaml:"allow_headers"`
	Credentials bool     `yaml:"allow_credentials"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type Endpoint struct {
	Path        string              `yaml:"path"`
	RemoteURL   string              `yaml:"remote_url"`
	Headers     []map[string]string `yaml:"headers"`
	QueryParams []map[string]string `yaml:"query_params"`
	Timeout     string              `yaml:"timeout"`
}

func LoadConfig(filename string) (*Config, error) {
	var config Config
	var data []byte

	data, err := os.ReadFile(filename)
	if err != nil {
		// Allow starting without configuration file.
		if filename == DEFAULT_PATH && os.IsNotExist(err) {
			slog.Info("No configuration file found, using defaults")
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	setDefaults(&config)
	return &config, nil
}

func validateConfig(config *Config) error {
	// Validate CORS configuration
	if err := validateCORSConfig(&config.CORS); err != nil {
		return fmt.Errorf("CORS configuration invalid: %w", err)
	}

	// Validate timeout configuration
	if err := validateTimeoutConfig(config); err != nil {
		return fmt.Errorf("timeout configuration invalid: %w", err)
	}

	// Validate endpoints
	for i, endpoint := range config.Endpoints {
		if endpoint.Path == "" {
			return fmt.Errorf("endpoint %d: path cannot be empty", i)
		}
		if endpoint.RemoteURL == "" {
			return fmt.Errorf("endpoint %d: remote_url cannot be empty", i)
		}
	}
	return nil
}

func (c *CORSConfig) WildcardOriginAllowed() bool {
	return slices.Contains(c.Origins, "*")
}

func validateCORSConfig(corsConfig *CORSConfig) error {
	hasWildcard := corsConfig.WildcardOriginAllowed()

	// Check for CORS spec violation: wildcard origins with credentials
	if corsConfig.Credentials && hasWildcard {
		return fmt.Errorf("allow_origins cannot contain '*' when allow_credentials is true - this violates CORS specification and browsers will reject requests")
	}

	// Check for invalid wildcard usage: '*' must be the only origin when used
	if hasWildcard && len(corsConfig.Origins) > 1 {
		return fmt.Errorf("allow_origins: '*' wildcard must be the only origin when used, cannot be mixed with specific origins")
	}

	return nil
}

func validateTimeoutConfig(config *Config) error {
	// Validate server default timeout
	if config.Server.DefaultTimeout != "" {
		if _, err := time.ParseDuration(config.Server.DefaultTimeout); err != nil {
			return fmt.Errorf("invalid server default_timeout '%s': %w (use format like '10s', '1m30s', '2m')", config.Server.DefaultTimeout, err)
		}
	}

	// Validate endpoint timeouts
	for i, endpoint := range config.Endpoints {
		if endpoint.Timeout != "" {
			if _, err := time.ParseDuration(endpoint.Timeout); err != nil {
				return fmt.Errorf("invalid timeout '%s' for endpoint %d: %w (use format like '10s', '1m30s', '2m')", endpoint.Timeout, i, err)
			}
		}
	}

	return nil
}

func (c *Config) GetDefaultTimeout() time.Duration {
	timeout, err := time.ParseDuration(c.Server.DefaultTimeout)
	if err != nil {
		slog.Warn("Invalid server timeout", "configured_timeout", c.Server.DefaultTimeout, "error", err)
		timeout = DEFAULT_TIMEOUT
	}
	return timeout
}

// GetEffectiveTimeout returns the timeout duration for an endpoint, using endpoint-specific timeout if set, otherwise the global default
func (c *Config) GetEffectiveTimeout(endpoint Endpoint) time.Duration {
	timeoutStr := endpoint.Timeout
	if timeoutStr == "" {
		return c.GetDefaultTimeout()
	}

	// Parse duration, fallback to 10s if parsing fails (should not happen due to validation)
	duration, err := time.ParseDuration(timeoutStr)
	if err != nil {
		slog.Error("Fail to parse configured timeout value", "timeout", timeoutStr)
		return DEFAULT_TIMEOUT
	}

	return duration
}

func setDefaults(config *Config) {
	if config.Server.Address == "" {
		config.Server.Address = "localhost"
	}
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.DefaultTimeout == "" {
		config.Server.DefaultTimeout = DEFAULT_TIMEOUT.String()
	}
	if config.Server.ForwardEndpointEnabled == nil {
		enabled := true
		config.Server.ForwardEndpointEnabled = &enabled
	}

	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}
}

package config

import (
	"fmt"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Endpoints   []EndpointConfig  `yaml:"endpoints"`
	Logging     LoggingConfig     `yaml:"logging"`
	Validation  ValidationConfig  `yaml:"validation"`
	WebAdmin    WebAdminConfig    `yaml:"web_admin"`
}

type ServerConfig struct {
	Port      int    `yaml:"port"`
	AuthToken string `yaml:"auth_token"`
}

type EndpointConfig struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	PathPrefix     string `yaml:"path_prefix"`
	AuthType       string `yaml:"auth_type"`
	AuthValue      string `yaml:"auth_value"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	Enabled        bool   `yaml:"enabled"`
	Priority       int    `yaml:"priority"`
}

type LoggingConfig struct {
	Level              string `yaml:"level"`
	LogFailedRequests  bool   `yaml:"log_failed_requests"`
	LogRequestBody     bool   `yaml:"log_request_body"`
	LogResponseBody    bool   `yaml:"log_response_body"`
	PersistToDisk      bool   `yaml:"persist_to_disk"`
	LogDirectory       string `yaml:"log_directory"`
}

type ValidationConfig struct {
	StrictAnthropicFormat bool `yaml:"strict_anthropic_format"`
	ValidateStreaming     bool `yaml:"validate_streaming"`
	DisconnectOnInvalid   bool `yaml:"disconnect_on_invalid"`
}

type WebAdminConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
}

func (e *EndpointConfig) GetAuthHeader() string {
	switch e.AuthType {
	case "api_key":
		return "Bearer " + e.AuthValue
	case "auth_token":
		return e.AuthValue
	default:
		return e.AuthValue
	}
}

func (e *EndpointConfig) GetTimeout() time.Duration {
	return time.Duration(e.TimeoutSeconds) * time.Second
}

func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	return &config, nil
}

func validateConfig(config *Config) error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.Server.AuthToken == "" {
		return fmt.Errorf("server auth_token cannot be empty")
	}

	if len(config.Endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be configured")
	}

	for i, endpoint := range config.Endpoints {
		if endpoint.Name == "" {
			return fmt.Errorf("endpoint %d: name cannot be empty", i)
		}
		if endpoint.URL == "" {
			return fmt.Errorf("endpoint %d: url cannot be empty", i)
		}
		if endpoint.AuthType != "api_key" && endpoint.AuthType != "auth_token" {
			return fmt.Errorf("endpoint %d: invalid auth_type '%s', must be 'api_key' or 'auth_token'", i, endpoint.AuthType)
		}
		if endpoint.AuthValue == "" {
			return fmt.Errorf("endpoint %d: auth_value cannot be empty", i)
		}
		if endpoint.TimeoutSeconds <= 0 {
			return fmt.Errorf("endpoint %d: timeout_seconds must be greater than 0", i)
		}
	}

	if config.WebAdmin.Enabled {
		if config.WebAdmin.Port <= 0 || config.WebAdmin.Port > 65535 {
			return fmt.Errorf("invalid web admin port: %d", config.WebAdmin.Port)
		}
		if config.WebAdmin.Host == "" {
			config.WebAdmin.Host = "127.0.0.1"
		}
	}

	if config.Logging.LogDirectory == "" {
		config.Logging.LogDirectory = "./logs"
	}

	return nil
}
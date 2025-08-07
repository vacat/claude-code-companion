package config

import (
	"fmt"
	"io/ioutil"
	"os"

	"claude-proxy/internal/utils"

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
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	AuthToken string `yaml:"auth_token"`
}

type EndpointConfig struct {
	Name       string `yaml:"name"`
	URL        string `yaml:"url"`
	PathPrefix string `yaml:"path_prefix"`
	AuthType   string `yaml:"auth_type"`
	AuthValue  string `yaml:"auth_value"`
	Enabled    bool   `yaml:"enabled"`
	Priority   int    `yaml:"priority"`
}

// 实现 EndpointConfig 接口，用于统一验证
func (e EndpointConfig) GetName() string     { return e.Name }
func (e EndpointConfig) GetURL() string      { return e.URL }
func (e EndpointConfig) GetAuthType() string { return e.AuthType }
func (e EndpointConfig) GetAuthValue() string { return e.AuthValue }

type LoggingConfig struct {
	Level           string `yaml:"level"`
	LogRequestTypes string `yaml:"log_request_types"`
	LogRequestBody  string `yaml:"log_request_body"`
	LogResponseBody string `yaml:"log_response_body"`
	LogDirectory    string `yaml:"log_directory"`
}

type ValidationConfig struct {
	StrictAnthropicFormat bool `yaml:"strict_anthropic_format"`
	ValidateStreaming     bool `yaml:"validate_streaming"`
	DisconnectOnInvalid   bool `yaml:"disconnect_on_invalid"`
}

type WebAdminConfig struct {
	Enabled bool `yaml:"enabled"`
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
	// 设置服务器主机默认值
	if config.Server.Host == "" {
		config.Server.Host = "127.0.0.1"
	}

	// 使用统一的服务器配置验证
	if err := utils.ValidateServerConfig(config.Server.Host, config.Server.Port, config.Server.AuthToken); err != nil {
		return err
	}

	// 转换为接口类型进行统一验证
	validator := utils.NewEndpointConfigValidator()
	endpointInterfaces := make([]utils.EndpointConfig, len(config.Endpoints))
	for i, ep := range config.Endpoints {
		endpointInterfaces[i] = ep
	}

	if err := validator.ValidateEndpoints(endpointInterfaces); err != nil {
		return err
	}

	// WebAdmin 现在合并到主服务器端口，无需单独验证

	if config.Logging.LogDirectory == "" {
		config.Logging.LogDirectory = "./logs"
	}

	// Validate log_request_types
	if config.Logging.LogRequestTypes == "" {
		config.Logging.LogRequestTypes = "all"
	}
	validRequestTypes := []string{"failed", "success", "all"}
	validRequestType := false
	for _, vt := range validRequestTypes {
		if config.Logging.LogRequestTypes == vt {
			validRequestType = true
			break
		}
	}
	if !validRequestType {
		return fmt.Errorf("invalid log_request_types '%s', must be one of: failed, success, all", config.Logging.LogRequestTypes)
	}

	// Validate log_request_body
	if config.Logging.LogRequestBody == "" {
		config.Logging.LogRequestBody = "full"
	}
	validBodyTypes := []string{"none", "truncated", "full"}
	validRequestBodyType := false
	for _, vt := range validBodyTypes {
		if config.Logging.LogRequestBody == vt {
			validRequestBodyType = true
			break
		}
	}
	if !validRequestBodyType {
		return fmt.Errorf("invalid log_request_body '%s', must be one of: none, truncated, full", config.Logging.LogRequestBody)
	}

	// Validate log_response_body
	if config.Logging.LogResponseBody == "" {
		config.Logging.LogResponseBody = "full"
	}
	validResponseBodyType := false
	for _, vt := range validBodyTypes {
		if config.Logging.LogResponseBody == vt {
			validResponseBodyType = true
			break
		}
	}
	if !validResponseBodyType {
		return fmt.Errorf("invalid log_response_body '%s', must be one of: none, truncated, full", config.Logging.LogResponseBody)
	}

	return nil
}

func SaveConfig(config *Config, filename string) error {
	// 首先验证配置
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	// 序列化为YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// 创建备份文件
	if _, err := os.Stat(filename); err == nil {
		backupFilename := filename + ".backup"
		if err := os.Rename(filename, backupFilename); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
	}

	// 写入新配置
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}
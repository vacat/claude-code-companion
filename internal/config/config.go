package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"claude-proxy/internal/utils"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Endpoints   []EndpointConfig  `yaml:"endpoints"`
	Logging     LoggingConfig     `yaml:"logging"`
	Validation  ValidationConfig  `yaml:"validation"`
	WebAdmin    WebAdminConfig    `yaml:"web_admin"`
	Tagging     TaggingConfig     `yaml:"tagging"`     // 新增：Tag系统配置
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type EndpointConfig struct {
	Name       string   `yaml:"name"`
	URL        string   `yaml:"url"`
	PathPrefix string   `yaml:"path_prefix"`
	AuthType   string   `yaml:"auth_type"`
	AuthValue  string   `yaml:"auth_value"`
	Enabled    bool     `yaml:"enabled"`
	Priority   int      `yaml:"priority"`
	Tags       []string `yaml:"tags"`       // 新增：支持的tag列表
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

// 新增：Tag系统配置结构
type TaggingConfig struct {
	Enabled         bool            `yaml:"enabled"`
	PipelineTimeout string          `yaml:"pipeline_timeout"`
	Taggers         []TaggerConfig  `yaml:"taggers"`
}

type TaggerConfig struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`         // "builtin" | "starlark"
	BuiltinType string                 `yaml:"builtin_type"` // 内置类型: "path" | "header" | "body-json" | "method" | "query"
	Tag         string                 `yaml:"tag"`          // 标记的tag名称
	Enabled     bool                   `yaml:"enabled"`
	Priority    int                    `yaml:"priority"`     // 执行优先级(未使用，因为并发执行)
	Config      map[string]interface{} `yaml:"config"`       // tagger特定配置
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
	if err := utils.ValidateServerConfig(config.Server.Host, config.Server.Port, ""); err != nil {
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

	// 验证Tagging配置
	if err := validateTaggingConfig(&config.Tagging); err != nil {
		return fmt.Errorf("tagging configuration error: %v", err)
	}

	return nil
}

func validateTaggingConfig(config *TaggingConfig) error {
	// 设置默认值
	if config.PipelineTimeout == "" {
		config.PipelineTimeout = "5s"
	}
	
	// 验证超时时间格式
	if _, err := time.ParseDuration(config.PipelineTimeout); err != nil {
		return fmt.Errorf("invalid pipeline_timeout '%s': %v", config.PipelineTimeout, err)
	}

	// 验证tagger配置
	tagNames := make(map[string]bool)
	for i, tagger := range config.Taggers {
		if tagger.Name == "" {
			return fmt.Errorf("tagger[%d]: name is required", i)
		}
		
		if tagNames[tagger.Name] {
			return fmt.Errorf("tagger[%d]: duplicate name '%s'", i, tagger.Name)
		}
		tagNames[tagger.Name] = true
		
		if tagger.Tag == "" {
			return fmt.Errorf("tagger[%d] '%s': tag is required", i, tagger.Name)
		}
		
		if tagger.Type != "builtin" && tagger.Type != "starlark" {
			return fmt.Errorf("tagger[%d] '%s': type must be 'builtin' or 'starlark'", i, tagger.Name)
		}
		
		// 验证内置tagger类型
		if tagger.Type == "builtin" {
			validBuiltinTypes := []string{"path", "header", "body-json", "method", "query"}
			validType := false
			for _, vt := range validBuiltinTypes {
				if tagger.BuiltinType == vt {
					validType = true
					break
				}
			}
			if !validType {
				return fmt.Errorf("tagger[%d] '%s': invalid builtin_type '%s', must be one of: %v", 
					i, tagger.Name, tagger.BuiltinType, validBuiltinTypes)
			}
		}
		
		// 验证starlark脚本配置
		if tagger.Type == "starlark" {
			// 支持两种方式：script_file 或 script
			scriptFile, hasScriptFile := tagger.Config["script_file"].(string)
			script, hasScript := tagger.Config["script"].(string)
			
			if hasScriptFile && scriptFile != "" {
				// 使用脚本文件 - 可以在这里添加脚本文件存在性检查
			} else if hasScript && script != "" {
				// 使用内联脚本 - 验证脚本内容非空
			} else {
				return fmt.Errorf("tagger[%d] '%s': starlark tagger requires either script_file or script in config", i, tagger.Name)
			}
		}
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
package config

import (
	"fmt"
	"os"
	"path/filepath"
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
	Timeouts    TimeoutConfig     `yaml:"timeouts"`    // 新增：超时配置
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type EndpointConfig struct {
	Name         string              `yaml:"name"`
	URL          string              `yaml:"url"`
	PathPrefix   string              `yaml:"path_prefix"`
	AuthType     string              `yaml:"auth_type"`
	AuthValue    string              `yaml:"auth_value"`
	Enabled      bool                `yaml:"enabled"`
	Priority     int                 `yaml:"priority"`
	Tags         []string            `yaml:"tags"`         // 新增：支持的tag列表
	ModelRewrite *ModelRewriteConfig `yaml:"model_rewrite,omitempty"` // 新增：模型重写配置
}

// 新增：模型重写配置结构
type ModelRewriteConfig struct {
	Enabled bool               `yaml:"enabled" json:"enabled"` // 是否启用模型重写
	Rules   []ModelRewriteRule `yaml:"rules" json:"rules"`     // 重写规则列表
}

// 新增：模型重写规则
type ModelRewriteRule struct {
	SourcePattern string `yaml:"source_pattern" json:"source_pattern"` // 源模型通配符模式
	TargetModel   string `yaml:"target_model" json:"target_model"`     // 目标模型名称
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

// 新增：超时配置结构
type TimeoutConfig struct {
	// 代理客户端超时设置
	Proxy ProxyTimeoutConfig `yaml:"proxy"`
	// 健康检查超时设置
	HealthCheck HealthCheckTimeoutConfig `yaml:"health_check"`
}

// 代理客户端超时配置
type ProxyTimeoutConfig struct {
	TLSHandshake     string `yaml:"tls_handshake"`      // TLS握手超时，默认10s
	ResponseHeader   string `yaml:"response_header"`    // 响应头超时，默认60s  
	IdleConnection   string `yaml:"idle_connection"`    // 空闲连接超时，默认90s
	OverallRequest   string `yaml:"overall_request"`    // 整体请求超时，默认无限制(支持流式)
}

// 健康检查超时配置
type HealthCheckTimeoutConfig struct {
	TLSHandshake     string `yaml:"tls_handshake"`      // TLS握手超时，默认5s
	ResponseHeader   string `yaml:"response_header"`    // 响应头超时，默认30s
	IdleConnection   string `yaml:"idle_connection"`    // 空闲连接超时，默认60s
	OverallRequest   string `yaml:"overall_request"`    // 整体请求超时，默认30s
	CheckInterval    string `yaml:"check_interval"`     // 健康检查间隔，默认30s
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
	data, err := os.ReadFile(filename)
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
	if err := utils.ValidateServerConfig(config.Server.Host, config.Server.Port); err != nil {
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

	// 验证Timeout配置
	if err := validateTimeoutConfig(&config.Timeouts); err != nil {
		return fmt.Errorf("timeout configuration error: %v", err)
	}

	// 验证ModelRewrite配置
	if err := validateModelRewriteConfigs(config.Endpoints); err != nil {
		return fmt.Errorf("model rewrite configuration error: %v", err)
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
			validBuiltinTypes := []string{"path", "header", "body-json", "method", "query", "user-message"}
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

func validateTimeoutConfig(config *TimeoutConfig) error {
	// 设置代理超时默认值
	if config.Proxy.TLSHandshake == "" {
		config.Proxy.TLSHandshake = "10s"
	}
	if config.Proxy.ResponseHeader == "" {
		config.Proxy.ResponseHeader = "60s"
	}
	if config.Proxy.IdleConnection == "" {
		config.Proxy.IdleConnection = "90s"
	}
	// OverallRequest 默认为空，表示无限制（支持流式响应）
	
	// 设置健康检查超时默认值
	if config.HealthCheck.TLSHandshake == "" {
		config.HealthCheck.TLSHandshake = "5s"
	}
	if config.HealthCheck.ResponseHeader == "" {
		config.HealthCheck.ResponseHeader = "30s"
	}
	if config.HealthCheck.IdleConnection == "" {
		config.HealthCheck.IdleConnection = "60s"
	}
	if config.HealthCheck.OverallRequest == "" {
		config.HealthCheck.OverallRequest = "30s"
	}
	if config.HealthCheck.CheckInterval == "" {
		config.HealthCheck.CheckInterval = "30s"
	}

	// 验证所有非空超时时间格式
	timeoutFields := map[string]string{
		"proxy.tls_handshake":          config.Proxy.TLSHandshake,
		"proxy.response_header":        config.Proxy.ResponseHeader,
		"proxy.idle_connection":        config.Proxy.IdleConnection,
		"health_check.tls_handshake":   config.HealthCheck.TLSHandshake,
		"health_check.response_header": config.HealthCheck.ResponseHeader,
		"health_check.idle_connection": config.HealthCheck.IdleConnection,
		"health_check.overall_request": config.HealthCheck.OverallRequest,
		"health_check.check_interval":  config.HealthCheck.CheckInterval,
	}

	// 如果配置了proxy overall_request，也验证它
	if config.Proxy.OverallRequest != "" {
		timeoutFields["proxy.overall_request"] = config.Proxy.OverallRequest
	}

	for fieldName, value := range timeoutFields {
		if value != "" {
			if _, err := time.ParseDuration(value); err != nil {
				return fmt.Errorf("invalid timeout '%s' for field '%s': %v", value, fieldName, err)
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
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// validateModelRewriteConfigs 验证端点的模型重写配置
func validateModelRewriteConfigs(endpoints []EndpointConfig) error {
	for i, endpoint := range endpoints {
		if endpoint.ModelRewrite == nil {
			continue // 没有配置模型重写，跳过验证
		}
		
		if err := validateModelRewriteConfig(endpoint.ModelRewrite, fmt.Sprintf("endpoint[%d] '%s'", i, endpoint.Name)); err != nil {
			return err
		}
	}
	return nil
}

// ValidateModelRewriteConfig 验证单个模型重写配置（导出函数）
func ValidateModelRewriteConfig(config *ModelRewriteConfig, context string) error {
	return validateModelRewriteConfig(config, context)
}

// validateModelRewriteConfig 验证单个模型重写配置
func validateModelRewriteConfig(config *ModelRewriteConfig, context string) error {
	if !config.Enabled {
		return nil // 未启用，跳过规则验证
	}
	
	if len(config.Rules) == 0 {
		return fmt.Errorf("%s: model_rewrite is enabled but no rules configured", context)
	}
	
	// 验证每个规则
	seenPatterns := make(map[string]bool)
	for i, rule := range config.Rules {
		if rule.SourcePattern == "" {
			return fmt.Errorf("%s: rule[%d] source_pattern is required", context, i)
		}
		
		if rule.TargetModel == "" {
			return fmt.Errorf("%s: rule[%d] target_model is required", context, i)
		}
		
		// 检查重复的源模式
		if seenPatterns[rule.SourcePattern] {
			return fmt.Errorf("%s: rule[%d] duplicate source_pattern '%s'", context, i, rule.SourcePattern)
		}
		seenPatterns[rule.SourcePattern] = true
		
		// 验证通配符模式语法（尝试用一个测试字符串匹配）
		if _, err := filepath.Match(rule.SourcePattern, "test-model"); err != nil {
			return fmt.Errorf("%s: rule[%d] invalid source_pattern '%s': %v", context, i, rule.SourcePattern, err)
		}
	}
	
	return nil
}
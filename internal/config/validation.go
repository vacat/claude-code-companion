package config

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"
)

// ValidateConfig 导出的配置验证函数
func ValidateConfig(config *Config) error {
	return validateConfig(config)
}

func validateConfig(config *Config) error {
	// 设置服务器主机默认值
	if config.Server.Host == "" {
		config.Server.Host = "127.0.0.1"
	}

	// 验证服务器配置
	if err := validateServerConfig(config.Server.Host, config.Server.Port); err != nil {
		return err
	}

	// 验证端点配置
	if err := validateEndpoints(config.Endpoints); err != nil {
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

	// 验证OpenAI端点配置
	if err := validateOpenAIEndpoints(config.Endpoints); err != nil {
		return fmt.Errorf("openai endpoint configuration error: %v", err)
	}

	// 验证代理配置
	if err := validateProxyConfigs(config.Endpoints); err != nil {
		return fmt.Errorf("proxy configuration error: %v", err)
	}

	// 验证OAuth配置
	if err := validateOAuthConfigs(config.Endpoints); err != nil {
		return fmt.Errorf("oauth configuration error: %v", err)
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
			validBuiltinTypes := []string{"path", "header", "body-json", "query", "user-message", "model", "thinking"}
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

// validateOAuthConfigs 验证端点的OAuth配置
func validateOAuthConfigs(endpoints []EndpointConfig) error {
	for i, endpoint := range endpoints {
		if endpoint.AuthType == "oauth" {
			if endpoint.OAuthConfig == nil {
				return fmt.Errorf("endpoint[%d] '%s': oauth_config is required when auth_type is 'oauth'", i, endpoint.Name)
			}
			
			if err := validateOAuthConfig(endpoint.OAuthConfig, fmt.Sprintf("endpoint[%d] '%s'", i, endpoint.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateOAuthConfig 验证单个OAuth配置（导出函数）
func ValidateOAuthConfig(config *OAuthConfig, context string) error {
	return validateOAuthConfig(config, context)
}

// validateOAuthConfig 验证单个OAuth配置
func validateOAuthConfig(config *OAuthConfig, context string) error {
	if config.AccessToken == "" {
		return fmt.Errorf("%s: oauth access_token is required", context)
	}
	
	if config.RefreshToken == "" {
		return fmt.Errorf("%s: oauth refresh_token is required", context)
	}
	
	if config.ExpiresAt <= 0 {
		return fmt.Errorf("%s: oauth expires_at must be a valid timestamp (milliseconds)", context)
	}
	
	if config.TokenURL == "" {
		return fmt.Errorf("%s: oauth token_url is required", context)
	}
	
	if config.ClientID == "" {
		config.ClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	}
	
	// 验证access token格式（如果是 Anthropic token）
	if strings.HasPrefix(config.AccessToken, "sk-ant-") && !strings.HasPrefix(config.AccessToken, "sk-ant-oat01-") {
		return fmt.Errorf("%s: Anthropic oauth access_token should start with 'sk-ant-oat01-'", context)
	}
	
	// 验证refresh token格式（如果是 Anthropic token）
	if strings.HasPrefix(config.RefreshToken, "sk-ant-") && !strings.HasPrefix(config.RefreshToken, "sk-ant-ort01-") {
		return fmt.Errorf("%s: Anthropic oauth refresh_token should start with 'sk-ant-ort01-'", context)
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

// validateOpenAIEndpoints 验证 OpenAI 端点配置
func validateOpenAIEndpoints(endpoints []EndpointConfig) error {
	for i, endpoint := range endpoints {
		if endpoint.EndpointType == "openai" {
			// OpenAI 端点不能使用 api_key 认证类型
			if endpoint.AuthType == "api_key" {
				return fmt.Errorf("endpoint[%d] '%s': OpenAI endpoints cannot use auth_type 'api_key', use 'auth_token' instead", i, endpoint.Name)
			}
			
			// 确保 OpenAI 端点有正确的认证配置
			if endpoint.AuthType == "" {
				return fmt.Errorf("endpoint[%d] '%s': OpenAI endpoints require auth_type to be specified", i, endpoint.Name)
			}
			
			if endpoint.AuthType != "auth_token" && endpoint.AuthType != "oauth" {
				return fmt.Errorf("endpoint[%d] '%s': OpenAI endpoints should use auth_type 'auth_token' or 'oauth'", i, endpoint.Name)
			}
			
			// 验证认证配置
			if endpoint.AuthType == "oauth" {
				if endpoint.OAuthConfig == nil {
					return fmt.Errorf("endpoint[%d] '%s': OpenAI endpoints with oauth auth_type require oauth_config", i, endpoint.Name)
				}
			} else if endpoint.AuthValue == "" {
				return fmt.Errorf("endpoint[%d] '%s': OpenAI endpoints with auth_token require auth_value to be specified", i, endpoint.Name)
			}
			
			// OpenAI 端点必须配置 path_prefix
			if endpoint.PathPrefix == "" {
				return fmt.Errorf("endpoint[%d] '%s': OpenAI endpoints require path_prefix to be specified (e.g., '/v1/chat/completions')", i, endpoint.Name)
			}
		}
		
		// Anthropic 端点不应该配置 path_prefix，因为会被固定为 /v1/messages
		if endpoint.EndpointType == "anthropic" || endpoint.EndpointType == "" {
			if endpoint.PathPrefix != "" {
				return fmt.Errorf("endpoint[%d] '%s': Anthropic endpoints cannot have custom path_prefix (automatically set to '/v1/messages')", i, endpoint.Name)
			}
		}
	}
	return nil
}

// validateProxyConfigs 验证端点的代理配置
func validateProxyConfigs(endpoints []EndpointConfig) error {
	for i, endpoint := range endpoints {
		if endpoint.Proxy == nil {
			continue // 没有配置代理，跳过验证
		}
		
		if err := validateProxyConfig(endpoint.Proxy, fmt.Sprintf("endpoint[%d] '%s'", i, endpoint.Name)); err != nil {
			return err
		}
	}
	return nil
}

// ValidateProxyConfig 验证单个代理配置（导出函数）
func ValidateProxyConfig(config *ProxyConfig, context string) error {
	return validateProxyConfig(config, context)
}

// validateProxyConfig 验证单个代理配置
func validateProxyConfig(config *ProxyConfig, context string) error {
	if config.Type == "" {
		return fmt.Errorf("%s: proxy type is required", context)
	}
	
	// 验证代理类型
	validTypes := []string{"http", "socks5"}
	validType := false
	for _, vt := range validTypes {
		if config.Type == vt {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("%s: invalid proxy type '%s', must be one of: %v", context, config.Type, validTypes)
	}
	
	if config.Address == "" {
		return fmt.Errorf("%s: proxy address is required", context)
	}
	
	// 验证地址格式（简单检查是否包含端口）
	if _, _, err := net.SplitHostPort(config.Address); err != nil {
		return fmt.Errorf("%s: invalid proxy address '%s': %v", context, config.Address, err)
	}
	
	// 验证认证配置一致性
	if (config.Username != "" && config.Password == "") || (config.Username == "" && config.Password != "") {
		return fmt.Errorf("%s: proxy username and password must both be provided or both be empty", context)
	}
	
	return nil
}

// validateServerConfig validates server configuration
func validateServerConfig(host string, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid server port: %d", port)
	}
	
	return nil
}

// validateEndpoints validates endpoint configurations
func validateEndpoints(endpoints []EndpointConfig) error {
	if len(endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be configured")
	}
	
	for i, endpoint := range endpoints {
		if err := validateEndpoint(endpoint, i); err != nil {
			return err
		}
	}
	
	return nil
}

// validateEndpoint validates a single endpoint configuration
func validateEndpoint(endpoint EndpointConfig, index int) error {
	if endpoint.Name == "" {
		return fmt.Errorf("endpoint %d: name cannot be empty", index)
	}
	
	if endpoint.URL == "" {
		return fmt.Errorf("endpoint %d: url cannot be empty", index)
	}
	
	if endpoint.AuthType != "api_key" && endpoint.AuthType != "auth_token" && endpoint.AuthType != "oauth" {
		return fmt.Errorf("endpoint %d: invalid auth_type '%s', must be 'api_key', 'auth_token', or 'oauth'", index, endpoint.AuthType)
	}
	
	// OAuth 认证不需要 auth_value，其他认证类型需要
	if endpoint.AuthType != "oauth" && endpoint.AuthValue == "" {
		return fmt.Errorf("endpoint %d: auth_value cannot be empty for non-oauth authentication", index)
	}
	
	return nil
}
package config

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Endpoints   []EndpointConfig  `yaml:"endpoints"`
	Logging     LoggingConfig     `yaml:"logging"`
	Validation  ValidationConfig  `yaml:"validation"`
	Tagging     TaggingConfig     `yaml:"tagging"`     // 标签系统配置（永远启用）
	Timeouts    TimeoutConfig     `yaml:"timeouts"`    // 超时配置
	I18n        I18nConfig        `yaml:"i18n"`        // 国际化配置
}

// I18nConfig 国际化配置
type I18nConfig struct {
	Enabled         bool   `yaml:"enabled"`          // 是否启用国际化
	DefaultLanguage string `yaml:"default_language"` // 默认语言
	LocalesPath     string `yaml:"locales_path"`     // 语言文件路径
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type EndpointConfig struct {
	Name              string              `yaml:"name"`
	URL               string              `yaml:"url"`
	EndpointType      string              `yaml:"endpoint_type"` // "anthropic" | "openai" 等
	PathPrefix        string              `yaml:"path_prefix,omitempty"` // OpenAI端点的路径前缀，如 "/v1/chat/completions"
	AuthType          string              `yaml:"auth_type"`
	AuthValue         string              `yaml:"auth_value"`
	Enabled           bool                `yaml:"enabled"`
	Priority          int                 `yaml:"priority"`
	Tags              []string            `yaml:"tags"`         // 新增：支持的tag列表
	ModelRewrite      *ModelRewriteConfig `yaml:"model_rewrite,omitempty"` // 新增：模型重写配置
	Proxy             *ProxyConfig        `yaml:"proxy,omitempty"`         // 新增：代理配置
	OAuthConfig       *OAuthConfig        `yaml:"oauth_config,omitempty"`  // 新增：OAuth配置
	OverrideMaxTokens *int                `yaml:"override_max_tokens,omitempty"` // 新增：覆盖max_tokens配置
}

// 新增：代理配置结构
type ProxyConfig struct {
	Type     string `yaml:"type" json:"type"`         // "http" | "socks5"
	Address  string `yaml:"address" json:"address"`   // 代理服务器地址，如 "127.0.0.1:1080"
	Username string `yaml:"username,omitempty" json:"username,omitempty"` // 代理认证用户名（可选）
	Password string `yaml:"password,omitempty" json:"password,omitempty"` // 代理认证密码（可选）
}

// 新增：OAuth 配置结构
type OAuthConfig struct {
	AccessToken  string   `yaml:"access_token" json:"access_token"`     // 访问令牌
	RefreshToken string   `yaml:"refresh_token" json:"refresh_token"`   // 刷新令牌  
	ExpiresAt    int64    `yaml:"expires_at" json:"expires_at"`         // 过期时间戳（毫秒）
	TokenURL     string   `yaml:"token_url" json:"token_url"`           // Token刷新URL（必填）
	ClientID     string   `yaml:"client_id,omitempty" json:"client_id,omitempty"`       // 客户端ID
	Scopes       []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`             // 权限范围
	AutoRefresh  bool     `yaml:"auto_refresh" json:"auto_refresh"`                     // 是否自动刷新
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

type LoggingConfig struct {
	Level           string `yaml:"level"`
	LogRequestTypes string `yaml:"log_request_types"`
	LogRequestBody  string `yaml:"log_request_body"`
	LogResponseBody string `yaml:"log_response_body"`
	LogDirectory    string `yaml:"log_directory"`
}

type ValidationConfig struct {
	StrictAnthropicFormat bool                    `yaml:"strict_anthropic_format"`
	ValidateStreaming     bool                    `yaml:"validate_streaming"`
	PythonJSONFixing      PythonJSONFixingConfig  `yaml:"python_json_fixing"`
}

// PythonJSONFixing 配置结构
type PythonJSONFixingConfig struct {
	Enabled       bool     `yaml:"enabled" json:"enabled"`               // 是否启用 Python JSON 修复
	TargetTools   []string `yaml:"target_tools" json:"target_tools"`     // 需要修复的工具列表
	DebugLogging  bool     `yaml:"debug_logging" json:"debug_logging"`   // 是否启用调试日志
	MaxAttempts   int      `yaml:"max_attempts" json:"max_attempts"`     // 最大修复尝试次数
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

// Tag系统配置结构 (永远启用)
type TaggingConfig struct {
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
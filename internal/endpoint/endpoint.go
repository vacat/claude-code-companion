package endpoint

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"claude-code-companion/internal/common/httpclient"
	"claude-code-companion/internal/config"
	"claude-code-companion/internal/interfaces"
	"claude-code-companion/internal/oauth"
	"claude-code-companion/internal/utils"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusChecking Status = "checking"
)

// 删除不再需要的 RequestRecord 定义，因为已经移到 utils 包

type Endpoint struct {
	ID                string                   `json:"id"`
	Name              string                   `json:"name"`
	URL               string                   `json:"url"`
	EndpointType      string                   `json:"endpoint_type"` // "anthropic" | "openai" 等
	PathPrefix        string                   `json:"path_prefix,omitempty"` // OpenAI端点的路径前缀
	AuthType          string                   `json:"auth_type"`
	AuthValue         string                   `json:"auth_value"`
	Enabled           bool                     `json:"enabled"`
	Priority          int                      `json:"priority"`
	Tags              []string                 `json:"tags"`           // 新增：支持的tag列表
	DefaultModel      string                   `json:"default_model,omitempty"` // 新增：默认模型配置
	ModelRewrite      *config.ModelRewriteConfig `json:"model_rewrite,omitempty"` // 新增：模型重写配置
	Proxy             *config.ProxyConfig      `json:"proxy,omitempty"` // 新增：代理配置
	OAuthConfig       *config.OAuthConfig      `json:"oauth_config,omitempty"` // 新增：OAuth配置
	OverrideMaxTokens *int                     `json:"override_max_tokens,omitempty"` // 新增：覆盖max_tokens配置
	Status            Status                   `json:"status"`
	LastCheck         time.Time                `json:"last_check"`
	FailureCount      int                      `json:"failure_count"`
	TotalRequests     int                      `json:"total_requests"`
	SuccessRequests   int                      `json:"success_requests"`
	LastFailure       time.Time                `json:"last_failure"`
	RequestHistory    *utils.CircularBuffer    `json:"-"` // 使用环形缓冲区，不导出到JSON
	mutex             sync.RWMutex
}

func NewEndpoint(config config.EndpointConfig) *Endpoint {
	// 如果没有指定 endpoint_type，默认为 anthropic （向后兼容）
	endpointType := config.EndpointType
	if endpointType == "" {
		endpointType = "anthropic"
	}
	
	return &Endpoint{
		ID:                generateID(config.Name),
		Name:              config.Name,
		URL:               config.URL,
		EndpointType:      endpointType,
		PathPrefix:        config.PathPrefix,  // 新增：复制PathPrefix
		AuthType:          config.AuthType,
		AuthValue:         config.AuthValue,
		Enabled:           config.Enabled,
		Priority:          config.Priority,
		Tags:              config.Tags,       // 新增：从配置中复制tags
		DefaultModel:      config.DefaultModel, // 新增：从配置中复制默认模型配置
		ModelRewrite:      config.ModelRewrite, // 新增：从配置中复制模型重写配置
		Proxy:             config.Proxy,      // 新增：从配置中复制代理配置
		OAuthConfig:       config.OAuthConfig, // 新增：从配置中复制OAuth配置
		OverrideMaxTokens: config.OverrideMaxTokens, // 新增：从配置中复制max_tokens覆盖配置
		Status:            StatusActive,
		LastCheck:         time.Now(),
		RequestHistory:    utils.NewCircularBuffer(100, 140*time.Second), // 100个记录，140秒窗口
	}
}

// 实现 EndpointSorter 接口
func (e *Endpoint) GetPriority() int {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.Priority
}

func (e *Endpoint) IsEnabled() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.Enabled
}

func (e *Endpoint) GetAuthHeader() (string, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	switch e.AuthType {
	case "api_key":
		return e.AuthValue, nil // api_key 直接返回值，会用 x-api-key 头部
	case "auth_token":
		return "Bearer " + e.AuthValue, nil // auth_token 使用 Bearer 前缀
	case "oauth":
		if e.OAuthConfig == nil {
			return "", fmt.Errorf("oauth config is required for oauth auth_type")
		}
		
		// 检查 token 是否需要刷新
		if oauth.IsTokenExpired(e.OAuthConfig) {
			return "", fmt.Errorf("oauth token expired, refresh required")
		}
		
		return oauth.GetAuthorizationHeader(e.OAuthConfig), nil
	default:
		return e.AuthValue, nil
	}
}

func (e *Endpoint) GetTags() []string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	// 返回tags的副本以避免并发修改
	tags := make([]string, len(e.Tags))
	copy(tags, e.Tags)
	return tags
}

// ToTaggedEndpoint 将Endpoint转换为TaggedEndpoint
func (e *Endpoint) ToTaggedEndpoint() interfaces.TaggedEndpoint {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	tags := make([]string, len(e.Tags))
	copy(tags, e.Tags)
	
	return interfaces.TaggedEndpoint{
		Name:     e.Name,
		URL:      e.URL,
		Tags:     tags,
		Priority: e.Priority,
		Enabled:  e.Enabled,
	}
}

func (e *Endpoint) GetFullURL(path string) string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	// 直接使用端点的URL作为基础URL
	baseURL := e.URL
	
	// 根据端点类型自动添加正确的路径前缀
	switch e.EndpointType {
	case "anthropic":
		// Anthropic 端点需要添加 /v1 前缀，因为路由组已经消费了 /v1
		return baseURL + "/v1" + path
	case "openai":
		// OpenAI 端点使用配置的路径前缀（不需要路径转换）
		return baseURL + e.PathPrefix
	default:
		// 向后兼容：默认使用 anthropic 格式，需要添加 /v1 前缀
		return baseURL + "/v1" + path
	}
}

// 优化 IsAvailable 方法，减少锁的持有时间
func (e *Endpoint) IsAvailable() bool {
	e.mutex.RLock()
	enabled := e.Enabled
	status := e.Status
	e.mutex.RUnlock()
	
	return enabled && status == StatusActive
}

func (e *Endpoint) RecordRequest(success bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	now := time.Now()
	
	// 添加到环形缓冲区
	record := utils.RequestRecord{
		Timestamp: now,
		Success:   success,
	}
	e.RequestHistory.Add(record)
	
	e.TotalRequests++
	if success {
		e.SuccessRequests++
		e.FailureCount = 0 // 重置失败计数
		// 如果成功且之前是不可用状态，恢复为可用
		if e.Status == StatusInactive {
			e.Status = StatusActive
		}
	} else {
		e.FailureCount++
		e.LastFailure = now
		
		// 使用环形缓冲区检查是否应该标记为不可用
		if e.Status == StatusActive && e.RequestHistory.ShouldMarkInactive(now) {
			e.Status = StatusInactive
		}
	}
}

func (e *Endpoint) MarkInactive() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Status = StatusInactive
}

func (e *Endpoint) MarkActive() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.Status = StatusActive
	e.FailureCount = 0
	// 可选：清理历史记录
	e.RequestHistory.Clear()
}


func generateID(name string) string {
	return fmt.Sprintf("endpoint-%s-%d", name, time.Now().Unix())
}

// parseDuration 解析时间字符串，失败时返回默认值
func parseDuration(durationStr string, defaultDuration time.Duration) time.Duration {
	if durationStr == "" {
		return defaultDuration
	}
	if duration, err := time.ParseDuration(durationStr); err == nil {
		return duration
	}
	return defaultDuration
}

// CreateProxyClient 为这个端点创建支持代理的HTTP客户端
func (e *Endpoint) CreateProxyClient(timeoutConfig config.ProxyTimeoutConfig) (*http.Client, error) {
	e.mutex.RLock()
	proxyConfig := e.Proxy
	e.mutex.RUnlock()
	
	factory := httpclient.NewFactory()
	clientConfig := httpclient.ClientConfig{
		Type: httpclient.ClientTypeEndpoint,
		Timeouts: httpclient.TimeoutConfig{
			TLSHandshake:   parseDuration(timeoutConfig.TLSHandshake, 10*time.Second),
			ResponseHeader: parseDuration(timeoutConfig.ResponseHeader, 60*time.Second),
			IdleConnection: parseDuration(timeoutConfig.IdleConnection, 90*time.Second),
			OverallRequest: parseDuration(timeoutConfig.OverallRequest, 0),
		},
		ProxyConfig: proxyConfig,
	}
	
	return factory.CreateClient(clientConfig)
}

// CreateHealthClient 为健康检查创建HTTP客户端（使用与代理相同的配置，但超时较短）
func (e *Endpoint) CreateHealthClient(timeoutConfig config.HealthCheckTimeoutConfig) (*http.Client, error) {
	e.mutex.RLock()
	proxyConfig := e.Proxy
	e.mutex.RUnlock()
	
	factory := httpclient.NewFactory()
	clientConfig := httpclient.ClientConfig{
		Type: httpclient.ClientTypeHealth,
		Timeouts: httpclient.TimeoutConfig{
			TLSHandshake:   parseDuration(timeoutConfig.TLSHandshake, 5*time.Second),
			ResponseHeader: parseDuration(timeoutConfig.ResponseHeader, 30*time.Second),
			IdleConnection: parseDuration(timeoutConfig.IdleConnection, 60*time.Second),
			OverallRequest: parseDuration(timeoutConfig.OverallRequest, 30*time.Second),
		},
		ProxyConfig: proxyConfig,
	}
	
	return factory.CreateClient(clientConfig)
}

// RefreshOAuthToken 刷新 OAuth token
func (e *Endpoint) RefreshOAuthToken(timeoutConfig config.ProxyTimeoutConfig) error {
	return e.RefreshOAuthTokenWithCallback(timeoutConfig, nil)
}

// RefreshOAuthTokenWithCallback 刷新 OAuth token 并可选地调用回调函数
func (e *Endpoint) RefreshOAuthTokenWithCallback(timeoutConfig config.ProxyTimeoutConfig, onTokenRefreshed func(*Endpoint) error) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	if e.AuthType != "oauth" {
		return fmt.Errorf("endpoint is not configured for oauth authentication")
	}
	
	if e.OAuthConfig == nil {
		return fmt.Errorf("oauth config is nil")
	}
	
	// 创建HTTP客户端用于刷新请求
	factory := httpclient.NewFactory()
	clientConfig := httpclient.ClientConfig{
		Type: httpclient.ClientTypeProxy,
		Timeouts: httpclient.TimeoutConfig{
			TLSHandshake:   parseDuration(timeoutConfig.TLSHandshake, 10*time.Second),
			ResponseHeader: parseDuration(timeoutConfig.ResponseHeader, 60*time.Second),
			IdleConnection: parseDuration(timeoutConfig.IdleConnection, 90*time.Second),
			OverallRequest: parseDuration(timeoutConfig.OverallRequest, 30*time.Second),
		},
		ProxyConfig: e.Proxy,
	}
	
	client, err := factory.CreateClient(clientConfig)
	if err != nil {
		return fmt.Errorf("failed to create http client for token refresh: %v", err)
	}
	
	// 刷新token
	newOAuthConfig, err := oauth.RefreshToken(e.OAuthConfig, client)
	if err != nil {
		return fmt.Errorf("failed to refresh oauth token: %v", err)
	}
	
	// 更新配置
	e.OAuthConfig = newOAuthConfig
	
	// 如果提供了回调函数，调用它来处理配置持久化
	if onTokenRefreshed != nil {
		if err := onTokenRefreshed(e); err != nil {
			// 回调失败，但token已经刷新成功，只记录错误
			return fmt.Errorf("oauth token refreshed successfully but failed to persist to config file: %v", err)
		}
	}
	
	return nil
}

// GetAuthHeaderWithRefresh 获取认证头部，如果需要会自动刷新OAuth token
func (e *Endpoint) GetAuthHeaderWithRefresh(timeoutConfig config.ProxyTimeoutConfig) (string, error) {
	return e.GetAuthHeaderWithRefreshCallback(timeoutConfig, nil)
}

// GetAuthHeaderWithRefreshCallback 获取认证头部，如果需要会自动刷新OAuth token，支持回调
func (e *Endpoint) GetAuthHeaderWithRefreshCallback(timeoutConfig config.ProxyTimeoutConfig, onTokenRefreshed func(*Endpoint) error) (string, error) {
	// 首先尝试获取认证头部
	authHeader, err := e.GetAuthHeader()
	
	if e.AuthType == "oauth" {
		if err != nil {
			// 如果获取失败且token确实过期，尝试刷新
			if oauth.IsTokenExpired(e.OAuthConfig) {
				if refreshErr := e.RefreshOAuthTokenWithCallback(timeoutConfig, onTokenRefreshed); refreshErr != nil {
					return "", fmt.Errorf("failed to refresh oauth token: %v", refreshErr)
				}
				// 重新获取认证头部
				return e.GetAuthHeader()
			}
			// 如果不是因为过期导致的错误，直接返回错误
			return "", err
		}
		
		// 即使获取成功，也检查是否应该主动刷新
		if oauth.ShouldRefreshToken(e.OAuthConfig) {
			// 主动刷新，但如果失败不影响当前请求
			if refreshErr := e.RefreshOAuthTokenWithCallback(timeoutConfig, onTokenRefreshed); refreshErr != nil {
				// 刷新失败，记录日志但继续使用当前token
				// 这里可以添加日志记录
			} else {
				// 刷新成功，获取新的认证头部
				if newAuthHeader, newErr := e.GetAuthHeader(); newErr == nil {
					return newAuthHeader, nil
				}
			}
		}
	}
	
	return authHeader, err
}
package httpclient

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"claude-code-companion/internal/config"
)

// ClientType 定义客户端类型
type ClientType string

const (
	ClientTypeProxy       ClientType = "proxy"
	ClientTypeHealth      ClientType = "health"
	ClientTypeEndpoint    ClientType = "endpoint"
)

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	TLSHandshake     time.Duration
	ResponseHeader   time.Duration
	IdleConnection   time.Duration
	OverallRequest   time.Duration // 0表示无超时
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Type            ClientType
	Timeouts        TimeoutConfig
	ProxyConfig     *config.ProxyConfig
	MaxIdleConns    int
	MaxIdlePerHost  int
	DisableKeepAlive bool
	InsecureSkipVerify bool
}

// Factory HTTP客户端工厂
type Factory struct {
	defaultConfigs map[ClientType]ClientConfig
}

// NewFactory 创建新的HTTP客户端工厂
func NewFactory() *Factory {
	return &Factory{
		defaultConfigs: map[ClientType]ClientConfig{
			ClientTypeProxy: {
				Type: ClientTypeProxy,
				Timeouts: TimeoutConfig{
					TLSHandshake:   10 * time.Second,
					ResponseHeader: 60 * time.Second,
					IdleConnection: 90 * time.Second,
					OverallRequest: 0, // 流式请求无超时
				},
				MaxIdleConns:   100,
				MaxIdlePerHost: 20,
			},
			ClientTypeHealth: {
				Type: ClientTypeHealth,
				Timeouts: TimeoutConfig{
					TLSHandshake:   5 * time.Second,
					ResponseHeader: 30 * time.Second,
					IdleConnection: 60 * time.Second,
					OverallRequest: 30 * time.Second,
				},
				MaxIdleConns:   50,
				MaxIdlePerHost: 10,
			},
			ClientTypeEndpoint: {
				Type: ClientTypeEndpoint,
				Timeouts: TimeoutConfig{
					TLSHandshake:   10 * time.Second,
					ResponseHeader: 60 * time.Second,
					IdleConnection: 90 * time.Second,
					OverallRequest: 0,
				},
				MaxIdleConns:   100,
				MaxIdlePerHost: 10,
			},
		},
	}
}

// CreateClient 根据配置创建HTTP客户端
func (f *Factory) CreateClient(config ClientConfig) (*http.Client, error) {
	// 合并默认配置
	if defaultConfig, exists := f.defaultConfigs[config.Type]; exists {
		config = f.mergeConfigs(defaultConfig, config)
	}

	transport := &http.Transport{
		TLSHandshakeTimeout:   config.Timeouts.TLSHandshake,
		ResponseHeaderTimeout: config.Timeouts.ResponseHeader,
		IdleConnTimeout:       config.Timeouts.IdleConnection,
		DisableKeepAlives:     config.DisableKeepAlive,
		MaxIdleConns:          config.MaxIdleConns,
		MaxIdleConnsPerHost:   config.MaxIdlePerHost,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		},
	}

	// 如果配置了代理，设置代理拨号器
	if config.ProxyConfig != nil {
		dialer, err := f.createProxyDialer(config.ProxyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy dialer: %v", err)
		}
		transport.DialContext = dialer.DialContext
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeouts.OverallRequest,
	}

	return client, nil
}

// CreateProxyClient 创建代理客户端（兼容性方法）
func (f *Factory) CreateProxyClient(timeouts TimeoutConfig) *http.Client {
	config := ClientConfig{
		Type:     ClientTypeProxy,
		Timeouts: timeouts,
	}
	client, _ := f.CreateClient(config)
	return client
}

// CreateHealthClient 创建健康检查客户端（兼容性方法）
func (f *Factory) CreateHealthClient(timeouts TimeoutConfig) *http.Client {
	config := ClientConfig{
		Type:     ClientTypeHealth,
		Timeouts: timeouts,
	}
	client, _ := f.CreateClient(config)
	return client
}

// CreateEndpointClient 创建端点客户端
func (f *Factory) CreateEndpointClient(proxyConfig *config.ProxyConfig, timeouts TimeoutConfig) (*http.Client, error) {
	config := ClientConfig{
		Type:        ClientTypeEndpoint,
		Timeouts:    timeouts,
		ProxyConfig: proxyConfig,
	}
	return f.CreateClient(config)
}

// mergeConfigs 合并配置，优先使用传入的配置
func (f *Factory) mergeConfigs(defaultConfig, userConfig ClientConfig) ClientConfig {
	result := defaultConfig
	
	// 只覆盖非零值
	if userConfig.Timeouts.TLSHandshake != 0 {
		result.Timeouts.TLSHandshake = userConfig.Timeouts.TLSHandshake
	}
	if userConfig.Timeouts.ResponseHeader != 0 {
		result.Timeouts.ResponseHeader = userConfig.Timeouts.ResponseHeader
	}
	if userConfig.Timeouts.IdleConnection != 0 {
		result.Timeouts.IdleConnection = userConfig.Timeouts.IdleConnection
	}
	if userConfig.Timeouts.OverallRequest != 0 {
		result.Timeouts.OverallRequest = userConfig.Timeouts.OverallRequest
	}
	if userConfig.MaxIdleConns != 0 {
		result.MaxIdleConns = userConfig.MaxIdleConns
	}
	if userConfig.MaxIdlePerHost != 0 {
		result.MaxIdlePerHost = userConfig.MaxIdlePerHost
	}
	if userConfig.ProxyConfig != nil {
		result.ProxyConfig = userConfig.ProxyConfig
	}
	
	result.DisableKeepAlive = userConfig.DisableKeepAlive
	result.InsecureSkipVerify = userConfig.InsecureSkipVerify
	
	return result
}

// ParseTimeoutWithDefault 解析超时字符串，失败时返回默认值
func ParseTimeoutWithDefault(value, fieldName string, defaultDuration time.Duration) (time.Duration, error) {
	if value == "" {
		return defaultDuration, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s timeout: %v", fieldName, err)
	}
	return d, nil
}
package utils

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

var (
	// Global HTTP client instances
	proxyClient     *http.Client
	healthClient    *http.Client
	clientMutex     sync.RWMutex // 替换sync.Once以支持重新配置
)

// TimeoutConfig represents timeout configuration for HTTP clients
type TimeoutConfig struct {
	TLSHandshake     time.Duration
	ResponseHeader   time.Duration
	IdleConnection   time.Duration
	OverallRequest   time.Duration // zero means no timeout
}

// InitHTTPClients initializes the global HTTP clients with default timeouts
func InitHTTPClients() {
	InitHTTPClientsWithTimeouts(TimeoutConfig{
		TLSHandshake:   10 * time.Second,
		ResponseHeader: 60 * time.Second,
		IdleConnection: 90 * time.Second,
		OverallRequest: 0, // no timeout for proxy
	}, TimeoutConfig{
		TLSHandshake:   5 * time.Second,
		ResponseHeader: 30 * time.Second,
		IdleConnection: 60 * time.Second,
		OverallRequest: 30 * time.Second,
	})
}

// InitHTTPClientsWithTimeouts initializes the global HTTP clients with custom timeouts
func InitHTTPClientsWithTimeouts(proxyTimeouts, healthTimeouts TimeoutConfig) {
	clientMutex.Lock()
	defer clientMutex.Unlock()
	
	// Proxy client - designed for long-running streaming requests
	proxyClient = &http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   proxyTimeouts.TLSHandshake,
			ResponseHeaderTimeout: proxyTimeouts.ResponseHeader,
			IdleConnTimeout:       proxyTimeouts.IdleConnection,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
		},
		Timeout: proxyTimeouts.OverallRequest, // 0 means no timeout
	}

	// Health check client - designed for quick health checks
	healthClient = &http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   healthTimeouts.TLSHandshake,
			ResponseHeaderTimeout: healthTimeouts.ResponseHeader,
			IdleConnTimeout:       healthTimeouts.IdleConnection,
			MaxIdleConns:          50,
			MaxIdleConnsPerHost:   10,
		},
		Timeout: healthTimeouts.OverallRequest,
	}
}

// GetProxyClient returns the shared HTTP client for proxy requests
func GetProxyClient() *http.Client {
	clientMutex.RLock()
	defer clientMutex.RUnlock()
	
	if proxyClient == nil {
		// 如果客户端未初始化，使用默认配置初始化
		clientMutex.RUnlock() // 释放读锁
		InitHTTPClients()     // 这会获取写锁
		clientMutex.RLock()   // 重新获取读锁
	}
	return proxyClient
}

// GetHealthClient returns the shared HTTP client for health checks
func GetHealthClient() *http.Client {
	clientMutex.RLock()
	defer clientMutex.RUnlock()
	
	if healthClient == nil {
		// 如果客户端未初始化，使用默认配置初始化
		clientMutex.RUnlock() // 释放读锁
		InitHTTPClients()     // 这会获取写锁
		clientMutex.RLock()   // 重新获取读锁
	}
	return healthClient
}

// ParseTimeoutWithDefault parses a timeout string with fallback to default
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


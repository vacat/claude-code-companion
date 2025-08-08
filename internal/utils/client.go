package utils

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var (
	// Global HTTP client instances
	proxyClient     *http.Client
	healthClient    *http.Client
	clientInitOnce  sync.Once
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
	clientInitOnce.Do(func() {
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
	})
}

// GetProxyClient returns the shared HTTP client for proxy requests
func GetProxyClient() *http.Client {
	InitHTTPClients()
	return proxyClient
}

// GetHealthClient returns the shared HTTP client for health checks
func GetHealthClient() *http.Client {
	InitHTTPClients()
	return healthClient
}

// CreateRequest creates a new HTTP request with proper error handling
func CreateRequest(method, url string, body []byte) (*http.Request, error) {
	if body != nil {
		return http.NewRequest(method, url, bytes.NewReader(body))
	}
	return http.NewRequest(method, url, nil)
}

// parseTimeoutWithDefault parses a timeout string with fallback to default
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
package utils

import (
	"bytes"
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

// InitHTTPClients initializes the global HTTP clients
func InitHTTPClients() {
	clientInitOnce.Do(func() {
		// Proxy client - designed for long-running streaming requests
		proxyClient = &http.Client{
			Transport: &http.Transport{
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second, // Long enough for LLM to start responding
				IdleConnTimeout:       90 * time.Second,
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
			},
			// No timeout for streaming responses
		}

		// Health check client - designed for quick health checks
		healthClient = &http.Client{
			Transport: &http.Transport{
				TLSHandshakeTimeout:   5 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       60 * time.Second,
				MaxIdleConns:          50,
				MaxIdleConnsPerHost:   10,
			},
			Timeout: 30 * time.Second,
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
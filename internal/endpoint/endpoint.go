package endpoint

import (
	"fmt"
	"sync"
	"time"

	"claude-proxy/internal/config"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusChecking Status = "checking"
)

type Endpoint struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	URL             string        `json:"url"`
	PathPrefix      string        `json:"path_prefix"`
	AuthType        string        `json:"auth_type"`
	AuthValue       string        `json:"auth_value"`
	Timeout         time.Duration `json:"timeout"`
	Enabled         bool          `json:"enabled"`
	Priority        int           `json:"priority"`
	Status          Status        `json:"status"`
	LastCheck       time.Time     `json:"last_check"`
	FailureCount    int           `json:"failure_count"`
	TotalRequests   int           `json:"total_requests"`
	SuccessRequests int           `json:"success_requests"`
	LastFailure     time.Time     `json:"last_failure"`
	mutex           sync.RWMutex
}

func NewEndpoint(config config.EndpointConfig) *Endpoint {
	return &Endpoint{
		ID:         generateID(config.Name),
		Name:       config.Name,
		URL:        config.URL,
		PathPrefix: config.PathPrefix,
		AuthType:   config.AuthType,
		AuthValue:  config.AuthValue,
		Timeout:    config.GetTimeout(),
		Enabled:    config.Enabled,
		Priority:   config.Priority,
		Status:     StatusActive,
		LastCheck:  time.Now(),
	}
}

func (e *Endpoint) GetAuthHeader() string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	switch e.AuthType {
	case "api_key":
		return e.AuthValue // api_key 直接返回值，会用 x-api-key 头部
	case "auth_token":
		return "Bearer " + e.AuthValue // auth_token 使用 Bearer 前缀
	default:
		return e.AuthValue
	}
}

func (e *Endpoint) GetFullURL(path string) string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.URL + "/v1" + path
}

func (e *Endpoint) IsAvailable() bool {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	return e.Enabled && e.Status == StatusActive
}

func (e *Endpoint) RecordRequest(success bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.TotalRequests++
	if success {
		e.SuccessRequests++
		e.FailureCount = 0 // 重置失败计数
	} else {
		e.FailureCount++
		e.LastFailure = time.Now()
	}
	// 不再改变端点状态，保持简单
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
}

type requestStats struct {
	success int
	failed  int
}

func (e *Endpoint) getRecentRequests(duration time.Duration) requestStats {
	// 简化实现，基于当前失败计数
	// 实际应该维护一个时间窗口内的请求历史
	return requestStats{
		success: e.SuccessRequests,
		failed:  e.FailureCount,
	}
}

func generateID(name string) string {
	return fmt.Sprintf("endpoint-%s-%d", name, time.Now().Unix())
}
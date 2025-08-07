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

type RequestRecord struct {
	Timestamp time.Time
	Success   bool
}

type Endpoint struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	URL             string          `json:"url"`
	PathPrefix      string          `json:"path_prefix"`
	AuthType        string          `json:"auth_type"`
	AuthValue       string          `json:"auth_value"`
	Enabled         bool            `json:"enabled"`
	Priority        int             `json:"priority"`
	Status          Status          `json:"status"`
	LastCheck       time.Time       `json:"last_check"`
	FailureCount    int             `json:"failure_count"`
	TotalRequests   int             `json:"total_requests"`
	SuccessRequests int             `json:"success_requests"`
	LastFailure     time.Time       `json:"last_failure"`
	RequestHistory  []RequestRecord `json:"-"` // 请求历史，不导出到JSON
	mutex           sync.RWMutex
}

func NewEndpoint(config config.EndpointConfig) *Endpoint {
	return &Endpoint{
		ID:             generateID(config.Name),
		Name:           config.Name,
		URL:            config.URL,
		PathPrefix:     config.PathPrefix,
		AuthType:       config.AuthType,
		AuthValue:      config.AuthValue,
		Enabled:        config.Enabled,
		Priority:       config.Priority,
		Status:         StatusActive,
		LastCheck:      time.Now(),
		RequestHistory: make([]RequestRecord, 0),
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

	now := time.Now()
	
	// 添加到请求历史
	e.RequestHistory = append(e.RequestHistory, RequestRecord{
		Timestamp: now,
		Success:   success,
	})
	
	// 清理140秒前的历史记录
	e.cleanOldHistory(now)
	
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
		
		// 检查140秒内的请求情况，判断是否需要标记为不可用
		e.checkAndUpdateStatus(now)
	}
}

// 清理140秒前的历史记录
func (e *Endpoint) cleanOldHistory(now time.Time) {
	cutoff := now.Add(-140 * time.Second)
	validIndex := 0
	
	for i, record := range e.RequestHistory {
		if record.Timestamp.After(cutoff) {
			e.RequestHistory[validIndex] = e.RequestHistory[i]
			validIndex++
		}
	}
	
	e.RequestHistory = e.RequestHistory[:validIndex]
}

// 检查140秒内的请求情况，判断是否需要标记为不可用
func (e *Endpoint) checkAndUpdateStatus(now time.Time) {
	// 只有当前是可用状态时才检查
	if e.Status != StatusActive {
		return
	}
	
	cutoff := now.Add(-140 * time.Second)
	totalRequests := 0
	failedRequests := 0
	
	for _, record := range e.RequestHistory {
		if record.Timestamp.After(cutoff) {
			totalRequests++
			if !record.Success {
				failedRequests++
			}
		}
	}
	
	// 140秒内请求数超过1且全部失败时，标记为不可用
	if totalRequests > 1 && failedRequests == totalRequests {
		e.Status = StatusInactive
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
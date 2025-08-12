package endpoint

import (
	"fmt"
	"sync"
	"time"

	"claude-proxy/internal/config"
	"claude-proxy/internal/interfaces"
	"claude-proxy/internal/utils"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusChecking Status = "checking"
)

// 删除不再需要的 RequestRecord 定义，因为已经移到 utils 包

type Endpoint struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	URL             string                   `json:"url"`
	EndpointType    string                   `json:"endpoint_type"` // "anthropic" | "openai" 等
	PathPrefix      string                   `json:"path_prefix,omitempty"` // OpenAI端点的路径前缀
	AuthType        string                   `json:"auth_type"`
	AuthValue       string                   `json:"auth_value"`
	Enabled         bool                     `json:"enabled"`
	Priority        int                      `json:"priority"`
	Tags            []string                 `json:"tags"`           // 新增：支持的tag列表
	ModelRewrite    *config.ModelRewriteConfig `json:"model_rewrite,omitempty"` // 新增：模型重写配置
	Status          Status                   `json:"status"`
	LastCheck       time.Time                `json:"last_check"`
	FailureCount    int                      `json:"failure_count"`
	TotalRequests   int                      `json:"total_requests"`
	SuccessRequests int                      `json:"success_requests"`
	LastFailure     time.Time                `json:"last_failure"`
	RequestHistory  *utils.CircularBuffer    `json:"-"` // 使用环形缓冲区，不导出到JSON
	mutex           sync.RWMutex
}

func NewEndpoint(config config.EndpointConfig) *Endpoint {
	// 如果没有指定 endpoint_type，默认为 anthropic （向后兼容）
	endpointType := config.EndpointType
	if endpointType == "" {
		endpointType = "anthropic"
	}
	
	return &Endpoint{
		ID:             generateID(config.Name),
		Name:           config.Name,
		URL:            config.URL,
		EndpointType:   endpointType,
		PathPrefix:     config.PathPrefix,  // 新增：复制PathPrefix
		AuthType:       config.AuthType,
		AuthValue:      config.AuthValue,
		Enabled:        config.Enabled,
		Priority:       config.Priority,
		Tags:           config.Tags,       // 新增：从配置中复制tags
		ModelRewrite:   config.ModelRewrite, // 新增：从配置中复制模型重写配置
		Status:         StatusActive,
		LastCheck:      time.Now(),
		RequestHistory: utils.NewCircularBuffer(100, 140*time.Second), // 100个记录，140秒窗口
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
	
	// 根据端点类型自动添加正确的路径前缀
	switch e.EndpointType {
	case "anthropic":
		// Anthropic 端点固定使用 /v1/messages
		return e.URL + "/v1/messages"
	case "openai":
		// OpenAI 端点使用配置的路径前缀（不需要路径转换）
		return e.URL + e.PathPrefix
	default:
		// 向后兼容：默认使用 anthropic 格式
		return e.URL + "/v1/messages"
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

type requestStats struct {
	success int
	failed  int
}

// getRecentRequests returns recent request statistics using the circular buffer
func (e *Endpoint) getRecentRequests(duration time.Duration) requestStats {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	now := time.Now()
	total, failed := e.RequestHistory.GetWindowStats(now)
	success := total - failed
	
	return requestStats{
		success: success,
		failed:  failed,
	}
}

func generateID(name string) string {
	return fmt.Sprintf("endpoint-%s-%d", name, time.Now().Unix())
}
package endpoint

import (
	"sync"
	"time"

	"claude-proxy/internal/config"
)

type HealthChecker interface {
	CheckEndpoint(ep *Endpoint) error
}

type Manager struct {
	selector        *Selector
	endpoints       []*Endpoint
	config          *config.Config
	mutex           sync.RWMutex
	healthChecker   HealthChecker
	healthTickers   map[string]*time.Ticker
}

func NewManager(cfg *config.Config) *Manager {
	endpoints := make([]*Endpoint, 0, len(cfg.Endpoints))
	for _, endpointConfig := range cfg.Endpoints {
		endpoint := NewEndpoint(endpointConfig)
		endpoints = append(endpoints, endpoint)
	}

	manager := &Manager{
		selector:        NewSelector(endpoints),
		endpoints:       endpoints,
		config:          cfg,
		healthChecker:   nil, // 稍后设置
		healthTickers:   make(map[string]*time.Ticker),
	}

	return manager
}

func (m *Manager) GetEndpoint() (*Endpoint, error) {
	return m.selector.SelectEndpoint()
}

// GetEndpointWithTags 根据tags选择endpoint
func (m *Manager) GetEndpointWithTags(tags []string) (*Endpoint, error) {
	return m.selector.SelectEndpointWithTags(tags)
}

func (m *Manager) GetAllEndpoints() []*Endpoint {
	return m.selector.GetAllEndpoints()
}

func (m *Manager) RecordRequest(endpointID string, success bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, endpoint := range m.endpoints {
		if endpoint.ID == endpointID {
			endpoint.RecordRequest(success)
			break
		}
	}
}

func (m *Manager) UpdateEndpoints(endpointConfigs []config.EndpointConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	newEndpoints := make([]*Endpoint, 0, len(endpointConfigs))
	for _, cfg := range endpointConfigs {
		endpoint := NewEndpoint(cfg)
		newEndpoints = append(newEndpoints, endpoint)
	}

	// 停止旧的健康检查
	m.stopHealthChecks()

	m.endpoints = newEndpoints
	m.selector.UpdateEndpoints(newEndpoints)
	
	// 重新启动健康检查
	m.startHealthChecks()
}


func (m *Manager) SetHealthChecker(checker HealthChecker) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.healthChecker = checker
	
	// 启动健康检查
	m.startHealthChecks()
}

func (m *Manager) startHealthChecks() {
	// 如果没有健康检查器，不启动
	if m.healthChecker == nil {
		return
	}

	// 获取健康检查间隔配置，默认30秒
	interval := 30 * time.Second
	if m.config.Timeouts.HealthCheck.CheckInterval != "" {
		if d, err := time.ParseDuration(m.config.Timeouts.HealthCheck.CheckInterval); err == nil {
			interval = d
		}
	}
	
	for _, endpoint := range m.endpoints {
		if endpoint.Enabled {
			ticker := time.NewTicker(interval)
			m.healthTickers[endpoint.ID] = ticker
			
			go m.runHealthCheck(endpoint, ticker)
		}
	}
}

func (m *Manager) stopHealthChecks() {
	for _, ticker := range m.healthTickers {
		ticker.Stop()
	}
	m.healthTickers = make(map[string]*time.Ticker)
}

func (m *Manager) runHealthCheck(endpoint *Endpoint, ticker *time.Ticker) {
	for range ticker.C {
		// 只对不可用的端点进行健康检查
		if endpoint.Status != StatusInactive {
			continue
		}
		
		if err := m.healthChecker.CheckEndpoint(endpoint); err != nil {
			// 健康检查失败，保持不可用状态
		} else {
			// 健康检查成功，恢复为可用状态
			endpoint.MarkActive()
		}
	}
}
package endpoint

import (
	"sync"
	"time"

	"claude-proxy/internal/config"
)

type Manager struct {
	selector     *Selector
	endpoints    []*Endpoint
	config       *config.Config
	mutex        sync.RWMutex
	retryTimers  map[string]*time.Timer
}

func NewManager(cfg *config.Config) *Manager {
	endpoints := make([]*Endpoint, 0, len(cfg.Endpoints))
	for _, endpointConfig := range cfg.Endpoints {
		endpoint := NewEndpoint(endpointConfig)
		endpoints = append(endpoints, endpoint)
	}

	return &Manager{
		selector:    NewSelector(endpoints),
		endpoints:   endpoints,
		config:      cfg,
		retryTimers: make(map[string]*time.Timer),
	}
}

func (m *Manager) GetEndpoint() (*Endpoint, error) {
	return m.selector.SelectEndpoint()
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
			
			if !success && endpoint.Status == StatusInactive {
				m.scheduleRetry(endpoint)
			}
			break
		}
	}
}

func (m *Manager) scheduleRetry(endpoint *Endpoint) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if timer, exists := m.retryTimers[endpoint.ID]; exists {
		timer.Stop()
	}

	m.retryTimers[endpoint.ID] = time.AfterFunc(60*time.Second, func() {
		endpoint.MarkActive()
		m.mutex.Lock()
		delete(m.retryTimers, endpoint.ID)
		m.mutex.Unlock()
	})
}

func (m *Manager) UpdateEndpoints(endpointConfigs []config.EndpointConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	newEndpoints := make([]*Endpoint, 0, len(endpointConfigs))
	for _, cfg := range endpointConfigs {
		endpoint := NewEndpoint(cfg)
		newEndpoints = append(newEndpoints, endpoint)
	}

	for _, timer := range m.retryTimers {
		timer.Stop()
	}
	m.retryTimers = make(map[string]*time.Timer)

	m.endpoints = newEndpoints
	m.selector.UpdateEndpoints(newEndpoints)
}
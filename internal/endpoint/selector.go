package endpoint

import (
	"fmt"
	"sort"
	"sync"
)

type Selector struct {
	endpoints []*Endpoint
	mutex     sync.RWMutex
}

func NewSelector(endpoints []*Endpoint) *Selector {
	return &Selector{
		endpoints: endpoints,
	}
}

func (s *Selector) SelectEndpoint() (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 按优先级排序所有启用的端点
	enabledEndpoints := make([]*Endpoint, 0)
	for _, ep := range s.endpoints {
		if ep.Enabled {
			enabledEndpoints = append(enabledEndpoints, ep)
		}
	}

	if len(enabledEndpoints) == 0 {
		return nil, fmt.Errorf("no enabled endpoints available")
	}

	// 按优先级排序（数字越小优先级越高）
	sort.Slice(enabledEndpoints, func(i, j int) bool {
		return enabledEndpoints[i].Priority < enabledEndpoints[j].Priority
	})

	// 返回第一个可用的端点
	for _, ep := range enabledEndpoints {
		if ep.IsAvailable() {
			return ep, nil
		}
	}

	// 如果没有可用端点，返回错误
	return nil, fmt.Errorf("no available endpoints found")
}

func (s *Selector) GetAllEndpoints() []*Endpoint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make([]*Endpoint, len(s.endpoints))
	copy(result, s.endpoints)
	return result
}

func (s *Selector) UpdateEndpoints(endpoints []*Endpoint) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.endpoints = endpoints
}
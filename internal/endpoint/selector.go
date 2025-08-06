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

	// 首先尝试找到可用的端点
	availableEndpoints := make([]*Endpoint, 0)
	for _, ep := range s.endpoints {
		if ep.IsAvailable() {
			availableEndpoints = append(availableEndpoints, ep)
		}
	}

	if len(availableEndpoints) == 0 {
		// 如果没有可用端点，重置所有端点状态并重试一次
		for _, ep := range s.endpoints {
			if ep.Enabled {
				ep.MarkActive()
				availableEndpoints = append(availableEndpoints, ep)
			}
		}
	}

	if len(availableEndpoints) == 0 {
		return nil, fmt.Errorf("no active endpoints available")
	}

	// 按优先级排序
	sort.Slice(availableEndpoints, func(i, j int) bool {
		return availableEndpoints[i].Priority < availableEndpoints[j].Priority
	})

	return availableEndpoints[0], nil
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
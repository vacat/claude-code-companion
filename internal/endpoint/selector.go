package endpoint

import (
	"fmt"
	"sync"

	"claude-proxy/internal/utils"
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

	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(s.endpoints))
	for i, ep := range s.endpoints {
		sorterEndpoints[i] = ep
	}

	// 使用统一的端点选择逻辑
	selected := utils.SelectBestEndpoint(sorterEndpoints)
	if selected == nil {
		return nil, fmt.Errorf("no available endpoints found")
	}

	// 类型断言转换回 *Endpoint
	return selected.(*Endpoint), nil
}

func (s *Selector) GetAllEndpoints() []*Endpoint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 返回切片引用而不是拷贝，因为端点数据本身是不可变的
	// 调用者不应该修改返回的切片
	return s.endpoints
}

func (s *Selector) UpdateEndpoints(endpoints []*Endpoint) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.endpoints = endpoints
}
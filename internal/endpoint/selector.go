package endpoint

import (
	"fmt"
	"sync"

	"claude-proxy/internal/interfaces"
	"claude-proxy/internal/routing"
	"claude-proxy/internal/utils"
)

type Selector struct {
	endpoints []*Endpoint
	matcher   *routing.TagMatcher  // 新增：tag匹配器
	mutex     sync.RWMutex
}

func NewSelector(endpoints []*Endpoint) *Selector {
	return &Selector{
		endpoints: endpoints,
		matcher:   routing.NewTagMatcher(),  // 初始化tag匹配器
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

// SelectEndpointWithTags 根据tags选择endpoint
func (s *Selector) SelectEndpointWithTags(tags []string) (*Endpoint, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 转换为TaggedEndpoint
	taggedEndpoints := make([]interfaces.TaggedEndpoint, len(s.endpoints))
	for i, ep := range s.endpoints {
		taggedEndpoints[i] = ep.ToTaggedEndpoint()
	}

	// 使用tag匹配器过滤endpoint
	matchedEndpoints := s.matcher.MatchEndpoints(tags, taggedEndpoints)
	if len(matchedEndpoints) == 0 {
		return nil, fmt.Errorf("no endpoints match the required tags: %v", tags)
	}

	// 在匹配的endpoint中按优先级选择最佳的
	var bestEndpoint *Endpoint
	for _, matched := range matchedEndpoints {
		// 从原始endpoints中找到对应的endpoint
		for _, ep := range s.endpoints {
			if ep.Name == matched.Name && ep.IsAvailable() {
				if bestEndpoint == nil || ep.Priority < bestEndpoint.Priority {
					bestEndpoint = ep
				}
				break
			}
		}
	}

	if bestEndpoint == nil {
		return nil, fmt.Errorf("no available endpoints match the required tags: %v", tags)
	}

	return bestEndpoint, nil
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
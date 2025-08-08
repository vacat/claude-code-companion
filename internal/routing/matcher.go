package routing

import (
	"claude-proxy/internal/interfaces"
)

// TagMatcher 实现tag匹配算法
type TagMatcher struct{}

// NewTagMatcher 创建新的tag匹配器
func NewTagMatcher() *TagMatcher {
	return &TagMatcher{}
}

// MatchEndpoints 根据请求tags过滤可用的endpoint
// 匹配规则：
// 1. 如果请求没有tag，可以匹配任意endpoint
// 2. 如果请求有tag，endpoint必须拥有请求的所有tag
// 3. 如果endpoint没有tag，被视为支持所有tag（万能endpoint）
func (tm *TagMatcher) MatchEndpoints(requestTags []string, endpoints []interfaces.TaggedEndpoint) []interfaces.TaggedEndpoint {
	var matchedEndpoints []interfaces.TaggedEndpoint

	for _, endpoint := range endpoints {
		if !endpoint.Enabled {
			continue // 跳过已禁用的endpoint
		}

		if tm.isEndpointMatch(requestTags, endpoint.Tags) {
			matchedEndpoints = append(matchedEndpoints, endpoint)
		}
	}

	return matchedEndpoints
}

// isEndpointMatch 判断endpoint是否匹配请求的tags
func (tm *TagMatcher) isEndpointMatch(requestTags []string, endpointTags []string) bool {
	// Case 1: 请求没有tag，可以匹配任意endpoint
	if len(requestTags) == 0 {
		return true
	}

	// Case 2: endpoint没有tag，被视为支持所有tag（万能endpoint）
	if len(endpointTags) == 0 {
		return true
	}

	// Case 3: endpoint必须包含请求的所有tag
	return tm.containsAllTags(endpointTags, requestTags)
}

// containsAllTags 检查endpointTags是否包含requestTags中的所有tag
func (tm *TagMatcher) containsAllTags(endpointTags []string, requestTags []string) bool {
	// 创建endpoint tags的map以便快速查找
	endpointTagMap := make(map[string]bool)
	for _, tag := range endpointTags {
		endpointTagMap[tag] = true
	}

	// 检查请求的所有tag是否都在endpoint中
	for _, requestTag := range requestTags {
		if !endpointTagMap[requestTag] {
			return false // 发现不匹配的tag
		}
	}

	return true // 所有tag都匹配
}

// EndpointMatchResult 记录endpoint匹配结果和原因
type EndpointMatchResult struct {
	Endpoint     interfaces.TaggedEndpoint
	Matched      bool
	MatchReason  string
	Selected     bool
}

// MatchEndpointsWithReason 根据请求tags过滤endpoint，并返回详细的匹配结果
func (tm *TagMatcher) MatchEndpointsWithReason(requestTags []string, endpoints []interfaces.TaggedEndpoint) []EndpointMatchResult {
	var results []EndpointMatchResult

	for _, endpoint := range endpoints {
		var matched bool
		var reason string

		if !endpoint.Enabled {
			matched = false
			reason = "endpoint disabled"
		} else if len(requestTags) == 0 {
			matched = true
			reason = "no request tags, matches any endpoint"
		} else if len(endpoint.Tags) == 0 {
			matched = true
			reason = "universal endpoint (no tags configured)"
		} else if tm.containsAllTags(endpoint.Tags, requestTags) {
			matched = true
			reason = "endpoint contains all required tags"
		} else {
			matched = false
			reason = "endpoint missing required tags"
		}

		results = append(results, EndpointMatchResult{
			Endpoint:    endpoint,
			Matched:     matched,
			MatchReason: reason,
			Selected:    false, // 由调用方设置
		})
	}

	return results
}
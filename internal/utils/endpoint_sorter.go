package utils

import (
	"sort"
)

// EndpointSorter interface for sorting endpoints
type EndpointSorter interface {
	GetPriority() int
	IsEnabled() bool
	IsAvailable() bool
	GetTags() []string
}

// SortEndpointsByTagsAndPriority sorts endpoints by tag matching and priority
// requiredTags: 请求需要的标签
// 排序规则:
// 1. 满足所有 requiredTags 的 endpoint 按 priority 排序
// 2. 万用 endpoint (无 tag 限制) 按 priority 排序
// 3. 不满足条件的 endpoint 按 priority 排序
func SortEndpointsByTagsAndPriority(endpoints []EndpointSorter, requiredTags []string) {
	sort.Slice(endpoints, func(i, j int) bool {
		endpointI := endpoints[i]
		endpointJ := endpoints[j]
		
		tierI := getEndpointTier(endpointI.GetTags(), requiredTags)
		tierJ := getEndpointTier(endpointJ.GetTags(), requiredTags)
		
		// 先按tier排序（数字越小优先级越高）
		if tierI != tierJ {
			return tierI < tierJ
		}
		
		// 同tier内按priority排序（数字越小优先级越高）
		return endpointI.GetPriority() < endpointJ.GetPriority()
	})
}

// getEndpointTier 计算端点的优先级层级
// 返回值：0=完全匹配（最高优先级），1=万用端点（中等优先级），2=不匹配（最低优先级）
func getEndpointTier(endpointTags, requiredTags []string) int {
	if len(requiredTags) == 0 {
		return 0 // 无标签要求时，所有端点同级
	}
	
	if matchesAllTags(endpointTags, requiredTags) {
		return 0 // 完全匹配，最高优先级
	}
	
	if len(endpointTags) == 0 {
		return 1 // 万用端点，中等优先级
	}
	
	return 2 // 不匹配，最低优先级
}

// matchesAllTags 检查 endpoint 的 tags 是否包含所有 requiredTags
func matchesAllTags(endpointTags, requiredTags []string) bool {
	if len(requiredTags) == 0 {
		return true // 如果没有要求任何标签，则认为匹配
	}
	
	tagSet := make(map[string]bool)
	for _, tag := range endpointTags {
		tagSet[tag] = true
	}
	
	for _, required := range requiredTags {
		if !tagSet[required] {
			return false
		}
	}
	return true
}

// FilterEndpointsForTags 过滤出满足标签要求的 endpoint
func FilterEndpointsForTags(endpoints []EndpointSorter, requiredTags []string) []EndpointSorter {
	if len(requiredTags) == 0 {
		return endpoints // 如果没有标签要求，返回所有 endpoint
	}
	
	filtered := make([]EndpointSorter, 0)
	for _, ep := range endpoints {
		tags := ep.GetTags()
		// 要么完全匹配所有标签，要么是万用 endpoint
		if matchesAllTags(tags, requiredTags) || len(tags) == 0 {
			filtered = append(filtered, ep)
		}
	}
	return filtered
}

// FilterEnabledEndpoints filters out disabled endpoints
func FilterEnabledEndpoints(endpoints []EndpointSorter) []EndpointSorter {
	return FilterEndpoints(endpoints, func(ep EndpointSorter) bool {
		return ep.IsEnabled()
	})
}

// FilterAvailableEndpoints filters out unavailable endpoints
func FilterAvailableEndpoints(endpoints []EndpointSorter) []EndpointSorter {
	return FilterEndpoints(endpoints, func(ep EndpointSorter) bool {
		return ep.IsAvailable()
	})
}

// FilterEndpoints applies a generic filter predicate to endpoints
func FilterEndpoints(endpoints []EndpointSorter, predicate func(EndpointSorter) bool) []EndpointSorter {
	filtered := make([]EndpointSorter, 0, len(endpoints))
	for _, ep := range endpoints {
		if predicate(ep) {
			filtered = append(filtered, ep)
		}
	}
	return filtered
}

// SortEndpointsByPriority sorts endpoints by priority (lower number = higher priority)
func SortEndpointsByPriority(endpoints []EndpointSorter) {
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].GetPriority() < endpoints[j].GetPriority()
	})
}

// SelectBestEndpoint selects the first available endpoint from sorted, enabled endpoints
func SelectBestEndpoint(endpoints []EndpointSorter) EndpointSorter {
	enabled := FilterEnabledEndpoints(endpoints)
	if len(enabled) == 0 {
		return nil
	}

	SortEndpointsByPriority(enabled)
	
	for _, ep := range enabled {
		if ep.IsAvailable() {
			return ep
		}
	}

	return nil
}

// SelectBestEndpointWithTags selects the first available endpoint matching the tags
func SelectBestEndpointWithTags(endpoints []EndpointSorter, requiredTags []string) EndpointSorter {
	// 首先过滤出启用的端点
	enabled := FilterEnabledEndpoints(endpoints)
	if len(enabled) == 0 {
		return nil
	}
	
	// 过滤出满足标签要求的端点
	filtered := FilterEndpointsForTags(enabled, requiredTags)
	if len(filtered) == 0 {
		return nil
	}
	
	// 按标签匹配和优先级排序
	SortEndpointsByTagsAndPriority(filtered, requiredTags)
	
	// 选择第一个可用的端点
	for _, ep := range filtered {
		if ep.IsAvailable() {
			return ep
		}
	}

	return nil
}
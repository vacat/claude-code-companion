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
func SortEndpointsByTagsAndPriority(endpoints []EndpointSorter, requiredTags []string) {
	sort.Slice(endpoints, func(i, j int) bool {
		endpointI := endpoints[i]
		endpointJ := endpoints[j]
		
		tagsI := endpointI.GetTags()
		tagsJ := endpointJ.GetTags()
		
		matchesI := matchesAllTags(tagsI, requiredTags)
		matchesJ := matchesAllTags(tagsJ, requiredTags)
		
		isUniversalI := len(tagsI) == 0  // 万用 endpoint
		isUniversalJ := len(tagsJ) == 0  // 万用 endpoint
		
		// 排序逻辑：
		// 1. 完全匹配的 endpoint 优先
		// 2. 万用 endpoint 其次
		// 3. 不满足条件的 endpoint 最后（实际上会被过滤掉）
		// 4. 相同类型内按 priority 排序
		
		if matchesI && matchesJ {
			// 都完全匹配，按 priority 排序
			return endpointI.GetPriority() < endpointJ.GetPriority()
		}
		if matchesI && !matchesJ {
			// i 匹配，j 不匹配
			if isUniversalJ {
				// i 匹配，j 万用，i 优先
				return true
			}
			// i 匹配，j 既不匹配也不万用，i 优先
			return true
		}
		if !matchesI && matchesJ {
			// j 匹配，i 不匹配
			if isUniversalI {
				// j 匹配，i 万用，j 优先
				return false
			}
			// j 匹配，i 既不匹配也不万用，j 优先
			return false
		}
		
		// 都不匹配
		if isUniversalI && isUniversalJ {
			// 都是万用，按 priority 排序
			return endpointI.GetPriority() < endpointJ.GetPriority()
		}
		if isUniversalI && !isUniversalJ {
			// i 万用，j 不万用，i 优先
			return true
		}
		if !isUniversalI && isUniversalJ {
			// j 万用，i 不万用，j 优先
			return false
		}
		
		// 都不匹配也不万用，按 priority 排序
		return endpointI.GetPriority() < endpointJ.GetPriority()
	})
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
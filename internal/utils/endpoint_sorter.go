package utils

import (
	"sort"
)

// EndpointSorter interface for sorting endpoints
type EndpointSorter interface {
	GetPriority() int
	IsEnabled() bool
	IsAvailable() bool
}

// SortEndpointsByPriority sorts endpoints by priority (lower number = higher priority)
func SortEndpointsByPriority(endpoints []EndpointSorter) {
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].GetPriority() < endpoints[j].GetPriority()
	})
}

// FilterEnabledEndpoints filters out disabled endpoints
func FilterEnabledEndpoints(endpoints []EndpointSorter) []EndpointSorter {
	enabled := make([]EndpointSorter, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.IsEnabled() {
			enabled = append(enabled, ep)
		}
	}
	return enabled
}

// FilterAvailableEndpoints filters out unavailable endpoints
func FilterAvailableEndpoints(endpoints []EndpointSorter) []EndpointSorter {
	available := make([]EndpointSorter, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.IsAvailable() {
			available = append(available, ep)
		}
	}
	return available
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
package proxy

import (
	"fmt"
	"net/http"
	"time"

	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/tagging"
	"claude-proxy/internal/utils"

	"github.com/gin-gonic/gin"
)

// tryProxyRequest attempts to proxy the request to the given endpoint
func (s *Server) tryProxyRequest(c *gin.Context, ep *endpoint.Endpoint, requestBody []byte, requestID string, startTime time.Time, path string, taggedRequest *tagging.TaggedRequest, attemptNumber int) (success, shouldRetry bool) {
	success, _ = s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime, taggedRequest, attemptNumber)
	if success {
		s.endpointManager.RecordRequest(ep.ID, true)
		
		// 尝试提取基准信息用于健康检查
		if len(requestBody) > 0 {
			extracted := s.healthChecker.GetExtractor().ExtractFromRequest(requestBody, c.Request.Header)
			if extracted {
				s.logger.Info("Successfully updated health check baseline info from request")
			}
		}
		
		s.logger.Debug(fmt.Sprintf("Request succeeded on endpoint %s", ep.Name))
		return true, false
	}
	
	// 记录失败
	s.endpointManager.RecordRequest(ep.ID, false)
	s.logger.Debug("Primary endpoint failed, trying fallback endpoints")
	return false, true
}

// tryEndpointList 尝试端点列表，返回(成功, 尝试次数)
func (s *Server) tryEndpointList(c *gin.Context, endpoints []utils.EndpointSorter, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest, phase string, startingAttemptNumber int) (bool, int) {
	attemptedCount := 0
	
	for _, epInterface := range endpoints {
		ep := epInterface.(*endpoint.Endpoint)
		attemptedCount++
		currentAttempt := startingAttemptNumber + attemptedCount - 1
		s.logger.Debug(fmt.Sprintf("%s: Attempting endpoint %s (attempt #%d)", phase, ep.Name, currentAttempt))
		
		success, shouldRetry := s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime, taggedRequest, currentAttempt)
		if success {
			s.endpointManager.RecordRequest(ep.ID, true)
			s.logger.Debug(fmt.Sprintf("%s: Request succeeded on endpoint %s (attempt #%d)", phase, ep.Name, currentAttempt))
			return true, attemptedCount
		}
		
		s.endpointManager.RecordRequest(ep.ID, false)
		s.logger.Debug(fmt.Sprintf("%s: Request failed on endpoint %s", phase, ep.Name))
		
		if !shouldRetry {
			s.logger.Debug("Endpoint indicated no retry, stopping fallback")
			break
		}
		
		// 重新构建请求体
		s.rebuildRequestBody(c, requestBody)
	}
	
	return false, attemptedCount
}

// filterAndSortEndpoints 过滤并排序端点
func (s *Server) filterAndSortEndpoints(allEndpoints []*endpoint.Endpoint, failedEndpoint *endpoint.Endpoint, filterFunc func(*endpoint.Endpoint) bool) []utils.EndpointSorter {
	var filtered []*endpoint.Endpoint
	
	for _, ep := range allEndpoints {
		// 跳过已失败的endpoint
		if ep.ID == failedEndpoint.ID {
			continue
		}
		// 跳过禁用或不可用的端点
		if !ep.Enabled || !ep.IsAvailable() {
			continue
		}
		
		if filterFunc(ep) {
			filtered = append(filtered, ep)
		}
	}
	
	// 转换为接口类型并排序
	sorter := make([]utils.EndpointSorter, len(filtered))
	for i, ep := range filtered {
		sorter[i] = ep
	}
	utils.SortEndpointsByPriority(sorter)
	
	return sorter
}

// endpointContainsAllTags 检查endpoint的标签是否包含请求的所有标签
func (s *Server) endpointContainsAllTags(endpointTags, requestTags []string) bool {
	if len(requestTags) == 0 {
		return true // 无标签请求总是匹配
	}
	
	// 将endpoint的标签转换为map以便快速查找
	tagSet := make(map[string]bool)
	for _, tag := range endpointTags {
		tagSet[tag] = true
	}
	
	// 检查是否包含所有请求的标签
	for _, reqTag := range requestTags {
		if !tagSet[reqTag] {
			return false
		}
	}
	return true
}

// fallbackToOtherEndpoints 当endpoint失败时，根据是否有tag决定fallback策略
func (s *Server) fallbackToOtherEndpoints(c *gin.Context, path string, requestBody []byte, requestID string, startTime time.Time, failedEndpoint *endpoint.Endpoint, taggedRequest *tagging.TaggedRequest) {
	// 记录失败的endpoint
	s.endpointManager.RecordRequest(failedEndpoint.ID, false)
	
	allEndpoints := s.endpointManager.GetAllEndpoints()
	var requestTags []string
	if taggedRequest != nil {
		requestTags = taggedRequest.Tags
	}
	
	totalAttempted := 1 // 包括最初失败的endpoint
	
	if len(requestTags) > 0 {
		// 有标签请求：分两阶段尝试
		s.logger.Debug(fmt.Sprintf("Tagged request failed on %s, trying fallback with tags: %v", failedEndpoint.Name, requestTags))
		
		// Phase 1：尝试有标签且匹配的端点
		taggedEndpoints := s.filterAndSortEndpoints(allEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
			return len(ep.Tags) > 0 && s.endpointContainsAllTags(ep.Tags, requestTags)
		})
		
		if len(taggedEndpoints) > 0 {
			s.logger.Debug(fmt.Sprintf("Phase 1: Trying %d tagged endpoints", len(taggedEndpoints)))
			success, attemptedCount := s.tryEndpointList(c, taggedEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Phase 1", totalAttempted+1)
			if success {
				return
			}
			totalAttempted += attemptedCount
		}
		
		// Phase 2：尝试万用端点
		universalEndpoints := s.filterAndSortEndpoints(allEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
			return len(ep.Tags) == 0
		})
		
		if len(universalEndpoints) > 0 {
			s.logger.Debug(fmt.Sprintf("Phase 2: Trying %d universal endpoints", len(universalEndpoints)))
			success, attemptedCount := s.tryEndpointList(c, universalEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Phase 2", totalAttempted+1)
			if success {
				return
			}
			totalAttempted += attemptedCount
		}
		
		// 所有endpoint都失败了，发送错误响应但不记录额外日志（每个endpoint的失败已经记录过了）
		s.sendProxyError(c, http.StatusBadGateway, "all_endpoints_failed", fmt.Sprintf("All %d endpoints failed for tagged request (tags: %v)", totalAttempted, requestTags), requestID)
		
	} else {
		// 无标签请求：只尝试万用端点
		s.logger.Debug("Untagged request failed, trying universal endpoints only")
		
		universalEndpoints := s.filterAndSortEndpoints(allEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
			return len(ep.Tags) == 0
		})
		
		if len(universalEndpoints) == 0 {
			s.logger.Error("No universal endpoints available for untagged request", nil)
			s.sendProxyError(c, http.StatusBadGateway, "no_universal_endpoints", "No universal endpoints available", requestID)
			return
		}
		
		s.logger.Debug(fmt.Sprintf("Trying %d universal endpoints for untagged request", len(universalEndpoints)))
		success, attemptedCount := s.tryEndpointList(c, universalEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Universal", totalAttempted+1)
		if success {
			return
		}
		totalAttempted += attemptedCount
		
		// 所有universal endpoint都失败了，发送错误响应但不记录额外日志（每个endpoint的失败已经记录过了）
		s.sendProxyError(c, http.StatusBadGateway, "all_universal_endpoints_failed", fmt.Sprintf("All %d universal endpoints failed for untagged request", totalAttempted), requestID)
	}
}
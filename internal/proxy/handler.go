package proxy

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"claude-code-companion/internal/endpoint"
	"claude-code-companion/internal/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleProxy(c *gin.Context) {
	requestID := c.GetString("request_id")
	startTime := c.MustGet("start_time").(time.Time)
	path := c.Param("path")

	// 读取请求体
	requestBody, err := s.readRequestBody(c)
	if err != nil {
		s.sendProxyError(c, http.StatusBadRequest, "request_body_error", "Failed to read request body", requestID)
		return
	}

	// 提取原始模型名（在任何重写之前）
	originalModel := s.extractModelFromRequest(requestBody)
	// 存储到context中，供后续使用
	c.Set("original_model", originalModel)

	// 提取 thinking 信息
	thinkingInfo, err := utils.ExtractThinkingInfo(string(requestBody))
	if err != nil {
		s.logger.Debug("Failed to extract thinking info", map[string]interface{}{"error": err.Error()})
	}
	// 存储到context中，供后续使用
	c.Set("thinking_info", thinkingInfo)

	// 处理请求标签
	taggedRequest := s.processRequestTags(c.Request)

	// 检查 OpenAI 端点是否收到 count_tokens 请求
	if strings.Contains(path, "/count_tokens") {
		selectedEndpoint, err := s.selectEndpointForRequest(taggedRequest)
		if err == nil && selectedEndpoint.EndpointType == "openai" {
			s.logger.Debug("Rejecting count_tokens request for OpenAI endpoint")
			s.sendProxyError(c, http.StatusNotFound, "unsupported_endpoint", "OpenAI endpoints do not support count_tokens API", requestID)
			return
		}
	}

	// 选择端点并处理请求
	selectedEndpoint, err := s.selectEndpointForRequest(taggedRequest)
	if err != nil {
		s.logger.Error("Failed to select endpoint", err)
		// 获取tags用于日志记录
		var tags []string
		if taggedRequest != nil {
			tags = taggedRequest.Tags
		}
		// 生成详细的错误消息
		errorMsg := s.generateDetailedEndpointUnavailableMessage(requestID, tags)
		s.sendFailureResponse(c, requestID, startTime, requestBody, tags, 0, errorMsg, "no_available_endpoints")
		return
	}

	// 尝试向选择的端点发送请求，失败时回退到其他端点
	success, shouldRetry := s.tryProxyRequest(c, selectedEndpoint, requestBody, requestID, startTime, path, taggedRequest, 1)
	if success {
		return
	}

	if shouldRetry {
		// 使用回退逻辑
		s.fallbackToOtherEndpoints(c, path, requestBody, requestID, startTime, selectedEndpoint, taggedRequest)
	}
}

// generateDetailedEndpointUnavailableMessage 生成详细的端点不可用错误消息
func (s *Server) generateDetailedEndpointUnavailableMessage(requestID string, requestTags []string) string {
	allEndpoints := s.endpointManager.GetAllEndpoints()
	
	if len(requestTags) > 0 {
		// 有tag的请求
		taggedActiveCount := 0
		taggedTotalCount := 0
		universalActiveCount := 0
		universalTotalCount := 0
		
		for _, ep := range allEndpoints {
			if !ep.Enabled {
				continue
			}
			
			if len(ep.Tags) == 0 {
				// 通用端点
				universalTotalCount++
				if ep.IsAvailable() {
					universalActiveCount++
				}
			} else {
				// 检查是否符合tag条件
				if s.endpointMatchesTags(ep, requestTags) {
					taggedTotalCount++
					if ep.IsAvailable() {
						taggedActiveCount++
					}
				}
			}
		}
		
		return fmt.Sprintf("request %s with tag (%s) had failed on %d active out of %d (with tags) and %d active of %d (universal) endpoints", 
			requestID, strings.Join(requestTags, ", "), taggedActiveCount, taggedTotalCount, universalActiveCount, universalTotalCount)
	} else {
		// 无tag的请求
		universalActiveCount := 0
		universalTotalCount := 0
		allEndpointsAreTagged := true
		
		for _, ep := range allEndpoints {
			if !ep.Enabled {
				continue
			}
			
			if len(ep.Tags) == 0 {
				universalTotalCount++
				allEndpointsAreTagged = false
				if ep.IsAvailable() {
					universalActiveCount++
				}
			}
		}
		
		message := fmt.Sprintf("request %s without tag had failed on %d active of %d (universal) endpoints", 
			requestID, universalActiveCount, universalTotalCount)
		
		if allEndpointsAreTagged && universalTotalCount == 0 {
			message += ". All endpoints are tagged but request is not tagged, make sure you understand how tags works"
		}
		
		return message
	}
}

// endpointMatchesTags 检查端点是否匹配所有请求的tags
func (s *Server) endpointMatchesTags(ep *endpoint.Endpoint, requestTags []string) bool {
	if len(requestTags) == 0 {
		return len(ep.Tags) == 0
	}
	
	epTagSet := make(map[string]bool)
	for _, tag := range ep.Tags {
		epTagSet[tag] = true
	}
	
	for _, required := range requestTags {
		if !epTagSet[required] {
			return false
		}
	}
	return true
}


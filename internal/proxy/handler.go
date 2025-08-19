package proxy

import (
	"net/http"
	"strings"
	"time"

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
		s.sendFailureResponse(c, requestID, startTime, requestBody, tags, 0, "All endpoints are currently unavailable", "no_available_endpoints")
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


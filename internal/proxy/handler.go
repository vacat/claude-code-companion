package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/interfaces"
	"claude-proxy/internal/logger"
	"claude-proxy/internal/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleProxy(c *gin.Context) {
	requestID := c.GetString("request_id")
	startTime := c.MustGet("start_time").(time.Time)
	path := c.Param("path")

	var requestBody []byte
	if c.Request.Body != nil {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			s.logger.Error("Failed to read request body", err)
			s.sendProxyError(c, http.StatusBadRequest, "request_body_error", "Failed to read request body", requestID)
			return
		}
		requestBody = body
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}

	// 使用tagging系统处理请求
	taggedRequest, err := s.taggingManager.ProcessRequest(c.Request)
	if err != nil {
		s.logger.Error("Failed to process request tags", err)
		// 如果tagging失败，继续使用原有逻辑（无tag）
		taggedRequest = nil
	}

	// 选择endpoint
	var selectedEndpoint *endpoint.Endpoint
	var endpointSelectionError error

	if taggedRequest != nil && len(taggedRequest.Tags) > 0 {
		// 使用tag匹配选择endpoint
		selectedEndpoint, endpointSelectionError = s.endpointManager.GetEndpointWithTags(taggedRequest.Tags)
		s.logger.Debug(fmt.Sprintf("Request tagged with: %v, selected endpoint: %s", 
			taggedRequest.Tags, 
			func() string { if selectedEndpoint != nil { return selectedEndpoint.Name } else { return "none" } }()))
	} else {
		// 使用原有逻辑选择endpoint
		selectedEndpoint, endpointSelectionError = s.endpointManager.GetEndpoint()
		s.logger.Debug("Request has no tags, using default endpoint selection")
	}

	if endpointSelectionError != nil {
		s.logger.Error("Failed to select endpoint", endpointSelectionError)
		s.sendProxyError(c, http.StatusBadGateway, "no_endpoints", endpointSelectionError.Error(), requestID)
		return
	}

	// 尝试代理到选择的endpoint
	success, _ := s.proxyToEndpoint(c, selectedEndpoint, path, requestBody, requestID, startTime, taggedRequest)
	if success {
		s.endpointManager.RecordRequest(selectedEndpoint.ID, true)
		
		// 尝试提取基准信息用于健康检查
		if len(requestBody) > 0 {
			extracted := s.healthChecker.GetExtractor().ExtractFromRequest(requestBody, c.Request.Header)
			if extracted {
				s.logger.Info("Successfully updated health check baseline info from request")
			}
		}
		
		s.logger.Debug(fmt.Sprintf("Request succeeded on endpoint %s", selectedEndpoint.Name))
		return
	}

	// 如果使用tag选择失败，尝试回退到无tag选择其他endpoint
	if taggedRequest != nil && len(taggedRequest.Tags) > 0 {
		s.logger.Debug("Tagged endpoint failed, trying fallback endpoints")
		s.fallbackToUntaggedEndpoints(c, path, requestBody, requestID, startTime, selectedEndpoint, taggedRequest)
	} else {
		// 记录失败
		s.endpointManager.RecordRequest(selectedEndpoint.ID, false)
		duration := time.Since(startTime)
		requestLog := s.logger.CreateRequestLog(requestID, "failed", c.Request.Method, path)
		requestLog.DurationMs = duration.Nanoseconds() / 1000000
		if len(requestBody) > 0 {
			requestLog.Model = logger.ExtractModelFromRequestBody(string(requestBody))
		}
		requestLog.Error = "Selected endpoint failed"
		s.logger.LogRequest(requestLog)
		s.sendProxyError(c, http.StatusBadGateway, "endpoint_failed", "Selected endpoint failed", requestID)
	}
}

// fallbackToUntaggedEndpoints 当带tag的endpoint失败时，回退到使用原有逻辑尝试其他endpoint
func (s *Server) fallbackToUntaggedEndpoints(c *gin.Context, path string, requestBody []byte, requestID string, startTime time.Time, failedEndpoint *endpoint.Endpoint, taggedRequest *interfaces.TaggedRequest) {
	// 记录失败的endpoint
	s.endpointManager.RecordRequest(failedEndpoint.ID, false)
	
	// 依次尝试每个端点，使用统一的排序逻辑
	allEndpoints := s.endpointManager.GetAllEndpoints()
	
	// 转换为 EndpointSorter 接口类型
	sorterEndpoints := make([]utils.EndpointSorter, len(allEndpoints))
	for i, ep := range allEndpoints {
		sorterEndpoints[i] = ep
	}
	
	// 获取已启用并按优先级排序的端点
	enabledEndpoints := utils.FilterEnabledEndpoints(sorterEndpoints)
	if len(enabledEndpoints) == 0 {
		s.logger.Error("No enabled endpoints for fallback", nil)
		s.sendProxyError(c, http.StatusBadGateway, "no_endpoints", "No enabled endpoints available", requestID)
		return
	}

	utils.SortEndpointsByPriority(enabledEndpoints)

	// 依次尝试每个可用的端点（跳过已经失败的那个）
	attemptedCount := 0
	for i, epInterface := range enabledEndpoints {
		ep := epInterface.(*endpoint.Endpoint)
		
		// 跳过已经失败的endpoint
		if ep.ID == failedEndpoint.ID {
			continue
		}
		
		// 跳过不可用的端点
		if !ep.IsAvailable() {
			s.logger.Debug(fmt.Sprintf("Skipping unavailable fallback endpoint %s", ep.Name))
			continue
		}
		
		attemptedCount++
		s.logger.Debug(fmt.Sprintf("Attempting fallback request to endpoint %s (%d/%d)", ep.Name, i+1, len(enabledEndpoints)))

		success, shouldRetry := s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime, taggedRequest)
		if success {
			s.endpointManager.RecordRequest(ep.ID, true)
			s.logger.Debug(fmt.Sprintf("Fallback request succeeded on endpoint %s", ep.Name))
			return
		}

		// 记录失败
		s.endpointManager.RecordRequest(ep.ID, false)
		s.logger.Debug(fmt.Sprintf("Fallback request failed on endpoint %s, trying next endpoint", ep.Name))

		// 如果不应该重试，则停止尝试其他端点
		if !shouldRetry {
			break
		}

		// 重新构建请求体用于下次尝试
		if c.Request.Body != nil {
			c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
		}
	}

	// 所有回退尝试都失败了
	duration := time.Since(startTime)
	requestLog := s.logger.CreateRequestLog(requestID, "failed", c.Request.Method, path)
	requestLog.DurationMs = duration.Nanoseconds() / 1000000
	if len(requestBody) > 0 {
		requestLog.Model = logger.ExtractModelFromRequestBody(string(requestBody))
	}
	
	// 在日志中记录tag信息
	if taggedRequest != nil && len(taggedRequest.Tags) > 0 {
		requestLog.Error = fmt.Sprintf("Tagged request failed (tags: %v), attempted %d fallback endpoints", taggedRequest.Tags, attemptedCount+1) // +1 包括最初的tagged endpoint
	} else {
		requestLog.Error = fmt.Sprintf("All %d available endpoints failed", attemptedCount+1)
	}
	
	s.logger.LogRequest(requestLog)
	s.sendProxyError(c, http.StatusBadGateway, "all_endpoints_failed", requestLog.Error, requestID)
}

func (s *Server) proxyToEndpoint(c *gin.Context, ep *endpoint.Endpoint, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *interfaces.TaggedRequest) (bool, bool) {
	targetURL := ep.GetFullURL(path)
	
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		s.logger.Error("Failed to create request", err)
		return false, false
	}

	for key, values := range c.Request.Header {
		if key == "Authorization" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 根据认证类型设置不同的认证头部
	if ep.AuthType == "api_key" {
		req.Header.Set("x-api-key", ep.AuthValue)
	} else {
		req.Header.Set("Authorization", ep.GetAuthHeader())
	}

	if c.Request.URL.RawQuery != "" {
		req.URL.RawQuery = c.Request.URL.RawQuery
	}

	client := utils.GetProxyClient()

	resp, err := client.Do(req)
	if err != nil {
		duration := time.Since(startTime)
		s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, nil, nil, duration, err)
		return false, true
	}
	defer resp.Body.Close()

	// 只有2xx状态码才认为是成功，其他所有状态码都尝试下一个端点
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		duration := time.Since(startTime)
		body, _ := io.ReadAll(resp.Body)
		
		// 解压响应体用于日志记录
		contentEncoding := resp.Header.Get("Content-Encoding")
		decompressedBody, err := s.validator.GetDecompressedBody(body, contentEncoding)
		if err != nil {
			decompressedBody = body // 如果解压失败，使用原始数据
		}
		
		s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, decompressedBody, duration, nil)
		s.logger.Debug(fmt.Sprintf("HTTP error %d from endpoint %s, trying next endpoint", resp.StatusCode, ep.Name))
		return false, true
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", err)
		return false, false
	}

	// 解压响应体仅用于日志记录和验证
	contentEncoding := resp.Header.Get("Content-Encoding")
	decompressedBody, err := s.validator.GetDecompressedBody(responseBody, contentEncoding)
	if err != nil {
		s.logger.Error("Failed to decompress response body", err)
		return false, false
	}

	// 检查Content-Type，如果是text/plain但状态码是200，改写Content-Type为application/json
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode == 200 && strings.Contains(strings.ToLower(contentType), "text/plain") {
		s.logger.Info(fmt.Sprintf("Rewriting Content-Type from text/plain to application/json for endpoint %s", ep.Name))
		
		// 复制所有响应头，但修改Content-Type
		for key, values := range resp.Header {
			if strings.ToLower(key) == "content-type" {
				c.Header(key, "application/json")
			} else {
				// 保持原始头部，包括Content-Encoding
				for _, value := range values {
					c.Header(key, value)
				}
			}
		}
	} else {
		// 正常情况下复制所有头部，保持原始状态
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}

	isStreaming := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
	if s.config.Validation.StrictAnthropicFormat {
		if err := s.validator.ValidateAnthropicResponse(decompressedBody, isStreaming); err != nil {
			// 如果是usage统计验证失败，尝试下一个endpoint
			if strings.Contains(err.Error(), "invalid usage stats") {
				s.logger.Info(fmt.Sprintf("Usage validation failed for endpoint %s: %v", ep.Name, err))
				duration := time.Since(startTime)
				errorLog := fmt.Sprintf("Usage validation failed: %v", err)
				s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, append(decompressedBody, []byte(errorLog)...), duration, fmt.Errorf(errorLog))
				return false, true // 验证失败，尝试下一个endpoint
			}
			
			if s.config.Validation.DisconnectOnInvalid {
				s.logger.Error("Invalid response format, disconnecting", err)
				c.Header("Connection", "close")
				c.AbortWithStatus(http.StatusBadGateway)
				return false, false
			}
		}
	}

	c.Status(resp.StatusCode)
	// 发送原始响应体给客户端（保持压缩状态）
	c.Writer.Write(responseBody)

	duration := time.Since(startTime)
	s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, decompressedBody, duration, nil)

	return true, false
}

// truncateBody truncates body content to specified length
func truncateBody(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	return body[:maxLen] + "... [truncated]"
}

// sendProxyError sends a standardized error response for proxy failures
func (s *Server) sendProxyError(c *gin.Context, statusCode int, errorType, message string, requestID string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":       errorType,
			"message":    message,
			"request_id": requestID,
		},
	})
}

// createAndLogRequest creates a request log entry, populates common fields, and logs it
func (s *Server) createAndLogRequest(requestID, endpoint, method, path string, requestBody []byte, req *http.Request, resp *http.Response, responseBody []byte, duration time.Duration, err error) {
	requestLog := s.logger.CreateRequestLog(requestID, endpoint, method, path)
	requestLog.RequestBodySize = len(requestBody)
	
	// 设置请求体日志
	if s.config.Logging.LogRequestBody != "none" && len(requestBody) > 0 {
		if s.config.Logging.LogRequestBody == "truncated" {
			requestLog.RequestBody = truncateBody(string(requestBody), 1024)
		} else {
			requestLog.RequestBody = string(requestBody)
		}
	}
	
	// 提取模型信息
	if len(requestBody) > 0 {
		requestLog.Model = logger.ExtractModelFromRequestBody(string(requestBody))
	}
	
	// 更新并记录日志
	s.logger.UpdateRequestLog(requestLog, req, resp, responseBody, duration, err)
	s.logger.LogRequest(requestLog)
}
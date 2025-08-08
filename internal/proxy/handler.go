package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-proxy/internal/endpoint"
	"claude-proxy/internal/tagging"
	"claude-proxy/internal/utils"

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

	// 处理请求标签
	taggedRequest := s.processRequestTags(c.Request)

	// 选择端点并处理请求
	selectedEndpoint, err := s.selectEndpointForRequest(taggedRequest)
	if err != nil {
		s.logger.Error("Failed to select endpoint", err)
		s.sendProxyError(c, http.StatusBadGateway, "no_available_endpoints", "All endpoints are currently unavailable", requestID)
		return
	}

	// 尝试向选择的端点发送请求，失败时回退到其他端点
	success, shouldRetry := s.tryProxyRequest(c, selectedEndpoint, requestBody, requestID, startTime, path)
	if success {
		return
	}

	if shouldRetry {
		// 使用回退逻辑
		s.fallbackToOtherEndpoints(c, path, requestBody, requestID, startTime, selectedEndpoint, taggedRequest)
	}
}

// readRequestBody reads and buffers the request body
func (s *Server) readRequestBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", err)
		return nil, err
	}
	
	// 重新设置请求体供后续使用
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// processRequestTags handles request tagging with error handling
func (s *Server) processRequestTags(req *http.Request) *tagging.TaggedRequest {
	taggedRequest, err := s.taggingManager.ProcessRequest(req)
	if err != nil {
		s.logger.Error("Failed to process request tags", err)
		return nil
	}
	
	if taggedRequest != nil {
		// 记录详细的tagging结果
		s.logger.Debug(fmt.Sprintf("Tagging completed: found %d tags: %v", len(taggedRequest.Tags), taggedRequest.Tags))
		for _, result := range taggedRequest.TaggerResults {
			if result.Error != nil {
				s.logger.Debug(fmt.Sprintf("Tagger %s failed: %v", result.TaggerName, result.Error))
			} else {
				s.logger.Debug(fmt.Sprintf("Tagger %s: matched=%t, tag=%s, duration=%v", 
					result.TaggerName, result.Matched, result.Tag, result.Duration))
			}
		}
	}
	
	return taggedRequest
}

// selectEndpointForRequest selects the appropriate endpoint based on tags
func (s *Server) selectEndpointForRequest(taggedRequest *tagging.TaggedRequest) (*endpoint.Endpoint, error) {
	if taggedRequest != nil && len(taggedRequest.Tags) > 0 {
		// 使用tag匹配选择endpoint
		selectedEndpoint, err := s.endpointManager.GetEndpointWithTags(taggedRequest.Tags)
		s.logger.Debug(fmt.Sprintf("Request tagged with: %v, selected endpoint: %s", 
			taggedRequest.Tags, 
			func() string { if selectedEndpoint != nil { return selectedEndpoint.Name } else { return "none" } }()))
		return selectedEndpoint, err
	} else {
		// 使用原有逻辑选择endpoint
		selectedEndpoint, err := s.endpointManager.GetEndpoint()
		s.logger.Debug("Request has no tags, using default endpoint selection")
		return selectedEndpoint, err
	}
}

// tryProxyRequest attempts to proxy the request to the given endpoint
func (s *Server) tryProxyRequest(c *gin.Context, ep *endpoint.Endpoint, requestBody []byte, requestID string, startTime time.Time, path string) (success, shouldRetry bool) {
	success, _ = s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime, nil)
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

// rebuildRequestBody rebuilds the request body from the cached bytes
func (s *Server) rebuildRequestBody(c *gin.Context, requestBody []byte) {
	if c.Request.Body != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
}

// tryEndpointList 尝试端点列表，返回(成功, 尝试次数)
func (s *Server) tryEndpointList(c *gin.Context, endpoints []utils.EndpointSorter, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest, phase string) (bool, int) {
	attemptedCount := 0
	
	for _, epInterface := range endpoints {
		ep := epInterface.(*endpoint.Endpoint)
		attemptedCount++
		s.logger.Debug(fmt.Sprintf("%s: Attempting endpoint %s", phase, ep.Name))
		
		success, shouldRetry := s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime, taggedRequest)
		if success {
			s.endpointManager.RecordRequest(ep.ID, true)
			s.logger.Debug(fmt.Sprintf("%s: Request succeeded on endpoint %s", phase, ep.Name))
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
			success, attemptedCount := s.tryEndpointList(c, taggedEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Phase 1")
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
			success, attemptedCount := s.tryEndpointList(c, universalEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Phase 2")
			if success {
				return
			}
			totalAttempted += attemptedCount
		}
		
		// 所有endpoint都失败了
		s.sendFailureResponse(c, requestID, startTime, requestBody, requestTags, totalAttempted, fmt.Sprintf("All %d endpoints failed for tagged request (tags: %v)", totalAttempted, requestTags), "all_endpoints_failed")
		
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
		success, attemptedCount := s.tryEndpointList(c, universalEndpoints, path, requestBody, requestID, startTime, taggedRequest, "Universal")
		if success {
			return
		}
		totalAttempted += attemptedCount
		
		// 所有universal endpoint都失败了
		s.sendFailureResponse(c, requestID, startTime, requestBody, nil, totalAttempted, fmt.Sprintf("All %d universal endpoints failed for untagged request", totalAttempted), "all_universal_endpoints_failed")
	}
}

// sendFailureResponse 发送失败响应
func (s *Server) sendFailureResponse(c *gin.Context, requestID string, startTime time.Time, requestBody []byte, requestTags []string, attemptedCount int, errorMsg, errorType string) {
	duration := time.Since(startTime)
	requestLog := s.logger.CreateRequestLog(requestID, "failed", c.Request.Method, c.Param("path"))
	requestLog.DurationMs = duration.Nanoseconds() / 1000000
	if len(requestBody) > 0 {
		requestLog.Model = utils.ExtractModelFromRequestBody(string(requestBody))
	}
	requestLog.Tags = requestTags
	requestLog.Error = errorMsg
	s.logger.LogRequest(requestLog)
	s.sendProxyError(c, http.StatusBadGateway, errorType, requestLog.Error, requestID)
}

func (s *Server) proxyToEndpoint(c *gin.Context, ep *endpoint.Endpoint, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest) (bool, bool) {
	targetURL := ep.GetFullURL(path)
	
	// Extract tags from taggedRequest
	var tags []string
	if taggedRequest != nil {
		tags = taggedRequest.Tags
	}
	
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		s.logger.Error("Failed to create request", err)
		// 记录创建请求失败的日志
		duration := time.Since(startTime)
		createRequestError := fmt.Sprintf("Failed to create request: %v", err)
		s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, nil, nil, nil, duration, fmt.Errorf(createRequestError), false, tags)
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
		s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, nil, nil, duration, err, s.isRequestExpectingStream(req), tags)
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
		
		s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(req), tags)
		s.logger.Debug(fmt.Sprintf("HTTP error %d from endpoint %s, trying next endpoint", resp.StatusCode, ep.Name))
		return false, true
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", err)
		// 记录读取响应体失败的日志
		duration := time.Since(startTime)
		readError := fmt.Sprintf("Failed to read response body: %v", err)
		s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, nil, duration, fmt.Errorf(readError), s.isRequestExpectingStream(req), tags)
		return false, false
	}

	// 解压响应体仅用于日志记录和验证
	contentEncoding := resp.Header.Get("Content-Encoding")
	decompressedBody, err := s.validator.GetDecompressedBody(responseBody, contentEncoding)
	if err != nil {
		s.logger.Error("Failed to decompress response body", err)
		// 记录解压响应体失败的日志
		duration := time.Since(startTime)
		decompressError := fmt.Sprintf("Failed to decompress response body: %v", err)
		s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, responseBody, duration, fmt.Errorf(decompressError), s.isRequestExpectingStream(req), tags)
		return false, false
	}

	// 检测流式响应：首先检查 Content-Type，然后检查响应体内容
	isStreaming := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
	contentTypeOverride := ""
	
	// Hack 1: 如果 Content-Type 不是 SSE，但响应体看起来像 SSE 格式，则也当作流式处理
	if !isStreaming && len(decompressedBody) > 0 {
		bodyStr := string(decompressedBody)
		if strings.Contains(bodyStr, "event: ") && strings.Contains(bodyStr, "data: ") {
			s.logger.Info(fmt.Sprintf("Detected SSE response with incorrect Content-Type from endpoint %s", ep.Name))
			isStreaming = true
			contentTypeOverride = "text/event-stream"
		}
	}
	
	// Hack 2: 如果 Content-Type 是 text/event-stream 但响应体是 JSON 格式，则改为 application/json
	if isStreaming && len(decompressedBody) > 0 {
		if s.validator.DetectJSONContent(decompressedBody) {
			s.logger.Info(fmt.Sprintf("Detected JSON response with text/event-stream Content-Type from endpoint %s, overriding to application/json", ep.Name))
			isStreaming = false
			contentTypeOverride = "application/json"
		}
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
		// 正常情况下复制所有头部，但检查是否需要修正 Content-Type
		for key, values := range resp.Header {
			if strings.ToLower(key) == "content-type" && contentTypeOverride != "" {
				c.Header(key, contentTypeOverride)
			} else {
				for _, value := range values {
					c.Header(key, value)
				}
			}
		}
	}
	if s.config.Validation.StrictAnthropicFormat {
		if err := s.validator.ValidateAnthropicResponse(decompressedBody, isStreaming); err != nil {
			// 如果是usage统计验证失败，尝试下一个endpoint
			if strings.Contains(err.Error(), "invalid usage stats") {
				s.logger.Info(fmt.Sprintf("Usage validation failed for endpoint %s: %v", ep.Name, err))
				duration := time.Since(startTime)
				errorLog := fmt.Sprintf("Usage validation failed: %v", err)
				s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, append(decompressedBody, []byte(errorLog)...), duration, fmt.Errorf(errorLog), s.isRequestExpectingStream(req), tags)
				return false, true // 验证失败，尝试下一个endpoint
			}
			
			if s.config.Validation.DisconnectOnInvalid {
				s.logger.Error("Invalid response format, disconnecting", err)
				// 记录验证失败的请求日志
				duration := time.Since(startTime)
				validationError := fmt.Sprintf("Response validation failed: %v", err)
				s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, decompressedBody, duration, fmt.Errorf(validationError), isStreaming, tags)
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
	s.createAndLogRequest(requestID, ep.URL, c.Request.Method, path, requestBody, req, resp, decompressedBody, duration, nil, isStreaming, tags)

	return true, false
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

// isRequestExpectingStream 检查请求是否期望流式响应
func (s *Server) isRequestExpectingStream(req *http.Request) bool {
	if req == nil {
		return false
	}
	accept := req.Header.Get("Accept")
	return accept == "text/event-stream" || strings.Contains(accept, "text/event-stream")
}

// createAndLogRequest creates a request log entry, populates common fields, and logs it
func (s *Server) createAndLogRequest(requestID, endpoint, method, path string, requestBody []byte, req *http.Request, resp *http.Response, responseBody []byte, duration time.Duration, err error, isStreaming bool, tags []string) {
	requestLog := s.logger.CreateRequestLog(requestID, endpoint, method, path)
	requestLog.RequestBodySize = len(requestBody)
	requestLog.Tags = tags
	
	// 设置请求体日志
	if s.config.Logging.LogRequestBody != "none" && len(requestBody) > 0 {
		if s.config.Logging.LogRequestBody == "truncated" {
			requestLog.RequestBody = utils.TruncateBody(string(requestBody), 1024)
		} else {
			requestLog.RequestBody = string(requestBody)
		}
	}
	
	// 提取模型信息
	if len(requestBody) > 0 {
		requestLog.Model = utils.ExtractModelFromRequestBody(string(requestBody))
	}
	
	// 更新并记录日志
	s.logger.UpdateRequestLog(requestLog, req, resp, responseBody, duration, err)
	
	// 覆盖流式检测结果
	requestLog.IsStreaming = isStreaming
	
	s.logger.LogRequest(requestLog)
}
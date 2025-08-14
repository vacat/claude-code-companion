package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-proxy/internal/conversion"
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

	// 提取原始模型名（在任何重写之前）
	originalModel := s.extractModelFromRequest(requestBody)
	// 存储到context中，供后续使用
	c.Set("original_model", originalModel)

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

// rebuildRequestBody rebuilds the request body from the cached bytes
func (s *Server) rebuildRequestBody(c *gin.Context, requestBody []byte) {
	if c.Request.Body != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
	}
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

// sendFailureResponse 发送失败响应
func (s *Server) sendFailureResponse(c *gin.Context, requestID string, startTime time.Time, requestBody []byte, requestTags []string, attemptedCount int, errorMsg, errorType string) {
	duration := time.Since(startTime)
	requestLog := s.logger.CreateRequestLog(requestID, "failed", c.Request.Method, c.Param("path"))
	requestLog.DurationMs = duration.Nanoseconds() / 1000000
	requestLog.StatusCode = http.StatusBadGateway
	
	// 记录请求头信息
	if c.Request != nil {
		requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
		requestLog.RequestHeaders = requestLog.OriginalRequestHeaders
		
		// 记录请求URL
		requestLog.OriginalRequestURL = c.Request.URL.String()
	}
	
	// 记录请求体信息
	if len(requestBody) > 0 {
		requestLog.Model = utils.ExtractModelFromRequestBody(string(requestBody))
		requestLog.RequestBodySize = len(requestBody)
		
		// 根据配置记录请求体内容
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.OriginalRequestBody = utils.TruncateBody(string(requestBody), 1024)
			} else {
				requestLog.OriginalRequestBody = string(requestBody)
			}
			// 同时设置RequestBody字段用于向后兼容
			requestLog.RequestBody = requestLog.OriginalRequestBody
		}
	}
	
	requestLog.Tags = requestTags
	requestLog.Error = errorMsg
	s.logger.LogRequest(requestLog)
	s.sendProxyError(c, http.StatusBadGateway, errorType, requestLog.Error, requestID)
}

// extractModelFromRequest extracts the model name from the request body
func (s *Server) extractModelFromRequest(requestBody []byte) string {
	if len(requestBody) == 0 {
		return ""
	}
	return utils.ExtractModelFromRequestBody(string(requestBody))
}

func (s *Server) proxyToEndpoint(c *gin.Context, ep *endpoint.Endpoint, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest, attemptNumber int) (bool, bool) {
	// 为这个端点记录独立的开始时间
	endpointStartTime := time.Now()
	targetURL := ep.GetFullURL(path)
	
	// Extract tags from taggedRequest
	var tags []string
	if taggedRequest != nil {
		tags = taggedRequest.Tags
	}
	
	// 创建HTTP请求用于模型重写处理
	tempReq, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(requestBody))
	if err != nil {
		s.logger.Error("Failed to create request", err)
		// 记录创建请求失败的日志
		duration := time.Since(endpointStartTime)
		createRequestError := fmt.Sprintf("Failed to create request: %v", err)
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, requestBody, c, nil, nil, nil, duration, fmt.Errorf(createRequestError), false, tags, "", "", "", attemptNumber)
		return false, false
	}

	// 应用模型重写（如果配置了）
	originalModel, rewrittenModel, err := s.modelRewriter.RewriteRequest(tempReq, ep.ModelRewrite)
	if err != nil {
		s.logger.Error("Model rewrite failed", err)
		// 记录模型重写失败的日志
		duration := time.Since(endpointStartTime)
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, requestBody, c, nil, nil, nil, duration, err, false, tags, "", "", "", attemptNumber)
		return false, false
	}

	// 如果进行了模型重写，获取重写后的请求体
	var finalRequestBody []byte
	if originalModel != "" && rewrittenModel != "" {
		finalRequestBody, err = io.ReadAll(tempReq.Body)
		if err != nil {
			s.logger.Error("Failed to read rewritten request body", err)
			duration := time.Since(endpointStartTime)
			s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, nil, nil, nil, duration, err, false, tags, "", originalModel, rewrittenModel, attemptNumber)
			return false, false
		}
	} else {
		finalRequestBody = requestBody // 使用原始请求体
	}

	// 格式转换（在模型重写之后）
	var conversionContext *conversion.ConversionContext
	if s.converter.ShouldConvert(ep.EndpointType) {
		s.logger.Info(fmt.Sprintf("Starting request conversion for endpoint type: %s", ep.EndpointType))
		convertedBody, ctx, err := s.converter.ConvertRequest(finalRequestBody, ep.EndpointType)
		if err != nil {
			s.logger.Error("Request format conversion failed", err)
			duration := time.Since(endpointStartTime)
			s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, nil, nil, nil, duration, err, false, tags, "", originalModel, rewrittenModel, attemptNumber)
			return false, true // 转换失败，尝试下一个端点
		}
		finalRequestBody = convertedBody
		conversionContext = ctx
		s.logger.Debug("Request format converted successfully", map[string]interface{}{
			"endpoint_type": ep.EndpointType,
			"original_size": len(requestBody),
			"converted_size": len(convertedBody),
		})
	}

	// 创建最终的HTTP请求
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(finalRequestBody))
	if err != nil {
		s.logger.Error("Failed to create final request", err)
		// 记录创建请求失败的日志
		duration := time.Since(endpointStartTime)
		createRequestError := fmt.Sprintf("Failed to create final request: %v", err)
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, nil, nil, nil, duration, fmt.Errorf(createRequestError), false, tags, "", originalModel, rewrittenModel, attemptNumber)
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

	// 为这个端点创建支持代理的HTTP客户端
	client, err := ep.CreateProxyClient(s.config.Timeouts.Proxy)
	if err != nil {
		s.logger.Error("Failed to create proxy client for endpoint", err)
		duration := time.Since(endpointStartTime)
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, req, nil, nil, duration, err, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		return false, true
	}

	resp, err := client.Do(req)
	if err != nil {
		duration := time.Since(endpointStartTime)
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, req, nil, nil, duration, err, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		return false, true
	}
	defer resp.Body.Close()

	// 只有2xx状态码才认为是成功，其他所有状态码都尝试下一个端点
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		duration := time.Since(endpointStartTime)
		body, _ := io.ReadAll(resp.Body)
		
		// 解压响应体用于日志记录
		contentEncoding := resp.Header.Get("Content-Encoding")
		decompressedBody, err := s.validator.GetDecompressedBody(body, contentEncoding)
		if err != nil {
			decompressedBody = body // 如果解压失败，使用原始数据
		}
		
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, nil, s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		s.logger.Debug(fmt.Sprintf("HTTP error %d from endpoint %s, trying next endpoint", resp.StatusCode, ep.Name))
		return false, true
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", err)
		// 记录读取响应体失败的日志
		duration := time.Since(endpointStartTime)
		readError := fmt.Sprintf("Failed to read response body: %v", err)
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, nil, duration, fmt.Errorf(readError), s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		return false, false
	}

	// 解压响应体仅用于日志记录和验证
	contentEncoding := resp.Header.Get("Content-Encoding")
	decompressedBody, err := s.validator.GetDecompressedBody(responseBody, contentEncoding)
	if err != nil {
		s.logger.Error("Failed to decompress response body", err)
		// 记录解压响应体失败的日志
		duration := time.Since(endpointStartTime)
		decompressError := fmt.Sprintf("Failed to decompress response body: %v", err)
		s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, responseBody, duration, fmt.Errorf(decompressError), s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
		return false, false
	}

	// 智能检测内容类型并自动覆盖
	currentContentType := resp.Header.Get("Content-Type")
	newContentType, overrideInfo := s.validator.SmartDetectContentType(decompressedBody, currentContentType, resp.StatusCode)
	
	// 确定最终的Content-Type和是否为流式响应
	finalContentType := currentContentType
	if newContentType != "" {
		finalContentType = newContentType
		s.logger.Info(fmt.Sprintf("Auto-detected content type mismatch for endpoint %s: %s", ep.Name, overrideInfo))
	}
	
	// 判断是否为流式响应（基于最终的Content-Type）
	isStreaming := strings.Contains(strings.ToLower(finalContentType), "text/event-stream")

	// 复制响应头，但跳过可能需要重新计算的头部
	for key, values := range resp.Header {
		keyLower := strings.ToLower(key)
		if keyLower == "content-type" && newContentType != "" {
			c.Header(key, finalContentType)
		} else if keyLower == "content-length" || keyLower == "content-encoding" {
			// 这些头部会在后面根据最终响应体重新设置
			continue
		} else {
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}
	if s.config.Validation.StrictAnthropicFormat {
		if err := s.validator.ValidateResponseWithPath(decompressedBody, isStreaming, ep.EndpointType, path); err != nil {
			// 如果是usage统计验证失败，尝试下一个endpoint
			if strings.Contains(err.Error(), "invalid usage stats") {
				s.logger.Info(fmt.Sprintf("Usage validation failed for endpoint %s: %v", ep.Name, err))
				duration := time.Since(endpointStartTime)
				errorLog := fmt.Sprintf("Usage validation failed: %v", err)
				s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, append(decompressedBody, []byte(errorLog)...), duration, fmt.Errorf(errorLog), s.isRequestExpectingStream(req), tags, "", originalModel, rewrittenModel, attemptNumber)
				return false, true // 验证失败，尝试下一个endpoint
			}
			
			if s.config.Validation.DisconnectOnInvalid {
				s.logger.Error("Invalid response format, disconnecting", err)
				// 记录验证失败的请求日志
				duration := time.Since(endpointStartTime)
				validationError := fmt.Sprintf("Response validation failed: %v", err)
				s.logSimpleRequest(requestID, ep.URL, c.Request.Method, path, requestBody, finalRequestBody, c, req, resp, decompressedBody, duration, fmt.Errorf(validationError), isStreaming, tags, "", originalModel, rewrittenModel, attemptNumber)
				c.Header("Connection", "close")
				c.AbortWithStatus(http.StatusBadGateway)
				return false, false
			}
		}
	}

	c.Status(resp.StatusCode)
	
	// 格式转换（在模型重写之前）
	convertedResponseBody := decompressedBody
	if conversionContext != nil {
		s.logger.Info(fmt.Sprintf("Starting response conversion. Streaming: %v, OriginalSize: %d", isStreaming, len(decompressedBody)))
		convertedResp, err := s.converter.ConvertResponse(decompressedBody, conversionContext, isStreaming)
		if err != nil {
			s.logger.Error("Response format conversion failed", err)
			// 转换失败，使用原始响应体
		} else {
			convertedResponseBody = convertedResp
			s.logger.Info(fmt.Sprintf("Response conversion successful! Original: %d bytes -> Converted: %d bytes", len(decompressedBody), len(convertedResp)))
			s.logger.Debug("Response format converted successfully", map[string]interface{}{
				"endpoint_type": conversionContext.EndpointType,
				"original_size": len(decompressedBody),
				"converted_size": len(convertedResp),
			})
		}
	}
	
	// 应用响应模型重写（如果进行了请求模型重写）
	finalResponseBody := convertedResponseBody
	if originalModel != "" && rewrittenModel != "" {
		rewrittenResponseBody, err := s.modelRewriter.RewriteResponse(convertedResponseBody, originalModel, rewrittenModel)
		if err != nil {
			s.logger.Error("Failed to rewrite response model", err)
			// 如果响应重写失败，使用转换后的响应体，不中断请求
		} else if len(rewrittenResponseBody) > 0 && !bytes.Equal(rewrittenResponseBody, convertedResponseBody) {
			// 如果响应重写成功且内容发生了变化，发送重写后的未压缩响应
			// 并移除Content-Encoding头（因为我们发送的是未压缩数据）
			c.Header("Content-Encoding", "")
			c.Header("Content-Length", fmt.Sprintf("%d", len(rewrittenResponseBody)))
			finalResponseBody = rewrittenResponseBody
		} else {
			// 如果没有重写或重写后内容没变化，使用转换后的响应体
			finalResponseBody = convertedResponseBody
		}
	} else if conversionContext != nil {
		// 只有格式转换没有模型重写的情况
		finalResponseBody = convertedResponseBody
	}
	
	// 设置正确的响应头部
	if conversionContext != nil || (originalModel != "" && rewrittenModel != "") {
		// 如果进行了转换或模型重写，需要重新设置头部
		// 移除压缩编码（因为我们发送的是解压后的数据）
		c.Header("Content-Encoding", "")
		// 设置正确的内容长度
		c.Header("Content-Length", fmt.Sprintf("%d", len(finalResponseBody)))
	}
	
	// 如果是流式响应，确保设置正确的SSE头部
	if isStreaming {
		c.Header("Content-Type", "text/event-stream; charset=utf-8")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no") // 防止中间层缓冲
		// 移除Content-Length头部（SSE不应该设置这个）
		c.Header("Content-Length", "")
	}
	
	// 发送最终响应体给客户端
	c.Writer.Write(finalResponseBody)

	duration := time.Since(endpointStartTime)
	// 创建日志条目，记录修改前后的完整数据
	requestLog := s.logger.CreateRequestLog(requestID, ep.URL, c.Request.Method, path)
	requestLog.RequestBodySize = len(requestBody)
	requestLog.Tags = tags
	requestLog.ContentTypeOverride = overrideInfo
	requestLog.AttemptNumber = attemptNumber
	
	// 记录原始客户端请求数据
	requestLog.OriginalRequestURL = c.Request.URL.String()
	requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
	if len(requestBody) > 0 {
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.OriginalRequestBody = utils.TruncateBody(string(requestBody), 1024)
			} else {
				requestLog.OriginalRequestBody = string(requestBody)
			}
		}
	}
	
	// 记录最终发送给上游的请求数据
	requestLog.FinalRequestURL = req.URL.String()
	requestLog.FinalRequestHeaders = utils.HeadersToMap(req.Header)
	if len(finalRequestBody) > 0 {
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.FinalRequestBody = utils.TruncateBody(string(finalRequestBody), 1024)
			} else {
				requestLog.FinalRequestBody = string(finalRequestBody)
			}
		}
	}
	
	// 记录上游原始响应数据
	requestLog.OriginalResponseHeaders = utils.HeadersToMap(resp.Header)
	if len(decompressedBody) > 0 {
		if s.config.Logging.LogResponseBody != "none" {
			if s.config.Logging.LogResponseBody == "truncated" {
				requestLog.OriginalResponseBody = utils.TruncateBody(string(decompressedBody), 1024)
			} else {
				requestLog.OriginalResponseBody = string(decompressedBody)
			}
		}
	}
	
	// 记录最终发送给客户端的响应数据
	finalHeaders := make(map[string]string)
	for key := range resp.Header {
		values := c.Writer.Header().Values(key)
		if len(values) > 0 {
			finalHeaders[key] = values[0]
		}
	}
	requestLog.FinalResponseHeaders = finalHeaders
	if len(finalResponseBody) > 0 {
		if s.config.Logging.LogResponseBody != "none" {
			if s.config.Logging.LogResponseBody == "truncated" {
				requestLog.FinalResponseBody = utils.TruncateBody(string(finalResponseBody), 1024)
			} else {
				requestLog.FinalResponseBody = string(finalResponseBody)
			}
		}
	}
	
	// 设置兼容性字段
	requestLog.RequestHeaders = requestLog.FinalRequestHeaders
	requestLog.RequestBody = requestLog.OriginalRequestBody
	requestLog.ResponseHeaders = requestLog.OriginalResponseHeaders
	requestLog.ResponseBody = requestLog.OriginalResponseBody
	
	// 设置模型信息
	if len(requestBody) > 0 {
		extractedModel := utils.ExtractModelFromRequestBody(string(requestBody))
		if originalModel != "" {
			requestLog.Model = originalModel
			requestLog.OriginalModel = originalModel
		} else {
			requestLog.Model = extractedModel
			requestLog.OriginalModel = extractedModel
		}
		
		if rewrittenModel != "" {
			requestLog.RewrittenModel = rewrittenModel
			requestLog.ModelRewriteApplied = rewrittenModel != requestLog.OriginalModel
		}
	}
	
	// 更新基本字段
	s.logger.UpdateRequestLog(requestLog, req, resp, decompressedBody, duration, nil)
	requestLog.IsStreaming = isStreaming
	s.logger.LogRequest(requestLog)

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

// logSimpleRequest creates and logs a simple request log entry for error cases
func (s *Server) logSimpleRequest(requestID, endpoint, method, path string, originalRequestBody []byte, finalRequestBody []byte, c *gin.Context, req *http.Request, resp *http.Response, responseBody []byte, duration time.Duration, err error, isStreaming bool, tags []string, contentTypeOverride string, originalModel, rewrittenModel string, attemptNumber int) {
	requestLog := s.logger.CreateRequestLog(requestID, endpoint, method, path)
	requestLog.RequestBodySize = len(originalRequestBody)
	requestLog.Tags = tags
	requestLog.ContentTypeOverride = contentTypeOverride
	requestLog.AttemptNumber = attemptNumber
	
	// 记录原始客户端请求数据
	if c != nil {
		requestLog.OriginalRequestURL = c.Request.URL.String()
		requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
	}
	
	if len(originalRequestBody) > 0 {
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.OriginalRequestBody = utils.TruncateBody(string(originalRequestBody), 1024)
				requestLog.RequestBody = requestLog.OriginalRequestBody
			} else {
				requestLog.OriginalRequestBody = string(originalRequestBody)
				requestLog.RequestBody = requestLog.OriginalRequestBody
			}
		}
	}
	
	// 记录最终请求体（如果不同于原始请求体）
	if len(finalRequestBody) > 0 && !bytes.Equal(originalRequestBody, finalRequestBody) {
		if s.config.Logging.LogRequestBody != "none" {
			if s.config.Logging.LogRequestBody == "truncated" {
				requestLog.FinalRequestBody = utils.TruncateBody(string(finalRequestBody), 1024)
			} else {
				requestLog.FinalRequestBody = string(finalRequestBody)
			}
		}
	}
	
	// 设置最终请求数据（发送给上游的数据）
	if req != nil {
		requestLog.FinalRequestURL = req.URL.String()
		requestLog.FinalRequestHeaders = utils.HeadersToMap(req.Header)
		requestLog.RequestHeaders = requestLog.FinalRequestHeaders
		
		// 尝试读取最终请求体（如果有的话）
		if req.Body != nil {
			if finalBody, err := io.ReadAll(req.Body); err == nil && len(finalBody) > 0 {
				// 重新设置请求体供后续使用
				req.Body = io.NopCloser(bytes.NewReader(finalBody))
				
				if s.config.Logging.LogRequestBody != "none" {
					if s.config.Logging.LogRequestBody == "truncated" {
						requestLog.FinalRequestBody = utils.TruncateBody(string(finalBody), 1024)
					} else {
						requestLog.FinalRequestBody = string(finalBody)
					}
				}
			}
		}
	} else if c != nil {
		// 如果没有最终请求，使用原始请求数据作为兼容
		requestLog.RequestHeaders = requestLog.OriginalRequestHeaders
	}
	
	// 设置响应数据
	if resp != nil {
		requestLog.OriginalResponseHeaders = utils.HeadersToMap(resp.Header)
		requestLog.ResponseHeaders = requestLog.OriginalResponseHeaders
		if len(responseBody) > 0 {
			if s.config.Logging.LogResponseBody != "none" {
				if s.config.Logging.LogResponseBody == "truncated" {
					requestLog.OriginalResponseBody = utils.TruncateBody(string(responseBody), 1024)
					requestLog.ResponseBody = requestLog.OriginalResponseBody
				} else {
					requestLog.OriginalResponseBody = string(responseBody)
					requestLog.ResponseBody = requestLog.OriginalResponseBody
				}
			}
		}
	}
	
	// 设置模型信息
	if len(originalRequestBody) > 0 {
		extractedModel := utils.ExtractModelFromRequestBody(string(originalRequestBody))
		if originalModel != "" {
			requestLog.Model = originalModel
			requestLog.OriginalModel = originalModel
		} else {
			requestLog.Model = extractedModel
			requestLog.OriginalModel = extractedModel
		}
		
		if rewrittenModel != "" {
			requestLog.RewrittenModel = rewrittenModel
			requestLog.ModelRewriteApplied = rewrittenModel != requestLog.OriginalModel
		}
	}
	
	// 更新并记录日志
	s.logger.UpdateRequestLog(requestLog, req, resp, responseBody, duration, err)
	requestLog.IsStreaming = isStreaming
	s.logger.LogRequest(requestLog)
}


package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"claude-proxy/internal/endpoint"
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
		s.logger.Error("No enabled endpoints", nil)
		s.sendProxyError(c, http.StatusBadGateway, "no_endpoints", "No enabled endpoints available", requestID)
		return
	}

	utils.SortEndpointsByPriority(enabledEndpoints)

	// 依次尝试每个可用的端点
	attemptedCount := 0
	for i, epInterface := range enabledEndpoints {
		ep := epInterface.(*endpoint.Endpoint) // 类型断言转换回 *Endpoint
		// 跳过不可用的端点
		if !ep.IsAvailable() {
			s.logger.Debug(fmt.Sprintf("Skipping unavailable endpoint %s", ep.Name))
			continue
		}
		
		attemptedCount++
		s.logger.Debug(fmt.Sprintf("Attempting request to endpoint %s (%d/%d)", ep.Name, i+1, len(enabledEndpoints)))

		success, shouldRetry := s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime)
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
			return
		}

		// 记录失败
		s.endpointManager.RecordRequest(ep.ID, false)
		s.logger.Debug(fmt.Sprintf("Request failed on endpoint %s, trying next endpoint", ep.Name))

		// 如果不应该重试，则停止尝试其他端点
		if !shouldRetry {
			break
		}

		// 重新构建请求体用于下次尝试
		if c.Request.Body != nil {
			c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
		}
	}

	duration := time.Since(startTime)
	requestLog := s.logger.CreateRequestLog(requestID, "failed", c.Request.Method, path)
	requestLog.DurationMs = duration.Nanoseconds() / 1000000
	if len(requestBody) > 0 {
		requestLog.Model = logger.ExtractModelFromRequestBody(string(requestBody))
	}
	
	if attemptedCount == 0 {
		requestLog.Error = "No available endpoints found"
		s.logger.LogRequest(requestLog)
		s.logger.Error("No available endpoints", nil)
		s.sendProxyError(c, http.StatusBadGateway, "no_available_endpoints", "No available endpoints found", requestID)
	} else {
		requestLog.Error = fmt.Sprintf("All %d available endpoints failed", attemptedCount)
		s.logger.LogRequest(requestLog)
		s.logger.Error(fmt.Sprintf("All %d available endpoints failed", attemptedCount), nil)
		s.sendProxyError(c, http.StatusBadGateway, "all_endpoints_failed", fmt.Sprintf("All %d available endpoints failed", attemptedCount), requestID)
	}
}

func (s *Server) proxyToEndpoint(c *gin.Context, ep *endpoint.Endpoint, path string, requestBody []byte, requestID string, startTime time.Time) (bool, bool) {
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
		
		// 解压响应体用于记录
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
			} else if key == "Content-Encoding" || key == "Content-Length" {
				continue // 跳过这些头部
			} else {
				for _, value := range values {
					c.Header(key, value)
				}
			}
		}
	} else {
		// 正常情况下复制所有头部
		for key, values := range resp.Header {
			if key == "Content-Encoding" || key == "Content-Length" {
				continue
			}
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}

	isStreaming := strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
	if s.config.Validation.StrictAnthropicFormat {
		if err := s.validator.ValidateAnthropicResponse(decompressedBody, isStreaming); err != nil {
			if s.config.Validation.DisconnectOnInvalid {
				s.logger.Error("Invalid response format, disconnecting", err)
				c.Header("Connection", "close")
				c.AbortWithStatus(http.StatusBadGateway)
				return false, false
			}
		}
	}

	c.Status(resp.StatusCode)
	c.Writer.Write(decompressedBody)

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
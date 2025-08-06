package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"claude-proxy/internal/endpoint"

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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		requestBody = body
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}

	// 获取所有启用的端点，按优先级排序
	allEndpoints := s.endpointManager.GetAllEndpoints()
	enabledEndpoints := make([]*endpoint.Endpoint, 0)
	for _, ep := range allEndpoints {
		if ep.Enabled {
			enabledEndpoints = append(enabledEndpoints, ep)
		}
	}

	if len(enabledEndpoints) == 0 {
		s.logger.Error("No enabled endpoints", nil)
		c.JSON(http.StatusBadGateway, gin.H{"error": "No enabled endpoints available"})
		return
	}

	// 按优先级排序
	sort.Slice(enabledEndpoints, func(i, j int) bool {
		return enabledEndpoints[i].Priority < enabledEndpoints[j].Priority
	})

	// 依次尝试每个端点，失败后直接尝试下一个
	for i, ep := range enabledEndpoints {
		s.logger.Debug(fmt.Sprintf("Attempting request to endpoint %s (%d/%d)", ep.Name, i+1, len(enabledEndpoints)))

		success, shouldRetry := s.proxyToEndpoint(c, ep, path, requestBody, requestID, startTime)
		if success {
			s.endpointManager.RecordRequest(ep.ID, true)
			s.logger.Debug(fmt.Sprintf("Request succeeded on endpoint %s", ep.Name))
			return
		}

		// 记录失败，但不改变端点状态
		s.endpointManager.RecordRequest(ep.ID, false)
		s.logger.Debug(fmt.Sprintf("Request failed on endpoint %s, trying next endpoint", ep.Name))

		// 如果不应该重试，或者这是最后一个端点，则停止
		if !shouldRetry || i == len(enabledEndpoints)-1 {
			break
		}

		// 重新构建请求体用于下次尝试
		if c.Request.Body != nil {
			c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
		}
	}

	duration := time.Since(startTime)
	requestLog := s.logger.CreateRequestLog(requestID, "failed", c.Request.Method, path)
	requestLog.Error = fmt.Sprintf("All %d endpoints failed", len(enabledEndpoints))
	requestLog.DurationMs = duration.Nanoseconds() / 1000000
	s.logger.LogRequest(requestLog)

	c.JSON(http.StatusBadGateway, gin.H{"error": "All endpoints failed"})
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
		req.Header.Set("x-api-key", ep.GetAuthHeader())
	} else {
		req.Header.Set("Authorization", ep.GetAuthHeader())
	}

	if c.Request.URL.RawQuery != "" {
		req.URL.RawQuery = c.Request.URL.RawQuery
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,  // TLS握手超时
			ResponseHeaderTimeout: 60 * time.Second,  // 等待响应头超时（足够长等待LLM开始响应）
			IdleConnTimeout:       90 * time.Second,  // 空闲连接超时
		},
		// 不设置Client.Timeout，允许长时间的流式响应
	}

	resp, err := client.Do(req)
	if err != nil {
		duration := time.Since(startTime)
		requestLog := s.logger.CreateRequestLog(requestID, ep.URL, c.Request.Method, path)
		requestLog.RequestBodySize = len(requestBody)
		if s.config.Logging.LogRequestBody && len(requestBody) > 0 {
			requestLog.RequestBody = string(requestBody)
		}
		s.logger.UpdateRequestLog(requestLog, req, nil, nil, duration, err)
		s.logger.LogRequest(requestLog)
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
		
		requestLog := s.logger.CreateRequestLog(requestID, ep.URL, c.Request.Method, path)
		requestLog.RequestBodySize = len(requestBody)
		if s.config.Logging.LogRequestBody && len(requestBody) > 0 {
			requestLog.RequestBody = string(requestBody)
		}
		s.logger.UpdateRequestLog(requestLog, req, resp, decompressedBody, duration, nil)
		s.logger.LogRequest(requestLog)
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
	requestLog := s.logger.CreateRequestLog(requestID, ep.URL, c.Request.Method, path)
	requestLog.RequestBodySize = len(requestBody)
	if s.config.Logging.LogRequestBody && len(requestBody) > 0 {
		requestLog.RequestBody = string(requestBody)
	}
	s.logger.UpdateRequestLog(requestLog, req, resp, decompressedBody, duration, nil)
	s.logger.LogRequest(requestLog)

	return true, false
}
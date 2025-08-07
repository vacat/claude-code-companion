package health

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"claude-proxy/internal/endpoint"
)

type Checker struct {
	extractor *RequestExtractor
}

func NewChecker() *Checker {
	return &Checker{
		extractor: NewRequestExtractor(),
	}
}

func (c *Checker) GetExtractor() *RequestExtractor {
	return c.extractor
}

func (c *Checker) CheckEndpoint(ep *endpoint.Endpoint) error {
	requestInfo := c.extractor.GetRequestInfo()
	
	if requestInfo.Extracted {
		fmt.Printf("[DEBUG] Health check for %s: Using extracted request info (model: %s, user: %s)\n", ep.Name, requestInfo.Model, requestInfo.UserID)
	} else {
		fmt.Printf("[DEBUG] Health check for %s: Using default request info (model: %s, user: %s)\n", ep.Name, requestInfo.Model, requestInfo.UserID)
	}
	
	// 构造健康检查请求
	healthCheckRequest := map[string]interface{}{
		"model":       requestInfo.Model,
		"max_tokens":  512,
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "hello",
			},
		},
		"system": []map[string]interface{}{
			{
				"type": "text",
				"text": "Analyze if this message indicates a new conversation topic. If it does, extract a 2-3 word title that captures the new topic. Format your response as a JSON object with two fields: 'isNewTopic' (boolean) and 'title' (string, or null if isNewTopic is false). Only include these fields, no other text.",
			},
		},
		"temperature": 0,
		"metadata": map[string]interface{}{
			"user_id": requestInfo.UserID,
		},
		"stream": true,
	}

	// 将请求序列化为JSON
	requestBody, err := json.Marshal(healthCheckRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal health check request: %v", err)
	}

	// 构造HTTP请求
	targetURL := ep.GetFullURL("/messages")
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create health check request: %v", err)
	}

	// 复制从实际请求中提取的头部（包含默认值）
	for key, value := range requestInfo.Headers {
		req.Header.Set(key, value)
	}

	// 单独设置认证头部（不包含在默认headers中）
	if ep.AuthType == "api_key" {
		req.Header.Set("x-api-key", ep.GetAuthHeader())
	} else {
		req.Header.Set("Authorization", ep.GetAuthHeader())
	}

	// 执行请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	fmt.Printf("[DEBUG] Health check for %s: Sending request to %s\n", ep.Name, targetURL)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[DEBUG] Health check for %s: Request failed: %v\n", ep.Name, err)
		return fmt.Errorf("health check request failed: %v", err)
	}
	defer resp.Body.Close()
	
	fmt.Printf("[DEBUG] Health check for %s: Got response with status %d\n", ep.Name, resp.StatusCode)

	// 检查状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 读取响应体验证是否为有效流式响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read health check response: %v", err)
	}

	// 简单验证：检查是否包含SSE格式的流式响应
	if !bytes.Contains(body, []byte("event:")) && !bytes.Contains(body, []byte("data:")) {
		// 如果不是流式响应，检查是否为有效的JSON响应
		var jsonResp map[string]interface{}
		if err := json.Unmarshal(body, &jsonResp); err != nil {
			return fmt.Errorf("health check response is neither valid SSE nor JSON: %v", err)
		}
		
		// 检查是否包含Anthropic响应的基本字段
		if _, hasContent := jsonResp["content"]; !hasContent {
			if _, hasError := jsonResp["error"]; !hasError {
				return fmt.Errorf("health check response missing required fields")
			}
		}
	}

	fmt.Printf("[DEBUG] Health check for %s: Check completed successfully\n", ep.Name)
	return nil
}
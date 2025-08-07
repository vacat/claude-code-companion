package health

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

type RequestInfo struct {
	Model    string            `json:"model"`
	UserID   string            `json:"user_id"`
	Headers  map[string]string `json:"headers"`
	Extracted bool             `json:"extracted"`
}

type RequestExtractor struct {
	mutex       sync.RWMutex
	requestInfo *RequestInfo
}

func NewRequestExtractor() *RequestExtractor {
	// 默认请求头，不包含认证信息
	defaultHeaders := map[string]string{
		"Accept":                                      "application/json",
		"Accept-Encoding":                             "gzip, deflate",
		"Accept-Language":                             "*",
		"Anthropic-Beta":                              "fine-grained-tool-streaming-2025-05-14",
		"Anthropic-Dangerous-Direct-Browser-Access":   "true",
		"Anthropic-Version":                           "2023-06-01",
		"Connection":                                  "keep-alive",
		"Content-Type":                                "application/json",
		"Sec-Fetch-Mode":                              "cors",
		"User-Agent":                                  "claude-cli/1.0.56 (external, cli)",
		"X-App":                                       "cli",
		"X-Stainless-Arch":                            "x64",
		"X-Stainless-Helper-Method":                   "stream",
		"X-Stainless-Lang":                            "js",
		"X-Stainless-Os":                              "Windows",
		"X-Stainless-Package-Version":                 "0.55.1",
		"X-Stainless-Retry-Count":                     "0",
		"X-Stainless-Runtime":                         "node",
		"X-Stainless-Runtime-Version":                 "v22.17.0",
		"X-Stainless-Timeout":                         "600",
	}

	return &RequestExtractor{
		requestInfo: &RequestInfo{
			Model:     "claude-3-5-haiku-20241022",
			UserID:    "user_test_account__session_test",
			Headers:   defaultHeaders,
			Extracted: false, // false表示使用默认值，true表示已从实际请求中提取
		},
	}
}

func (re *RequestExtractor) ExtractFromRequest(body []byte, headers http.Header) bool {
	re.mutex.Lock()
	defer re.mutex.Unlock()

	// 总是尝试从请求中提取信息来覆盖默认值
	extracted := false

	// 提取模型信息
	model := re.extractModel(body)
	if model != "" && strings.HasPrefix(model, "claude-3-5") {
		re.requestInfo.Model = model
		extracted = true
	}

	// 提取用户ID
	userID := re.extractUserID(body)
	if userID != "" {
		re.requestInfo.UserID = userID
		extracted = true
	}

	// 提取请求头
	requestHeaders := re.extractHeaders(headers)
	if len(requestHeaders) > 0 {
		// 合并请求头，新的头部会覆盖旧的
		for k, v := range requestHeaders {
			re.requestInfo.Headers[k] = v
		}
		extracted = true
	}

	// 如果成功提取了任何信息，标记为已提取
	if extracted {
		re.requestInfo.Extracted = true
	}

	return extracted
}

func (re *RequestExtractor) extractModel(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return ""
	}

	if model, ok := requestData["model"].(string); ok {
		return model
	}

	return ""
}

func (re *RequestExtractor) extractUserID(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return ""
	}

	if metadata, ok := requestData["metadata"].(map[string]interface{}); ok {
		if userID, ok := metadata["user_id"].(string); ok {
			return userID
		}
	}

	return ""
}

func (re *RequestExtractor) extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	
	// 提取所有请求头，但排除一些敏感或不相关的头部
	excludeHeaders := map[string]bool{
		"authorization":    true,
		"x-api-key":       true,
		"content-length":  true,
		"host":           true,
	}

	for key, values := range headers {
		lowKey := strings.ToLower(key)
		if !excludeHeaders[lowKey] && len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result
}

func (re *RequestExtractor) GetRequestInfo() *RequestInfo {
	re.mutex.RLock()
	defer re.mutex.RUnlock()

	// 返回拷贝以避免并发修改
	return &RequestInfo{
		Model:     re.requestInfo.Model,
		UserID:    re.requestInfo.UserID,
		Headers:   copyHeaders(re.requestInfo.Headers),
		Extracted: re.requestInfo.Extracted,
	}
}

func (re *RequestExtractor) HasExtracted() bool {
	re.mutex.RLock()
	defer re.mutex.RUnlock()
	return re.requestInfo.Extracted
}

func copyHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return make(map[string]string)
	}
	
	result := make(map[string]string, len(headers))
	for k, v := range headers {
		result[k] = v
	}
	return result
}
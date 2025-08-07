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
	return &RequestExtractor{
		requestInfo: &RequestInfo{
			Extracted: false,
		},
	}
}

func (re *RequestExtractor) ExtractFromRequest(body []byte, headers http.Header) bool {
	re.mutex.Lock()
	defer re.mutex.Unlock()

	// 如果已经提取过了，不再提取
	if re.requestInfo.Extracted {
		return false
	}

	// 提取模型信息
	model := re.extractModel(body)
	if model == "" || !strings.HasPrefix(model, "claude-3-5") {
		return false
	}

	// 提取用户ID
	userID := re.extractUserID(body)
	if userID == "" {
		return false
	}

	// 提取请求头
	requestHeaders := re.extractHeaders(headers)

	// 保存提取的信息
	re.requestInfo = &RequestInfo{
		Model:     model,
		UserID:    userID,
		Headers:   requestHeaders,
		Extracted: true,
	}

	return true
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
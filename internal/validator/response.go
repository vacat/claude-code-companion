package validator

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type ResponseValidator struct {
	strictMode      bool
	validateStream  bool
}

func NewResponseValidator(strictMode, validateStream bool) *ResponseValidator {
	return &ResponseValidator{
		strictMode:     strictMode,
		validateStream: validateStream,
	}
}

func (v *ResponseValidator) ValidateAnthropicResponse(body []byte, isStreaming bool) error {
	if isStreaming && !v.validateStream {
		// 如果禁用了流式验证，直接返回成功
		return nil
	}
	if isStreaming {
		return v.ValidateSSEChunk(body)
	}
	return v.ValidateStandardResponse(body)
}

func (v *ResponseValidator) ValidateStandardResponse(body []byte) error {
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("invalid JSON response: %v", err)
	}

	if v.strictMode {
		requiredFields := []string{"id", "type", "content", "model"}
		for _, field := range requiredFields {
			if _, exists := response[field]; !exists {
				return fmt.Errorf("missing required field: %s", field)
			}
		}

		if msgType, ok := response["type"].(string); !ok || msgType != "message" {
			return fmt.Errorf("invalid message type: expected 'message', got '%v'", response["type"])
		}

		if role, exists := response["role"]; exists {
			if roleStr, ok := role.(string); !ok || roleStr != "assistant" {
				return fmt.Errorf("invalid role: expected 'assistant', got '%v'", role)
			}
		}
	} else {
		// 非严格模式：只要是有效JSON且包含content或error字段之一即可
		if _, hasContent := response["content"]; hasContent {
			return nil
		}
		if _, hasError := response["error"]; hasError {
			return nil
		}
		// 如果既没有content也没有error，认为是无效响应
		return fmt.Errorf("response missing both 'content' and 'error' fields")
	}

	return nil
}

func (v *ResponseValidator) ValidateSSEChunk(chunk []byte) error {
	lines := bytes.Split(chunk, []byte("\n"))

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if bytes.HasPrefix(line, []byte("event: ")) {
			eventType := string(line[7:])
			validEvents := []string{
				"message_start", "content_block_start", "ping",
				"content_block_delta", "content_block_stop", "message_stop",
				"message_delta", "error",
			}

			valid := false
			for _, validEvent := range validEvents {
				if eventType == validEvent {
					valid = true
					break
				}
			}

			if !valid {
				return fmt.Errorf("invalid SSE event type: %s", eventType)
			}
		}

		if bytes.HasPrefix(line, []byte("data: ")) {
			dataContent := line[6:]
			if len(dataContent) == 0 || string(dataContent) == "[DONE]" {
				continue
			}

			var data map[string]interface{}
			if err := json.Unmarshal(dataContent, &data); err != nil {
				return fmt.Errorf("invalid JSON in SSE data: %v", err)
			}

			if v.strictMode {
				if _, hasType := data["type"]; !hasType {
					return fmt.Errorf("missing 'type' field in SSE data")
				}
				
				// 检查message_start事件的usage统计
				if err := v.ValidateMessageStartUsage(data); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (v *ResponseValidator) DecompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip data: %v", err)
	}

	return decompressed, nil
}

func (v *ResponseValidator) GetDecompressedBody(body []byte, contentEncoding string) ([]byte, error) {
	if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
		return v.DecompressGzip(body)
	}
	return body, nil
}

func (v *ResponseValidator) IsGzipContent(contentEncoding string) bool {
	return strings.Contains(strings.ToLower(contentEncoding), "gzip")
}

func (v *ResponseValidator) ValidateMessageStartUsage(eventData map[string]interface{}) error {
	eventType, ok := eventData["type"].(string)
	if !ok || eventType != "message_start" {
		return nil
	}

	message, ok := eventData["message"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid message_start: missing message field")
	}

	usage, ok := message["usage"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid message_start: missing usage field")
	}

	// 检查是否存在 input_tokens 和 output_tokens 字段
	_, hasInputTokens := usage["input_tokens"]
	_, hasOutputTokens := usage["output_tokens"]

	if hasInputTokens && hasOutputTokens {
		// 如果存在标准字段，直接认为是合法的（不管值是什么）
		return nil
	} else {
		// 如果不存在标准字段，检查是否为不合法的格式
		promptTokens := float64(-1)
		completionTokens := float64(-1)
		totalTokens := float64(-1)

		if val, ok := usage["prompt_tokens"]; ok {
			if num, ok := val.(float64); ok {
				promptTokens = num
			}
		}

		if val, ok := usage["completion_tokens"]; ok {
			if num, ok := val.(float64); ok {
				completionTokens = num
			}
		}

		if val, ok := usage["total_tokens"]; ok {
			if num, ok := val.(float64); ok {
				totalTokens = num
			}
		}

		// 只有当三个字段都存在且都为0时才判定为不合法
		if promptTokens == 0 && completionTokens == 0 && totalTokens == 0 {
			return fmt.Errorf("invalid usage stats: prompt_tokens, completion_tokens and total_tokens are all zero, indicating malformed response")
		}
	}

	return nil
}

// DetectJSONContent 检测内容是否为JSON格式（而非SSE格式）
func (v *ResponseValidator) DetectJSONContent(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	
	// 检查是否为有效JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return false
	}
	
	// 如果能解析为JSON，再检查是否不是SSE格式
	bodyStr := string(body)
	// SSE格式通常包含 "event: " 和 "data: " 标记
	hasSSEFormat := strings.Contains(bodyStr, "event: ") && strings.Contains(bodyStr, "data: ")
	
	// 如果是有效JSON且不包含SSE格式标记，则认为是JSON内容
	return !hasSSEFormat
}
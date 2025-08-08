package validator

import (
	"testing"
)

func TestValidateMessageStartUsage(t *testing.T) {
	validator := NewResponseValidator(true, true)
	
	// 测试用例1: 正常的message_start事件，包含非零usage统计
	validEvent := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id": "msg_123",
			"usage": map[string]interface{}{
				"input_tokens":  float64(100),
				"output_tokens": float64(0), // output可以为0，因为刚开始
			},
		},
	}
	
	err := validator.ValidateMessageStartUsage(validEvent)
	if err != nil {
		t.Errorf("Expected valid event to pass, got error: %v", err)
	}
	
	// 测试用例2: 无效事件，使用旧版本字段且所有token统计都为0
	invalidEvent := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id": "msg_123",
			"usage": map[string]interface{}{
				"prompt_tokens":     float64(0),
				"completion_tokens": float64(0),
				"total_tokens":      float64(0),
			},
		},
	}
	
	err = validator.ValidateMessageStartUsage(invalidEvent)
	if err == nil {
		t.Error("Expected invalid event (all tokens zero) to fail validation")
	}
	if !contains(err.Error(), "invalid usage stats") {
		t.Errorf("Expected error message to contain 'invalid usage stats', got: %v", err)
	}
	
	// 测试用例3: 非message_start事件应该跳过验证
	otherEvent := map[string]interface{}{
		"type": "content_block_delta",
		"delta": map[string]interface{}{
			"text": "Hello",
		},
	}
	
	err = validator.ValidateMessageStartUsage(otherEvent)
	if err != nil {
		t.Errorf("Expected non-message_start event to be skipped, got error: %v", err)
	}
}

func TestValidateSSEChunkWithUsage(t *testing.T) {
	validator := NewResponseValidator(true, true)
	
	// 测试用例1: 包含有效usage的流式响应
	validSSEData := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","usage":{"input_tokens":100,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0}

`)
	
	err := validator.ValidateSSEChunk(validSSEData)
	if err != nil {
		t.Errorf("Expected valid SSE data to pass, got error: %v", err)
	}
	
	// 测试用例2: 包含无效usage统计的流式响应（使用旧版本字段）
	invalidSSEData := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}}

`)
	
	err = validator.ValidateSSEChunk(invalidSSEData)
	if err == nil {
		t.Error("Expected invalid SSE data (zero tokens) to fail validation")
	}
	if !contains(err.Error(), "invalid usage stats") {
		t.Errorf("Expected error message to contain 'invalid usage stats', got: %v", err)
	}
}

func TestDetectJSONContent(t *testing.T) {
	validator := NewResponseValidator(true, true)
	
	// 测试用例1: 纯JSON内容（非SSE格式）
	jsonContent := []byte(`{"id":"msg_123","type":"message","content":[{"type":"text","text":"Hello"}],"model":"claude-3"}`)
	
	if !validator.DetectJSONContent(jsonContent) {
		t.Error("Expected pure JSON content to be detected as JSON")
	}
	
	// 测试用例2: SSE格式内容
	sseContent := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123"}}

event: content_block_start  
data: {"type":"content_block_start","index":0}

`)
	
	if validator.DetectJSONContent(sseContent) {
		t.Error("Expected SSE content to NOT be detected as JSON")
	}
	
	// 测试用例3: 空内容
	emptyContent := []byte("")
	
	if validator.DetectJSONContent(emptyContent) {
		t.Error("Expected empty content to NOT be detected as JSON")
	}
	
	// 测试用例4: 无效JSON内容  
	invalidContent := []byte("this is not json")
	
	if validator.DetectJSONContent(invalidContent) {
		t.Error("Expected invalid JSON content to NOT be detected as JSON")
	}
	
	// 测试用例5: JSON格式但包含SSE关键字（边界情况）
	jsonWithSSEKeywords := []byte(`{"message":"This contains event: and data: keywords but is still JSON"}`)
	
	if validator.DetectJSONContent(jsonWithSSEKeywords) {
		t.Error("Expected JSON with SSE keywords to NOT be detected as JSON (conservative approach)")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
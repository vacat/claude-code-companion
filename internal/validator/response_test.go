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
	
	// 测试用例2: 无效事件，所有token统计都为0
	invalidEvent := map[string]interface{}{
		"type": "message_start",
		"message": map[string]interface{}{
			"id": "msg_123",
			"usage": map[string]interface{}{
				"input_tokens":  float64(0),
				"output_tokens": float64(0),
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
	
	// 测试用例2: 包含无效usage统计的流式响应
	invalidSSEData := []byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","usage":{"input_tokens":0,"output_tokens":0}}}

`)
	
	err = validator.ValidateSSEChunk(invalidSSEData)
	if err == nil {
		t.Error("Expected invalid SSE data (zero tokens) to fail validation")
	}
	if !contains(err.Error(), "invalid usage stats") {
		t.Errorf("Expected error message to contain 'invalid usage stats', got: %v", err)
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
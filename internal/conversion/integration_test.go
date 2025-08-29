package conversion

import (
	"encoding/json"
	"testing"

	"claude-code-companion/internal/logger"
)

// 辅助函数
func intPtr(i int) *int {
	return &i
}

func getTestLogger() *logger.Logger {
	// Create a simple test logger
	testLogger, _ := logger.NewLogger(logger.LogConfig{
		Level:           "debug",
		LogRequestTypes: "all",
		LogDirectory:    "", // Empty to avoid file operations in tests
	})
	return testLogger
}

// 集成测试：完整的请求-响应循环
func TestFullConversionCycle(t *testing.T) {
	reqConverter := NewRequestConverter(getTestLogger())
	respConverter := NewResponseConverter(getTestLogger())

	// 1. Anthropic 请求
	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "What's the weather like?"},
				},
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get current weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		ToolChoice: &AnthropicToolChoice{Type: "auto"},
		MaxTokens:  intPtr(1024),
	}

	// 2. 转换为 OpenAI 请求
	anthReqBytes, _ := json.Marshal(anthReq)
	oaReqBytes, ctx, err := reqConverter.Convert(anthReqBytes, &EndpointInfo{Type: "openai"})
	if err != nil {
		t.Fatalf("Request conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(oaReqBytes, &oaReq); err != nil {
		t.Fatalf("Failed to parse converted request: %v", err)
	}

	// 3. 模拟 OpenAI 响应（带工具调用）
	oaResp := OpenAIResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4",
		Choices: []OpenAIChoice{
			{
				Index:        0,
				FinishReason: "tool_calls",
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: "I'll check the weather for you.",
					ToolCalls: []OpenAIToolCall{
						{
							ID:   "call_456",
							Type: "function",
							Function: OpenAIToolCallDetail{
								Name:      "get_weather",
								Arguments: `{"location": "New York"}`,
							},
						},
					},
				},
			},
		},
		Usage: &OpenAIUsage{
			PromptTokens:     20,
			CompletionTokens: 30,
			TotalTokens:      50,
		},
	}

	// 4. 转换为 Anthropic 响应
	oaRespBytes, _ := json.Marshal(oaResp)
	anthRespBytes, err := respConverter.convertNonStreamingResponse(oaRespBytes, ctx)
	if err != nil {
		t.Fatalf("Response conversion failed: %v", err)
	}

	var anthResp AnthropicResponse
	if err := json.Unmarshal(anthRespBytes, &anthResp); err != nil {
		t.Fatalf("Failed to parse converted response: %v", err)
	}

	// 5. 验证完整转换结果
	if anthResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthResp.Type)
	}

	if anthResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthResp.Role)
	}

	if len(anthResp.Content) != 2 {
		t.Fatalf("Expected 2 content blocks, got %d", len(anthResp.Content))
	}

	// 验证文本块
	if anthResp.Content[0].Type != "text" {
		t.Errorf("Expected first block type 'text', got '%s'", anthResp.Content[0].Type)
	}

	// 验证工具调用块
	toolBlock := anthResp.Content[1]
	if toolBlock.Type != "tool_use" {
		t.Errorf("Expected second block type 'tool_use', got '%s'", toolBlock.Type)
	}

	if toolBlock.Name != "get_weather" {
		t.Errorf("Expected tool name 'get_weather', got '%s'", toolBlock.Name)
	}

	// 验证使用统计
	if anthResp.Usage.InputTokens != 20 {
		t.Errorf("Expected input_tokens 20, got %d", anthResp.Usage.InputTokens)
	}

	if anthResp.Usage.OutputTokens != 30 {
		t.Errorf("Expected output_tokens 30, got %d", anthResp.Usage.OutputTokens)
	}

	t.Logf("Full conversion cycle test passed successfully")
}
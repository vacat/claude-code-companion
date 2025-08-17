package conversion

import (
	"encoding/json"
	"testing"
)

func TestConvertOpenAIResponseToAnthropic_SimpleText(t *testing.T) {
	converter := NewResponseConverter(getTestLogger())

	oaResp := OpenAIResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4",
		Choices: []OpenAIChoice{
			{
				Index:        0,
				FinishReason: "stop",
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: "Hello! I'm doing well, thank you for asking.",
				},
			},
		},
		Usage: &OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 15,
			TotalTokens:      25,
		},
	}

	respBytes, _ := json.Marshal(oaResp)
	ctx := &ConversionContext{}
	result, err := converter.convertNonStreamingResponse(respBytes, ctx)

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var anthResp AnthropicResponse
	if err := json.Unmarshal(result, &anthResp); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证基本字段
	if anthResp.Type != "message" {
		t.Errorf("Expected type 'message', got '%s'", anthResp.Type)
	}

	if anthResp.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", anthResp.Role)
	}

	if anthResp.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", anthResp.Model)
	}

	if anthResp.StopReason != "end_turn" {
		t.Errorf("Expected stop_reason 'end_turn', got '%s'", anthResp.StopReason)
	}

	// 验证内容
	if len(anthResp.Content) != 1 {
		t.Fatalf("Expected 1 content block, got %d", len(anthResp.Content))
	}

	content := anthResp.Content[0]
	if content.Type != "text" {
		t.Errorf("Expected content type 'text', got '%s'", content.Type)
	}

	if content.Text != "Hello! I'm doing well, thank you for asking." {
		t.Errorf("Expected text 'Hello! I'm doing well, thank you for asking.', got '%s'", content.Text)
	}

	// 验证使用统计
	if anthResp.Usage == nil {
		t.Fatal("Usage should not be nil")
	}

	if anthResp.Usage.InputTokens != 10 {
		t.Errorf("Expected input_tokens 10, got %d", anthResp.Usage.InputTokens)
	}

	if anthResp.Usage.OutputTokens != 15 {
		t.Errorf("Expected output_tokens 15, got %d", anthResp.Usage.OutputTokens)
	}
}

func TestConvertOpenAIResponseToAnthropic_WithToolCalls(t *testing.T) {
	converter := NewResponseConverter(getTestLogger())

	oaResp := OpenAIResponse{
		ID:    "chatcmpl-123",
		Model: "gpt-4",
		Choices: []OpenAIChoice{
			{
				Index:        0,
				FinishReason: "tool_calls",
				Message: OpenAIMessage{
					Role:    "assistant",
					Content: "I'll list the files for you.",
					ToolCalls: []OpenAIToolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: OpenAIToolCallDetail{
								Name:      "list_files",
								Arguments: `{"path": "/tmp"}`,
							},
						},
					},
				},
			},
		},
	}

	respBytes, _ := json.Marshal(oaResp)
	ctx := &ConversionContext{}
	result, err := converter.convertNonStreamingResponse(respBytes, ctx)

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var anthResp AnthropicResponse
	if err := json.Unmarshal(result, &anthResp); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证内容块数量（text + tool_use）
	if len(anthResp.Content) != 2 {
		t.Fatalf("Expected 2 content blocks, got %d", len(anthResp.Content))
	}

	// 验证文本块
	textBlock := anthResp.Content[0]
	if textBlock.Type != "text" {
		t.Errorf("Expected first block type 'text', got '%s'", textBlock.Type)
	}

	if textBlock.Text != "I'll list the files for you." {
		t.Errorf("Expected text 'I'll list the files for you.', got '%s'", textBlock.Text)
	}

	// 验证工具调用块
	toolBlock := anthResp.Content[1]
	if toolBlock.Type != "tool_use" {
		t.Errorf("Expected second block type 'tool_use', got '%s'", toolBlock.Type)
	}

	if toolBlock.ID != "call_123" {
		t.Errorf("Expected tool use ID 'call_123', got '%s'", toolBlock.ID)
	}

	if toolBlock.Name != "list_files" {
		t.Errorf("Expected tool name 'list_files', got '%s'", toolBlock.Name)
	}

	// 验证工具参数（应该是原生JSON）
	expectedInput := `{"path":"/tmp"}` // JSON.Marshal没有空格
	if string(toolBlock.Input) != expectedInput {
		t.Errorf("Expected input '%s', got '%s'", expectedInput, string(toolBlock.Input))
	}

	if anthResp.StopReason != "tool_use" {
		t.Errorf("Expected stop_reason 'tool_use', got '%s'", anthResp.StopReason)
	}
}
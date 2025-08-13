package conversion

import (
	"encoding/json"
	"testing"

	"claude-proxy/internal/logger"
)


func TestConvertAnthropicRequestToOpenAI_SimpleText(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "Hello, how are you?"},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes)

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证基本字段
	if oaReq.Model != "claude-3-sonnet-20240229" {
		t.Errorf("Expected model 'claude-3-sonnet-20240229', got '%s'", oaReq.Model)
	}

	if *oaReq.MaxCompletionTokens != 1024 {
		t.Errorf("Expected max_completion_tokens 1024, got %d", *oaReq.MaxCompletionTokens)
	}

	// 验证消息转换
	if len(oaReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(oaReq.Messages))
	}

	msg := oaReq.Messages[0]
	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	if msg.Content != "Hello, how are you?" {
		t.Errorf("Expected content 'Hello, how are you?', got '%v'", msg.Content)
	}

	// 验证上下文
	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithTools(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "What files are in the current directory?"},
				},
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "list_files",
				Description: "List files in a directory",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		ToolChoice: &AnthropicToolChoice{Type: "auto"},
		MaxTokens:  intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes)

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证工具转换
	if len(oaReq.Tools) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(oaReq.Tools))
	}

	tool := oaReq.Tools[0]
	if tool.Type != "function" {
		t.Errorf("Expected tool type 'function', got '%s'", tool.Type)
	}

	if tool.Function.Name != "list_files" {
		t.Errorf("Expected function name 'list_files', got '%s'", tool.Function.Name)
	}

	// 验证tool_choice转换
	if oaReq.ToolChoice != "auto" {
		t.Errorf("Expected tool_choice 'auto', got '%v'", oaReq.ToolChoice)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithToolUse(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "List files in /tmp"},
				},
			},
			{
				Role: "assistant",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "I'll list the files for you."},
					{
						Type:  "tool_use",
						ID:    "call_123",
						Name:  "list_files",
						Input: json.RawMessage(`{"path": "/tmp"}`),
					},
				},
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "list_files",
				Description: "List files in a directory",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes)

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证消息数量（user + assistant）
	if len(oaReq.Messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(oaReq.Messages))
	}

	// 验证assistant消息的tool_calls
	assistantMsg := oaReq.Messages[1]
	if assistantMsg.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", assistantMsg.Role)
	}

	if assistantMsg.Content != "I'll list the files for you." {
		t.Errorf("Expected content 'I'll list the files for you.', got '%v'", assistantMsg.Content)
	}

	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}

	toolCall := assistantMsg.ToolCalls[0]
	if toolCall.ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", toolCall.ID)
	}

	if toolCall.Function.Name != "list_files" {
		t.Errorf("Expected function name 'list_files', got '%s'", toolCall.Function.Name)
	}

	expectedArgs := `{"path":"/tmp"}` // JSON.Marshal没有空格
	if toolCall.Function.Arguments != expectedArgs {
		t.Errorf("Expected arguments '%s', got '%s'", expectedArgs, toolCall.Function.Arguments)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithToolResult(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{
						Type:      "tool_result",
						ToolUseID: "call_123",
						Content: []AnthropicContentBlock{
							{Type: "text", Text: "file1.txt\nfile2.txt"},
						},
					},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes)

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证tool result转换为tool消息
	if len(oaReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(oaReq.Messages))
	}

	msg := oaReq.Messages[0]
	if msg.Role != "tool" {
		t.Errorf("Expected role 'tool', got '%s'", msg.Role)
	}

	if msg.ToolCallID != "call_123" {
		t.Errorf("Expected tool_call_id 'call_123', got '%s'", msg.ToolCallID)
	}

	if msg.Content != "file1.txt\nfile2.txt" {
		t.Errorf("Expected content 'file1.txt\\nfile2.txt', got '%v'", msg.Content)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

func TestConvertAnthropicRequestToOpenAI_WithImage(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	anthReq := AnthropicRequest{
		Model: "claude-3-sonnet-20240229",
		Messages: []AnthropicMessage{
			{
				Role: "user",
				Content: []AnthropicContentBlock{
					{Type: "text", Text: "What's in this image?"},
					{
						Type: "image",
						Source: &AnthropicImageSource{
							Type:      "base64",
							MediaType: "image/png",
							Data:      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChAGH",
						},
					},
				},
			},
		},
		MaxTokens: intPtr(1024),
	}

	reqBytes, _ := json.Marshal(anthReq)
	result, ctx, err := converter.Convert(reqBytes)

	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	var oaReq OpenAIRequest
	if err := json.Unmarshal(result, &oaReq); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// 验证消息转换
	if len(oaReq.Messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(oaReq.Messages))
	}

	msg := oaReq.Messages[0]
	if msg.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.Role)
	}

	// 应该是数组格式的content（因为有图片）
	contentInterface, ok := msg.Content.([]interface{})
	if !ok {
		t.Fatalf("Expected content to be array, got %T", msg.Content)
	}
	
	// 将interface{}数组转换为OpenAIMessageContent数组
	var contentArray []OpenAIMessageContent
	for _, item := range contentInterface {
		if itemMap, ok := item.(map[string]interface{}); ok {
			content := OpenAIMessageContent{
				Type: itemMap["type"].(string),
			}
			if content.Type == "text" {
				content.Text = itemMap["text"].(string)
			} else if content.Type == "image_url" {
				if imageURL, ok := itemMap["image_url"].(map[string]interface{}); ok {
					content.ImageURL = &OpenAIImageURL{
						URL: imageURL["url"].(string),
					}
				}
			}
			contentArray = append(contentArray, content)
		}
	}

	if len(contentArray) != 2 {
		t.Fatalf("Expected 2 content parts, got %d", len(contentArray))
	}

	// 验证图片部分
	var imageContent *OpenAIMessageContent
	var textContent *OpenAIMessageContent
	for i := range contentArray {
		if contentArray[i].Type == "image_url" {
			imageContent = &contentArray[i]
		} else if contentArray[i].Type == "text" {
			textContent = &contentArray[i]
		}
	}

	if imageContent == nil {
		t.Fatal("Image content not found")
	}

	if textContent == nil {
		t.Fatal("Text content not found")
	}

	if textContent.Text != "What's in this image?" {
		t.Errorf("Expected text 'What's in this image?', got '%s'", textContent.Text)
	}

	expectedURL := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChAGH"
	if imageContent.ImageURL.URL != expectedURL {
		t.Errorf("Expected image URL '%s', got '%s'", expectedURL, imageContent.ImageURL.URL)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}
}

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

	if anthResp.StopReason != "stop" {
		t.Errorf("Expected stop_reason 'stop', got '%s'", anthResp.StopReason)
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

	if anthResp.StopReason != "tool_calls" {
		t.Errorf("Expected stop_reason 'tool_calls', got '%s'", anthResp.StopReason)
	}
}

func TestToolChoiceMapping(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	testCases := []struct {
		name           string
		anthToolChoice *AnthropicToolChoice
		expectedOA     interface{}
	}{
		{
			name:           "auto",
			anthToolChoice: &AnthropicToolChoice{Type: "auto"},
			expectedOA:     "auto",
		},
		{
			name:           "any -> required",
			anthToolChoice: &AnthropicToolChoice{Type: "any"},
			expectedOA:     "required",
		},
		{
			name:           "tool with name",
			anthToolChoice: &AnthropicToolChoice{Type: "tool", Name: "list_files"},
			expectedOA: map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": "list_files",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			anthReq := AnthropicRequest{
				Model: "claude-3-sonnet-20240229",
				Messages: []AnthropicMessage{
					{
						Role: "user",
						Content: []AnthropicContentBlock{
							{Type: "text", Text: "Test"},
						},
					},
				},
				ToolChoice: tc.anthToolChoice,
				MaxTokens:  intPtr(1024),
			}

			reqBytes, _ := json.Marshal(anthReq)
			result, _, err := converter.Convert(reqBytes)

			if err != nil {
				t.Fatalf("Conversion failed: %v", err)
			}

			var oaReq OpenAIRequest
			if err := json.Unmarshal(result, &oaReq); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			// 比较结果
			expectedJson, _ := json.Marshal(tc.expectedOA)
			actualJson, _ := json.Marshal(oaReq.ToolChoice)

			if string(expectedJson) != string(actualJson) {
				t.Errorf("Expected tool_choice %s, got %s", string(expectedJson), string(actualJson))
			}
		})
	}
}

func TestSystemMessageHandling(t *testing.T) {
	converter := NewRequestConverter(getTestLogger())
	

	testCases := []struct {
		name           string
		system         interface{}
		expectedExists bool
		expectedText   string
	}{
		{
			name:           "string system",
			system:         "You are a helpful assistant.",
			expectedExists: true,
			expectedText:   "You are a helpful assistant.",
		},
		{
			name: "blocks system",
			system: []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "You are a helpful assistant.",
				},
			},
			expectedExists: true,
			expectedText:   "You are a helpful assistant.",
		},
		{
			name:           "nil system",
			system:         nil,
			expectedExists: false,
			expectedText:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			anthReq := AnthropicRequest{
				Model: "claude-3-sonnet-20240229",
				System: tc.system,
				Messages: []AnthropicMessage{
					{
						Role: "user",
						Content: []AnthropicContentBlock{
							{Type: "text", Text: "Hello"},
						},
					},
				},
				MaxTokens: intPtr(1024),
			}

			reqBytes, _ := json.Marshal(anthReq)
			result, _, err := converter.Convert(reqBytes)

			if err != nil {
				t.Fatalf("Conversion failed: %v", err)
			}

			var oaReq OpenAIRequest
			if err := json.Unmarshal(result, &oaReq); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			// 检查系统消息
			var systemMsg *OpenAIMessage
			for _, msg := range oaReq.Messages {
				if msg.Role == "system" {
					systemMsg = &msg
					break
				}
			}

			if tc.expectedExists {
				if systemMsg == nil {
					t.Fatal("Expected system message, but not found")
				}
				if systemMsg.Content != tc.expectedText {
					t.Errorf("Expected system content '%s', got '%v'", tc.expectedText, systemMsg.Content)
				}
			} else {
				if systemMsg != nil {
					t.Fatal("Expected no system message, but found one")
				}
			}
		})
	}
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
	oaReqBytes, ctx, err := reqConverter.Convert(anthReqBytes)
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

// 辅助函数
func intPtr(i int) *int {
	return &i
}

func getTestLogger() *logger.Logger {
	return (*logger.Logger)(nil) // 使用nil logger用于测试
}

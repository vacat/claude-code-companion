package conversion

import (
	"encoding/json"
	"strings"
	"testing"
)

// Test the MessageAggregator component
func TestMessageAggregator_SimpleText(t *testing.T) {
	aggregator := NewMessageAggregator(getTestLogger())

	// Create test chunks for simple text
	chunks := []OpenAIStreamChunk{
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIMessage{
						Content: "Hello",
					},
				},
			},
		},
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIMessage{
						Content: " World!",
					},
				},
			},
		},
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index:        0,
					Delta:        OpenAIMessage{},
					FinishReason: "stop",
				},
			},
			Usage: &OpenAIUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
	}

	// Aggregate chunks
	result, err := aggregator.AggregateChunks(chunks)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	// Verify results
	if result.ID != "msg_chatcmpl-123" {
		t.Errorf("Expected ID 'msg_chatcmpl-123', got '%s'", result.ID)
	}

	if result.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", result.Model)
	}

	if result.TextContent != "Hello World!" {
		t.Errorf("Expected text content 'Hello World!', got '%s'", result.TextContent)
	}

	if result.FinishReason != "end_turn" {
		t.Errorf("Expected finish reason 'end_turn', got '%s'", result.FinishReason)
	}

	if len(result.ToolCalls) != 0 {
		t.Errorf("Expected 0 tool calls, got %d", len(result.ToolCalls))
	}

	if result.Usage == nil {
		t.Fatal("Expected usage to be set")
	}

	if result.Usage.PromptTokens != 10 {
		t.Errorf("Expected prompt tokens 10, got %d", result.Usage.PromptTokens)
	}
}

func TestMessageAggregator_WithToolCalls(t *testing.T) {
	aggregator := NewMessageAggregator(getTestLogger())

	// Create test chunks with tool calls
	chunks := []OpenAIStreamChunk{
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIMessage{
						Content: "I'll help you with that.",
					},
				},
			},
		},
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIMessage{
						ToolCalls: []OpenAIToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: OpenAIToolCallDetail{
									Name:      "list_files",
									Arguments: `{"path":`,
								},
							},
						},
					},
				},
			},
		},
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index: 0,
					Delta: OpenAIMessage{
						ToolCalls: []OpenAIToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: OpenAIToolCallDetail{
									Arguments: `"/tmp"}`,
								},
							},
						},
					},
				},
			},
		},
		{
			ID:    "chatcmpl-123",
			Model: "gpt-4",
			Choices: []OpenAIStreamChoice{
				{
					Index:        0,
					Delta:        OpenAIMessage{},
					FinishReason: "tool_calls",
				},
			},
		},
	}

	// Aggregate chunks
	result, err := aggregator.AggregateChunks(chunks)
	if err != nil {
		t.Fatalf("Aggregation failed: %v", err)
	}

	// Verify results
	if result.TextContent != "I'll help you with that." {
		t.Errorf("Expected text content 'I'll help you with that.', got '%s'", result.TextContent)
	}

	if result.FinishReason != "tool_use" {
		t.Errorf("Expected finish reason 'tool_use', got '%s'", result.FinishReason)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(result.ToolCalls))
	}

	toolCall := result.ToolCalls[0]
	if toolCall.ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got '%s'", toolCall.ID)
	}

	if toolCall.Name != "list_files" {
		t.Errorf("Expected tool name 'list_files', got '%s'", toolCall.Name)
	}

	if toolCall.Arguments != `{"path":"/tmp"}` {
		t.Errorf("Expected arguments '{\"path\":\"/tmp\"}', got '%s'", toolCall.Arguments)
	}
}

// Test the UnifiedConverter component
func TestUnifiedConverter_SimpleText(t *testing.T) {
	converter := NewUnifiedConverter(getTestLogger())

	// Create aggregated message
	msg := &AggregatedMessage{
		ID:           "msg_chatcmpl-123",
		Model:        "gpt-4",
		TextContent:  "Hello World!",
		ToolCalls:    []AggregatedToolCall{},
		FinishReason: "end_turn",
		Usage: &OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	// Convert to events
	result, err := converter.ConvertAggregatedMessage(msg)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Verify event sequence
	expectedEvents := []string{
		"message_start",
		"content_block_start",
		"ping",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}

	if len(result.Events) != len(expectedEvents) {
		t.Fatalf("Expected %d events, got %d", len(expectedEvents), len(result.Events))
	}

	for i, expectedType := range expectedEvents {
		if result.Events[i].Type != expectedType {
			t.Errorf("Event %d: expected type '%s', got '%s'", i, expectedType, result.Events[i].Type)
		}
	}

	// Verify message_start event
	messageStartEvent := result.Events[0]
	messageStart, ok := messageStartEvent.Data.(*AnthropicMessageStart)
	if !ok {
		t.Fatal("Expected message_start event data to be AnthropicMessageStart")
	}

	if messageStart.Message.ID != "msg_chatcmpl-123" {
		t.Errorf("Expected message ID 'msg_chatcmpl-123', got '%s'", messageStart.Message.ID)
	}

	if messageStart.Message.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", messageStart.Message.Model)
	}

	// Verify content_block_delta event contains text
	deltaEvent := result.Events[3]
	delta, ok := deltaEvent.Data.(*AnthropicContentBlockDelta)
	if !ok {
		t.Fatal("Expected content_block_delta event data to be AnthropicContentBlockDelta")
	}

	if delta.Delta.Text != "Hello World!" {
		t.Errorf("Expected delta text 'Hello World!', got '%s'", delta.Delta.Text)
	}
}

func TestUnifiedConverter_WithToolCalls(t *testing.T) {
	converter := NewUnifiedConverter(getTestLogger())

	// Create aggregated message with tool calls
	msg := &AggregatedMessage{
		ID:          "msg_chatcmpl-123",
		Model:       "gpt-4",
		TextContent: "I'll help you.",
		ToolCalls: []AggregatedToolCall{
			{
				ID:        "call_123",
				Name:      "list_files",
				Arguments: `{"path":"/tmp"}`,
			},
		},
		FinishReason: "tool_use",
	}

	// Convert to events
	result, err := converter.ConvertAggregatedMessage(msg)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Should have events for text block + tool block + message events
	expectedMinEvents := 9 // message_start, text_start, ping, text_delta, text_stop, tool_start, tool_delta, tool_stop, message_delta, message_stop
	if len(result.Events) < expectedMinEvents {
		t.Fatalf("Expected at least %d events, got %d", expectedMinEvents, len(result.Events))
	}

	// Look for tool use content block start
	var foundToolStart bool
	for _, event := range result.Events {
		if event.Type == "content_block_start" {
			if start, ok := event.Data.(*AnthropicContentBlockStart); ok {
				if start.ContentBlock.Type == "tool_use" {
					foundToolStart = true
					if start.ContentBlock.ID != "call_123" {
						t.Errorf("Expected tool ID 'call_123', got '%s'", start.ContentBlock.ID)
					}
					if start.ContentBlock.Name != "list_files" {
						t.Errorf("Expected tool name 'list_files', got '%s'", start.ContentBlock.Name)
					}
				}
			}
		}
	}

	if !foundToolStart {
		t.Error("Expected to find tool_use content_block_start event")
	}
}

// Test the complete refactored streaming conversion
func TestRefactoredStreamingConversion_Complete(t *testing.T) {
	converter := NewResponseConverter(getTestLogger())

	// Create OpenAI SSE stream with text and tool calls
	openaiSSE := `data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{"content":"I'll help"},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{"content":" you."},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{"tool_calls":[{"id":"call_123","type":"function","function":{"name":"list_files","arguments":""}}]},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{"tool_calls":[{"id":"call_123","type":"function","function":{"arguments":"{\"path\":\"/tmp\"}"}}]},"index":0,"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{},"index":0,"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":15,"total_tokens":35}}

data: [DONE]

`

	// Create conversion context
	ctx := &ConversionContext{
		EndpointType: "openai",
	}

	// Execute refactored conversion
	result, err := converter.convertStreamingResponseRefactored([]byte(openaiSSE), ctx)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	resultStr := string(result)
	t.Logf("Refactored conversion result:\n%s", resultStr)

	// Verify SSE format
	if !strings.Contains(resultStr, "event: message_start") {
		t.Error("Expected message_start event")
	}

	if !strings.Contains(resultStr, "event: content_block_start") {
		t.Error("Expected content_block_start event")
	}

	if !strings.Contains(resultStr, "event: content_block_delta") {
		t.Error("Expected content_block_delta event")
	}

	if !strings.Contains(resultStr, "event: content_block_stop") {
		t.Error("Expected content_block_stop event")
	}

	if !strings.Contains(resultStr, "event: message_delta") {
		t.Error("Expected message_delta event")
	}

	if !strings.Contains(resultStr, "event: message_stop") {
		t.Error("Expected message_stop event")
	}

	if !strings.Contains(resultStr, "event: ping") {
		t.Error("Expected ping event")
	}

	// Verify content
	if !strings.Contains(resultStr, "I'll help you.") {
		t.Error("Expected aggregated text content")
	}

	if !strings.Contains(resultStr, "list_files") {
		t.Error("Expected tool name in result")
	}

	if !strings.Contains(resultStr, "tool_use") {
		t.Error("Expected tool_use finish reason")
	}

	// Verify JSON structure is valid
	lines := strings.Split(resultStr, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataContent := strings.TrimPrefix(line, "data: ")
			if dataContent != "" {
				var jsonData interface{}
				if err := json.Unmarshal([]byte(dataContent), &jsonData); err != nil {
					t.Errorf("Invalid JSON in SSE data: %s, error: %v", dataContent, err)
				}
			}
		}
	}
}

// Test edge cases
func TestMessageAggregator_EmptyChunks(t *testing.T) {
	aggregator := NewMessageAggregator(getTestLogger())

	// Test with empty chunks
	_, err := aggregator.AggregateChunks([]OpenAIStreamChunk{})
	if err == nil {
		t.Error("Expected error for empty chunks")
	}
}

func TestMessageAggregator_FinishReasonMapping(t *testing.T) {
	aggregator := NewMessageAggregator(getTestLogger())

	testCases := []struct {
		openaiReason     string
		expectedAnthropic string
	}{
		{"stop", "end_turn"},
		{"tool_calls", "tool_use"},
		{"length", "max_tokens"},
		{"unknown_reason", "end_turn"},
	}

	for _, tc := range testCases {
		chunks := []OpenAIStreamChunk{
			{
				ID:    "test",
				Model: "gpt-4",
				Choices: []OpenAIStreamChoice{
					{
						Index:        0,
						FinishReason: tc.openaiReason,
					},
				},
			},
		}

		result, err := aggregator.AggregateChunks(chunks)
		if err != nil {
			t.Fatalf("Aggregation failed for reason %s: %v", tc.openaiReason, err)
		}

		if result.FinishReason != tc.expectedAnthropic {
			t.Errorf("For OpenAI reason '%s', expected '%s', got '%s'",
				tc.openaiReason, tc.expectedAnthropic, result.FinishReason)
		}
	}
}
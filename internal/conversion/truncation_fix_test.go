package conversion

import (
	"testing"
)

func TestUnifiedConverter_TruncatedToolCall(t *testing.T) {
	converter := NewUnifiedConverter(nil) // nil logger for testing

	// Create a test case with truncated tool call
	msg := &AggregatedMessage{
		ID:           "chatcmpl-68a2f01c436aa79cbdbc5de2",
		Model:        "kimi-k2-0711-preview",
		TextContent:  "",
		FinishReason: "max_tokens", // This indicates truncation
		ToolCalls: []AggregatedToolCall{
			{
				ID:   "Write:16",
				Name: "Write",
				// Incomplete JSON - missing closing braces
				Arguments: `{"file_path":"D:\\Work\\sptest\\internal\\proxy\\proxy.go", "content":"package proxy\\nim`,
			},
		},
	}

	// Convert the message
	result, err := converter.ConvertAggregatedMessage(msg)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Check that no tool_use content_block_start event was generated
	hasToolUseBlock := false
	hasMaxTokensStop := false

	for _, event := range result.Events {
		if event.Type == "content_block_start" {
			if startEvent, ok := event.Data.(*AnthropicContentBlockStart); ok {
				if startEvent.ContentBlock != nil && startEvent.ContentBlock.Type == "tool_use" {
					hasToolUseBlock = true
				}
			}
		}
		if event.Type == "message_delta" {
			if deltaEvent, ok := event.Data.(*AnthropicMessageDelta); ok {
				if deltaEvent.Delta != nil && deltaEvent.Delta.StopReason == "max_tokens" {
					hasMaxTokensStop = true
				}
			}
		}
	}

	// Assertions
	if hasToolUseBlock {
		t.Error("Expected no tool_use content block for truncated response with incomplete JSON, but found one")
	}

	if !hasMaxTokensStop {
		t.Error("Expected max_tokens stop reason in message_delta event")
	}

	t.Logf("Test passed: tool_use block correctly skipped for truncated response")
}

func TestUnifiedConverter_CompleteToolCall(t *testing.T) {
	converter := NewUnifiedConverter(nil) // nil logger for testing

	// Create a test case with complete tool call
	msg := &AggregatedMessage{
		ID:           "chatcmpl-12345",
		Model:        "gpt-4",
		TextContent:  "",
		FinishReason: "tool_use", // Normal tool completion
		ToolCalls: []AggregatedToolCall{
			{
				ID:   "call_123",
				Name: "get_weather",
				// Complete JSON
				Arguments: `{"location": "New York", "unit": "celsius"}`,
			},
		},
	}

	// Convert the message
	result, err := converter.ConvertAggregatedMessage(msg)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Check that tool_use content_block_start event was generated
	hasToolUseBlock := false

	for _, event := range result.Events {
		if event.Type == "content_block_start" {
			if startEvent, ok := event.Data.(*AnthropicContentBlockStart); ok {
				if startEvent.ContentBlock != nil && startEvent.ContentBlock.Type == "tool_use" {
					hasToolUseBlock = true
				}
			}
		}
	}

	// Assertions
	if !hasToolUseBlock {
		t.Error("Expected tool_use content block for complete tool call, but found none")
	}

	t.Logf("Test passed: tool_use block correctly generated for complete tool call")
}

func TestUnifiedConverter_MaxTokensWithCompleteJSON(t *testing.T) {
	converter := NewUnifiedConverter(nil) // nil logger for testing

	// Create a test case with max_tokens but complete JSON
	msg := &AggregatedMessage{
		ID:           "chatcmpl-67890",
		Model:        "gpt-4",
		TextContent:  "",
		FinishReason: "max_tokens", // Truncated but JSON is complete
		ToolCalls: []AggregatedToolCall{
			{
				ID:   "call_789",
				Name: "calculate",
				// Complete JSON even though response was truncated
				Arguments: `{"operation": "add", "numbers": [1, 2, 3]}`,
			},
		},
	}

	// Convert the message
	result, err := converter.ConvertAggregatedMessage(msg)
	if err != nil {
		t.Fatalf("Conversion failed: %v", err)
	}

	// Check that tool_use content_block_start event was generated since JSON is complete
	hasToolUseBlock := false

	for _, event := range result.Events {
		if event.Type == "content_block_start" {
			if startEvent, ok := event.Data.(*AnthropicContentBlockStart); ok {
				if startEvent.ContentBlock != nil && startEvent.ContentBlock.Type == "tool_use" {
					hasToolUseBlock = true
				}
			}
		}
	}

	// Assertions
	if !hasToolUseBlock {
		t.Error("Expected tool_use content block for max_tokens with complete JSON, but found none")
	}

	t.Logf("Test passed: tool_use block correctly generated for max_tokens with complete JSON")
}

func TestIsValidCompleteJSON(t *testing.T) {
	converter := NewUnifiedConverter(nil)

	testCases := []struct {
		name     string
		json     string
		expected bool
	}{
		{
			name:     "empty string",
			json:     "",
			expected: true,
		},
		{
			name:     "complete object",
			json:     `{"key": "value"}`,
			expected: true,
		},
		{
			name:     "incomplete object - missing closing brace",
			json:     `{"key": "value"`,
			expected: false,
		},
		{
			name:     "incomplete string - unclosed quote",
			json:     `{"key": "value`,
			expected: false,
		},
		{
			name:     "complete array",
			json:     `[1, 2, 3]`,
			expected: true,
		},
		{
			name:     "incomplete array - missing closing bracket",
			json:     `[1, 2, 3`,
			expected: false,
		},
		{
			name:     "nested complete object",
			json:     `{"outer": {"inner": "value"}}`,
			expected: true,
		},
		{
			name:     "nested incomplete object",
			json:     `{"outer": {"inner": "value"`,
			expected: false,
		},
		{
			name:     "invalid JSON",
			json:     `{key: value}`,
			expected: false,
		},
		{
			name:     "complex incomplete - sample case",
			json:     `{"file_path":"D:\\Work\\sptest\\internal\\proxy\\proxy.go", "content":"package proxy\\nim`,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := converter.isValidCompleteJSON(tc.json)
			if result != tc.expected {
				t.Errorf("isValidCompleteJSON(%q) = %v, expected %v", tc.json, result, tc.expected)
			}
		})
	}
}
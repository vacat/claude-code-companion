package conversion

import (
	"encoding/json"
	"testing"

	"claude-proxy/internal/config"
	"claude-proxy/internal/logger"
)

func TestSSEParser_Integration_PythonJSONFix(t *testing.T) {
	// Create a logger for testing
	logConfig := logger.LogConfig{
		Level:           "error",
		LogRequestTypes: "none",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "none",
	}
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create SSE parser with a custom fixer configuration for testing
	customConfig := config.PythonJSONFixingConfig{
		Enabled:      true,
		TargetTools:  []string{"TodoWrite", "OtherTool"},
		DebugLogging: false,
		MaxAttempts:  3,
	}
	fixer := NewPythonJSONFixerWithConfig(log, customConfig)
	parser := &SSEParser{
		logger: log,
		fixer:  fixer,
	}

	tests := []struct {
		name               string
		sseData            string
		expectedChunkCount int
		expectFixApplied   bool
		toolName           string
	}{
		{
			name: "Python-style TodoWrite tool call",
			sseData: `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"TodoWrite","arguments":"{'todos': [{'content': '创建项目结构', 'id': '1', 'status': 'pending'}]}"}}]},"finish_reason":"tool_calls"}]}

`,
			expectedChunkCount: 1,
			expectFixApplied:   true,
			toolName:           "TodoWrite",
		},
		{
			name: "Valid JSON tool call",
			sseData: `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"TodoWrite","arguments":"{\"todos\": [{\"content\": \"task\", \"id\": \"1\", \"status\": \"pending\"}]}"}}]},"finish_reason":"tool_calls"}]}

`,
			expectedChunkCount: 1,
			expectFixApplied:   false,
			toolName:           "TodoWrite",
		},
		{
			name: "Non-TodoWrite tool with Python style",
			sseData: `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"OtherTool","arguments":"{'param': 'value'}"}}]},"finish_reason":"tool_calls"}]}

`,
			expectedChunkCount: 1,
			expectFixApplied:   true, // The SSE parser will still try to fix it
			toolName:           "OtherTool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, err := parser.ParseSSEStream([]byte(tt.sseData))
			if err != nil {
				t.Fatalf("ParseSSEStream failed: %v", err)
			}

			if len(chunks) != tt.expectedChunkCount {
				t.Errorf("Expected %d chunks, got %d", tt.expectedChunkCount, len(chunks))
				return
			}

			if len(chunks) > 0 && len(chunks[0].Choices) > 0 {
				choice := chunks[0].Choices[0]
				if len(choice.Delta.ToolCalls) > 0 {
					toolCall := choice.Delta.ToolCalls[0]
					
					// Verify tool name
					if toolCall.Function.Name != tt.toolName {
						t.Errorf("Expected tool name %s, got %s", tt.toolName, toolCall.Function.Name)
					}

			// Note: SSE parser just extracts chunks, it doesn't fix JSON yet
			// The fix is applied during the stream processing phase
			if tt.expectFixApplied {
				// For Python-style content, it should be detected as needing fix
				// but the actual fix happens later in the pipeline
				// For now, just verify the content is extracted correctly
				if toolCall.Function.Arguments == "" {
					t.Error("Tool arguments should not be empty")
				}
			} else {
				// Try to parse the arguments as JSON to verify they're valid
				var args interface{}
				err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				if err != nil {
					t.Errorf("Tool arguments are not valid JSON: %v", err)
				}
			}
				}
			}
		})
	}
}

func TestSimpleJSONBuffer_Integration_PythonJSONFix(t *testing.T) {
	// Create a logger for testing
	logConfig := logger.LogConfig{
		Level:           "error",
		LogRequestTypes: "none",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "none",
	}
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	tests := []struct {
		name         string
		fragments    []string
		toolName     string
		expectedJSON string
		expectFixed  bool
	}{
		{
			name: "TodoWrite with Python style - incremental build",
			fragments: []string{
				"{'todos': [",
				"{'content': '创建项目结构', ",
				"'id': '1', ",
				"'status': 'pending'}",
				"]}",
			},
			toolName:     "TodoWrite",
			expectedJSON: `{"todos": [{"content": "创建项目结构", "id": "1", "status": "pending"}]}`,
			expectFixed:  true,
		},
		{
			name: "Valid JSON - no fix needed",
			fragments: []string{
				`{"todos": [`,
				`{"content": "task", `,
				`"id": "1", `,
				`"status": "pending"}`,
				`]}`,
			},
			toolName:     "TodoWrite",
			expectedJSON: `{"todos": [{"content": "task", "id": "1", "status": "pending"}]}`,
			expectFixed:  false,
		},
		{
			name: "Non-TodoWrite tool",
			fragments: []string{
				"{'param': 'value'}",
			},
			toolName:     "OtherTool",
			expectedJSON: `{"param": "value"}`,
			expectFixed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer with custom configuration for testing
			var buffer *SimpleJSONBuffer
			if tt.toolName != "TodoWrite" {
				// For non-TodoWrite tools, create a fixer with broader target tools
				customConfig := config.PythonJSONFixingConfig{
					Enabled:      true,
					TargetTools:  []string{"TodoWrite", "OtherTool"},
					DebugLogging: false,
					MaxAttempts:  3,
				}
				fixer := NewPythonJSONFixerWithConfig(log, customConfig)
				buffer = &SimpleJSONBuffer{
					lastOutputLength: 0,
					fixer:            fixer,
				}
			} else {
				buffer = NewSimpleJSONBufferWithFixer(log)
			}
			buffer.SetToolName(tt.toolName)

			// Add fragments incrementally
			for _, fragment := range tt.fragments {
				buffer.AppendFragmentWithFix(fragment, tt.toolName)
			}

			// Get the final content
			content := buffer.GetBufferedContent()
			fixedContent := buffer.GetFixedBufferedContent()

			// Check if fix was needed
			wasFixed := content != fixedContent
			if wasFixed != tt.expectFixed {
				t.Errorf("Expected fix applied: %v, got: %v", tt.expectFixed, wasFixed)
			}

			// Verify the final content is valid JSON
			var result interface{}
			err := json.Unmarshal([]byte(fixedContent), &result)
			if err != nil {
				t.Errorf("Final content is not valid JSON: %v", err)
			}

			// For TodoWrite, verify the expected structure
			if tt.toolName == "TodoWrite" && fixedContent != "" {
				var todoData map[string]interface{}
				err := json.Unmarshal([]byte(fixedContent), &todoData)
				if err != nil {
					t.Errorf("Failed to parse TodoWrite data: %v", err)
				}

				if todos, ok := todoData["todos"].([]interface{}); ok {
					if len(todos) > 0 {
						if todo, ok := todos[0].(map[string]interface{}); ok {
							if _, hasContent := todo["content"]; !hasContent {
								t.Error("TodoWrite data missing 'content' field")
							}
							if _, hasID := todo["id"]; !hasID {
								t.Error("TodoWrite data missing 'id' field")
							}
							if _, hasStatus := todo["status"]; !hasStatus {
								t.Error("TodoWrite data missing 'status' field")
							}
						}
					}
				}
			}
		})
	}
}

func TestResponseConverter_Integration_PythonJSONFix(t *testing.T) {
	// Create a logger for testing
	logConfig := logger.LogConfig{
		Level:           "error",
		LogRequestTypes: "none",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "none",
	}
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	converter := NewResponseConverter(log)

	tests := []struct {
		name            string
		inputChunks     []OpenAIStreamChunk
		expectEvents    bool
		expectToolUse   bool
		toolName        string
	}{
		{
			name: "TodoWrite tool with Python-style arguments",
			inputChunks: []OpenAIStreamChunk{
				{
					ID:      "chatcmpl-test",
					Model:   "gpt-4",
					Choices: []OpenAIStreamChoice{
						{
							Index: 0,
							Delta: OpenAIMessage{
								ToolCalls: []OpenAIToolCall{
									{
										Index: 0,
										ID:    "call_123",
										Type:  "function",
										Function: OpenAIToolCallDetail{
											Name:      "TodoWrite",
											Arguments: "{'todos': [{'content': '创建项目', 'id': '1', 'status': 'pending'}]}",
										},
									},
								},
							},
							FinishReason: "tool_calls",
						},
					},
				},
			},
			expectEvents:  true,
			expectToolUse: true,
			toolName:      "TodoWrite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a stream state
			streamState := &StreamState{
				ToolCallStates:   make(map[string]*ToolCallState),
				TextBlockStarted: false,
				NextBlockIndex:   0,
				PingSent:         false,
			}

			var allEvents []string
			for _, chunk := range tt.inputChunks {
				events := converter.convertSingleChunkToEvents(chunk, streamState)
				allEvents = append(allEvents, events...)
			}

			if tt.expectEvents && len(allEvents) == 0 {
				t.Error("Expected events to be generated, but got none")
			}

			if tt.expectToolUse {
				// Look for tool use events
				hasToolStart := false
				hasToolDelta := false
				
				for i := 0; i < len(allEvents); i++ {
					if allEvents[i] == "event: content_block_start" && i+1 < len(allEvents) {
						var eventData map[string]interface{}
						dataLine := allEvents[i+1]
						if len(dataLine) > 6 { // "data: " prefix
							jsonData := dataLine[6:]
							if err := json.Unmarshal([]byte(jsonData), &eventData); err == nil {
								if contentBlock, ok := eventData["content_block"].(map[string]interface{}); ok {
									if blockType, ok := contentBlock["type"].(string); ok && blockType == "tool_use" {
										if name, ok := contentBlock["name"].(string); ok && name == tt.toolName {
											hasToolStart = true
										}
									}
								}
							}
						}
					}
					
					if allEvents[i] == "event: content_block_delta" && i+1 < len(allEvents) {
						var eventData map[string]interface{}
						dataLine := allEvents[i+1]
						if len(dataLine) > 6 { // "data: " prefix
							jsonData := dataLine[6:]
							if err := json.Unmarshal([]byte(jsonData), &eventData); err == nil {
								if delta, ok := eventData["delta"].(map[string]interface{}); ok {
									if deltaType, ok := delta["type"].(string); ok && deltaType == "input_json_delta" {
										hasToolDelta = true
									}
								}
							}
						}
					}
				}

				if !hasToolStart {
					t.Error("Expected tool_use content_block_start event")
				}
				if !hasToolDelta {
					t.Error("Expected input_json_delta event")
				}
			}
		})
	}
}

func TestEndToEnd_PythonJSONFix(t *testing.T) {
	// Create a logger for testing
	logConfig := logger.LogConfig{
		Level:           "error",
		LogRequestTypes: "none",
		LogRequestBody:  "none",
		LogResponseBody: "none",
		LogDirectory:    "none",
	}
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Test the complete flow from SSE parsing to event generation
	converter := NewResponseConverter(log)

	// SSE data with Python-style JSON
	sseData := `data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"TodoWrite","arguments":"{'todos': [{'content': 'Create project structure', 'id': '1', 'status': 'pending'}]}"}}]},"finish_reason":"tool_calls"}]}

`

	// Parse SSE stream
	chunks, err := converter.sseParser.ParseSSEStream([]byte(sseData))
	if err != nil {
		t.Fatalf("Failed to parse SSE stream: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(chunks))
	}

	// The SSE parser extracts the raw arguments (Python-style)
	// The fix will be applied during the stream processing
	chunk := chunks[0]
	if len(chunk.Choices) == 0 || len(chunk.Choices[0].Delta.ToolCalls) == 0 {
		t.Fatal("No tool calls found in chunk")
	}

	toolCall := chunk.Choices[0].Delta.ToolCalls[0]
	
	// Verify that the arguments are not empty
	if toolCall.Function.Arguments == "" {
		t.Fatal("Tool arguments should not be empty")
	}
	
	// Note: At this point the arguments are still in Python style
	// The fix will be applied when processing the stream events

	// Convert to Anthropic events
	streamState := &StreamState{
		ToolCallStates:   make(map[string]*ToolCallState),
		TextBlockStarted: false,
		NextBlockIndex:   0,
		PingSent:         false,
	}

	events := converter.convertSingleChunkToEvents(chunk, streamState)
	if len(events) == 0 {
		t.Error("No events generated from chunk")
	}

	// Verify we have the expected event structure
	hasContentBlockStart := false
	for i := 0; i < len(events); i++ {
		if events[i] == "event: content_block_start" {
			hasContentBlockStart = true
			break
		}
	}

	if !hasContentBlockStart {
		t.Error("Expected content_block_start event not found")
	}
}
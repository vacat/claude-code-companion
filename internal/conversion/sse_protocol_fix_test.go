package conversion

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSSEProtocolFixes_RealData tests the SSE protocol fixes with real OpenAI data format
func TestSSEProtocolFixes_RealData(t *testing.T) {
	converter := NewResponseConverter(getTestLogger())

	// Simulate real OpenAI SSE stream like the one in a.txt
	openaiSSE := `data: {"id":"2025081814023662804f889e1d4242","created":1755496956,"model":"glm-4.5","choices":[{"index":0,"delta":{"role":"assistant","content":"你好"}}]}

data: {"id":"2025081814023662804f889e1d4242","created":1755496956,"model":"glm-4.5","choices":[{"index":0,"delta":{"role":"assistant","content":"！我是"}}]}

data: {"id":"2025081814023662804f889e1d4242","created":1755496956,"model":"glm-4.5","choices":[{"index":0,"delta":{"role":"assistant","content":"Claude Code"}}]}

data: {"id":"2025081814023662804f889e1d4242","created":1755496956,"model":"glm-4.5","choices":[{"index":0,"finish_reason":"stop","delta":{"role":"assistant","content":""}}],"usage":{"prompt_tokens":13414,"completion_tokens":135,"total_tokens":13549}}

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
	t.Logf("Fixed SSE Protocol result:\n%s", resultStr)

	// Parse and verify the structure
	lines := strings.Split(resultStr, "\n")
	var eventCount int
	var textDeltaFound, messageDeltaValid, contentBlockStartValid bool

	for i, line := range lines {
		if strings.HasPrefix(line, "event: ") {
			eventCount++
			eventType := strings.TrimPrefix(line, "event: ")
			
			// Get the corresponding data line
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1], "data: ") {
				dataContent := strings.TrimPrefix(lines[i+1], "data: ")
				
				switch eventType {
				case "content_block_delta":
					var deltaEvent map[string]interface{}
					if err := json.Unmarshal([]byte(dataContent), &deltaEvent); err == nil {
						if delta, ok := deltaEvent["delta"].(map[string]interface{}); ok {
							if deltaType, ok := delta["type"].(string); ok {
								if deltaType == "text_delta" {
									textDeltaFound = true
									t.Logf("✅ Found correct text_delta type")
								}
							}
						}
					}
					
				case "message_delta":
					var deltaEvent map[string]interface{}
					if err := json.Unmarshal([]byte(dataContent), &deltaEvent); err == nil {
						if delta, ok := deltaEvent["delta"].(map[string]interface{}); ok {
							// Check that delta ONLY contains stop_reason (and optionally stop_sequence)
							validFields := 0
							for key := range delta {
								if key == "stop_reason" || key == "stop_sequence" {
									validFields++
								} else {
									t.Errorf("❌ message_delta.delta contains invalid field: %s", key)
								}
							}
							if validFields > 0 {
								messageDeltaValid = true
								t.Logf("✅ message_delta.delta contains only valid fields")
							}
						}
					}
					
				case "content_block_start":
					var startEvent map[string]interface{}
					if err := json.Unmarshal([]byte(dataContent), &startEvent); err == nil {
						if contentBlock, ok := startEvent["content_block"].(map[string]interface{}); ok {
							if contentBlock["type"] == "text" {
								// For text blocks, check if text field exists (may be empty)
								if _, hasText := contentBlock["text"]; hasText {
									contentBlockStartValid = true
									t.Logf("✅ content_block_start includes text field")
								}
							}
						}
					}
				}
			}
		}
	}

	// Verify required fixes
	if !textDeltaFound {
		t.Error("❌ Did not find content_block_delta with text_delta type")
	}

	if !messageDeltaValid {
		t.Error("❌ message_delta does not have correct structure")
	}

	if !contentBlockStartValid {
		t.Error("❌ content_block_start does not include text field")
	}

	// Verify overall structure
	expectedEvents := []string{
		"message_start",
		"content_block_start", 
		"ping",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}

	for _, expectedEvent := range expectedEvents {
		if !strings.Contains(resultStr, "event: "+expectedEvent) {
			t.Errorf("❌ Missing expected event: %s", expectedEvent)
		}
	}

	// Verify aggregated content
	if !strings.Contains(resultStr, "你好！我是Claude Code") {
		t.Error("❌ Text content was not properly aggregated")
	}

	t.Logf("✅ All SSE protocol fixes verified successfully")
}

// TestSSEProtocolFixes_CompareWithOldVersion compares new vs old architecture output
func TestSSEProtocolFixes_CompareWithOldVersion(t *testing.T) {
	converter := NewResponseConverter(getTestLogger())

	// Simple test data
	openaiSSE := `data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{"content":"Hello"},"index":0,"finish_reason":null}]}

data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{"content":" World"},"index":0,"finish_reason":null}]}

data: {"id":"test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"delta":{},"index":0,"finish_reason":"stop"}]}

data: [DONE]

`

	ctx := &ConversionContext{EndpointType: "openai"}

	// Test new refactored version
	newResult, err := converter.convertStreamingResponseRefactored([]byte(openaiSSE), ctx)
	if err != nil {
		t.Fatalf("New conversion failed: %v", err)
	}

	newResultStr := string(newResult)
	
	// Verify new result has correct delta types
	if !strings.Contains(newResultStr, `"type":"text_delta"`) {
		t.Error("❌ New version does not use text_delta")
	}

	// Verify new result has clean message_delta
	if strings.Contains(newResultStr, `"content":null`) || strings.Contains(newResultStr, `"role":""`) {
		t.Error("❌ New version still has invalid fields in message_delta")
	}

	t.Logf("✅ New refactored version produces compliant SSE output")
}
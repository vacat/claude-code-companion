package conversion

import (
	"encoding/json"
	"fmt"
)

// ConvertAggregatedMessage converts an aggregated message to Anthropic event sequence
func (c *UnifiedConverter) ConvertAggregatedMessage(msg *AggregatedMessage) (*ConversionResult, error) {
	var events []AnthropicSSEEvent
	
	// Generate message start event
	messageStartEvent, err := c.generateMessageStartEvent(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate message start event: %w", err)
	}
	events = append(events, messageStartEvent)

	// Track block index for content coordination
	blockIndex := 0

	// Generate content events (text and tool calls)
	contentEvents, err := c.generateContentEvents(msg, &blockIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content events: %w", err)
	}
	events = append(events, contentEvents...)

	// Generate message end events
	messageEndEvents, err := c.generateMessageEndEvents(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate message end events: %w", err)
	}
	events = append(events, messageEndEvents...)

	result := &ConversionResult{
		Events: events,
		Metadata: ConversionMetadata{
			ProcessingNotes: fmt.Sprintf("Converted message with %d events", len(events)),
		},
	}

	if c.logger != nil {
		c.logger.Debug("Message conversion completed", map[string]interface{}{
			"message_id": msg.ID,
			"event_count": len(events),
			"has_text": len(msg.TextContent) > 0,
			"tool_calls": len(msg.ToolCalls),
		})
	}

	return result, nil
}

// generateMessageStartEvent creates the initial message_start event
func (c *UnifiedConverter) generateMessageStartEvent(msg *AggregatedMessage) (AnthropicSSEEvent, error) {
	// Create Anthropic response structure for message start
	anthropicResp := &AnthropicResponse{
		ID:         msg.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      msg.Model,
		Content:    []AnthropicContentBlock{}, // Will be populated by content events
		StopReason: "", // Will be set in message_delta
		Usage: &AnthropicUsage{
			InputTokens:  0, // Will be updated if usage is available
			OutputTokens: 0,
		},
	}

	// Update usage if available - for message_start, show input tokens but 0 output tokens
	if msg.Usage != nil {
		anthropicResp.Usage.InputTokens = msg.Usage.PromptTokens
		anthropicResp.Usage.OutputTokens = 0 // Always show 0 in message_start
	}

	messageStart := &AnthropicMessageStart{
		Type:    "message_start",
		Message: anthropicResp,
	}

	return AnthropicSSEEvent{
		Type: "message_start",
		Data: messageStart,
	}, nil
}

// generateContentEvents creates content block events for text and tool calls
func (c *UnifiedConverter) generateContentEvents(msg *AggregatedMessage, blockIndex *int) ([]AnthropicSSEEvent, error) {
	var events []AnthropicSSEEvent
	
	// Generate text content events if present
	if len(msg.TextContent) > 0 {
		textEvents, err := c.generateTextEvents(msg.TextContent, blockIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to generate text events: %w", err)
		}
		events = append(events, textEvents...)
	}

	// Generate tool call events
	for _, toolCall := range msg.ToolCalls {
		toolEvents, err := c.generateToolUseEvents(toolCall, blockIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to generate tool events for %s: %w", toolCall.Name, err)
		}
		events = append(events, toolEvents...)
	}

	// Add ping event after first content block (Anthropic requirement)
	if len(events) > 0 {
		pingEvent := AnthropicSSEEvent{
			Type: "ping",
			Data: map[string]interface{}{
				"type": "ping",
			},
		}
		// Insert ping after first content_block_start
		events = c.insertPingAfterFirstContentBlock(events, pingEvent)
	}

	return events, nil
}

// generateTextEvents creates events for text content
func (c *UnifiedConverter) generateTextEvents(textContent string, blockIndex *int) ([]AnthropicSSEEvent, error) {
	var events []AnthropicSSEEvent
	currentIndex := *blockIndex

	// Content block start (text should start with empty text field)
	contentBlock := &AnthropicContentBlockForStart{
		Type: "text",
		Text: "",  // 这个字段现在会被序列化，即使为空
	}

	startEvent := &AnthropicContentBlockStart{
		Type:         "content_block_start",
		Index:        currentIndex,
		ContentBlock: contentBlock,
	}

	events = append(events, AnthropicSSEEvent{
		Type: "content_block_start",
		Data: startEvent,
	})

	// Content block delta (send all text at once for simplicity)
	deltaEvent := &AnthropicContentBlockDelta{
		Type:  "content_block_delta",
		Index: currentIndex,
		Delta: &AnthropicContentBlock{
			Type: "text_delta", // FIXED: Was "text", should be "text_delta"
			Text: textContent,
		},
	}

	events = append(events, AnthropicSSEEvent{
		Type: "content_block_delta",
		Data: deltaEvent,
	})

	// Content block stop
	stopEvent := &AnthropicContentBlockStop{
		Type:  "content_block_stop",
		Index: currentIndex,
	}

	events = append(events, AnthropicSSEEvent{
		Type: "content_block_stop",
		Data: stopEvent,
	})

	*blockIndex++
	return events, nil
}

// generateToolUseEvents creates events for a tool call using streaming partial_json approach
func (c *UnifiedConverter) generateToolUseEvents(toolCall AggregatedToolCall, blockIndex *int) ([]AnthropicSSEEvent, error) {
	var events []AnthropicSSEEvent
	currentIndex := *blockIndex

	// Content block start for tool use - with empty input (streaming approach)
	contentBlock := &AnthropicContentBlockForStart{
		Type:  "tool_use",
		ID:    toolCall.ID,
		Name:  toolCall.Name,
		Input: json.RawMessage("{}"), // Empty input for streaming
		// Note: No Text field for tool_use type - Text field will be omitted
	}

	startEvent := &AnthropicContentBlockStart{
		Type:         "content_block_start",
		Index:        currentIndex,
		ContentBlock: contentBlock,
	}

	events = append(events, AnthropicSSEEvent{
		Type: "content_block_start",
		Data: startEvent,
	})

	// Generate partial_json delta events for the tool arguments
	if toolCall.Arguments != "" {
		// Split the JSON string into UTF-8 safe chunks for streaming effect
		jsonStr := toolCall.Arguments
		chunkSize := 10 // Use smaller chunk size for better streaming (in runes, not bytes)
		
		chunks := splitUTF8String(jsonStr, chunkSize)
		
		for _, chunk := range chunks {
			deltaEvent := &AnthropicContentBlockDelta{
				Type:  "content_block_delta",
				Index: currentIndex,
				Delta: &AnthropicContentBlock{
					Type:        "input_json_delta",
					PartialJSON: chunk,
				},
			}
			
			events = append(events, AnthropicSSEEvent{
				Type: "content_block_delta",
				Data: deltaEvent,
			})
		}
	}

	// Content block stop
	stopEvent := &AnthropicContentBlockStop{
		Type:  "content_block_stop",
		Index: currentIndex,
	}

	events = append(events, AnthropicSSEEvent{
		Type: "content_block_stop",
		Data: stopEvent,
	})

	*blockIndex++
	return events, nil
}

// generateMessageEndEvents creates the final message events
func (c *UnifiedConverter) generateMessageEndEvents(msg *AggregatedMessage) ([]AnthropicSSEEvent, error) {
	var events []AnthropicSSEEvent

	// Message delta with stop reason and usage information
	messageDelta := &AnthropicMessageDelta{
		Type: "message_delta",
		Delta: &AnthropicMessageDeltaContent{
			StopReason: msg.FinishReason,
			// StopSequence is omitted (nil) as it's not used in OpenAI responses
		},
	}

	// Add usage information if available (as sibling to delta) - show actual output tokens in message_delta
	if msg.Usage != nil {
		messageDelta.Usage = &AnthropicUsage{
			InputTokens:  msg.Usage.PromptTokens,
			OutputTokens: msg.Usage.CompletionTokens, // Show actual completion tokens in message_delta
		}
	}

	events = append(events, AnthropicSSEEvent{
		Type: "message_delta",
		Data: messageDelta,
	})

	// Message stop
	messageStop := &AnthropicMessageStop{
		Type: "message_stop",
	}

	events = append(events, AnthropicSSEEvent{
		Type: "message_stop",
		Data: messageStop,
	})

	return events, nil
}

// insertPingAfterFirstContentBlock inserts a ping event after the first content_block_start
func (c *UnifiedConverter) insertPingAfterFirstContentBlock(events []AnthropicSSEEvent, pingEvent AnthropicSSEEvent) []AnthropicSSEEvent {
	for i, event := range events {
		if event.Type == "content_block_start" {
			// Insert ping after this event
			result := make([]AnthropicSSEEvent, 0, len(events)+1)
			result = append(result, events[:i+1]...)
			result = append(result, pingEvent)
			result = append(result, events[i+1:]...)
			return result
		}
	}
	// If no content_block_start found, append at end
	return append(events, pingEvent)
}

// splitUTF8String safely splits a UTF-8 string into chunks without breaking multi-byte characters
func splitUTF8String(s string, chunkSize int) []string {
	if len(s) == 0 {
		return nil
	}
	
	var chunks []string
	runes := []rune(s)
	
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		chunks = append(chunks, chunk)
	}
	
	return chunks
}
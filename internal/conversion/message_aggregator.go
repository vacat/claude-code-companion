package conversion

import (
	"fmt"
	"strings"
)

// AggregateChunks aggregates OpenAI chunks into a complete message
func (a *MessageAggregator) AggregateChunks(chunks []OpenAIStreamChunk) (*AggregatedMessage, error) {
	if len(chunks) == 0 {
		return nil, NewConversionError("aggregation_error", "No chunks to aggregate", nil)
	}

	// Initialize aggregated message with metadata from first chunk
	firstChunk := chunks[0]
	aggregated := &AggregatedMessage{
		ID:           firstChunk.ID,
		Model:        firstChunk.Model,
		TextContent:  "",
		ToolCalls:    []AggregatedToolCall{},
		FinishReason: "",
		Usage:        nil,
	}

	// Track tool call states for aggregation
	toolCallStates := make(map[string]*AggregatedToolCall)

	// Process all chunks
	for _, chunk := range chunks {
		if err := a.processChunk(chunk, aggregated, toolCallStates); err != nil {
			return nil, fmt.Errorf("failed to process chunk: %w", err)
		}
	}

	// Finalize tool calls
	a.finalizeToolCalls(aggregated, toolCallStates)

	// Apply content fixes
	a.applyContentFixes(aggregated)

	if a.logger != nil {
		a.logger.Debug("Message aggregation completed", map[string]interface{}{
			"chunk_count": len(chunks),
			"text_length": len(aggregated.TextContent),
			"tool_calls": len(aggregated.ToolCalls),
			"finish_reason": aggregated.FinishReason,
		})
	}

	return aggregated, nil
}

// processChunk processes a single chunk and updates the aggregated message
func (a *MessageAggregator) processChunk(chunk OpenAIStreamChunk, aggregated *AggregatedMessage, toolCallStates map[string]*AggregatedToolCall) error {
	for _, choice := range chunk.Choices {
		// Process text content
		if choice.Delta.Content != nil {
			if contentStr, ok := choice.Delta.Content.(string); ok {
				aggregated.TextContent += contentStr
			}
		}

		// Process tool calls
		if len(choice.Delta.ToolCalls) > 0 {
			for _, toolCall := range choice.Delta.ToolCalls {
				a.processToolCall(toolCall, toolCallStates)
			}
		}

		// Update finish reason (last non-empty wins)
		if choice.FinishReason != "" {
			aggregated.FinishReason = choice.FinishReason
		}
	}

	// Update usage info (last non-nil wins)
	if chunk.Usage != nil {
		aggregated.Usage = chunk.Usage
	}

	return nil
}

// processToolCall processes a single tool call delta
func (a *MessageAggregator) processToolCall(toolCall OpenAIToolCall, toolCallStates map[string]*AggregatedToolCall) {
	var state *AggregatedToolCall
	var stateKey string
	
	// Handle case where subsequent chunks don't have ID (OpenAI streaming behavior)
	if toolCall.ID == "" {
		// Find existing tool call by index (should only have one active tool call per index)
		// In most cases, there will be only one tool call, so we can use the first one
		for key, existingState := range toolCallStates {
			if key != "" { // Skip any accidentally created empty-key states
				state = existingState
				stateKey = key
				break
			}
		}
		
		// If no existing state found, this is an error case - create one with a placeholder ID
		if state == nil {
			stateKey = fmt.Sprintf("tool_call_%d", toolCall.Index)
			state = &AggregatedToolCall{
				ID:        stateKey,
				Name:      "",
				Arguments: "",
			}
			toolCallStates[stateKey] = state
		}
	} else {
		// Normal case: tool call has ID
		stateKey = toolCall.ID
		var exists bool
		state, exists = toolCallStates[stateKey]
		if !exists {
			state = &AggregatedToolCall{
				ID:        toolCall.ID,
				Name:      "",
				Arguments: "",
			}
			toolCallStates[stateKey] = state
		}
	}

	// Update name (should only be set once)
	if toolCall.Function.Name != "" {
		state.Name = toolCall.Function.Name
	}

	// Append arguments
	if toolCall.Function.Arguments != "" {
		state.Arguments += toolCall.Function.Arguments
	}
}

// finalizeToolCalls converts tool call states to final array
func (a *MessageAggregator) finalizeToolCalls(aggregated *AggregatedMessage, toolCallStates map[string]*AggregatedToolCall) {
	for _, state := range toolCallStates {
		aggregated.ToolCalls = append(aggregated.ToolCalls, *state)
	}
}

// applyContentFixes applies content fixes to the aggregated message
func (a *MessageAggregator) applyContentFixes(aggregated *AggregatedMessage) {
	// Trim whitespace from text content
	aggregated.TextContent = strings.TrimSpace(aggregated.TextContent)

	// Apply finish reason mapping
	aggregated.FinishReason = a.mapFinishReason(aggregated.FinishReason)

	// Generate message ID if needed (following existing convention)
	if aggregated.ID == "" && len(aggregated.ID) > 0 {
		aggregated.ID = "msg_" + aggregated.ID
	} else if aggregated.ID != "" && !strings.HasPrefix(aggregated.ID, "msg_") {
		aggregated.ID = "msg_" + aggregated.ID
	}
}

// mapFinishReason maps OpenAI finish reasons to Anthropic format
func (a *MessageAggregator) mapFinishReason(openaiReason string) string {
	switch openaiReason {
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default: // including "stop" and others
		return "end_turn"
	}
}
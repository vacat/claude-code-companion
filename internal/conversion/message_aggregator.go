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

		// Process usage info from choice (some APIs put usage in choice rather than chunk level)
		if choice.Usage != nil {
			a.updateUsageInfo(aggregated, choice.Usage)
		}
	}

	// Update usage info with accumulation strategy from chunk level
	if chunk.Usage != nil {
		a.updateUsageInfo(aggregated, chunk.Usage)
	}

	return nil
}

// updateUsageInfo updates usage information with accumulation strategy
func (a *MessageAggregator) updateUsageInfo(aggregated *AggregatedMessage, usage *OpenAIUsage) {
	if aggregated.Usage == nil {
		// First usage info - initialize with a copy
		aggregated.Usage = &OpenAIUsage{
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		}
	} else {
		// Accumulate usage info
		// For prompt_tokens: use the latest non-zero value (usually consistent across chunks)
		if usage.PromptTokens > 0 {
			aggregated.Usage.PromptTokens = usage.PromptTokens
		}
		// For completion_tokens: accumulate (sum up incremental tokens)
		aggregated.Usage.CompletionTokens += usage.CompletionTokens
		// For total_tokens: use the latest non-zero value or calculate if needed
		if usage.TotalTokens > 0 {
			aggregated.Usage.TotalTokens = usage.TotalTokens
		} else if aggregated.Usage.PromptTokens > 0 && aggregated.Usage.CompletionTokens > 0 {
			aggregated.Usage.TotalTokens = aggregated.Usage.PromptTokens + aggregated.Usage.CompletionTokens
		}
	}
}

// processToolCall processes a single tool call delta
func (a *MessageAggregator) processToolCall(toolCall OpenAIToolCall, toolCallStates map[string]*AggregatedToolCall) {
	var state *AggregatedToolCall
	var stateKey string
	
	// Always use index-based key to avoid duplicates when ID is missing
	indexKey := fmt.Sprintf("index_%d", toolCall.Index)
	
	// Handle case where subsequent chunks don't have ID (OpenAI streaming behavior)
	if toolCall.ID == "" {
		// Find existing tool call by index - this is the correct approach for streaming
		var exists bool
		state, exists = toolCallStates[indexKey]
		
		if !exists {
			// Create new state using index as fallback ID
			stateKey = fmt.Sprintf("tool_call_%d", toolCall.Index)
			state = &AggregatedToolCall{
				ID:        stateKey,
				Name:      "",
				Arguments: "",
			}
			toolCallStates[indexKey] = state
		} else {
			stateKey = indexKey
		}
	} else {
		// Normal case: tool call has ID, but use consistent index-based key
		var exists bool
		state, exists = toolCallStates[indexKey]
		if !exists {
			state = &AggregatedToolCall{
				ID:        toolCall.ID, // Use the real ID when available
				Name:      "",
				Arguments: "",
			}
			toolCallStates[indexKey] = state // Store using index-based key to avoid duplicates
		}
		stateKey = indexKey
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
	// Create a slice to store tool calls with their indices
	type indexedToolCall struct {
		index    int
		toolCall AggregatedToolCall
	}
	
	var indexedCalls []indexedToolCall
	
	// Extract index from key and create indexed tool calls
	for key, state := range toolCallStates {
		// Extract index from key like "index_0", "index_1", etc.
		var index int
		if n, err := fmt.Sscanf(key, "index_%d", &index); n == 1 && err == nil {
			indexedCalls = append(indexedCalls, indexedToolCall{
				index:    index,
				toolCall: *state,
			})
		}
	}
	
	// Sort by index to maintain original order
	for i := 0; i < len(indexedCalls); i++ {
		for j := i + 1; j < len(indexedCalls); j++ {
			if indexedCalls[i].index > indexedCalls[j].index {
				indexedCalls[i], indexedCalls[j] = indexedCalls[j], indexedCalls[i]
			}
		}
	}
	
	// Add sorted tool calls to aggregated message
	for _, indexed := range indexedCalls {
		aggregated.ToolCalls = append(aggregated.ToolCalls, indexed.toolCall)
	}
}

// applyContentFixes applies content fixes to the aggregated message
func (a *MessageAggregator) applyContentFixes(aggregated *AggregatedMessage) {
	// Trim whitespace from text content
	aggregated.TextContent = strings.TrimSpace(aggregated.TextContent)

	// Apply finish reason mapping
	aggregated.FinishReason = a.mapFinishReason(aggregated.FinishReason)

	// Apply Python JSON fixes to tool call arguments, especially for TodoWrite
	for i := range aggregated.ToolCalls {
		toolCall := &aggregated.ToolCalls[i]
		
		// Check if this tool should have Python JSON fixing applied
		if a.pythonFixer != nil && a.pythonFixer.ShouldApplyFix(toolCall.Name, toolCall.Arguments) {
			if fixedArgs, wasFixed := a.pythonFixer.FixPythonStyleJSON(toolCall.Arguments); wasFixed {
				if a.logger != nil {
					a.logger.Debug("Applied Python JSON fix to tool arguments", map[string]interface{}{
						"tool_name": toolCall.Name,
						"tool_id": toolCall.ID,
						"original": toolCall.Arguments,
						"fixed": fixedArgs,
					})
				}
				toolCall.Arguments = fixedArgs
			}
		}
	}

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
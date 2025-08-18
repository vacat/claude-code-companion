package conversion

// convertStreamingResponseRefactored converts streaming response using the new aggregation-then-conversion architecture
func (c *ResponseConverter) convertStreamingResponseRefactored(openaiResp []byte, ctx *ConversionContext) ([]byte, error) {
	// 1. Parse SSE stream (keep existing logic)
	chunks, err := c.sseParser.ParseSSEStream(openaiResp)
	if err != nil {
		return nil, NewConversionError("sse_parse_error", "Failed to parse SSE stream", err)
	}

	if len(chunks) == 0 {
		return nil, NewConversionError("empty_stream", "No valid chunks found in SSE stream", nil)
	}

	// 2. Aggregate message
	aggregator := NewMessageAggregator(c.logger)
	aggregatedMsg, err := aggregator.AggregateChunks(chunks)
	if err != nil {
		return nil, NewConversionError("aggregation_error", "Failed to aggregate chunks", err)
	}

	// 3. Unified conversion
	converter := NewUnifiedConverter(c.logger)
	result, err := converter.ConvertAggregatedMessage(aggregatedMsg)
	if err != nil {
		return nil, NewConversionError("conversion_error", "Failed to convert aggregated message", err)
	}

	// 4. Build SSE output
	sseOutput := c.sseParser.BuildAnthropicSSEFromEvents(result.Events)

	if c.logger != nil {
		c.logger.Debug("Refactored streaming conversion completed", map[string]interface{}{
			"chunk_count": len(chunks),
			"event_count": len(result.Events),
			"output_size": len(sseOutput),
		})
	}

	return sseOutput, nil
}
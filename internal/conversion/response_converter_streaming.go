package conversion

import (
	"encoding/json"

	_ "claude-proxy/internal/logger"
)

// convertStreamingResponse 转换流式响应 - 处理完整的 SSE 流
func (c *ResponseConverter) convertStreamingResponse(openaiResp []byte, ctx *ConversionContext) ([]byte, error) {
	// 检查是否为SSE格式
	if !c.sseParser.ValidateSSEFormat(openaiResp) {
		// 如果不是SSE格式，尝试作为单个chunk处理（向后兼容）
		return c.convertSingleChunk(openaiResp, ctx)
	}

	// 解析完整的 SSE 流
	chunks, err := c.sseParser.ParseSSEStream(openaiResp)
	if err != nil {
		return nil, NewConversionError("sse_parse_error", "Failed to parse SSE stream", err)
	}

	if len(chunks) == 0 {
		return nil, NewConversionError("empty_stream", "No valid chunks found in SSE stream", nil)
	}

	// 初始化流状态
	if ctx.StreamState == nil {
		ctx.StreamState = &StreamState{
			ContentBlockIndex: 0,
			ToolCallStates:    make(map[string]*ToolCallState),
			NextBlockIndex:    0,
			TextBlockStarted:  false,
			MessageStarted:    false,
			PingSent:          false,
		}
	}

	var allEvents []string

	// 添加 message_start 事件（只在第一次）
	if !ctx.StreamState.MessageStarted {
		messageStartEvent := map[string]interface{}{
			"type": "message_start",
			"message": map[string]interface{}{
				"id":      "msg_" + chunks[0].ID, // 使用第一个chunk的ID
				"type":    "message",
				"role":    "assistant",
				"content": []interface{}{},
				"model":   chunks[0].Model,
				"stop_reason": nil,
				"stop_sequence": nil,
				"usage": map[string]interface{}{
					"input_tokens":        0,
					"output_tokens":       0,
					"cache_creation_input_tokens": 0,
					"cache_read_input_tokens":     0,
				},
			},
		}
		messageStartData, _ := json.Marshal(messageStartEvent)
		
		allEvents = append(allEvents, "event: message_start")
		allEvents = append(allEvents, "data: "+string(messageStartData))
		allEvents = append(allEvents, "")
		ctx.StreamState.MessageStarted = true
	}

	// 逐个转换每个chunk
	var finalStopReason string
	var finalUsage *OpenAIUsage
	
	for i, chunk := range chunks {
		events := c.convertSingleChunkToEvents(chunk, ctx.StreamState)
		allEvents = append(allEvents, events...)
		
		// 记录最后的 stop_reason 和 usage 信息
		for _, choice := range chunk.Choices {
			if choice.FinishReason != "" {
				finalStopReason = choice.FinishReason
			}
		}
		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}
		
		if c.logger != nil {
			c.logger.Debug("Converted chunk", map[string]interface{}{
				"chunk_index": i,
				"events_generated": len(events),
			})
		}
	}
	
	// 在所有chunks处理完后，统一发送结束事件
	if finalStopReason != "" {
		// 结束所有活跃的工具调用块
		for _, state := range ctx.StreamState.ToolCallStates {
			// 只有已经开始且未完成的工具调用才需要发送 content_block_stop
			if state.Started && !state.Completed {
				stopEvent := map[string]interface{}{
					"type":  "content_block_stop",
					"index": state.BlockIndex,
				}
				stopData, _ := json.Marshal(stopEvent)
				allEvents = append(allEvents, "event: content_block_stop")
				allEvents = append(allEvents, "data: "+string(stopData))
				allEvents = append(allEvents, "")
				state.Completed = true
			}
		}

		// 结束文本块（如果有）
		if ctx.StreamState.TextBlockStarted {
			stopEvent := map[string]interface{}{
				"type":  "content_block_stop",
				"index": 0,
			}
			stopData, _ := json.Marshal(stopEvent)
			allEvents = append(allEvents, "event: content_block_stop")
			allEvents = append(allEvents, "data: "+string(stopData))
			allEvents = append(allEvents, "")
		}

		// 转换 OpenAI 的 finish_reason 到 Anthropic 格式
		stopReason := "end_turn"  // 默认值
		if finalStopReason == "tool_calls" {
			stopReason = "tool_use"
		} else if finalStopReason == "length" {
			stopReason = "max_tokens"
		}
		// 其他情况（包括 "stop"）都映射为 "end_turn"
		
		// 发送 message_delta 事件（包含 stop_reason）
		messageDeltaEvent := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": stopReason,
				"stop_sequence": nil,
			},
		}
		
		// 如果有 usage 信息，则添加到 message_delta 中
		if finalUsage != nil {
			messageDeltaEvent["usage"] = map[string]interface{}{
				"input_tokens":  finalUsage.PromptTokens,
				"output_tokens": finalUsage.CompletionTokens,
			}
		}
		
		messageDeltaData, _ := json.Marshal(messageDeltaEvent)
		allEvents = append(allEvents, "event: message_delta")
		allEvents = append(allEvents, "data: "+string(messageDeltaData))
		allEvents = append(allEvents, "")

		// 发送最终的消息结束事件
		messageStopEvent := map[string]interface{}{
			"type": "message_stop",
		}
		stopData, _ := json.Marshal(messageStopEvent)
		allEvents = append(allEvents, "event: message_stop")
		allEvents = append(allEvents, "data: "+string(stopData))
		allEvents = append(allEvents, "")
	}

	// 重新组装成 SSE 格式
	result := c.sseParser.BuildAnthropicSSEStream(allEvents)

	if c.logger != nil {
		c.logger.Debug("Stream conversion completed", map[string]interface{}{
			"total_chunks": len(chunks),
			"total_events": len(allEvents),
			"output_size": len(result),
		})
	}

	return result, nil
}
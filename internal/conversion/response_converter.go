package conversion

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"claude-proxy/internal/logger"
)

// ResponseConverter 响应转换器 - 基于参考实现
type ResponseConverter struct {
	logger    *logger.Logger
	sseParser *SSEParser
}

// NewResponseConverter 创建响应转换器
func NewResponseConverter(logger *logger.Logger) *ResponseConverter {
	return &ResponseConverter{
		logger:    logger,
		sseParser: NewSSEParser(logger),
	}
}

// Convert 转换 OpenAI 响应为 Anthropic 格式
func (c *ResponseConverter) Convert(openaiResp []byte, ctx *ConversionContext, isStreaming bool) ([]byte, error) {
	if isStreaming {
		return c.convertStreamingResponse(openaiResp, ctx)
	}
	return c.convertNonStreamingResponse(openaiResp, ctx)
}

// convertNonStreamingResponse 转换非流式响应 - 基于参考实现
func (c *ResponseConverter) convertNonStreamingResponse(openaiResp []byte, ctx *ConversionContext) ([]byte, error) {
	// 解析 OpenAI 响应
	var in OpenAIResponse
	if err := json.Unmarshal(openaiResp, &in); err != nil {
		return nil, NewConversionError("parse_error", "Failed to parse OpenAI response", err)
	}

	if len(in.Choices) == 0 {
		return nil, errors.New("no choices in OpenAI response")
	}
	choice := in.Choices[0] // 只取 top-1，常见用法

	msg := choice.Message
	var blocks []AnthropicContentBlock

	// 文本
	switch ct := msg.Content.(type) {
	case string:
		if strings.TrimSpace(ct) != "" {
			blocks = append(blocks, AnthropicContentBlock{
				Type: "text",
				Text: ct,
			})
		}
	case []interface{}:
		// 如果上游返回了多模态数组（少见），这里只抽取 text
		b, _ := json.Marshal(ct)
		var parts []OpenAIMessageContent
		if err := json.Unmarshal(b, &parts); err == nil {
			var sb strings.Builder
			for _, p := range parts {
				if p.Type == "text" {
					sb.WriteString(p.Text)
				}
			}
			if s := strings.TrimSpace(sb.String()); s != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "text",
					Text: s,
				})
			}
		}
	}

	// 工具调用
	for _, tc := range msg.ToolCalls {
		blocks = append(blocks, AnthropicContentBlock{
			Type: "tool_use",
			ID:   tc.ID,
			Name: tc.Function.Name,
			// OpenAI.arguments 是 JSON 字符串；Anthropic.input 是原生 JSON
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	// 转换 OpenAI finish_reason 到 Anthropic stop_reason
	stopReason := "end_turn" // 默认值
	if choice.FinishReason == "tool_calls" {
		stopReason = "tool_use"
	} else if choice.FinishReason == "length" {
		stopReason = "max_tokens"
	}
	// 其他情况（包括 "stop"）都映射为 "end_turn"

	out := AnthropicResponse{
		Type:       "message",
		Role:       "assistant",
		Model:      in.Model,
		Content:    blocks,
		StopReason: stopReason,
	}
	if in.Usage != nil {
		out.Usage = &AnthropicUsage{
			InputTokens:  in.Usage.PromptTokens,
			OutputTokens: in.Usage.CompletionTokens,
		}
	}

	// 序列化结果
	result, err := json.Marshal(out)
	if err != nil {
		return nil, NewConversionError("marshal_error", "Failed to marshal Anthropic response", err)
	}

	if c.logger != nil {
		c.logger.Debug("Response conversion completed")
	}

	return result, nil
}

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

		// 发送 message_delta 事件（即使没有usage信息也要发送停止原因）
		// 转换 OpenAI 的 finish_reason 到 Anthropic 格式
		stopReason := "end_turn"  // 默认值
		if finalStopReason == "tool_calls" {
			stopReason = "tool_use"
		} else if finalStopReason == "length" {
			stopReason = "max_tokens"
		}
		// 其他情况（包括 "stop"）都映射为 "end_turn"
		
		messageDeltaEvent := map[string]interface{}{
			"type": "message_delta",
			"delta": map[string]interface{}{
				"stop_reason": stopReason,
				"stop_sequence": nil,
			},
		}
		
		// 如果有usage信息，添加到事件中
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

// convertSingleChunk 转换单个chunk（向后兼容）
func (c *ResponseConverter) convertSingleChunk(openaiResp []byte, ctx *ConversionContext) ([]byte, error) {
	// 解析单个流式chunk
	var chunk OpenAIStreamChunk
	if err := json.Unmarshal(openaiResp, &chunk); err != nil {
		return nil, NewConversionError("parse_error", "Failed to parse OpenAI stream chunk", err)
	}

	// 初始化流状态
	if ctx.StreamState == nil {
		ctx.StreamState = &StreamState{
			ContentBlockIndex: 0,
			ToolCallStates:    make(map[string]*ToolCallState),
			NextBlockIndex:    0,
		}
	}

	events := c.convertSingleChunkToEvents(chunk, ctx.StreamState)

	// 组合所有事件
	result := strings.Join(events, "\n")
	if len(events) > 0 {
		result += "\n"
	}

	if c.logger != nil {
		c.logger.Debug("Single chunk conversion completed")
	}

	return []byte(result), nil
}

// convertSingleChunkToEvents 将单个OpenAI chunk转换为Anthropic事件列表
func (c *ResponseConverter) convertSingleChunkToEvents(chunk OpenAIStreamChunk, streamState *StreamState) []string {
	var events []string

	for _, choice := range chunk.Choices {
		// 检查是否有工具调用即将开始
		hasToolName := false
		for _, tc := range choice.Delta.ToolCalls {
			if tc.Function.Name != "" {
				hasToolName = true
				break
			}
		}
		
		// 处理文本内容
		if choice.Delta.Content != nil {
			switch content := choice.Delta.Content.(type) {
			case string:
				if content != "" {
					// 如果即将有工具调用开始（有name），先结束文本块
					// 这样可以避免文本和工具调用混合在同一个块中
					if hasToolName && streamState.TextBlockStarted {
						stopEvent := map[string]interface{}{
							"type":  "content_block_stop",
							"index": 0,
						}
						stopData, _ := json.Marshal(stopEvent)
						events = append(events, "event: content_block_stop")
						events = append(events, "data: "+string(stopData))
						events = append(events, "")
						streamState.TextBlockStarted = false
						// 不继续处理文本，让工具调用优先
						return events
					}
					
					// 如果文本块还没开始，先发送 content_block_start
					if !streamState.TextBlockStarted {
						startEvent := map[string]interface{}{
							"type":  "content_block_start",
							"index": 0, // 文本块总是 index 0
							"content_block": map[string]interface{}{
								"type": "text",
								"text": "",
							},
						}
						startData, _ := json.Marshal(startEvent)
						events = append(events, "event: content_block_start")
						events = append(events, "data: "+string(startData))
						events = append(events, "")
						
						// 在第一个content_block_start后发送ping事件
						if !streamState.PingSent {
							events = append(events, "event: ping")
							events = append(events, "data: {}")
							events = append(events, "")
							streamState.PingSent = true
						}
						
						streamState.TextBlockStarted = true
						streamState.NextBlockIndex = 1 // 下一个块从 index 1 开始
					}

					// 发送文本增量
					deltaEvent := map[string]interface{}{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]interface{}{
							"type": "text_delta",
							"text": content,
						},
					}
					deltaData, _ := json.Marshal(deltaEvent)
					events = append(events, "event: content_block_delta")
					events = append(events, "data: "+string(deltaData))
					events = append(events, "")
				}
			}
		}

		// 处理工具调用
		for _, tc := range choice.Delta.ToolCalls {
			// 使用 tc.Index 作为key，因为在streaming中后续chunks可能没有ID
			indexKey := fmt.Sprintf("%d", tc.Index)
			state, exists := streamState.ToolCallStates[indexKey]
			if !exists {
				// 新的工具调用块 - 等待收集到工具名称后再发送 content_block_start
				blockIndex := streamState.NextBlockIndex
				state = &ToolCallState{
					BlockIndex:      blockIndex,
					ID:              tc.ID, // 记录ID（如果有的话）
					ArgumentsBuffer: "",
					JSONBuffer:      NewSimpleJSONBuffer(),
					Completed:       false,
					Started:         false,
					NameReceived:    false,
				}
				streamState.ToolCallStates[indexKey] = state
				streamState.NextBlockIndex++
			}

			// 更新ID（如果这个chunk包含ID的话）
			if tc.ID != "" {
				state.ID = tc.ID
			}

			// 处理工具名称
			if tc.Function.Name != "" {
				state.Name = tc.Function.Name
				state.NameReceived = true
				
				// 如果文本块仍在进行中，需要先结束文本块
				if streamState.TextBlockStarted {
					stopEvent := map[string]interface{}{
						"type":  "content_block_stop",
						"index": 0,
					}
					stopData, _ := json.Marshal(stopEvent)
					events = append(events, "event: content_block_stop")
					events = append(events, "data: "+string(stopData))
					events = append(events, "")
					streamState.TextBlockStarted = false
				}
				
				// 如果这是第一次收到工具名称，发送 content_block_start（包含 name）
				if !state.Started {
					startEvent := map[string]interface{}{
						"type":  "content_block_start",
						"index": state.BlockIndex,
						"content_block": map[string]interface{}{
							"type":  "tool_use",
							"id":    state.ID, // 使用记录的ID
							"name":  state.Name,
							"input": map[string]interface{}{},
						},
					}
					startData, _ := json.Marshal(startEvent)
					events = append(events, "event: content_block_start")
					events = append(events, "data: "+string(startData))
					events = append(events, "")
					
					// 在第一个content_block_start后发送ping事件
					if !streamState.PingSent {
						events = append(events, "event: ping")
						events = append(events, "data: {}")
						events = append(events, "")
						streamState.PingSent = true
					}
					
					state.Started = true
				}
			}
			
			// 处理参数增量
			if tc.Function.Arguments != "" {
				state.ArgumentsBuffer += tc.Function.Arguments
				
				// 使用简单JSON缓冲器处理参数增量
				state.JSONBuffer.AppendFragment(tc.Function.Arguments)
				
				// 只有在工具已经开始（有name）时才发送增量
				if state.Started && state.NameReceived {
					// 获取增量输出
					if incrementalContent, hasNewContent := state.JSONBuffer.GetIncrementalOutput(); hasNewContent && incrementalContent != "" {
						deltaEvent := map[string]interface{}{
							"type":  "content_block_delta",
							"index": state.BlockIndex,
							"delta": map[string]interface{}{
								"type": "input_json_delta",
								"partial_json": incrementalContent,
							},
						}
						deltaData, _ := json.Marshal(deltaEvent)
						events = append(events, "event: content_block_delta")
						events = append(events, "data: "+string(deltaData))
						events = append(events, "")
					}
				}
			}
		}

		// 不在这里处理结束事件，改为在最后统一处理
	}

	return events
}

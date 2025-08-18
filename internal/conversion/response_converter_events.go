package conversion

import (
	"encoding/json"
	"fmt"
)

// DEPRECATED: This file contains the old fragment-based event conversion logic.
// The new refactored architecture is in unified_converter.go and message_aggregator.go
// This file is kept for backward compatibility during migration.

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
					JSONBuffer:      NewSimpleJSONBufferWithFixer(c.logger),
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
				state.JSONBuffer.AppendFragmentWithFix(tc.Function.Arguments, state.Name)
				
				// 只有在工具已经开始（有name）时才发送增量
				if state.Started && state.NameReceived {
					// 获取修复后的增量输出
					if incrementalContent, hasNewContent := state.JSONBuffer.GetFixedIncrementalOutput(); hasNewContent && incrementalContent != "" {
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
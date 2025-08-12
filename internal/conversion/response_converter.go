package conversion

import (
	"encoding/json"
	"errors"
	"strings"

	"claude-proxy/internal/logger"
)

// ResponseConverter 响应转换器 - 基于参考实现
type ResponseConverter struct {
	logger *logger.Logger
}

// NewResponseConverter 创建响应转换器
func NewResponseConverter(logger *logger.Logger) *ResponseConverter {
	return &ResponseConverter{
		logger: logger,
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

	out := AnthropicResponse{
		Type:       "message",
		Role:       "assistant",
		Model:      in.Model,
		Content:    blocks,
		StopReason: choice.FinishReason, // "stop" | "tool_calls"...
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

// convertStreamingResponse 转换流式响应 - 基于参考实现
func (c *ResponseConverter) convertStreamingResponse(openaiResp []byte, ctx *ConversionContext) ([]byte, error) {
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

	var events []string

	for _, choice := range chunk.Choices {
		if choice.Delta.Content != nil {
			// 文本内容增量
			switch content := choice.Delta.Content.(type) {
			case string:
				if content != "" {
					// 如果是第一个文本块，先发送content_block_start
					if ctx.StreamState.ContentBlockIndex == 0 {
						startEvent := map[string]interface{}{
							"type":  "content_block_start",
							"index": 0,
							"content_block": map[string]interface{}{
								"type": "text",
								"text": "",
							},
						}
						startData, _ := json.Marshal(startEvent)
						events = append(events, "data: "+string(startData))
					}

					// 发送文本增量
					deltaEvent := map[string]interface{}{
						"type":  "content_block_delta",
						"index": ctx.StreamState.ContentBlockIndex,
						"delta": map[string]interface{}{
							"type": "text_delta",
							"text": content,
						},
					}
					deltaData, _ := json.Marshal(deltaEvent)
					events = append(events, "data: "+string(deltaData))
				}
			}
		}

		// 处理工具调用增量
		for _, tc := range choice.Delta.ToolCalls {
			state, exists := ctx.StreamState.ToolCallStates[tc.ID]
			if !exists {
				// 新的工具调用
				state = &ToolCallState{
					BlockIndex:      ctx.StreamState.NextBlockIndex,
					ID:              tc.ID,
					ArgumentsBuffer: "",
					Completed:       false,
				}
				ctx.StreamState.ToolCallStates[tc.ID] = state
				ctx.StreamState.NextBlockIndex++

				// 发送tool_use块开始事件
				startEvent := map[string]interface{}{
					"type":  "content_block_start",
					"index": state.BlockIndex,
					"content_block": map[string]interface{}{
						"type": "tool_use",
						"id":   tc.ID,
					},
				}
				startData, _ := json.Marshal(startEvent)
				events = append(events, "data: "+string(startData))
			}

			// 更新工具调用状态
			if tc.Function.Name != "" {
				state.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				state.ArgumentsBuffer += tc.Function.Arguments
			}

			// 发送工具调用增量
			if tc.Function.Name != "" || tc.Function.Arguments != "" {
				deltaEvent := map[string]interface{}{
					"type":  "content_block_delta",
					"index": state.BlockIndex,
					"delta": map[string]interface{}{
						"type": "input_json_delta",
					},
				}

				if tc.Function.Name != "" {
					deltaEvent["delta"].(map[string]interface{})["partial_json"] = `{"name":"` + tc.Function.Name + `"`
				}
				if tc.Function.Arguments != "" {
					deltaEvent["delta"].(map[string]interface{})["partial_json"] = tc.Function.Arguments
				}

				deltaData, _ := json.Marshal(deltaEvent)
				events = append(events, "data: "+string(deltaData))
			}
		}

		// 处理完成事件
		if choice.FinishReason != "" {
			// 结束所有活跃的内容块
			for _, state := range ctx.StreamState.ToolCallStates {
				if !state.Completed {
					stopEvent := map[string]interface{}{
						"type":  "content_block_stop",
						"index": state.BlockIndex,
					}
					stopData, _ := json.Marshal(stopEvent)
					events = append(events, "data: "+string(stopData))
					state.Completed = true
				}
			}

			// 如果有文本内容，结束文本块
			if ctx.StreamState.ContentBlockIndex == 0 {
				stopEvent := map[string]interface{}{
					"type":  "content_block_stop",
					"index": 0,
				}
				stopData, _ := json.Marshal(stopEvent)
				events = append(events, "data: "+string(stopData))
			}

			// 发送消息结束事件
			messageStopEvent := map[string]interface{}{
				"type": "message_stop",
			}
			stopData, _ := json.Marshal(messageStopEvent)
			events = append(events, "data: "+string(stopData))
		}
	}

	// 组合所有事件
	result := strings.Join(events, "\n")
	if len(events) > 0 {
		result += "\n"
	}

	if c.logger != nil {
		c.logger.Debug("Stream conversion completed")
	}

	return []byte(result), nil
}

// ConvertOpenAIStreamToAnthropic 将一组已收集的 OpenAI 流式片段组装成"完整的一条 Anthropic assistant 消息"
// 基于参考实现，但这里处理的是已收集的chunks而不是单个chunk
func ConvertOpenAIStreamToAnthropic(chunks []OpenAIStreamChunk) (AnthropicResponse, error) {
	if len(chunks) == 0 {
		return AnthropicResponse{}, errors.New("empty stream chunks")
	}
	model := chunks[0].Model

	var textBuf strings.Builder
	// tool_call_id -> builder
	type toolAgg struct {
		ID   string
		Name string
		Args strings.Builder
	}
	toolMap := map[string]*toolAgg{}
	var finishReason string

	for _, ck := range chunks {
		for _, c := range ck.Choices {
			if c.Delta.Content != nil {
				switch v := c.Delta.Content.(type) {
				case string:
					textBuf.WriteString(v)
				case []interface{}:
					// 如果是多模态 parts，这里仅抽取 text 增量
					b, _ := json.Marshal(v)
					var parts []OpenAIMessageContent
					if err := json.Unmarshal(b, &parts); err == nil {
						for _, p := range parts {
							if p.Type == "text" {
								textBuf.WriteString(p.Text)
							}
						}
					}
				}
			}
			// 工具增量（OpenAI 流式会逐步给出 name/arguments 片段）
			for _, tc := range c.Delta.ToolCalls {
				agg := toolMap[tc.ID]
				if agg == nil {
					agg = &toolAgg{ID: tc.ID}
					toolMap[tc.ID] = agg
				}
				if tc.Function.Name != "" {
					agg.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					agg.Args.WriteString(tc.Function.Arguments)
				}
			}
			if c.FinishReason != "" {
				finishReason = c.FinishReason
			}
		}
	}

	var blocks []AnthropicContentBlock
	if s := strings.TrimSpace(textBuf.String()); s != "" {
		blocks = append(blocks, AnthropicContentBlock{
			Type: "text",
			Text: s,
		})
	}
	for _, agg := range toolMap {
		// 合并后的 arguments 必须是合法 JSON；有些模型会分段打断 JSON，
		// 这里不强做修复，只是原样拼接；调用方可在执行前校验。
		blocks = append(blocks, AnthropicContentBlock{
			Type:  "tool_use",
			ID:    agg.ID,
			Name:  agg.Name,
			Input: json.RawMessage(agg.Args.String()),
		})
	}

	return AnthropicResponse{
		Type:       "message",
		Role:       "assistant",
		Model:      model,
		Content:    blocks,
		StopReason: finishReason,
	}, nil
}

// BuildAnthropicToolResultMessage 构造一条"user"消息，
// 用于把工具执行结果（来自你本地真正执行的 function）回给 Anthropic 模型。
// 这是上下游对齐的关键步骤（否则下一轮模型看不到工具结果）。
func BuildAnthropicToolResultMessage(toolCallID string, text string, isErr bool) AnthropicMessage {
	return AnthropicMessage{
		Role: "user",
		Content: []AnthropicContentBlock{
			{
				Type:      "tool_result",
				ToolUseID: toolCallID,
				Content: []AnthropicContentBlock{
					{Type: "text", Text: text},
				},
				IsError: &isErr,
			},
		},
	}
}
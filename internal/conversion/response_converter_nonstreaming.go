package conversion

import (
	"encoding/json"
	"errors"
	"strings"

	_ "claude-code-companion/internal/logger"
)

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
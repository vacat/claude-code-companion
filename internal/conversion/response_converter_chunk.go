package conversion

import (
	"encoding/json"
	"strings"

	_ "claude-proxy/internal/logger"
)

// DEPRECATED: This file contains the old single chunk conversion logic.
// The new refactored architecture is in response_converter_streaming_refactored.go
// This file is kept for backward compatibility during migration.

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
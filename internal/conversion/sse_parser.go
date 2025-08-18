package conversion

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"claude-proxy/internal/logger"
)

// SSEParser 处理 Server-Sent Events 流的解析和重组
type SSEParser struct {
	logger *logger.Logger
	fixer  *PythonJSONFixer
}

// NewSSEParser 创建新的 SSE 解析器
func NewSSEParser(logger *logger.Logger) *SSEParser {
	return &SSEParser{
		logger: logger,
		fixer:  NewPythonJSONFixer(logger),
	}
}

// ParseSSEStream 解析完整的 SSE 流，提取所有的 OpenAI 流式 chunks
func (p *SSEParser) ParseSSEStream(sseData []byte) ([]OpenAIStreamChunk, error) {
	var chunks []OpenAIStreamChunk
	scanner := bufio.NewScanner(bytes.NewReader(sseData))
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		
		// 处理 data: 行
		if strings.HasPrefix(line, "data: ") {
			dataContent := strings.TrimPrefix(line, "data: ")
			
			// 跳过 [DONE] 标记
			if dataContent == "[DONE]" {
				continue
			}
			
			// 尝试解析 JSON
			var chunk OpenAIStreamChunk
			if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
				// 尝试使用 Python JSON 修复器
				if fixedData, wasFixed := p.fixer.FixPythonStyleJSON(dataContent); wasFixed {
					if fixErr := json.Unmarshal([]byte(fixedData), &chunk); fixErr == nil {
						if p.logger != nil {
							p.logger.Debug("Successfully fixed and parsed Python-style JSON", map[string]interface{}{
								"original": dataContent,
								"fixed":    fixedData,
							})
						}
						chunks = append(chunks, chunk)
						continue
					} else {
						if p.logger != nil {
							p.logger.Debug("Fixed JSON still failed to parse", map[string]interface{}{
								"original": dataContent,
								"fixed":    fixedData,
								"error":    fixErr.Error(),
							})
						}
					}
				}
				
				if p.logger != nil {
					p.logger.Debug("Failed to parse SSE data chunk, skipping", map[string]interface{}{
						"data": dataContent,
						"error": err.Error(),
					})
				}
				continue
			}
			
			chunks = append(chunks, chunk)
		}
		// 其他 SSE 字段 (event:, id:, retry:) 在 OpenAI 流中不常用，暂时忽略
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning SSE stream: %w", err)
	}
	
	if p.logger != nil {
		p.logger.Debug("Successfully parsed SSE stream", map[string]interface{}{
			"total_chunks": len(chunks),
		})
	}
	
	return chunks, nil
}

// BuildAnthropicSSEStream 将 Anthropic 事件列表重新组装成 SSE 格式
func (p *SSEParser) BuildAnthropicSSEStream(events []string) []byte {
	var buffer bytes.Buffer
	
	for _, event := range events {
		buffer.WriteString(event)
		buffer.WriteString("\n")
	}
	
	// Anthropic 流式响应没有 [DONE] 标记，直接结束
	
	return buffer.Bytes()
}

// BuildAnthropicSSEFromEvents 将 AnthropicSSEEvent 数组转换为 SSE 格式
func (p *SSEParser) BuildAnthropicSSEFromEvents(events []AnthropicSSEEvent) []byte {
	var buffer bytes.Buffer
	
	for _, event := range events {
		// 序列化事件数据
		eventData, err := json.Marshal(event.Data)
		if err != nil {
			if p.logger != nil {
				p.logger.Debug("Failed to marshal event data", map[string]interface{}{
					"event_type": event.Type,
					"error": err.Error(),
				})
			}
			continue
		}
		
		// 写入 SSE 格式
		buffer.WriteString("event: " + event.Type + "\n")
		buffer.WriteString("data: " + string(eventData) + "\n")
		buffer.WriteString("\n")
	}
	
	return buffer.Bytes()
}

// ValidateSSEFormat 验证数据是否为有效的 SSE 格式
func (p *SSEParser) ValidateSSEFormat(data []byte) bool {
	// 简单检查是否包含 SSE 特征
	dataStr := string(data)
	return strings.Contains(dataStr, "data:") && 
		   (strings.Contains(dataStr, "text/event-stream") || 
		    strings.Contains(dataStr, "[DONE]") ||
		    len(strings.Split(dataStr, "\n")) > 1)
}
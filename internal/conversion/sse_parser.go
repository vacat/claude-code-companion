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

// ValidateSSEFormat 验证数据是否为有效的 SSE 格式
func (p *SSEParser) ValidateSSEFormat(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	
	dataStr := string(data)
	
	// 首先检查是否是有效的JSON，如果是JSON则不是SSE
	trimmed := strings.TrimSpace(dataStr)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		// 尝试解析JSON来确认
		var temp interface{}
		if json.Unmarshal(data, &temp) == nil {
			return false // 是有效的JSON，不是SSE
		}
	}
	
	// 检查SSE格式：必须有以"event: "或"data: "开头的行
	lines := strings.Split(dataStr, "\n")
	hasEventLine := false
	hasDataLine := false
	
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "event: ") {
			hasEventLine = true
		}
		if strings.HasPrefix(trimmedLine, "data: ") {
			hasDataLine = true
		}
	}
	
	// SSE格式必须至少有一个data:行
	return hasDataLine && (hasEventLine || strings.Contains(dataStr, "[DONE]") || len(lines) > 2)
}
package conversion

import (
	"encoding/json"
	"strings"
	
	"claude-proxy/internal/logger"
)

// SimpleJSONBuffer 简单的JSON缓冲器
// 专门处理OpenAI function.arguments的逐字符流式输出
type SimpleJSONBuffer struct {
	buffer           strings.Builder
	lastOutputLength int
	fixer            *PythonJSONFixer
	toolName         string
}

// NewSimpleJSONBuffer 创建新的JSON缓冲器
func NewSimpleJSONBuffer() *SimpleJSONBuffer {
	return &SimpleJSONBuffer{
		lastOutputLength: 0,
	}
}

// NewSimpleJSONBufferWithFixer 创建带有格式修复功能的JSON缓冲器
func NewSimpleJSONBufferWithFixer(logger *logger.Logger) *SimpleJSONBuffer {
	return &SimpleJSONBuffer{
		lastOutputLength: 0,
		fixer:           NewPythonJSONFixer(logger),
	}
}

// AppendFragment 添加新的arguments片段
func (b *SimpleJSONBuffer) AppendFragment(fragment string) {
	if fragment != "" {
		b.buffer.WriteString(fragment)
	}
}

// AppendFragmentWithFix 添加新的arguments片段，支持格式修复
func (b *SimpleJSONBuffer) AppendFragmentWithFix(fragment string, toolName string) {
	if fragment != "" {
		b.buffer.WriteString(fragment)
		b.toolName = toolName
	}
}

// SetToolName 设置当前工具名称
func (b *SimpleJSONBuffer) SetToolName(toolName string) {
	b.toolName = toolName
}

// GetIncrementalOutput 获取增量输出
// 返回 (incrementalContent, hasNewContent)
func (b *SimpleJSONBuffer) GetIncrementalOutput() (string, bool) {
	current := b.buffer.String()
	currentLength := len(current)
	
	if currentLength <= b.lastOutputLength {
		return "", false
	}
	
	// 返回自上次输出以来的新增内容
	newContent := current[b.lastOutputLength:]
	b.lastOutputLength = currentLength
	
	return newContent, true
}

// GetBufferedContent 获取当前缓冲的全部内容
func (b *SimpleJSONBuffer) GetBufferedContent() string {
	return b.buffer.String()
}

// IsValidJSON 检查当前缓冲内容是否为有效JSON
func (b *SimpleJSONBuffer) IsValidJSON() bool {
	content := b.buffer.String()
	if content == "" {
		return false
	}
	
	var js interface{}
	return json.Unmarshal([]byte(content), &js) == nil
}

// Reset 重置缓冲器
func (b *SimpleJSONBuffer) Reset() {
	b.buffer.Reset()
	b.lastOutputLength = 0
}

// GetSmartIncrementalOutput 智能增量输出
// 尝试在JSON边界处分割，避免输出破碎的JSON片段
func (b *SimpleJSONBuffer) GetSmartIncrementalOutput() (string, bool) {
	current := b.buffer.String()
	currentLength := len(current)
	
	if currentLength <= b.lastOutputLength {
		return "", false
	}
	
	newContent := current[b.lastOutputLength:]
	
	// 如果新内容很短（少于20个字符），先缓冲一下
	if len(newContent) < 20 {
		// 检查是否到达了一个相对完整的状态
		if strings.HasSuffix(current, `"`) || 
		   strings.HasSuffix(current, `,`) || 
		   strings.HasSuffix(current, `}`) ||
		   strings.HasSuffix(current, `]`) {
			// 在这些位置可以安全输出
			b.lastOutputLength = currentLength
			return newContent, true
		}
		
		// 如果缓冲区已经很大了，强制输出
		if len(newContent) > 100 {
			b.lastOutputLength = currentLength
			return newContent, true
		}
		
		// 否则继续缓冲
		return "", false
	}
	
	// 内容足够长，直接输出
	b.lastOutputLength = currentLength
	return newContent, true
}

// GetFixedBufferedContent 获取修复后的缓冲内容
func (b *SimpleJSONBuffer) GetFixedBufferedContent() string {
	content := b.buffer.String()
	if content == "" {
		return content
	}
	
	// 如果有修复器且需要修复
	if b.fixer != nil && b.fixer.ShouldApplyFix(b.toolName, content) {
		if fixed, wasFixed := b.fixer.FixPythonStyleJSON(content); wasFixed {
			return fixed
		}
	}
	
	return content
}

// tryFixPythonStyle 尝试修复当前缓冲内容中的Python风格格式
func (b *SimpleJSONBuffer) tryFixPythonStyle() bool {
	if b.fixer == nil {
		return false
	}
	
	content := b.buffer.String()
	if content == "" {
		return false
	}
	
	if b.fixer.ShouldApplyFix(b.toolName, content) {
		if fixed, wasFixed := b.fixer.FixPythonStyleJSON(content); wasFixed {
			// 替换缓冲区内容
			b.buffer.Reset()
			b.buffer.WriteString(fixed)
			// 重置输出长度以确保增量输出正确
			b.lastOutputLength = 0
			return true
		}
	}
	
	return false
}
package conversion

import (
	"encoding/json"
	"strings"
)

// SimpleJSONBuffer 简单的JSON缓冲器
// 专门处理OpenAI function.arguments的逐字符流式输出
type SimpleJSONBuffer struct {
	buffer           strings.Builder
	fixedBuffer      strings.Builder // 修复后的缓冲区
	lastOutputLength int
}

// NewSimpleJSONBuffer 创建新的JSON缓冲器
func NewSimpleJSONBuffer() *SimpleJSONBuffer {
	return &SimpleJSONBuffer{
		lastOutputLength: 0,
	}
}

// AppendFragment 添加新的arguments片段
func (b *SimpleJSONBuffer) AppendFragment(fragment string) {
	if fragment != "" {
		b.buffer.WriteString(fragment)
		// 重建修复后的缓冲区
		b.fixedBuffer.Reset()
		fixedContent := b.fixJSONFormat(b.buffer.String())
		b.fixedBuffer.WriteString(fixedContent)
	}
}

// GetIncrementalOutput 获取增量输出
// 返回 (incrementalContent, hasNewContent)
func (b *SimpleJSONBuffer) GetIncrementalOutput() (string, bool) {
	current := b.fixedBuffer.String() // 使用修复后的缓冲区
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
	b.fixedBuffer.Reset()
	b.lastOutputLength = 0
}

// GetSmartIncrementalOutput 智能增量输出
// 尝试在JSON边界处分割，避免输出破碎的JSON片段
func (b *SimpleJSONBuffer) GetSmartIncrementalOutput() (string, bool) {
	current := b.fixedBuffer.String() // 使用修复后的缓冲区
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

// FixJSONFormat 修复JSON格式问题
// 将Python dict格式的单引号转换为JSON格式的双引号
func (b *SimpleJSONBuffer) FixJSONFormat(content string) string {
	return b.fixJSONFormat(content)
}

// fixJSONFormat 修复JSON格式问题
// 将Python dict格式的单引号转换为JSON格式的双引号
// 正确处理字符串内部的转义字符
func (b *SimpleJSONBuffer) fixJSONFormat(content string) string {
	if content == "" {
		return content
	}
	
	// 更健壮的方法：逐字符解析，正确处理转义
	return b.convertPythonDictToJSON(content)
}

// convertPythonDictToJSON 将Python字典格式转换为JSON格式
// 正确处理转义字符：
// 1. Python中的 \' 转换为JSON中的 '  
// 2. Python中的 " 转换为JSON中的 \"
// 3. 其他转义字符保持不变
func (b *SimpleJSONBuffer) convertPythonDictToJSON(input string) string {
	if input == "" {
		return input
	}
	
	var result strings.Builder
	runes := []rune(input)
	length := len(runes)
	i := 0
	
	for i < length {
		ch := runes[i]
		
		if ch == '\'' {
			// 发现单引号，需要判断是否为字符串边界
			if b.isStringBoundary(runes, i) {
				// 这是字符串边界，转换为双引号
				result.WriteRune('"')
				i++
				
				// 解析字符串内容直到找到匹配的单引号
				stringContent := b.parseStringContent(runes, &i)
				
				// 转换字符串内容中的转义字符
				convertedContent := b.convertStringEscapes(stringContent)
				result.WriteString(convertedContent)
				
				// 写入结束的双引号
				result.WriteRune('"')
				// i 已经在 parseStringContent 中被更新，指向结束单引号的下一个位置
			} else {
				// 这不是字符串边界，直接写入
				result.WriteRune(ch)
				i++
			}
		} else {
			// 其他字符直接写入
			result.WriteRune(ch)
			i++
		}
	}
	
	return result.String()
}

// isStringBoundary 判断给定位置的单引号是否为字符串边界
// 通过检查前面的字符来判断：: ' 或 , ' 或 { ' 或 [ ' 等
func (b *SimpleJSONBuffer) isStringBoundary(runes []rune, pos int) bool {
	if pos == 0 {
		return true // 开头的单引号很可能是字符串边界
	}
	
	// 向前查找最近的非空白字符
	for i := pos - 1; i >= 0; i-- {
		ch := runes[i]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			continue // 跳过空白字符
		}
		
		// 检查是否为字符串边界的标识符
		switch ch {
		case ':', ',', '{', '[', '(':
			return true
		case '"', '\'':
			// 如果前面是引号，这个单引号可能不是边界
			return false
		default:
			// 其他字符，可能不是边界
			return false
		}
	}
	
	return true // 如果前面都是空白字符，认为是边界
}

// parseStringContent 解析单引号字符串的内容
// 更新 i 指向结束单引号的下一个位置
func (b *SimpleJSONBuffer) parseStringContent(runes []rune, i *int) string {
	var content strings.Builder
	length := len(runes)
	
	for *i < length {
		ch := runes[*i]
		
		if ch == '\'' {
			// 检查是否为转义的单引号
			if *i > 0 && runes[*i-1] == '\\' {
				// 这是转义的单引号，作为内容的一部分
				content.WriteRune(ch)
				*i++
			} else {
				// 这是结束的单引号
				*i++ // 跳过结束单引号
				break
			}
		} else {
			content.WriteRune(ch)
			*i++
		}
	}
	
	return content.String()
}

// convertStringEscapes 转换字符串内容中的转义字符
// Python -> JSON:
// \' -> '  (Python中转义的单引号在JSON中不需要转义)
// " -> \"  (Python中的双引号在JSON中需要转义)
// 其他转义字符保持不变
func (b *SimpleJSONBuffer) convertStringEscapes(content string) string {
	if content == "" {
		return content
	}
	
	var result strings.Builder
	runes := []rune(content)
	length := len(runes)
	
	for i := 0; i < length; i++ {
		ch := runes[i]
		
		if ch == '\\' && i+1 < length {
			nextCh := runes[i+1]
			if nextCh == '\'' {
				// Python中的 \' 转换为JSON中的 '
				result.WriteRune('\'')
				i++ // 跳过下一个字符
			} else {
				// 其他转义字符保持不变
				result.WriteRune(ch)
			}
		} else if ch == '"' {
			// Python中的双引号需要在JSON中转义
			result.WriteString("\\\"")
		} else {
			result.WriteRune(ch)
		}
	}
	
	return result.String()
}
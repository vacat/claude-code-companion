package conversion

import (
	"encoding/json"
	"strings"
)

// SimpleJSONBuffer 简单的JSON缓冲器
// 专门处理OpenAI function.arguments的逐字符流式输出
type SimpleJSONBuffer struct {
	buffer           strings.Builder
	lastOutputLength int
	
	// 新增：用于最终输出时的格式转换
	finalConversionEnabled bool
}

// NewSimpleJSONBuffer 创建新的JSON缓冲器
func NewSimpleJSONBuffer() *SimpleJSONBuffer {
	return &SimpleJSONBuffer{
		lastOutputLength:       0,
		finalConversionEnabled: false,
	}
}

// AppendFragment 添加新的arguments片段
func (b *SimpleJSONBuffer) AppendFragment(fragment string) {
	if fragment != "" {
		b.buffer.WriteString(fragment)
		// 不再每次都修复，而是原样保存
	}
}

// GetIncrementalOutput 获取增量输出
// 返回 (incrementalContent, hasNewContent)
func (b *SimpleJSONBuffer) GetIncrementalOutput() (string, bool) {
	current := b.buffer.String()
	currentLength := len(current)
	
	if currentLength <= b.lastOutputLength {
		return "", false
	}
	
	// 返回自上次输出以来的新增内容（原样，不修复）
	newContent := current[b.lastOutputLength:]
	b.lastOutputLength = currentLength
	
	return newContent, true
}

// GetBufferedContent 获取当前缓冲的全部内容
func (b *SimpleJSONBuffer) GetBufferedContent() string {
	return b.buffer.String()
}

// GetFinalJSON 获取最终的JSON格式内容
// 智能检测并转换Python格式为JSON格式
func (b *SimpleJSONBuffer) GetFinalJSON() string {
	content := b.buffer.String()
	if content == "" {
		return content
	}
	
	// 智能检测是否需要转换
	if b.isPythonDictFormat(content) {
		return b.convertPythonDictToJSON(content)
	}
	
	// 已经是JSON格式，直接返回
	return content
}

// isPythonDictFormat 检测是否为Python字典格式
// 通过检查是否包含单引号字符串来判断
func (b *SimpleJSONBuffer) isPythonDictFormat(content string) bool {
	if content == "" {
		return false
	}
	
	// 简单的启发式检测：
	// 1. 检查是否包含 'key': 模式
	// 2. 检查是否包含 : 'value' 模式
	// 3. 确保不是JSON中的单引号（JSON中单引号应该被转义）
	
	// 查找单引号字符串模式
	inString := false
	escaped := false
	
	runes := []rune(content)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		
		if escaped {
			escaped = false
			continue
		}
		
		if ch == '\\' {
			escaped = true
			continue
		}
		
		if ch == '"' {
			inString = !inString
			continue
		}
		
		// 如果不在双引号字符串内，检查是否有单引号字符串模式
		if !inString && ch == '\'' {
			// 检查前后文，判断是否为字符串边界
			if b.isStringBoundaryAtPos(runes, i) {
				return true
			}
		}
	}
	
	return false
}

// isStringBoundaryAtPos 检查指定位置的单引号是否为字符串边界
func (b *SimpleJSONBuffer) isStringBoundaryAtPos(runes []rune, pos int) bool {
	if pos == 0 || pos >= len(runes)-1 {
		return true
	}
	
	// 检查前面的字符
	for i := pos - 1; i >= 0; i-- {
		ch := runes[i]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			continue
		}
		
		switch ch {
		case ':', ',', '{', '[', '(':
			return true
		default:
			break
		}
		break
	}
	
	return false
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

// FixJSONFormat 修复JSON格式问题（保留用于向后兼容）
func (b *SimpleJSONBuffer) FixJSONFormat(content string) string {
	if b.isPythonDictFormat(content) {
		return b.convertPythonDictToJSON(content)
	}
	return content
}

// IsPythonDictFormat 检测是否为Python字典格式（公开方法）
func (b *SimpleJSONBuffer) IsPythonDictFormat(content string) bool {
	return b.isPythonDictFormat(content)
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
# TodoWrite 工具调用格式修正 - 技术实现文档

## 基于现有代码结构的实现方案

### 1. 问题定位

通过分析现有代码，发现问题出现在以下位置：

- **SSE 解析阶段**：`internal/conversion/sse_parser.go:48-57`，JSON 解析失败时直接跳过
- **工具调用处理**：`internal/conversion/response_converter_events.go:161-185`，参数追加到 SimpleJSONBuffer
- **JSON 缓冲处理**：`internal/conversion/simple_json_buffer.go`，当前没有格式修正机制

### 2. 实现策略

#### 2.1 分层修正架构

```
SSE 解析层 (sse_parser.go)
    ↓ JSON 解析失败时调用修正器
格式检测与修正层 (新增)
    ↓ 修正后重新解析
SimpleJSONBuffer 增强
    ↓ 运行时检测和修正
工具调用事件处理
```

#### 2.2 核心组件设计

##### A. PythonJSONFixer (新增)

```go
// 在 internal/conversion/ 目录下新增
type PythonJSONFixer struct {
    logger *logger.Logger
}

// FixPythonStyleJSON 修正 Python 风格的 JSON
func (f *PythonJSONFixer) FixPythonStyleJSON(input string) (string, bool) {
    // 返回 (修正后的JSON, 是否进行了修正)
}

// DetectPythonStyle 检测是否为 Python 风格字典
func (f *PythonJSONFixer) DetectPythonStyle(input string) bool {
    // 检测模式：包含单引号包围的键值对
}
```

##### B. 增强 SSE 解析器

在 `sse_parser.go` 中的 JSON 解析失败处添加修正逻辑：

```go
// 第 48-57 行的修改
if err := json.Unmarshal([]byte(dataContent), &chunk); err != nil {
    // 新增：尝试修正 Python 风格的 JSON
    if fixer := NewPythonJSONFixer(p.logger); fixer != nil {
        if fixedJSON, wasFixed := fixer.FixPythonStyleJSON(dataContent); wasFixed {
            if err2 := json.Unmarshal([]byte(fixedJSON), &chunk); err2 == nil {
                if p.logger != nil {
                    p.logger.Info("Successfully fixed Python-style JSON in SSE stream", map[string]interface{}{
                        "original": dataContent,
                        "fixed": fixedJSON,
                    })
                }
                chunks = append(chunks, chunk)
                continue
            }
        }
    }
    
    // 原有的错误处理逻辑
    if p.logger != nil {
        p.logger.Debug("Failed to parse SSE data chunk, skipping", ...)
    }
    continue
}
```

##### C. 增强 SimpleJSONBuffer

在 `simple_json_buffer.go` 中添加实时修正能力：

```go
// 新增方法
func (b *SimpleJSONBuffer) AppendFragmentWithFix(fragment string, toolName string) {
    if fragment != "" {
        b.buffer.WriteString(fragment)
        
        // 如果是 TodoWrite 工具，进行格式检查和修正
        if toolName == "TodoWrite" {
            if content := b.buffer.String(); len(content) > 20 {
                b.tryFixPythonStyle()
            }
        }
    }
}

func (b *SimpleJSONBuffer) tryFixPythonStyle() {
    content := b.buffer.String()
    fixer := NewPythonJSONFixer(nil)
    
    if fixedJSON, wasFixed := fixer.FixPythonStyleJSON(content); wasFixed {
        // 验证修正后的 JSON 是否有效
        var js interface{}
        if json.Unmarshal([]byte(fixedJSON), &js) == nil {
            // 替换缓冲内容
            b.buffer.Reset()
            b.buffer.WriteString(fixedJSON)
            // 保持 lastOutputLength 的相对位置
        }
    }
}
```

### 3. 智能修正算法

#### 3.1 Python 到 JSON 转换逻辑

```go
func (f *PythonJSONFixer) FixPythonStyleJSON(input string) (string, bool) {
    if !f.DetectPythonStyle(input) {
        return input, false
    }
    
    // 状态机算法
    var result strings.Builder
    var state ConversionState = OutsideString
    var depth int = 0
    
    runes := []rune(input)
    for i, r := range runes {
        switch state {
        case OutsideString:
            if r == '\'' {
                // 检查是否为结构性单引号（键名或值的开始）
                if f.isStructuralQuote(runes, i) {
                    result.WriteRune('"')  // 转换为双引号
                    state = InsideSingleQuotedString
                } else {
                    result.WriteRune(r)
                }
            } else {
                result.WriteRune(r)
                if r == '{' || r == '[' {
                    depth++
                } else if r == '}' || r == ']' {
                    depth--
                }
            }
            
        case InsideSingleQuotedString:
            if r == '\'' {
                result.WriteRune('"')  // 转换结束的单引号
                state = OutsideString
            } else if r == '"' {
                // 字符串内部的双引号需要转义
                result.WriteString(`\"`)
            } else {
                result.WriteRune(r)
            }
        }
    }
    
    return result.String(), true
}

type ConversionState int
const (
    OutsideString ConversionState = iota
    InsideSingleQuotedString
)

// isStructuralQuote 判断单引号是否为结构性引号
func (f *PythonJSONFixer) isStructuralQuote(runes []rune, pos int) bool {
    // 检查前面的字符，判断是否在键位置或值位置
    for i := pos - 1; i >= 0; i-- {
        r := runes[i]
        if r == '{' || r == ',' {
            return true  // 键的开始
        } else if r == ':' {
            return true  // 值的开始
        } else if r == ' ' || r == '\t' || r == '\n' {
            continue  // 跳过空白字符
        } else {
            return false
        }
    }
    return false
}
```

#### 3.2 检测算法

```go
func (f *PythonJSONFixer) DetectPythonStyle(input string) bool {
    // 检测关键模式
    patterns := []string{
        `'[^']*'\s*:\s*'[^']*'`,     // 'key': 'value'
        `{\s*'[^']*'\s*:`,           // {'key':
        `'\s*:\s*\[?\s*{`,           // ': [{
        `}\s*,\s*{\s*'`,             // }, {'
    }
    
    for _, pattern := range patterns {
        if matched, _ := regexp.MatchString(pattern, input); matched {
            return true
        }
    }
    
    // 额外检查：是否包含 TodoWrite 特有的模式
    if strings.Contains(input, `"todos"`) && 
       strings.Contains(input, `'content'`) && 
       strings.Contains(input, `'status'`) {
        return true
    }
    
    return false
}
```

### 4. 工具调用事件处理增强

在 `response_converter_events.go` 中修改第 165 行：

```go
// 原来的代码
// state.JSONBuffer.AppendFragment(tc.Function.Arguments)

// 修改为
state.JSONBuffer.AppendFragmentWithFix(tc.Function.Arguments, state.Name)
```

### 5. 配置和日志

#### 5.1 配置选项

```yaml
conversion:
  python_json_fixing:
    enabled: true
    target_tools: ["TodoWrite"]
    strict_validation: true
    debug_logging: false
    max_attempts: 3
```

#### 5.2 详细日志

```go
func (f *PythonJSONFixer) logFixingAttempt(original, fixed string, success bool) {
    if f.logger != nil {
        f.logger.Info("Python JSON fixing attempt", map[string]interface{}{
            "success":  success,
            "original_length": len(original),
            "fixed_length":   len(fixed),
            "original_sample": f.getSample(original, 100),
            "fixed_sample":   f.getSample(fixed, 100),
        })
    }
}
```

### 6. 测试策略

#### 6.1 单元测试文件

创建 `internal/conversion/python_json_fixer_test.go`：

```go
func TestPythonJSONFixer_BasicConversion(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected string
        shouldFix bool
    }{
        {
            name:     "Simple Python dict",
            input:    `{'content': 'test', 'id': '1'}`,
            expected: `{"content": "test", "id": "1"}`,
            shouldFix: true,
        },
        {
            name:     "TodoWrite format",
            input:    `{"todos": [{'content': '创建项目', 'id': '1', 'status': 'pending'}]}`,
            expected: `{"todos": [{"content": "创建项目", "id": "1", "status": "pending"}]}`,
            shouldFix: true,
        },
        {
            name:     "Valid JSON (no change)",
            input:    `{"content": "test", "id": "1"}`,
            expected: `{"content": "test", "id": "1"}`,
            shouldFix: false,
        },
    }
    
    fixer := NewPythonJSONFixer(nil)
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result, wasFixed := fixer.FixPythonStyleJSON(tc.input)
            assert.Equal(t, tc.shouldFix, wasFixed)
            assert.Equal(t, tc.expected, result)
            
            // 验证结果是有效的 JSON
            var js interface{}
            assert.NoError(t, json.Unmarshal([]byte(result), &js))
        })
    }
}
```

#### 6.2 集成测试

创建 `internal/conversion/integration_fix_test.go`：

```go
func TestSSEStreamWithPythonJSON(t *testing.T) {
    // 使用真实的 SSE 流数据进行测试
    sseData := `data: {"choices":[{"delta":{"tool_calls":[{"function":{"arguments":"{'content': '测试', 'id': '1'}"}}]}}]}
    
data: [DONE]`
    
    parser := NewSSEParser(nil)
    chunks, err := parser.ParseSSEStream([]byte(sseData))
    
    assert.NoError(t, err)
    assert.Len(t, chunks, 1)
    
    // 验证工具调用参数被正确解析
    args := chunks[0].Choices[0].Delta.ToolCalls[0].Function.Arguments
    var parsed map[string]interface{}
    assert.NoError(t, json.Unmarshal([]byte(args), &parsed))
}
```

### 7. 性能考虑

#### 7.1 优化策略

1. **按需触发**：仅对 JSON 解析失败的情况进行修正
2. **工具白名单**：仅对指定工具（如 TodoWrite）进行检测
3. **缓存检测结果**：避免重复检测相同内容
4. **增量处理**：在 SimpleJSONBuffer 中进行增量修正

#### 7.2 性能监控

```go
type FixingMetrics struct {
    TotalAttempts    int64
    SuccessfulFixes  int64
    FailedFixes     int64
    AverageTime     time.Duration
}

func (f *PythonJSONFixer) RecordMetrics(start time.Time, success bool) {
    duration := time.Since(start)
    // 记录性能指标
}
```

### 8. 错误处理和回退

#### 8.1 多重回退机制

```go
func (f *PythonJSONFixer) FixWithFallback(input string) string {
    // 尝试 1：智能转换
    if fixed, ok := f.FixPythonStyleJSON(input); ok {
        if f.validateJSON(fixed) {
            return fixed
        }
    }
    
    // 尝试 2：简单替换
    if fixed := f.simpleQuoteReplace(input); f.validateJSON(fixed) {
        return fixed
    }
    
    // 回退：返回原始数据
    return input
}
```

#### 8.2 错误监控

```go
func (f *PythonJSONFixer) handleFixingError(input string, err error) {
    if f.logger != nil {
        f.logger.Warn("Python JSON fixing failed", map[string]interface{}{
            "input_sample": f.getSample(input, 200),
            "error": err.Error(),
        })
    }
}
```

### 9. 部署和监控

#### 9.1 渐进式部署

1. **阶段 1**：添加检测和日志，不进行修正
2. **阶段 2**：启用修正，但记录所有操作
3. **阶段 3**：根据日志分析调优算法
4. **阶段 4**：完全启用并优化性能

#### 9.2 监控指标

- 修正成功率
- 处理延迟
- 错误率
- 影响的工具调用数量

这个实现方案充分利用了现有的代码架构，通过最小侵入性的修改来解决 Python 风格 JSON 的格式问题，同时保持了高性能和可靠性。
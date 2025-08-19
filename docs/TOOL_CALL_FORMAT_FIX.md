# TodoWrite 工具调用格式修正技术分析

## 问题描述

某些 OpenAI 兼容模型在调用 TodoWrite 工具时，返回的 `arguments` 字段使用了 Python 风格的字典语法而不是标准的 JSON 格式：

```json
// 错误格式（Python 风格）
{"todos": [{'content': '创建项目结构和主程序文件', 'id': '1', 'status': 'in_progress'}]}

// 正确格式（JSON 标准）
{"todos": [{"content": "创建项目结构和主程序文件", "id": "1", "status": "in_progress"}]}
```

这种格式错误会导致 JSON 解析失败，影响工具调用的正常执行。

## 技术分析

### 1. 问题根源

- **模型训练差异**：某些模型在训练时可能接触了更多 Python 代码，导致倾向于输出 Python 字典格式
- **SSE 流式输出**：错误格式在 Server-Sent Events 流中逐步构建，需要在流处理过程中检测和修正
- **OpenAI 接口兼容性**：问题出现在 OpenAI 兼容接口的响应中

### 2. 错误模式识别

从示例 SSE 流中可以观察到的错误模式：

```
tool_calls[0].function.arguments += "{'content': '"     // 单引号开始
tool_calls[0].function.arguments += "', 'id': '"        // 单引号分隔
tool_calls[0].function.arguments += "'}"                // 单引号结束
```

### 3. 检测策略

需要在以下时机进行检测：

1. **工具调用开始**：当检测到 `tool_calls` 且 `function.name` 为 "TodoWrite" 时
2. **参数累积过程**：在 SSE 流的 `arguments` 字段逐步构建时
3. **工具调用完成**：当 `finish_reason` 为 "tool_calls" 时进行最终验证

### 4. 修正算法设计

#### 4.1 智能引号转换

```go
// 伪代码
func fixPythonStyleJSON(input string) string {
    // 1. 检测是否包含 Python 风格的字典语法
    if containsPythonStyle(input) {
        // 2. 安全地将单引号转换为双引号
        return convertQuotes(input)
    }
    return input
}

func containsPythonStyle(input string) bool {
    // 检测模式：{'key': 'value'}
    patterns := []string{
        `{'[^']*':\s*'[^']*'}`,           // 单个键值对
        `'[^']*':\s*'[^']*'`,             // 键值对片段
        `\[{'[^']*':\s*'[^']*'`,          // 数组开始
    }
    // 使用正则表达式检测
}

func convertQuotes(input string) string {
    // 状态机算法，智能转换引号
    // 需要区分字符串内容中的引号和结构性引号
}
```

#### 4.2 状态机转换逻辑

```
状态1: 寻找键名开始 -> 发现 { 或 , -> 状态2
状态2: 处理键名 -> 发现 ' -> 状态3
状态3: 键名内容 -> 发现 ' -> 状态4
状态4: 寻找冒号 -> 发现 : -> 状态5
状态5: 寻找值开始 -> 发现 ' -> 状态6
状态6: 值内容 -> 发现 ' -> 状态1
```

### 5. 实现考虑

#### 5.1 性能优化

- **按需检测**：仅对 TodoWrite 工具调用进行检测
- **增量处理**：在 SSE 流中增量检测，避免重复处理
- **缓存机制**：对已验证正确的格式进行缓存

#### 5.2 安全性保障

- **格式验证**：修正后必须进行 JSON 有效性验证
- **内容完整性**：确保修正过程不会丢失或损坏数据
- **回退机制**：修正失败时使用原始数据

#### 5.3 兼容性处理

- **多模型支持**：兼容不同模型的输出格式差异
- **向后兼容**：不影响已经正确格式的工具调用
- **错误降级**：格式修正失败时的优雅降级

## 实现架构

### 1. 组件设计

```
SSE Stream Handler
    ↓
Tool Call Detector
    ↓
Format Validator
    ↓
Python-to-JSON Converter
    ↓
JSON Validator
    ↓
Response Forwarder
```

### 2. 代码结构

```
internal/
├── conversion/
│   ├── tool_call_fixer.go      // 主要修正逻辑
│   ├── quote_converter.go      // 引号转换算法
│   └── json_validator.go       // JSON 验证
├── detector/
│   ├── format_detector.go      // 格式问题检测
│   └── pattern_matcher.go      // 模式匹配
└── stream/
    ├── sse_processor.go        // SSE 流处理
    └── tool_call_tracker.go    // 工具调用跟踪
```

### 3. 配置选项

```yaml
format_fixing:
  enabled: true
  target_tools: ["TodoWrite"]  # 需要修正的工具列表
  strict_validation: true      # 严格 JSON 验证
  fallback_on_error: true     # 错误时回退到原始数据
  debug_logging: false        # 调试日志
```

## 测试策略

### 1. 单元测试

- **引号转换准确性**：验证各种 Python 格式到 JSON 的转换
- **边界条件处理**：嵌套结构、特殊字符、转义字符等
- **性能测试**：大量数据的处理性能

### 2. 集成测试

- **端到端流程**：完整的 SSE 流处理和修正
- **多模型兼容性**：不同 OpenAI 兼容模型的输出
- **错误场景**：各种异常情况的处理

### 3. 回归测试

- **正常格式保护**：确保不影响已正确的 JSON 格式
- **其他工具调用**：确保不影响非 TodoWrite 工具的调用

## 风险评估

### 1. 低风险

- **误修正**：将正确的 JSON 错误地修改
- **性能影响**：额外的检测和转换开销

### 2. 中风险

- **数据损坏**：修正过程中丢失或破坏数据内容
- **兼容性问题**：与某些特定模型的不兼容

### 3. 缓解措施

- **严格测试**：充分的单元测试和集成测试
- **可配置开关**：允许用户禁用此功能
- **详细日志**：记录所有修正操作以便调试
- **回退机制**：修正失败时使用原始数据

## 实施计划

1. **阶段1**：核心算法实现和单元测试
2. **阶段2**：SSE 流集成和端到端测试  
3. **阶段3**：配置选项和用户界面
4. **阶段4**：性能优化和生产部署

通过这个技术方案，可以有效解决 TodoWrite 工具调用的格式问题，提高系统的鲁棒性和用户体验。
# OpenAI 端点集成设计文档

## 概述

本文档描述了为 Claude Code Companion 项目添加 OpenAI 格式端点支持的详细设计方案。该功能允许代理服务将客户端发送的 Anthropic Messages API 请求转换为 OpenAI Chat Completions API 格式，并将响应转换回 Anthropic 格式。

## 目标

1. 支持将 Anthropic `/v1/messages` 请求转换为 OpenAI `/v1/chat/completions` 请求
2. 支持将 OpenAI 响应转换回 Anthropic 格式
3. 处理流式和非流式响应
4. 支持工具调用转换
5. 支持多模态内容转换
6. 与现有的模型重写、重试机制、标签系统集成
7. 保持 gzip 响应处理的兼容性

## 核心架构设计

### 1. 转换器组件结构

```
internal/
├── conversion/
│   ├── types.go              # 转换相关的类型定义
│   ├── anthropic_types.go    # Anthropic API 类型定义
│   ├── openai_types.go       # OpenAI API 类型定义
│   ├── converter.go          # 主转换器接口和实现
│   ├── request_converter.go  # 请求转换实现
│   ├── response_converter.go # 响应转换实现
│   └── stream_converter.go   # 流式响应转换实现
```

### 2. 转换器接口设计

```go
type Converter interface {
    // 转换请求
    ConvertRequest(anthropicReq []byte, endpointType string) ([]byte, *ConversionContext, error)

    // 转换响应
    ConvertResponse(openaiResp []byte, ctx *ConversionContext, isStreaming bool) ([]byte, error)

    // 检查是否需要转换
    ShouldConvert(endpointType string) bool
}

type ConversionContext struct {
    EndpointType    string                 // "anthropic" | "openai"
    ToolCallIDMap   map[string]string      // 工具调用ID映射
    IsStreaming     bool                   // 是否为流式请求
    RequestHeaders  map[string]string      // 原始请求头
    // 注意：不包含模型映射，因为转换发生在模型重写之后
}
```

## 详细实现方案

### 1. 请求转换 (Anthropic → OpenAI)

#### 核心转换逻辑

1. **基本字段映射**

   - `model`: 直接使用（转换发生在模型重写之后，已是目标模型名）
   - `max_tokens`: 直接映射到 `max_tokens`
   - `temperature`, `top_p`: 直接映射
   - `top_k`: 忽略（OpenAI 不支持）
   - `stop_sequences`: 映射到 `stop`
2. **System 消息处理**

   ```go
   // Anthropic 的 system 字段提升为 OpenAI 的首条 system 消息
   if anthropicReq.System != "" {
       openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
           Role: "system",
           Content: anthropicReq.System,
       })
   }
   ```
3. **消息转换**

   - 文本消息：直接转换
   - 图像消息：转换为 OpenAI 的 `image_url` 格式
   - 工具调用：转换为 OpenAI 的 `tool_calls` 格式
4. **工具定义转换**

   ```go
   for _, anthropicTool := range anthropicReq.Tools {
       openaiTool := OpenAITool{
           Type: "function",
           Function: OpenAIFunction{
               Name:        anthropicTool.Name,
               Description: anthropicTool.Description,
               Parameters:  anthropicTool.InputSchema,
           },
       }
       openaiReq.Tools = append(openaiReq.Tools, openaiTool)
   }
   ```

### 2. 响应转换 (OpenAI → Anthropic)

#### 非流式响应转换

1. **基本结构转换**

   ```go
   anthropicResp := AnthropicResponse{
       Type: "message",
       Role: "assistant",
       Content: []AnthropicContentBlock{},
   }
   ```
2. **内容转换**

   - 文本内容：包装为 `text` 类型的内容块
   - 工具调用：转换为 `tool_use` 类型的内容块
3. **停止原因映射**

   - `stop` → `end_turn`
   - `length` → `max_tokens`
   - 存在 `tool_calls` → `tool_use`

#### 流式响应转换

1. **事件序列转换**

   ```
   OpenAI 流式格式:
   data: {"choices":[{"delta":{"content":"Hello"}}]}
   data: [DONE]

   转换为 Anthropic 格式:
   event: message_start
   data: {"type":"message_start","message":{"role":"assistant","content":[]}}

   event: content_block_start
   data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

   event: content_block_delta
   data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

   event: content_block_stop
   data: {"type":"content_block_stop","index":0}

   event: message_stop
   data: {"type":"message_stop"}
   ```

### 3. 集成点设计

#### 在 proxy/handler.go 中的集成

```go
func (s *Server) proxyToEndpoint(...) (bool, bool) {
    // ... 现有代码 ...

    // 1. 模型重写（现有逻辑）
    originalModel, rewrittenModel, err := s.modelRewriter.RewriteRequest(tempReq, ep.ModelRewrite)

    // 2. 格式转换（新增）
    var conversionContext *conversion.ConversionContext
    if s.converter.ShouldConvert(ep.EndpointType) {
        convertedBody, ctx, err := s.converter.ConvertRequest(finalRequestBody, ep.EndpointType)
        if err != nil {
            // 处理转换错误
        }
        finalRequestBody = convertedBody
        conversionContext = ctx
    }

    // ... 发送请求 ...

    // 3. 响应处理
    if conversionContext != nil {
        // 转换响应格式
        convertedResp, err := s.converter.ConvertResponse(decompressedBody, conversionContext, isStreaming)
        if err != nil {
            // 处理转换错误
        }
        decompressedBody = convertedResp
    }

    // 4. 模型重写响应（现有逻辑）
    if originalModel != "" && rewrittenModel != "" {
        rewrittenResponseBody, err := s.modelRewriter.RewriteResponse(decompressedBody, originalModel, rewrittenModel)
        // ...
    }
}
```

### 4. 错误处理和降级策略

1. **转换失败处理**

   - 记录详细错误日志
   - 尝试下一个端点
   - 不影响现有的重试机制
2. **部分转换支持**

   - 对于不支持的特性（如 `thinking`），记录警告但继续处理
   - 对于关键转换失败，终止请求并尝试下一个端点

### 5. 配置扩展

#### 端点配置扩展

```yaml
endpoints:
  - name: "openai-endpoint"
    url: "https://api.openai.com"
    endpoint_type: "openai"  # 关键字段：指定端点类型
    auth_type: "auth_token"
    auth_value: "sk-..."
    # 模型映射通过现有的 model_rewrite 配置处理
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: "claude-3-5-sonnet-*"
          target_model: "gpt-4"
```

#### 全局转换配置

```yaml
conversion:
  enabled: true
  strict_validation: true
  # 模型映射不在此处配置，使用现有的 model_rewrite 功能
```

## 技术挑战和解决方案

### 1. Gzip 响应处理

**挑战**: 现有代码在 `proxyToEndpoint` 中处理 gzip 响应，转换后需要确保响应格式正确。

**解决方案**:

- 在转换前确保响应已解压
- 转换后移除 `Content-Encoding` 头部
- 更新 `Content-Length` 头部

### 2. 工具调用 ID 映射

**挑战**: Anthropic 和 OpenAI 的工具调用 ID 格式不同，需要维护映射关系。

**解决方案**:

- 在 `ConversionContext` 中维护 ID 映射表
- 确保请求和响应中的 ID 一致性

### 3. 流式响应实时转换

**挑战**: 流式响应需要实时转换事件格式，可能影响性能。

**解决方案**:

- 使用流式解析器，逐行处理
- 维护转换状态，确保事件序列正确
- 考虑缓冲机制以优化性能

### 4. 重试机制兼容性

**挑战**: 转换过程不应影响现有的端点重试机制。

**解决方案**:

- 转换器是无状态的，每次重试都重新转换
- 转换错误被视为端点失败，触发重试
- `ConversionContext` 不在重试间共享状态

## 实现阶段规划

### 阶段 1: 基础转换器实现

- [x] 创建转换器基础结构
- [x] 实现基本的请求/响应转换
- [x] 添加单元测试

### 阶段 2: 集成到代理处理流程

- [x] 在 `proxyToEndpoint` 中集成转换逻辑
- [x] 确保与模型重写的正确顺序
- [x] 处理错误场景
- [x] 添加路径转换支持
- [x] 添加配置验证

### 阶段 3: 高级特性支持

- [x] 基础流式响应转换框架
- [ ] 工具调用支持（需要进一步测试）
- [ ] 多模态内容支持（需要进一步测试）

### 阶段 4: 测试和优化

- [x] 单元测试
- [ ] 端到端测试
- [ ] 性能优化
- [ ] 错误处理完善

## 需要澄清的问题

1. **模型映射策略**:

   - 是否需要支持动态模型映射？
   - 如何处理未配置映射的模型？
   - 回答：这个实现里面不要处理模型映射，模型映射已经在之前的 rewrite model 功能里面实现了，这个转换发生在 rewrite model 之后，得到的是已经改写成需要的模型名的请求。
2. **错误处理粒度**:

   - 转换失败时是否应该完全跳过该端点？
   - 是否需要支持部分转换成功的场景？
   - 回答：暂时先完全跳过该端点，实际使用的时候发生错误再看日志是怎么回事
3. **性能考虑**:

   - 是否需要缓存转换结果以提高性能？
   - 流式转换的缓冲策略？
   - 回答：不需要缓存和缓冲，特别这个 proxy 目前对流式的处理是完全收下来再处理和解析的简单实现方式，这个我们这个版本不要去动它，不会有缓冲策略问题了
4. **配置管理**:

   - 模型映射配置是否应该支持热重载？
   - 回答：目前应该实现实际是支持热重载的，可以再确认一下
   - 是否需要提供 Web UI 来管理转换配置？
   - 回答：需要，但这个功能对外暴露的其实就只有一个 endpoint 类型，使用下拉框选择即可，选择是 anthropic 还是 openai 格式的
5. **监控和日志**:

   - 需要什么级别的转换过程日志？
   - 回答：这个目前没有概念，你先按照你想的来
   - 是否需要转换成功率的统计？
   - 回答：不强求，有就挺好，没有也能接受
6. **向后兼容性**:

   - 现有的 Anthropic 端点是否需要任何修改？
   - 回答：这个我没明白是什么意思，请展开说一下
   - 澄清：这里指的是现有配置为 `endpoint_type: "anthropic"` 的端点是否需要修改代码逻辑，答案是不需要，它们将继续使用现有的直通逻辑
   - 配置文件变更是否向后兼容？
   - 回答：之前应该已经做过了，配置文件现在应该已经可以指定 endpoint 的类型了
   - 确认：现有配置文件中缺少 `endpoint_type` 字段的端点将默认为 "anthropic" 类型

## 总结

此设计方案提供了一个全面的 OpenAI 格式支持实现框架，充分考虑了与现有系统的集成、错误处理、性能优化等关键问题。实现时需要特别注意转换顺序、错误处理和重试机制的兼容性。

建议按阶段实施，先实现基础转换功能，再逐步添加高级特性，确保每个阶段都有充分的测试覆盖。

## 基于反馈的设计调整

### 主要变更

1. **移除模型映射逻辑**: 转换器不处理模型映射，依赖现有的 model_rewrite 功能
2. **简化错误处理**: 转换失败直接跳过端点，触发重试机制
3. **保持流式处理简单**: 不修改现有的流式响应处理方式
4. **Web UI 集成**: 在端点管理界面添加 endpoint_type 下拉选择

### 技术细节确认结果

1. **配置热重载验证**: ✅ 当前实现支持配置热重载

2. **流式响应处理方式**: ✅ 现有代码完全接收后再处理，转换逻辑将遵循相同方式

3. **Web UI 集成点**: ✅ 在端点管理页面的添加/修改弹出对话框中添加 endpoint_type 下拉选择器

4. **认证头处理**: ✅ 不需要特殊处理，使用现有的 endpoint 认证配置逻辑，OpenAI 端点如配置为 api_key 类型将报错拒绝启动

5. **路径映射**: ✅ 按设计执行路径转换，注意处理 endpoint base URL 可能包含的路径前缀

### 实现优先级调整

基于反馈，建议实现顺序：

1. **阶段 1**: 基础请求/响应转换（不含流式）
2. **阶段 2**: 集成到代理处理流程
3. **阶段 3**: Web UI 支持
4. **阶段 4**: 流式转换支持
5. **阶段 5**: 工具调用和多模态支持

## 剩余需要澄清的实现细节

基于目前的反馈，设计已经相对完整，但还有几个实现细节需要确认：

### 1. 路径转换的具体实现

当前 `endpoint.GetFullURL()` 方法的实现：
```go
func (e *Endpoint) GetFullURL(path string) string {
    switch e.EndpointType {
    case "anthropic":
        return e.URL + "/v1" + path
    case "openai":
        return e.URL + "/v1" + path
    }
}
```

需要确认：
- 对于 OpenAI 端点，客户端发送 `/messages` 请求时，是否需要转换为 `/chat/completions` 路径？
- 还是在转换器中统一处理，保持 `GetFullURL` 方法不变？

### 2. 配置验证

需要在启动时验证：
- OpenAI 端点不能配置 `auth_type: "api_key"`，应该使用 `auth_type: "auth_token"`
- 这个验证应该在哪个组件中实现？（config 包还是 endpoint 包？）

### 3. 错误分类

转换过程中可能遇到的错误类型：
- JSON 解析错误（跳过端点）
- 不支持的字段/特性（记录警告，继续处理）
- 工具调用格式错误（跳过端点）

是否需要更细粒度的错误分类和处理策略？

### 4. 日志格式

转换过程的日志应该包含哪些信息：
- 转换前后的请求/响应大小？
- 转换耗时？
- 具体的转换操作（如工具调用数量、图片数量等）？

### 5. 测试策略

需要什么级别的测试：
- 单元测试：转换器的各个组件
- 集成测试：完整的请求转换流程
- 端到端测试：通过代理的完整请求/响应流程

如果这些细节都已经明确，设计文档就可以作为实施的完整蓝图了。

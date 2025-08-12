# Claude API Proxy 设计文档

本文档包含 Claude API Proxy 项目的详细实现规范和技术细节。基础架构信息请参考 [CLAUDE.md](./CLAUDE.md)。

## 错误处理和端点切换机制

### 故障检测逻辑

1. **实时故障检测**

   - 监控每个请求的响应状态
   - 在 10 秒滑动窗口内统计失败次数
   - 超过阈值（>1 次失败且全部失败）标记端点为不可用

2. **响应格式验证**

```go
type ResponseValidator struct{}

func (v *ResponseValidator) ValidateAnthropicResponse(body []byte) error {
    // 检查是否为有效的JSON
    var response map[string]interface{}
    if err := json.Unmarshal(body, &response); err != nil {
        return fmt.Errorf("invalid JSON response")
    }

    // 检查必要的字段结构
    if _, hasContent := response["content"]; hasContent {
        return nil // 正常响应
    }
    if _, hasError := response["error"]; hasError {
        return nil // 错误响应但格式正确
    }

    return fmt.Errorf("response format not compatible with Anthropic API")
}
```

3. **端点切换策略**

   - 按优先级顺序选择端点（priority 数值越小优先级越高）
   - 跳过标记为不可用的端点
   - 如果所有端点都不可用，返回 **502 Bad Gateway** 错误

4. **健康检查和恢复**

```go
func (h *HealthChecker) CheckEndpoint(endpoint *Endpoint) error {
    // 使用 /v1/models 端点进行健康检查
    req, _ := http.NewRequest("GET", endpoint.URL+endpoint.PathPrefix+"/models", nil)
    req.Header.Set("Authorization", endpoint.GetAuthHeader())
    req.Header.Set("anthropic-version", "2023-06-01")

    client := &http.Client{Timeout: time.Duration(endpoint.Timeout) * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // 检查响应状态码
    if resp.StatusCode >= 400 {
        return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
    }

    // 验证响应格式（简单检查是否包含models数组）
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("failed to read health check response: %v", err)
    }

    var modelsResp map[string]interface{}
    if err := json.Unmarshal(body, &modelsResp); err != nil {
        return fmt.Errorf("invalid JSON in health check response: %v", err)
    }

    if _, hasData := modelsResp["data"]; !hasData {
        return fmt.Errorf("invalid models response format")
    }

    return nil
}
```

5. **响应格式验证器**

```go
type ResponseValidator struct{}

// 验证标准JSON响应
func (v *ResponseValidator) ValidateStandardResponse(body []byte) error {
    var response map[string]interface{}
    if err := json.Unmarshal(body, &response); err != nil {
        return fmt.Errorf("invalid JSON response")
    }

    // 检查Anthropic响应必要字段
    requiredFields := []string{"id", "type", "role", "content", "model"}
    for _, field := range requiredFields {
        if _, exists := response[field]; !exists {
            return fmt.Errorf("missing required field: %s", field)
        }
    }

    // 验证type字段值
    if msgType, ok := response["type"].(string); !ok || msgType != "message" {
        return fmt.Errorf("invalid message type: expected 'message'")
    }

    // 验证role字段值
    if role, ok := response["role"].(string); !ok || role != "assistant" {
        return fmt.Errorf("invalid role: expected 'assistant'")
    }

    return nil
}

// 验证流式响应（SSE）
func (v *ResponseValidator) ValidateSSEChunk(chunk []byte) error {
    lines := bytes.Split(chunk, []byte("\n"))

    for _, line := range lines {
        line = bytes.TrimSpace(line)
        if len(line) == 0 {
            continue
        }

        if bytes.HasPrefix(line, []byte("event: ")) {
            eventType := string(line[7:])
            validEvents := []string{
                "message_start", "content_block_start", "ping",
                "content_block_delta", "content_block_stop", "message_stop",
            }

            valid := false
            for _, validEvent := range validEvents {
                if eventType == validEvent {
                    valid = true
                    break
                }
            }

            if !valid {
                return fmt.Errorf("invalid SSE event type: %s", eventType)
            }
        }

        if bytes.HasPrefix(line, []byte("data: ")) {
            dataContent := line[6:] // 跳过 "data: "
            if len(dataContent) == 0 {
                continue
            }

            var data map[string]interface{}
            if err := json.Unmarshal(dataContent, &data); err != nil {
                return fmt.Errorf("invalid JSON in SSE data: %v", err)
            }

            // 验证数据包含type字段
            if _, hasType := data["type"]; !hasType {
                return fmt.Errorf("missing 'type' field in SSE data")
            }
        }
    }

    return nil
}
```

6. **端点选择策略**

```go
type EndpointSelector struct {
    endpoints []*Endpoint
    mutex     sync.RWMutex
}

func (es *EndpointSelector) SelectEndpoint() (*Endpoint, error) {
    es.mutex.RLock()
    defer es.mutex.RUnlock()

    // 按优先级排序，选择第一个可用的端点
    availableEndpoints := make([]*Endpoint, 0)
    for _, ep := range es.endpoints {
        if ep.Enabled && ep.Status == "active" {
            availableEndpoints = append(availableEndpoints, ep)
        }
    }

    if len(availableEndpoints) == 0 {
        return nil, fmt.Errorf("no active endpoints available")
    }

    // 按优先级排序
    sort.Slice(availableEndpoints, func(i, j int) bool {
        return availableEndpoints[i].Priority < availableEndpoints[j].Priority
    })

    return availableEndpoints[0], nil
}
```

## Web 管理界面设计

### 页面结构

1. **主 Dashboard** (`/admin/`)

   - 端点状态概览
   - 请求统计图表
   - 最近错误日志

2. **端点配置页** (`/admin/endpoints`)

   - 端点列表和状态
   - 添加/编辑/删除端点
   - 手动启用/禁用端点
   - 测试端点连通性

3. **日志查看页** (`/admin/logs`)

   - 请求日志列表（分页）
   - 过滤器（失败请求、特定端点、时间范围）
   - 请求/响应详情查看，包括 header 和 body，注意 body 如果是 json 格式需要 pretty 化，流式响应可以不用把每条结果都 pretty，只要每行流式能正确换行即可

4. **系统设置页** (`/admin/settings`)

   - 服务器配置（端口、认证 token）
   - 日志配置
   - 健康检查配置
   - 配置文件导入/导出

### 界面功能特性

- 实时状态刷新（WebSocket 或 Server-Sent Events）
- 响应式设计，支持移动端查看
- 深色模式支持
- 请求日志搜索和过滤
- 配置变更确认机制

## Anthropic API 调研结论

### 1. 健康检查机制

- **调研结果**：Anthropic API 没有专门的健康检查端点
- **解决方案**：不进行健康检查，每次都认为此 endpoint 可用，只有尝试失败的时候才会将请求发送到下一个
- **重试策略**：失败后 60s 重试，配置可调

### 2. Anthropic API 响应格式

**标准响应格式**：

```json
{
  "id": "msg_01ABC...", // 消息唯一标识符
  "type": "message", // 固定值 "message"
  "role": "assistant", // 固定值 "assistant"
  "content": [
    // 内容数组，支持多种类型
    {
      "type": "text",
      "text": "实际回复内容"
    }
  ],
  "model": "claude-3-5-sonnet-20241022", // 使用的具体模型
  "stop_reason": "end_turn", // 停止原因: end_turn | max_tokens | stop_sequence
  "stop_sequence": null, // 触发停止的序列(如果有)
  "usage": {
    // token使用统计
    "input_tokens": 123,
    "output_tokens": 456
  }
}
```

**流式响应格式（SSE）**：

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_01...","content":[],...}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_stop
data: {"type":"message_stop"}
```

## 最终技术方案

基于调研结果，确定以下简化技术方案：

### 1. 健康检查

不做健康检查

### 2. 响应验证

- **标准响应**：验证 `id`, `type`, `content`, `model`, `usage` 必要字段
- **流式响应**：验证 SSE 格式和事件类型（`message_start`, `content_block_*`, `message_stop`）
- **异常处理**：格式不符合直接断开连接让客户端重连
- **内容解压**：自动解压 gzip 响应内容，移除压缩相关 HTTP 头部

### 3. 日志系统

- 持久化到 `./logs/` 目录
- 可通过配置开关关闭持久化
- 不实现轮转
- 不需要导出功能

### 4. Web 管理界面

- 无用户认证（本地访问）
- HTTP 协议（不需要 HTTPS）
- 配置修改后需要重启服务（不实现热更新）
- 监听 127.0.0.1（仅本地访问）

### 5. 性能设计

- 不设置并发限制
- 不实现请求限流
- 不设置内存日志上限

### 6. 错误处理

- 所有端点不可用返回 **502 Bad Gateway**
- 当端点返回非 200 错误时候，按照错误处理，将请求发给下一个端点。
- 支持端点优先级配置（按配置顺序）
- 端点恢复后直接使用，无需预热

### 7. 认证机制

- 本地 token 写死在配置文件中
- 不支持动态更新
- 仅监听 127.0.0.1，无需 IP 白名单
- 上游 API 认证信息明文存储

## 内容处理机制

### gzip 压缩内容处理

**问题背景**：上游 API 可能返回 gzip 压缩的响应内容，如果直接转发给客户端，会导致以下问题：

1. 客户端收到压缩内容但 HTTP 头部不一致，导致解析错误
2. 代理无法对压缩内容进行格式验证

**解决方案**：

```go
// 处理流程
1. 接收上游响应（可能是 gzip 压缩的）
2. 检测 Content-Encoding: gzip 头部
3. 如果是压缩内容：
   - 解压内容用于验证
   - 发送解压后的内容给客户端
   - 移除 Content-Encoding 头部
   - 更新 Content-Length 头部
4. 日志记录解压后的可读内容
```

**关键实现**：

- `decompressGzip()`: 解压 gzip 内容
- `getDecompressedBody()`: 智能检测并解压
- 双重处理：压缩内容用于验证，解压内容返回客户端
- 头部清理：移除 `Content-Encoding` 和 `Content-Length`

## 标签系统设计

### 标签系统概述

标签系统用于对 HTTP 请求进行分类和标记，以支持基于标签的路由、统计和管理功能。系统由以下核心组件组成：

1. **Tagger接口** - 定义标记规则的执行器
2. **TagRegistry** - 管理所有注册的标签和标记器  
3. **TaggerPipeline** - 并发执行所有标记器的管道
4. **TaggedRequest** - 包含标签信息的请求对象

### 标签注册机制

**重要变更**：从v2.0开始，系统允许多个tagger注册相同的tag名称，效果不叠加。

#### 注册规则

1. **Tagger唯一性**：每个tagger必须有唯一的名称，重复注册会返回错误
2. **Tag重复允许**：多个tagger可以注册相同的tag名称  
3. **Tag去重处理**：当多个tagger对同一请求打上相同标签时，最终结果中该标签只出现一次

#### 实现逻辑

```go
// TagRegistry.RegisterTagger 允许tag重复注册
func (tr *TagRegistry) RegisterTagger(tagger Tagger) error {
    // 检查tagger名称唯一性
    if _, exists := tr.taggers[name]; exists {
        return fmt.Errorf("tagger '%s' already registered", name)
    }
    
    // 自动注册tag（允许多个tagger使用相同tag）
    tag := tagger.Tag()
    if _, exists := tr.tags[tag]; !exists {
        tr.tags[tag] = &Tag{
            Name:        tag,
            Description: fmt.Sprintf("Tag from tagger '%s'", name),
        }
    }
    
    tr.taggers[name] = tagger
    return nil
}
```

#### 标签去重处理

在TaggerPipeline执行过程中，对相同标签进行去重：

```go
// TaggerPipeline.ProcessRequest 中的去重逻辑
if matched && err == nil {
    // 检查tag是否已存在，避免重复添加
    tagExists := false
    for _, existingTag := range tags {
        if existingTag == t.Tag() {
            tagExists = true
            break
        }
    }
    if !tagExists {
        tags = append(tags, t.Tag())
    }
}
```

### 使用场景示例

#### 场景1：多个AI模型检测器

```yaml
taggers:
  - name: "claude-detector-v1"
    type: "builtin"
    rule: "path-contains"
    value: "/v1/messages"  
    tag: "ai-request"
    
  - name: "claude-detector-v2"  
    type: "builtin"
    rule: "header-contains"
    value: "anthropic"
    tag: "ai-request"    # 与v1使用相同tag
```

**结果**：当请求同时匹配两个检测器时，最终只会有一个"ai-request"标签。

#### 场景2：不同维度的分类

```yaml  
taggers:
  - name: "model-classifier"
    tag: "claude-3"
    
  - name: "source-classifier"  
    tag: "web-ui"
    
  - name: "priority-classifier"
    tag: "high-priority"
```

**结果**：一个请求可能同时具有多个不同的标签：["claude-3", "web-ui", "high-priority"]

### 配置格式

标签系统配置添加到主配置文件中：

```yaml
tagging:
  enabled: true
  timeout_seconds: 5  # tagger执行超时
  
  # 内置标记器配置
  builtin_taggers:
    - name: "model-detector"
      type: "path-match" 
      pattern: "/v1/messages"
      tag: "anthropic-api"
      
    - name: "streaming-detector"
      type: "header-match"
      header: "accept"
      pattern: "text/event-stream"  
      tag: "streaming"

  # 自定义Starlark标记器
  custom_taggers:
    - name: "custom-classifier"
      script_file: "./taggers/classifier.star"
      tag: "custom-category"
```

### Web管理界面

标签管理页面 (`/admin/taggers`) 包含：

1. **标记器列表**
   - 显示所有已注册的tagger及其状态
   - 支持启用/禁用特定tagger
   - 显示每个tagger的执行统计

2. **标签统计**  
   - 展示各标签的使用频率
   - 标签相关的请求数量统计
   - 标签组合分析

3. **规则测试**
   - 提供请求样本测试功能
   - 实时预览标记结果
   - 调试tagger执行过程

这种设计既保持了tagger的独立性，又允许灵活的标签复用，满足复杂场景下的分类需求。
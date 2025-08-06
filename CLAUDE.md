# Claude API Proxy 项目开发规格书

## 项目概述

本项目是一个为 Claude Code 设计的本地 API 代理服务，主要解决以下问题：

1. **响应格式验证**：上游 API 有时返回 HTTP 200 但内容不符合 Anthropic 协议格式，代理需要检测并断开连接，让客户端重连
2. **多端点负载均衡**：支持配置多个上游 Anthropic 端点，提供故障切换和负载分发能力
3. **内容解压透传**：自动处理 gzip 压缩响应，解压后透传给客户端，确保客户端能正确解析

## 系统架构设计

### 核心组件

1. **HTTP 代理服务器** (`proxy/server.go`)

   - 监听本地端口，接收客户端请求
   - 本地认证（固定 authtoken）
   - 请求转发和响应处理

2. **端点管理器** (`endpoint/manager.go`)

   - 维护上游端点列表和状态
   - 端点选择策略（按配置顺序）
   - 故障检测和切换逻辑

3. **健康检查器** (`health/checker.go`)

   - 定期检查端点可用性
   - 恢复不可用端点状态
   - 健康状态缓存

4. **响应验证器** (`validator/response.go`)

   - 验证上游响应格式
   - Anthropic 协议兼容性检查
   - 异常响应处理

5. **Web 管理界面** (`web/admin.go`)

   - 端点配置管理
   - 请求日志查看
   - 系统状态监控

6. **日志系统** (`logger/logger.go`)

   - 请求/响应日志记录，注意日志不要有任何的截断，即使 body 很大，也要完整记录下来供页面展示
   - 错误日志和调试信息
   - 日志不需要轮转和清理

### 技术栈

- **语言**：Go 1.19+
- **Web 框架**：Gin (HTTP 服务) + 原生 net/http
- **前端界面**：HTML + JavaScript + Bootstrap（嵌入到二进制）
- **配置文件**：YAML 格式
- **日志库**：logrus 或 zap
- **数据存储**：内存 + 可选文件持久化

## API 设计

### 1. 代理 API

所有 Claude API 请求都通过代理转发：

```
Method: POST/GET/PUT/DELETE
Path: /v1/* (转发所有 v1 路径)
Headers:
  Authorization: Bearer <固定的本地token>
  其他原始头部信息
```

### 2. 管理 API

**获取端点状态**

```http
GET /admin/api/endpoints
Response: {
  "endpoints": [
    {
      "id": "endpoint-1",
      "url": "https://api.anthropic.com",
      "status": "active|inactive|checking",
      "lastCheck": "2025-01-01T12:00:00Z",
      "failureCount": 0,
      "totalRequests": 100,
      "successRequests": 95
    }
  ]
}
```

**更新端点配置**

```http
PUT /admin/api/endpoints
Request: {
  "endpoints": [
    {
      "url": "https://api.anthropic.com",
      "path_prefix": "/v1",
      "auth_type": "api_key", // "api_key" | "auth_token"
      "auth_value": "sk-xxx",
      "timeout": 30,
      "enabled": true
    }
  ]
}
```

**获取请求日志**

```http
GET /admin/api/logs?limit=100&offset=0&failed_only=false
Response: {
  "logs": [
    {
      "timestamp": "2025-01-01T12:00:00Z",
      "request_id": "req-12345",
      "endpoint": "https://api.anthropic.com",
      "method": "POST",
      "path": "/v1/messages",
      "status_code": 200,
      "duration_ms": 1500,
      "request_headers": {...},
      "request_body": "...",
      "response_headers": {...},
      "response_body": "...",
      "error": null
    }
  ],
  "total": 1000
}
```

## 配置文件结构

**config.yaml**

```yaml
server:
  port: 8080
  auth_token: "claude-proxy-token-2025" # 固定的本地认证token

endpoints:
  - name: "anthropic-primary"
    url: "https://api.anthropic.com"
    path_prefix: "/v1"
    auth_type: "api_key" # api_key | auth_token
    auth_value: "sk-ant-xxx"
    timeout_seconds: 30
    enabled: true
    priority: 1 # 端点优先级，数字越小优先级越高

  - name: "anthropic-backup"
    url: "https://backup.anthropic.com"
    path_prefix: ""
    auth_type: "auth_token"
    auth_value: "bearer-token-xxx"
    timeout_seconds: 30
    enabled: true
    priority: 2

health_check:
  enabled: true
  endpoint: "/v1/models" # 使用models端点进行健康检查
  interval_seconds: 60 # 健康检查间隔
  timeout_seconds: 10 # 单次检查超时时间
  failure_threshold: 2 # 10秒内超过1次失败标记不可用
  recovery_threshold: 2 # 连续2次成功检查恢复可用
  retry_backoff:
    initial_seconds: 60 # 初始重试间隔
    max_seconds: 600 # 最大重试间隔
    multiplier: 2.0 # 退避倍数

logging:
  level: "info" # debug | info | warn | error
  log_failed_requests: true
  log_request_body: true
  log_response_body: true
  persist_to_disk: true # 是否持久化到磁盘
  log_directory: "./logs" # 日志目录
  max_file_size: "100MB" # 单个日志文件最大大小（建议值）

validation:
  strict_anthropic_format: true # 严格验证Anthropic响应格式
  validate_streaming: true # 验证流式响应格式
  disconnect_on_invalid: true # 无效响应时断开连接

web_admin:
  enabled: true
  host: "127.0.0.1" # 仅本地访问
  port: 8081
  # 无需认证配置
```

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

## 项目结构

```
claude-proxy/
├── cmd/
│   └── main.go                 # 程序入口
├── internal/
│   ├── config/
│   │   ├── config.go           # 配置结构和加载
│   │   └── config.yaml         # 默认配置文件
│   ├── proxy/
│   │   ├── server.go           # HTTP代理服务器
│   │   ├── handler.go          # 请求处理逻辑
│   │   └── middleware.go       # 认证等中间件
│   ├── endpoint/
│   │   ├── manager.go          # 端点管理器
│   │   ├── endpoint.go         # 端点数据结构
│   │   └── selector.go         # 端点选择策略
│   ├── health/
│   │   ├── checker.go          # 健康检查器
│   │   └── monitor.go          # 故障监控
│   ├── validator/
│   │   └── response.go         # 响应格式验证
│   ├── logger/
│   │   ├── logger.go           # 日志系统
│   │   └── storage.go          # 日志存储
│   └── web/
│       ├── admin.go            # Web管理接口
│       ├── handlers.go         # Web处理函数
│       └── static/             # 静态文件
├── web/                        # 前端资源
│   ├── templates/
│   ├── static/
│   └── assets/
├── config.yaml                 # 配置文件
├── go.mod
├── go.sum
├── Makefile                    # 构建脚本
└── README.md
```

## 部署和运行

### 构建

```bash
make build          # 构建二进制文件
make build-linux    # 交叉编译Linux版本
make build-windows  # 交叉编译Windows版本
```

### 运行

```bash
./claude-proxy -config config.yaml
# 或
./claude-proxy --port 8080 --admin-port 8081
```

## 调研结果和最终技术方案

### Anthropic API 调研结论

**1. 健康检查机制**

- **调研结果**：Anthropic API 没有专门的健康检查端点
- **解决方案**：不进行健康检查，每次都认为此 endpoint 可用，只有尝试失败的时候才会将请求发送到下一个
- **重试策略**：失败后 60s 重试，配置可调

**2. Anthropic API 响应格式**

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

### 最终技术方案

基于你的回答和调研结果，确定以下简化技术方案：

**1. 健康检查**

    不做健康检查

**2. 响应验证**

- **标准响应**：验证 `id`, `type`, `content`, `model`, `usage` 必要字段
- **流式响应**：验证 SSE 格式和事件类型（`message_start`, `content_block_*`, `message_stop`）
- **异常处理**：格式不符合直接断开连接让客户端重连
- **内容解压**：自动解压 gzip 响应内容，移除压缩相关 HTTP 头部

**3. 日志系统**

- 持久化到 `./logs/` 目录
- 可通过配置开关关闭持久化
- 不实现轮转
- 不需要导出功能

**4. Web 管理界面**

- 无用户认证（本地访问）
- HTTP 协议（不需要 HTTPS）
- 配置修改后需要重启服务（不实现热更新）
- 监听 127.0.0.1（仅本地访问）

**5. 性能设计**

- 不设置并发限制
- 不实现请求限流
- 不设置内存日志上限

**6. 错误处理**

- 所有端点不可用返回 **502 Bad Gateway**
- 当端点返回非 200 错误时候，按照错误处理，将请求发给下一个端点。
- 支持端点优先级配置（按配置顺序）
- 端点恢复后直接使用，无需预热

**7. 认证机制**

- 本地 token 写死在配置文件中
- 不支持动态更新
- 仅监听 127.0.0.1，无需 IP 白名单
- 上游 API 认证信息明文存储

## 开发准备就绪

基于以上调研和你的具体需求，技术规格书已完善：

**✅ 已确定的技术要点**：

- Anthropic API 响应格式验证规范明确
- 简化版本设计，去除不必要的复杂功能
- 502 错误响应，优先级端点切换
- 本地监听，无复杂认证机制

**📋 技术实现清单**：

- [x] 系统架构设计
- [x] API 接口定义
- [x] 配置文件结构
- [x] 错误处理机制
- [x] 响应验证逻辑
- [x] 健康检查策略
- [x] Web 管理界面设计

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

现在可以开始具体的编码实现工作。

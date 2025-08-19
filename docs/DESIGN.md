# Claude Code Companion 设计文档

本文档包含 Claude Code Companion 项目的详细实现规范和技术细节。基础架构信息请参考 [CLAUDE.md](./CLAUDE.md)。

## 系统架构概览

Claude Code Companion 是一个多协议 API 代理服务，支持：

- **多端点类型**：Anthropic API、OpenAI API 等
- **格式转换**：OpenAI 格式请求自动转换为 Anthropic 格式  
- **OAuth 认证**：支持自动 token 刷新机制
- **模型重写**：动态重写请求中的模型名称
- **标签路由**：基于请求特征的智能路由
- **多语言界面**：Web 管理界面支持国际化
- **健康检查**：端点状态监控和故障转移

## 核心组件架构

### 1. 端点管理器 (endpoint.Manager)

负责管理所有上游端点，支持多种端点类型：

- **Anthropic 端点**：直接代理 Anthropic API
- **OpenAI 端点**：自动格式转换，支持 OpenAI 兼容 API
- **优先级路由**：按配置优先级选择可用端点
- **健康检查**：定期检查端点状态，自动故障转移

### 2. 格式转换器 (conversion.Converter)

实现不同 API 格式之间的转换：

```go
type Converter interface {
    ConvertRequest(body []byte, headers map[string]string) ([]byte, map[string]string, error)
    ConvertResponse(body []byte, headers map[string]string, isStreaming bool) ([]byte, map[string]string, error)
    SupportsStreaming() bool
}
```

**转换流程**：
1. 检测请求格式（OpenAI vs Anthropic）
2. 转换请求体和头部
3. 发送到上游端点
4. 转换响应格式返回客户端

### 3. OAuth 认证管理 (oauth.OAuth)

支持自动 token 刷新机制：

```go
type OAuthConfig struct {
    AccessToken  string `yaml:"access_token"`
    RefreshToken string `yaml:"refresh_token"`
    ExpiresAt    int64  `yaml:"expires_at"`
    TokenURL     string `yaml:"token_url"`
    ClientID     string `yaml:"client_id"`
    AutoRefresh  bool   `yaml:"auto_refresh"`
}
```

**刷新逻辑**：
- 检查 token 过期时间
- 自动刷新即将过期的 token
- 更新配置文件中的新 token

### 4. 模型重写器 (modelrewrite.Rewriter)

动态重写请求中的模型名称：

```go
type ModelRewriteConfig struct {
    Enabled bool                `yaml:"enabled"`
    Rules   []ModelRewriteRule  `yaml:"rules"`
}

type ModelRewriteRule struct {
    SourcePattern string `yaml:"source_pattern"`
    TargetModel   string `yaml:"target_model"`
}
```

**重写规则**：
- 支持通配符模式匹配
- 按规则顺序执行重写
- 支持动态模型映射

### 5. 标签系统 (tagging.Manager)

基于请求特征的智能路由和分类：

```go
type TaggingManager struct {
    registry *TagRegistry
    pipeline *TaggerPipeline
}

type Tagger interface {
    Name() string
    Tag() string
    ShouldTag(req *TaggedRequest) (bool, error)
}
```

**支持的标记器类型**：
- **内置标记器**：路径匹配、头部检查、查询参数等
- **Starlark 脚本**：自定义标记逻辑
- **优先级执行**：按配置优先级并发执行

### 6. 健康检查器 (health.Checker)

定期监控端点健康状态：

```go
type Checker struct {
    endpoints       []*endpoint.Endpoint
    checkInterval   time.Duration
    client          *http.Client
}

func (c *Checker) CheckEndpoint(ep *endpoint.Endpoint) error {
    // 根据端点类型使用不同的检查策略
    switch ep.EndpointType {
    case "anthropic":
        return c.checkAnthropicEndpoint(ep)
    case "openai":
        return c.checkOpenAIEndpoint(ep)
    default:
        return c.checkGenericEndpoint(ep)
    }
}
```

### 7. 响应验证器 (validator.ResponseValidator)

验证上游 API 响应格式：

```go
type ResponseValidator struct {
    strictMode      bool
    validateStreaming bool
}

func (v *ResponseValidator) ValidateResponse(body []byte, isStreaming bool) error {
    if isStreaming {
        return v.validateSSEResponse(body)
    }
    return v.validateJSONResponse(body)
}
```

**验证策略**：
- JSON 格式检查
- Anthropic API 字段验证
- SSE 流式响应验证
- 可配置严格模式

## 配置文件详细结构

### 完整配置示例

```yaml
server:
  host: 0.0.0.0
  port: 8080

endpoints:
  - name: "anthropic-primary"
    url: "https://api.anthropic.com"
    endpoint_type: "anthropic"
    auth_type: "api_key"
    auth_value: "sk-ant-..."
    enabled: true
    priority: 1
    tags: ["primary", "anthropic"]
    
  - name: "openai-compatible"
    url: "https://api.openai.com"
    endpoint_type: "openai"
    path_prefix: "/v1/chat/completions"
    auth_type: "auth_token"
    auth_value: "sk-..."
    enabled: true
    priority: 2
    model_rewrite:
      enabled: true
      rules:
        - source_pattern: "claude-*"
          target_model: "gpt-4"
          
  - name: "oauth-endpoint"
    url: "https://portal.qwen.ai"
    endpoint_type: "openai"
    path_prefix: "/v1/chat/completions"
    auth_type: "oauth"
    enabled: true
    priority: 3
    oauth_config:
      access_token: "token..."
      refresh_token: "refresh..."
      expires_at: 1755291655969
      token_url: "https://api.example.com/oauth/token"
      client_id: "client_id"
      auto_refresh: true
      
  - name: "proxy-endpoint"
    url: "https://api.example.com"
    endpoint_type: "anthropic"
    auth_type: "api_key"
    auth_value: "sk-ant-..."
    enabled: true
    priority: 4
    proxy:
      type: "http"
      address: "192.168.1.100:8080"

logging:
  level: "debug"
  log_request_types: "all"  # all, errors, none
  log_request_body: "full"  # full, truncated, none
  log_response_body: "full" # full, truncated, none
  log_directory: "./logs"

validation:
  strict_anthropic_format: false
  validate_streaming: false
  disconnect_on_invalid: false

tagging:
  pipeline_timeout: 10s
  taggers:
    - name: "api-detector"
      type: "builtin"
      builtin_type: "path"
      tag: "api-v1"
      enabled: true
      priority: 1
      config:
        path_pattern: "/v1/*"
        
    - name: "custom-tagger"
      type: "starlark"
      tag: "custom"
      enabled: true
      priority: 2
      config:
        script: |
          def should_tag():
              if "custom-header" in request.headers:
                  return True
              return False

timeouts:
  proxy:
    tls_handshake: "10s"
    response_header: "60s"
    idle_connection: "90s"
    overall_request: ""  # 空表示无限制，支持流式
  health_check:
    tls_handshake: "5s"
    response_header: "30s"
    idle_connection: "60s"
    overall_request: "30s"
    check_interval: "30s"

i18n:
  enabled: true
  default_language: "en"
  locales_path: "./web/locales"
```

## 故障转移和端点管理

### 端点选择策略

系统按以下优先级选择端点：

1. **优先级排序**：按配置中的 `priority` 值升序排列
2. **可用性检查**：只选择 `enabled: true` 且健康检查通过的端点
3. **标签匹配**：如果请求包含标签，优先选择支持该标签的端点
4. **故障转移**：当前端点失败时自动切换到下一个可用端点

### 健康检查机制

```go
type HealthCheckConfig struct {
    CheckInterval   time.Duration
    TLSHandshake   time.Duration
    ResponseHeader time.Duration
    IdleConnection time.Duration
    OverallRequest time.Duration
}

// 不同端点类型的健康检查策略
func (c *Checker) checkAnthropicEndpoint(ep *Endpoint) error {
    // 使用 /v1/messages 端点进行轻量检查
    req := &http.Request{
        Method: "POST",
        URL:    ep.URL + "/v1/messages",
        Header: map[string][]string{
            "Authorization":     {ep.GetAuthHeader()},
            "Content-Type":      {"application/json"},
            "anthropic-version": {"2023-06-01"},
        },
        Body: strings.NewReader(`{"model":"claude-3-haiku-20240307","max_tokens":1,"messages":[]}`),
    }
    
    // 执行请求并检查响应
    return c.executeHealthCheck(req, ep)
}

func (c *Checker) checkOpenAIEndpoint(ep *Endpoint) error {
    // 使用 /v1/models 端点检查
    req := &http.Request{
        Method: "GET",
        URL:    ep.URL + ep.PathPrefix + "/../models",
        Header: map[string][]string{
            "Authorization": {ep.GetAuthHeader()},
        },
    }
    
    return c.executeHealthCheck(req, ep)
}
```

### 错误处理策略

1. **网络错误**：立即标记端点为不可用，触发故障转移
2. **HTTP 4xx 错误**：记录错误但不影响端点可用性（可能是请求问题）
3. **HTTP 5xx 错误**：累计错误次数，超过阈值后标记端点不可用
4. **响应格式错误**：根据 `validation.disconnect_on_invalid` 配置决定是否断开
5. **超时错误**：按网络错误处理，立即故障转移

## Web 管理界面

### 页面结构

#### 1. 主仪表板 (`/admin/`)
- **端点状态概览**：实时显示所有端点的健康状态
- **请求统计**：成功/失败请求数量、响应时间等
- **活跃连接**：当前正在处理的请求数量
- **错误日志**：最近的错误和警告信息

#### 2. 端点管理 (`/admin/endpoints`)
- **端点列表**：显示所有配置的端点及其状态
- **端点配置**：添加、编辑、删除端点
- **健康检查**：手动触发健康检查
- **优先级调整**：拖拽调整端点优先级
- **模型重写规则**：配置模型名称重写

#### 3. 请求日志 (`/admin/logs`)
- **日志查看**：分页显示请求/响应日志
- **过滤功能**：按端点、状态码、时间范围过滤
- **详情查看**：展开查看完整的请求/响应内容
- **JSON 美化**：自动格式化 JSON 内容
- **SSE 流显示**：流式响应的实时显示

#### 4. 标签管理 (`/admin/taggers`)
- **标签器列表**：显示所有已注册的标签器
- **规则测试**：实时测试标签匹配规则
- **统计信息**：标签使用频率和分布
- **Starlark 编辑器**：在线编辑自定义标签脚本

#### 5. 系统设置 (`/admin/settings`)
- **服务器配置**：端口、超时等基础配置
- **日志配置**：日志级别、存储设置
- **验证设置**：响应格式验证规则
- **国际化设置**：语言和本地化配置

### 界面特性

- **多语言支持**：基于 `i18n` 配置的动态语言切换
- **实时更新**：WebSocket 连接实现状态实时刷新
- **响应式设计**：支持桌面和移动设备访问
- **主题切换**：支持明暗主题模式
- **键盘快捷键**：常用操作的快捷键支持
- **搜索功能**：全局搜索端点、日志等内容

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

## API 格式转换机制

### OpenAI 到 Anthropic 转换

系统支持将 OpenAI 格式的请求自动转换为 Anthropic 格式：

```go
type RequestConverter struct {
    modelRewriter *modelrewrite.Rewriter
}

// OpenAI 请求格式
type OpenAIRequest struct {
    Model       string                   `json:"model"`
    Messages    []OpenAIMessage         `json:"messages"`
    MaxTokens   int                     `json:"max_tokens,omitempty"`
    Temperature float64                 `json:"temperature,omitempty"`
    Stream      bool                    `json:"stream,omitempty"`
}

// Anthropic 请求格式
type AnthropicRequest struct {
    Model       string               `json:"model"`
    Messages    []AnthropicMessage  `json:"messages"`
    MaxTokens   int                 `json:"max_tokens"`
    Temperature float64             `json:"temperature,omitempty"`
    Stream      bool                `json:"stream,omitempty"`
}
```

**转换规则**：
1. **模型映射**：根据 `model_rewrite` 配置重写模型名
2. **消息转换**：将 OpenAI 消息格式转换为 Anthropic 格式
3. **参数映射**：将兼容的参数进行对应转换
4. **响应转换**：将 Anthropic 响应转换回 OpenAI 格式

### 流式响应转换

支持 SSE 流式响应的实时转换：

```go
type SSEParser struct {
    buffer *SimpleJSONBuffer
}

func (p *SSEParser) ParseAndConvert(chunk []byte, targetFormat string) ([]byte, error) {
    // 解析 SSE 数据块
    events := p.parseSSEChunk(chunk)
    
    // 转换为目标格式
    for _, event := range events {
        converted := p.convertEvent(event, targetFormat)
        // 返回转换后的数据
    }
    
    return convertedChunk, nil
}
```

## 安全和认证

### 认证机制

系统支持多种认证方式：

1. **API Key 认证** (`auth_type: "api_key"`)
   - 适用于 Anthropic API
   - 格式：`Authorization: Bearer sk-ant-...`

2. **Auth Token 认证** (`auth_type: "auth_token"`)
   - 适用于通用 API
   - 格式：`Authorization: Bearer <token>`

3. **OAuth2 认证** (`auth_type: "oauth"`)
   - 支持自动 token 刷新
   - 配置包含 access_token、refresh_token 等

### 代理支持

支持通过 HTTP 代理访问上游端点：

```yaml
proxy:
  type: "http"  # 或 "socks5"
  address: "192.168.1.100:8080"
  username: "user"     # 可选
  password: "pass"     # 可选
```

### 超时配置

细粒度的超时控制：

- **TLS 握手超时**：控制 SSL/TLS 连接建立时间
- **响应头超时**：等待服务器响应头的时间
- **空闲连接超时**：连接池中空闲连接的保持时间
- **整体请求超时**：单个请求的总超时时间

### 日志系统

**SQLite 存储**：
- 使用 SQLite 数据库存储请求日志
- 支持索引和查询优化
- 自动清理过期日志

**日志级别**：
- `debug`：详细的调试信息
- `info`：一般信息
- `warn`：警告信息
- `error`：错误信息

**日志内容**：
- 完整的请求/响应头部
- 可配置的请求/响应体记录
- 响应时间、状态码等元数据
- 错误堆栈跟踪
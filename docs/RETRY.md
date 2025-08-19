# Claude Proxy 重试逻辑完整分析

本文档详细说明了 Claude Proxy 中的所有重试逻辑、验证机制和失败处理策略。

## 重试决策逻辑概述

每个端点处理函数 `proxyToEndpoint()` 返回两个布尔值：
- `success`: 请求是否成功
- `shouldRetry`: 失败时是否应该尝试下一个端点

```go
func proxyToEndpoint(...) (success bool, shouldRetry bool) {
    // 返回值组合含义:
    // (true, _)     - 成功，停止重试
    // (false, true) - 失败，尝试下一个端点
    // (false, false)- 失败，停止所有重试
}
```

## 一、重试场景 (返回 `false, true`)

### 1.1 HTTP 状态码验证失败

**位置**: `internal/proxy/proxy_logic.go:156-169`

**触发条件**:
```go
if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return false, true
}
```

**说明**: 任何非 2xx 状态码都会触发重试，包括：
- 4xx 客户端错误（400, 401, 403, 404, 429 等）
- 5xx 服务器错误（500, 502, 503, 504 等）
- 3xx 重定向（不常见，但也会重试）

### 1.2 请求格式转换失败

**位置**: `internal/proxy/proxy_logic.go:85-91`

**触发条件**:
```go
convertedBody, ctx, err := s.converter.ConvertRequest(finalRequestBody, ep.EndpointType)
if err != nil {
    return false, true // 转换失败，尝试下一个端点
}
```

**说明**: 当 OpenAI 格式请求转换为 Anthropic 格式失败时重试

### 1.3 网络层面请求失败

**位置**: `internal/proxy/proxy_logic.go:147-152`

**触发条件**:
```go
resp, err := client.Do(req)
if err != nil {
    return false, true
}
```

**说明**: 包括但不限于：
- 网络连接超时
- DNS 解析失败
- 连接被拒绝
- SSL/TLS 握手失败

### 1.4 创建代理客户端失败

**位置**: `internal/proxy/proxy_logic.go:139-145`

**触发条件**:
```go
client, err := ep.CreateProxyClient(s.config.Timeouts.Proxy)
if err != nil {
    return false, true
}
```

**说明**: 代理配置错误或代理服务器不可用时重试

### 1.5 响应内容验证失败

**位置**: `internal/proxy/proxy_logic.go:224-241`

#### 1.5.1 Usage 统计验证失败
**触发条件**:
```go
if strings.Contains(err.Error(), "invalid usage stats") {
    return false, true // 验证失败，尝试下一个endpoint
}
```

**验证逻辑** (`internal/validator/response.go:354-355`):
```go
// 只有当三个字段都存在且都为0时才判定为不合法
if promptTokens == 0 && completionTokens == 0 && totalTokens == 0 {
    return fmt.Errorf("invalid usage stats: prompt_tokens, completion_tokens and total_tokens are all zero")
}
```

#### 1.5.2 SSE 流不完整验证失败
**触发条件**:
```go
if strings.Contains(err.Error(), "incomplete SSE stream") || 
   strings.Contains(err.Error(), "missing message_stop") || 
   strings.Contains(err.Error(), "missing [DONE]") || 
   strings.Contains(err.Error(), "missing finish_reason") {
    return false, true // SSE流不完整，尝试下一个endpoint
}
```

**Anthropic SSE 完整性验证**:
- 必须有 `message_start` 和对应的 `message_stop` 事件
- 验证事件类型有效性

**OpenAI SSE 完整性验证**:
- 必须包含 `[DONE]` 标记
- 必须包含 `finish_reason` 字段

## 二、不重试场景 (返回 `false, false`)

### 2.1 基础设施错误

#### 2.1.1 创建 HTTP 请求失败
**位置**: `internal/proxy/proxy_logic.go:32-38`, `102-109`

**触发条件**:
```go
req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewReader(requestBody))
if err != nil {
    return false, false
}
```

**说明**: 请求参数错误，重试其他端点也会失败

#### 2.1.2 模型重写失败
**位置**: `internal/proxy/proxy_logic.go:42-48`

**触发条件**:
```go
originalModel, rewrittenModel, err := s.modelRewriter.RewriteRequestWithTags(tempReq, ep.ModelRewrite, ep.Tags)
if err != nil {
    return false, false
}
```

**说明**: 模型重写配置错误，重试其他端点意义不大

#### 2.1.3 读取重写后请求体失败
**位置**: `internal/proxy/proxy_logic.go:55-59`

**触发条件**:
```go
finalRequestBody, err = io.ReadAll(tempReq.Body)
if err != nil {
    return false, false
}
```

### 2.2 认证失败
**位置**: `internal/proxy/proxy_logic.go:125-129`

**触发条件**:
```go
authHeader, err := ep.GetAuthHeaderWithRefreshCallback(...)
if err != nil {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
    return false, false
}
```

**说明**: 认证失败通常是配置问题，直接返回错误

### 2.3 响应处理错误

#### 2.3.1 读取响应体失败
**位置**: `internal/proxy/proxy_logic.go:173-179`

**触发条件**:
```go
responseBody, err := io.ReadAll(resp.Body)
if err != nil {
    return false, false
}
```

#### 2.3.2 解压响应体失败
**位置**: `internal/proxy/proxy_logic.go:184-191`

**触发条件**:
```go
decompressedBody, err := s.validator.GetDecompressedBody(responseBody, contentEncoding)
if err != nil {
    return false, false
}
```

### 2.4 严格验证失败且配置断开连接
**位置**: `internal/proxy/proxy_logic.go:243-251`

**触发条件**:
```go
if s.config.Validation.DisconnectOnInvalid {
    c.Header("Connection", "close")
    c.AbortWithStatus(http.StatusBadGateway)
    return false, false
}
```

**说明**: 配置为严格模式且要求断开连接时，不重试直接断开

## 三、端点回退策略

### 3.1 回退控制逻辑
**位置**: `internal/proxy/endpoint_management.go:59-62`

```go
if !shouldRetry {
    s.logger.Debug("Endpoint indicated no retry, stopping fallback")
    break
}
```

当端点明确返回 `shouldRetry = false` 时，停止所有后续端点尝试。

### 3.2 端点选择策略

#### 有标签请求的两阶段回退:
1. **Phase 1**: 尝试匹配标签的端点
   ```go
   taggedEndpoints := s.filterAndSortEndpoints(allEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
       return len(ep.Tags) > 0 && s.endpointContainsAllTags(ep.Tags, requestTags)
   })
   ```

2. **Phase 2**: 尝试万用端点（无标签）
   ```go
   universalEndpoints := s.filterAndSortEndpoints(allEndpoints, failedEndpoint, func(ep *endpoint.Endpoint) bool {
       return len(ep.Tags) == 0
   })
   ```

#### 无标签请求:
- 只尝试万用端点

### 3.3 端点过滤条件
```go
// 跳过已失败的endpoint
if ep.ID == failedEndpoint.ID {
    continue
}
// 跳过禁用或不可用的端点
if !ep.Enabled || !ep.IsAvailable() {
    continue
}
```

## 四、配置选项影响

### 4.1 严格验证模式
```yaml
validation:
  strict_anthropic_format: true    # 启用严格验证
  disconnect_on_invalid: true      # 验证失败时断开连接
```

**影响**:
- 启用严格 Anthropic 格式验证
- 启用 SSE 流完整性检查
- 启用 usage 统计验证
- `disconnect_on_invalid = true` 时验证失败不重试

### 4.2 超时配置
```yaml
timeouts:
  proxy: 30s    # 代理请求超时
```

**影响**: 网络超时会触发重试逻辑

### 4.3 端点配置
```yaml
endpoints:
  - name: "primary"
    enabled: true       # 禁用时跳过
    priority: 1         # 影响选择顺序
    tags: ["tag1"]      # 影响回退策略
```

## 五、日志记录

所有失败情况都会记录详细日志，包括：
- 尝试次数 (`attemptNumber`)
- 失败原因
- 请求和响应详情
- 重试决策

**日志示例**:
```
HTTP error 429 from endpoint primary, trying next endpoint
Phase 1: Trying 2 tagged endpoints
Phase 2: Trying 1 universal endpoints
All 3 endpoints failed for tagged request (tags: [gpu])
```

## 六、最佳实践建议

1. **端点配置**: 配置多个不同类型的端点以提高容错性
2. **标签使用**: 合理使用标签进行请求路由和回退
3. **超时设置**: 设置合理的超时时间避免过长等待
4. **验证配置**: 根据需要选择严格或宽松的验证模式
5. **监控日志**: 监控重试模式识别潜在问题

## 七、故障排查

### 常见重试场景:
1. **频繁 HTTP 错误**: 检查上游 API 状态
2. **格式转换失败**: 检查请求格式和端点类型匹配
3. **验证失败**: 检查响应格式和验证配置
4. **网络错误**: 检查网络连接和代理配置

### 调试技巧:
1. 查看日志中的 `attemptNumber` 了解重试次数
2. 关注 `shouldRetry` 的决策原因
3. 检查端点的 `enabled` 和 `IsAvailable()` 状态
4. 验证标签匹配逻辑
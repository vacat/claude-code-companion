# 被拉黑端点日志记录增强功能设计方案

## 功能概述

本设计方案旨在增强被拉黑（失效）端点的日志记录功能，包括：

1. **恢复被拉黑端点的日志记录** - 之前被拉黑的端点请求不记录日志，现在恢复记录
2. **记录端点失效原因** - 当端点被判定为失效时，记录导致失效的所有请求ID
3. **在后续日志中包含失效原因** - 对失败端点的后续请求，日志中包含导致端点失效的请求ID
4. **端点恢复时清除失效原因** - 当端点恢复正常时，清除内存中的失败原因记录

## 当前系统分析

### 端点状态管理机制

```go
type Status string

const (
    StatusActive   Status = "active"    // 端点可用
    StatusInactive Status = "inactive"  // 端点被拉黑/失效
    StatusChecking Status = "checking"  // 健康检查中
)
```

### 端点失效判定逻辑

端点失效通过 `CircularBuffer.ShouldMarkInactive()` 判定：
- 在时间窗口内（默认140秒）
- 总请求数 > 1 
- 所有请求都失败

失效判定代码位置：`internal/endpoint/endpoint.go:238`

### 当前日志处理问题

1. **被拉黑端点不记录日志**：在 `filterAndSortEndpoints()` 中，`!ep.IsAvailable()` 的端点被过滤掉
2. **缺少失效原因追踪**：没有记录是哪些请求导致端点失效
3. **后续请求缺少上下文**：对已失效端点的请求日志中没有失效原因信息

## 设计方案

### 1. 数据结构设计

#### 1.1 端点失效原因记录结构

```go
// BlacklistReason 记录端点被拉黑的原因
type BlacklistReason struct {
    // 导致失效的请求ID列表
    CausingRequestIDs []string `json:"causing_request_ids"`
    
    // 失效时间
    BlacklistedAt time.Time `json:"blacklisted_at"`
    
    // 失效时的错误信息摘要
    ErrorSummary string `json:"error_summary"`
}
```

#### 1.2 扩展 Endpoint 结构

```go
type Endpoint struct {
    // ... 现有字段 ...
    
    // 新增：被拉黑的原因（内存中，不持久化）
    BlacklistReason *BlacklistReason `json:"-"`
    
    // 新增：保护 BlacklistReason 的互斥锁
    blacklistMutex sync.RWMutex
}
```

#### 1.3 扩展 CircularBuffer 结构

```go
type RequestRecord struct {
    Timestamp time.Time
    Success   bool
    
    // 新增：请求ID（用于追踪失败原因）
    RequestID string
}
```

### 2. 核心功能实现

#### 2.1 记录端点失效原因

```go
// MarkInactiveWithReason 标记端点为失效并记录原因
func (e *Endpoint) MarkInactiveWithReason() {
    e.mutex.Lock()
    defer e.mutex.Unlock()
    
    if e.Status == StatusActive {
        e.Status = StatusInactive
        
        // 从循环缓冲区获取导致失效的请求ID
        failedRequestIDs := e.RequestHistory.GetRecentFailureRequestIDs(time.Now())
        
        // 构建失效原因记录
        e.blacklistMutex.Lock()
        e.BlacklistReason = &BlacklistReason{
            BlacklistedAt:     time.Now(),
            CausingRequestIDs: failedRequestIDs,
            ErrorSummary:      fmt.Sprintf("Endpoint failed due to %d consecutive failures", len(failedRequestIDs)),
        }
        e.blacklistMutex.Unlock()
    }
}
```

#### 2.2 扩展循环缓冲区功能

```go
// GetRecentFailureRequestIDs 获取时间窗口内的所有失败请求ID
func (cb *CircularBuffer) GetRecentFailureRequestIDs(now time.Time) []string {
    cb.mutex.RLock()
    defer cb.mutex.RUnlock()

    cutoff := now.Add(-cb.windowDur)
    var failureRequestIDs []string

    for i := 0; i < cb.count; i++ {
        idx := (cb.head - 1 - i + cb.size) % cb.size
        record := cb.records[idx]

        if record.Timestamp.Before(cutoff) {
            break
        }

        if !record.Success && record.RequestID != "" {
            failureRequestIDs = append(failureRequestIDs, record.RequestID)
        }
    }

    return failureRequestIDs
}

// AddWithRequestID 添加带请求ID的请求记录
func (cb *CircularBuffer) AddWithRequestID(record RequestRecord, requestID string) {
    record.RequestID = requestID
    cb.Add(record)
}
```

#### 2.3 端点恢复时清除失效原因

```go
// MarkActive 标记端点为可用并清除失效原因
func (e *Endpoint) MarkActive() {
    e.mutex.Lock()
    defer e.mutex.Unlock()
    
    e.Status = StatusActive
    e.FailureCount = 0
    e.SuccessiveSuccesses = 0
    
    // 清除失效原因记录
    e.blacklistMutex.Lock()
    e.BlacklistReason = nil
    e.blacklistMutex.Unlock()
    
    // 清理历史记录
    e.RequestHistory.Clear()
}
```

### 3. 日志记录增强

#### 3.1 被拉黑端点日志记录恢复

修改 `filterAndSortEndpoints` 函数，允许记录被拉黑端点的请求日志：

```go
// 在 proxy_logic.go 中修改，对被拉黑端点也尝试记录日志
func (s *Server) proxyToBlacklistedEndpoint(c *gin.Context, ep *endpoint.Endpoint, path string, requestBody []byte, requestID string, startTime time.Time, taggedRequest *tagging.TaggedRequest, attemptNumber int) {
    // 立即失败，但记录详细日志
    duration := time.Since(startTime)
    
    // 获取失效原因
    ep.blacklistMutex.RLock()
    blacklistReason := ep.BlacklistReason
    ep.blacklistMutex.RUnlock()
    
    // 构建包含失效原因的错误信息
    var errorMsg string
    var causingRequestIDs []string
    
    if blacklistReason != nil {
        causingRequestIDs = blacklistReason.CausingRequestIDs
        errorMsg = fmt.Sprintf("Endpoint blacklisted due to previous failures. Causing request IDs: %v. Original error: %s", 
            causingRequestIDs, blacklistReason.ErrorSummary)
    } else {
        errorMsg = "Endpoint is blacklisted (no detailed reason available)"
    }
    
    // 记录日志，包含失效原因
    s.logBlacklistedEndpointRequest(requestID, ep, path, requestBody, c, duration, errorMsg, causingRequestIDs, attemptNumber, taggedRequest)
}
```

#### 3.2 扩展日志记录结构

```go
// 在现有的 RequestLog 结构中添加新字段
type RequestLog struct {
    // ... 现有字段 ...
    
    // 新增：导致端点失效的请求ID（如果当前请求是对被拉黑端点的请求）
    BlacklistCausingRequestIDs []string `json:"blacklist_causing_request_ids,omitempty"`
    
    // 新增：端点失效时间（如果适用）
    EndpointBlacklistedAt *time.Time `json:"endpoint_blacklisted_at,omitempty"`
    
    // 新增：端点失效原因摘要
    EndpointBlacklistReason string `json:"endpoint_blacklist_reason,omitempty"`
}
```

#### 3.3 专用的被拉黑端点日志记录函数

```go
func (s *Server) logBlacklistedEndpointRequest(requestID string, ep *endpoint.Endpoint, path string, requestBody []byte, c *gin.Context, duration time.Duration, errorMsg string, causingRequestIDs []string, attemptNumber int, taggedRequest *tagging.TaggedRequest) {
    requestLog := s.logger.CreateRequestLog(requestID, ep.URL, c.Request.Method, path)
    requestLog.RequestBodySize = len(requestBody)
    requestLog.AttemptNumber = attemptNumber
    requestLog.DurationMs = duration.Nanoseconds() / 1000000
    requestLog.StatusCode = http.StatusServiceUnavailable
    requestLog.Error = errorMsg
    
    // 设置被拉黑端点相关信息
    requestLog.BlacklistCausingRequestIDs = causingRequestIDs
    if ep.BlacklistReason != nil {
        requestLog.EndpointBlacklistedAt = &ep.BlacklistReason.BlacklistedAt
        requestLog.EndpointBlacklistReason = ep.BlacklistReason.ErrorSummary
    }
    
    // 设置请求标签
    if taggedRequest != nil {
        requestLog.Tags = taggedRequest.Tags
    }
    
    // 记录原始请求数据
    if c.Request != nil {
        requestLog.OriginalRequestHeaders = utils.HeadersToMap(c.Request.Header)
        requestLog.OriginalRequestURL = c.Request.URL.String()
        requestLog.RequestHeaders = requestLog.OriginalRequestHeaders
    }
    
    // 记录请求体
    if len(requestBody) > 0 {
        requestLog.Model = utils.ExtractModelFromRequestBody(string(requestBody))
        requestLog.SessionID = utils.ExtractSessionIDFromRequestBody(string(requestBody))
        
        if s.config.Logging.LogRequestBody != "none" {
            if s.config.Logging.LogRequestBody == "truncated" {
                requestLog.OriginalRequestBody = utils.TruncateBody(string(requestBody), 1024)
            } else {
                requestLog.OriginalRequestBody = string(requestBody)
            }
            requestLog.RequestBody = requestLog.OriginalRequestBody
        }
    }
    
    s.logger.LogRequest(requestLog)
}
```

### 4. 集成到现有请求处理流程

#### 4.1 修改端点选择逻辑

```go
// 在 endpoint_management.go 中修改 filterAndSortEndpoints
func (s *Server) filterAndSortEndpoints(allEndpoints []*endpoint.Endpoint, failedEndpoint *endpoint.Endpoint, filterFunc func(*endpoint.Endpoint) bool) []utils.EndpointSorter {
    var filtered []*endpoint.Endpoint
    var blacklistedButLoggable []*endpoint.Endpoint
    
    for _, ep := range allEndpoints {
        if ep.ID == failedEndpoint.ID {
            continue
        }
        
        if !ep.Enabled {
            continue
        }
        
        if ep.IsAvailable() {
            if filterFunc(ep) {
                filtered = append(filtered, ep)
            }
        } else {
            // 被拉黑的端点不参与请求处理，但可以用于日志记录
            blacklistedButLoggable = append(blacklistedButLoggable, ep)
        }
    }
    
    // 将可用端点转换为排序接口
    var sorters []utils.EndpointSorter
    for _, ep := range filtered {
        sorters = append(sorters, utils.EndpointSorter(ep))
    }
    
    // 排序可用端点
    utils.SortEndpoints(sorters)
    
    // 存储被拉黑端点供日志使用（不参与实际请求处理）
    s.storeBlacklistedEndpointsForLogging(blacklistedButLoggable)
    
    return sorters
}
```

#### 4.2 修改请求记录逻辑

```go
// 在 endpoint.go 中修改 RecordRequest 方法
func (e *Endpoint) RecordRequest(success bool, requestID string) {
    e.mutex.Lock()
    defer e.mutex.Unlock()

    now := time.Now()
    
    // 添加到环形缓冲区（包含请求ID）
    record := utils.RequestRecord{
        Timestamp: now,
        Success:   success,
        RequestID: requestID,
    }
    e.RequestHistory.Add(record)
    
    e.TotalRequests++
    if success {
        e.SuccessRequests++
        e.FailureCount = 0
        e.SuccessiveSuccesses++
        if e.Status == StatusInactive {
            // 恢复时清除失效原因
            e.MarkActive()
        }
    } else {
        e.FailureCount++
        e.LastFailure = now
        e.SuccessiveSuccesses = 0
        
        // 检查是否应该标记为失效（包含原因记录）
        if e.Status == StatusActive && e.RequestHistory.ShouldMarkInactive(now) {
            e.MarkInactiveWithReason()
        }
    }
}
```

### 5. 错误处理优化

#### 5.1 最终失败响应增强

```go
func (s *Server) sendFailureResponse(c *gin.Context, requestID string, startTime time.Time, requestBody []byte, requestTags []string, attemptedCount int, errorMsg, errorType string) {
    // ... 现有逻辑 ...
    
    // 添加被拉黑端点的详细信息
    allEndpoints := s.endpointManager.GetAllEndpoints()
    var blacklistedEndpoints []string
    var blacklistReasons []string
    
    for _, ep := range allEndpoints {
        if !ep.IsAvailable() && ep.BlacklistReason != nil {
            blacklistedEndpoints = append(blacklistedEndpoints, ep.Name)
            blacklistReasons = append(blacklistReasons, 
                fmt.Sprintf("caused by requests: %v", 
                    ep.BlacklistReason.CausingRequestIDs))
        }
    }
    
    if len(blacklistedEndpoints) > 0 {
        errorMsg += fmt.Sprintf(". Blacklisted endpoints: %v. Reasons: %v", 
            blacklistedEndpoints, blacklistReasons)
    }
    
    // ... 继续现有逻辑 ...
}
```

## 实现步骤

### 阶段1：数据结构扩展
1. 扩展 `Endpoint` 结构添加失效原因记录
2. 扩展 `RequestRecord` 结构添加上下文信息
3. 扩展 `RequestLog` 结构添加被拉黑端点信息

### 阶段2：核心逻辑实现
1. 实现 `MarkInactiveWithReason()` 方法
2. 实现 `GetRecentFailures()` 方法
3. 修改 `RecordRequest()` 方法支持上下文记录

### 阶段3：日志记录增强
1. 实现 `logBlacklistedEndpointRequest()` 函数
2. 修改现有的日志记录逻辑
3. 添加被拉黑端点的专门处理逻辑

### 阶段4：集成测试
1. 测试端点失效原因记录
2. 测试被拉黑端点日志记录
3. 测试端点恢复时的清理逻辑

## 配置选项

```yaml
logging:
  # 是否记录被拉黑端点的请求日志
  log_blacklisted_requests: true
  
  # 被拉黑端点失效原因保留时间（小时）
  blacklist_reason_retention_hours: 24
  
  # 是否在最终错误响应中包含详细的失效原因
  include_blacklist_details_in_error: true
```

## 性能考虑

1. **内存使用**：`BlacklistReason` 只在内存中存储，不持久化，只保存请求ID列表
2. **锁竞争**：使用专门的 `blacklistMutex` 减少锁竞争
3. **日志量**：被拉黑端点的请求可能增加日志量，可通过配置控制
4. **循环缓冲区**：不需要额外的映射表，直接从循环缓冲区提取失败请求ID

## 兼容性

1. 新增字段使用 `json:"-"` 或 `omitempty` 标签确保向后兼容
2. 现有API和日志格式保持不变
3. 新功能通过配置项可选启用

---

这个设计方案完整地实现了用户提出的四个核心需求，同时保持了与现有系统的兼容性和良好的性能特征。
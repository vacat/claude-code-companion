# 端点统计数据持久化设计方案

## 问题分析

### 当前问题
1. **统计数据重置**：端点配置修改时，`Manager.UpdateEndpoints()` 会创建新的 `Endpoint` 对象，导致所有统计数据丢失
2. **重启数据丢失**：服务重启时，内存中的统计数据（`TotalRequests`、`SuccessRequests` 等）全部丢失
3. **无历史追踪**：无法查看端点的历史表现趋势

### 当前实现分析
- `Endpoint` 结构体包含统计字段：`TotalRequests`、`SuccessRequests`、`FailureCount` 等
- 使用 `CircularBuffer` 存储最近的请求记录（100个记录，140秒窗口）
- `RecordRequest()` 方法更新统计数据和环形缓冲区
- 所有数据都在内存中，无持久化机制

## 设计方案

### 方案选择：SQLite + 仅统计数据（方案A）

**为什么选择 SQLite？**
- 轻量级，无需额外服务
- 事务支持，保证数据一致性
- 项目已有 GORM 基础设施
- 独立于配置文件，不影响现有逻辑

**为什么选择方案A？**
- 简单可靠，数据量可控
- 满足主要需求：统计数据持久化
- 不影响现有健康检查逻辑
- 避免历史表带来的存储和清理复杂性

### 数据库设计

#### 统计数据库文件
- **文件名**：`statistics.db`
- **位置**：与 `logs.db` 分离，独立管理
- **用途**：专门存储端点统计数据

#### 主表：endpoint_statistics
```sql
CREATE TABLE endpoint_statistics (
    id TEXT PRIMARY KEY,                    -- 端点稳定ID（基于名称哈希）
    name TEXT NOT NULL,                     -- 端点名称
    url TEXT NOT NULL,                      -- 当前端点URL
    endpoint_type TEXT NOT NULL,            -- 当前端点类型
    auth_type TEXT NOT NULL,                -- 当前认证类型
    total_requests INTEGER DEFAULT 0,       -- 总请求数
    success_requests INTEGER DEFAULT 0,     -- 成功请求数
    failure_count INTEGER DEFAULT 0,        -- 连续失败计数
    successive_successes INTEGER DEFAULT 0, -- 连续成功次数
    last_failure DATETIME,                  -- 最后失败时间
    last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

**字段说明：**
- `id`：基于端点名称生成的稳定ID，名称不变则ID不变
- `url/endpoint_type/auth_type`：当前配置信息，仅用于展示，变更不影响统计

### 端点ID和统计数据继承设计

#### 问题分析
当前端点ID生成方式：`fmt.Sprintf("endpoint-%s-%d", name, time.Now().Unix())`

**存在的问题：**
1. 删除端点后重新添加同名端点，无法继承之前的统计数据
2. 基于时间戳的ID在短时间内可能重复
3. **核心问题**：配置变更会导致统计数据丢失，用户期望统计数据持续累积

#### 解决方案：基于名称的稳定ID + 智能配置继承

**设计理念：**
- **端点名称** = **统计数据身份**：相同名称的端点应继承统计数据
- **配置变更** ≠ **新端点**：配置调整不应导致统计重置
- **名称变更** = **新端点**：只有名称变更才创建新的统计记录

**新的ID生成策略：**
```go
// 基于端点名称生成稳定的ID
func generateEndpointID(cfg config.EndpointConfig) string {
    // 使用端点名称的哈希作为稳定标识符
    nameHash := sha256.Sum256([]byte(cfg.Name))
    return fmt.Sprintf("ep-name-%x", nameHash[:12])
}
```

**ID特性：**
- **名称唯一**：基于名称哈希，确保同名端点ID相同
- **稳定性**：配置变更不影响ID，统计数据持续累积
- **可读性**：ID中包含"name"标识，便于识别

#### 智能配置变更处理

**核心配置 vs 非核心配置：**
```go
// 核心配置：影响端点运行的关键配置，变更需要记录
type CoreEndpointConfig struct {
    URL          string
    EndpointType string  
    AuthType     string
}

// 元数据配置：描述性信息，变更不影响统计
type MetadataConfig struct {
    Tags              []string
    ModelRewrite      *config.ModelRewriteConfig
    Proxy             *config.ProxyConfig
    OverrideMaxTokens *int
}
```

**配置变更处理逻辑：**
```go
func (m *Manager) handleEndpointUpdate(oldEndpoint *Endpoint, newConfig config.EndpointConfig) error {
    if oldEndpoint.Name == newConfig.Name {
        // 同名端点：继承统计数据，更新配置
        
        // 1. 更新端点配置
        oldEndpoint.UpdateConfig(newConfig)
        
        // 2. 更新数据库中的元数据信息
        return m.statisticsManager.UpdateEndpointMetadata(
            oldEndpoint.ID, newConfig.Name, newConfig.URL, newConfig.EndpointType, newConfig.AuthType)
            
    } else {
        // 名称变更：创建新端点，旧统计数据保留但不再关联
        m.logger.Info("Endpoint renamed, creating new statistics record",
            map[string]interface{}{
                "old_name": oldEndpoint.Name,
                "new_name": newConfig.Name,
            })
        return m.createNewEndpointWithStatistics(newConfig)
    }
}
```

**统计数据继承策略：**
1. **名称相同** → **继承统计数据**：配置变更不重置统计
2. **名称变更** → **新建统计记录**：视为新端点
3. **端点删除** → **统计数据保留**：标记为非活跃，可选择性清理
4. **重名冲突** → **合并处理**：同名端点重新添加时继承原有统计

## 实现架构

### 核心组件

#### 1. StatisticsManager
```go
type Manager struct {
    db *gorm.DB
    dbPath string
}

// 核心方法
func (m *Manager) LoadStatistics(endpointID string) (*EndpointStatistics, error)
func (m *Manager) LoadStatisticsByName(endpointName string) (*EndpointStatistics, error)
func (m *Manager) SaveStatistics(stats *EndpointStatistics) error  
func (m *Manager) RecordRequest(endpointID string, success bool) error
func (m *Manager) InitializeEndpointStatistics(endpointName, url, endpointType, authType string) (*EndpointStatistics, error)
func (m *Manager) UpdateEndpointMetadata(endpointID, name, url, endpointType, authType string) error
func (m *Manager) DeleteStatistics(endpointID string) error
```

#### 2. Endpoint 集成
- **稳定ID生成**：基于端点名称生成固定ID，配置变更不影响
- **统计数据继承**：启动时通过名称匹配，自动继承现有统计数据
- **配置变更处理**：配置变更时保留统计数据，仅更新元数据字段
- **双重更新**：`RecordRequest()` 同时更新内存和数据库
- **健康检查保持**：CircularBuffer 继续在内存中用于实时健康检查

#### 3. Manager 集成  
- **智能端点匹配**：`UpdateEndpoints()` 通过名称匹配现有端点
- **统计数据保持**：配置变更时保留统计，仅更新元数据
- **名称变更处理**：端点重命名时创建新统计记录
- **清理机制**：删除端点时同时清理对应的统计数据

### 数据流程

#### 启动时
1. StatisticsManager 初始化独立的 `statistics.db` 数据库
2. 为每个端点配置基于名称生成稳定ID
3. 尝试加载同名端点的统计数据，实现数据继承
4. 无匹配统计时创建新记录，配置变更时更新元数据
5. CircularBuffer 从空状态开始积累，用于健康检查

#### 运行时  
1. 每次请求调用 `Endpoint.RecordRequest()`
2. 同时更新内存统计字段和数据库记录（事务保证一致性）
3. CircularBuffer 继续积累用于实时健康检查和端点状态判断
4. 数据库记录所有累积统计，内存保持最新状态

#### 配置更新时
1. **名称匹配**：通过端点名称匹配现有端点和统计数据
2. **同名端点**：保留统计数据，更新配置参数
3. **名称变更**：创建新统计记录，旧数据保留但不再关联
4. **元数据更新**：URL、类型等变更时更新数据库元数据字段
5. **端点删除**：同时删除对应的统计数据记录

## 风险和注意事项

### 数据一致性
- 使用事务确保统计数据的一致性更新
- 数据库写入失败时的错误处理和日志记录
- 内存统计和数据库记录的同步问题

### 性能影响  
- 每个请求都会触发数据库写入操作
- 数据库文件锁定可能影响并发性能
- 考虑异步写入或批量更新优化（后期优化）

### 数据库文件管理
- **文件位置**：独立的 `statistics.db` 文件，与 `logs.db` 分离
- **权限控制**：确保应用有读写权限
- **备份策略**：统计数据的备份和恢复
- **大小监控**：虽然数据量小，但仍需监控文件大小

### 端点ID和统计继承管理
- **稳定性保证**：基于名称的ID确保配置变更时统计数据不丢失
- **名称冲突处理**：同名端点自动继承统计数据，避免意外重置
- **数据一致性**：名称变更时正确创建新统计，避免数据混淆
- **删除清理**：端点删除时同步清理统计数据，避免孤立记录

### 兼容性
- 现有 CircularBuffer 逻辑保持完全不变
- 健康检查机制不受影响
- Web 界面需要适配新的统计数据源

## 实施步骤

### 第一阶段：核心持久化功能
1. **创建统计数据模型和管理器**
   - 实现 `EndpointStatistics` GORM 模型
   - 实现 `StatisticsManager` 核心功能
   - 独立的 `statistics.db` 数据库初始化

2. **修改端点ID生成和继承机制**
   - 基于名称哈希的稳定ID生成算法
   - 智能配置变更检测和统计数据继承逻辑
   - 核心配置vs非核心配置的区分机制

3. **集成到现有系统**
   - 修改 `NewEndpoint()` 通过名称匹配和继承统计数据
   - 修改 `RecordRequest()` 同时更新内存和数据库
   - 修改 `Manager.UpdateEndpoints()` 智能保留统计数据

### 第二阶段：完善和优化
1. **错误处理和日志**
   - 数据库操作失败的处理策略
   - 详细的操作日志记录
   - 统计数据不一致时的修复机制

2. **管理功能**
   - 孤立统计数据的清理
   - 统计数据的手动重置功能
   - 数据库维护工具

3. **性能优化**
   - 数据库连接池优化
   - 写入性能测试和调优
   - 必要时考虑异步写入

### 第三阶段：监控和维护
1. **监控指标**
   - 统计数据库文件大小监控
   - 写入操作成功率监控
   - 性能指标收集

2. **维护工具**
   - 统计数据的导出/导入
   - 数据库完整性检查
   - 性能分析工具

## 已确定的设计决策

✅ **采用方案A**：仅保存统计数据，不实现请求历史表  
✅ **数据库分离**：`statistics.db` 与 `logs.db` 独立管理  
✅ **基于名称的稳定ID**：端点名称不变则统计数据持续累积  
✅ **统计数据继承**：配置变更不重置统计，解决用户核心痛点  
✅ **CircularBuffer保留**：内存中继续使用，用于健康检查  
✅ **智能配置变更**：区分核心配置和元数据配置的变更影响

## 核心优势

🎯 **解决统计丢失问题**：配置调整（URL、认证等）不会导致统计数据重置  
🎯 **简单可靠**：基于端点名称的稳定标识符，逻辑清晰  
🎯 **管理简单**：端点删除时自动清理统计，无孤立数据  
🎯 **向后兼容**：不影响现有健康检查和CircularBuffer逻辑  

## 实施细节确认

✅ **数据库文件路径**：`statistics.db` 与 `logs.db` 放置在同一目录，不单独配置  
✅ **统计数据清理策略**：删除端点时同时清理统计数据，长期不活跃数据永久保留  
✅ **配置变更日志**：移除配置变更日志记录需求，简化实现
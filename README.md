# Claude API Proxy

一个专为 Claude Code 设计的本地 API 代理服务，提供负载均衡、故障转移和响应验证功能。

## 功能特性

- 🔄 **多端点负载均衡**: 支持配置多个上游 Anthropic API 端点，按优先级进行故障转移
- 🛡️ **响应格式验证**: 验证上游 API 响应是否符合 Anthropic 协议格式，异常时自动断开连接
- 📊 **智能故障检测**: 140秒窗口内的请求失败率检测，避免误判单次超时
- 📦 **内容解压透传**: 自动处理 gzip 压缩响应，解压后透传给客户端
- 🏷️ **智能标签路由**: 基于请求特征（路径、头部、内容等）的动态端点选择系统
- 📋 **SQLite 日志存储**: 企业级数据库日志存储，支持高效查询和自动清理
- ⚡ **配置热更新**: 支持端点配置和标签的实时更新，无需重启服务
- 🌐 **完整 Web 管理**: 提供端点管理、标签配置、日志查看和系统监控界面
- 🔧 **Starlark 脚本**: 支持自定义标签脚本，实现复杂的路由逻辑

## 工作原理

### 系统架构

```
客户端 (Claude Code)
       ↓
   本地代理服务器 (8080)
       ↓
   标签处理器 (Tagging Pipeline)
   ┌─────────────────────────────────────┐
   │ Path    Header   Method   Query    │
   │ Tagger  Tagger   Tagger   Tagger   │
   │    ↓       ↓        ↓       ↓      │
   │        生成请求标签                 │
   └─────────────────────────────────────┘
       ↓
   端点选择器 (基于标签匹配)
       ↓
┌─────────────────────────────────────────┐
│ 端点1 (tags: [api-v1, claude-3])       │
│     ↓                                   │
│ 上游API1                               │
│                                         │
│ 端点2 (tags: [beta])                   │
│     ↓                                   │
│ 上游API2                               │
│                                         │
│ 端点3 (tags: [])  <-- 万能端点         │
│     ↓                                   │
│ 上游API3                               │
└─────────────────────────────────────────┘
```

### 标签路由工作原理

**标签路由系统** 是本代理的核心特性，通过分析请求特征自动选择最适合的端点：

1. **请求分析**: 并发执行多个标签处理器（Tagger）分析请求
2. **标签生成**: 根据路径、头部、内容等特征生成请求标签
3. **端点匹配**: 选择拥有匹配标签的端点（子集匹配原则）
4. **智能回退**: 标签端点不可用时自动回退到万能端点

**匹配算法**:
- 请求必须拥有端点所需的**所有标签**才能匹配
- 无标签端点为**万能端点**，匹配所有请求
- 优先选择**标签匹配的端点**，其次选择万能端点

### 核心工作流程

1. **请求接收**: 客户端向本地代理 (默认8080端口) 发送请求
2. **身份验证**: 使用配置的 `auth_token` 进行本地认证
3. **标签处理**: 
   - 并发执行所有启用的标签处理器
   - 分析请求的路径、头部、方法、查询参数、请求体等特征
   - 生成请求的标签集合（如：`[api-v1, claude-3, json-request]`）
4. **端点选择**: 基于标签匹配和优先级选择最佳端点
5. **请求转发**: 将请求转发到选中的上游端点，添加相应的认证信息
6. **响应处理**: 
   - 验证响应格式是否符合 Anthropic 协议
   - 自动解压 gzip 内容
   - 记录完整的请求/响应日志到 SQLite 数据库
7. **故障处理**: 如果请求失败，自动切换到下一个匹配的端点

### 故障检测机制

端点被标记为不可用的条件：
- 在 **140秒** 的滑动窗口内
- 有 **超过1个** 请求失败
- **且该窗口内所有请求都失败**

这种设计避免了因单次超时（通常60秒）就切换端点的问题。

### 响应验证

代理会验证上游响应是否符合 Anthropic API 格式：

**标准响应验证**:
- 必须包含 `id`, `type`, `content`, `model` 等字段
- `type` 字段必须为 `"message"`
- `role` 字段必须为 `"assistant"`

**流式响应验证**:
- 验证 SSE (Server-Sent Events) 格式
- 检查事件类型: `message_start`, `content_block_start`, `content_block_delta`, `message_stop` 等
- 验证每个数据包的 JSON 格式

## 安装使用

### 1. 编译程序

```bash
# 克隆项目
git clone <repository-url>
cd claude-proxy

# 编译
go build -o claude-proxy cmd/main.go

# 或使用 Makefile
make build
```

### 2. 配置文件

复制示例配置文件：

```bash
cp config.yaml.example config.yaml
```

编辑 `config.yaml`，配置您的端点信息：

```yaml
server:
    port: 8080
    auth_token: your-proxy-secret-token

endpoints:
    - name: primary-endpoint
      url: https://api.anthropic.com
      endpoint_type: anthropic   # 端点类型：anthropic | openai
      auth_type: api_key
      auth_value: sk-ant-api03-your-api-key
      enabled: true
      priority: 1
```

### 3. 启动服务

```bash
./claude-proxy -config config.yaml
```

或直接使用默认配置文件：

```bash
./claude-proxy
```

### 4. 配置 Claude Code

将 Claude Code 的 API 端点配置为：

```
API URL: http://localhost:8080
API Key: your-proxy-secret-token
```

## 配置说明

### 服务器配置

```yaml
server:
    host: 127.0.0.1               # 监听地址 (127.0.0.1=仅本地, 0.0.0.0=所有接口)
    port: 8080                    # 代理服务监听端口
    auth_token: your-secret       # 客户端认证令牌
```

**监听地址说明**:
- `127.0.0.1`: 仅本地访问，推荐用于开发和个人使用
- `0.0.0.0`: 监听所有网络接口，可以被局域网内其他设备访问

### 端点配置

```yaml
endpoints:
    - name: endpoint-name         # 端点名称（用于日志和管理）
      url: https://api.example.com # 上游 API 基础URL
      endpoint_type: anthropic   # 端点类型：anthropic | openai
      auth_type: api_key          # 认证类型: api_key | auth_token
      auth_value: your-key        # 认证值
      enabled: true               # 是否启用
      priority: 1                 # 优先级（数字越小优先级越高）
      tags: [api-v1, claude-3]    # 端点支持的标签列表（可选）
```

**端点标签配置**:
- `tags: []` 或省略 tags 字段: **万能端点**，接受所有请求
- `tags: [api-v1]`: 只接受包含 `api-v1` 标签的请求
- `tags: [api-v1, claude-3]`: 只接受同时包含 `api-v1` 和 `claude-3` 标签的请求
- 标签配置支持 **热更新**，修改后立即生效

**认证类型说明**:
- `api_key`: 使用 `x-api-key` 头部，值为 `auth_value`
- `auth_token`: 使用 `Authorization` 头部，值为 `Bearer {auth_value}`

### 日志配置

```yaml
logging:
    level: info                   # 日志级别: debug | info | warn | error
    log_request_types: failed     # 记录请求类型: failed | success | all
    log_request_body: truncated   # 请求体记录: none | truncated | full
    log_response_body: truncated  # 响应体记录: none | truncated | full
    log_directory: ./logs         # 日志存储目录
```

**说明**: 系统使用 SQLite 数据库存储日志，自动创建 `logs.db` 文件，支持：
- **自动清理**: 30天自动删除旧日志
- **结构化查询**: 支持按时间、端点、状态等条件查询
- **标签记录**: 记录每个请求的所有标签信息
- **性能监控**: 统计请求成功率、响应时间等指标

### 标签系统配置

```yaml
tagging:
    enabled: true                 # 启用标签系统
    pipeline_timeout: 5s          # 标签处理超时时间
    taggers:
        # Path Tagger - 匹配HTTP请求路径
        - name: api-v1-detector
          type: builtin
          builtin_type: path
          tag: api-v1
          enabled: true
          priority: 1
          config:
            path_pattern: /v1/*   # 匹配所有/v1/开头的路径
        
        # Body JSON Tagger - 匹配JSON请求体中的字段值
        - name: claude-3-detector
          type: builtin
          builtin_type: body-json
          tag: claude-3
          enabled: true
          priority: 2
          config:
            json_path: model      # JSON路径，支持嵌套如 data.model
            expected_value: claude-3*  # 期望值，支持通配符
        
        # Header Tagger - 匹配HTTP请求头部
        - name: content-type-detector
          type: builtin
          builtin_type: header
          tag: json-request
          enabled: false
          priority: 3
          config:
            header_name: Content-Type      # 头部字段名
            expected_value: application/json  # 期望值，支持通配符
        
        # Method Tagger - 匹配HTTP请求方法
        - name: post-method-detector
          type: builtin
          builtin_type: method
          tag: post-request
          enabled: false
          priority: 4
          config:
            methods: [POST, PUT]  # 支持的HTTP方法列表
        
        # Query Tagger - 匹配URL查询参数
        - name: beta-feature-detector
          type: builtin
          builtin_type: query
          tag: beta-feature
          enabled: false
          priority: 5
          config:
            param_name: beta      # 查询参数名
            expected_value: "true"  # 期望值，支持通配符
        
        # Starlark Tagger - 自定义脚本逻辑
        - name: custom-detector
          type: starlark
          tag: custom-tag
          enabled: false
          priority: 6
          config:
            script: |-           # 内联Starlark脚本
                def should_tag():
                    # 检查请求头
                    if "anthropic-beta" in request.headers:
                        return True
                    # 检查路径
                    if "beta" in lower(request.path):
                        return True
                    return False
            # 或使用外部脚本文件
            # script_file: /path/to/custom.star
```

**内置标签处理器类型**:

1. **`path`**: 匹配HTTP请求路径，支持通配符模式
2. **`header`**: 匹配HTTP请求头部字段值
3. **`method`**: 匹配HTTP请求方法（GET、POST等）
4. **`query`**: 匹配URL查询参数值
5. **`body-json`**: 匹配JSON请求体中的字段值，支持嵌套路径

**Starlark脚本功能**:
- 支持完整的 Starlark 语法
- 提供 `request` 对象访问请求信息
- 内置函数：`lower()`, `contains()`, `matches()` 等
- 3秒执行超时保护
- 支持内联脚本和外部脚本文件

### 验证配置

```yaml
validation:
    strict_anthropic_format: true # 严格验证 Anthropic 响应格式
    validate_streaming: true      # 验证流式响应格式
    disconnect_on_invalid: true   # 响应格式无效时断开连接
```

### Web 管理界面

```yaml
web_admin:
    enabled: true                 # 启用 Web 管理界面（与主服务器共用端口）
```

**说明**: Web 管理界面现在与代理服务器合并到同一个端口，通过 `/admin/` 路径访问。

## Web 管理界面

访问 `http://127.0.0.1:8080/admin/` 可以进行完整的系统管理：

### 📊 Dashboard - 系统概览
- **实时状态监控**: 端点健康状态、请求统计、成功率
- **标签统计**: 各标签的使用频率和匹配情况
- **性能指标**: 系统整体性能和响应时间统计
- **故障告警**: 失败端点和异常请求高亮显示

### 🔗 端点管理
- **端点CRUD**: 添加、编辑、删除端点配置
- **标签配置**: 直接在Web界面配置端点支持的标签
- **实时切换**: 启用/禁用端点，立即生效
- **健康监控**: 查看各端点的健康状态和统计信息
- **热更新**: 端点配置修改后立即生效，无需重启

### 🏷️ 标签管理
- **Tagger管理**: 创建、编辑、删除标签处理器
- **内置Tagger**: 支持所有5种内置类型的图形化配置
- **Starlark编辑器**: 内置代码编辑器，支持语法高亮和验证
- **实时测试**: 测试Tagger规则是否正确匹配
- **执行统计**: 查看各Tagger的执行情况和匹配统计

### 📋 日志查看
- **结构化查询**: 基于SQLite的高效日志搜索和过滤
- **标签显示**: 每个请求的标签信息详细展示
- **请求详情**: 完整的请求/响应头部和正文查看
- **JSON格式化**: 自动格式化JSON内容，便于阅读
- **流式响应**: 流式响应每行自动换行显示
- **统计分析**: 成功率、响应时间、端点分布等统计图表

### ⚙️ 系统设置
- **配置管理**: 直接编辑服务器和日志配置
- **热重载**: 支持的配置项目修改后立即生效
- **数据库管理**: 查看日志数据库大小、清理旧日志
- **导入导出**: 配置文件的备份和恢复功能

## 标签路由使用场景

### 场景一：API版本路由
```yaml
endpoints:
    - name: v1-api
      url: https://api-v1.example.com
      tags: [api-v1]
    - name: v2-api  
      url: https://api-v2.example.com
      tags: [api-v2]

taggers:
    - name: v1-detector
      builtin_type: path
      tag: api-v1
      config:
        path_pattern: /v1/*
    - name: v2-detector
      builtin_type: path
      tag: api-v2
      config:
        path_pattern: /v2/*
```

### 场景二：模型专用端点
```yaml
endpoints:
    - name: claude3-endpoint
      url: https://claude3-api.example.com
      tags: [claude-3]
    - name: general-endpoint
      url: https://general-api.example.com
      tags: []  # 万能端点，处理其他所有请求

taggers:
    - name: claude3-detector
      builtin_type: body-json
      tag: claude-3
      config:
        json_path: model
        expected_value: claude-3*
```

### 场景三：实验功能路由
```yaml
endpoints:
    - name: beta-endpoint
      url: https://beta-api.example.com
      tags: [beta-feature]
    - name: stable-endpoint
      url: https://stable-api.example.com
      tags: []

taggers:
    - name: beta-detector
      type: starlark
      tag: beta-feature
      config:
        script: |-
          def should_tag():
              # 检查头部的beta标识
              if request.headers.get("anthropic-beta"):
                  return True
              # 检查查询参数
              if request.query.get("experimental") == "true":
                  return True
              return False
```

### 场景四：负载分流
```yaml
endpoints:
    - name: high-priority
      url: https://premium-api.example.com
      tags: [premium, claude-3]
    - name: standard
      url: https://standard-api.example.com
      tags: [claude-3]

taggers:
    - name: premium-user-detector
      builtin_type: header
      tag: premium
      config:
        header_name: X-User-Tier
        expected_value: premium
    - name: claude3-detector
      builtin_type: body-json
      tag: claude-3
      config:
        json_path: model
        expected_value: claude-3*
```

**工作原理说明**:
- Premium用户的Claude-3请求会路由到高优先级端点
- 普通用户的Claude-3请求会路由到标准端点
- 其他请求回退到万能端点处理

## 故障排除

### 常见问题

1. **端点频繁切换**
   - 检查网络连接和上游 API 状态
   - 适当调整日志级别查看详细错误信息

2. **响应格式验证失败**
   - 确认上游 API 返回的是标准 Anthropic 格式
   - 可临时关闭 `strict_anthropic_format` 进行调试

3. **标签路由不工作**
   - 检查 `tagging.enabled` 是否为 `true`
   - 确认 Tagger 的 `enabled` 状态
   - 查看日志中的标签生成情况
   - 验证端点的 `tags` 配置是否正确

4. **Starlark脚本执行失败**
   - 检查脚本语法是否正确
   - 确认 `should_tag()` 函数已定义
   - 查看3秒超时是否足够
   - 使用 `debug` 日志级别查看详细错误信息

5. **请求总是路由到万能端点**
   - 检查是否有标签匹配的端点可用
   - 确认 Tagger 是否正确生成了标签
   - 验证端点的健康状态

### 调试模式

设置日志级别为 `debug` 可获得详细的运行信息：

```yaml
logging:
    level: debug
    log_request_types: all
    log_request_body: full
    log_response_body: full
```

### 日志位置

- **SQLite数据库**: `./logs/logs.db`
- **系统日志**: 控制台输出
- **自动清理**: 30天自动删除旧日志记录

## API 参考

### 管理 API 端点

所有管理API都通过 `/admin/api/` 路径访问：

#### 端点管理
```http
GET    /admin/api/endpoints          # 获取所有端点状态
PUT    /admin/api/hot-update         # 热更新端点配置
```

**热更新请求示例**:
```json
PUT /admin/api/hot-update
Content-Type: application/json

{
  "endpoints": [
    {
      "name": "primary",
      "url": "https://api.anthropic.com",
      "endpoint_type": "anthropic",
      "auth_type": "api_key",
      "auth_value": "sk-ant-xxx",
      "enabled": true,
      "priority": 1,
      "tags": ["api-v1", "claude-3"]
    }
  ],
  "logging": {
    "level": "info",
    "log_request_types": "failed"
  }
}
```

#### 标签管理
```http
GET    /admin/api/taggers            # 获取所有标签处理器
POST   /admin/api/taggers            # 创建新的标签处理器
PUT    /admin/api/taggers/{name}     # 更新标签处理器
DELETE /admin/api/taggers/{name}     # 删除标签处理器
GET    /admin/api/tags               # 获取所有已注册标签
```

**创建Tagger请求示例**:
```json
POST /admin/api/taggers
Content-Type: application/json

{
  "name": "my-custom-tagger",
  "type": "builtin",
  "builtin_type": "path",
  "tag": "my-tag",
  "enabled": true,
  "priority": 1,
  "config": {
    "path_pattern": "/custom/*"
  }
}
```

#### 日志查询
```http
GET /admin/api/logs?limit=50&offset=0&failed_only=false&endpoint=&start_time=&end_time=
```

**响应示例**:
```json
{
  "logs": [
    {
      "id": 1,
      "timestamp": "2025-01-01T12:00:00Z",
      "request_id": "req-12345",
      "endpoint": "primary",
      "method": "POST",
      "path": "/v1/messages",
      "status_code": 200,
      "duration_ms": 1500,
      "tags": ["api-v1", "claude-3"],
      "is_streaming": false,
      "model": "claude-3-5-sonnet-20241022",
      "request_body_size": 1024,
      "response_body_size": 2048,
      "request_headers": {...},
      "request_body": "...",
      "response_headers": {...},
      "response_body": "...",
      "error": null
    }
  ],
  "total": 1000,
  "summary": {
    "total_requests": 1000,
    "failed_requests": 50,
    "success_rate": 0.95,
    "avg_duration_ms": 1200
  }
}
```

## 高级功能

### 配置热更新机制

**支持热更新的配置**:
- ✅ 端点配置（URL、认证、标签等）
- ✅ 端点启用/禁用状态  
- ✅ 日志级别和记录设置
- ❌ 标签处理器配置（需要重启）

**热更新API使用**:
```bash
# 更新端点配置
curl -X PUT http://localhost:8080/admin/api/hot-update \
  -H "Authorization: Bearer your-auth-token" \
  -H "Content-Type: application/json" \
  -d @new-config.json

# 配置会立即生效，同时写入配置文件
```

### SQLite 日志存储

**数据库特性**:
- **自动索引**: 时间戳、端点、状态码等字段建立索引
- **并发安全**: 支持多线程安全读写
- **自动清理**: 每24小时清理30天前的日志
- **性能优化**: 使用连接池和事务批处理

**日志表结构**:
```sql
CREATE TABLE request_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    request_id TEXT,
    endpoint TEXT,
    method TEXT,
    path TEXT,
    status_code INTEGER,
    duration_ms INTEGER,
    tags TEXT,  -- JSON数组格式
    is_streaming BOOLEAN,
    model TEXT,
    request_body_size INTEGER,
    response_body_size INTEGER,
    request_headers TEXT,
    request_body TEXT,
    response_headers TEXT,
    response_body TEXT,
    error TEXT
);
```

### 标签处理器执行机制

**并发执行**:
- 所有启用的标签处理器并发执行
- 使用goroutine和WaitGroup确保并发安全
- 5秒总超时限制，超时不影响已完成的结果

**错误隔离**:
- 单个标签处理器失败不影响其他处理器
- 标签系统故障不影响基本代理功能
- 完整的错误日志记录和诊断信息

**性能优化**:
- 标签匹配使用O(1)算法
- 端点选择基于预排序列表
- 读写锁分离，支持高并发访问

### Starlark 脚本环境

**内置变量**:
```python
request.method      # HTTP方法
request.path        # 请求路径  
request.query       # 查询参数字典
request.headers     # 请求头字典
request.body        # 请求体字符串
```

**内置函数**:
```python
lower(s)           # 转换为小写
upper(s)           # 转换为大写
contains(s, sub)   # 检查子字符串
matches(s, pattern) # 正则匹配
json_get(obj, path) # JSON路径提取
```

**脚本示例**:
```python
def should_tag():
    # 复杂的多条件判断
    if request.method == "POST":
        if "beta" in lower(request.path):
            return True
        if request.headers.get("x-experimental"):
            return True
        # 检查JSON体中的特定字段
        if "claude-3.5" in request.body:
            return True
    return False
```

## 性能和监控

### 系统指标

**端点统计**:
- 总请求数、成功数、失败数
- 平均响应时间、最大响应时间
- 成功率趋势图表
- 健康状态历史

**标签统计**:
- 各标签的匹配频率
- 标签处理器执行时间
- 标签路由效果分析

**数据库统计**:
- 日志总数和数据库大小
- 查询性能指标
- 自动清理统计

### 性能建议

**生产环境配置**:
```yaml
logging:
    level: info                    # 减少日志输出
    log_request_types: failed      # 只记录失败请求
    log_request_body: truncated    # 截断请求体
    log_response_body: none        # 不记录响应体

tagging:
    pipeline_timeout: 3s           # 缩短标签处理超时时间
```

**高负载优化**:
- 禁用不必要的标签处理器
- 使用万能端点减少标签匹配开销
- 定期清理日志数据库
- 监控内存使用情况

## 安全注意事项

- **配置文件安全**: 包含API密钥等敏感信息，请设置适当的文件权限
- **网络访问控制**: 建议仅监听本地地址 (`127.0.0.1`)  
- **认证令牌**: 使用强随机字符串作为 `auth_token`
- **日志隐私**: 注意日志中可能包含敏感请求数据，合理配置日志级别
- **脚本安全**: Starlark脚本在沙箱环境中执行，但仍需谨慎编写

## 许可证

本项目基于 MIT 许可证开源。
# Claude API Proxy

一个专为 Claude Code 设计的本地 API 代理服务，提供负载均衡、故障转移和响应验证功能。

## 功能特性

- 🔄 **多端点负载均衡**: 支持配置多个上游 Anthropic API 端点，按优先级进行故障转移
- 🛡️ **响应格式验证**: 验证上游 API 响应是否符合 Anthropic 协议格式，异常时自动断开连接
- 📊 **智能故障检测**: 140秒窗口内的请求失败率检测，避免误判单次超时
- 📦 **内容解压透传**: 自动处理 gzip 压缩响应，解压后透传给客户端
- 🔍 **完整请求日志**: 记录所有请求/响应详情，支持 Web 界面查看
- ⚡ **健康检查机制**: 自动检测不可用端点并在恢复后重新启用
- 🌐 **Web 管理界面**: 提供端点状态监控、日志查看和配置管理

## 工作原理

### 系统架构

```
客户端 (Claude Code)
       ↓
   本地代理服务器 (8080)
       ↓
   端点选择器 (按优先级)
       ↓
┌─────────────────────────────┐
│ 端点1 (优先级1)  端点2 (优先级2) │
│     ↓              ↓        │
│ 上游API1        上游API2     │
└─────────────────────────────┘
```

### 核心工作流程

1. **请求接收**: 客户端向本地代理 (默认8080端口) 发送请求
2. **身份验证**: 使用配置的 `auth_token` 进行本地认证
3. **端点选择**: 按优先级选择第一个可用的上游端点
4. **请求转发**: 将请求转发到选中的上游端点，添加相应的认证信息
5. **响应处理**: 
   - 验证响应格式是否符合 Anthropic 协议
   - 自动解压 gzip 内容
   - 记录完整的请求/响应日志
6. **故障处理**: 如果请求失败，自动切换到下一个可用端点

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
      path_prefix: /v1
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
    port: 8080                    # 代理服务监听端口
    auth_token: your-secret       # 客户端认证令牌
```

### 端点配置

```yaml
endpoints:
    - name: endpoint-name         # 端点名称（用于日志和管理）
      url: https://api.example.com # 上游 API 基础URL
      path_prefix: /v1            # 路径前缀
      auth_type: api_key          # 认证类型: api_key | auth_token
      auth_value: your-key        # 认证值
      enabled: true               # 是否启用
      priority: 1                 # 优先级（数字越小优先级越高）
```

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
    enabled: true                 # 启用 Web 管理界面
    host: 127.0.0.1              # 监听地址（建议仅本地访问）
    port: 8081                    # 管理界面端口
```

## Web 管理界面

访问 `http://127.0.0.1:8081` 可以：

- 📊 **查看端点状态**: 实时监控各端点的健康状态和请求统计
- 📋 **浏览请求日志**: 查看详细的请求/响应日志，支持过滤和搜索
- ⚙️ **管理端点配置**: 添加、编辑、启用/禁用端点
- 📈 **监控系统状态**: 查看系统运行状态和性能指标

## 故障排除

### 常见问题

1. **端点频繁切换**
   - 检查网络连接和上游 API 状态
   - 适当调整日志级别查看详细错误信息

2. **响应格式验证失败**
   - 确认上游 API 返回的是标准 Anthropic 格式
   - 可临时关闭 `strict_anthropic_format` 进行调试

3. **健康检查失败**
   - 检查端点 URL 和认证信息是否正确
   - 确认端点支持 `/v1/messages` 接口

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

- 默认日志目录: `./logs/`
- 日志文件按日期命名
- 包含完整的请求/响应详情

## 性能建议

- 生产环境建议将日志级别设为 `info` 或 `warn`
- 大量请求时可考虑设置 `log_request_body: truncated`
- 定期清理日志目录以节省磁盘空间

## 安全注意事项

- 配置文件包含敏感信息，请妥善保管
- 建议仅在本地环境使用 (`127.0.0.1`)
- 定期轮换 API 密钥和认证令牌

## 许可证

本项目基于 MIT 许可证开源。
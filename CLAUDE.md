# Claude Code Companion 项目说明

Claude Code Companion 是一个多协议 API 代理服务，为 Claude Code 等客户端提供统一的 API 访问入口。

## 核心功能

- **多端点支持**：支持 Anthropic API、OpenAI 兼容 API 等多种端点类型
- **格式转换**：自动转换 OpenAI 请求格式为 Anthropic 格式
- **故障转移**：智能端点切换和健康检查
- **OAuth 认证**：支持自动 token 刷新机制
- **模型重写**：动态重写请求中的模型名称
- **标签路由**：基于请求特征的智能路由
- **Web 管理**：图形化管理界面，支持多语言

## 快速开始

### 构建和运行

```bash
# 构建
make build

# 运行
./claude-proxy -config config.yaml

# 或指定端口
./claude-proxy --port 8080
```

### 基本配置

```yaml
server:
  host: 0.0.0.0
  port: 8080

endpoints:
  - name: "primary"
    url: "https://api.anthropic.com"
    endpoint_type: "anthropic"
    auth_type: "api_key"
    auth_value: "sk-ant-..."
    enabled: true
    priority: 1

logging:
  level: "info"
  log_directory: "./logs"

validation:
  strict_anthropic_format: false
```

## 访问方式

- **代理服务**：`http://localhost:8080`
- **管理界面**：`http://localhost:8080/admin/`

## 详细文档

完整的实现细节和配置说明请参考：

- **[DESIGN.md](./DESIGN.md)** - 详细的技术实现文档
- **[config.yaml.example](./config.yaml.example)** - 完整配置示例

# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

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
./claude-code-companion -config config.yaml

# 或指定端口
./claude-code-companion --port 8080
```

### 开发常用命令

```bash
# 开发模式（热重载）
make dev

# 运行测试
make test

# 代码格式化
make fmt

# 代码检查（需要安装 golangci-lint）
make lint

# 构建所有平台
make all

# 清理构建产物
make clean
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
  # 严格 Anthropic 格式校验和流式响应校验已永久启用
```

## 访问方式

- **代理服务**：`http://localhost:8080`
- **管理界面**：`http://localhost:8080/admin/`

## 代码架构

### 核心组件

1. **端点管理器 (endpoint.Manager)** - 管理所有上游端点，支持多种端点类型和优先级路由
2. **格式转换器 (conversion.Converter)** - 实现不同 API 格式之间的转换（OpenAI ↔ Anthropic）
3. **OAuth 认证管理 (oauth.OAuth)** - 支持自动 token 刷新机制
4. **模型重写器 (modelrewrite.Rewriter)** - 动态重写请求中的模型名称
5. **标签系统 (tagging.Manager)** - 基于请求特征的智能路由和分类
6. **健康检查器 (health.Checker)** - 定期监控端点健康状态
7. **响应验证器 (validator.ResponseValidator)** - 验证上游 API 响应格式

### 目录结构

```
├── internal/                 # 核心代码
│   ├── proxy/               # 代理核心逻辑
│   ├── endpoint/            # 端点管理
│   ├── conversion/          # 格式转换
│   ├── oauth/               # OAuth 认证
│   ├── modelrewrite/        # 模型重写
│   ├── tagging/             # 标签系统
│   ├── health/              # 健康检查
│   ├── validator/           # 响应验证
│   ├── config/              # 配置管理
│   ├── logger/              # 日志系统
│   └── web/                 # Web 管理界面
├── web/                     # Web 前端资源
├── docs/                    # 文档
└── main.go                  # 程序入口
```

## 详细文档

完整的实现细节和配置说明请参考：

- **[DESIGN.md](./docs/DESIGN.md)** - 详细的技术实现文档
- **[config.yaml.example](./config.yaml.example)** - 完整配置示例
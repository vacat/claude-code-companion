# Claude API Proxy 项目架构文档

## 项目概述

本项目是一个为 Claude Code 设计的本地 API 代理服务，主要解决以下问题：

1. **响应格式验证**：上游 API 有时返回 HTTP 200 但内容不符合 Anthropic 协议格式，代理需要检测并断开连接，让客户端重连
2. **多端点负载均衡**：支持配置多个上游 Anthropic 端点，提供故障切换和负载分发能力
3. **内容解压透传**：自动处理 gzip 压缩响应，解压后透传给客户端，确保客户端能正确解析

**详细实现规范请参考 [DESIGN.md](./DESIGN.md)**

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

3. **响应验证器** (`validator/response.go`)

   - 验证上游响应格式
   - Anthropic 协议兼容性检查
   - 异常响应处理

4. **Web 管理界面** (`web/admin.go`)

   - 端点配置管理
   - 请求日志查看
   - 系统状态监控

5. **日志系统** (`logger/logger.go`)

   - 请求/响应日志记录（完整记录，无截断）
   - 错误日志和调试信息

6. **标签系统** (`tagging/`)

   - 请求标记和分类功能
   - 支持多个 tagger 对请求进行标记
   - 基于标签的路由和端点选择

### 技术栈

- **语言**：Go 1.19+
- **Web 框架**：Gin (HTTP 服务) + 原生 net/http
- **前端界面**：HTML + JavaScript + Bootstrap（嵌入到二进制）
- **配置文件**：YAML 格式
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

基本的管理接口：

- 端点状态查看
- 端点配置管理
- 请求日志查询
- 系统设置修改

_详细的 API 规范请参考 [DESIGN.md](./DESIGN.md)_

## 配置文件结构

主要配置分为几个部分：

- **server**: 代理服务器配置（端口、认证 token）
- **endpoints**: 上游端点列表（URL、认证信息、优先级）
- **logging**: 日志记录配置
- **validation**: 响应格式验证设置
- **web_admin**: Web 管理界面配置
- **tagging**: 标签系统配置（可选）

_完整的配置文件样例请参考 [DESIGN.md](./DESIGN.md)_

## 错误处理和端点切换机制

### 核心机制

1. **响应格式验证**: 检查上游 API 返回是否符合 Anthropic 协议格式
2. **端点故障检测**: 基于请求失败率自动标记不可用端点
3. **优先级切换**: 按配置的优先级顺序选择可用端点
4. **502 错误返回**: 所有端点不可用时返回 Bad Gateway
5. **内容解压处理**: 自动处理 gzip 压缩响应并验证格式

_详细的实现代码和逻辑请参考 [DESIGN.md](./DESIGN.md)_

## Web 管理界面

### 页面结构

1. **主 Dashboard**: 端点状态概览、请求统计、错误日志
2. **端点配置页**: 端点管理、连通性测试
3. **日志查看页**: 请求日志查询、详情查看
4. **系统设置页**: 服务器配置、日志配置
5. **标签管理页**: tagger 管理、标签统计（可选）

### 特性

- 本地访问（127.0.0.1）
- 无需用户认证
- 响应式设计
- 实时状态更新
- JSON 格式美化显示

_详细的页面设计和功能说明请参考 [DESIGN.md](./DESIGN.md)_

## 项目结构

```
claude-proxy/
├── cmd/
│   └── main.go                 # 程序入口
├── internal/
│   ├── config/                 # 配置管理
│   ├── proxy/                  # HTTP代理服务器
│   ├── endpoint/               # 端点管理
│   ├── validator/              # 响应格式验证
│   ├── logger/                 # 日志系统
│   ├── tagging/                # 标签系统（可选）
│   └── web/                    # Web管理界面
├── web/                        # 前端资源
├── config.yaml                 # 配置文件
├── go.mod
└── Makefile                    # 构建脚本
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

### 使用方式

1. 启动代理服务
2. 配置客户端指向本地代理端点
3. 通过 Web 界面管理端点和查看日志
4. 代理会自动处理故障切换和格式验证

Run multiple Task invocations in a SINGLE message

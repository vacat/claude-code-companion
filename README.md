# Claude Code 伴侣

Claude Code 伴侣是一个为 Claude Code 提供的本地 API 代理工具。它通过管理多个上游端点、验证返回格式并在必要时自动切换端点，提升代理的稳定性与可观测性，同时提供完整的 Web 管理界面，方便新手快速上手与维护。

## 核心功能

- 多端点负载均衡与故障转移：支持配置多个上游服务（端点），按优先级尝试并自动切换不可用端点。
- 响应格式验证：校验上游返回是否满足 Anthropic 协议，遇到异常响应可断开并触发重连。
- OpenAI 兼容节点接入：通过“OpenAI 兼容”类型可将 GPT5、GLM、K2 等模型接入 Claude Code 使用。
- 智能故障检测：自动标记异常端点并在后台检测恢复情况。
- 智能标签路由：基于请求路径、头部或内容的动态路由规则，支持按标签选择端点。
- 请求日志与可视化管理：记录完整请求/响应日志，提供端点管理、日志查看与系统监控的 Web 界面。

## 快速开始（面向新手）

[一个带图的配置多个号池入口的例子文档](https://ucn0s6hcz1w1.feishu.cn/docx/PkCGd4qproRu80xr2yBcz1PinVe)

## 快速开始

1. 下载并解压

   - 从 Release 页面下载对应操作系统的压缩包，解压后进入目录。

2. 第一次运行

   - 直接执行程序（Linux/Windows 下的二进制文件），程序会在当前目录生成默认配置文件 config.yaml。

3. 打开管理界面

   - 在浏览器访问： http://localhost:8080/admin
   - 管理界面提供端点配置、标签规则、日志查看和系统设置。

4. 添加上游端点

   - 进入 Admin → Endpoints，点击新增并填写上游 URL、鉴权信息与类型（例如 Anthropic 或 OpenAI 兼容）。
   - 拖拽可调整优先级，配置实时生效。

5. 在 Claude Code 中使用 Claude Code Companion

   - 将 ANTHROPIC_BASE_URL 环境变量指向代理地址（例如 http://localhost:8080/）
   - ANTHROPIC_AUTH_TOKEN 可以随便设置一个，但是不能不设置
   - 还需要设置 API_TIMEOUT_MS=600000 ，这样才能在号池超时的时候，客户端自己不超时
   - 建议设置 CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 ，可以避免 claude code 往他们公司报东西

## 🆕 环境变量支持

Claude Code Companion 现在支持在配置文件中使用环境变量，提升安全性：

```yaml
endpoints:
  - name: anthropic-prod
    auth_value: "${ANTHROPIC_API_KEY:sk-ant-your-key-here}"
  - name: openai-prod  
    auth_value: "${OPENAI_API_KEY:sk-your-openai-key}"
```

启动前设置环境变量：
```bash
export ANTHROPIC_API_KEY="your-real-key"
export OPENAI_API_KEY="your-real-key"
./claude-code-companion -config config.yaml
```

详细使用方法请参考：[环境变量支持文档.md](./环境变量支持文档.md)

## 一些文档

[常见端点提供商的参数参考](https://ucn0s6hcz1w1.feishu.cn/sheets/RNPHswfIThqQ1itf1m4cb0mKnrc)

[深入理解TAG系统和一些实际案例](https://ucn0s6hcz1w1.feishu.cn/docx/YTvYdv7kzodpr9xZ2RXcGOc5n3c)

## Docker 部署

Claude Code Companion 提供完整的 Docker 支持，方便在容器环境中部署和管理。

### 🚀 快速启动（推荐新手）

项目提供了一键启动脚本，让 Docker 部署变得更简单：

```bash
# 一键启动（推荐）
./docker-start.sh start

# 查看状态
./docker-start.sh status

# 查看实时日志
./docker-start.sh logs

# 停止服务
./docker-start.sh stop
```

### 方式一：使用 Docker

1. **构建 Docker 镜像**

   ```bash
   # 使用默认配置构建
   make docker-build
   
   # 或者手动构建
   docker build -t claude-code-companion .
   ```

2. **运行 Docker 容器**

   ```bash
   # 使用 Makefile 命令（推荐）
   make docker-run
   
   # 或者手动运行
   docker run -d --name claude-code-companion \
     -p 8080:8080 \
     -v $(pwd)/config.docker.yaml:/app/config/config.yaml:ro \
     -v $(pwd)/logs:/app/logs \
     -e ANTHROPIC_API_KEY="your-api-key" \
     claude-code-companion:latest
   ```

3. **访问服务**
   - 代理服务：http://localhost:8080/
   - 管理界面：http://localhost:8080/admin/

### 方式二：使用 Docker Compose（推荐）

1. **启动服务**

   ```bash
   # 使用 Makefile 命令
   make docker-compose-up
   
   # 或者直接使用 docker-compose
   docker-compose up -d
   ```

2. **查看服务状态**

   ```bash
   docker-compose ps
   docker-compose logs -f claude-code-companion
   ```

3. **停止服务**

   ```bash
   # 使用 Makefile 命令
   make docker-compose-down
   
   # 或者直接使用 docker-compose
   docker-compose down
   ```

### Docker 环境配置

项目包含专门为 Docker 环境优化的配置文件 `config.docker.yaml`，主要特点：

- **容器网络优化**：监听所有接口 (`0.0.0.0:8080`)
- **日志格式**：使用 JSON 格式便于日志收集
- **环境变量支持**：支持通过环境变量传递 API 密钥
- **健康检查**：内置健康检查端点
- **持久化存储**：日志和数据库文件挂载到主机

### 环境变量配置

在 Docker 环境中，你可以通过环境变量传递敏感信息：

```bash
# 设置环境变量
export ANTHROPIC_API_KEY="your-anthropic-key"
export OPENAI_API_KEY="your-openai-key"

# 启动服务
docker-compose up -d
```

### 生产环境部署建议

1. **资源限制**：`docker-compose.yml` 已配置合理的资源限制
2. **日志管理**：使用 Docker 的日志驱动或外部日志收集系统
3. **监控**：可以添加 Prometheus 等监控服务（配置文件中已有示例）
4. **备份**：定期备份配置文件和日志数据
5. **安全**：在生产环境中更改默认的 CSRF 密钥

### Docker 相关命令

项目的 Makefile 提供了便捷的 Docker 管理命令：

```bash
make docker-build         # 构建 Docker 镜像
make docker-run           # 运行 Docker 容器
make docker-compose-up    # 启动 Docker Compose 服务
make docker-compose-down  # 停止 Docker Compose 服务
make docker-push          # 推送镜像到仓库（需要先登录）
```

## 常见使用场景

- 多个号池自动切换：
  - 将多个号池提供的端点信息(目前市面上的号池除了 GAC 是使用 API key 方式认证，其他都使用的是 Auth Token 方式，在号池的配置页面里面可以看到这个信息)，依次添加到端点列表即可。代理会按照顺序自动尝试并在失败时切换。可以通过拖拽来调整尝试顺序，操作是实时生效的。
- 使用第三方模型：
  - 对 GLM 和 K2 这样官方提供了 Anthropic 类型端点入口的，可以直接像添加号池一样添加使用，拖拽到第一个即可生效
  - 对 openrouter 或者火山千问之类只有 OpenAI 兼容入口的，添加端点的时候选择 OpenAI 兼容端点，将默认模型设置为你要的模型名字，然后将这个端点拖拽到第一个，即可使得 Claude Code 使用这个第三方模型

# Docker 部署指南

## 📋 概述

本文档详细介绍了如何使用 Docker 部署 Claude Code Companion。我们为项目添加了完整的容器化支持，让您可以轻松地在任何支持 Docker 的环境中运行服务。

## 🗂️ 新增文件说明

### 核心配置文件

1. **`Dockerfile`** - 多阶段构建配置
   - 使用 Go 1.23 Alpine 作为构建环境
   - 优化的生产镜像（基于 Alpine Linux）
   - 内置健康检查
   - 非 root 用户运行确保安全性

2. **`docker-compose.yml`** - 容器编排配置
   - 一键启动完整环境
   - 卷挂载配置（配置文件、日志目录）
   - 环境变量支持
   - 资源限制配置
   - 健康检查配置

3. **`config.docker.yaml`** - Docker 专用配置
   - 容器网络优化（监听所有接口）
   - JSON 格式日志（便于日志收集）
   - 环境变量集成（支持 `${VAR}` 语法）
   - 合理的超时和重试配置

4. **`.dockerignore`** - 构建优化
   - 排除不必要的文件
   - 减少构建上下文大小
   - 提升构建速度

5. **`docker-start.sh`** - 一键启动脚本
   - 简化 Docker 操作
   - 自动检查依赖
   - 彩色输出和用户友好提示
   - 支持多种操作（启动、停止、查看日志等）

### Makefile 扩展

添加了以下 Docker 相关命令：
- `make docker-build` - 构建 Docker 镜像
- `make docker-run` - 运行 Docker 容器
- `make docker-compose-up` - 启动 Docker Compose 服务
- `make docker-compose-down` - 停止 Docker Compose 服务
- `make docker-push` - 推送镜像到仓库

## 🚀 快速开始

### 方式一：使用快速启动脚本（推荐）

```bash
# 一键启动
./docker-start.sh start

# 查看状态
./docker-start.sh status

# 查看日志
./docker-start.sh logs

# 停止服务
./docker-start.sh stop
```

### 方式二：使用 Docker Compose

```bash
# 启动服务
make docker-compose-up

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 停止服务
make docker-compose-down
```

### 方式三：直接使用 Docker

```bash
# 构建镜像
make docker-build

# 运行容器
make docker-run
```

## 🔧 环境变量配置

支持通过环境变量传递敏感信息：

```bash
# 设置环境变量
export ANTHROPIC_API_KEY="your-anthropic-key"
export OPENAI_API_KEY="your-openai-key"

# 启动服务
./docker-start.sh start
```

配置文件中使用环境变量：

```yaml
endpoints:
  - name: "anthropic-official"
    auth_value: "${ANTHROPIC_API_KEY}"
  - name: "openai-compatible"
    auth_value: "${OPENAI_API_KEY}"
```

## 📊 生产环境部署建议

### 1. 资源配置

`docker-compose.yml` 已包含合理的资源限制：
- 内存限制：512MB（保留 256MB）
- CPU 限制：1.0 核心（保留 0.5 核心）

### 2. 日志管理

```bash
# 配置 Docker 日志驱动
docker run --log-driver=json-file --log-opt max-size=10m --log-opt max-file=3 ...
```

### 3. 持久化存储

重要目录已配置卷挂载：
- `./config.docker.yaml:/app/config/config.yaml` - 配置文件（读写，支持Admin Console修改）
- `./logs:/app/logs` - 日志文件
- SQLite 数据库文件在日志目录中

### 4. 安全配置

- 容器以非 root 用户运行
- 配置文件通过环境变量保护敏感信息
- 建议在生产环境更改默认的 CSRF 密钥

### 5. 监控和健康检查

- 内置健康检查端点：`/admin/health`
- 可以集成 Prometheus 监控（配置文件中有示例）

## 🔍 故障排除

### 常见问题

1. **端口冲突**
   ```bash
   # 修改端口映射
   docker run -p 9080:8080 ...
   ```

2. **权限问题**
   ```bash
   # 确保日志目录权限
   chmod 755 logs/
   ```

3. **配置文件问题**
   ```bash
   # 检查配置文件语法
   docker run --rm -v $(pwd)/config.docker.yaml:/tmp/config.yaml \
     alpine/httpie --yaml /tmp/config.yaml
   ```

4. **网络问题**
   ```bash
   # 检查容器网络
   docker network ls
   docker inspect claude-network
   ```

### 调试命令

```bash
# 进入容器调试
docker exec -it claude-code-companion sh

# 查看容器日志
docker logs claude-code-companion

# 检查容器状态
docker inspect claude-code-companion
```

## 🔄 更新和维护

### 更新镜像

```bash
# 重新构建
make docker-build

# 重启服务
./docker-start.sh restart
```

### 备份数据

```bash
# 备份配置和日志
tar -czf backup-$(date +%Y%m%d).tar.gz config.docker.yaml logs/
```

### 清理资源

```bash
# 清理停止的容器
docker container prune

# 清理未使用的镜像
docker image prune

# 清理构建缓存
docker builder prune
```

## 📈 性能优化

### 1. 构建优化

- 使用 `.dockerignore` 减少构建上下文
- 多阶段构建减少最终镜像大小
- Go 编译优化（`-ldflags "-s -w"`）

### 2. 运行时优化

- Alpine Linux 基础镜像（约 10MB）
- 静态编译的 Go 二进制文件
- 合理的资源限制

### 3. 网络优化

- 容器内监听所有接口（`0.0.0.0`）
- 健康检查配置优化
- 连接池和超时配置

## 📚 相关文档

- [README.md](README.md) - 项目主要文档
- [config.docker.yaml](config.docker.yaml) - Docker 专用配置
- [docker-compose.yml](docker-compose.yml) - 容器编排配置
- [Dockerfile](Dockerfile) - 镜像构建配置

## 💡 贡献指南

如果您在使用 Docker 部署时遇到问题或有改进建议，请：

1. 提交 Issue 描述问题
2. 提供详细的环境信息
3. 包含相关的日志输出
4. 欢迎提交 Pull Request

---

**注意**：本 Docker 支持基于项目的内存信息和最佳实践设计，确保了安全性、可维护性和生产就绪性。
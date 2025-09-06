# Stage 1: Build stage
FROM golang:1.23-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git make

# 设置工作目录
WORKDIR /app

# 配置Go代理和超时设置
ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
ENV GOSUMDB=sum.golang.org
ENV GOTIMEOUT=300s
ENV GO111MODULE=on

# 复制go模块文件
COPY go.mod go.sum ./

# 下载依赖（增加重试和超时设置）
RUN go mod download -x

# 复制源代码
COPY . .

# 构建应用程序
ARG VERSION=docker-build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.Version=${VERSION} -s -w" \
    -o claude-code-companion .

# Stage 2: Final stage
FROM alpine:latest

# 安装CA证书和tzdata用于时区支持
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN adduser -D -s /bin/sh appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/claude-code-companion .

# 复制web资源（如果需要的话）
COPY --from=builder /app/web ./web

# 创建必要的目录
RUN mkdir -p /app/logs /app/config

# 设置文件权限
RUN chown -R appuser:appuser /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/admin/health || exit 1

# 默认命令
CMD ["./claude-code-companion", "-config", "/app/config/config.yaml"]
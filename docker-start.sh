#!/bin/bash

# Claude Code Companion Docker 快速启动脚本
# 此脚本帮助用户快速启动 Docker 容器

set -e  # 出错时退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查Docker是否安装
check_docker() {
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        print_error "Docker 服务未运行，请启动 Docker 服务"
        exit 1
    fi
}

# 检查配置文件
check_config() {
    if [ ! -f "config.docker.yaml" ]; then
        print_warning "config.docker.yaml 不存在，将使用默认配置"
        print_info "请在启动后访问 http://localhost:8080/admin/ 配置端点"
    fi
}

# 检查环境变量
check_env() {
    if [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$OPENAI_API_KEY" ]; then
        print_warning "未设置 API 密钥环境变量"
        print_info "你可以通过以下方式设置："
        echo "  export ANTHROPIC_API_KEY=\"your-key\""
        echo "  export OPENAI_API_KEY=\"your-key\""
        echo ""
    fi
}

# 显示帮助信息
show_help() {
    echo "Claude Code Companion Docker 快速启动脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  start    启动服务 (默认)"
    echo "  stop     停止服务"
    echo "  restart  重启服务"
    echo "  logs     查看日志"
    echo "  build    构建镜像"
    echo "  status   查看状态"
    echo "  help     显示帮助"
    echo ""
    echo "环境变量:"
    echo "  ANTHROPIC_API_KEY  Anthropic API 密钥"
    echo "  OPENAI_API_KEY     OpenAI API 密钥"
    echo ""
    echo "示例:"
    echo "  $0 start"
    echo "  ANTHROPIC_API_KEY=sk-xxx $0 start"
    echo "  $0 logs"
}

# 启动服务
start_service() {
    print_info "正在启动 Claude Code Companion..."
    
    check_docker
    check_config
    check_env
    
    # 创建日志目录
    mkdir -p logs
    
    # 启动服务
    if command -v docker-compose &> /dev/null; then
        print_info "使用 Docker Compose 启动服务..."
        docker-compose up -d
    else
        print_info "使用 Docker 启动服务..."
        
        # 构建镜像（如果不存在）
        if ! docker images claude-code-companion:latest --format "table" | grep -q claude-code-companion; then
            print_info "镜像不存在，正在构建..."
            docker build -t claude-code-companion:latest .
        fi
        
        # 停止现有容器
        docker stop claude-code-companion 2>/dev/null || true
        docker rm claude-code-companion 2>/dev/null || true
        
        # 启动新容器
        docker run -d --name claude-code-companion \
            -p 8080:8080 \
            -v "$(pwd)/config.docker.yaml:/app/config/config.yaml:ro" \
            -v "$(pwd)/logs:/app/logs" \
            -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
            -e OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
            -e TZ="Asia/Shanghai" \
            --restart unless-stopped \
            claude-code-companion:latest
    fi
    
    print_success "服务启动成功！"
    echo ""
    echo "访问地址:"
    echo "  代理服务: http://localhost:8080/"
    echo "  管理界面: http://localhost:8080/admin/"
    echo ""
    echo "查看日志: $0 logs"
    echo "停止服务: $0 stop"
}

# 停止服务
stop_service() {
    print_info "正在停止 Claude Code Companion..."
    
    if command -v docker-compose &> /dev/null && [ -f "docker-compose.yml" ]; then
        docker-compose down
    else
        docker stop claude-code-companion 2>/dev/null || true
        docker rm claude-code-companion 2>/dev/null || true
    fi
    
    print_success "服务已停止"
}

# 重启服务
restart_service() {
    print_info "正在重启 Claude Code Companion..."
    stop_service
    sleep 2
    start_service
}

# 查看日志
show_logs() {
    if command -v docker-compose &> /dev/null && [ -f "docker-compose.yml" ]; then
        docker-compose logs -f claude-code-companion
    else
        docker logs -f claude-code-companion 2>/dev/null || print_error "容器未运行"
    fi
}

# 构建镜像
build_image() {
    print_info "正在构建 Docker 镜像..."
    docker build -t claude-code-companion:latest .
    print_success "镜像构建完成"
}

# 查看状态
show_status() {
    print_info "服务状态:"
    
    if command -v docker-compose &> /dev/null && [ -f "docker-compose.yml" ]; then
        docker-compose ps
    else
        if docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -q claude-code-companion; then
            docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep claude-code-companion
            print_success "服务正在运行"
        else
            print_warning "服务未运行"
        fi
    fi
}

# 主逻辑
case "${1:-start}" in
    "start")
        start_service
        ;;
    "stop")
        stop_service
        ;;
    "restart")
        restart_service
        ;;
    "logs")
        show_logs
        ;;
    "build")
        build_image
        ;;
    "status")
        show_status
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        print_error "未知命令: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
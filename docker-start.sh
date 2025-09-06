#!/bin/bash

# Claude Code Companion Docker 快速启动脚本
# 此脚本帮助用户快速启动 Docker 容器

set -e  # 出错时退出

# 默认配置文件路径
CONFIG_FILE="config.docker.yaml"
ENV_FILE=".env"

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
    # 检查当前目录的配置文件
    if [ -f "$CONFIG_FILE" ]; then
        print_info "使用当前目录配置文件: $CONFIG_FILE"
        return 0
    fi
    
    # 检查 ~/.claude-code-companion 目录的配置文件
    if [ -f "$HOME/.claude-code-companion/$CONFIG_FILE" ]; then
        print_info "使用用户目录配置文件: $HOME/.claude-code-companion/$CONFIG_FILE"
        return 0
    fi
    
    print_warning "$CONFIG_FILE 不存在，将使用默认配置"
    print_info "请在启动后访问 http://localhost:8080/admin/ 配置端点"
}

# 检查环境变量文件
check_env_file() {
    # 检查当前目录的环境文件
    if [ -f "$ENV_FILE" ]; then
        print_info "使用当前目录环境文件: $ENV_FILE"
        return 0
    fi
    
    # 检查 ~/.claude-code-companion 目录的环境文件
    if [ -f "$HOME/.claude-code-companion/$ENV_FILE" ]; then
        print_info "使用用户目录环境文件: $HOME/.claude-code-companion/$ENV_FILE"
        return 0
    fi
    
    print_info "未找到环境文件 $ENV_FILE"
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
    check_env_file
    
    # 创建日志目录
    mkdir -p logs
    
    # 确定使用的配置文件路径
    if [ -f "$CONFIG_FILE" ]; then
        CONFIG_PATH="$(pwd)/$CONFIG_FILE"
    elif [ -f "$HOME/.claude-code-companion/$CONFIG_FILE" ]; then
        CONFIG_PATH="$HOME/.claude-code-companion/$CONFIG_FILE"
    else
        CONFIG_PATH="$(pwd)/$CONFIG_FILE"  # 使用默认路径
    fi
    
    # 确定使用的环境文件路径
    if [ -f "$ENV_FILE" ]; then
        ENV_PATH="$(pwd)/$ENV_FILE"
    elif [ -f "$HOME/.claude-code-companion/$ENV_FILE" ]; then
        ENV_PATH="$HOME/.claude-code-companion/$ENV_FILE"
    else
        ENV_PATH=""  # 不使用环境文件
    fi
    
    # 启动服务
    if command -v docker-compose &> /dev/null; then
        print_info "使用 Docker Compose 启动服务..."
        if [ -n "$ENV_PATH" ] && [ -f "$ENV_PATH" ]; then
            print_info "加载环境文件: $ENV_PATH"
            export $(grep -v '^#' "$ENV_PATH" | xargs)
        fi
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
        
        # 构建挂载参数
        DOCKER_RUN_ARGS=(
            "docker" "run" "-d" "--name" "claude-code-companion"
            "-p" "8080:8080"
            "-v" "$CONFIG_PATH:/app/config/config.yaml"
            "-v" "$(pwd)/logs:/app/logs"
            "-e" "TZ=Asia/Shanghai"
            "--restart" "unless-stopped"
        )
        
        # 添加环境文件中的变量
        if [ -n "$ENV_PATH" ] && [ -f "$ENV_PATH" ]; then
            print_info "加载环境文件: $ENV_PATH"
            while IFS= read -r line; do
                if [[ ! "$line" =~ ^#.* ]] && [[ "$line" =~ .*=. ]]; then
                    DOCKER_RUN_ARGS+=("-e" "$line")
                fi
            done < "$ENV_PATH"
        fi
        
        # 添加命令行环境变量
        if [ -n "$ANTHROPIC_API_KEY" ]; then
            DOCKER_RUN_ARGS+=("-e" "ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY")
        fi
        if [ -n "$OPENAI_API_KEY" ]; then
            DOCKER_RUN_ARGS+=("-e" "OPENAI_API_KEY=$OPENAI_API_KEY")
        fi
        
        # 添加镜像名称
        DOCKER_RUN_ARGS+=("claude-code-companion:latest")
        
        # 启动容器
        "${DOCKER_RUN_ARGS[@]}"
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
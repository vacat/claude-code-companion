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

# 确定工作目录
find_work_dir() {
    local current_dir="$1"
    
    # 检查当前目录是否有 docker-compose.yml
    if [ -f "$current_dir/docker-compose.yml" ]; then
        echo "$current_dir"
        return 0
    fi
    
    # 检查用户目录是否有 docker-compose.yml
    if [ -f "$HOME/.claude-code-companion/docker-compose.yml" ]; then
        echo "$HOME/.claude-code-companion"
        return 0
    fi
    
    # 如果都没有，使用当前目录
    echo "$current_dir"
}

# 检查配置文件是否存在
check_config_files() {
    local work_dir="$1"
    
    # 检查配置文件是否存在
    if [ ! -f "$work_dir/config.yaml" ] && [ ! -f "$work_dir/config.docker.yaml" ]; then
        print_warning "工作目录中未找到配置文件 (config.yaml 或 config.docker.yaml)"
        print_info "将在启动时创建默认配置文件: $work_dir/config.docker.yaml"
        return 1
    fi
    
    return 0
}

# 检查配置文件（在工作目录中）
check_config() {
    local work_dir="$1"
    
    # 检查工作目录的配置文件 - 优先使用 config.docker.yaml
    if [ -f "$work_dir/config.docker.yaml" ]; then
        print_info "使用工作目录配置文件: $work_dir/config.docker.yaml"
        return 0
    elif [ -f "$work_dir/config.yaml" ]; then
        print_info "使用工作目录配置文件: $work_dir/config.yaml"
        return 0
    fi
    
    print_warning "未找到配置文件，将使用默认配置"
    print_info "请在启动后访问 http://localhost:8080/admin/ 配置端点"
}

# 检查环境变量文件（在工作目录中）
check_env_file() {
    local work_dir="$1"
    
    # 检查工作目录的环境文件
    if [ -f "$work_dir/$ENV_FILE" ]; then
        print_info "使用工作目录环境文件: $work_dir/$ENV_FILE"
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
    
    # 保存原始目录（用于镜像构建）
    ORIGINAL_DIR="$(pwd)"
    
    # 确定工作目录
    WORK_DIR="$(find_work_dir "$ORIGINAL_DIR")"
    
    # 如果工作目录不是当前目录，切换到工作目录
    if [ "$WORK_DIR" != "$ORIGINAL_DIR" ]; then
        print_info "切换到工作目录: $WORK_DIR"
        cd "$WORK_DIR"
    fi
    
    # 检查配置文件是否存在并显示警告
    check_config_files "$WORK_DIR"
    
    # 检查配置文件和环境文件
    check_config "$WORK_DIR"
    check_env_file "$WORK_DIR"
    
    # 创建日志目录（在工作目录）
    mkdir -p "$WORK_DIR/logs"
    
    # 确定使用的配置文件路径 - 优先使用 config.docker.yaml
    if [ -f "$WORK_DIR/config.docker.yaml" ]; then
        CONFIG_PATH="$WORK_DIR/config.docker.yaml"
    elif [ -f "$WORK_DIR/config.yaml" ]; then
        CONFIG_PATH="$WORK_DIR/config.yaml"
    else
        # 如果没有找到配置文件，创建一个默认配置在工作目录
        CONFIG_PATH="$WORK_DIR/config.docker.yaml"
        print_warning "配置文件不存在，将在工作目录创建默认配置: $CONFIG_PATH"
        echo "# Claude Code Companion 默认配置" > "$CONFIG_PATH"
        echo "# 请编辑此文件配置您的端点信息" >> "$CONFIG_PATH"
    fi
    
    # 确定使用的环境文件路径
    if [ -f "$WORK_DIR/$ENV_FILE" ]; then
        ENV_PATH="$WORK_DIR/$ENV_FILE"
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
            
            # 切换回原始目录构建镜像
            cd "$ORIGINAL_DIR"
            if [ ! -f "Dockerfile" ]; then
                print_error "当前目录未找到 Dockerfile，无法构建镜像"
                exit 1
            fi
            docker build -t claude-code-companion:latest .
            
            # 构建完成后切换回工作目录
            cd "$WORK_DIR"
        fi
        
        # 停止现有容器
        docker stop claude-code-companion 2>/dev/null || true
        docker rm claude-code-companion 2>/dev/null || true
        
        # 构建挂载参数
        DOCKER_RUN_ARGS=(
            "docker" "run" "-d" "--name" "claude-code-companion"
            "-p" "8080:8080"
            "-v" "$CONFIG_PATH:/app/config/config.yaml"
            "-v" "$WORK_DIR/logs:/app/logs"
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
    
    # 恢复原始目录
    if [ "$WORK_DIR" != "$ORIGINAL_DIR" ]; then
        cd "$ORIGINAL_DIR"
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
    
    # 保存原始目录
    ORIGINAL_DIR="$(pwd)"
    
    # 确定工作目录
    WORK_DIR="$(find_work_dir "$ORIGINAL_DIR")"
    
    # 如果工作目录不是当前目录，切换到工作目录
    if [ "$WORK_DIR" != "$ORIGINAL_DIR" ]; then
        print_info "切换到工作目录: $WORK_DIR"
        cd "$WORK_DIR"
    fi
    
    if command -v docker-compose &> /dev/null && [ -f "docker-compose.yml" ]; then
        docker-compose down
    else
        docker stop claude-code-companion 2>/dev/null || true
        docker rm claude-code-companion 2>/dev/null || true
    fi
    
    # 恢复原始目录
    if [ "$WORK_DIR" != "$ORIGINAL_DIR" ]; then
        cd "$ORIGINAL_DIR"
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
    # 保存原始目录
    ORIGINAL_DIR="$(pwd)"
    
    # 确定工作目录
    WORK_DIR="$(find_work_dir "$ORIGINAL_DIR")"
    
    # 如果工作目录不是当前目录，切换到工作目录
    if [ "$WORK_DIR" != "$ORIGINAL_DIR" ]; then
        print_info "切换到工作目录: $WORK_DIR"
        cd "$WORK_DIR"
    fi
    
    if command -v docker-compose &> /dev/null && [ -f "docker-compose.yml" ]; then
        docker-compose logs -f claude-code-companion
    else
        docker logs -f claude-code-companion 2>/dev/null || print_error "容器未运行"
    fi
    
    # 注意：日志查看是持续的，不需要恢复目录
}

# 构建镜像
build_image() {
    print_info "正在构建 Docker 镜像..."
    
    # 始终在当前目录构建镜像，不切换工作目录
    if [ ! -f "Dockerfile" ]; then
        print_error "当前目录未找到 Dockerfile，请在包含 Dockerfile 的目录中运行此命令"
        exit 1
    fi
    
    print_info "使用当前目录的 Dockerfile 构建镜像"
    docker build -t claude-code-companion:latest .
    
    print_success "镜像构建完成"
}

# 查看状态
show_status() {
    print_info "服务状态:"
    
    # 保存原始目录
    ORIGINAL_DIR="$(pwd)"
    
    # 确定工作目录
    WORK_DIR="$(find_work_dir "$ORIGINAL_DIR")"
    
    # 如果工作目录不是当前目录，切换到工作目录
    if [ "$WORK_DIR" != "$ORIGINAL_DIR" ]; then
        print_info "切换到工作目录: $WORK_DIR"
        cd "$WORK_DIR"
    fi
    
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
    
    # 恢复原始目录
    if [ "$WORK_DIR" != "$ORIGINAL_DIR" ]; then
        cd "$ORIGINAL_DIR"
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
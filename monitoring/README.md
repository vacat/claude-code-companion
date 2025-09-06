# Claude Code Companion 监控配置

本目录包含 Claude Code Companion 的 Prometheus 和 Grafana 监控配置。

## 🚀 快速启动

### 启动监控服务

```bash
# 启动包含监控的完整服务栈
docker-compose up -d

# 或者只启动监控服务
docker-compose up -d prometheus grafana
```

### 访问监控面板

- **Prometheus**: http://localhost:9090
  - 查看原始指标数据和查询
  - 验证数据采集是否正常

- **Grafana**: http://localhost:3000
  - 用户名: `admin`
  - 密码: `admin`
  - 预配置的 Claude Code Companion 监控面板

- **应用监控端点**:
  - 健康检查: http://localhost:8080/admin/health
  - Prometheus 指标: http://localhost:8080/admin/metrics

## 📊 监控指标说明

### 应用层指标

- `claude_proxy_endpoints_total`: 配置的端点总数
- `claude_proxy_endpoints_active`: 活跃且启用的端点数量
- `claude_proxy_endpoint_requests_total`: 各端点处理的请求总数
- `claude_proxy_endpoint_requests_success`: 各端点成功处理的请求数
- `claude_proxy_endpoint_status`: 端点状态 (1=在线, 0=离线)
- `claude_proxy_info`: 代理服务信息和版本

### 系统指标

可以通过添加 Node Exporter 或 cAdvisor 来监控系统资源：

```yaml
# 在 docker-compose.yml 中添加
node-exporter:
  image: prom/node-exporter:latest
  container_name: claude-node-exporter
  restart: unless-stopped
  ports:
    - "9100:9100"
  volumes:
    - /proc:/host/proc:ro
    - /sys:/host/sys:ro
    - /:/rootfs:ro
  command:
    - '--path.procfs=/host/proc'
    - '--path.rootfs=/rootfs'
    - '--path.sysfs=/host/sys'
    - '--collector.filesystem.mount-points-exclude=^/(sys|proc|dev|host|etc)($$|/)'
  networks:
    - claude-network
```

## 🎨 Grafana 仪表板

预配置的仪表板包含：

1. **端点状态概览**: 显示总端点数和活跃端点数
2. **端点请求总数**: 各端点的请求量时间序列图
3. **端点成功率**: 各端点的成功率趋势
4. **端点状态表**: 当前所有端点的状态表格

### 自定义仪表板

你可以在 Grafana 中创建自定义仪表板，或修改现有的仪表板配置文件：
`monitoring/provisioning/dashboards/claude-proxy-dashboard.json`

## 🔧 配置自定义

### Prometheus 配置

编辑 `monitoring/prometheus.yml` 来：
- 调整采集间隔
- 添加新的监控目标
- 配置告警规则

### Grafana 配置

编辑 `monitoring/grafana.ini` 来：
- 修改管理员密码
- 配置 LDAP 认证
- 调整安全设置

### 数据保留

默认配置：
- Prometheus 数据保留 15 天
- Grafana 数据存储在持久化卷中

修改数据保留策略：
```yaml
command:
  - '--storage.tsdb.retention.time=30d'  # 保留 30 天
```

## 🚨 告警配置

### Prometheus 告警规则

创建 `monitoring/alert-rules.yml`:

```yaml
groups:
  - name: claude-proxy-alerts
    rules:
      - alert: EndpointDown
        expr: claude_proxy_endpoint_status == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "端点 {{ $labels.endpoint }} 离线"
          description: "端点 {{ $labels.endpoint }} ({{ $labels.url }}) 已离线超过 5 分钟"

      - alert: HighErrorRate
        expr: rate(claude_proxy_endpoint_requests_success[5m]) / rate(claude_proxy_endpoint_requests_total[5m]) < 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "端点 {{ $labels.endpoint }} 错误率过高"
          description: "端点 {{ $labels.endpoint }} 在过去 5 分钟内错误率超过 10%"
```

然后在 `prometheus.yml` 中添加规则文件：
```yaml
rule_files:
  - "alert-rules.yml"
```

### 告警通知

配置 Alertmanager 来发送告警通知到邮件、Slack、微信等。

## 📈 性能优化

### Prometheus 优化

```yaml
# 在 docker-compose.yml 中调整 Prometheus 配置
command:
  - '--config.file=/etc/prometheus/prometheus.yml'
  - '--storage.tsdb.path=/prometheus'
  - '--storage.tsdb.retention.time=15d'
  - '--storage.tsdb.retention.size=10GB'  # 限制存储大小
  - '--web.enable-lifecycle'
  - '--web.enable-admin-api'
```

### Grafana 优化

```yaml
environment:
  - GF_DATABASE_TYPE=postgres          # 使用 PostgreSQL 替代 SQLite
  - GF_DATABASE_HOST=postgres:5432
  - GF_DATABASE_NAME=grafana
  - GF_DATABASE_USER=grafana
  - GF_DATABASE_PASSWORD=grafana
```

## 🛠️ 故障排除

### 常见问题

1. **Prometheus 无法采集数据**
   - 检查 `claude-code-companion:8080` 是否可访问
   - 确认 `/admin/metrics` 端点返回正确的指标数据

2. **Grafana 无法连接 Prometheus**
   - 检查数据源配置中的 URL: `http://prometheus:9090`
   - 确认容器网络连通性

3. **面板显示 "No data"**
   - 检查时间范围设置
   - 确认查询语句正确
   - 验证指标名称是否匹配

### 调试命令

```bash
# 检查容器状态
docker-compose ps

# 查看 Prometheus 日志
docker-compose logs prometheus

# 查看 Grafana 日志
docker-compose logs grafana

# 测试指标端点
curl http://localhost:8080/admin/metrics

# 测试健康检查端点
curl http://localhost:8080/admin/health
```

## 📚 参考资料

- [Prometheus 官方文档](https://prometheus.io/docs/)
- [Grafana 官方文档](https://grafana.com/docs/)
- [PromQL 查询语言](https://prometheus.io/docs/prometheus/latest/querying/)
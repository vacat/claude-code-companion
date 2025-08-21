# 调试信息导出格式文档

## 概述

调试信息导出功能允许用户导出特定请求ID的完整调试信息，包括原始和最终的请求/响应数据、元信息、端点配置和tagger配置。导出的数据以ZIP文件格式提供，便于用户进行错误报告和问题排查。

## 导出触发方式

在管理界面的日志详情页面，点击"导出调试信息"按钮即可触发导出。支持以下两种场景：
- 单次请求的详情页面
- 多次重试请求的详情页面

## ZIP文件结构

导出的ZIP文件命名格式：`debug_[REQUEST_ID]_[TIMESTAMP].zip`

### 目录结构

```
debug_[REQUEST_ID]_[TIMESTAMP].zip
├── README.txt                           # 文件说明（ASCII编码）
├── meta.json                            # 全局元信息文件（UTF-8编码，JSON格式）
├── attempts/                            # 尝试记录目录
│   ├── attempt_1/                       # 第一次尝试
│   │   ├── meta.json                    # 第一次尝试的详细元信息（UTF-8编码，JSON格式）
│   │   ├── original_request_headers.txt # 原始请求头（UTF-8编码）
│   │   ├── original_request_body.txt    # 原始请求体（UTF-8编码）
│   │   ├── final_request_headers.txt    # 最终请求头（UTF-8编码）
│   │   ├── final_request_body.txt       # 最终请求体（UTF-8编码）
│   │   ├── original_response_headers.txt # 原始响应头（UTF-8编码）
│   │   ├── original_response_body.txt   # 原始响应体（UTF-8编码）
│   │   ├── final_response_headers.txt   # 最终响应头（UTF-8编码）
│   │   └── final_response_body.txt      # 最终响应体（UTF-8编码）
│   ├── attempt_2/                       # 第二次尝试（如果存在）
│   │   ├── meta.json                    # 第二次尝试的详细元信息
│   │   └── ...                          # 同上结构
│   └── ...                              # 更多尝试（如果存在）
├── endpoints/                           # 端点配置目录
│   ├── endpoint_[NAME_1].json           # 端点配置文件（UTF-8编码，JSON格式）
│   ├── endpoint_[NAME_2].json           # 其他端点配置
│   └── ...
└── taggers/                             # Tagger配置目录
    ├── tagger_[NAME_1].json             # Tagger配置文件（UTF-8编码，JSON格式）
    ├── tagger_[NAME_2].json             # 其他Tagger配置
    └── ...
```

## 文件格式说明

### 1. README.txt
- **编码**: ASCII
- **格式**: 纯文本
- **内容**: 包含导出信息概述、文件结构说明和注意事项

### 2. meta.json (全局)
- **编码**: UTF-8
- **格式**: JSON
- **内容结构**:
```json
{
  "request_id": "string",              // 请求ID
  "export_timestamp": 1234567890,      // 导出时间戳（Unix时间）
  "total_attempts": 2,                 // 总尝试次数
  "first_request_time": 1234567890,    // 第一次请求时间戳
  "last_request_time": 1234567891,     // 最后一次请求时间戳
  "total_duration_ms": 3500,           // 所有尝试的总耗时
  "final_status_code": 200,            // 最终状态码
  "has_errors": false,                 // 是否有错误发生
  "unique_endpoints": ["endpoint1"]    // 涉及的端点列表
}
```

### 3. attempts/attempt_N/meta.json (尝试级别)
- **编码**: UTF-8  
- **格式**: JSON
- **内容结构**:
```json
{
  "attempt_number": 1,                 // 尝试编号
  "timestamp": 1234567890,             // 请求时间戳
  "endpoint": "string",                // 端点名称
  "method": "POST",                    // HTTP方法
  "path": "/v1/chat/completions",      // 请求路径
  "status_code": 200,                  // 状态码
  "duration_ms": 1500,                 // 耗时（毫秒）
  "model": "claude-3-sonnet",          // 模型名称
  "original_model": "gpt-4",           // 原始模型名（如果有重写）
  "rewritten_model": "claude-3-sonnet", // 重写后模型名
  "model_rewrite_applied": true,       // 是否应用了模型重写
  "thinking_enabled": false,           // 是否启用思考模式
  "thinking_budget_tokens": 0,         // 思考模式token预算
  "is_streaming": true,                // 是否流式响应
  "content_type_override": "text/event-stream", // Content-Type覆盖
  "request_body_size": 1024,           // 请求体大小（字节）
  "response_body_size": 2048,          // 响应体大小（字节）
  "tags": ["tag1", "tag2"],            // 标签数组
  "error": ""                          // 错误信息（如果有）
}
```

### 4. 尝试记录文件 (attempts/attempt_N/)

每个尝试目录包含9个文件：1个JSON元数据文件和8个纯文本文件，所有文件均为UTF-8编码：

#### 元数据文件：
- **meta.json**: 该次尝试的详细元信息（见上文格式说明）

#### 请求相关文件：
- **original_request_headers.txt**: 原始请求头，格式为 `Header-Name: Header-Value`，每行一个头
- **original_request_body.txt**: 原始请求体内容
- **final_request_headers.txt**: 最终请求头（经过处理后的），格式同原始请求头
- **final_request_body.txt**: 最终请求体内容（经过处理后的）

#### 响应相关文件：
- **original_response_headers.txt**: 原始响应头，格式为 `Header-Name: Header-Value`，每行一个头
- **original_response_body.txt**: 原始响应体内容
- **final_response_headers.txt**: 最终响应头（经过处理后的），格式同原始响应头
- **final_response_body.txt**: 最终响应体内容（经过处理后的）

**注意**: 如果某个字段为空，文件内容将显示 `(no headers)` 或为空。

### 5. 端点配置文件 (endpoints/endpoint_[NAME].json)

- **编码**: UTF-8
- **格式**: JSON
- **命名**: `endpoint_` + 清理后的端点名称 + `.json`
- **内容**: 端点的完整配置，**敏感信息已清理**：
  - `auth_value`: 替换为 `[REDACTED]`
  - `oauth_config.access_token`: 替换为 `[REDACTED]`
  - `oauth_config.refresh_token`: 替换为 `[REDACTED]`
  - `oauth_config.client_id`: 替换为 `[REDACTED]`
  - `proxy.username`: 替换为 `[REDACTED]`
  - `proxy.password`: 替换为 `[REDACTED]`

### 6. Tagger配置文件 (taggers/tagger_[NAME].json)

- **编码**: UTF-8
- **格式**: JSON
- **命名**: `tagger_` + 清理后的tagger名称 + `.json`
- **内容**: Tagger的完整配置信息

## 文件命名规则

为避免ZIP文件名乱码问题，所有文件名均遵循以下规则：
- 只包含ASCII字符（a-z、A-Z、0-9、下划线、短横线）
- 非ASCII字符替换为下划线 `_`
- 文件名长度限制在50个字符以内
- ZIP文件名格式：`debug_[SANITIZED_REQUEST_ID]_[TIMESTAMP].zip`

## API接口

### 导出调试信息
```
GET /admin/api/logs/{request_id}/export
```

**参数**:
- `request_id`: 要导出的请求ID

**响应**:
- **成功**: HTTP 200，返回ZIP文件流
- **请求ID不存在**: HTTP 404
- **服务器错误**: HTTP 500

**响应头**:
```
Content-Type: application/zip
Content-Disposition: attachment; filename="debug_[REQUEST_ID]_[TIMESTAMP].zip"
Content-Length: [FILE_SIZE]
```

## 使用场景

1. **错误报告**: 用户遇到请求失败时，可以导出完整的调试信息提交给技术支持
2. **问题排查**: 开发人员可以通过导出的数据分析请求处理流程
3. **配置验证**: 通过导出的端点和tagger配置验证当前设置是否正确
4. **性能分析**: 通过多次尝试的时间数据分析性能问题

## 安全考虑

1. **敏感信息清理**: 导出的配置文件中，所有认证信息、密码等敏感数据均已清理
2. **访问控制**: 导出功能需要管理员权限
3. **数据最小化**: 只导出与特定请求ID相关的数据，不会泄露其他请求的信息

## 自动化处理

此文档的结构化格式便于开发自动化的ZIP内容解析器，可以根据以下特点进行解析：

1. **固定的目录结构**: 所有导出文件都遵循相同的目录结构
2. **标准的文件命名**: 文件名具有可预测的模式
3. **统一的编码格式**: 所有文本文件使用UTF-8编码，JSON文件格式标准
4. **元信息文件**: `meta.json` 提供了完整的索引信息，便于快速解析

## 版本信息

- **文档版本**: 1.0
- **实现版本**: 与 Claude Code Companion 主版本同步
- **最后更新**: 2024年实现
# Anthropic Response Inspector 设计文档

## 概述

基于现有的 Request Inspector 架构，设计一个 Response Inspector 功能来分析和展示 Anthropic API 响应体内容。主要目标是帮助开发者理解响应结构、合并流式数据、分析工具调用结果、查看 thinking 内容等。

## 功能目标

### 核心功能
1. **响应解析**：解析 Anthropic API 响应（支持流式和非流式）
2. **内容统一展示**：将流式和非流式响应的内容以统一格式展示
3. **内容分析**：分析响应中的文本、工具调用、thinking 等内容
4. **Usage 统计增强**：展示详细的 token 使用量，包括 cache 相关统计
5. **错误分析**：识别和展示错误信息

### 高级功能
1. **工具调用分析**：展示工具输入/输出配对
2. **Thinking 模式**：解析和展示推理过程
3. **内容分类**：按类型组织不同的内容块
4. **Cache 效率分析**：分析 prompt caching 的使用效果

## 技术架构

### 1. 数据来源

#### 非流式响应
```json
{
  "id": "msg_123",
  "type": "message", 
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello world"
    },
    {
      "type": "tool_use",
      "id": "toolu_123",
      "name": "get_weather",
      "input": {"location": "San Francisco"}
    }
  ],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 100,
    "output_tokens": 50,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 80
  }
}
```

#### 流式响应 (SSE)
```
event: message_start
data: {"type": "message_start", "message": {"id": "msg_123", ...}}

event: content_block_start  
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}

event: content_block_stop
data: {"type": "content_block_stop", "index": 0}

event: message_delta
data: {"type": "message_delta", "delta": {"stop_reason": "end_turn"}, "usage": {"output_tokens": 50}}

event: message_stop
data: {"type": "message_stop"}
```

### 2. 数据结构设计

#### 解析后的响应结构
```javascript
{
  metadata: {
    id: "msg_123",
    model: "claude-sonnet-4-20250514", 
    role: "assistant",
    stop_reason: "end_turn",
    stop_sequence: null
  },
  usage: {
    input_tokens: 100,
    output_tokens: 50,
    cache_creation_input_tokens: 0,
    cache_read_input_tokens: 80,
    total_input_tokens: 180, // input_tokens + cache_read_input_tokens + cache_creation_input_tokens
    total_tokens: 230,       // total_input_tokens + output_tokens
    cache_efficiency: 44.4   // cache_read_input_tokens / total_input_tokens * 100
  },
  content: [
    {
      index: 0,
      type: "text",
      content: "Hello world",
      metadata: {
        characterCount: 11,
        wordCount: 2
      }
    },
    {
      index: 1,
      type: "tool_use", 
      content: {
        id: "toolu_123",
        name: "get_weather",
        input: {"location": "San Francisco"}
      },
      metadata: {
        inputSize: 28
      }
    },
    {
      index: 2,
      type: "thinking",
      content: "Let me think about this...",
      metadata: {
        characterCount: 25,
        isVisible: false
      }
    }
  ],
  errors: []
}
```

### 3. 组件架构

```
ResponseInspector/
├── ResponseParser.js          # 响应解析器
│   ├── parseNonStreaming()    # 解析普通响应
│   ├── parseStreaming()       # 解析流式响应并合并内容
│   ├── extractContent()       # 提取内容块
│   └── calculateUsage()       # 计算 usage 统计
├── ResponseUI.js              # UI 渲染器  
│   ├── renderOverview()       # 概览信息
│   ├── renderUsage()          # 详细 usage 统计
│   ├── renderContent()        # 内容块展示
│   └── renderErrors()         # 错误信息
└── ResponseInspectorMain.js   # 主控制器
```

## 功能设计

### 1. 概览面板
```
📊 响应概览
┌─────────────────────────────────────────────┐
│ 消息ID: msg_123                              │
│ 模型: claude-sonnet-4-20250514              │
│ 角色: assistant                             │
│ 停止原因: end_turn                          │
│ 内容块数: 3                                 │
└─────────────────────────────────────────────┘
```

### 2. 增强 Usage 统计面板
```
💰 Token 使用详情
┌─────────────────────────────────────────────┐
│ 基础输入 Token: 100                         │
│ 输出 Token: 50                              │
│ Cache 创建 Token: 0                         │
│ Cache 读取 Token: 80                        │
│ ────────────────────────────────────────     │
│ 总输入 Token: 180                           │
│ 总计 Token: 230                             │
│ ────────────────────────────────────────     │
│ Cache 效率: 44.4% 💡                       │
│ 输出/总计比: 21.7%                          │
│ 预估费用: $0.0069                          │
└─────────────────────────────────────────────┘
```

### 3. 内容块展示
```
💬 响应内容
┌─────────────────────────────────────────────┐
│ [1] 📝 文本内容 (11 字符, 2 词)               │
│     Hello world                             │
│                                             │
│ [2] 🔧 工具调用 - get_weather                │
│     输入: {"location": "San Francisco"}     │
│                                             │  
│ [3] 🧠 Thinking 内容 (25 字符, 隐藏)          │
│     Let me think about this...              │
└─────────────────────────────────────────────┘
```

### 4. Cache 效率分析
```
🎯 Cache 性能分析
┌─────────────────────────────────────────────┐
│ Cache 命中率: 44.4%                         │
│ 节省的费用: ~$0.0032 (约46%)                │
│ Cache 状态: 高效使用 ✅                     │
│                                             │
│ 💡 Cache 优化建议:                          │
│ • 当前 cache 使用良好                       │
│ • 可考虑增加更多静态内容到 cache             │
└─────────────────────────────────────────────┘
```

## 实现方案

### 1. 入口集成

#### 在 logs.html 中添加 Response Inspector 按钮
```html
<!-- 在响应详情工具栏添加 -->
<div class="btn-toolbar mb-2">
    <button id="inspectResponseBtn" class="btn btn-outline-success btn-sm ms-2"
            onclick="openResponseInspector()" title="检查 Anthropic 响应">
        🔍 检查响应
    </button>
</div>
```

#### 判断显示条件
```javascript
function showLogDetail(logId) {
    // ... 现有代码 ...
    
    // 检查是否为 Anthropic 响应
    const inspectBtn = document.getElementById('inspectResponseBtn');
    if (isAnthropicResponse(log.response_body)) {
        inspectBtn.style.display = 'inline-block';
        inspectBtn.setAttribute('data-response-body', log.response_body);
        inspectBtn.setAttribute('data-is-streaming', log.is_streaming);
        inspectBtn.setAttribute('data-final-response', log.final_response_body || '');
    } else {
        inspectBtn.style.display = 'none';
    }
}

function isAnthropicResponse(responseBody) {
    try {
        // 检查非流式响应
        const data = JSON.parse(responseBody);
        return data.type === 'message' && data.role === 'assistant';
    } catch {
        // 检查流式响应（SSE 格式）
        return responseBody.includes('event: message_start') && 
               responseBody.includes('data: {"type"');
    }
}
```

### 2. 响应解析器实现

#### ResponseParser.js
```javascript
class AnthropicResponseParser {
    constructor(responseBody, isStreaming = false, finalResponseBody = '') {
        this.rawResponse = responseBody;
        this.isStreaming = isStreaming;
        this.finalResponse = finalResponseBody;
        this.parsed = {
            metadata: {},
            usage: {},
            content: [],
            streamingInfo: null,
            errors: []
        };
        this.parse();
    }

    parse() {
        try {
            if (this.isStreaming) {
                this.parseStreaming();
            } else {
                this.parseNonStreaming();
            }
        } catch (error) {
            this.parsed.errors.push(`解析失败: ${error.message}`);
        }
    }

    parseNonStreaming() {
        const data = JSON.parse(this.rawResponse);
        
        // 解析元数据
        this.parsed.metadata = {
            id: data.id,
            model: data.model,
            role: data.role,
            stop_reason: data.stop_reason,
            stop_sequence: data.stop_sequence,
            isStreaming: false,
            completedAt: new Date().toISOString()
        };

        // 解析使用统计
        if (data.usage) {
            this.parsed.usage = {
                input_tokens: data.usage.input_tokens || 0,
                output_tokens: data.usage.output_tokens || 0,
                total_tokens: (data.usage.input_tokens || 0) + (data.usage.output_tokens || 0)
            };
        }

        // 解析内容块
        if (data.content && Array.isArray(data.content)) {
            this.parsed.content = data.content.map((block, index) => 
                this.parseContentBlock(block, index)
            );
        }
    }

    parseStreaming() {
        // 简化流式解析，专注于最终内容
        const events = this.parseSSEEvents();
        const mergedData = this.mergeStreamEvents(events);
        
        this.parsed.metadata = mergedData.metadata;
        this.parsed.usage = this.calculateUsage(mergedData.usage);
        this.parsed.content = mergedData.content;
    }

    parseSSEEvents() {
        const events = [];
        const lines = this.rawResponse.split('\n');
        let currentEvent = {};
        
        for (const line of lines) {
            if (line.startsWith('event: ')) {
                if (currentEvent.type) {
                    events.push({ ...currentEvent });
                }
                currentEvent = { type: line.substring(7) };
            } else if (line.startsWith('data: ')) {
                try {
                    currentEvent.data = JSON.parse(line.substring(6));
                } catch (e) {
                    currentEvent.data = line.substring(6);
                }
            }
        }
        
        if (currentEvent.type) {
            events.push(currentEvent);
        }
        
        return events;
    }

    mergeStreamEvents(events) {
        const result = { metadata: {}, usage: {}, content: [] };
        let contentBlocks = [];
        
        for (const event of events) {
            switch (event.type) {
                case 'message_start':
                    result.metadata = {
                        id: event.data.message.id,
                        model: event.data.message.model,
                        role: event.data.message.role
                    };
                    break;
                    
                case 'content_block_start':
                    contentBlocks[event.data.index] = {
                        type: event.data.content_block.type,
                        content: event.data.content_block.text || ''
                    };
                    break;
                    
                case 'content_block_delta':
                    if (contentBlocks[event.data.index]) {
                        if (event.data.delta.type === 'text_delta') {
                            contentBlocks[event.data.index].content += event.data.delta.text;
                        } else if (event.data.delta.type === 'input_json_delta') {
                            contentBlocks[event.data.index].content += event.data.delta.partial_json;
                        }
                    }
                    break;
                    
                case 'message_delta':
                    if (event.data.delta.stop_reason) {
                        result.metadata.stop_reason = event.data.delta.stop_reason;
                    }
                    if (event.data.usage) {
                        Object.assign(result.usage, event.data.usage);
                    }
                    break;
            }
        }

        result.content = contentBlocks.map((block, index) => 
            this.parseContentBlock(block, index)
        ).filter(Boolean);

        return result;
    }

    parseContentBlock(block, index) {
        if (!block) return null;
        
        const baseBlock = {
            index,
            type: block.type,
            metadata: {}
        };

        switch (block.type) {
            case 'text':
                return {
                    ...baseBlock,
                    content: block.text || block.content || '',
                    metadata: {
                        characterCount: (block.text || block.content || '').length,
                        wordCount: (block.text || block.content || '').split(/\s+/).length
                    }
                };
                
            case 'tool_use':
                return {
                    ...baseBlock,
                    content: {
                        id: block.id,
                        name: block.name,
                        input: block.input
                    },
                    metadata: {
                        inputSize: JSON.stringify(block.input || {}).length
                    }
                };
                
            case 'thinking':
                return {
                    ...baseBlock,
                    content: block.content || '',
                    metadata: {
                        characterCount: (block.content || '').length,
                        isVisible: false
                    }
                };
                
            default:
                return {
                    ...baseBlock,
                    content: block,
                    metadata: {}
                };
        }
    }

    calculateUsage(rawUsage) {
        const usage = {
            input_tokens: rawUsage.input_tokens || 0,
            output_tokens: rawUsage.output_tokens || 0,
            cache_creation_input_tokens: rawUsage.cache_creation_input_tokens || 0,
            cache_read_input_tokens: rawUsage.cache_read_input_tokens || 0
        };
        
        // 计算衍生数据
        usage.total_input_tokens = usage.input_tokens + usage.cache_creation_input_tokens + usage.cache_read_input_tokens;
        usage.total_tokens = usage.total_input_tokens + usage.output_tokens;
        
        // 计算 cache 效率
        if (usage.total_input_tokens > 0) {
            usage.cache_efficiency = ((usage.cache_read_input_tokens / usage.total_input_tokens) * 100).toFixed(1);
        } else {
            usage.cache_efficiency = 0;
        }
        
        // 计算输出比例
        if (usage.total_tokens > 0) {
            usage.output_ratio = ((usage.output_tokens / usage.total_tokens) * 100).toFixed(1);
        } else {
            usage.output_ratio = 0;
        }
        
        return usage;
    }
}
```

### 3. UI 渲染器实现

#### ResponseUI.js
```javascript
class ResponseInspectorUI {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
    }

    render(parser) {
        this.container.innerHTML = '';
        
        this.renderOverview(parser.parsed.metadata);
        this.renderUsage(parser.parsed.usage);
        this.renderContent(parser.parsed.content);
        
        if (parser.parsed.errors.length > 0) {
            this.renderErrors(parser.parsed.errors);
        }
    }

    renderOverview(metadata) {
        const overviewHtml = `
            <div class="response-inspector-section">
                <h6 class="response-inspector-title">📊 响应概览</h6>
                <div class="row g-3">
                    <div class="col-md-3">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">消息ID</div>
                            <div class="response-inspector-stat-value">${metadata.id || 'Unknown'}</div>
                        </div>
                    </div>
                    <div class="col-md-3">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">模型</div>
                            <div class="response-inspector-stat-value">${metadata.model || 'Unknown'}</div>
                        </div>
                    </div>
                    <div class="col-md-3">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">停止原因</div>
                            <div class="response-inspector-stat-value">${metadata.stop_reason || 'Unknown'}</div>
                        </div>
                    </div>
                    <div class="col-md-3">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">流式响应</div>
                            <div class="response-inspector-stat-value">${metadata.isStreaming ? '✅' : '❌'}</div>
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        this.container.appendChild(this.createElementFromHTML(overviewHtml));
    }

    renderUsage(usage) {
        if (!usage.total_tokens) return;
        
        const usageHtml = `
            <div class="response-inspector-section">
                <h6 class="response-inspector-title">💰 Token 使用详情</h6>
                <div class="row g-3">
                    <div class="col-md-2">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">基础输入</div>
                            <div class="response-inspector-stat-value">${usage.input_tokens}</div>
                        </div>
                    </div>
                    <div class="col-md-2">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">输出 Token</div>
                            <div class="response-inspector-stat-value">${usage.output_tokens}</div>
                        </div>
                    </div>
                    <div class="col-md-2">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">Cache 创建</div>
                            <div class="response-inspector-stat-value">${usage.cache_creation_input_tokens}</div>
                        </div>
                    </div>
                    <div class="col-md-2">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">Cache 读取</div>
                            <div class="response-inspector-stat-value">${usage.cache_read_input_tokens}</div>
                        </div>
                    </div>
                    <div class="col-md-2">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">总输入</div>
                            <div class="response-inspector-stat-value">${usage.total_input_tokens}</div>
                        </div>
                    </div>
                    <div class="col-md-2">
                        <div class="response-inspector-stat">
                            <div class="response-inspector-stat-label">总计</div>
                            <div class="response-inspector-stat-value">${usage.total_tokens}</div>
                        </div>
                    </div>
                </div>
                ${this.renderCacheAnalysis(usage)}
            </div>
        `;
        
        this.container.appendChild(this.createElementFromHTML(usageHtml));
    }

    renderCacheAnalysis(usage) {
        if (usage.cache_read_input_tokens === 0 && usage.cache_creation_input_tokens === 0) {
            return '';
        }

        const cacheStatus = usage.cache_efficiency > 30 ? '高效使用 ✅' : 
                           usage.cache_efficiency > 10 ? '中等使用 ⚠️' : '低效使用 ⚠️';
        
        return `
            <div class="mt-3 p-3 bg-light border rounded">
                <h6>🎯 Cache 性能分析</h6>
                <div class="row g-3">
                    <div class="col-md-4">
                        <strong>Cache 效率:</strong> ${usage.cache_efficiency}%
                    </div>
                    <div class="col-md-4">
                        <strong>输出比例:</strong> ${usage.output_ratio}%
                    </div>
                    <div class="col-md-4">
                        <strong>Cache 状态:</strong> ${cacheStatus}
                    </div>
                </div>
            </div>
        `;
    }

    renderContent(content) {
        let contentHtml = `
            <div class="response-inspector-section">
                <h6 class="response-inspector-title">💬 响应内容</h6>
        `;

        content.forEach(block => {
            contentHtml += this.renderContentBlock(block);
        });

        contentHtml += '</div>';
        this.container.appendChild(this.createElementFromHTML(contentHtml));
    }

    renderContentBlock(block) {
        const typeIcon = this.getContentTypeIcon(block.type);
        const blockId = `response-content-${block.index}`;
        
        let contentPreview = '';
        let contentDetails = '';
        
        switch (block.type) {
            case 'text':
                contentPreview = `${block.metadata.characterCount} 字符, ${block.metadata.wordCount} 词`;
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <pre class="response-inspector-text">${this.escapeHtml(block.content)}</pre>
                    </div>
                `;
                break;
                
            case 'tool_use':
                contentPreview = `${block.content.name} - ${block.metadata.inputSize} 字符输入`;
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <strong>工具名称:</strong> ${block.content.name}<br>
                        <strong>工具ID:</strong> ${block.content.id}<br>
                        <strong>输入参数:</strong>
                        <pre class="response-inspector-json">${JSON.stringify(block.content.input, null, 2)}</pre>
                    </div>
                `;
                break;
                
            case 'thinking':
                contentPreview = `${block.metadata.characterCount} 字符推理内容`;
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <div class="alert alert-info">
                            <strong>🧠 Thinking 模式内容</strong><br>
                            此内容为模型的内部推理过程，通常对用户不可见。
                        </div>
                        <pre class="response-inspector-text">${this.escapeHtml(block.content)}</pre>
                    </div>
                `;
                break;
                
            default:
                contentPreview = '未知内容类型';
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <pre class="response-inspector-json">${JSON.stringify(block.content, null, 2)}</pre>
                    </div>
                `;
        }
        
        const streamInfo = block.metadata.streamEvents > 0 ? 
            ` (${block.metadata.streamEvents} 个流式事件)` : '';
        
        return `
            <div class="response-inspector-content-item">
                <div class="response-inspector-collapse-header" onclick="toggleResponseCollapse('${blockId}')">
                    <span class="response-inspector-collapse-icon" id="${blockId}-icon">▶</span>
                    [${block.index + 1}] ${typeIcon} ${block.type} - ${contentPreview}
                </div>
                <div class="response-inspector-collapse-content" id="${blockId}" style="display: none;">
                    ${contentDetails}
                </div>
            </div>
        `;
    }

    getContentTypeIcon(type) {
        const icons = {
            'text': '📝',
            'tool_use': '🔧',
            'thinking': '🧠',
            'tool_result': '📤'
        };
        return icons[type] || '❓';
    }

    renderErrors(errors) {
        const errorsHtml = `
            <div class="response-inspector-section response-inspector-errors">
                <h6 class="response-inspector-title text-danger">⚠️ 解析错误</h6>
                ${errors.map(error => `<div class="alert alert-danger">${this.escapeHtml(error)}</div>`).join('')}
            </div>
        `;
        this.container.appendChild(this.createElementFromHTML(errorsHtml));
    }

    createElementFromHTML(htmlString) {
        const div = document.createElement('div');
        div.innerHTML = htmlString.trim();
        return div.firstChild;
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}
```

### 4. 主控制器和样式

#### ResponseInspectorMain.js
```javascript
let currentResponseParser = null;
let currentResponseUI = null;

function openResponseInspector() {
    const responseBtn = document.getElementById('inspectResponseBtn');
    const responseBody = responseBtn.getAttribute('data-response-body');
    const isStreaming = responseBtn.getAttribute('data-is-streaming') === 'true';
    const finalResponse = responseBtn.getAttribute('data-final-response');

    if (!responseBody) {
        alert('未找到响应数据');
        return;
    }

    try {
        currentResponseParser = new AnthropicResponseParser(responseBody, isStreaming, finalResponse);
        currentResponseUI = new ResponseInspectorUI('responseInspectorContent');
        
        currentResponseUI.render(currentResponseParser);
        
        const modalElement = document.getElementById('responseInspectorModal');
        if (modalElement) {
            const modal = new bootstrap.Modal(modalElement);
            modal.show();
        }
    } catch (error) {
        console.error('Failed to open response inspector:', error);
        alert('打开响应检查器时出错: ' + error.message);
    }
}

function toggleResponseCollapse(elementId) {
    const element = document.getElementById(elementId);
    const icon = document.getElementById(elementId + '-icon');
    
    if (!element) return;
    
    if (element.style.display === 'none') {
        element.style.display = 'block';
        if (icon) icon.textContent = '▼';
    } else {
        element.style.display = 'none';
        if (icon) icon.textContent = '▶';
    }
}

function exportResponseAnalysis() {
    if (!currentResponseParser) return;
    
    const analysis = {
        metadata: currentResponseParser.parsed.metadata,
        usage: currentResponseParser.parsed.usage,
        contentSummary: currentResponseParser.parsed.content.map(block => ({
            type: block.type,
            size: block.type === 'text' ? block.metadata.characterCount : JSON.stringify(block.content).length
        })),
        streamingInfo: currentResponseParser.parsed.streamingInfo,
        exportTime: new Date().toISOString()
    };
    
    const dataStr = JSON.stringify(analysis, null, 2);
    const dataBlob = new Blob([dataStr], {type: 'application/json'});
    const url = URL.createObjectURL(dataBlob);
    
    const link = document.createElement('a');
    link.href = url;
    link.download = `anthropic-response-analysis-${Date.now()}.json`;
    link.click();
    
    URL.revokeObjectURL(url);
}
```

#### response-inspector.css
```css
/* Response Inspector 样式 */
.response-inspector-section {
    margin-bottom: 2rem;
    border: 1px solid #e0e0e0;
    border-radius: 8px;
    padding: 1rem;
}

.response-inspector-title {
    color: #333;
    font-weight: 600;
    margin-bottom: 1rem;
    border-bottom: 2px solid #f0f0f0;
    padding-bottom: 0.5rem;
}

.response-inspector-stat {
    text-align: center;
    padding: 0.75rem;
    background: #f8f9fa;
    border-radius: 6px;
    border: 1px solid #e9ecef;
}

.response-inspector-stat-label {
    font-size: 0.875rem;
    color: #6c757d;
    margin-bottom: 0.25rem;
}

.response-inspector-stat-value {
    font-size: 1.1rem;
    font-weight: 600;
    color: #495057;
}

.response-inspector-content-item {
    margin-bottom: 1rem;
}

.response-inspector-collapse-header {
    cursor: pointer;
    padding: 0.5rem;
    background: #f8f9fa;
    border: 1px solid #dee2e6;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    transition: background-color 0.2s;
    font-weight: 500;
}

.response-inspector-collapse-header:hover {
    background: #e9ecef;
}

.response-inspector-collapse-icon {
    display: inline-block;
    width: 1rem;
    text-align: center;
    margin-right: 0.5rem;
    transition: transform 0.2s;
}

.response-inspector-collapse-content {
    margin-left: 1rem;
    padding: 0.5rem;
    border-left: 3px solid #dee2e6;
}

.response-inspector-content-box {
    background: #f8f9fa;
    border: 1px solid #e9ecef;
    border-radius: 4px;
    padding: 0.75rem;
    margin: 0.5rem 0;
}

.response-inspector-text, .response-inspector-json {
    background: transparent;
    border: none;
    padding: 0;
    margin: 0;
    font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
    font-size: 0.875rem;
    line-height: 1.4;
    white-space: pre-wrap;
    word-wrap: break-word;
}

.response-inspector-errors {
    border-color: #dc3545;
    background: #f8d7da;
}

@media (max-width: 768px) {
    .response-inspector-stat {
        margin-bottom: 0.5rem;
    }
    
    .response-inspector-collapse-content {
        margin-left: 0.5rem;
    }
}
```

## 集成和测试

### 1. 文件结构
```
web/
├── static/
│   ├── css/
│   │   └── response-inspector.css
│   └── js/
│       ├── response-inspector-main.js
│       ├── response-inspector-parser.js
│       └── response-inspector-ui.js
└── templates/
    └── logs.html (修改)
```

### 2. 集成步骤
1. 在 `logs.html` 中添加响应检查器按钮和模态框
2. 引入 CSS 和 JavaScript 文件
3. 修改 `showLogDetail` 函数添加按钮显示逻辑
4. 测试各种响应格式的解析效果

### 3. 测试用例
- 简单文本响应
- 包含工具调用的响应
- 流式响应解析与内容合并
- Thinking 模式响应
- 带 Cache 的响应分析
- 错误响应处理

这个简化的设计专注于内容展示和 usage 分析，移除了复杂的流式时间分析，更适合实际使用场景。
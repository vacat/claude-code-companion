# Anthropic 请求检查器实现文档

## 实现概览

基于设计文档，本实现文档详细说明如何在现有的 Web 管理界面中添加 Anthropic 请求检查器功能。

## 技术架构

### 前端技术栈
- **基础**: HTML5 + JavaScript (ES6+) + Bootstrap 5
- **JSON 处理**: 原生 JSON.parse/stringify
- **UI 组件**: 自定义折叠组件 + Modal 弹窗
- **语法高亮**: highlight.js (可选)
- **图标系统**: Bootstrap Icons + Emoji

### 后端支持
- **无需后端修改**: 纯前端解析和展示
- **数据来源**: 现有日志详情中的请求体数据

## 文件结构

```
web/
├── static/
│   ├── css/
│   │   └── inspector.css          # 检查器专用样式
│   └── js/
│       ├── inspector.js           # 主要逻辑
│       ├── inspector-parser.js    # 解析器
│       └── inspector-ui.js        # UI 组件
└── templates/
    └── inspector-modal.html       # 模态框模板 (嵌入到现有页面)
```

## 核心实现

### 1. 入口集成 (logs.html 修改)

#### 在请求详情工具栏添加按钮
```html
<!-- 在现有编辑框工具栏中添加 -->
<div class="btn-toolbar mb-2">
    <!-- 现有按钮... -->
    <button id="inspectRequestBtn" class="btn btn-outline-primary btn-sm ms-2" 
            onclick="openRequestInspector()" title="检查 Anthropic 请求">
        🔍 检查请求
    </button>
</div>
```

#### 判断显示条件
```javascript
// 在 showLogDetail 函数中添加
function showLogDetail(logId) {
    // ... 现有代码 ...
    
    // 检查是否为 Anthropic 请求
    const inspectBtn = document.getElementById('inspectRequestBtn');
    if (isAnthropicRequest(log.request_body)) {
        inspectBtn.style.display = 'inline-block';
        inspectBtn.setAttribute('data-request-body', log.request_body);
    } else {
        inspectBtn.style.display = 'none';
    }
}

function isAnthropicRequest(requestBody) {
    try {
        const data = JSON.parse(requestBody);
        return data.model && data.messages && Array.isArray(data.messages);
    } catch {
        return false;
    }
}
```

### 2. 模态框 HTML 结构

```html
<!-- Anthropic 请求检查器模态框 -->
<div class="modal fade" id="requestInspectorModal" tabindex="-1" 
     aria-labelledby="requestInspectorModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-xl modal-dialog-scrollable">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title" id="requestInspectorModalLabel">
                    🔍 Anthropic 请求检查器
                </h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div class="modal-body" id="inspectorContent">
                <!-- 动态内容 -->
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-outline-secondary" onclick="exportAnalysis()">
                    📄 导出分析
                </button>
                <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">
                    关闭
                </button>
            </div>
        </div>
    </div>
</div>
```

### 3. 解析器实现 (inspector-parser.js)

```javascript
class AnthropicRequestParser {
    constructor(requestBody) {
        this.raw = requestBody;
        this.data = null;
        this.parsed = {
            overview: {},
            system: {},
            messages: [],
            tools: [],
            errors: []
        };
        this.parse();
    }

    parse() {
        try {
            this.data = JSON.parse(this.raw);
            this.parseOverview();
            this.parseSystem();
            this.parseMessages();
            this.parseTools();
        } catch (error) {
            this.errors.push(`JSON 解析失败: ${error.message}`);
        }
    }

    parseOverview() {
        this.parsed.overview = {
            model: this.data.model || 'Unknown',
            maxTokens: this.data.max_tokens || 'Not set',
            messageCount: this.data.messages ? this.data.messages.length : 0,
            toolCount: this.data.tools ? this.data.tools.length : 0,
            hasSystem: !!this.data.system,
            estimatedTokens: this.estimateTokens()
        };
    }

    parseSystem() {
        if (this.data.system) {
            this.parsed.system = {
                content: this.data.system,
                characterCount: this.data.system.length,
                wordCount: this.data.system.split(/\s+/).length
            };
        }
    }

    parseMessages() {
        if (!this.data.messages) return;

        this.data.messages.forEach((message, index) => {
            const parsedMessage = {
                index: index + 1,
                role: message.role,
                content: [],
                toolUses: [],
                systemReminders: []
            };

            if (Array.isArray(message.content)) {
                message.content.forEach(content => {
                    if (content.type === 'text') {
                        // 检查是否包含 system reminder
                        if (content.text.includes('<system-reminder>')) {
                            parsedMessage.systemReminders.push(...this.extractSystemReminders(content.text));
                            parsedMessage.content.push({
                                type: 'text',
                                text: this.removeSystemReminders(content.text),
                                preview: this.createPreview(this.removeSystemReminders(content.text))
                            });
                        } else {
                            parsedMessage.content.push({
                                type: 'text',
                                text: content.text,
                                preview: this.createPreview(content.text)
                            });
                        }
                    } else if (content.type === 'tool_use') {
                        parsedMessage.toolUses.push({
                            id: content.id,
                            name: content.name,
                            input: content.input,
                            type: 'use'
                        });
                    } else if (content.type === 'tool_result') {
                        // 查找对应的 tool_use
                        const toolUse = this.findToolUse(content.tool_use_id);
                        parsedMessage.toolUses.push({
                            id: content.tool_use_id,
                            name: toolUse ? toolUse.name : 'Unknown',
                            input: toolUse ? toolUse.input : null,
                            result: content.content,
                            isError: content.is_error || false,
                            type: 'result'
                        });
                    }
                });
            } else if (typeof message.content === 'string') {
                parsedMessage.content.push({
                    type: 'text',
                    text: message.content,
                    preview: this.createPreview(message.content)
                });
            }

            this.parsed.messages.push(parsedMessage);
        });

        // 配对 tool uses 和 results
        this.pairToolCalls();
    }

    parseTools() {
        if (!this.data.tools) return;

        this.data.tools.forEach(tool => {
            const parsedTool = {
                name: tool.name,
                description: tool.description || '',
                parameters: [],
                schema: tool.input_schema || {}
            };

            if (tool.input_schema && tool.input_schema.properties) {
                Object.entries(tool.input_schema.properties).forEach(([name, prop]) => {
                    parsedTool.parameters.push({
                        name: name,
                        type: prop.type || 'unknown',
                        description: prop.description || '',
                        required: tool.input_schema.required && tool.input_schema.required.includes(name),
                        enum: prop.enum || null
                    });
                });
            }

            this.parsed.tools.push(parsedTool);
        });
    }

    extractSystemReminders(text) {
        const reminders = [];
        const regex = /<system-reminder>([\s\S]*?)<\/system-reminder>/g;
        let match;
        
        while ((match = regex.exec(text)) !== null) {
            reminders.push({
                content: match[1].trim(),
                preview: this.createPreview(match[1].trim()),
                type: this.detectReminderType(match[1])
            });
        }
        
        return reminders;
    }

    removeSystemReminders(text) {
        return text.replace(/<system-reminder>[\s\S]*?<\/system-reminder>/g, '').trim();
    }

    detectReminderType(content) {
        if (content.includes('context')) return 'context';
        if (content.includes('tool')) return 'tool';
        if (content.includes('reminder')) return 'reminder';
        return 'general';
    }

    createPreview(text, maxLength = 100) {
        if (text.length <= maxLength) return text;
        return text.substring(0, maxLength) + '...';
    }

    pairToolCalls() {
        const toolPairs = new Map();
        
        this.parsed.messages.forEach(message => {
            message.toolUses.forEach(tool => {
                if (tool.type === 'use') {
                    toolPairs.set(tool.id, { use: tool, result: null });
                } else if (tool.type === 'result') {
                    if (toolPairs.has(tool.id)) {
                        toolPairs.get(tool.id).result = tool;
                    } else {
                        toolPairs.set(tool.id, { use: null, result: tool });
                    }
                }
            });
        });

        // 重新组织工具调用
        this.parsed.messages.forEach(message => {
            message.pairedToolCalls = [];
            message.toolUses.forEach(tool => {
                if (tool.type === 'use' && toolPairs.has(tool.id)) {
                    const pair = toolPairs.get(tool.id);
                    message.pairedToolCalls.push({
                        id: tool.id,
                        name: tool.name,
                        input: tool.input,
                        result: pair.result ? pair.result.result : null,
                        isError: pair.result ? pair.result.isError : false,
                        status: pair.result ? (pair.result.isError ? 'error' : 'success') : 'pending',
                        isThinking: this.isThinkingResult(pair.result)
                    });
                }
            });
        });
    }

    isThinkingResult(result) {
        if (!result || !result.result) return false;
        // 检查是否为 thinking 结果
        return typeof result.result === 'string' && result.result.includes('<thinking>');
    }

    findToolUse(id) {
        for (const message of this.parsed.messages) {
            const toolUse = message.toolUses.find(tool => tool.id === id && tool.type === 'use');
            if (toolUse) return toolUse;
        }
        return null;
    }

    estimateTokens() {
        // 简单的 token 估算
        const text = JSON.stringify(this.data);
        return Math.ceil(text.length / 4); // 粗略估算
    }
}
```

### 4. UI 渲染器实现 (inspector-ui.js)

```javascript
class InspectorUI {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        this.collapseStates = new Map();
    }

    render(parser) {
        this.container.innerHTML = '';
        
        // 渲染概览
        this.renderOverview(parser.parsed.overview);
        
        // 渲染系统配置
        this.renderSystem(parser.parsed.system, parser.parsed.tools);
        
        // 渲染消息
        this.renderMessages(parser.parsed.messages);
        
        // 如果有错误，显示错误信息
        if (parser.errors.length > 0) {
            this.renderErrors(parser.errors);
        }
    }

    renderOverview(overview) {
        const overviewHtml = `
            <div class="inspector-section">
                <h6 class="inspector-title">📊 请求概览</h6>
                <div class="row g-3">
                    <div class="col-md-3">
                        <div class="inspector-stat">
                            <div class="inspector-stat-label">模型</div>
                            <div class="inspector-stat-value">${overview.model}</div>
                        </div>
                    </div>
                    <div class="col-md-3">
                        <div class="inspector-stat">
                            <div class="inspector-stat-label">最大令牌</div>
                            <div class="inspector-stat-value">${overview.maxTokens}</div>
                        </div>
                    </div>
                    <div class="col-md-3">
                        <div class="inspector-stat">
                            <div class="inspector-stat-label">消息数</div>
                            <div class="inspector-stat-value">${overview.messageCount}</div>
                        </div>
                    </div>
                    <div class="col-md-3">
                        <div class="inspector-stat">
                            <div class="inspector-stat-label">工具数</div>
                            <div class="inspector-stat-value">${overview.toolCount}</div>
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        this.container.appendChild(this.createElementFromHTML(overviewHtml));
    }

    renderSystem(system, tools) {
        let systemHtml = `
            <div class="inspector-section">
                <h6 class="inspector-title">🔧 系统配置</h6>
        `;

        // System Prompt
        if (system.content) {
            const systemId = 'system-prompt';
            systemHtml += `
                <div class="inspector-subsection">
                    <div class="inspector-collapse-header" onclick="toggleCollapse('${systemId}')">
                        <span class="inspector-collapse-icon" id="${systemId}-icon">▶</span>
                        📝 System Prompt (${system.characterCount} 字符)
                    </div>
                    <div class="inspector-collapse-content" id="${systemId}" style="display: none;">
                        <div class="inspector-content-box">
                            <pre class="inspector-code">${this.escapeHtml(system.content)}</pre>
                        </div>
                    </div>
                </div>
            `;
        }

        // Tools
        if (tools.length > 0) {
            const toolsId = 'available-tools';
            systemHtml += `
                <div class="inspector-subsection">
                    <div class="inspector-collapse-header" onclick="toggleCollapse('${toolsId}')">
                        <span class="inspector-collapse-icon" id="${toolsId}-icon">▶</span>
                        🛠️ 可用工具 (${tools.length}个)
                    </div>
                    <div class="inspector-collapse-content" id="${toolsId}" style="display: none;">
                        ${this.renderToolsList(tools)}
                    </div>
                </div>
            `;
        }

        systemHtml += '</div>';
        this.container.appendChild(this.createElementFromHTML(systemHtml));
    }

    renderToolsList(tools) {
        return tools.map(tool => {
            const toolId = `tool-${tool.name}`;
            return `
                <div class="inspector-tool-item">
                    <div class="inspector-collapse-header inspector-tool-header" onclick="toggleCollapse('${toolId}')">
                        <span class="inspector-collapse-icon" id="${toolId}-icon">▶</span>
                        🔧 ${tool.name} - ${tool.description || '无描述'}
                    </div>
                    <div class="inspector-collapse-content" id="${toolId}" style="display: none;">
                        ${this.renderToolDetails(tool)}
                    </div>
                </div>
            `;
        }).join('');
    }

    renderToolDetails(tool) {
        let detailsHtml = '<div class="inspector-tool-details">';
        
        if (tool.parameters.length > 0) {
            detailsHtml += `
                <div class="inspector-param-section">
                    <strong>📋 参数列表:</strong>
                    <ul class="inspector-param-list">
            `;
            
            tool.parameters.forEach(param => {
                const requiredBadge = param.required ? '<span class="badge bg-danger">必需</span>' : '<span class="badge bg-secondary">可选</span>';
                detailsHtml += `
                    <li class="inspector-param-item">
                        <code>${param.name}</code> 
                        <span class="inspector-param-type">(${param.type})</span>
                        ${requiredBadge}
                        ${param.description ? `<div class="inspector-param-desc">${param.description}</div>` : ''}
                    </li>
                `;
            });
            
            detailsHtml += '</ul></div>';
        }

        if (tool.description) {
            detailsHtml += `
                <div class="inspector-desc-section">
                    <strong>📖 完整描述:</strong>
                    <div class="inspector-content-box">
                        ${this.escapeHtml(tool.description)}
                    </div>
                </div>
            `;
        }

        detailsHtml += '</div>';
        return detailsHtml;
    }

    renderMessages(messages) {
        let messagesHtml = `
            <div class="inspector-section">
                <h6 class="inspector-title">💬 对话消息</h6>
        `;

        messages.forEach(message => {
            messagesHtml += this.renderMessage(message);
        });

        messagesHtml += '</div>';
        this.container.appendChild(this.createElementFromHTML(messagesHtml));
    }

    renderMessage(message) {
        const roleIcon = message.role === 'user' ? '👤' : '🤖';
        const roleClass = `inspector-message-${message.role}`;
        
        let messageHtml = `
            <div class="inspector-message ${roleClass}">
                <div class="inspector-message-header">
                    [${message.index}] ${roleIcon} ${message.role.charAt(0).toUpperCase() + message.role.slice(1)}
                </div>
                <div class="inspector-message-content">
        `;

        // 渲染文本内容
        message.content.forEach((content, idx) => {
            if (content.type === 'text') {
                const contentId = `message-${message.index}-content-${idx}`;
                messageHtml += `
                    <div class="inspector-content-item">
                        <div class="inspector-collapse-header" onclick="toggleCollapse('${contentId}')">
                            <span class="inspector-collapse-icon" id="${contentId}-icon">▶</span>
                            💭 正文内容 (${content.text.length} 字符)
                        </div>
                        <div class="inspector-collapse-content" id="${contentId}" style="display: none;">
                            <div class="inspector-content-box">
                                <pre class="inspector-text">${this.escapeHtml(content.text)}</pre>
                            </div>
                        </div>
                        <div class="inspector-preview">${this.escapeHtml(content.preview)}</div>
                    </div>
                `;
            }
        });

        // 渲染 System Reminders
        if (message.systemReminders.length > 0) {
            const remindersId = `message-${message.index}-reminders`;
            messageHtml += `
                <div class="inspector-content-item">
                    <div class="inspector-collapse-header" onclick="toggleCollapse('${remindersId}')">
                        <span class="inspector-collapse-icon" id="${remindersId}-icon">▶</span>
                        ⚠️ System Reminders (${message.systemReminders.length}个)
                    </div>
                    <div class="inspector-collapse-content" id="${remindersId}" style="display: none;">
                        ${this.renderSystemReminders(message.systemReminders)}
                    </div>
                </div>
            `;
        }

        // 渲染工具调用
        if (message.pairedToolCalls && message.pairedToolCalls.length > 0) {
            const toolCallsId = `message-${message.index}-tools`;
            messageHtml += `
                <div class="inspector-content-item">
                    <div class="inspector-collapse-header" onclick="toggleCollapse('${toolCallsId}')">
                        <span class="inspector-collapse-icon" id="${toolCallsId}-icon">▶</span>
                        🔧 工具调用 (${message.pairedToolCalls.length}次)
                    </div>
                    <div class="inspector-collapse-content" id="${toolCallsId}" style="display: none;">
                        ${this.renderToolCalls(message.pairedToolCalls, message.index)}
                    </div>
                </div>
            `;
        }

        messageHtml += '</div></div>';
        return messageHtml;
    }

    renderSystemReminders(reminders) {
        return reminders.map((reminder, idx) => {
            const reminderId = `reminder-${idx}`;
            const typeIcon = this.getReminderIcon(reminder.type);
            
            return `
                <div class="inspector-reminder-item">
                    <div class="inspector-collapse-header" onclick="toggleCollapse('${reminderId}')">
                        <span class="inspector-collapse-icon" id="${reminderId}-icon">▶</span>
                        ${typeIcon} ${reminder.type}: ${reminder.preview}
                    </div>
                    <div class="inspector-collapse-content" id="${reminderId}" style="display: none;">
                        <div class="inspector-content-box">
                            <pre class="inspector-text">${this.escapeHtml(reminder.content)}</pre>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
    }

    renderToolCalls(toolCalls, messageIndex) {
        return toolCalls.map((call, idx) => {
            const callId = `toolcall-${messageIndex}-${idx}`;
            const statusIcon = this.getToolStatusIcon(call.status, call.isThinking);
            const thinkingLabel = call.isThinking ? ' (Thinking)' : '';
            
            return `
                <div class="inspector-tool-call">
                    <div class="inspector-tool-call-header">
                        <span class="inspector-tool-status">${statusIcon}</span>
                        🔧 ${call.name}${thinkingLabel}
                        <button class="btn btn-sm btn-outline-secondary ms-2" onclick="toggleCollapse('${callId}')">
                            详情
                        </button>
                    </div>
                    <div class="inspector-collapse-content" id="${callId}" style="display: none;">
                        ${this.renderToolCallDetails(call)}
                    </div>
                </div>
            `;
        }).join('');
    }

    renderToolCallDetails(call) {
        let detailsHtml = '<div class="inspector-tool-call-details">';
        
        // 调用参数
        detailsHtml += `
            <div class="inspector-call-section">
                <strong>📤 调用参数:</strong>
                <div class="inspector-content-box">
                    <pre class="inspector-json">${JSON.stringify(call.input, null, 2)}</pre>
                </div>
            </div>
        `;

        // 返回结果
        if (call.result !== null) {
            const resultPreview = typeof call.result === 'string' ? 
                (call.result.length > 200 ? call.result.substring(0, 200) + '...' : call.result) :
                JSON.stringify(call.result);
                
            detailsHtml += `
                <div class="inspector-call-section">
                    <strong>📥 返回结果:</strong>
                    <div class="inspector-result-status">
                        状态: ${call.status === 'success' ? '✅ 成功' : '❌ 失败'}
                        ${typeof call.result === 'string' ? `(${call.result.length} 字符)` : ''}
                    </div>
                    <div class="inspector-content-box">
                        <pre class="inspector-text">${this.escapeHtml(resultPreview)}</pre>
                    </div>
                </div>
            `;
        } else {
            detailsHtml += `
                <div class="inspector-call-section">
                    <strong>📥 返回结果:</strong>
                    <div class="inspector-result-status text-muted">⏳ 等待结果...</div>
                </div>
            `;
        }

        detailsHtml += '</div>';
        return detailsHtml;
    }

    getReminderIcon(type) {
        const icons = {
            'context': '🔄',
            'tool': '⚡',
            'reminder': '📌',
            'general': '💡'
        };
        return icons[type] || '💡';
    }

    getToolStatusIcon(status, isThinking) {
        if (isThinking) return '🧠';
        const icons = {
            'success': '✅',
            'error': '❌',
            'pending': '⏳'
        };
        return icons[status] || '❓';
    }

    renderErrors(errors) {
        const errorsHtml = `
            <div class="inspector-section inspector-errors">
                <h6 class="inspector-title text-danger">⚠️ 解析错误</h6>
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

### 5. 主控制器实现 (inspector.js)

```javascript
// 全局变量
let currentParser = null;
let currentUI = null;

// 入口函数
function openRequestInspector() {
    const requestBody = document.getElementById('inspectRequestBtn').getAttribute('data-request-body');
    
    if (!requestBody) {
        alert('未找到请求数据');
        return;
    }

    // 创建解析器和UI
    currentParser = new AnthropicRequestParser(requestBody);
    currentUI = new InspectorUI('inspectorContent');
    
    // 渲染界面
    currentUI.render(currentParser);
    
    // 显示模态框
    const modal = new bootstrap.Modal(document.getElementById('requestInspectorModal'));
    modal.show();
}

// 折叠/展开控制
function toggleCollapse(elementId) {
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

// 导出分析功能
function exportAnalysis() {
    if (!currentParser) return;
    
    const analysis = {
        overview: currentParser.parsed.overview,
        messageCount: currentParser.parsed.messages.length,
        toolCallCount: currentParser.parsed.messages.reduce((count, msg) => 
            count + (msg.pairedToolCalls ? msg.pairedToolCalls.length : 0), 0),
        systemRemindersCount: currentParser.parsed.messages.reduce((count, msg) => 
            count + msg.systemReminders.length, 0),
        exportTime: new Date().toISOString()
    };
    
    const dataStr = JSON.stringify(analysis, null, 2);
    const dataBlob = new Blob([dataStr], {type: 'application/json'});
    const url = URL.createObjectURL(dataBlob);
    
    const link = document.createElement('a');
    link.href = url;
    link.download = `anthropic-analysis-${Date.now()}.json`;
    link.click();
    
    URL.revokeObjectURL(url);
}

// 工具函数
function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatTimestamp(timestamp) {
    return new Date(timestamp).toLocaleString();
}
```

### 6. 样式实现 (inspector.css)

```css
/* Anthropic 请求检查器样式 */
.inspector-section {
    margin-bottom: 2rem;
    border: 1px solid #e0e0e0;
    border-radius: 8px;
    padding: 1rem;
}

.inspector-title {
    color: #333;
    font-weight: 600;
    margin-bottom: 1rem;
    border-bottom: 2px solid #f0f0f0;
    padding-bottom: 0.5rem;
}

.inspector-stat {
    text-align: center;
    padding: 0.75rem;
    background: #f8f9fa;
    border-radius: 6px;
    border: 1px solid #e9ecef;
}

.inspector-stat-label {
    font-size: 0.875rem;
    color: #6c757d;
    margin-bottom: 0.25rem;
}

.inspector-stat-value {
    font-size: 1.1rem;
    font-weight: 600;
    color: #495057;
}

.inspector-subsection {
    margin-bottom: 1rem;
}

.inspector-collapse-header {
    cursor: pointer;
    padding: 0.5rem;
    background: #f8f9fa;
    border: 1px solid #dee2e6;
    border-radius: 4px;
    margin-bottom: 0.5rem;
    transition: background-color 0.2s;
    font-weight: 500;
}

.inspector-collapse-header:hover {
    background: #e9ecef;
}

.inspector-collapse-icon {
    display: inline-block;
    width: 1rem;
    text-align: center;
    margin-right: 0.5rem;
    transition: transform 0.2s;
}

.inspector-collapse-content {
    margin-left: 1rem;
    padding: 0.5rem;
    border-left: 3px solid #dee2e6;
}

.inspector-content-box {
    background: #f8f9fa;
    border: 1px solid #e9ecef;
    border-radius: 4px;
    padding: 0.75rem;
    margin: 0.5rem 0;
}

.inspector-code, .inspector-json, .inspector-text {
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

.inspector-tool-item {
    margin-bottom: 0.75rem;
    border: 1px solid #e0e0e0;
    border-radius: 6px;
    overflow: hidden;
}

.inspector-tool-header {
    background: #fff;
    border: none;
    margin: 0;
}

.inspector-tool-details {
    padding: 1rem;
    background: #fafafa;
}

.inspector-param-list {
    list-style: none;
    padding: 0;
    margin: 0.5rem 0;
}

.inspector-param-item {
    padding: 0.5rem 0;
    border-bottom: 1px solid #eee;
}

.inspector-param-item:last-child {
    border-bottom: none;
}

.inspector-param-type {
    color: #6c757d;
    font-size: 0.875rem;
}

.inspector-param-desc {
    color: #6c757d;
    font-size: 0.875rem;
    margin-top: 0.25rem;
    font-style: italic;
}

.inspector-message {
    margin-bottom: 1.5rem;
    border: 1px solid #e0e0e0;
    border-radius: 8px;
    overflow: hidden;
}

.inspector-message-user {
    border-left: 4px solid #007bff;
}

.inspector-message-assistant {
    border-left: 4px solid #28a745;
}

.inspector-message-header {
    background: #f8f9fa;
    padding: 0.75rem;
    font-weight: 600;
    border-bottom: 1px solid #e0e0e0;
}

.inspector-message-content {
    padding: 1rem;
}

.inspector-content-item {
    margin-bottom: 1rem;
}

.inspector-preview {
    color: #6c757d;
    font-style: italic;
    font-size: 0.875rem;
    margin: 0.25rem 0;
    padding: 0.5rem;
    background: #f8f9fa;
    border-radius: 4px;
}

.inspector-reminder-item {
    margin-bottom: 0.5rem;
    border: 1px solid #ffc107;
    border-radius: 4px;
    background: #fff3cd;
}

.inspector-tool-call {
    margin-bottom: 0.75rem;
    border: 1px solid #17a2b8;
    border-radius: 6px;
    background: #d1ecf1;
}

.inspector-tool-call-header {
    padding: 0.75rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-weight: 500;
}

.inspector-tool-status {
    font-size: 1.1rem;
    margin-right: 0.5rem;
}

.inspector-tool-call-details {
    padding: 1rem;
    background: #fff;
    border-top: 1px solid #bee5eb;
}

.inspector-call-section {
    margin-bottom: 1rem;
}

.inspector-result-status {
    margin: 0.5rem 0;
    font-size: 0.875rem;
    font-weight: 500;
}

.inspector-errors {
    border-color: #dc3545;
    background: #f8d7da;
}

/* 响应式调整 */
@media (max-width: 768px) {
    .inspector-stat {
        margin-bottom: 0.5rem;
    }
    
    .inspector-tool-call-header {
        flex-direction: column;
        align-items: flex-start;
    }
    
    .inspector-collapse-content {
        margin-left: 0.5rem;
    }
}

/* 模态框大小调整 */
.modal-xl {
    max-width: 90%;
}

@media (min-width: 1200px) {
    .modal-xl {
        max-width: 1400px;
    }
}
```

## 集成步骤

### 1. 文件添加
1. 将 CSS 文件添加到 `web/static/css/inspector.css`
2. 将 JS 文件添加到 `web/static/js/` 目录
3. 在 `logs.html` 中引入相关文件

### 2. HTML 修改
1. 在 `logs.html` 的 `<head>` 中添加样式引用
2. 在页面底部添加 JavaScript 引用
3. 在请求详情工具栏添加检查按钮
4. 在页面底部添加模态框 HTML

### 3. 现有代码修改
1. 修改 `showLogDetail` 函数，添加按钮显示逻辑
2. 添加请求类型检测函数

### 4. 测试验证
1. 测试各种类型的 Anthropic 请求
2. 验证折叠/展开功能
3. 测试工具调用配对逻辑
4. 验证响应式布局

这个实现方案提供了完整的技术细节，可以直接基于此进行开发。需要我开始实现某个具体部分吗？
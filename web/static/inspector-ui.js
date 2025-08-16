class InspectorUI {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        this.collapseStates = new Map();
    }

    render(parser) {
        if (!this.container) {
            console.error('Inspector container not found');
            return;
        }

        this.container.innerHTML = '';
        
        // 渲染概览
        this.renderOverview(parser.parsed.overview);
        
        // 渲染消息
        this.renderMessages(parser.parsed.messages);
        
        // 渲染系统配置（移至消息后面）
        this.renderSystem(parser.parsed.system, parser.parsed.tools);
        
        // 如果有错误，显示错误信息
        if (parser.parsed.errors.length > 0) {
            this.renderErrors(parser.parsed.errors);
        }
    }

    renderOverview(overview) {
        const overviewId = 'request-overview';
        const overviewHtml = `
            <div class="inspector-section">
                <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${overviewId}')">
                    <span class="inspector-collapse-icon" id="${overviewId}-icon">▶</span>
                    📊 请求概览
                </div>
                <div class="inspector-collapse-content" id="${overviewId}" style="display: none;">
                    <div class="row g-3">
                    <div class="col-md-3">
                        <div class="inspector-stat">
                            <div class="inspector-stat-label">模型</div>
                            <div class="inspector-stat-value">${this.escapeHtml(overview.model)}</div>
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
                    ${overview.thinkingEnabled ? `
                    <div class="col-md-3">
                        <div class="inspector-stat">
                            <div class="inspector-stat-label">思考模式</div>
                            <div class="inspector-stat-value">${overview.thinkingBudget} tokens</div>
                        </div>
                    </div>
                    ` : ''}
                    </div>
                    ${overview.estimatedTokens > 0 ? `
                    <div class="row g-3 mt-2">
                        <div class="col-md-12">
                            <div class="inspector-stat">
                                <div class="inspector-stat-label">预估令牌</div>
                                <div class="inspector-stat-value">${overview.estimatedTokens}</div>
                            </div>
                        </div>
                    </div>
                    ` : ''}
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
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${systemId}')">
                        <span class="inspector-collapse-icon" id="${systemId}-icon">▶</span>
                        📝 System Prompt (${system.characterCount} 字符, ${system.wordCount} 词)
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
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${toolsId}')">
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
            const toolId = `tool-${this.sanitizeId(tool.name)}`;
            return `
                <div class="inspector-tool-item">
                    <div class="inspector-collapse-header inspector-tool-header" onclick="window.inspectorToggleCollapse('${toolId}')">
                        <span class="inspector-collapse-icon" id="${toolId}-icon">▶</span>
                        🔧 ${this.escapeHtml(tool.name)}
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
        
        // 工具描述（可折叠）
        if (tool.description) {
            const descId = `tool-desc-${this.sanitizeId(tool.name)}`;
            detailsHtml += `
                <div class="inspector-tool-subsection">
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${descId}')">
                        <span class="inspector-collapse-icon" id="${descId}-icon">▶</span>
                        📖 描述
                    </div>
                    <div class="inspector-collapse-content" id="${descId}" style="display: none;">
                        <div class="inspector-content-box">
                            ${this.escapeHtml(tool.description)}
                        </div>
                    </div>
                </div>
            `;
        }
        
        // 参数列表（可折叠）
        if (tool.parameters.length > 0) {
            const paramsId = `tool-params-${this.sanitizeId(tool.name)}`;
            detailsHtml += `
                <div class="inspector-tool-subsection">
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${paramsId}')">
                        <span class="inspector-collapse-icon" id="${paramsId}-icon">▶</span>
                        📋 参数列表 (${tool.parameters.length}个)
                    </div>
                    <div class="inspector-collapse-content" id="${paramsId}" style="display: none;">
                        <ul class="inspector-param-list">
            `;
            
            tool.parameters.forEach(param => {
                const requiredBadge = param.required ? 
                    '<span class="badge bg-danger">必需</span>' : 
                    '<span class="badge bg-secondary">可选</span>';
                detailsHtml += `
                    <li class="inspector-param-item">
                        <code>${this.escapeHtml(param.name)}</code> 
                        <span class="inspector-param-type">(${this.escapeHtml(param.type)})</span>
                        ${requiredBadge}
                        ${param.description ? `<div class="inspector-param-desc">${this.escapeHtml(param.description)}</div>` : ''}
                        ${param.enum ? `<div class="inspector-param-desc">可选值: ${param.enum.map(v => `<code>${this.escapeHtml(v)}</code>`).join(', ')}</div>` : ''}
                    </li>
                `;
            });
            
            detailsHtml += `
                        </ul>
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
                        <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${contentId}')">
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
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${remindersId}')">
                        <span class="inspector-collapse-icon" id="${remindersId}-icon">▶</span>
                        ⚠️ System Reminders (${message.systemReminders.length}个)
                    </div>
                    <div class="inspector-collapse-content" id="${remindersId}" style="display: none;">
                        ${this.renderSystemReminders(message.systemReminders, message.index)}
                    </div>
                </div>
            `;
        }

        // 渲染工具调用 - assistant 使用配对的工具调用，user 显示原始工具调用
        if (message.role === 'assistant' && message.pairedToolCalls && message.pairedToolCalls.length > 0) {
            const toolCallsId = `message-${message.index}-tools`;
            messageHtml += `
                <div class="inspector-content-item">
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${toolCallsId}')">
                        <span class="inspector-collapse-icon" id="${toolCallsId}-icon">▼</span>
                        🔧 工具调用 (${message.pairedToolCalls.length}次)
                    </div>
                    <div class="inspector-collapse-content" id="${toolCallsId}" style="display: block;">
                        ${this.renderToolCalls(message.pairedToolCalls, message.index)}
                    </div>
                </div>
            `;
        } else if (message.role === 'user' && message.toolUses && message.toolUses.length > 0) {
            // 为用户消息显示工具调用，只显示参数
            const userToolsId = `message-${message.index}-user-tools`;
            messageHtml += `
                <div class="inspector-content-item">
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${userToolsId}')">
                        <span class="inspector-collapse-icon" id="${userToolsId}-icon">▼</span>
                        🔧 工具调用 (${message.toolUses.length}个)
                    </div>
                    <div class="inspector-collapse-content" id="${userToolsId}" style="display: block;">
                        ${this.renderUserToolCalls(message.toolUses, message.index)}
                    </div>
                </div>
            `;
        }

        messageHtml += '</div></div>';
        return messageHtml;
    }

    renderSystemReminders(reminders, messageIndex) {
        return reminders.map((reminder, idx) => {
            const reminderId = `reminder-${messageIndex}-${idx}`;
            const typeIcon = this.getReminderIcon(reminder.type);
            
            return `
                <div class="inspector-reminder-item">
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${reminderId}')">
                        <span class="inspector-collapse-icon" id="${reminderId}-icon">▶</span>
                        ${typeIcon} ${this.escapeHtml(reminder.type)}: ${this.escapeHtml(reminder.preview)}
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

    renderUserToolCalls(toolUses, messageIndex) {
        // 用户消息中可能包含 tool_result (工具调用结果) 或 tool_use (工具调用请求)
        const relevantTools = toolUses.filter(tool => tool.type === 'use' || tool.type === 'result');
        return relevantTools.map((tool, idx) => {
            const callId = `user-tool-${messageIndex}-${idx}`;
            const isResult = tool.type === 'result';
            const toolName = isResult ? `Tool Result (${tool.id})` : tool.name;
            const statusIcon = isResult ? '📥' : '🔧';
            
            return `
                <div class="inspector-tool-call">
                    <div class="inspector-tool-call-header" onclick="window.inspectorToggleCollapse('${callId}')" style="cursor: pointer;">
                        <div>
                            <span class="inspector-collapse-icon" id="${callId}-icon">▶</span>
                            <span class="inspector-tool-status">${statusIcon}</span>
                            ${this.escapeHtml(toolName)}
                        </div>
                    </div>
                    <div class="inspector-collapse-content" id="${callId}" style="display: none;">
                        <div class="inspector-tool-call-details">
                            ${isResult ? `
                                <div class="inspector-call-section">
                                    <strong>📥 工具结果:</strong>
                                    <div class="inspector-content-box">
                                        <pre class="inspector-text">${this.escapeHtml(typeof tool.result === 'string' ? tool.result : JSON.stringify(tool.result, null, 2))}</pre>
                                    </div>
                                </div>
                            ` : `
                                <div class="inspector-call-section">
                                    <strong>📤 调用参数:</strong>
                                    <div class="inspector-content-box">
                                        <pre class="inspector-json">${this.formatJSON(tool.input)}</pre>
                                    </div>
                                </div>
                            `}
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
                    <div class="inspector-tool-call-header" onclick="window.inspectorToggleCollapse('${callId}')" style="cursor: pointer;">
                        <div>
                            <span class="inspector-collapse-icon" id="${callId}-icon">▶</span>
                            <span class="inspector-tool-status">${statusIcon}</span>
                            🔧 ${this.escapeHtml(call.name)}${thinkingLabel}
                        </div>
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
                    <pre class="inspector-json">${this.formatJSON(call.input)}</pre>
                </div>
            </div>
        `;

        // 返回结果
        if (call.result !== null) {
            const resultStr = typeof call.result === 'string' ? call.result : JSON.stringify(call.result);
            const resultPreview = resultStr.length > 200 ? resultStr.substring(0, 200) + '...' : resultStr;
                
            detailsHtml += `
                <div class="inspector-call-section">
                    <strong>📥 返回结果:</strong>
                    <div class="inspector-result-status">
                        状态: ${call.status === 'success' ? '✅ 成功' : call.status === 'error' ? '❌ 失败' : '⏳ 处理中'}
                        ${resultStr ? `(${resultStr.length} 字符)` : ''}
                    </div>
                    <div class="inspector-content-box">
                        <pre class="inspector-text">${this.escapeHtml(resultPreview)}</pre>
                        ${resultStr.length > 200 ? `
                        <div class="mt-2">
                            <button class="btn btn-sm btn-outline-info w-100 mb-3" onclick="const target = this.parentElement.querySelector('.full-result-container'); const isHidden = target.style.display === 'none' || !target.style.display; target.style.display = isHidden ? 'block' : 'none'; this.textContent = isHidden ? '隐藏完整结果' : '显示完整结果'">显示完整结果</button>
                            <div class="full-result-container" style="display: none; clear: both;">
                                <pre class="inspector-text">${this.escapeHtml(resultStr)}</pre>
                            </div>
                        </div>
                        ` : ''}
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
            'instruction': '📋',
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
        if (!text) return '';
        // 保持中文字符不变，只转义必要的HTML字符
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    formatJSON(obj) {
        try {
            return JSON.stringify(obj, null, 2);
        } catch (e) {
            return String(obj);
        }
    }

    sanitizeId(str) {
        return str.replace(/[^a-zA-Z0-9-_]/g, '_');
    }
}
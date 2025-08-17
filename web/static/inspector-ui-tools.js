// Inspector UI Tools - Tool rendering functionality
InspectorUI.prototype.renderSystemReminders = function(reminders, messageIndex) {
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
};

InspectorUI.prototype.renderAssistantToolUses = function(toolUses, messageIndex) {
    return toolUses.map((tool, idx) => {
        const callId = `assistant-tool-${messageIndex}-${idx}`;
        const paramsPreview = this.formatParametersPreview(tool.input);
        
        return `
            <div class="inspector-tool-call">
                <div class="inspector-tool-call-header" onclick="window.inspectorToggleCollapse('${callId}')" style="cursor: pointer;">
                    <div>
                        <span class="inspector-collapse-icon" id="${callId}-icon">▶</span>
                        <span class="inspector-tool-status">🔧</span>
                        ${this.escapeHtml(tool.name)}${paramsPreview}
                    </div>
                </div>
                <div class="inspector-collapse-content" id="${callId}" style="display: none;">
                    <div class="inspector-tool-call-details">
                        <div class="inspector-call-section">
                            <strong>📤 调用参数:</strong>
                            <div class="inspector-content-box">
                                <pre class="inspector-json">${this.formatJSON(tool.input)}</pre>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }).join('');
};

InspectorUI.prototype.renderUserToolResults = function(toolResults, messageIndex) {
    return toolResults.map((toolResult, idx) => {
        const callId = `user-result-${messageIndex}-${idx}`;
        // 查找对应的 tool_use 来获取工具名称和参数
        const correspondingUse = this.findCorrespondingToolUseGlobally(toolResult.id);
        const toolName = correspondingUse ? correspondingUse.name : 'Unknown Tool';
        const paramsPreview = correspondingUse ? this.formatParametersPreview(correspondingUse.input) : '';
        
        return `
            <div class="inspector-tool-call">
                <div class="inspector-tool-call-header" onclick="window.inspectorToggleCollapse('${callId}')" style="cursor: pointer;">
                    <div>
                        <span class="inspector-collapse-icon" id="${callId}-icon">▶</span>
                        <span class="inspector-tool-status">📥</span>
                        ${this.escapeHtml(toolName)}${paramsPreview}
                    </div>
                </div>
                <div class="inspector-collapse-content" id="${callId}" style="display: none;">
                    <div class="inspector-tool-call-details">
                        ${correspondingUse ? `
                            <div class="inspector-call-section">
                                <strong>📤 调用参数:</strong>
                                <div class="inspector-content-box">
                                    <pre class="inspector-json">${this.formatJSON(correspondingUse.input)}</pre>
                                </div>
                            </div>
                        ` : ''}
                        <div class="inspector-call-section">
                            <strong>📥 返回结果:</strong>
                            <div class="inspector-content-box">
                                <pre class="inspector-text">${this.escapeHtml(typeof toolResult.result === 'string' ? toolResult.result : JSON.stringify(toolResult.result, null, 2))}</pre>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        `;
    }).join('');
};

InspectorUI.prototype.findCorrespondingToolUseGlobally = function(toolId) {
    // 在所有消息中查找对应的 tool_use
    if (this.currentParser && this.currentParser.parsed && this.currentParser.parsed.messages) {
        for (const message of this.currentParser.parsed.messages) {
            if (message.toolUses) {
                const foundTool = message.toolUses.find(tool => tool.type === 'use' && tool.id === toolId);
                if (foundTool) {
                    return foundTool;
                }
            }
        }
    }
    return null;
};

InspectorUI.prototype.renderUserToolCalls = function(toolUses, messageIndex) {
    // 用户消息中可能包含 tool_result (工具调用结果) 或 tool_use (工具调用请求)
    const relevantTools = toolUses.filter(tool => tool.type === 'use' || tool.type === 'result');
    return relevantTools.map((tool, idx) => {
        const callId = `user-tool-${messageIndex}-${idx}`;
        const isResult = tool.type === 'result';
        
        let toolName, statusIcon, paramsPreview = '';
        if (isResult) {
            // 尝试找到对应的工具调用来获取工具名称
            const correspondingUse = this.findCorrespondingToolUse(tool.id, toolUses);
            toolName = correspondingUse ? correspondingUse.name : `Tool Result`;
            statusIcon = '📥';
            if (correspondingUse && correspondingUse.input) {
                paramsPreview = this.formatParametersPreview(correspondingUse.input);
            }
        } else {
            toolName = tool.name;
            statusIcon = '🔧';
            if (tool.input) {
                paramsPreview = this.formatParametersPreview(tool.input);
            }
        }
        
        return `
            <div class="inspector-tool-call">
                <div class="inspector-tool-call-header" onclick="window.inspectorToggleCollapse('${callId}')" style="cursor: pointer;">
                    <div>
                        <span class="inspector-collapse-icon" id="${callId}-icon">▶</span>
                        <span class="inspector-tool-status">${statusIcon}</span>
                        ${this.escapeHtml(toolName)}${paramsPreview}
                    </div>
                </div>
                <div class="inspector-collapse-content" id="${callId}" style="display: none;">
                    <div class="inspector-tool-call-details">
                        ${isResult ? `
                            ${this.renderToolResultWithParameters(tool, toolUses)}
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
};

InspectorUI.prototype.findCorrespondingToolUse = function(toolId, toolUses) {
    return toolUses.find(tool => tool.type === 'use' && tool.id === toolId);
};

InspectorUI.prototype.renderToolResultWithParameters = function(toolResult, toolUses) {
    const correspondingUse = this.findCorrespondingToolUse(toolResult.id, toolUses);
    
    let html = '';
    
    // 显示对应的调用参数
    if (correspondingUse) {
        html += `
            <div class="inspector-call-section">
                <strong>📤 调用参数:</strong>
                <div class="inspector-content-box">
                    <pre class="inspector-json">${this.formatJSON(correspondingUse.input)}</pre>
                </div>
            </div>
        `;
    }
    
    // 显示工具结果
    html += `
        <div class="inspector-call-section">
            <strong>📥 工具结果:</strong>
            <div class="inspector-content-box">
                <pre class="inspector-text">${this.escapeHtml(typeof toolResult.result === 'string' ? toolResult.result : JSON.stringify(toolResult.result, null, 2))}</pre>
            </div>
        </div>
    `;
    
    return html;
};

InspectorUI.prototype.renderToolCalls = function(toolCalls, messageIndex) {
    return toolCalls.map((call, idx) => {
        const callId = `toolcall-${messageIndex}-${idx}`;
        const statusIcon = this.getToolStatusIcon(call.status, call.isThinking);
        const thinkingLabel = call.isThinking ? ' (Thinking)' : '';
        const paramsPreview = call.input ? this.formatParametersPreview(call.input) : '';
        
        return `
            <div class="inspector-tool-call">
                <div class="inspector-tool-call-header" onclick="window.inspectorToggleCollapse('${callId}')" style="cursor: pointer;">
                    <div>
                        <span class="inspector-collapse-icon" id="${callId}-icon">▶</span>
                        <span class="inspector-tool-status">${statusIcon}</span>
                        🔧 ${this.escapeHtml(call.name)}${thinkingLabel}${paramsPreview}
                    </div>
                </div>
                <div class="inspector-collapse-content" id="${callId}" style="display: none;">
                    ${this.renderToolCallDetails(call)}
                </div>
            </div>
        `;
    }).join('');
};
// Inspector UI Messages - Message rendering functionality
InspectorUI.prototype.renderMessages = function(messages) {
    // Default to reverse order (newest first)
    const reversedMessages = [...messages].reverse();
    
    let messagesHtml = `
        <div class="inspector-section">
            <div class="inspector-title-bar" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
                <h6 class="inspector-title" style="margin-bottom: 0;">${T('inspector_conversation_messages', '💬 对话消息')}</h6>
                <button class="btn btn-outline-primary btn-sm inspector-main-btn" onclick="window.inspectorToggleMessageOrder()" id="message-order-toggle" data-reversed="true" title="${T('inspector_toggle_message_order', '切换消息排序')}">
                    <span id="message-order-icon">↓</span>
                    <span id="message-order-text">${T('inspector_reverse_order', '逆向排列')}</span>
                </button>
            </div>
            <div id="messages-container">
    `;

    reversedMessages.forEach(message => {
        messagesHtml += this.renderMessage(message);
    });

    messagesHtml += '</div></div>';
    this.container.appendChild(this.createElementFromHTML(messagesHtml));
};

InspectorUI.prototype.renderMessage = function(message) {
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
                        💭 ${T('inspector_text_content', '正文内容')} (${content.text.length} ${T('inspector_characters', '字符')})
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
                    ⚠️ ${T('inspector_system_reminders', 'System Reminders')} (${message.systemReminders.length}${T('inspector_count_suffix', '个')})
                </div>
                <div class="inspector-collapse-content" id="${remindersId}" style="display: none;">
                    ${this.renderSystemReminders(message.systemReminders, message.index)}
                </div>
            </div>
        `;
    }

    // 渲染工具调用 - assistant 显示 tool_use，user 显示 tool_result
    if (message.role === 'assistant' && message.toolUses && message.toolUses.length > 0) {
        // 只显示 tool_use，不显示结果
        const toolUses = message.toolUses.filter(tool => tool.type === 'use');
        if (toolUses.length > 0) {
            const toolCallsId = `message-${message.index}-tools`;
            messageHtml += `
                <div class="inspector-content-item">
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${toolCallsId}')">
                        <span class="inspector-collapse-icon" id="${toolCallsId}-icon">▼</span>
                        🔧 ${T('inspector_tool_calls', '工具调用')} (${toolUses.length}${T('inspector_times_suffix', '次')})
                    </div>
                    <div class="inspector-collapse-content" id="${toolCallsId}" style="display: block;">
                        ${this.renderAssistantToolUses(toolUses, message.index)}
                    </div>
                </div>
            `;
        }
    } else if (message.role === 'user' && message.toolUses && message.toolUses.length > 0) {
        // 用户消息显示工具结果
        const toolResults = message.toolUses.filter(tool => tool.type === 'result');
        if (toolResults.length > 0) {
            const userToolsId = `message-${message.index}-user-tools`;
            messageHtml += `
                <div class="inspector-content-item">
                    <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${userToolsId}')">
                        <span class="inspector-collapse-icon" id="${userToolsId}-icon">▼</span>
                        🔧 ${T('inspector_tool_results', '工具结果')} (${toolResults.length}${T('inspector_count_suffix', '个')})
                    </div>
                    <div class="inspector-collapse-content" id="${userToolsId}" style="display: block;">
                        ${this.renderUserToolResults(toolResults, message.index)}
                    </div>
                </div>
            `;
        }
    }

    messageHtml += '</div></div>';
    return messageHtml;
};
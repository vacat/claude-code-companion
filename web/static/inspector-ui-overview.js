// Inspector UI Overview - Overview rendering functionality
InspectorUI.prototype.renderOverview = function(overview) {
    const overviewId = 'request-overview';
    const overviewHtml = `
        <div class="inspector-section">
            <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${overviewId}')">
                <span class="inspector-collapse-icon" id="${overviewId}-icon">▼</span>
${T('inspector_request_overview', '📊 请求概览')}
            </div>
            <div class="inspector-collapse-content" id="${overviewId}" style="display: block;">
                <div class="inspector-overview-compact">
                    📈 ${this.escapeHtml(overview.model)} | 
                    🎯 ${overview.maxTokens} ${T('inspector_tokens', 'tokens')} | 
                    💬 ${overview.messageCount} ${T('inspector_messages', '消息')} | 
                    🔧 ${overview.toolCount} ${T('inspector_tools', '工具')}${overview.thinkingEnabled ? ` | 🧠 ${overview.thinkingBudget} ${T('inspector_tokens', 'tokens')}` : ''}${overview.estimatedTokens > 0 ? ` | 📊 ${T('inspector_estimated', '预估')} ${overview.estimatedTokens} ${T('inspector_tokens', 'tokens')}` : ''}
                </div>
            </div>
        </div>
    `;
    
    this.container.appendChild(this.createElementFromHTML(overviewHtml));
};
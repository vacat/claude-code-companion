// Inspector UI Overview - Overview rendering functionality
InspectorUI.prototype.renderOverview = function(overview) {
    const overviewId = 'request-overview';
    const overviewHtml = `
        <div class="inspector-section">
            <div class="inspector-collapse-header" onclick="window.inspectorToggleCollapse('${overviewId}')">
                <span class="inspector-collapse-icon" id="${overviewId}-icon">▼</span>
                📊 请求概览
            </div>
            <div class="inspector-collapse-content" id="${overviewId}" style="display: block;">
                <div class="inspector-overview-compact">
                    📈 ${this.escapeHtml(overview.model)} | 
                    🎯 ${overview.maxTokens} tokens | 
                    💬 ${overview.messageCount} 消息 | 
                    🔧 ${overview.toolCount} 工具${overview.thinkingEnabled ? ` | 🧠 ${overview.thinkingBudget} tokens` : ''}${overview.estimatedTokens > 0 ? ` | 📊 预估 ${overview.estimatedTokens} tokens` : ''}
                </div>
            </div>
        </div>
    `;
    
    this.container.appendChild(this.createElementFromHTML(overviewHtml));
};
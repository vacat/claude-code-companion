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
            <div class="response-inspector-section compact">
                <h6 class="response-inspector-title">${T('inspector_response_overview', '📊 响应概览')}</h6>
                <div class="response-inspector-compact-grid">
                    <span><strong>${T('model', '模型')}:</strong> ${metadata.model || T('inspector_unknown', 'Unknown')}</span>
                    <span><strong>${T('inspector_stop_reason', '停止原因')}:</strong> ${metadata.stop_reason || T('inspector_unknown', 'Unknown')}</span>
                    <span><strong>${T('streaming', '流式')}:</strong> ${metadata.isStreaming ? '✅' : '❌'}</span>
                </div>
            </div>
        `;
        
        this.container.appendChild(this.createElementFromHTML(overviewHtml));
    }

    renderUsage(usage) {
        if (!usage.total_tokens) return;
        
        // 准备Cache效率状态
        const cacheStatus = usage.cache_efficiency > 30 ? `${T('inspector_cache_efficient', '高效')} ✅` : 
                           usage.cache_efficiency > 10 ? `${T('inspector_cache_medium', '中等')} ⚠️` : `${T('inspector_cache_inefficient', '低效')} ⚠️`;
        
        const usageHtml = `
            <div class="response-inspector-section compact">
                <h6 class="response-inspector-title">${T('inspector_token_cache_usage', '💰 Token和Cache使用情况')}</h6>
                <div class="response-inspector-compact-grid">
                    <span><strong>${T('inspector_original_input', '原始输入')}:</strong> ${usage.input_tokens}</span>
                    <span><strong>${T('inspector_cache_creation', 'Cache创建')}:</strong> ${usage.cache_creation_input_tokens}</span>
                    <span><strong>${T('inspector_cache_read', 'Cache读取')}:</strong> ${usage.cache_read_input_tokens}</span>
                    <span><strong>${T('inspector_total_input', '总输入')}:</strong> ${usage.total_input_tokens}</span>
                    <span><strong>${T('inspector_total_output', '总输出')}:</strong> ${usage.output_tokens}</span>
                    <span><strong>${T('inspector_total', '总计')}:</strong> ${usage.total_tokens}</span>
                    <span><strong>${T('inspector_cache_efficiency', 'Cache效率')}:</strong> ${usage.cache_efficiency}%</span>
                    <span><strong>${T('inspector_cache_status', 'Cache状态')}:</strong> ${cacheStatus}</span>
                </div>
            </div>
        `;
        
        this.container.appendChild(this.createElementFromHTML(usageHtml));
    }

    renderContent(content) {
        let contentHtml = `
            <div class="response-inspector-section">
                <h6 class="response-inspector-title">${T('inspector_response_content', '💬 响应内容')}</h6>
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
                contentPreview = `${block.metadata.characterCount} ${T('inspector_characters', '字符')}, ${block.metadata.wordCount} ${T('inspector_words', '词')}`;
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <pre class="response-inspector-text">${this.escapeHtml(block.content)}</pre>
                    </div>
                `;
                break;
                
            case 'tool_use':
                contentPreview = `${block.content.name} - ${block.metadata.inputSize} ${T('inspector_character_input', '字符输入')}`;
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <strong>${T('inspector_tool_name', '工具名称')}:</strong> ${block.content.name}<br>
                        <strong>${T('inspector_tool_id', '工具ID')}:</strong> ${block.content.id}<br>
                        <strong>${T('inspector_input_parameters', '输入参数')}:</strong>
                        <pre class="response-inspector-json">${JSON.stringify(block.content.input, null, 2)}</pre>
                    </div>
                `;
                break;
                
            case 'thinking':
                contentPreview = `${block.metadata.characterCount} ${T('inspector_character_thinking_content', '字符推理内容')}`;
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <div class="alert alert-info">
                            <strong>🧠 ${T('inspector_thinking_mode_content', 'Thinking 模式内容')}</strong><br>
                            ${T('inspector_thinking_content_description', '此内容为模型的内部推理过程，通常对用户不可见。')}
                        </div>
                        <pre class="response-inspector-text">${this.escapeHtml(block.content)}</pre>
                    </div>
                `;
                break;
                
            default:
                contentPreview = T('inspector_unknown_content_type', '未知内容类型');
                contentDetails = `
                    <div class="response-inspector-content-box">
                        <pre class="response-inspector-json">${JSON.stringify(block.content, null, 2)}</pre>
                    </div>
                `;
        }
        
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
                <h6 class="response-inspector-title text-danger">⚠️ ${T('inspector_parse_errors', '解析错误')}</h6>
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
        return escapeHtml(text);
    }
}
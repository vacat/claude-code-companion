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
// Inspector UI Utilities - Utility methods and error handling
InspectorUI.prototype.renderToolCallDetails = function(call) {
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
                    ${resultStr.length > 200 ? `
                        <div class="result-preview">
                            <pre class="inspector-text">${this.escapeHtml(resultPreview)}</pre>
                        </div>
                        <div class="mt-2">
                            <button class="btn btn-sm btn-outline-info w-100 mb-2" onclick="
                                const preview = this.parentElement.parentElement.querySelector('.result-preview');
                                const fullResult = this.parentElement.parentElement.querySelector('.full-result-container');
                                const isShowingFull = fullResult.style.display === 'block';
                                if (isShowingFull) {
                                    preview.style.display = 'block';
                                    fullResult.style.display = 'none';
                                    this.textContent = '显示完整结果';
                                } else {
                                    preview.style.display = 'none';
                                    fullResult.style.display = 'block';
                                    this.textContent = '隐藏完整结果';
                                }
                            ">显示完整结果</button>
                        </div>
                        <div class="full-result-container" style="display: none;">
                            <pre class="inspector-text">${this.escapeHtml(resultStr)}</pre>
                        </div>
                    ` : `
                        <pre class="inspector-text">${this.escapeHtml(resultStr)}</pre>
                    `}
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
};

InspectorUI.prototype.getReminderIcon = function(type) {
    const icons = {
        'context': '🔄',
        'tool': '⚡',
        'reminder': '📌',
        'instruction': '📋',
        'general': '💡'
    };
    return icons[type] || '💡';
};

InspectorUI.prototype.getToolStatusIcon = function(status, isThinking) {
    if (isThinking) return '🧠';
    const icons = {
        'success': '✅',
        'error': '❌',
        'pending': '⏳'
    };
    return icons[status] || '❓';
};

InspectorUI.prototype.renderErrors = function(errors) {
    const errorsHtml = `
        <div class="inspector-section inspector-errors">
            <h6 class="inspector-title text-danger">⚠️ 解析错误</h6>
            ${errors.map(error => `<div class="alert alert-danger">${this.escapeHtml(error)}</div>`).join('')}
        </div>
    `;
    this.container.appendChild(this.createElementFromHTML(errorsHtml));
};

InspectorUI.prototype.formatParametersPreview = function(input) {
    if (!input || typeof input !== 'object') return '';
    
    const params = [];
    const maxValueLength = 30; // 最大参数值长度
    const maxTotalLength = 80; // 最大总长度
    
    for (const [key, value] of Object.entries(input)) {
        let valueStr = '';
        if (typeof value === 'string') {
            valueStr = value.length > maxValueLength ? value.substring(0, maxValueLength) + '...' : value;
        } else if (typeof value === 'number' || typeof value === 'boolean') {
            valueStr = String(value);
        } else if (Array.isArray(value)) {
            valueStr = `[${value.length} items]`;
        } else if (typeof value === 'object') {
            const keys = Object.keys(value);
            valueStr = `{${keys.length} keys}`;
        } else {
            valueStr = String(value);
        }
        
        // 转义HTML特殊字符
        valueStr = this.escapeHtml(valueStr);
        
        params.push(`${key}: ${valueStr}`);
    }
    
    if (params.length === 0) return '';
    
    let result = ' (' + params.join(', ') + ')';
    
    // 如果总长度超过限制，截短
    if (result.length > maxTotalLength) {
        result = result.substring(0, maxTotalLength - 3) + '...)';
    }
    
    // 返回带样式的HTML
    return `<span class="inspector-tool-params-preview">${result}</span>`;
};
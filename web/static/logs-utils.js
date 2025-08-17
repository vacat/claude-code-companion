// Logs Page Utility Functions

function displayLogDetails(log) {
    const modalBody = document.getElementById('modalBody');
    
    // Check if there are data transformations
    const requestChanges = hasRequestChanges(log);
    const responseChanges = hasResponseChanges(log);

    modalBody.innerHTML = `
        <div class="mb-3">
            <div class="collapsible-header" onclick="toggleCollapsible('basicInfo')">
                <span class="collapsible-toggle collapsed">▼</span>
                <h6 class="mb-0">基本信息</h6>
            </div>
            <div class="collapsible-content collapsed" id="basicInfo">
                <table class="table table-sm">
                    <tr><th>请求ID:</th><td>${escapeHtml(log.request_id)}</td></tr>
                    <tr><th>时间戳:</th><td>${new Date(log.timestamp).toLocaleString()}</td></tr>
                    <tr><th>端点:</th><td>${escapeHtml(log.endpoint)}</td></tr>
                    <tr><th>请求方法:</th><td>${escapeHtml(log.method)}</td></tr>
                    <tr><th>路径:</th><td>${escapeHtml(log.path)}</td></tr>
                    <tr><th>状态码:</th><td>${log.status_code}</td></tr>
                    <tr><th>重试次数:</th><td>
                        ${log.attempt_number && log.attempt_number > 1 ? 
                            `<span class="badge bg-warning text-dark">#${log.attempt_number - 1}</span>` : 
                            '<span class="text-muted">无重试</span>'
                        }
                    </td></tr>
                    <tr><th>模型:</th><td>
                        ${log.model ? 
                            (log.model_rewrite_applied ? 
                                `<span class="model-rewritten" title="→ ${escapeHtml(log.rewritten_model)}">${escapeHtml(log.model)}</span>` : 
                                `<span class="model-original">${escapeHtml(log.model)}</span>`
                            ) : 
                            '<small class="text-muted">无</small>'
                        }
                    </td></tr>
                    <tr><th>耗时:</th><td>${log.duration_ms}ms</td></tr>
                    <tr><th>请求体大小:</th><td>${log.request_body_size} 字节</td></tr>
                    <tr><th>响应体大小:</th><td>${log.response_body_size} 字节</td></tr>
                    <tr><th>流式响应:</th><td>${log.is_streaming ? '是 (SSE)' : '否'}</td></tr>
                    <tr><th>标签:</th><td>${log.tags && log.tags.length > 0 ? log.tags.map(tag => `<span class="badge bg-primary me-1">${escapeHtml(tag)}</span>`).join('') : '<small class="text-muted">无</small>'}</td></tr>
                    <tr><th>Content-Type覆盖:</th><td>${log.content_type_override ? `<span class="badge bg-warning text-dark">${escapeHtml(log.content_type_override)}</span>` : '<small class="text-muted">无</small>'}</td></tr>
                    ${log.error ? `<tr><th>错误:</th><td class="text-danger">${escapeHtml(log.error)}</td></tr>` : ''}
                </table>
            </div>
        </div>
        
        <!-- Request/Response Tabs -->
        <ul class="nav nav-tabs before-after-tabs" id="singleLogTabs" role="tablist">
            <li class="nav-item" role="presentation">
                <button class="nav-link active" id="single-request-tab" data-bs-toggle="tab" data-bs-target="#single-request" type="button" role="tab">
                    请求数据 ${requestChanges ? '<span class="comparison-badge badge bg-warning">修改</span>' : ''}
                </button>
            </li>
            <li class="nav-item" role="presentation">
                <button class="nav-link" id="single-response-tab" data-bs-toggle="tab" data-bs-target="#single-response" type="button" role="tab">
                    响应数据 ${responseChanges ? '<span class="comparison-badge badge bg-warning">修改</span>' : ''}
                </button>
            </li>
        </ul>
        
        <div class="tab-content mt-3" id="singleLogTabsContent">
            <!-- Request Tab -->
            <div class="tab-pane fade show active" id="single-request" role="tabpanel">
                ${generateRequestComparisonHtml(log, 'single')}
            </div>
            
            <!-- Response Tab -->  
            <div class="tab-pane fade" id="single-response" role="tabpanel">
                ${generateResponseComparisonHtml(log, 'single')}
            </div>
        </div>
    `;
    
    // Reinitialize tooltips for dynamic content
    var tooltipTriggerList = [].slice.call(modalBody.querySelectorAll('[title]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });
    
    const modal = new bootstrap.Modal(document.getElementById('logModal'));
    modal.show();
}

function toggleCollapsible(id) {
    const content = document.getElementById(id);
    const toggle = content.previousElementSibling.querySelector('.collapsible-toggle');
    
    if (content.classList.contains('collapsed')) {
        // Expand
        content.classList.remove('collapsed');
        toggle.classList.remove('collapsed');
        content.style.maxHeight = content.scrollHeight + 'px';
    } else {
        // Collapse
        content.classList.add('collapsed');
        toggle.classList.add('collapsed');
        content.style.maxHeight = '0px';
    }
}

// Helper function to create content box with floating actions
function createContentBoxWithActions(content, filename, encodedContent, maxHeight = '400px') {
    if (!content) content = '无内容';
    if (!encodedContent) encodedContent = '';
    
    return `
        <div class="json-pretty-container">
            <div class="json-pretty" style="max-height: ${maxHeight};">${content}</div>
            <div class="floating-actions">
                <button class="floating-action-btn" 
                        data-content="${encodedContent}"
                        onclick="copyFromButton(this)"
                        title="复制到剪贴板">
                    <i class="fas fa-copy"></i>
                </button>
                <button class="floating-action-btn" 
                        data-filename="${filename}"
                        data-content="${encodedContent}"
                        onclick="saveAsFileFromButton(this)"
                        ${!encodedContent ? 'disabled' : ''}
                        title="保存到文件">
                    <i class="fas fa-download"></i>
                </button>
            </div>
        </div>`;
}

function isAnthropicResponse(responseBody) {
    if (!responseBody) return false;
    
    try {
        // 尝试解码 base64
        let decodedBody = responseBody;
        try {
            decodedBody = safeBase64Decode(responseBody);
        } catch (e) {
            // 如果不是 base64，就使用原始字符串
        }
        
        // 检查非流式响应
        try {
            const data = JSON.parse(decodedBody);
            return data.type === 'message' && data.role === 'assistant';
        } catch {
            // 检查流式响应（SSE 格式）
            return decodedBody.includes('event: message_start') && 
                   decodedBody.includes('data: {"type"');
        }
    } catch (error) {
        console.error('Error checking if response is Anthropic:', error);
        return false;
    }
}
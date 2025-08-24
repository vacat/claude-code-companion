// Logs Page Utility Functions

function displayLogDetails(log) {
    const modalBody = document.getElementById('modalBody');
    
    // Check if there are data transformations
    const requestChanges = hasRequestChanges(log);
    const responseChanges = hasResponseChanges(log);

    modalBody.innerHTML = `
        <div class="mb-3">
            <div class="d-flex justify-content-between align-items-center">
                <h6 data-t="request_details">Request Details</h6>
                <button class="btn btn-sm btn-outline-success" onclick="exportDebugInfo('${escapeHtml(log.request_id)}')" 
                        data-t="export_debug_info" data-t-title="export_debug_info_tooltip">
                    <i class="fas fa-download"></i> <span data-t="export_debug_info">Export Debug Info</span>
                </button>
            </div>
        </div>
        
        <div class="mb-3">
            <div class="collapsible-header" onclick="toggleCollapsible('basicInfo')">
                <span class="collapsible-toggle collapsed">▼</span>
                <h6 class="mb-0" data-t="basic_info">Basic Information</h6>
            </div>
            <div class="collapsible-content collapsed" id="basicInfo">
                <table class="table table-sm">
                    <tr><th data-t="request_id">请求ID:</th><td>${escapeHtml(log.request_id)}</td></tr>
                    <tr><th data-t="timestamp">时间戳:</th><td>${new Date(log.timestamp).toLocaleString()}</td></tr>
                    <tr><th data-t="endpoint">端点:</th><td>${escapeHtml(log.endpoint)}</td></tr>
                    <tr><th data-t="request_method">请求方法:</th><td>${escapeHtml(log.method)}</td></tr>
                    <tr><th data-t="path">路径:</th><td>${escapeHtml(log.path)}</td></tr>
                    <tr><th data-t="status_code">状态码:</th><td>${log.status_code}</td></tr>
                    <tr><th data-t="retry_count">重试次数:</th><td>
                        ${log.attempt_number && log.attempt_number > 1 ? 
                            `<span class="badge bg-warning text-dark">#${log.attempt_number - 1}</span>` : 
                            `<span class="text-muted" data-t="no_retry">无重试</span>`
                        }
                    </td></tr>
                    <tr><th data-t="model">模型:</th><td>
                        ${log.model ? 
                            (log.model_rewrite_applied ? 
                                `<span class="model-rewritten" title="→ ${escapeHtml(log.rewritten_model)}">${escapeHtml(log.model)}</span>` : 
                                `<span class="model-original">${escapeHtml(log.model)}</span>`
                            ) : 
                            `<small class="text-muted" data-t="none">无</small>`
                        }
                    </td></tr>
                    <tr><th data-t="duration">耗时:</th><td>${log.duration_ms}ms</td></tr>
                    <tr><th data-t="request_body_size">请求体大小:</th><td>${log.request_body_size} <span data-t="bytes">字节</span></td></tr>
                    <tr><th data-t="response_body_size">响应体大小:</th><td>${log.response_body_size} <span data-t="bytes">字节</span></td></tr>
                    <tr><th data-t="streaming_response">流式响应:</th><td>${log.is_streaming ? `<span data-t="yes_sse">是 (SSE)</span>` : `<span data-t="no">否</span>`}</td></tr>
                    <tr><th data-t="tags">标签:</th><td>${log.tags && log.tags.length > 0 ? log.tags.map(tag => `<span class="badge bg-primary me-1">${escapeHtml(tag)}</span>`).join('') : `<small class="text-muted" data-t="none">无</small>`}</td></tr>
                    <tr><th data-t="content_type_override">Content-Type覆盖:</th><td>${log.content_type_override ? `<span class="badge bg-warning text-dark">${escapeHtml(log.content_type_override)}</span>` : `<small class="text-muted" data-t="none">无</small>`}</td></tr>
                    ${log.error ? `<tr><th data-t="error">错误:</th><td class="text-danger">${escapeHtml(log.error)}</td></tr>` : ''}
                </table>
            </div>
        </div>
        
        <!-- Request/Response Tabs -->
        <ul class="nav nav-tabs before-after-tabs" id="singleLogTabs" role="tablist">
            <li class="nav-item" role="presentation">
                <button class="nav-link active" id="single-request-tab" data-bs-toggle="tab" data-bs-target="#single-request" type="button" role="tab">
                    <span data-t="request_data">请求数据</span> ${requestChanges ? `<span class="comparison-badge badge bg-warning" data-t="modified">修改</span>` : ''}
                </button>
            </li>
            <li class="nav-item" role="presentation">
                <button class="nav-link" id="single-response-tab" data-bs-toggle="tab" data-bs-target="#single-response" type="button" role="tab">
                    <span data-t="response_data">Response Data</span> ${responseChanges ? `<span class="comparison-badge badge bg-warning" data-t="modified">Modified</span>` : ''}
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
    
    // Process translations for dynamic content
    if (window.I18n && window.I18n.processDataTElements) {
        window.I18n.processDataTElements();
    }
    
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
    if (!content) content = t('no_content', '无内容');
    if (!encodedContent) encodedContent = '';
    
    return `
        <div class="json-pretty-container">
            <div class="json-pretty" style="max-height: ${maxHeight};">${content}</div>
            <div class="floating-actions">
                <button class="floating-action-btn" 
                        data-content="${encodedContent}"
                        onclick="copyFromButton(this)"
                        data-t-title="copy_to_clipboard">
                    <i class="fas fa-copy"></i>
                </button>
                <button class="floating-action-btn" 
                        data-filename="${filename}"
                        data-content="${encodedContent}"
                        onclick="saveAsFileFromButton(this)"
                        ${!encodedContent ? 'disabled' : ''}
                        data-t-title="save_to_file">
                    <i class="fas fa-download"></i>
                </button>
            </div>
        </div>`;
}

function hasSSEFormatError(log) {
    if (!log || !log.error) return false;
    
    // 检查是否有 SSE 格式相关的错误信息
    const sseErrorPatterns = [
        'Incomplete SSE stream',
        'incomplete SSE stream',
        'missing message_stop',
        'missing [DONE]',
        'missing finish_reason',
        'has message_start but missing message_stop'
    ];
    
    return sseErrorPatterns.some(pattern => 
        log.error.includes(pattern)
    );
}

// 导出调试信息
function exportDebugInfo(requestId) {
    if (!requestId) {
        console.error('Request ID is required for export');
        return;
    }

    // 显示加载状态
    const exportButton = document.querySelector(`button[onclick="exportDebugInfo('${requestId}')"]`);
    if (exportButton) {
        const originalText = exportButton.innerHTML;
        exportButton.innerHTML = `<i class="fas fa-spinner fa-spin"></i> <span data-t="exporting">导出中...</span>`;
        exportButton.disabled = true;

        // 导出完成后恢复按钮状态
        const restoreButton = () => {
            exportButton.innerHTML = originalText;
            exportButton.disabled = false;
        };

        // 创建下载链接
        const downloadUrl = `/admin/api/logs/${encodeURIComponent(requestId)}/export`;
        
        // 创建一个隐藏的链接来触发下载
        const link = document.createElement('a');
        link.href = downloadUrl;
        link.download = `debug_${requestId}_${new Date().toISOString().replace(/[:.]/g, '-')}.zip`;
        StyleUtils.hide(link);
        
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);

        // 短暂延迟后恢复按钮状态
        setTimeout(restoreButton, 2000);
        
        // 显示成功提示
        showToast(t('export_debug_success', '导出调试信息成功，文件将开始下载'), 'success');
    }
}

// 显示提示消息的辅助函数
function showToast(message, type = 'info') {
    // 创建 toast 元素
    const toast = document.createElement('div');
    toast.className = `alert alert-${type} alert-dismissible fade show position-fixed`;
    StyleUtils.positionToast(toast);
    toast.innerHTML = `
        ${message}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    document.body.appendChild(toast);
    
    // 3秒后自动移除
    setTimeout(() => {
        if (toast.parentNode) {
            toast.parentNode.removeChild(toast);
        }
    }, 3000);
}

// Generate sessionid color based on last 6 characters
function generateSessionIdColor(sessionId) {
    try {
        if (!sessionId || typeof sessionId !== 'string') {
            return '#FFFFFF';
        }
        
        // Get last 6 characters
        const last6 = sessionId.slice(-6);
        
        // If less than 6 characters, pad with zeros
        const padded = last6.padStart(6, '0');
        
        // Validate hex characters
        if (!/^[0-9a-fA-F]{6}$/.test(padded)) {
            return '#FFFFFF';
        }
        
        return '#' + padded.toUpperCase();
    } catch (error) {
        console.warn('Error generating session ID color:', error);
        return '#FFFFFF';
    }
}

// Get sessionid display text (last 2 characters)
function getSessionIdDisplayText(sessionId) {
    try {
        if (!sessionId || typeof sessionId !== 'string') {
            return '--';
        }
        
        const last2 = sessionId.slice(-2);
        return last2.length > 0 ? last2.toUpperCase() : '--';
    } catch (error) {
        console.warn('Error getting session ID display text:', error);
        return '--';
    }
}
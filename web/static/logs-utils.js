// Logs Page Utility Functions

function displayLogDetails(log) {
    const modalBody = document.getElementById('modalBody');
    
    // Check if there are data transformations
    const requestChanges = hasRequestChanges(log);
    const responseChanges = hasResponseChanges(log);

    modalBody.innerHTML = `
        <div class="mb-3">
            <div class="d-flex justify-content-between align-items-center">
                <h6>请求详情</h6>
                <button class="btn btn-sm btn-outline-success" onclick="exportDebugInfo('${escapeHtml(log.request_id)}')" 
                        data-t="export_debug_info" title="导出调试信息为ZIP文件">
                    <i class="fas fa-download"></i> 导出调试信息
                </button>
            </div>
        </div>
        
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
        exportButton.innerHTML = '<i class="fas fa-spinner fa-spin"></i> 导出中...';
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
        link.style.display = 'none';
        
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);

        // 短暂延迟后恢复按钮状态
        setTimeout(restoreButton, 2000);
        
        // 显示成功提示
        showToast('导出调试信息成功，文件将开始下载', 'success');
    }
}

// 显示提示消息的辅助函数
function showToast(message, type = 'info') {
    // 创建 toast 元素
    const toast = document.createElement('div');
    toast.className = `alert alert-${type} alert-dismissible fade show position-fixed`;
    toast.style.cssText = 'top: 20px; right: 20px; z-index: 9999; max-width: 300px;';
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
// Logs Page JavaScript

// Format cells after page loads
document.addEventListener('DOMContentLoaded', function() {
    // Initialize common features from shared.js
    initializeCommonFeatures();
    
    // Format endpoint cells to show only domain with hover for full URL
    document.querySelectorAll('.endpoint-cell').forEach(function(cell) {
        const fullEndpoint = cell.getAttribute('data-endpoint');
        if (fullEndpoint && fullEndpoint !== 'failed') {
            const urlFormatted = formatUrlDisplay(fullEndpoint);
            cell.innerHTML = `<small><code title="${urlFormatted.title}">${urlFormatted.display}</code></small>`;
        } else {
            // For 'failed' or other non-URL values, keep as is
            cell.innerHTML = `<small>${fullEndpoint}</small>`;
        }
    });
});

function toggleFailedOnly(failedOnly, currentPage) {
    failedOnly = !failedOnly;
    window.location.href = `/admin/logs?page=1&failed_only=${failedOnly}`;
}

function refreshLogs(currentPage, failedOnly) {
    window.location.href = `/admin/logs?page=${currentPage}&failed_only=${failedOnly}`;
}

function showLogDetails(requestId) {
    fetch(`/admin/api/logs?request_id=${requestId}`)
        .then(response => response.json())
        .then(data => {
            if (data.logs && data.logs.length > 0) {
                displayMultipleLogDetails(data.logs);
            }
        })
        .catch(error => {
            console.error('Error fetching log details:', error);
        });
}

function displayMultipleLogDetails(logs) {
    const modalBody = document.getElementById('modalBody');
    
    if (logs.length === 1) {
        displayLogDetails(logs[0]);
        return;
    }
    
    let html = `<div class="mb-3">
        <h6>请求详情 - ${logs.length} 次尝试</h6>
        <div class="alert alert-info">
            <strong>请求ID:</strong> ${escapeHtml(logs[0].request_id)}<br>
            <strong>路径:</strong> ${escapeHtml(logs[0].path)}<br>
            <strong>请求方法:</strong> ${escapeHtml(logs[0].method)}<br>
            <strong>总耗时:</strong> ${logs.reduce((sum, log) => sum + log.duration_ms, 0)}ms
        </div>
    </div>`;
    
    logs.forEach((log, index) => {
        html += generateLogAttemptHtml(log, index + 1);
    });
    
    modalBody.innerHTML = html;
    
    // Reinitialize tooltips for dynamic content
    var tooltipTriggerList = [].slice.call(modalBody.querySelectorAll('[title]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });
    
    const modal = new bootstrap.Modal(document.getElementById('logModal'));
    modal.show();
}

function generateLogAttemptHtml(log, attemptNum) {
    const isSuccess = log.status_code >= 200 && log.status_code < 300;
    const badgeClass = isSuccess ? 'bg-success' : 'bg-danger';
    
    // Check if there are data transformations
    const requestChanges = hasRequestChanges(log);
    const responseChanges = hasResponseChanges(log);
    
    // Use actual attempt number from log if available
    const displayAttemptNum = log.attempt_number || attemptNum;
    
    return `
        <div class="card mb-3">
            <div class="card-header">
                <h6 class="mb-0">
                    ${displayAttemptNum > 1 ? `重试 #${displayAttemptNum - 1}` : '首次尝试'}: ${escapeHtml(log.endpoint)} 
                    <span class="badge ${badgeClass}">${log.status_code}</span>
                    <span class="badge bg-secondary">${log.duration_ms}ms</span>
                    ${log.model ? 
                        (log.model_rewrite_applied ? 
                            `<span class="badge bg-success model-rewritten" title="→ ${escapeHtml(log.rewritten_model)}">${escapeHtml(log.model)}</span>` :
                            `<span class="badge bg-primary">${escapeHtml(log.model)}</span>`
                        ) : ''
                    }
                    ${log.is_streaming ? '<span class="badge bg-info">SSE</span>' : ''}
                    ${log.content_type_override ? `<span class="badge bg-warning text-dark" title="Content-Type覆盖: ${escapeHtml(log.content_type_override)}">${escapeHtml(log.content_type_override)}</span>` : ''}
                    ${requestChanges || responseChanges ? '<span class="badge bg-info">有修改</span>' : ''}
                </h6>
            </div>
            <div class="card-body">
                <!-- Request/Response Tabs -->
                <ul class="nav nav-tabs before-after-tabs" id="logTabs${attemptNum}" role="tablist">
                    <li class="nav-item" role="presentation">
                        <button class="nav-link active" id="request-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#request-${attemptNum}" type="button" role="tab">
                            请求数据 ${requestChanges ? '<span class="comparison-badge badge bg-warning">修改</span>' : ''}
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link" id="response-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#response-${attemptNum}" type="button" role="tab">
                            响应数据 ${responseChanges ? '<span class="comparison-badge badge bg-warning">修改</span>' : ''}
                        </button>
                    </li>
                </ul>
                
                <div class="tab-content mt-3" id="logTabsContent${attemptNum}">
                    <!-- Request Tab -->
                    <div class="tab-pane fade show active" id="request-${attemptNum}" role="tabpanel">
                        ${generateRequestComparisonHtml(log, attemptNum)}
                    </div>
                    
                    <!-- Response Tab -->  
                    <div class="tab-pane fade" id="response-${attemptNum}" role="tabpanel">
                        ${generateResponseComparisonHtml(log, attemptNum)}
                    </div>
                </div>
                
                ${log.error ? `<div class="alert alert-danger mt-3"><strong>错误:</strong> ${escapeHtml(log.error)}</div>` : ''}
            </div>
        </div>`;
}

function generateRequestComparisonHtml(log, attemptNum) {
    const hasUrlChanges = log.original_request_url && log.final_request_url && log.original_request_url !== log.final_request_url;
    const hasHeaderChanges = log.original_request_headers && log.final_request_headers && 
                           JSON.stringify(log.original_request_headers) !== JSON.stringify(log.final_request_headers);
    const hasBodyChanges = log.original_request_body && log.final_request_body && log.original_request_body !== log.final_request_body;
    
    let html = '';
    
    // URL comparison (if there are changes)
    if (hasUrlChanges) {
        html += `
            <div class="mb-3">
                <div class="collapsible-header" onclick="toggleCollapsible('urlComparison${attemptNum}')">
                    <span class="collapsible-toggle collapsed">▼</span>
                    <h6 class="mb-0">URL 对比</h6>
                </div>
                <div class="collapsible-content collapsed" id="urlComparison${attemptNum}">
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">客户端原始 URL:</small>
                            <div class="json-pretty" style="max-height: 100px;">${escapeHtml(log.original_request_url || '-')}</div>
                        </div>
                        <div class="col-6">
                            <small class="text-success">发送给上游 URL:</small>
                            <div class="json-pretty" style="max-height: 100px;">${escapeHtml(log.final_request_url || log.original_request_url || '-')}</div>
                        </div>
                    </div>
                </div>
            </div>`;
    }
    
    // Headers comparison
    html += `
        <div class="mb-3">
            <div class="content-section">
                <div class="content-header">
                    <div class="collapsible-header" onclick="toggleCollapsible('requestHeaders${attemptNum}')" style="flex: 1; margin-bottom: 0; border-bottom: none;">
                        <span class="collapsible-toggle collapsed">▼</span>
                        <h6 class="mb-0">请求头对比 ${hasHeaderChanges ? '<span class="badge bg-warning">有修改</span>' : ''}</h6>
                    </div>
                </div>
                <div class="collapsible-content collapsed" id="requestHeaders${attemptNum}">
                    ${hasHeaderChanges ? `
                        <div class="row">
                            <div class="col-6">
                                <small class="text-muted">客户端原始请求头:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.original_request_headers || {}, null, 2))), 
                                    `原始请求头_${log.request_id}_尝试${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.original_request_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                            <div class="col-6">
                                <small class="text-success">发送给上游请求头:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.final_request_headers || log.request_headers || {}, null, 2))), 
                                    `最终请求头_${log.request_id}_尝试${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.final_request_headers || log.request_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                        </div>
                    ` : `
                        ${createContentBoxWithActions(
                            escapeHtml(formatJson(JSON.stringify(log.request_headers || {}, null, 2))), 
                            `请求头_${log.request_id}_尝试${attemptNum}.json`,
                            safeBase64Encode(JSON.stringify(log.request_headers || {}, null, 2)),
                            '300px'
                        )}
                    `}
                </div>
            </div>
        </div>`;
    
    // Body comparison
    html += `
        <div class="content-section">
            <div class="content-header">
                <div class="collapsible-header" onclick="toggleCollapsible('requestBody${attemptNum}')" style="flex: 1; margin-bottom: 0; border-bottom: none;">
                    <span class="collapsible-toggle">▼</span>
                    <h6 class="mb-0">请求体对比 (${log.request_body_size} 字节) ${hasBodyChanges ? '<span class="badge bg-warning">有修改</span>' : ''}</h6>
                </div>
            </div>
            <div class="collapsible-content" id="requestBody${attemptNum}">
                ${hasBodyChanges ? `
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">客户端原始请求体:</small>
                            ${log.original_request_body ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.original_request_body)), 
                                    `原始请求体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.original_request_body)}`,
                                    safeBase64Encode(log.original_request_body),
                                    '400px'
                                ) : 
                                '<div class="text-muted">无请求体</div>'
                            }
                        </div>
                        <div class="col-6">
                            <small class="text-success">发送给上游请求体:</small>
                            ${(log.final_request_body || log.request_body) ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.final_request_body || log.request_body)), 
                                    `最终请求体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.final_request_body || log.request_body)}`,
                                    safeBase64Encode(log.final_request_body || log.request_body),
                                    '400px'
                                ) : 
                                '<div class="text-muted">无请求体</div>'
                            }
                        </div>
                    </div>
                ` : `
                    ${log.request_body ? 
                        createContentBoxWithActions(
                            escapeHtml(formatJson(log.request_body)), 
                            `请求体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.request_body)}`,
                            safeBase64Encode(log.request_body),
                            '400px'
                        ) : 
                        '<div class="text-muted">无请求体</div>'
                    }
                `}
            </div>
        </div>`;
    
    return html;
}

function generateResponseComparisonHtml(log, attemptNum) {
    const hasHeaderChanges = log.original_response_headers && log.final_response_headers && 
                           JSON.stringify(log.original_response_headers) !== JSON.stringify(log.final_response_headers);
    const hasBodyChanges = log.original_response_body && log.final_response_body && log.original_response_body !== log.final_response_body;
    
    let html = '';
    
    // Headers comparison
    html += `
        <div class="mb-3">
            <div class="content-section">
                <div class="content-header">
                    <div class="collapsible-header" onclick="toggleCollapsible('responseHeaders${attemptNum}')" style="flex: 1; margin-bottom: 0; border-bottom: none;">
                        <span class="collapsible-toggle collapsed">▼</span>
                        <h6 class="mb-0">响应头对比 ${hasHeaderChanges ? '<span class="badge bg-warning">有修改</span>' : ''}</h6>
                    </div>
                </div>
                <div class="collapsible-content collapsed" id="responseHeaders${attemptNum}">
                    ${hasHeaderChanges ? `
                        <div class="row">
                            <div class="col-6">
                                <small class="text-muted">上游原始响应头:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.original_response_headers || {}, null, 2))), 
                                    `原始响应头_${log.request_id}_尝试${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.original_response_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                            <div class="col-6">
                                <small class="text-success">发送给客户端响应头:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.final_response_headers || log.response_headers || {}, null, 2))), 
                                    `最终响应头_${log.request_id}_尝试${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.final_response_headers || log.response_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                        </div>
                    ` : `
                        ${createContentBoxWithActions(
                            escapeHtml(formatJson(JSON.stringify(log.response_headers || {}, null, 2))), 
                            `响应头_${log.request_id}_尝试${attemptNum}.json`,
                            safeBase64Encode(JSON.stringify(log.response_headers || {}, null, 2)),
                            '300px'
                        )}
                    `}
                </div>
            </div>
        </div>`;
    
    // Body comparison
    html += `
        <div class="content-section">
            <div class="content-header">
                <div class="collapsible-header" onclick="toggleCollapsible('responseBody${attemptNum}')" style="flex: 1; margin-bottom: 0; border-bottom: none;">
                    <span class="collapsible-toggle">▼</span>
                    <h6 class="mb-0">响应体对比 (${log.response_body_size} 字节) ${hasBodyChanges ? '<span class="badge bg-warning">有修改</span>' : ''}</h6>
                </div>
            </div>
            <div class="collapsible-content" id="responseBody${attemptNum}">
                ${hasBodyChanges ? `
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">上游原始响应体:</small>
                            ${log.original_response_body ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.original_response_body)), 
                                    `原始响应体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.original_response_body)}`,
                                    safeBase64Encode(log.original_response_body),
                                    '400px'
                                ) : 
                                '<div class="text-muted">无响应体</div>'
                            }
                        </div>
                        <div class="col-6">
                            <small class="text-success">发送给客户端响应体:</small>
                            ${(log.final_response_body || log.response_body) ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.final_response_body || log.response_body)), 
                                    `最终响应体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.final_response_body || log.response_body)}`,
                                    safeBase64Encode(log.final_response_body || log.response_body),
                                    '400px'
                                ) : 
                                '<div class="text-muted">无响应体</div>'
                            }
                        </div>
                    </div>
                ` : `
                    ${log.response_body ? 
                        createContentBoxWithActions(
                            escapeHtml(formatJson(log.response_body)), 
                            `响应体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.response_body)}`,
                            safeBase64Encode(log.response_body),
                            '400px'
                        ) : 
                        '<div class="text-muted">无响应体</div>'
                    }
                `}
            </div>
        </div>`;
    
    return html;
}

function hasDataChanges(originalUrl, originalHeaders, originalBody, finalUrl, finalHeaders, finalBody) {
    // Check URL changes
    if (originalUrl && finalUrl && originalUrl !== finalUrl) return true;
    
    // Check headers changes
    if (originalHeaders && finalHeaders) {
        if (JSON.stringify(originalHeaders) !== JSON.stringify(finalHeaders)) return true;
    }
    
    // Check body changes
    if (originalBody && finalBody && originalBody !== finalBody) return true;
    
    return false;
}

function hasRequestChanges(log) {
    const hasUrlChanges = log.original_request_url && log.final_request_url && log.original_request_url !== log.final_request_url;
    const hasHeaderChanges = log.original_request_headers && log.final_request_headers && 
                           JSON.stringify(log.original_request_headers) !== JSON.stringify(log.final_request_headers);
    const hasBodyChanges = log.original_request_body && log.final_request_body && log.original_request_body !== log.final_request_body;
    
    return hasUrlChanges || hasHeaderChanges || hasBodyChanges;
}

function hasResponseChanges(log) {
    const hasHeaderChanges = log.original_response_headers && log.final_response_headers && 
                           JSON.stringify(log.original_response_headers) !== JSON.stringify(log.final_response_headers);
    const hasBodyChanges = log.original_response_body && log.final_response_body && log.original_response_body !== log.final_response_body;
    
    return hasHeaderChanges || hasBodyChanges;
}

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

// 显示清理日志对话框
function showCleanupModal() {
    const modal = new bootstrap.Modal(document.getElementById('cleanupModal'));
    modal.show();
}

// 确认清理日志
function confirmCleanup() {
    const selectedRange = document.querySelector('input[name="cleanupRange"]:checked');
    if (!selectedRange) {
        alert('请选择清理范围');
        return;
    }

    const days = parseInt(selectedRange.value);
    let confirmMessage;
    
    if (days === 0) {
        confirmMessage = '确定要清除所有日志吗？此操作不可撤销！';
    } else {
        confirmMessage = `确定要清除 ${days} 天前的日志吗？此操作不可撤销！`;
    }

    if (!confirm(confirmMessage)) {
        return;
    }

    // 禁用确认按钮，显示加载状态
    const confirmBtn = document.querySelector('#cleanupModal .btn-danger');
    const originalText = confirmBtn.innerHTML;
    confirmBtn.disabled = true;
    confirmBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> 清理中...';

    // 发送清理请求
    fetch('/admin/api/logs/cleanup', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            days: days
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            throw new Error(data.error);
        }
        
        // 显示成功消息
        alert(data.message);
        
        // 关闭对话框
        const modal = bootstrap.Modal.getInstance(document.getElementById('cleanupModal'));
        modal.hide();
        
        // 刷新页面显示最新的日志列表
        window.location.reload();
    })
    .catch(error => {
        console.error('清理日志失败:', error);
        alert('清理日志失败: ' + error.message);
    })
    .finally(() => {
        // 恢复按钮状态
        confirmBtn.disabled = false;
        confirmBtn.innerHTML = originalText;
    });
}
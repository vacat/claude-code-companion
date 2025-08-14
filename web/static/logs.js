// Logs Page JavaScript

// Format cells after page loads
document.addEventListener('DOMContentLoaded', function() {
    // Initialize common features from shared.js
    initializeCommonFeatures();
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
    
    return `
        <div class="card mb-3">
            <div class="card-header">
                <h6 class="mb-0">
                    尝试 ${attemptNum}: ${escapeHtml(log.endpoint)} 
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
                    <div>
                        <button class="btn btn-sm btn-outline-secondary save-file-btn me-1" 
                                data-filename="原始请求头_${log.request_id}_尝试${attemptNum}.json"
                                data-content="${safeBase64Encode(JSON.stringify(log.original_request_headers || {}, null, 2))}"
                                onclick="event.stopPropagation(); saveAsFileFromButton(this)">
                            <i class="fas fa-download"></i> 原始
                        </button>
                        <button class="btn btn-sm btn-outline-secondary save-file-btn" 
                                data-filename="最终请求头_${log.request_id}_尝试${attemptNum}.json"
                                data-content="${safeBase64Encode(JSON.stringify(log.final_request_headers || log.request_headers || {}, null, 2))}"
                                onclick="event.stopPropagation(); saveAsFileFromButton(this)">
                            <i class="fas fa-download"></i> 最终
                        </button>
                    </div>
                </div>
                <div class="collapsible-content collapsed" id="requestHeaders${attemptNum}">
                    ${hasHeaderChanges ? `
                        <div class="row">
                            <div class="col-6">
                                <small class="text-muted">客户端原始请求头:</small>
                                <div class="json-pretty" style="max-height: 300px;">${escapeHtml(formatJson(JSON.stringify(log.original_request_headers || {}, null, 2)))}</div>
                            </div>
                            <div class="col-6">
                                <small class="text-success">发送给上游请求头:</small>
                                <div class="json-pretty" style="max-height: 300px;">${escapeHtml(formatJson(JSON.stringify(log.final_request_headers || log.request_headers || {}, null, 2)))}</div>
                            </div>
                        </div>
                    ` : `
                        <div class="json-pretty" style="max-height: 300px;">${escapeHtml(formatJson(JSON.stringify(log.request_headers || {}, null, 2)))}</div>
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
                <div>
                    <button class="btn btn-sm btn-outline-secondary save-file-btn me-1" 
                            data-filename="原始请求体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.original_request_body)}"
                            data-content="${log.original_request_body ? safeBase64Encode(log.original_request_body) : ''}"
                            onclick="event.stopPropagation(); saveAsFileFromButton(this)" 
                            ${!log.original_request_body ? 'disabled' : ''}>
                        <i class="fas fa-download"></i> 原始
                    </button>
                    <button class="btn btn-sm btn-outline-secondary save-file-btn" 
                            data-filename="最终请求体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.final_request_body || log.request_body)}"
                            data-content="${(log.final_request_body || log.request_body) ? safeBase64Encode(log.final_request_body || log.request_body) : ''}"
                            onclick="event.stopPropagation(); saveAsFileFromButton(this)" 
                            ${!(log.final_request_body || log.request_body) ? 'disabled' : ''}>
                        <i class="fas fa-download"></i> 最终
                    </button>
                </div>
            </div>
            <div class="collapsible-content" id="requestBody${attemptNum}">
                ${hasBodyChanges ? `
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">客户端原始请求体:</small>
                            <div class="json-pretty" style="max-height: 400px;">
                                ${log.original_request_body ? escapeHtml(formatJson(log.original_request_body)) : '无请求体'}
                            </div>
                        </div>
                        <div class="col-6">
                            <small class="text-success">发送给上游请求体:</small>
                            <div class="json-pretty" style="max-height: 400px;">
                                ${(log.final_request_body || log.request_body) ? escapeHtml(formatJson(log.final_request_body || log.request_body)) : '无请求体'}
                            </div>
                        </div>
                    </div>
                ` : `
                    <div class="json-pretty" style="max-height: 400px;">
                        ${log.request_body ? escapeHtml(formatJson(log.request_body)) : '无请求体'}
                    </div>
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
                    <div>
                        <button class="btn btn-sm btn-outline-secondary save-file-btn me-1" 
                                data-filename="原始响应头_${log.request_id}_尝试${attemptNum}.json"
                                data-content="${safeBase64Encode(JSON.stringify(log.original_response_headers || {}, null, 2))}"
                                onclick="event.stopPropagation(); saveAsFileFromButton(this)">
                            <i class="fas fa-download"></i> 原始
                        </button>
                        <button class="btn btn-sm btn-outline-secondary save-file-btn" 
                                data-filename="最终响应头_${log.request_id}_尝试${attemptNum}.json"
                                data-content="${safeBase64Encode(JSON.stringify(log.final_response_headers || log.response_headers || {}, null, 2))}"
                                onclick="event.stopPropagation(); saveAsFileFromButton(this)">
                            <i class="fas fa-download"></i> 最终
                        </button>
                    </div>
                </div>
                <div class="collapsible-content collapsed" id="responseHeaders${attemptNum}">
                    ${hasHeaderChanges ? `
                        <div class="row">
                            <div class="col-6">
                                <small class="text-muted">上游原始响应头:</small>
                                <div class="json-pretty" style="max-height: 300px;">${escapeHtml(formatJson(JSON.stringify(log.original_response_headers || {}, null, 2)))}</div>
                            </div>
                            <div class="col-6">
                                <small class="text-success">发送给客户端响应头:</small>
                                <div class="json-pretty" style="max-height: 300px;">${escapeHtml(formatJson(JSON.stringify(log.final_response_headers || log.response_headers || {}, null, 2)))}</div>
                            </div>
                        </div>
                    ` : `
                        <div class="json-pretty" style="max-height: 300px;">${escapeHtml(formatJson(JSON.stringify(log.response_headers || {}, null, 2)))}</div>
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
                <div>
                    <button class="btn btn-sm btn-outline-secondary save-file-btn me-1" 
                            data-filename="原始响应体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.original_response_body)}"
                            data-content="${log.original_response_body ? safeBase64Encode(log.original_response_body) : ''}"
                            onclick="event.stopPropagation(); saveAsFileFromButton(this)" 
                            ${!log.original_response_body ? 'disabled' : ''}>
                        <i class="fas fa-download"></i> 原始
                    </button>
                    <button class="btn btn-sm btn-outline-secondary save-file-btn" 
                            data-filename="最终响应体_${log.request_id}_尝试${attemptNum}.${getFileExtension(log.final_response_body || log.response_body)}"
                            data-content="${(log.final_response_body || log.response_body) ? safeBase64Encode(log.final_response_body || log.response_body) : ''}"
                            onclick="event.stopPropagation(); saveAsFileFromButton(this)" 
                            ${!(log.final_response_body || log.response_body) ? 'disabled' : ''}>
                        <i class="fas fa-download"></i> 最终
                    </button>
                </div>
            </div>
            <div class="collapsible-content" id="responseBody${attemptNum}">
                ${hasBodyChanges ? `
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">上游原始响应体:</small>
                            <div class="json-pretty" style="max-height: 400px;">
                                ${log.original_response_body ? escapeHtml(formatJson(log.original_response_body)) : '无响应体'}
                            </div>
                        </div>
                        <div class="col-6">
                            <small class="text-success">发送给客户端响应体:</small>
                            <div class="json-pretty" style="max-height: 400px;">
                                ${(log.final_response_body || log.response_body) ? escapeHtml(formatJson(log.final_response_body || log.response_body)) : '无响应体'}
                            </div>
                        </div>
                    </div>
                ` : `
                    <div class="json-pretty" style="max-height: 400px;">
                        ${log.response_body ? escapeHtml(formatJson(log.response_body)) : '无响应体'}
                    </div>
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
// Logs Page Modal Functions

function showLogDetails(requestId) {
    apiRequest(`/admin/api/logs?request_id=${requestId}`)
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
        <div class="d-flex justify-content-between align-items-center">
            <h6>${T('request_details_with_attempts', '请求详情 - {0} 次尝试').replace('{0}', logs.length)}</h6>
            <button class="btn btn-sm btn-outline-success" onclick="exportDebugInfo('${escapeHtml(logs[0].request_id)}')" 
                    ${T('export_debug_info', '导出调试信息')} title="导出调试信息为ZIP文件">
                <i class="fas fa-download"></i> ${T('export_debug_info', '导出调试信息')}
            </button>
        </div>
        <div class="alert alert-info">
            <strong>${T('request_id', '请求ID')}:</strong> ${escapeHtml(logs[0].request_id)}<br>
            <strong>${T('path', '路径')}:</strong> ${escapeHtml(logs[0].path)}<br>
            <strong>${T('request_method', '请求方法')}:</strong> ${escapeHtml(logs[0].method)}<br>
            <strong>${T('total_duration', '总耗时')}:</strong> ${logs.reduce((sum, log) => sum + log.duration_ms, 0)}ms
        </div>
    </div>`;
    
    // Generate tabs for multiple attempts
    html += generateAttemptsTabsHtml(logs);
    
    modalBody.innerHTML = html;
    
    // Reinitialize tooltips for dynamic content
    var tooltipTriggerList = [].slice.call(modalBody.querySelectorAll('[title]'));
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });
    
    const modal = new bootstrap.Modal(document.getElementById('logModal'));
    modal.show();
}

// Generate tabs HTML for multiple attempts
function generateAttemptsTabsHtml(logs) {
    // Generate tab navigation
    let tabsHtml = `<ul class="nav nav-tabs attempts-tabs" id="attemptsTabs" role="tablist">`;
    
    logs.forEach((log, index) => {
        const domain = extractDomain(log.endpoint);
        const isSuccess = log.status_code >= 200 && log.status_code < 300;
        const badgeClass = isSuccess ? 'bg-success' : 'bg-danger';
        const isActive = index === 0 ? ' active' : '';
        
        tabsHtml += `
            <li class="nav-item" role="presentation">
                <button class="nav-link${isActive}" id="attempt-tab-${index}" data-bs-toggle="tab" 
                        data-bs-target="#attempt-${index}" type="button" role="tab">
                    ${domain} <span class="badge ${badgeClass}">${log.status_code}</span> <span class="text-muted">${log.duration_ms}ms</span>
                </button>
            </li>`;
    });
    
    tabsHtml += `</ul>`;
    
    // Generate tab content
    let contentHtml = `<div class="tab-content mt-3" id="attemptsTabsContent">`;
    
    logs.forEach((log, index) => {
        const isActive = index === 0 ? ' show active' : '';
        const displayAttemptNum = log.attempt_number || (index + 1);
        
        contentHtml += `
            <div class="tab-pane fade${isActive}" id="attempt-${index}" role="tabpanel">
                ${generateLogAttemptContentHtml(log, displayAttemptNum)}
            </div>`;
    });
    
    contentHtml += `</div>`;
    
    return tabsHtml + contentHtml;
}

// Extract domain from endpoint URL
function extractDomain(endpoint) {
    try {
        const url = new URL(endpoint);
        return url.hostname;
    } catch (e) {
        // If it's not a full URL, try to extract domain-like part
        const parts = endpoint.split('/');
        return parts[0] || endpoint;
    }
}
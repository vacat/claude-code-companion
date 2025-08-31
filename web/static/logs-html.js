// Logs Page HTML Generation Functions

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
                    ${displayAttemptNum > 1 ? `<span data-t="retry_number">重试</span> #${displayAttemptNum - 1}` : `<span data-t="first_attempt">首次尝试</span>`}: ${escapeHtml(log.endpoint)} 
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
                    ${requestChanges || responseChanges ? `<span class="badge bg-info">${T('has_modifications', '有修改')}</span>` : ''}
                </h6>
            </div>
            <div class="card-body">
                ${log.error ? `<div class="alert alert-danger mb-3"><strong data-t="error">${T('error', '错误')}:</strong> ${escapeHtml(log.error)}</div>` : ''}
                <!-- Request/Response Tabs -->
                <ul class="nav nav-tabs before-after-tabs" id="logTabs${attemptNum}" role="tablist">
                    <li class="nav-item" role="presentation">
                        <button class="nav-link active" id="request-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#request-${attemptNum}" type="button" role="tab">
                            <span data-t="request_data">请求数据</span> ${requestChanges ? `<span class="comparison-badge badge bg-warning" data-t="modified">修改</span>` : ''}
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link" id="response-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#response-${attemptNum}" type="button" role="tab">
                            <span data-t="response_data">响应数据</span> ${responseChanges ? `<span class="comparison-badge badge bg-warning" data-t="modified">修改</span>` : ''}
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
            </div>
        </div>`;
}

// Generate content for log attempt in tab (without card wrapper)
function generateLogAttemptContentHtml(log, attemptNum) {
    const isSuccess = log.status_code >= 200 && log.status_code < 300;
    const badgeClass = isSuccess ? 'bg-success' : 'bg-danger';
    
    // Check if there are data transformations
    const requestChanges = hasRequestChanges(log);
    const responseChanges = hasResponseChanges(log);
    
    // Use actual attempt number from log if available
    const displayAttemptNum = log.attempt_number || attemptNum;
    
    return `
        ${log.error ? `<div class="alert alert-danger mb-3"><strong data-t="error">${T('error', '错误')}:</strong> ${escapeHtml(log.error)}</div>` : ''}
        
        <div class="mb-3">
            <h6 class="mb-2">
                ${displayAttemptNum > 1 ? T('retry_attempt', '重试 #{0}').replace('{0}', displayAttemptNum - 1) : T('first_attempt', '首次尝试')}: ${escapeHtml(log.endpoint)} 
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
                ${requestChanges || responseChanges ? `<span class="badge bg-info">${T('has_modifications', '有修改')}</span>` : ''}
            </h6>
        </div>
        
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
        </div>`;
}
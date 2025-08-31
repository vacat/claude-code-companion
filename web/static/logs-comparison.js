// Logs Page Request/Response Comparison Functions

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
                    <h6 class="mb-0">${T('url_comparison', 'URL 对比')}</h6>
                </div>
                <div class="collapsible-content collapsed" id="urlComparison${attemptNum}">
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">${T('client_original_url', '客户端原始 URL')}:</small>
                            <div class="json-pretty" style="max-height: 100px;">${escapeHtml(log.original_request_url || '-')}</div>
                        </div>
                        <div class="col-6">
                            <small class="text-success">${T('sent_to_upstream_url', '发送给上游 URL')}:</small>
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
                        <h6 class="mb-0">${T('request_headers_comparison', '请求头对比')} ${hasHeaderChanges ? `<span class="badge bg-warning">${T('modified', '有修改')}</span>` : ''}</h6>
                    </div>
                </div>
                <div class="collapsible-content collapsed" id="requestHeaders${attemptNum}">
                    ${hasHeaderChanges ? `
                        <div class="row">
                            <div class="col-6">
                                <small class="text-muted">${T('client_original_request_headers', '客户端原始请求头')}:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.original_request_headers || {}, null, 2))), 
                                    `${T('original_request_headers', '原始请求头')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.original_request_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                            <div class="col-6">
                                <small class="text-success">${T('sent_to_upstream_request_headers', '发送给上游请求头')}:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.final_request_headers || log.request_headers || {}, null, 2))), 
                                    `${T('final_request_headers', '最终请求头')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.final_request_headers || log.request_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                        </div>
                    ` : `
                        ${createContentBoxWithActions(
                            escapeHtml(formatJson(JSON.stringify(log.request_headers || {}, null, 2))), 
                            `${T('request_headers', '请求头')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.json`,
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
                    <h6 class="mb-0">${T('request_body_comparison', '请求体对比')} (${log.request_body_size} ${T('bytes', '字节')}) ${hasBodyChanges ? `<span class="badge bg-warning">${T('modified', '有修改')}</span>` : ''}</h6>
                </div>
                ${isRequestBodyAnthropicRequest(log.original_request_body || log.request_body) ? `
                <button class="btn btn-outline-primary btn-sm ms-2 inspector-main-btn" 
                        data-request-body="${safeBase64Encode(log.original_request_body || log.request_body)}"
                        onclick="openRequestInspectorFromMain(this)"
                        title="${T('open_anthropic_request_inspector', '打开 Anthropic 请求检查器')}">
                    ${T('analyze_request', '🔍 分析请求')}
                </button>
                ` : ''}
            </div>
            <div class="collapsible-content" id="requestBody${attemptNum}">
                ${hasBodyChanges ? `
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">${T('client_original_request_body', '客户端原始请求体')}:</small>
                            ${log.original_request_body ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.original_request_body)), 
                                    `${T('original_request_body', '原始请求体')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.${getFileExtension(log.original_request_body)}`,
                                    safeBase64Encode(log.original_request_body),
                                    '400px'
                                ) : 
                                `<div class="text-muted">${T('no_request_body', '无请求体')}</div>`
                            }
                        </div>
                        <div class="col-6">
                            <small class="text-success">${T('sent_to_upstream_request_body', '发送给上游请求体')}:</small>
                            ${(log.final_request_body || log.request_body) ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.final_request_body || log.request_body)), 
                                    `${T('final_request_body', '最终请求体')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.${getFileExtension(log.final_request_body || log.request_body)}`,
                                    safeBase64Encode(log.final_request_body || log.request_body),
                                    '400px'
                                ) : 
                                `<div class="text-muted">${T('no_request_body', '无请求体')}</div>`
                            }
                        </div>
                    </div>
                ` : `
                    ${log.request_body ? 
                        createContentBoxWithActions(
                            escapeHtml(formatJson(log.request_body)), 
                            `${T('request_body', '请求体')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.${getFileExtension(log.request_body)}`,
                            safeBase64Encode(log.request_body),
                            '400px'
                        ) : 
                        `<div class="text-muted">${T('no_request_body', '无请求体')}</div>`
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
                        <h6 class="mb-0">${T('response_headers_comparison', '响应头对比')} ${hasHeaderChanges ? `<span class="badge bg-warning">${T('modified', '有修改')}</span>` : ''}</h6>
                    </div>
                </div>
                <div class="collapsible-content collapsed" id="responseHeaders${attemptNum}">
                    ${hasHeaderChanges ? `
                        <div class="row">
                            <div class="col-6">
                                <small class="text-muted">${T('upstream_original_response_headers', '上游原始响应头')}:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.original_response_headers || {}, null, 2))), 
                                    `${T('original_response_headers', '原始响应头')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.original_response_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                            <div class="col-6">
                                <small class="text-success">${T('sent_to_client_response_headers', '发送给客户端响应头')}:</small>
                                ${createContentBoxWithActions(
                                    escapeHtml(formatJson(JSON.stringify(log.final_response_headers || log.response_headers || {}, null, 2))), 
                                    `${T('final_response_headers', '最终响应头')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.json`,
                                    safeBase64Encode(JSON.stringify(log.final_response_headers || log.response_headers || {}, null, 2)),
                                    '300px'
                                )}
                            </div>
                        </div>
                    ` : `
                        ${createContentBoxWithActions(
                            escapeHtml(formatJson(JSON.stringify(log.response_headers || {}, null, 2))), 
                            `${T('response_headers', '响应头')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.json`,
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
                    <h6 class="mb-0">${T('response_body_comparison', '响应体对比')} (${log.response_body_size} ${T('bytes', '字节')}) ${hasBodyChanges ? `<span class="badge bg-warning">${T('modified', '有修改')}</span>` : ''}</h6>
                </div>
                ${(isAnthropicResponse(log.final_response_body || log.response_body || log.original_response_body) && !hasSSEFormatError(log)) ? `
                <button class="inspect-response-btn btn btn-outline-success btn-sm ms-2" 
                        data-response-body="${safeBase64Encode(log.final_response_body || log.response_body || log.original_response_body)}"
                        data-is-streaming="${log.is_streaming || false}"
                        data-final-response="${safeBase64Encode(log.final_response_body || '')}"
                        onclick="openResponseInspector(this)"
                        title="${T('check_anthropic_response', '检查 Anthropic 响应')}">
                    ${T('check_response', '🔍 检查响应')}
                </button>
                ` : ''}
            </div>
            <div class="collapsible-content" id="responseBody${attemptNum}">
                ${hasBodyChanges ? `
                    <div class="row">
                        <div class="col-6">
                            <small class="text-muted">${T('upstream_original_response_body', '上游原始响应体')}:</small>
                            ${log.original_response_body ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.original_response_body)), 
                                    `${T('original_response_body', '原始响应体')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.${getFileExtension(log.original_response_body)}`,
                                    safeBase64Encode(log.original_response_body),
                                    '400px'
                                ) : 
                                `<div class="text-muted">${T('no_response_body', '无响应体')}</div>`
                            }
                        </div>
                        <div class="col-6">
                            <small class="text-success">${T('sent_to_client_response_body', '发送给客户端响应体')}:</small>
                            ${(log.final_response_body || log.response_body) ? 
                                createContentBoxWithActions(
                                    escapeHtml(formatJson(log.final_response_body || log.response_body)), 
                                    `${T('final_response_body', '最终响应体')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.${getFileExtension(log.final_response_body || log.response_body)}`,
                                    safeBase64Encode(log.final_response_body || log.response_body),
                                    '400px'
                                ) : 
                                `<div class="text-muted">${T('no_response_body', '无响应体')}</div>`
                            }
                        </div>
                    </div>
                ` : `
                    ${log.response_body ? 
                        createContentBoxWithActions(
                            escapeHtml(formatJson(log.response_body)), 
                            `${T('response_body', '响应体')}_${log.request_id}_${T('attempt', '尝试')}${attemptNum}.${getFileExtension(log.response_body)}`,
                            safeBase64Encode(log.response_body),
                            '400px'
                        ) : 
                        `<div class="text-muted">${T('no_response_body', '无响应体')}</div>`
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
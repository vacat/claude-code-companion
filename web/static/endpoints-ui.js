// Endpoints UI JavaScript - UI 相关功能

// Truncate path, show ... if exceeds specified length
function truncatePath(path, maxLength = 10) {
    if (!path || path.length <= maxLength) {
        return path;
    }
    return path.substring(0, maxLength) + '...';
}

function rebuildTable(endpoints) {
    const specialTbody = document.getElementById('special-endpoint-list');
    const generalTbody = document.getElementById('general-endpoint-list');
    const specialSection = document.getElementById('special-endpoints-section');
    
    // Clear existing content
    specialTbody.innerHTML = '';
    generalTbody.innerHTML = '';
    
    // Separate tagged and untagged endpoints
    const specialEndpoints = endpoints.filter(endpoint => endpoint.tags && endpoint.tags.length > 0);
    const generalEndpoints = endpoints.filter(endpoint => !endpoint.tags || endpoint.tags.length === 0);
    
    // Show/hide special endpoint section
    if (specialEndpoints.length > 0) {
        StyleUtils.show(specialSection);
    } else {
        StyleUtils.hide(specialSection);
    }
    
    // Function to create endpoint row
    function createEndpointRow(endpoint, index) {
        const row = document.createElement('tr');
        row.className = 'endpoint-row';
        row.setAttribute('data-endpoint-name', escapeHtml(endpoint.name));
        
        // Build status badge - 三种状态：禁用（灰色）、正常（绿色）、不可用（红色）
        let statusBadge = '';
        if (!endpoint.enabled) {
            // 如果端点被禁用，显示灰色的"禁用"状态
            statusBadge = `<span class="badge bg-secondary"><i class="fas fa-ban"></i> <span data-t="disabled">禁用</span></span>`;
        } else if (endpoint.status === 'active') {
            // 如果端点已启用且状态为活跃，显示绿色的"正常"状态
            statusBadge = `<span class="badge bg-success"><i class="fas fa-check-circle"></i> <span data-t="normal">正常</span></span>`;
        } else if (endpoint.status === 'inactive') {
            // 如果端点已启用但状态为不活跃，显示红色的"不可用"状态
            statusBadge = `<span class="badge bg-danger"><i class="fas fa-times-circle"></i> <span data-t="unavailable">不可用</span></span>`;
        } else {
            // 其他状态（如检测中）
            statusBadge = `<span class="badge bg-warning"><i class="fas fa-clock"></i> <span data-t="detecting">检测中</span></span>`;
        }
        
        // Build enabled status badge
        const enabledBadge = endpoint.enabled 
            ? `<span class="badge bg-success"><i class="fas fa-toggle-on"></i> <span data-t="enabled">已启用</span></span>`
            : `<span class="badge bg-secondary"><i class="fas fa-toggle-off"></i> <span data-t="disabled">已禁用</span></span>`;
        
        // Build endpoint type badge
        const endpointTypeBadge = endpoint.endpoint_type === 'openai' 
            ? '<span class="badge bg-warning">openai</span>'
            : '<span class="badge bg-primary">anthropic</span>';
        
        // Build URL display: only show domain, full URL in title, truncate domain if over 25 chars
        const urlFormatted = formatUrlDisplay(endpoint.url);
        const urlDisplay = `<code class="url-display" title="${urlFormatted.title}">${urlFormatted.display}</code>`;
        
        // Build path display: truncate if over 10 characters
        let pathDisplay;
        if (endpoint.endpoint_type === 'openai') {
            const fullPath = endpoint.path_prefix || '';
            const truncatedPath = truncatePath(fullPath, 10);
            pathDisplay = `<code class="path-display" title="${fullPath}">${truncatedPath}</code>`;
        } else {
            pathDisplay = '<span class="text-muted">/v1/messages</span>';
        }
        
        // Build auth type badge
        let authTypeBadge;
        if (endpoint.auth_type === 'api_key') {
            authTypeBadge = '<span class="badge bg-primary">api_key</span>';
        } else if (endpoint.auth_type === 'oauth') {
            authTypeBadge = '<span class="badge bg-success">oauth</span>';
        } else {
            authTypeBadge = '<span class="badge bg-secondary">auth_token</span>';
        }
        
        // Build proxy status display
        let proxyDisplay = '';
        if (endpoint.proxy && endpoint.proxy.type && endpoint.proxy.address) {
            const proxyType = endpoint.proxy.type.toUpperCase();
            const hasAuth = endpoint.proxy.username ? ' 🔐' : '';
            proxyDisplay = `<span class="badge bg-warning" title="${t('proxy_with_auth', '代理')}: ${endpoint.proxy.type}://${endpoint.proxy.address}">${proxyType}${hasAuth}</span>`;
        } else {
            proxyDisplay = `<span class="text-muted" data-t="no_proxy">无</span>`;
        }
        
        // Build tags display
        let tagsDisplay = '';
        if (endpoint.tags && endpoint.tags.length > 0) {
            tagsDisplay = endpoint.tags.map(tag => `<span class="badge bg-info me-1 mb-1">${escapeHtml(tag)}</span>`).join('');
        } else {
            tagsDisplay = `<span class="text-muted" data-t="general">通用</span>`;
        }
        
        row.innerHTML = `
            <td class="drag-handle text-center">
                <i class="fas fa-arrows-alt text-muted"></i>
            </td>
            <td>
                <span class="badge bg-info priority-badge">${endpoint.priority}</span>
            </td>
            <td><strong>${escapeHtml(endpoint.name)}</strong></td>
            <td>${urlDisplay}</td>
            <td>${endpointTypeBadge}</td>
            <td>${pathDisplay}</td>
            <td>${authTypeBadge}</td>
            <td>${proxyDisplay}</td>
            <td>${tagsDisplay}</td>
            <td>${statusBadge}</td>
            <td>${enabledBadge}</td>
            <td class="action-buttons">
                <div class="btn-group btn-group-sm" role="group">
                    <button class="btn ${endpoint.enabled ? 'btn-success' : 'btn-secondary'} btn-sm" 
                            onclick="event.stopPropagation(); toggleEndpointEnabled('${escapeHtml(endpoint.name)}', ${endpoint.enabled})"
                            title="${endpoint.enabled ? '点击禁用' : '点击启用'}">
                        <i class="fas ${endpoint.enabled ? 'fa-toggle-on' : 'fa-toggle-off'}"></i>
                    </button>
                    <button class="btn btn-outline-primary btn-sm" 
                            onclick="event.stopPropagation(); showEditEndpointModal('${escapeHtml(endpoint.name)}')"
                            title="编辑">
                        <i class="fas fa-edit"></i>
                    </button>
                    <button class="btn btn-outline-info btn-sm" 
                            onclick="event.stopPropagation(); copyEndpoint('${escapeHtml(endpoint.name)}')"
                            title="复制">
                        <i class="fas fa-copy"></i>
                    </button>
                    <button class="btn btn-outline-warning btn-sm" 
                            onclick="event.stopPropagation(); resetEndpointStatus('${escapeHtml(endpoint.name)}')"
                            title="重置状态">
                        <i class="fas fa-redo"></i>
                    </button>
                    <button class="btn btn-outline-danger btn-sm" 
                            onclick="event.stopPropagation(); deleteEndpoint('${escapeHtml(endpoint.name)}')"
                            title="删除">
                        <i class="fas fa-trash"></i>
                    </button>
                </div>
            </td>
        `;
        
        return row;
    }
    
    // Add special endpoints
    specialEndpoints.forEach((endpoint, index) => {
        const row = createEndpointRow(endpoint, index);
        specialTbody.appendChild(row);
    });
    
    // Add general endpoints
    generalEndpoints.forEach((endpoint, index) => {
        const row = createEndpointRow(endpoint, specialEndpoints.length + index);
        generalTbody.appendChild(row);
    });
    
    // Reinitialize drag-and-drop sorting
    initializeSortable();
}

function updateEndpointToggleButton(endpointName, enabled) {
    // Try to find in special endpoint list first
    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    if (!row) {
        // If not found, search in general endpoint list
        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    }
    
    if (row) {
        const toggleButton = row.querySelector('.btn-group button:first-child');
        if (toggleButton) {
            // Update button class
            toggleButton.className = `btn ${enabled ? 'btn-success' : 'btn-secondary'} btn-sm`;
            // Update button icon
            const icon = toggleButton.querySelector('i');
            icon.className = `fas ${enabled ? 'fa-toggle-on' : 'fa-toggle-off'}`;
            // Update button title
            toggleButton.title = enabled ? '点击禁用' : '点击启用';
            // Update button onclick
            toggleButton.onclick = function(event) {
                event.stopPropagation();
                toggleEndpointEnabled(endpointName, enabled);
            };
        }
    }
}

function updateEndpointEnabledBadge(endpointName, enabled) {
    // Try to find in special endpoint list first
    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    if (!row) {
        // If not found, search in general endpoint list
        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    }
    
    if (row) {
        const enabledCell = row.children[10]; // The "启用" column is at index 10
        const enabledBadge = enabled 
            ? '<span class="badge bg-success"><i class="fas fa-toggle-on"></i> 已启用</span>'
            : '<span class="badge bg-secondary"><i class="fas fa-toggle-off"></i> 已禁用</span>';
        enabledCell.innerHTML = enabledBadge;
    }
}

function updateEndpointStatusBadge(endpointName, enabled, status) {
    // Try to find in special endpoint list first
    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    if (!row) {
        // If not found, search in general endpoint list
        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    }
    
    if (row) {
        const statusCell = row.children[9]; // The "状态" column is at index 9
        let statusBadge = '';
        if (!enabled) {
            // 如果端点被禁用，显示灰色的"禁用"状态
            statusBadge = '<span class="badge bg-secondary"><i class="fas fa-ban"></i> 禁用</span>';
        } else if (status === 'active') {
            // 如果端点已启用且状态为活跃，显示绿色的"正常"状态
            statusBadge = '<span class="badge bg-success"><i class="fas fa-check-circle"></i> 正常</span>';
        } else if (status === 'inactive') {
            // 如果端点已启用但状态为不活跃，显示红色的"不可用"状态
            statusBadge = '<span class="badge bg-danger"><i class="fas fa-times-circle"></i> 不可用</span>';
        } else {
            // 其他状态（如检测中）
            statusBadge = '<span class="badge bg-warning"><i class="fas fa-clock"></i> 检测中</span>';
        }
        statusCell.innerHTML = statusBadge;
    }
}
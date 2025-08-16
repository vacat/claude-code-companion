// Endpoints Page JavaScript

let currentEndpoints = [];
let editingEndpointName = null;
let endpointModal = null;
let originalAuthValue = '';
let isAuthVisible = false;

let specialSortableInstance = null;
let generalSortableInstance = null;

document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    endpointModal = new bootstrap.Modal(document.getElementById('endpointModal'));
    loadEndpoints();
    
    // Auto-refresh every 30 seconds (only status updates)
    setInterval(refreshEndpointStatus, 30000);
});

function initializeSortable() {
    // Destroy existing sortable instances
    if (specialSortableInstance) {
        specialSortableInstance.destroy();
        specialSortableInstance = null;
    }
    if (generalSortableInstance) {
        generalSortableInstance.destroy();
        generalSortableInstance = null;
    }

    // Initialize special endpoint list drag-and-drop sorting
    const specialTbody = document.getElementById('special-endpoint-list');
    if (specialTbody && specialTbody.children.length > 0) {
        specialSortableInstance = new Sortable(specialTbody, {
            animation: 150,
            ghostClass: 'sortable-ghost',
            chosenClass: 'sortable-chosen',
            dragClass: 'sortable-drag',
            group: 'special-endpoints', // Restrict to special endpoint group
            onStart: function(evt) {
                document.body.style.cursor = 'grabbing';
            },
            onEnd: function (evt) {
                document.body.style.cursor = '';
                reorderEndpoints();
            }
        });
    }
    
    // Initialize general endpoint list drag-and-drop sorting
    const generalTbody = document.getElementById('general-endpoint-list');
    if (generalTbody && generalTbody.children.length > 0) {
        generalSortableInstance = new Sortable(generalTbody, {
            animation: 150,
            ghostClass: 'sortable-ghost',
            chosenClass: 'sortable-chosen',
            dragClass: 'sortable-drag',
            group: 'general-endpoints', // Restrict to general endpoint group
            onStart: function(evt) {
                document.body.style.cursor = 'grabbing';
            },
            onEnd: function (evt) {
                document.body.style.cursor = '';
                reorderEndpoints();
            }
        });
    }
}

function loadEndpoints() {
    fetch('/admin/api/endpoints')
        .then(response => response.json())
        .then(data => {
            currentEndpoints = data.endpoints;
            rebuildTable(currentEndpoints);
        })
        .catch(error => {
            console.error('Failed to load endpoints:', error);
            showAlert('Failed to load endpoints', 'danger');
        });
}

function refreshEndpointStatus() {
    fetch('/admin/api/endpoints')
        .then(response => response.json())
        .then(data => {
            // Only update status and statistics, not the full table
            data.endpoints.forEach(endpoint => {
                // Try to find in special endpoint list
                let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpoint.name}"]`);
                if (!row) {
                    // If not found, search in general endpoint list
                    row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpoint.name}"]`);
                }
                if (row) {
                    updateEndpointRowStatus(row, endpoint);
                }
            });
        })
        .catch(error => console.error('Failed to refresh endpoint status:', error));
}

function updateEndpointRowStatus(row, endpoint) {
    const statusCell = row.children[9]; // Adjust index: added proxy column, was 8, now 9
    
    // Update status
    let statusBadge = '';
    if (endpoint.status === 'active') {
        statusBadge = '<span class="badge bg-success"><i class="fas fa-check-circle"></i> 活跃</span>';
    } else if (endpoint.status === 'inactive') {
        statusBadge = '<span class="badge bg-danger"><i class="fas fa-times-circle"></i> 不可用</span>';
    } else {
        statusBadge = '<span class="badge bg-warning"><i class="fas fa-clock"></i> 检测中</span>';
    }
    statusCell.innerHTML = statusBadge;
}

function refreshTable() {
    // Reload endpoint data instead of refreshing the entire page
    loadEndpoints();
}

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
        specialSection.style.display = 'block';
    } else {
        specialSection.style.display = 'none';
    }
    
    // Function to create endpoint row
    function createEndpointRow(endpoint, index) {
        const row = document.createElement('tr');
        row.className = 'endpoint-row';
        row.setAttribute('data-endpoint-name', endpoint.name);
        
        // Build status badge
        let statusBadge = '';
        if (endpoint.status === 'active') {
            statusBadge = '<span class="badge bg-success"><i class="fas fa-check-circle"></i> 活跃</span>';
        } else if (endpoint.status === 'inactive') {
            statusBadge = '<span class="badge bg-danger"><i class="fas fa-times-circle"></i> 不可用</span>';
        } else {
            statusBadge = '<span class="badge bg-warning"><i class="fas fa-clock"></i> 检测中</span>';
        }
        
        // Build enabled status badge
        const enabledBadge = endpoint.enabled 
            ? '<span class="badge bg-success"><i class="fas fa-toggle-on"></i> 已启用</span>'
            : '<span class="badge bg-secondary"><i class="fas fa-toggle-off"></i> 已禁用</span>';
        
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
            proxyDisplay = `<span class="badge bg-warning" title="代理: ${endpoint.proxy.type}://${endpoint.proxy.address}">${proxyType}${hasAuth}</span>`;
        } else {
            proxyDisplay = '<span class="text-muted">无</span>';
        }
        
        // Build tags display
        let tagsDisplay = '';
        if (endpoint.tags && endpoint.tags.length > 0) {
            tagsDisplay = endpoint.tags.map(tag => `<span class="badge bg-info me-1 mb-1">${tag}</span>`).join('');
        } else {
            tagsDisplay = '<span class="text-muted">通用</span>';
        }
        
        row.innerHTML = `
            <td class="drag-handle text-center">
                <i class="fas fa-arrows-alt text-muted"></i>
            </td>
            <td>
                <span class="badge bg-info priority-badge">${endpoint.priority}</span>
            </td>
            <td><strong>${endpoint.name}</strong></td>
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
                            onclick="event.stopPropagation(); toggleEndpointEnabled('${endpoint.name}', ${endpoint.enabled})"
                            title="${endpoint.enabled ? '点击禁用' : '点击启用'}">
                        <i class="fas ${endpoint.enabled ? 'fa-toggle-on' : 'fa-toggle-off'}"></i>
                    </button>
                    <button class="btn btn-outline-primary btn-sm" 
                            onclick="event.stopPropagation(); showEditEndpointModal('${endpoint.name}')"
                            title="编辑">
                        <i class="fas fa-edit"></i>
                    </button>
                    <button class="btn btn-outline-info btn-sm" 
                            onclick="event.stopPropagation(); copyEndpoint('${endpoint.name}')"
                            title="复制">
                        <i class="fas fa-copy"></i>
                    </button>
                    <button class="btn btn-outline-danger btn-sm" 
                            onclick="event.stopPropagation(); deleteEndpoint('${endpoint.name}')"
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

function showAddEndpointModal() {
    editingEndpointName = null;
    originalAuthValue = '';
    isAuthVisible = false;
    
    document.getElementById('endpointModalTitle').textContent = '添加端点';
    document.getElementById('endpointForm').reset();
    document.getElementById('endpoint-enabled').checked = true;
    document.getElementById('endpoint-type').value = 'anthropic'; // Default to Anthropic
    document.getElementById('endpoint-tags').value = ''; // Clear tags field
    
    // Set endpoint type and switch path prefix display
    onEndpointTypeChange();
    
    // Reset auth visibility
    resetAuthVisibility();
    
    // Clear proxy configuration
    loadProxyConfig(null);
    
    // Clear model rewrite configuration
    loadModelRewriteConfig(null);
    
    // Reset to basic configuration tab
    resetModalTabs();
    
    endpointModal.show();
}

function showEditEndpointModal(endpointName) {
    const endpoint = currentEndpoints.find(ep => ep.name === endpointName);
    if (!endpoint) {
        showAlert('端点未找到', 'danger');
        return;
    }

    editingEndpointName = endpointName;
    originalAuthValue = endpoint.auth_value;
    isAuthVisible = false;
    
    document.getElementById('endpointModalTitle').textContent = '编辑端点';
    
    // Populate form
    document.getElementById('endpoint-name').value = endpoint.name;
    document.getElementById('endpoint-url').value = endpoint.url;
    document.getElementById('endpoint-type').value = endpoint.endpoint_type || 'anthropic';
    document.getElementById('endpoint-path-prefix').value = endpoint.path_prefix || '';
    document.getElementById('endpoint-enabled').checked = endpoint.enabled;
    
    // Set endpoint type and switch path prefix display first
    onEndpointTypeChange();
    
    // Then set the auth type after the options are populated
    document.getElementById('endpoint-auth-type').value = endpoint.auth_type;
    
    // Set tags field
    const tagsValue = endpoint.tags && endpoint.tags.length > 0 ? endpoint.tags.join(', ') : '';
    document.getElementById('endpoint-tags').value = tagsValue;
    
    // Set auth value or OAuth config based on auth type
    if (endpoint.auth_type === 'oauth' && endpoint.oauth_config) {
        // Load OAuth configuration
        loadOAuthConfig(endpoint.oauth_config);
    } else {
        // Set auth value to asterisks
        document.getElementById('endpoint-auth-value').value = '*'.repeat(Math.min(endpoint.auth_value.length, 50));
        document.getElementById('endpoint-auth-value').type = 'password'; // Ensure it's password type
        document.getElementById('endpoint-auth-value').placeholder = '输入您的 API Key 或 Token';
        resetAuthVisibility();
    }
    
    // Update auth type display
    onAuthTypeChange();
    
    // Load proxy configuration
    loadProxyConfig(endpoint.proxy);
    
    // Load model rewrite configuration
    loadModelRewriteConfig(endpoint.model_rewrite);
    
    // Reset to basic configuration tab
    resetModalTabs();
    
    endpointModal.show();
}

// Reset modal tabs to basic configuration
function resetModalTabs() {
    // Reset tab state
    const basicTab = document.getElementById('basic-tab');
    const advancedTab = document.getElementById('advanced-tab');
    const basicPane = document.getElementById('basic-tab-pane');
    const advancedPane = document.getElementById('advanced-tab-pane');
    
    // Activate basic configuration tab
    basicTab.classList.add('active');
    basicTab.setAttribute('aria-selected', 'true');
    basicPane.classList.add('show', 'active');
    
    // Deactivate advanced configuration tab
    advancedTab.classList.remove('active');
    advancedTab.setAttribute('aria-selected', 'false');
    advancedPane.classList.remove('show', 'active');
}

function toggleAuthVisibility() {
    const authValueField = document.getElementById('endpoint-auth-value');
    const eyeIcon = document.getElementById('auth-eye-icon');
    
    if (isAuthVisible) {
        // Hide: show asterisks
        if (originalAuthValue) {
            authValueField.value = '*'.repeat(Math.min(originalAuthValue.length, 50));
        }
        authValueField.type = 'password'; // Set to password type
        eyeIcon.className = 'fas fa-eye';
        isAuthVisible = false;
    } else {
        // Show: show real value
        authValueField.value = originalAuthValue;
        authValueField.type = 'text'; // Set to text type
        eyeIcon.className = 'fas fa-eye-slash';
        isAuthVisible = true;
    }
}

function toggleOAuthVisibility(inputId, iconId) {
    const inputField = document.getElementById(inputId);
    const eyeIcon = document.getElementById(iconId);
    
    if (inputField.type === 'password') {
        inputField.type = 'text';
        eyeIcon.className = 'fas fa-eye-slash';
    } else {
        inputField.type = 'password';
        eyeIcon.className = 'fas fa-eye';
    }
}

function loadOAuthConfig(oauthConfig) {
    if (!oauthConfig) {
        // Clear OAuth fields
        document.getElementById('oauth-access-token').value = '';
        document.getElementById('oauth-refresh-token').value = '';
        document.getElementById('oauth-expires-at').value = '';
        document.getElementById('oauth-token-url').value = '';
        document.getElementById('oauth-client-id').value = '';
        document.getElementById('oauth-scopes').value = '';
        document.getElementById('oauth-auto-refresh').checked = true;
        return;
    }
    
    // Load OAuth configuration
    document.getElementById('oauth-access-token').value = oauthConfig.access_token || '';
    document.getElementById('oauth-refresh-token').value = oauthConfig.refresh_token || '';
    document.getElementById('oauth-expires-at').value = oauthConfig.expires_at || '';
    document.getElementById('oauth-token-url').value = oauthConfig.token_url || '';
    document.getElementById('oauth-client-id').value = oauthConfig.client_id || '';
    document.getElementById('oauth-auto-refresh').checked = oauthConfig.auto_refresh !== false;
    
    // Load scopes
    if (oauthConfig.scopes && Array.isArray(oauthConfig.scopes)) {
        document.getElementById('oauth-scopes').value = oauthConfig.scopes.join(', ');
    } else {
        document.getElementById('oauth-scopes').value = '';
    }
}

function resetAuthVisibility() {
    const eyeIcon = document.getElementById('auth-eye-icon');
    eyeIcon.className = 'fas fa-eye';
    isAuthVisible = false;
}

function saveEndpoint() {
    const form = document.getElementById('endpointForm');
    if (!form.checkValidity()) {
        form.reportValidity();
        return;
    }

    const authType = document.getElementById('endpoint-auth-type').value;
    
    // Get auth value or OAuth config based on auth type
    let authValue = '';
    let oauthConfig = null;
    
    if (authType === 'oauth') {
        // Collect OAuth configuration
        const scopesInput = document.getElementById('oauth-scopes').value.trim();
        const scopes = scopesInput ? scopesInput.split(',').map(s => s.trim()).filter(s => s) : [];
        
        oauthConfig = {
            access_token: document.getElementById('oauth-access-token').value,
            refresh_token: document.getElementById('oauth-refresh-token').value,
            expires_at: parseInt(document.getElementById('oauth-expires-at').value),
            token_url: document.getElementById('oauth-token-url').value,
            client_id: document.getElementById('oauth-client-id').value || '',
            scopes: scopes,
            auto_refresh: document.getElementById('oauth-auto-refresh').checked
        };
        
        // Remove empty optional fields
        if (!oauthConfig.client_id) delete oauthConfig.client_id;
        if (oauthConfig.scopes.length === 0) delete oauthConfig.scopes;
    } else {
        // Get regular auth value
        authValue = document.getElementById('endpoint-auth-value').value;
        if (!isAuthVisible && originalAuthValue && authValue.startsWith('*')) {
            // If showing asterisks and has original value, use original value
            authValue = originalAuthValue;
        }
    }

    // Parse tags field
    const tagsInput = document.getElementById('endpoint-tags').value.trim();
    const tags = tagsInput ? tagsInput.split(',').map(tag => tag.trim()).filter(tag => tag) : [];

    const data = {
        name: document.getElementById('endpoint-name').value,
        url: document.getElementById('endpoint-url').value,
        endpoint_type: document.getElementById('endpoint-type').value,
        path_prefix: document.getElementById('endpoint-path-prefix').value || '', // PathPrefix can be empty
        auth_type: authType,
        auth_value: authValue,
        enabled: document.getElementById('endpoint-enabled').checked,
        tags: tags,
        proxy: collectProxyData() // New: collect proxy configuration
    };
    
    // Add OAuth config if present
    if (oauthConfig) {
        data.oauth_config = oauthConfig;
    }

    const isEditing = editingEndpointName !== null;
    const url = isEditing 
        ? `/admin/api/endpoints/${encodeURIComponent(editingEndpointName)}` 
        : '/admin/api/endpoints';
    const method = isEditing ? 'PUT' : 'POST';

    fetch(url, {
        method: method,
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data)
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            // After successful save, if there's model rewrite configuration, save it too
            const modelRewriteConfig = collectModelRewriteData();
            const endpointName = document.getElementById('endpoint-name').value;
            
            const saveModelRewrite = modelRewriteConfig 
                ? saveModelRewriteConfig(endpointName, modelRewriteConfig)
                : Promise.resolve();
            
            saveModelRewrite
                .then(() => {
                    endpointModal.hide();
                    showAlert(data.message, 'success');
                    loadEndpoints(); // Reload data instead of refreshing page
                })
                .catch(error => {
                    console.error('Failed to save model rewrite config:', error);
                    showAlert('端点保存成功，但模型重写配置保存失败: ' + error.message, 'warning');
                    endpointModal.hide();
                    loadEndpoints();
                });
        }
    })
    .catch(error => {
        console.error('Failed to save endpoint:', error);
        showAlert('Failed to save endpoint', 'danger');
    });
}

function deleteEndpoint(endpointName) {
    if (!confirm(`确定要删除端点 "${endpointName}" 吗？`)) {
        return;
    }

    fetch(`/admin/api/endpoints/${encodeURIComponent(endpointName)}`, {
        method: 'DELETE'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            showAlert(data.message, 'success');
            loadEndpoints(); // Reload data instead of refreshing page
        }
    })
    .catch(error => {
        console.error('Failed to delete endpoint:', error);
        showAlert('Failed to delete endpoint', 'danger');
    });
}

function copyEndpoint(endpointName) {
    if (!confirm(`确定要复制端点 "${endpointName}" 吗？`)) {
        return;
    }

    fetch(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/copy`, {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            showAlert(data.message, 'success');
            loadEndpoints(); // Reload data to show newly copied endpoint
        }
    })
    .catch(error => {
        console.error('Failed to copy endpoint:', error);
        showAlert('Failed to copy endpoint', 'danger');
    });
}

function toggleEndpointEnabled(endpointName, currentEnabled) {
    const newEnabled = !currentEnabled;
    const actionText = newEnabled ? '启用' : '禁用';
    
    fetch(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/toggle`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            enabled: newEnabled
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            showAlert(`端点 "${endpointName}" 已${actionText}`, 'success');
            // 更新按钮状态而不重新加载整个表格
            updateEndpointToggleButton(endpointName, newEnabled);
            // 更新启用状态显示
            updateEndpointEnabledBadge(endpointName, newEnabled);
        }
    })
    .catch(error => {
        console.error('Failed to toggle endpoint:', error);
        showAlert(`${actionText}端点失败`, 'danger');
    });
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

function reorderEndpoints() {
    // Get special endpoint order
    const specialRows = document.querySelectorAll('#special-endpoint-list tr');
    const specialOrderedNames = Array.from(specialRows).map(row => row.dataset.endpointName);
    
    // Get general endpoint order
    const generalRows = document.querySelectorAll('#general-endpoint-list tr');
    const generalOrderedNames = Array.from(generalRows).map(row => row.dataset.endpointName);
    
    // Merge order: special endpoints first, general endpoints later
    const orderedNames = [...specialOrderedNames, ...generalOrderedNames];
    
    fetch('/admin/api/endpoints/reorder', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            ordered_names: orderedNames
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
            loadEndpoints(); // Reload to restore order
        } else {
            showAlert(data.message, 'success');
            // Update priority display, no need to reload entire table
            let priorityIndex = 1;
            
            // Update special endpoint priorities
            specialRows.forEach((row) => {
                const priorityBadge = row.querySelector('.priority-badge');
                if (priorityBadge) {
                    priorityBadge.textContent = priorityIndex++;
                }
            });
            
            // Update general endpoint priorities
            generalRows.forEach((row) => {
                const priorityBadge = row.querySelector('.priority-badge');
                if (priorityBadge) {
                    priorityBadge.textContent = priorityIndex++;
                }
            });
        }
    })
    .catch(error => {
        console.error('Failed to reorder endpoints:', error);
        showAlert('Failed to reorder endpoints', 'danger');
        loadEndpoints(); // Reload to restore order
    });
}

// ===== Endpoint Type and Auth Type Functions =====

function onEndpointTypeChange() {
    togglePathPrefixField();
    toggleAuthTypeForEndpointType();
}

function togglePathPrefixField() {
    const endpointType = document.getElementById('endpoint-type').value;
    const pathPrefixGroup = document.getElementById('path-prefix-group');
    const pathPrefixInput = document.getElementById('endpoint-path-prefix');
    
    if (endpointType === 'openai') {
        pathPrefixGroup.style.display = 'block';
        pathPrefixInput.required = true;
        if (!pathPrefixInput.value) {
            pathPrefixInput.value = '/v1/chat/completions'; // Default value
        }
    } else {
        pathPrefixGroup.style.display = 'none';
        pathPrefixInput.required = false;
        pathPrefixInput.value = ''; // Clear value
    }
}

function toggleAuthTypeForEndpointType() {
    const endpointType = document.getElementById('endpoint-type').value;
    const authTypeSelect = document.getElementById('endpoint-auth-type');
    const currentValue = authTypeSelect.value;
    
    // Clear existing options
    authTypeSelect.innerHTML = '';
    
    if (endpointType === 'openai') {
        // OpenAI compatible endpoints only support authtoken and oauth
        authTypeSelect.innerHTML = `
            <option value="auth_token">Auth Token (Authorization Bearer)</option>
            <option value="oauth">OAuth 2.0</option>
        `;
        
        // Set default or preserve current value if valid
        if (currentValue === 'auth_token' || currentValue === 'oauth') {
            authTypeSelect.value = currentValue;
        } else {
            authTypeSelect.value = 'auth_token'; // Default to auth_token
        }
    } else {
        // Anthropic endpoints support all auth types
        authTypeSelect.innerHTML = `
            <option value="api_key">API Key (x-api-key)</option>
            <option value="auth_token">Auth Token (Authorization Bearer)</option>
            <option value="oauth">OAuth 2.0</option>
        `;
        
        // Set default or preserve current value
        if (currentValue && (currentValue === 'api_key' || currentValue === 'auth_token' || currentValue === 'oauth')) {
            authTypeSelect.value = currentValue;
        } else {
            authTypeSelect.value = 'auth_token'; // Default to auth_token
        }
    }
    
    authTypeSelect.disabled = false;
    
    // Trigger auth type change to update the display
    onAuthTypeChange();
}

function onAuthTypeChange() {
    const authType = document.getElementById('endpoint-auth-type').value;
    const authValueGroup = document.getElementById('auth-value-group');
    const oauthConfigGroup = document.getElementById('oauth-config-group');
    const authValueInput = document.getElementById('endpoint-auth-value');
    
    if (authType === 'oauth') {
        // 显示 OAuth 配置，隐藏认证值输入
        authValueGroup.style.display = 'none';
        oauthConfigGroup.style.display = 'block';
        authValueInput.required = false;
        
        // OAuth 必填字段设置为必填
        document.getElementById('oauth-access-token').required = true;
        document.getElementById('oauth-refresh-token').required = true;
        document.getElementById('oauth-expires-at').required = true;
        document.getElementById('oauth-token-url').required = true;
    } else {
        // 显示认证值输入，隐藏 OAuth 配置
        authValueGroup.style.display = 'block';
        oauthConfigGroup.style.display = 'none';
        authValueInput.required = true;
        
        // OAuth 字段不再必填
        document.getElementById('oauth-access-token').required = false;
        document.getElementById('oauth-refresh-token').required = false;
        document.getElementById('oauth-expires-at').required = false;
        document.getElementById('oauth-token-url').required = false;
    }
}

// ===== Modal Functions =====

// Proxy configuration enable/disable toggle
document.getElementById('proxy-enabled').addEventListener('change', function() {
    const proxyConfigDiv = document.getElementById('proxy-config');
    proxyConfigDiv.style.display = this.checked ? 'block' : 'none';
});

// Model rewrite enable/disable toggle
document.getElementById('model-rewrite-enabled').addEventListener('change', function() {
    const rulesDiv = document.getElementById('model-rewrite-rules');
    rulesDiv.style.display = this.checked ? 'block' : 'none';
});

// Add rewrite rule
function addRewriteRule(sourcePattern = '', targetModel = '') {
    const rulesList = document.getElementById('rewrite-rules-list');
    const ruleIndex = rulesList.children.length;
    
    const ruleDiv = document.createElement('div');
    ruleDiv.className = 'row mb-2 rewrite-rule';
    ruleDiv.innerHTML = `
        <div class="col-5">
            <select class="form-select source-model-select" onchange="updateSourcePattern(${ruleIndex})">
                <option value="">选择预设模型</option>
                <option value="claude-*haiku*">Haiku 系列</option>
                <option value="claude-*sonnet*">Sonnet 系列</option>
                <option value="claude-*opus*">Opus 系列</option>
                <option value="claude-*">所有 Claude</option>
                <option value="custom">自定义通配符</option>
            </select>
            <input type="text" class="form-control mt-1 source-pattern-input" 
                   placeholder="通配符模式" value="${sourcePattern}" readonly>
        </div>
        <div class="col-5">
            <input type="text" class="form-control target-model-input" 
                   placeholder="目标模型 (如: deepseek-chat)" value="${targetModel}">
        </div>
        <div class="col-2">
            <button type="button" class="btn btn-outline-danger btn-sm" onclick="removeRewriteRule(this)">
                <i class="fas fa-trash"></i>
            </button>
            <button type="button" class="btn btn-outline-info btn-sm mt-1" onclick="testRewriteRule(${ruleIndex})" title="测试规则">
                <i class="fas fa-play"></i>
            </button>
        </div>
    `;
    
    rulesList.appendChild(ruleDiv);
}

// Update source pattern input
function updateSourcePattern(ruleIndex) {
    const ruleDiv = document.querySelectorAll('.rewrite-rule')[ruleIndex];
    const select = ruleDiv.querySelector('.source-model-select');
    const input = ruleDiv.querySelector('.source-pattern-input');
    
    if (select.value === 'custom') {
        input.readOnly = false;
        input.focus();
    } else {
        input.readOnly = true;
        input.value = select.value;
    }
}

// Remove rewrite rule
function removeRewriteRule(button) {
    button.closest('.rewrite-rule').remove();
}

// Test rewrite rule
function testRewriteRule(ruleIndex) {
    const testModel = prompt('请输入要测试的模型名称:', 'claude-3-haiku-20240307');
    if (!testModel) return;

    if (!editingEndpointName) {
        alert('请先保存端点后再测试规则');
        return;
    }

    fetch(`/admin/api/endpoints/${encodeURIComponent(editingEndpointName)}/test-model-rewrite`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ test_model: testModel })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            alert(`测试失败: ${data.error}`);
        } else {
            const message = data.rewrite_applied 
                ? `✅ 重写生效!\\n原模型: ${data.original_model}\\n重写为: ${data.rewritten_model}\\n匹配规则: ${data.matched_rule}`
                : `❌ 无重写\\n模型: ${data.original_model}\\n未匹配任何规则`;
            alert(message);
        }
    })
    .catch(error => {
        console.error('Test failed:', error);
        alert('测试失败，请检查网络连接');
    });
}

// Collect proxy configuration data
function collectProxyData() {
    const enabled = document.getElementById('proxy-enabled').checked;
    if (!enabled) {
        return null;
    }

    const type = document.getElementById('proxy-type').value;
    const address = document.getElementById('proxy-address').value.trim();
    const username = document.getElementById('proxy-username').value.trim();
    const password = document.getElementById('proxy-password').value.trim();
    
    if (!address) {
        return null; // Don't save proxy config if address is empty
    }

    const proxyConfig = {
        type: type,
        address: address
    };

    // Only add auth info if both username and password are not empty
    if (username && password) {
        proxyConfig.username = username;
        proxyConfig.password = password;
    }

    return proxyConfig;
}

// Load proxy configuration to form
function loadProxyConfig(config) {
    const checkbox = document.getElementById('proxy-enabled');
    const configDiv = document.getElementById('proxy-config');
    
    if (config) {
        checkbox.checked = true;
        configDiv.style.display = 'block';
        
        document.getElementById('proxy-type').value = config.type || 'http';
        document.getElementById('proxy-address').value = config.address || '';
        document.getElementById('proxy-username').value = config.username || '';
        document.getElementById('proxy-password').value = config.password || '';
    } else {
        checkbox.checked = false;
        configDiv.style.display = 'none';
        
        // Reset form fields
        document.getElementById('proxy-type').value = 'http';
        document.getElementById('proxy-address').value = '';
        document.getElementById('proxy-username').value = '';
        document.getElementById('proxy-password').value = '';
    }
}

// Collect model rewrite configuration data
function collectModelRewriteData() {
    const enabled = document.getElementById('model-rewrite-enabled').checked;
    if (!enabled) {
        return null;
    }

    const rules = [];
    document.querySelectorAll('.rewrite-rule').forEach(ruleDiv => {
        const sourcePattern = ruleDiv.querySelector('.source-pattern-input').value.trim();
        const targetModel = ruleDiv.querySelector('.target-model-input').value.trim();
        
        if (sourcePattern && targetModel) {
            rules.push({
                source_pattern: sourcePattern,
                target_model: targetModel
            });
        }
    });

    return rules.length > 0 ? { enabled: true, rules: rules } : null;
}

// Load model rewrite configuration to form
function loadModelRewriteConfig(config) {
    const checkbox = document.getElementById('model-rewrite-enabled');
    const rulesDiv = document.getElementById('model-rewrite-rules');
    const rulesList = document.getElementById('rewrite-rules-list');
    
    // Clear existing rules
    rulesList.innerHTML = '';
    
    if (config && config.enabled && config.rules) {
        checkbox.checked = true;
        rulesDiv.style.display = 'block';
        
        config.rules.forEach(rule => {
            addRewriteRule(rule.source_pattern, rule.target_model);
        });
    } else {
        checkbox.checked = false;
        rulesDiv.style.display = 'none';
    }
}

// Save model rewrite configuration
function saveModelRewriteConfig(endpointName, config) {
    if (!config) return Promise.resolve();

    return fetch(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/model-rewrite`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            throw new Error(data.error);
        }
        return data;
    });
}
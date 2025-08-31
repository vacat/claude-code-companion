// Tagger Management JavaScript

let taggers = [];
let tags = [];
let editingTagger = null;

// Initialize page
document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    loadTaggers();
    loadTags();
    
    // Wait for elements and translation system to be ready
    function initializeEventListeners() {
        // Check if all required elements exist and translation system is ready
        const addBtn = document.getElementById('addTaggerBtn');
        const saveBtn = document.getElementById('saveTaggerBtn');
        const typeSelect = document.getElementById('taggerType');
        const builtinSelect = document.getElementById('builtinType');
        
        if (!addBtn || !saveBtn || !typeSelect || !builtinSelect) {
            console.log('Tagger elements not ready, waiting...');
            setTimeout(initializeEventListeners, 100);
            return;
        }
        
        // Check if translation system is ready
        if (typeof T !== 'function' || !window.I18n) {
            console.log('Translation system not ready, waiting...');
            setTimeout(initializeEventListeners, 100);
            return;
        }
        
        // Check if translations are loaded
        const allTranslations = window.I18n.getAllTranslations();
        const currentLang = window.I18n.getLanguage();
        if (!allTranslations[currentLang] || Object.keys(allTranslations[currentLang]).length === 0) {
            console.log('Translations not loaded yet, waiting...');
            setTimeout(initializeEventListeners, 100);
            return;
        }
        
        console.log('Initializing tagger event listeners...');
        
        // Event listeners
        addBtn.addEventListener('click', function() {
            console.log('Add tagger button clicked');
            showAddTaggerModal();
        });
        saveBtn.addEventListener('click', saveTagger);
        typeSelect.addEventListener('change', onTypeChange);
        builtinSelect.addEventListener('change', onBuiltinTypeChange);
    }
    
    initializeEventListeners();
});

// Load taggers from API
async function loadTaggers() {
    try {
        const response = await apiRequest('/admin/api/taggers');
        const data = await response.json();
        
        if (data.enabled) {
            document.getElementById('systemStatus').textContent = T('enabled_status', '已启用');
            document.getElementById('systemStatus').className = 'fs-4 fw-bold text-success';
            document.getElementById('pipelineTimeout').textContent = data.timeout || '5s';
            
            taggers = data.taggers || [];
            document.getElementById('taggerCount').textContent = taggers.filter(t => t.enabled).length;
            
            renderTaggers();
        } else {
            document.getElementById('systemStatus').textContent = T('disabled_status', '已禁用');
            document.getElementById('systemStatus').className = 'fs-4 fw-bold text-warning';
            taggers = [];
            renderTaggers();
        }
    } catch (error) {
        console.error('Failed to load taggers:', error);
        showAlert(T('load_tagger_failed', '加载 Tagger 失败'), 'danger');
    }
}

// Load tags from API
async function loadTags() {
    try {
        const response = await apiRequest('/admin/api/tags');
        const data = await response.json();
        
        if (data.enabled) {
            tags = data.tags || [];
            document.getElementById('tagCount').textContent = tags.length;
            renderTags();
        }
    } catch (error) {
        console.error('Failed to load tags:', error);
        showAlert('加载标签失败', 'danger');
    }
}

// Render taggers table
function renderTaggers() {
    const tbody = document.querySelector('#taggersTable tbody');
    tbody.innerHTML = '';
    
    if (taggers.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="text-center text-muted">No taggers configured</td></tr>';
        return;
    }
    
    taggers.forEach(tagger => {
        const row = document.createElement('tr');
        row.innerHTML = `
            <td>
                <strong>${escapeHtml(tagger.name)}</strong>
            </td>
            <td>
                <span class="badge ${tagger.type === 'builtin' ? 'bg-primary' : 'bg-info'}">${escapeHtml(tagger.type)}</span>
            </td>
            <td>
                ${tagger.builtin_type ? `<span class="badge bg-secondary">${escapeHtml(tagger.builtin_type)}</span>` : ''}
            </td>
            <td><span class="badge bg-secondary">${escapeHtml(tagger.tag)}</span></td>
            <td>${tagger.priority}</td>
            <td>
                <span class="badge ${tagger.enabled ? 'bg-success' : 'bg-warning'}">
                    ${tagger.enabled ? T('enabled', '已启用') : T('disabled', '已禁用')}
                </span>
            </td>
            <td>
                <button class="btn btn-sm btn-outline-primary" onclick="editTagger('${tagger.name}')">
                    <i class="fas fa-edit"></i> 编辑
                </button>
                <button class="btn btn-sm btn-outline-danger" onclick="deleteTagger('${tagger.name}')">
                    <i class="fas fa-trash"></i> 删除
                </button>
            </td>
        `;
        tbody.appendChild(row);
    });
}

// Render tags
function renderTags() {
    const container = document.getElementById('tagsContainer');
    container.innerHTML = '';
    
    if (tags.length === 0) {
        container.innerHTML = '<p class="text-muted">' + T('no_tags_registered', 'No tags registered') + '</p>';
        // Process translation for dynamic content
        if (window.I18n && window.I18n.processDataTElements) {
            window.I18n.processDataTElements();
        }
        return;
    }
    
    tags.forEach(tag => {
        const tagElement = document.createElement('span');
        tagElement.className = `badge me-2 mb-2 ${tag.in_use ? 'bg-success' : 'bg-secondary'}`;
        tagElement.innerHTML = `
            ${escapeHtml(tag.name)}
            ${tag.in_use ? '<i class="fas fa-check-circle ms-1"></i>' : ''}
        `;
        tagElement.title = tag.description + (tag.in_use ? ' (in use)' : ' (not used)');
        container.appendChild(tagElement);
    });
}

// Show add tagger modal
function showAddTaggerModal() {
    // Check if translation system is ready
    if (typeof T !== 'function') {
        console.warn('Translation system not ready for showAddTaggerModal');
        return;
    }
    
    editingTagger = null;
    document.getElementById('taggerModalTitle').textContent = T('add_tagger', '添加 Tagger');
    document.getElementById('taggerForm').reset();
    document.getElementById('taggerEnabled').checked = true;
    clearConfigFields();
    new bootstrap.Modal(document.getElementById('taggerModal')).show();
}

// Edit tagger
function editTagger(name) {
    editingTagger = taggers.find(t => t.name === name);
    if (!editingTagger) return;
    
    document.getElementById('taggerModalTitle').textContent = T('edit_tagger', '编辑 Tagger');
    document.getElementById('taggerName').value = editingTagger.name;
    document.getElementById('taggerType').value = editingTagger.type;
    document.getElementById('taggerTag').value = editingTagger.tag;
    document.getElementById('taggerPriority').value = editingTagger.priority;
    document.getElementById('taggerEnabled').checked = editingTagger.enabled;
    
    if (editingTagger.type === 'builtin') {
        document.getElementById('builtinType').value = editingTagger.builtin_type || '';
        onBuiltinTypeChange();
    }
    
    onTypeChange();
    loadConfigFields(editingTagger.config || {});
    
    new bootstrap.Modal(document.getElementById('taggerModal')).show();
}

// Delete tagger
async function deleteTagger(name) {
    if (!confirm(T('confirm_delete_tagger', '确定要删除 Tagger "{0}" 吗？').replace('{0}', name))) {
        return;
    }
    
    try {
        const response = await apiRequest(`/admin/api/taggers/${encodeURIComponent(name)}`, {
            method: 'DELETE'
        });
        
        const data = await response.json();
        
        if (response.ok) {
            showAlert(T('tagger_deleted_successfully', 'Tagger deleted successfully'), 'success');
            loadTaggers();
            loadTags();
        } else {
            showAlert(data.error || T('delete_tagger_failed', '删除 Tagger 失败'), 'danger');
        }
    } catch (error) {
        console.error('Failed to delete tagger:', error);
        showAlert(T('delete_tagger_failed', '删除 Tagger 失败'), 'danger');
    }
}

// Save tagger
async function saveTagger() {
    const name = document.getElementById('taggerName').value.trim();
    const type = document.getElementById('taggerType').value;
    const tag = document.getElementById('taggerTag').value.trim();
    const priority = parseInt(document.getElementById('taggerPriority').value);
    const enabled = document.getElementById('taggerEnabled').checked;
    
    if (!name || !type || !tag) {
        showAlert(T('please_fill_required_fields', 'Please fill in all required fields'), 'warning');
        return;
    }
    
    const taggerData = {
        name,
        type,
        tag,
        priority,
        enabled,
        config: collectConfigFields()
    };
    
    if (type === 'builtin') {
        const builtinType = document.getElementById('builtinType').value;
        if (!builtinType) {
            showAlert(T('please_select_builtin_type', 'Please select a built-in type'), 'warning');
            return;
        }
        taggerData.builtin_type = builtinType;
    }
    
    try {
        const url = editingTagger ? 
            `/admin/api/taggers/${encodeURIComponent(editingTagger.name)}` : 
            '/admin/api/taggers';
        const method = editingTagger ? 'PUT' : 'POST';
        
        const response = await apiRequest(url, {
            method,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(taggerData)
        });
        
        const data = await response.json();
        
        if (response.ok) {
            showAlert(editingTagger ? T('tagger_updated_successfully', 'Tagger updated successfully') : T('tagger_created_successfully', 'Tagger created successfully'), 'success');
            bootstrap.Modal.getInstance(document.getElementById('taggerModal')).hide();
            loadTaggers();
            loadTags();
        } else {
            showAlert(data.error || T('save_tagger_failed', '保存 Tagger 失败'), 'danger');
        }
    } catch (error) {
        console.error('Failed to save tagger:', error);
        showAlert(T('save_tagger_failed', '保存 Tagger 失败'), 'danger');
    }
}

// Handle type change
function onTypeChange() {
    const type = document.getElementById('taggerType').value;
    const builtinGroup = document.getElementById('builtinTypeGroup');
    
    if (type === 'builtin') {
        StyleUtils.show(builtinGroup);
        onBuiltinTypeChange();
    } else if (type === 'starlark') {
        StyleUtils.hide(builtinGroup);
        showStarlarkConfig();
    } else {
        StyleUtils.hide(builtinGroup);
        clearConfigFields();
    }
}

// Handle builtin type change
function onBuiltinTypeChange() {
    const builtinType = document.getElementById('builtinType').value;
    clearConfigFields();
    
    switch (builtinType) {
        case 'path':
            addConfigField('path_pattern', 'text', T('path_pattern_label', '路径(支持通配符)'), '/v1/*');
            break;
        case 'header':
            addConfigField('header_name', 'text', T('http_header_name', 'HTTP 头名称'), 'Content-Type');
            addConfigField('expected_value', 'text', T('http_header_content', 'HTTP 头内容(支持通配符)'), 'application/json');
            break;
        case 'query':
            addConfigField('param_name', 'text', T('http_param_name', 'HTTP 参数名'), 'beta');
            addConfigField('expected_value', 'text', T('http_param_content', 'HTTP 参数内容(支持通配符)'), 'true');
            break;
        case 'body-json':
            addConfigField('json_path', 'text', T('json_path_label', 'JSON 路径'), 'messages[0].text');
            addConfigField('expected_value', 'text', T('field_content_wildcards', '字段内容(支持通配符)'), 'claude-3*');
            break;
        case 'user-message':
            addConfigField('expected_value', 'text', T('prompt_content_wildcards', 'Prompt内容(支持通配符)'), '*#use-claude*');
            break;
        case 'model':
            addConfigField('expected_value', 'text', T('model_name_wildcards', '模型名(支持通配符)'), 'claude-3*');
            break;
        case 'thinking':
            addConfigField('min_budget_tokens', 'number', T('min_budget_tokens_label', '最小 Budget Tokens (optional, default: 0)'), '0');
            break;
    }
}

// Show Starlark config
function showStarlarkConfig() {
    clearConfigFields();
    addConfigField('script', 'textarea', 'Starlark Script', 
        'def should_tag():\n    # Write your logic here\n    return True');
}

// Add config field
function addConfigField(key, type, label, placeholder = '') {
    const container = document.getElementById('configContainer');
    const fieldDiv = document.createElement('div');
    fieldDiv.className = 'mb-3';
    fieldDiv.dataset.configKey = key;
    
    if (type === 'textarea') {
        fieldDiv.innerHTML = `
            <label class="form-label">${escapeHtml(label)}</label>
            <textarea class="form-control" rows="8" placeholder="${escapeHtml(placeholder)}"></textarea>
        `;
    } else {
        fieldDiv.innerHTML = `
            <label class="form-label">${escapeHtml(label)}</label>
            <input type="${type}" class="form-control" placeholder="${escapeHtml(placeholder)}">
        `;
    }
    
    container.appendChild(fieldDiv);
}

// Clear config fields
function clearConfigFields() {
    document.getElementById('configContainer').innerHTML = '';
}

// Load config fields with values
function loadConfigFields(config) {
    const fields = document.querySelectorAll('#configContainer [data-config-key]');
    fields.forEach(field => {
        const key = field.dataset.configKey;
        const input = field.querySelector('input, textarea');
        if (input && config[key] !== undefined) {
            input.value = config[key];
        }
    });
}

// Collect config field values
function collectConfigFields() {
    const config = {};
    const fields = document.querySelectorAll('#configContainer [data-config-key]');
    
    fields.forEach(field => {
        const key = field.dataset.configKey;
        const input = field.querySelector('input, textarea');
        if (input && input.value.trim()) {
            config[key] = input.value.trim();
        }
    });
    
    return config;
}

// Utility functions
function showAlert(message, type) {
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type} alert-dismissible fade show`;
    alertDiv.innerHTML = `
        ${escapeHtml(message)}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    document.querySelector('.container').insertBefore(alertDiv, document.querySelector('.container').firstChild);
    
    setTimeout(() => {
        alertDiv.remove();
    }, 5000);
}


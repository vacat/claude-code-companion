// Endpoints Advanced JavaScript - 高级功能

// ===== Modal Functions =====

// Proxy configuration enable/disable toggle
document.getElementById('proxy-enabled').addEventListener('change', function() {
    const proxyConfigDiv = document.getElementById('proxy-config');
    this.checked ? StyleUtils.show(proxyConfigDiv) : StyleUtils.hide(proxyConfigDiv);
});

// Model rewrite enable/disable toggle
document.getElementById('model-rewrite-enabled').addEventListener('change', function() {
    const rulesDiv = document.getElementById('model-rewrite-rules');
    this.checked ? StyleUtils.show(rulesDiv) : StyleUtils.hide(rulesDiv);
    
    // If disabling model rewrite, check if we should clear default model rules
    if (!this.checked) {
        clearDefaultModelRulesIfApplicable();
    }
    
    // Update default model state when model rewrite toggle changes
    updateDefaultModelState();
});

// Max tokens override enable/disable toggle
document.getElementById('max-tokens-override-enabled').addEventListener('change', function() {
    const configDiv = document.getElementById('max-tokens-override-config');
    this.checked ? StyleUtils.show(configDiv) : StyleUtils.hide(configDiv);
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
                   placeholder="通配符模式" value="${escapeHtml(sourcePattern)}" readonly>
        </div>
        <div class="col-5">
            <input type="text" class="form-control target-model-input" 
                   placeholder="目标模型 (如: deepseek-chat)" value="${escapeHtml(targetModel)}" 
                   oninput="onRewriteRuleTargetChange()">
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
    
    // Update default model state when rules change
    updateDefaultModelState();
}

// Clear default model rules when disabling model rewrite
function clearDefaultModelRulesIfApplicable() {
    const rules = collectCurrentRewriteRules();
    
    // If there's exactly one rule with pattern "*", remove it
    if (rules.length === 1 && rules[0].source_pattern === '*') {
        document.getElementById('rewrite-rules-list').innerHTML = '';
        // Clear default model value as well
        document.getElementById('endpoint-default-model').value = '';
    }
}

// Handle target model changes in rewrite rules
function onRewriteRuleTargetChange() {
    // Update default model state when target model changes
    updateDefaultModelState();
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
    
    // Update default model state when pattern changes
    updateDefaultModelState();
}

// Remove rewrite rule
function removeRewriteRule(button) {
    button.closest('.rewrite-rule').remove();
    // Update default model state when rules change
    updateDefaultModelState();
}

// Test rewrite rule
function testRewriteRule(ruleIndex) {
    const testModel = prompt('请输入要测试的模型名称:', 'claude-3-haiku-20240307');
    if (!testModel) return;

    if (!editingEndpointName) {
        alert('请先保存端点后再测试规则');
        return;
    }

    apiRequest(`/admin/api/endpoints/${encodeURIComponent(editingEndpointName)}/test-model-rewrite`, {
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
        StyleUtils.show(configDiv);
        
        document.getElementById('proxy-type').value = config.type || 'http';
        document.getElementById('proxy-address').value = config.address || '';
        document.getElementById('proxy-username').value = config.username || '';
        document.getElementById('proxy-password').value = config.password || '';
    } else {
        checkbox.checked = false;
        StyleUtils.hide(configDiv);
        
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
        StyleUtils.show(rulesDiv);
        
        config.rules.forEach(rule => {
            addRewriteRule(rule.source_pattern, rule.target_model);
        });
    } else {
        checkbox.checked = false;
        StyleUtils.hide(rulesDiv);
    }
    
    // Update default model state after loading model rewrite config
    updateDefaultModelState();
}

// Save model rewrite configuration
function saveModelRewriteConfig(endpointName, config) {
    if (!config) return Promise.resolve();

    return apiRequest(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/model-rewrite`, {
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

// ===== Max Tokens Override Functions =====

// Collect max_tokens override data
function collectMaxTokensOverrideData() {
    const enabled = document.getElementById('max-tokens-override-enabled').checked;
    if (!enabled) {
        return null;
    }

    const value = parseInt(document.getElementById('max-tokens-value').value);
    if (!value || value <= 0) {
        return null; // Don't save if value is empty or invalid
    }

    return value;
}

// Load max_tokens override configuration
function loadMaxTokensOverrideConfig(value) {
    const checkbox = document.getElementById('max-tokens-override-enabled');
    const configDiv = document.getElementById('max-tokens-override-config');
    
    if (value && value > 0) {
        checkbox.checked = true;
        StyleUtils.show(configDiv);
        
        document.getElementById('max-tokens-value').value = value;
    } else {
        checkbox.checked = false;
        StyleUtils.hide(configDiv);
        
        // Reset form field
        document.getElementById('max-tokens-value').value = '';
    }
}

// ===== Default Model Functions =====

// Load default model from model rewrite configuration
function loadDefaultModel(modelRewriteConfig) {
    const defaultModelInput = document.getElementById('endpoint-default-model');
    
    if (modelRewriteConfig && modelRewriteConfig.enabled && modelRewriteConfig.rules) {
        // Check if there's exactly one rule with pattern "*"
        if (modelRewriteConfig.rules.length === 1 && modelRewriteConfig.rules[0].source_pattern === '*') {
            defaultModelInput.value = modelRewriteConfig.rules[0].target_model;
        } else {
            defaultModelInput.value = '';
        }
    } else {
        defaultModelInput.value = '';
    }
    
    updateDefaultModelState();
}

// Update default model state based on model rewrite configuration
function updateDefaultModelState() {
    const defaultModelInput = document.getElementById('endpoint-default-model');
    const defaultModelHint = document.getElementById('default-model-hint');
    const modelRewriteEnabled = document.getElementById('model-rewrite-enabled').checked;
    
    if (!modelRewriteEnabled) {
        // Model rewrite disabled - default model can be edited
        defaultModelInput.disabled = false;
        defaultModelInput.title = '';
        StyleUtils.hide(defaultModelHint);
    } else {
        // Model rewrite enabled - check rules
        const rules = collectCurrentRewriteRules();
        
        if (rules.length === 0) {
            // No rules - default model can be edited
            defaultModelInput.disabled = false;
            defaultModelInput.title = '';
            StyleUtils.hide(defaultModelHint);
        } else if (rules.length === 1 && rules[0].source_pattern === '*') {
            // Single "*" rule - sync with default model
            defaultModelInput.disabled = false;
            defaultModelInput.title = '';
            defaultModelInput.value = rules[0].target_model;
            StyleUtils.hide(defaultModelHint);
        } else {
            // Multiple rules or non-"*" rules - disable default model
            defaultModelInput.disabled = true;
            defaultModelInput.title = 'Model Rewrite中有和默认模型不兼容的设置';
            StyleUtils.show(defaultModelHint);
        }
    }
}

// Collect current rewrite rules from the form
function collectCurrentRewriteRules() {
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
    return rules;
}

// Handle default model changes
function onDefaultModelChange() {
    const defaultModelInput = document.getElementById('endpoint-default-model');
    const modelRewriteEnabled = document.getElementById('model-rewrite-enabled').checked;
    const defaultModel = defaultModelInput.value.trim();
    
    if (!modelRewriteEnabled && defaultModel) {
        // Enable model rewrite and set single "*" rule
        document.getElementById('model-rewrite-enabled').checked = true;
        StyleUtils.show(document.getElementById('model-rewrite-rules'));
        
        // Clear existing rules and add new "*" rule
        document.getElementById('rewrite-rules-list').innerHTML = '';
        addRewriteRule('*', defaultModel);
    } else if (modelRewriteEnabled) {
        const rules = collectCurrentRewriteRules();
        if (rules.length === 1 && rules[0].source_pattern === '*') {
            // Update the single "*" rule
            const targetInput = document.querySelector('.rewrite-rule .target-model-input');
            if (targetInput) {
                targetInput.value = defaultModel;
            }
        }
    }
    
    updateDefaultModelState();
}

// Add event listener for default model input
document.addEventListener('DOMContentLoaded', function() {
    const defaultModelInput = document.getElementById('endpoint-default-model');
    if (defaultModelInput) {
        defaultModelInput.addEventListener('input', onDefaultModelChange);
        defaultModelInput.addEventListener('blur', onDefaultModelChange);
    }
});
// Endpoints Advanced JavaScript - 高级功能

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
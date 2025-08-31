// Anthropic 请求检查器主控制器

// 全局变量
let currentParser = null;
let currentUI = null;

// 入口函数
function openRequestInspector() {
    let requestBody = null;
    
    // 首先尝试从主检查按钮获取数据
    const mainBtn = document.getElementById('inspectRequestBtn');
    if (mainBtn) {
        requestBody = mainBtn.getAttribute('data-request-body');
    }
    
    // 如果主按钮没有数据，尝试从临时按钮获取
    if (!requestBody) {
        const tempBtn = document.getElementById('tempInspectRequestBtn');
        if (tempBtn) {
            requestBody = tempBtn.getAttribute('data-request-body');
        }
    }
    
    if (!requestBody) {
        const alertText = typeof T === 'function' ? T('request_data_not_found', '未找到请求数据') : '未找到请求数据';
        alert(alertText);
        return;
    }

    try {
        // 创建解析器和UI
        currentParser = new AnthropicRequestParser(requestBody);
        currentUI = new InspectorUI('inspectorContent');
        
        // 渲染界面
        currentUI.render(currentParser);
        
        // 显示模态框
        const modalElement = document.getElementById('requestInspectorModal');
        if (modalElement) {
            const modal = new bootstrap.Modal(modalElement);
            modal.show();
        } else {
            console.error('Inspector modal not found');
        }
    } catch (error) {
        console.error('Failed to open request inspector:', error);
        const alertText = typeof T === 'function' ? T('open_inspector_error', '打开请求检查器时出错') : '打开请求检查器时出错';
        alert(alertText + ': ' + error.message);
    }
}

// 检查是否为 Anthropic 请求
function isAnthropicRequest(requestBody) {
    if (!requestBody) return false;
    
    try {
        const data = JSON.parse(requestBody);
        // 检查基本的 Anthropic API 格式
        return data.model && data.messages && Array.isArray(data.messages);
    } catch {
        return false;
    }
}

// 折叠/展开控制 - 使用全局函数避免作用域问题
window.inspectorToggleCollapse = function(elementId) {
    const element = document.getElementById(elementId);
    const icon = document.getElementById(elementId + '-icon');
    
    if (!element) {
        console.warn('Element not found:', elementId);
        return;
    }
    
    // 获取当前显示状态
    const currentDisplay = window.getComputedStyle(element).display;
    const isHidden = currentDisplay === 'none' || element.classList.contains('d-none-custom');
    
    if (isHidden) {
        // 展开
        StyleUtils.show(element);
        if (icon) icon.textContent = '▼';
    } else {
        // 折叠
        StyleUtils.hide(element);
        if (icon) icon.textContent = '▶';
    }
};

// 导出分析功能
function exportAnalysis() {
    if (!currentParser) {
        const alertText = typeof T === 'function' ? T('no_data_to_export', '没有可导出的分析数据') : '没有可导出的分析数据';
        alert(alertText);
        return;
    }
    
    try {
        const messageStats = currentParser.getMessageStats();
        const toolStats = currentParser.getToolUsageStats();
        
        const analysis = {
            metadata: {
                exportTime: new Date().toISOString(),
                version: '1.0'
            },
            overview: currentParser.parsed.overview,
            statistics: {
                messages: messageStats,
                tools: toolStats,
                totalToolCalls: messageStats.totalToolCalls,
                totalSystemReminders: messageStats.totalSystemReminders
            },
            summary: {
                messageCount: currentParser.parsed.messages.length,
                toolCount: currentParser.parsed.tools.length,
                hasSystemPrompt: !!currentParser.parsed.system.content,
                hasErrors: currentParser.parsed.errors.length > 0
            }
        };
        
        const dataStr = JSON.stringify(analysis, null, 2);
        const dataBlob = new Blob([dataStr], {type: 'application/json'});
        const url = URL.createObjectURL(dataBlob);
        
        const link = document.createElement('a');
        link.href = url;
        link.download = `anthropic-analysis-${new Date().toISOString().slice(0, 19).replace(/:/g, '-')}.json`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        
        URL.revokeObjectURL(url);
    } catch (error) {
        console.error('Export failed:', error);
        const alertText = typeof T === 'function' ? T('export_failed_error', '导出失败') : '导出失败';
        alert(alertText + ': ' + error.message);
    }
}

// 在日志详情显示时检查并显示检查按钮
function updateInspectorButton(requestBody) {
    const inspectBtn = document.getElementById('inspectRequestBtn');
    if (!inspectBtn) return;
    
    if (isAnthropicRequest(requestBody)) {
        StyleUtils.show(inspectBtn);
        inspectBtn.setAttribute('data-request-body', requestBody);
    } else {
        StyleUtils.hide(inspectBtn);
        inspectBtn.removeAttribute('data-request-body');
    }
}

// 工具函数
function formatBytes(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatTimestamp(timestamp) {
    return new Date(timestamp).toLocaleString('zh-CN');
}

// 搜索功能（扩展功能）
function searchInInspector(query) {
    if (!query || !currentParser) return;
    
    const results = [];
    
    // 搜索系统提示
    if (currentParser.parsed.system.content && 
        currentParser.parsed.system.content.toLowerCase().includes(query.toLowerCase())) {
        results.push({ type: 'system', content: 'System Prompt' });
    }
    
    // 搜索消息内容
    currentParser.parsed.messages.forEach((message, idx) => {
        message.content.forEach(content => {
            if (content.text && content.text.toLowerCase().includes(query.toLowerCase())) {
                results.push({ 
                    type: 'message', 
                    content: `Message ${idx + 1} (${message.role})`,
                    messageIndex: idx + 1
                });
            }
        });
        
        // 搜索工具调用
        message.pairedToolCalls.forEach(call => {
            if (call.name.toLowerCase().includes(query.toLowerCase()) ||
                JSON.stringify(call.input).toLowerCase().includes(query.toLowerCase()) ||
                (call.result && JSON.stringify(call.result).toLowerCase().includes(query.toLowerCase()))) {
                results.push({
                    type: 'tool',
                    content: `Tool call: ${call.name} in Message ${idx + 1}`,
                    messageIndex: idx + 1,
                    toolName: call.name
                });
            }
        });
    });
    
    return results;
}

// 快捷键支持
document.addEventListener('keydown', function(e) {
    // Ctrl/Cmd + F 在检查器中搜索
    if ((e.ctrlKey || e.metaKey) && e.key === 'f' && 
        !document.getElementById('requestInspectorModal')?.classList.contains('d-none-custom')) {
        e.preventDefault();
        const promptText = typeof T === 'function' ? T('search_content_prompt', '搜索内容:') : '搜索内容:';
        const query = prompt(promptText);
        if (query) {
            const results = searchInInspector(query);
            if (results.length > 0) {
                const successText = typeof T === 'function' ? 
                    T('search_results_found', '找到 {0} 个结果:\n{1}').replace('{0}', results.length).replace('{1}', results.map(r => r.content).join('\n')) :
                    `找到 ${results.length} 个结果:\n${results.map(r => r.content).join('\n')}`;
                alert(successText);
            } else {
                const noResultText = typeof T === 'function' ? T('no_matching_content', '未找到匹配的内容') : '未找到匹配的内容';
                alert(noResultText);
            }
        }
    }
});

// 消息排序切换功能
window.inspectorToggleMessageOrder = function() {
    if (!currentParser) {
        console.warn('No parser available for message ordering');
        return;
    }
    
    const container = document.getElementById('messages-container');
    const toggleBtn = document.getElementById('message-order-toggle');
    const icon = document.getElementById('message-order-icon');
    const text = document.getElementById('message-order-text');
    
    if (!container || !toggleBtn || !icon || !text) {
        console.warn('Message order toggle elements not found');
        return;
    }
    
    // 获取当前的排序状态
    const isReversed = toggleBtn.getAttribute('data-reversed') === 'true';
    
    // 获取消息数据并重新渲染
    const messages = currentParser.parsed.messages;
    let orderedMessages;
    
    if (isReversed) {
        // 当前是逆序，切换到正序
        orderedMessages = [...messages];
        toggleBtn.setAttribute('data-reversed', 'false');
        icon.textContent = '↑';
        text.textContent = T('normal_order', '正向排列');
    } else {
        // 当前是正序，切换到逆序
        orderedMessages = [...messages].reverse();
        toggleBtn.setAttribute('data-reversed', 'true');
        icon.textContent = '↓';
        text.textContent = T('reverse_order', '逆向排列');
    }
    
    // 重新渲染消息容器内容
    let messagesHtml = '';
    orderedMessages.forEach(message => {
        messagesHtml += currentUI.renderMessage(message);
    });
    
    container.innerHTML = messagesHtml;
};

// 检查器初始化
function initializeInspector() {
    // 检查依赖
    if (typeof AnthropicRequestParser === 'undefined') {
        console.error('AnthropicRequestParser not loaded');
        return false;
    }
    
    if (typeof InspectorUI === 'undefined') {
        console.error('InspectorUI not loaded');
        return false;
    }
    
    if (typeof bootstrap === 'undefined') {
        console.error('Bootstrap not loaded');
        return false;
    }
    
    console.log('Anthropic Request Inspector initialized successfully');
    return true;
}

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', function() {
    // 延迟初始化以确保所有依赖都加载完成
    setTimeout(initializeInspector, 100);
});
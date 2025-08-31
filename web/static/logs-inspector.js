// Logs Page Request Inspector Functions

// Open request inspector from floating button
function openRequestInspectorFromFloating(button) {
    const encodedContent = button.getAttribute('data-request-body');
    if (!encodedContent) {
        alert(T('request_data_not_found', '未找到请求数据'));
        return;
    }
    
    try {
        // Use safeBase64Decode instead of atob for proper UTF-8 handling
        const requestBody = safeBase64Decode(encodedContent);
        
        // Temporarily set to hidden button element for openRequestInspector to use
        let tempBtn = document.getElementById('tempInspectRequestBtn');
        if (!tempBtn) {
            tempBtn = document.createElement('button');
            tempBtn.id = 'tempInspectRequestBtn';
            StyleUtils.hide(tempBtn);
            document.body.appendChild(tempBtn);
        }
        tempBtn.setAttribute('data-request-body', requestBody);
        
        // Call inspector
        openRequestInspector();
    } catch (e) {
        console.error('Failed to decode request body:', e);
        alert(T('request_decode_failed', '请求数据解码失败'));
    }
}

// Open request inspector from main button
function openRequestInspectorFromMain(button) {
    const encodedContent = button.getAttribute('data-request-body');
    if (!encodedContent) {
        alert(T('request_data_not_found', '未找到请求数据'));
        return;
    }
    
    try {
        // Use safeBase64Decode instead of atob for proper UTF-8 handling
        const requestBody = safeBase64Decode(encodedContent);
        
        // Temporarily set to hidden button element for openRequestInspector to use
        let tempBtn = document.getElementById('tempInspectRequestBtn');
        if (!tempBtn) {
            tempBtn = document.createElement('button');
            tempBtn.id = 'tempInspectRequestBtn';
            StyleUtils.hide(tempBtn);
            document.body.appendChild(tempBtn);
        }
        tempBtn.setAttribute('data-request-body', requestBody);
        
        // Call inspector
        openRequestInspector();
    } catch (e) {
        console.error('Failed to decode request body:', e);
        alert(T('request_decode_failed', '请求数据解码失败'));
    }
}

// Check if request body is Anthropic request
function isRequestBodyAnthropicRequest(requestBody) {
    if (!requestBody) return false;
    
    try {
        const data = JSON.parse(requestBody);
        // Check basic Anthropic API format
        return data.model && data.messages && Array.isArray(data.messages);
    } catch {
        return false;
    }
}
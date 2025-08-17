// Logs Page Request Inspector Functions

// 从浮动按钮打开请求检查器
function openRequestInspectorFromFloating(button) {
    const encodedContent = button.getAttribute('data-request-body');
    if (!encodedContent) {
        alert('未找到请求数据');
        return;
    }
    
    try {
        // 使用 safeBase64Decode 而不是 atob 来正确处理UTF-8编码
        const requestBody = safeBase64Decode(encodedContent);
        
        // 临时设置到隐藏的按钮元素上，供 openRequestInspector 使用
        let tempBtn = document.getElementById('tempInspectRequestBtn');
        if (!tempBtn) {
            tempBtn = document.createElement('button');
            tempBtn.id = 'tempInspectRequestBtn';
            tempBtn.style.display = 'none';
            document.body.appendChild(tempBtn);
        }
        tempBtn.setAttribute('data-request-body', requestBody);
        
        // 调用检查器
        openRequestInspector();
    } catch (e) {
        console.error('Failed to decode request body:', e);
        alert('请求数据解码失败');
    }
}

// 从主按钮打开请求检查器
function openRequestInspectorFromMain(button) {
    const encodedContent = button.getAttribute('data-request-body');
    if (!encodedContent) {
        alert('未找到请求数据');
        return;
    }
    
    try {
        // 使用 safeBase64Decode 而不是 atob 来正确处理UTF-8编码
        const requestBody = safeBase64Decode(encodedContent);
        
        // 临时设置到隐藏的按钮元素上，供 openRequestInspector 使用
        let tempBtn = document.getElementById('tempInspectRequestBtn');
        if (!tempBtn) {
            tempBtn = document.createElement('button');
            tempBtn.id = 'tempInspectRequestBtn';
            tempBtn.style.display = 'none';
            document.body.appendChild(tempBtn);
        }
        tempBtn.setAttribute('data-request-body', requestBody);
        
        // 调用检查器
        openRequestInspector();
    } catch (e) {
        console.error('Failed to decode request body:', e);
        alert('请求数据解码失败');
    }
}

// 检查请求体是否为Anthropic请求
function isRequestBodyAnthropicRequest(requestBody) {
    if (!requestBody) return false;
    
    try {
        const data = JSON.parse(requestBody);
        // 检查基本的 Anthropic API 格式
        return data.model && data.messages && Array.isArray(data.messages);
    } catch {
        return false;
    }
}
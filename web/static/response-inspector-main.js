let currentResponseParser = null;
let currentResponseUI = null;

function openResponseInspector(buttonElement) {
    const responseBtn = buttonElement || document.getElementById('inspectResponseBtn');
    const responseBody = responseBtn.getAttribute('data-response-body');
    const isStreaming = responseBtn.getAttribute('data-is-streaming') === 'true';
    const finalResponse = responseBtn.getAttribute('data-final-response');

    if (!responseBody) {
        alert('未找到响应数据');
        return;
    }

    try {
        // 解码 base64 数据
        const decodedResponseBody = safeBase64Decode(responseBody);
        const decodedFinalResponse = finalResponse ? safeBase64Decode(finalResponse) : '';
        
        currentResponseParser = new AnthropicResponseParser(decodedResponseBody, isStreaming, decodedFinalResponse);
        currentResponseUI = new ResponseInspectorUI('responseInspectorContent');
        
        currentResponseUI.render(currentResponseParser);
        
        const modalElement = document.getElementById('responseInspectorModal');
        if (modalElement) {
            const modal = new bootstrap.Modal(modalElement);
            modal.show();
        }
    } catch (error) {
        console.error('Failed to open response inspector:', error);
        alert('打开响应检查器时出错: ' + error.message);
    }
}

function toggleResponseCollapse(elementId) {
    const element = document.getElementById(elementId);
    const icon = document.getElementById(elementId + '-icon');
    
    if (!element) return;
    
    if (element.style.display === 'none') {
        element.style.display = 'block';
        if (icon) icon.textContent = '▼';
    } else {
        element.style.display = 'none';
        if (icon) icon.textContent = '▶';
    }
}

function exportResponseAnalysis() {
    if (!currentResponseParser) return;
    
    const analysis = {
        metadata: currentResponseParser.parsed.metadata,
        usage: currentResponseParser.parsed.usage,
        contentSummary: currentResponseParser.parsed.content.map(block => ({
            type: block.type,
            size: block.type === 'text' ? block.metadata.characterCount : JSON.stringify(block.content).length
        })),
        streamingInfo: currentResponseParser.parsed.streamingInfo,
        exportTime: new Date().toISOString()
    };
    
    const dataStr = JSON.stringify(analysis, null, 2);
    const dataBlob = new Blob([dataStr], {type: 'application/json'});
    const url = URL.createObjectURL(dataBlob);
    
    const link = document.createElement('a');
    link.href = url;
    link.download = `anthropic-response-analysis-${Date.now()}.json`;
    link.click();
    
    URL.revokeObjectURL(url);
}

function isAnthropicResponse(responseBody) {
    try {
        // 检查非流式响应
        const data = JSON.parse(responseBody);
        return data.type === 'message' && data.role === 'assistant';
    } catch {
        // 检查流式响应（SSE 格式）
        return responseBody.includes('event: message_start') && 
               responseBody.includes('data: {"type"');
    }
}
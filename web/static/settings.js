// Settings Page JavaScript

let originalConfig = null;

// Save original configuration when page loads
document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    originalConfig = collectFormData();
});

function collectFormData() {
    return {
        server: {
            host: document.getElementById('serverHost').value,
            port: parseInt(document.getElementById('serverPort').value)
        },
        logging: {
            level: document.getElementById('logLevel').value,
            log_request_types: document.getElementById('logRequestTypes').value,
            log_request_body: document.getElementById('logRequestBody').value,
            log_response_body: document.getElementById('logResponseBody').value,
            log_directory: document.getElementById('logDirectory').value
        },
        validation: {
        },
        timeouts: {
            tls_handshake: document.getElementById('tlsHandshake').value,
            response_header: document.getElementById('responseHeader').value,
            idle_connection: document.getElementById('idleConnection').value,
            health_check_timeout: document.getElementById('healthCheckTimeout').value,
            check_interval: document.getElementById('checkInterval').value
        }
    };
}

function saveSettings() {
    const config = collectFormData();
    
    // Show loading status
    const saveBtn = document.querySelector('button[onclick="saveSettings()"]');
    const originalText = saveBtn.innerHTML;
    saveBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> 保存中...';
    saveBtn.disabled = true;
    
    fetch('/admin/api/settings', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(config)
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            throw new Error(data.error);
        }
        
        // Update original configuration
        originalConfig = config;
        
        // Show success message
        showAlert('配置已保存！配置文件已更新，重启服务后生效。', 'success');
    })
    .catch(error => {
        console.error('Error saving settings:', error);
        showAlert('保存失败: ' + error.message, 'danger');
    })
    .finally(() => {
        // Restore button state
        saveBtn.innerHTML = originalText;
        saveBtn.disabled = false;
    });
}

function resetSettings() {
    if (!originalConfig) return;
    
    // Restore form values
    document.getElementById('serverHost').value = originalConfig.server.host;
    document.getElementById('serverPort').value = originalConfig.server.port;
    document.getElementById('logLevel').value = originalConfig.logging.level;
    document.getElementById('logRequestTypes').value = originalConfig.logging.log_request_types;
    document.getElementById('logRequestBody').value = originalConfig.logging.log_request_body;
    document.getElementById('logResponseBody').value = originalConfig.logging.log_response_body;
    document.getElementById('logDirectory').value = originalConfig.logging.log_directory;
    document.getElementById('tlsHandshake').value = originalConfig.timeouts.tls_handshake;
    document.getElementById('responseHeader').value = originalConfig.timeouts.response_header;
    document.getElementById('idleConnection').value = originalConfig.timeouts.idle_connection;
    document.getElementById('healthCheckTimeout').value = originalConfig.timeouts.health_check_timeout;
    document.getElementById('checkInterval').value = originalConfig.timeouts.check_interval;
}
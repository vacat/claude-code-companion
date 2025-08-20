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
            proxy: {
                tls_handshake: document.getElementById('proxyTLSHandshake').value,
                response_header: document.getElementById('proxyResponseHeader').value,
                idle_connection: document.getElementById('proxyIdleConnection').value,
                overall_request: document.getElementById('proxyOverallRequest').value
            },
            health_check: {
                tls_handshake: document.getElementById('healthCheckTLSHandshake').value,
                response_header: document.getElementById('healthCheckResponseHeader').value,
                idle_connection: document.getElementById('healthCheckIdleConnection').value,
                overall_request: document.getElementById('healthCheckOverallRequest').value,
                check_interval: document.getElementById('healthCheckInterval').value
            }
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
    document.getElementById('proxyTLSHandshake').value = originalConfig.timeouts.proxy.tls_handshake;
    document.getElementById('proxyResponseHeader').value = originalConfig.timeouts.proxy.response_header;
    document.getElementById('proxyIdleConnection').value = originalConfig.timeouts.proxy.idle_connection;
    document.getElementById('proxyOverallRequest').value = originalConfig.timeouts.proxy.overall_request;
    document.getElementById('healthCheckTLSHandshake').value = originalConfig.timeouts.health_check.tls_handshake;
    document.getElementById('healthCheckResponseHeader').value = originalConfig.timeouts.health_check.response_header;
    document.getElementById('healthCheckIdleConnection').value = originalConfig.timeouts.health_check.idle_connection;
    document.getElementById('healthCheckOverallRequest').value = originalConfig.timeouts.health_check.overall_request;
    document.getElementById('healthCheckInterval').value = originalConfig.timeouts.health_check.check_interval;
}